package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/fsnotify/fsnotify"
	"github.com/k1LoW/donegroup"
	"github.com/k1LoW/mo/internal/static"
	"github.com/k1LoW/mo/version"
)

type FileEntry struct {
	Name string `json:"name"`
	ID   int    `json:"id"`
	Path string `json:"path"`
}

type Group struct {
	Name  string       `json:"name"`
	Files []*FileEntry `json:"files"`
}

type sseEvent struct {
	Name string // SSE event name
	Data string // SSE data payload (JSON)
}

type State struct {
	mu          sync.RWMutex
	groups      map[string]*Group
	nextID      int
	subscribers map[chan sseEvent]struct{}
	watcher     *fsnotify.Watcher
	restartCh   chan string
}

func NewState(ctx context.Context) *State {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		slog.Warn("failed to create file watcher", "error", err)
	}

	s := &State{
		groups:      make(map[string]*Group),
		nextID:      1,
		subscribers: make(map[chan sseEvent]struct{}),
		watcher:     w,
		restartCh:   make(chan string, 1),
	}

	if w != nil {
		donegroup.Go(ctx, func() error {
			s.watchLoop()
			return nil
		})
	}

	return s
}

func (s *State) AddFile(absPath, groupName string) *FileEntry {
	s.mu.Lock()
	defer s.mu.Unlock()

	g, ok := s.groups[groupName]
	if !ok {
		g = &Group{Name: groupName}
		s.groups[groupName] = g
	}

	for _, f := range g.Files {
		if f.Path == absPath {
			return f
		}
	}

	entry := &FileEntry{
		Name: filepath.Base(absPath),
		ID:   s.nextID,
		Path: absPath,
	}
	s.nextID++
	g.Files = append(g.Files, entry)

	if s.watcher != nil {
		if err := s.watcher.Add(absPath); err != nil {
			slog.Warn("failed to watch file", "path", absPath, "error", err)
		}
	}

	s.sendEvent(sseEvent{Name: "update", Data: "{}"})
	return entry
}

func (s *State) Groups() []Group {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]Group, 0, len(s.groups))
	for _, g := range s.groups {
		result = append(result, *g)
	}
	return result
}

func (s *State) FindFile(id int) *FileEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, g := range s.groups {
		for _, f := range g.Files {
			if f.ID == id {
				return f
			}
		}
	}
	return nil
}

// FindGroupForFile returns the group name for a given file ID.
func (s *State) FindGroupForFile(id int) string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, g := range s.groups {
		for _, f := range g.Files {
			if f.ID == id {
				return g.Name
			}
		}
	}
	return ""
}

func (s *State) RemoveFile(id int) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	var removedPath string
	found := false
	for gName, g := range s.groups {
		for i, f := range g.Files {
			if f.ID == id {
				removedPath = f.Path
				g.Files = append(g.Files[:i], g.Files[i+1:]...)
				if len(g.Files) == 0 {
					delete(s.groups, gName)
				}
				found = true
				break
			}
		}
		if found {
			break
		}
	}
	if !found {
		return false
	}

	// Remove watcher only if no other file references the same path
	if s.watcher != nil && removedPath != "" {
		stillReferenced := false
		for _, g := range s.groups {
			for _, f := range g.Files {
				if f.Path == removedPath {
					stillReferenced = true
					break
				}
			}
			if stillReferenced {
				break
			}
		}
		if !stillReferenced {
			if err := s.watcher.Remove(removedPath); err != nil {
				slog.Warn("failed to unwatch file", "path", removedPath, "error", err)
			}
		}
	}

	s.sendEvent(sseEvent{Name: "update", Data: "{}"})
	return true
}

func (s *State) Subscribe() chan sseEvent {
	s.mu.Lock()
	defer s.mu.Unlock()

	ch := make(chan sseEvent, 4)
	s.subscribers[ch] = struct{}{}
	return ch
}

func (s *State) Unsubscribe(ch chan sseEvent) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.subscribers[ch]; ok {
		delete(s.subscribers, ch)
		close(ch)
	}
}

