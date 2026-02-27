package server

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/fsnotify/fsnotify"
	"github.com/k1LoW/mo/internal/static"
)

type FileEntry struct {
	Name string `json:"name"`
	ID   int    `json:"id"`
	Path string `json:"-"`
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
}

func NewState() *State {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		log.Printf("warning: failed to create file watcher: %v", err)
	}

	s := &State{
		groups:      make(map[string]*Group),
		nextID:      1,
		subscribers: make(map[chan sseEvent]struct{}),
		watcher:     w,
	}

	if w != nil {
		go s.watchLoop()
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
			log.Printf("warning: failed to watch %s: %v", absPath, err)
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
	mux.HandleFunc("GET /_/api/groups", handleGroups(state))
	mux.HandleFunc("GET /_/api/files/{id}/content", handleFileContent(state))
	mux.HandleFunc("GET /_/api/files/{id}/raw/{path...}", handleFileRaw(state))
	mux.HandleFunc("POST /_/api/files/open", handleOpenFile(state))
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
			log.Printf("failed to encode response: %v", err)
		}
	}
}

func handleGroups(state *State) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		groups := state.Groups()
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(groups); err != nil {
			log.Printf("failed to encode response: %v", err)
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
			log.Printf("failed to encode response: %v", err)
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
			log.Printf("failed to encode response: %v", err)
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
		log.Fatalf("failed to create sub filesystem: %v", err)
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