// CloseAllSubscribers closes all SSE subscriber channels so that
// SSE handlers return and in-flight requests complete before Shutdown.
func (s *State) CloseAllSubscribers() {
	s.mu.Lock()
	defer s.mu.Unlock()

	for ch := range s.subscribers {
		close(ch)
		delete(s.subscribers, ch)
	}

	if s.watcher != nil {
		s.watcher.Close()
	}
}

// RestartCh returns a channel that receives the restore file path when a restart is requested.
func (s *State) RestartCh() <-chan string {
	return s.restartCh
}

// RestoreData represents the state to be persisted across restarts.
type RestoreData struct {
	Groups map[string][]string `json:"groups"`
}

// ExportState writes the current groups and file paths to a temporary file and returns the path.
func (s *State) ExportState() (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	data := RestoreData{
		Groups: make(map[string][]string, len(s.groups)),
	}
	for name, g := range s.groups {
		paths := make([]string, 0, len(g.Files))
		for _, f := range g.Files {
			paths = append(paths, f.Path)
		}
		data.Groups[name] = paths
	}

	f, err := os.CreateTemp("", "mo-restore-*.json")
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %w", err)
	}
	defer f.Close()

	if err := json.NewEncoder(f).Encode(data); err != nil {
		os.Remove(f.Name()) //nolint:gosec // Path is from our own CreateTemp, not user-supplied
		return "", fmt.Errorf("failed to write restore data: %w", err)
	}

	return f.Name(), nil
}

func (s *State) watchLoop() {
	for {
		select {
		case event, ok := <-s.watcher.Events:
			if !ok {
				return
			}
			if event.Has(fsnotify.Write) {
				ids := s.findIDsByPath(event.Name)
				for _, id := range ids {
					s.sendEvent(sseEvent{
						Name: "file-changed",
						Data: fmt.Sprintf(`{"id":%d}`, id),
					})
				}
			}
		case _, ok := <-s.watcher.Errors:
			if !ok {
				return
			}
		}
	}
}

func (s *State) findIDsByPath(absPath string) []int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var ids []int
	for _, g := range s.groups {
		for _, f := range g.Files {
			if f.Path == absPath {
				ids = append(ids, f.ID)
			}
		}
	}
	return ids
}

func (s *State) sendEvent(e sseEvent) {
	for ch := range s.subscribers {
		select {
		case ch <- e:
		default:
		}
	}
}

type addFileRequest struct {
	Path  string `json:"path"`
	Group string `json:"group"`
}

type fileContentResponse struct {
	Content string `json:"content"`
	BaseDir string `json:"baseDir"`
}

type openFileRequest struct {
	FileID int    `json:"fileId"`
	Path   string `json:"path"`
}

func NewHandler(state *State) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("POST /_/api/files", handleAddFile(state))
	mux.HandleFunc("DELETE /_/api/files/{id}", handleRemoveFile(state))
	mux.HandleFunc("GET /_/api/groups", handleGroups(state))
	mux.HandleFunc("GET /_/api/files/{id}/content", handleFileContent(state))
	mux.HandleFunc("GET /_/api/files/{id}/raw/{path...}", handleFileRaw(state))
	mux.HandleFunc("POST /_/api/files/open", handleOpenFile(state))
	mux.HandleFunc("POST /_/api/restart", handleRestart(state))
	mux.HandleFunc("GET /_/api/version", handleVersion())
	mux.HandleFunc("GET /_/events", handleSSE(state))
	mux.HandleFunc("GET /", handleSPA())

	return mux
}

func handleAddFile(state *State) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req addFileRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		absPath, err := filepath.Abs(req.Path)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		if _, err := os.Stat(absPath); err != nil {
			http.Error(w, fmt.Sprintf("file not found: %s", absPath), http.StatusBadRequest)
			return
		}

		group := req.Group
		if group == "" {
			group = "default"
		}

		entry := state.AddFile(absPath, group)
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(entry); err != nil {
			slog.Error("failed to encode response", "error", err)
		}
	}
}

func handleRemoveFile(state *State) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.Atoi(r.PathValue("id"))
		if err != nil {
			http.Error(w, "invalid file id", http.StatusBadRequest)
			return
		}
		if !state.RemoveFile(id) {
			http.Error(w, "file not found", http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func handleGroups(state *State) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		groups := state.Groups()
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(groups); err != nil {
			slog.Error("failed to encode response", "error", err)
		}
	}
}

func handleFileContent(state *State) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.Atoi(r.PathValue("id"))
		if err != nil {
			http.Error(w, "invalid file id", http.StatusBadRequest)
			return
		}

		entry := state.FindFile(id)
		if entry == nil {
			http.Error(w, "file not found", http.StatusNotFound)
			return
		}

		content, err := os.ReadFile(entry.Path) //nolint:gosec // Path is server-managed, not user-supplied
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		resp := fileContentResponse{
			Content: string(content),
			BaseDir: filepath.Dir(entry.Path),
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			slog.Error("failed to encode response", "error", err)
		}
	}
}

func handleFileRaw(state *State) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.Atoi(r.PathValue("id"))
		if err != nil {
			http.Error(w, "invalid file id", http.StatusBadRequest)
			return
		}

		entry := state.FindFile(id)
		if entry == nil {
			http.Error(w, "file not found", http.StatusNotFound)
			return
		}

		relPath := r.PathValue("path")
		absPath := filepath.Join(filepath.Dir(entry.Path), relPath)
		absPath = filepath.Clean(absPath)

		// Prevent directory traversal outside the base directory
		baseDir := filepath.Dir(entry.Path)
		if !strings.HasPrefix(absPath, baseDir) {
			http.Error(w, "access denied", http.StatusForbidden)
			return
		}

		http.ServeFile(w, r, absPath)
	}
}

func handleOpenFile(state *State) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req openFileRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		entry := state.FindFile(req.FileID)
		if entry == nil {
			http.Error(w, "source file not found", http.StatusNotFound)
			return
		}

		absPath := filepath.Join(filepath.Dir(entry.Path), req.Path)
		absPath = filepath.Clean(absPath)

		if _, err := os.Stat(absPath); err != nil {
			http.Error(w, fmt.Sprintf("file not found: %s", absPath), http.StatusNotFound)
			return
		}

		groupName := state.FindGroupForFile(req.FileID)
		if groupName == "" {
			groupName = "default"
		}

		newEntry := state.AddFile(absPath, groupName)
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(newEntry); err != nil {
			slog.Error("failed to encode response", "error", err)
		}
	}
}

func handleRestart(state *State) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		restoreFile, err := state.ExportState()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusAccepted)

		// Send restart signal after response is written
		state.restartCh <- restoreFile
	}
}

func handleVersion() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(map[string]string{
			"version":  version.Version,
			"revision": version.Revision,
		}); err != nil {
			slog.Error("failed to encode version response", "error", err)
		}
	}
}

func handleSSE(state *State) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming not supported", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		ch := state.Subscribe()
		defer state.Unsubscribe(ch)

		ctx := r.Context()
		for {
			select {
			case <-ctx.Done():
				return
			case e, ok := <-ch:
				if !ok {
					return
				}
				fmt.Fprintf(w, "event: %s\ndata: %s\n\n", e.Name, e.Data)
				flusher.Flush()
			}
		}
	}
}

func handleSPA() http.HandlerFunc {
	distFS, err := fs.Sub(static.Frontend, "dist")
	if err != nil {
		slog.Error("failed to create sub filesystem", "error", err)
		os.Exit(1)
	}
	fileServer := http.FileServer(http.FS(distFS))

	return func(w http.ResponseWriter, r *http.Request) {
		// Try to serve the exact file first
		path := r.URL.Path
		if path == "/" {
			path = "/index.html"
		}

		f, err := distFS.Open(strings.TrimPrefix(path, "/"))
		if err == nil {
			f.Close()
			fileServer.ServeHTTP(w, r)
			return
		}

		// SPA fallback: serve index.html for all non-file routes
		r.URL.Path = "/"
		fileServer.ServeHTTP(w, r)
	}
}
