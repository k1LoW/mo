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
	"time"

	"github.com/bmatcuk/doublestar/v4"
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

// GlobPattern represents a glob pattern being watched for new files.
type GlobPattern struct {
	Pattern      string // Absolute glob pattern
	PatternSlash string // Pre-converted to forward slashes for doublestar matching
	BaseDir      string // Base directory extracted via SplitPattern
	Group        string // Target group for matched files
}

// IsRecursive returns true if the pattern contains ** for recursive matching.
func (gp *GlobPattern) IsRecursive() bool {
	return strings.Contains(gp.Pattern, "**")
}

type State struct {
	mu          sync.RWMutex
	groups      map[string]*Group
	nextID      int
	subscribers map[chan sseEvent]struct{}
	watcher     *fsnotify.Watcher
	restartCh   chan string
	shutdownCh  chan struct{}
	patterns    []*GlobPattern
	watchedDirs map[string]int // directory → reference count
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
		shutdownCh:  make(chan struct{}, 1),
		watchedDirs: make(map[string]int),
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

	slog.Info("file added", "path", absPath, "group", groupName, "id", entry.ID)

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

func (s *State) ReorderFiles(groupName string, fileIDs []int) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	g, ok := s.groups[groupName]
	if !ok {
		return false
	}

	if len(fileIDs) != len(g.Files) {
		return false
	}

	idToFile := make(map[int]*FileEntry, len(g.Files))
	for _, f := range g.Files {
		idToFile[f.ID] = f
	}

	reordered := make([]*FileEntry, 0, len(fileIDs))
	for _, id := range fileIDs {
		f, ok := idToFile[id]
		if !ok {
			return false
		}
		reordered = append(reordered, f)
	}

	g.Files = reordered
	s.sendEvent(sseEvent{Name: "update", Data: "{}"})
	return true
}

func (s *State) MoveFile(id int, targetGroup string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	var file *FileEntry
	var sourceGroupName string
	var sourceGroup *Group
	for gName, g := range s.groups {
		for _, f := range g.Files {
			if f.ID == id {
				file = f
				sourceGroupName = gName
				sourceGroup = g
				break
			}
		}
		if file != nil {
			break
		}
	}
	if file == nil {
		return fmt.Errorf("file not found")
	}

	if sourceGroupName == targetGroup {
		return fmt.Errorf("file is already in group %q", targetGroup)
	}

	// Check for duplicate path in target group
	if tg, ok := s.groups[targetGroup]; ok {
		for _, f := range tg.Files {
			if f.Path == file.Path {
				return fmt.Errorf("file %q already exists in group %q", file.Name, targetGroup)
			}
		}
	}

	// Remove from source group
	for i, f := range sourceGroup.Files {
		if f.ID == id {
			sourceGroup.Files = append(sourceGroup.Files[:i], sourceGroup.Files[i+1:]...)
			break
		}
	}
	if len(sourceGroup.Files) == 0 {
		delete(s.groups, sourceGroupName)
	}

	// Add to target group
	tg, ok := s.groups[targetGroup]
	if !ok {
		tg = &Group{Name: targetGroup}
		s.groups[targetGroup] = tg
	}
	tg.Files = append(tg.Files, file)

	s.sendEvent(sseEvent{Name: "update", Data: "{}"})
	return nil
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

	slog.Info("file removed", "path", removedPath, "id", id)

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

	ch := make(chan sseEvent, 16)
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

// ShutdownCh returns a channel that signals when a shutdown is requested via API.
func (s *State) ShutdownCh() <-chan struct{} {
	return s.shutdownCh
}

// AddPattern registers a glob pattern for automatic file discovery.
// It performs an initial expansion to add existing matches and starts
// watching the base directory for new files.
func (s *State) AddPattern(absPattern, groupName string) (int, error) {
	// Use forward slashes for doublestar
	dsPattern := filepath.ToSlash(absPattern)
	base, relPat := doublestar.SplitPattern(dsPattern)
	base = filepath.FromSlash(base)

	info, err := os.Stat(base)
	if err != nil {
		return 0, fmt.Errorf("base directory %q does not exist: %w", base, err)
	}
	if !info.IsDir() {
		return 0, fmt.Errorf("base path %q is not a directory", base)
	}

	gp, added := func() (*GlobPattern, bool) {
		s.mu.Lock()
		defer s.mu.Unlock()
		for _, p := range s.patterns {
			if p.Pattern == absPattern && p.Group == groupName {
				return nil, false
			}
		}
		gp := &GlobPattern{
			Pattern:      absPattern,
			PatternSlash: dsPattern,
			BaseDir:      base,
			Group:        groupName,
		}
		s.patterns = append(s.patterns, gp)
		return gp, true
	}()
	if !added {
		return 0, nil
	}

	// Initial expansion
	matches, err := doublestar.Glob(os.DirFS(base), relPat, doublestar.WithFilesOnly())
	if err != nil {
		return 0, fmt.Errorf("glob expansion failed: %w", err)
	}

	for _, m := range matches {
		abs := filepath.Join(base, m)
		s.AddFile(abs, groupName)
	}

	s.watchDirsForPattern(gp)

	return len(matches), nil
}

// Patterns returns a copy of all registered glob patterns.
func (s *State) Patterns() []*GlobPattern {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]*GlobPattern, len(s.patterns))
	copy(result, s.patterns)
	return result
}

// PatternsForGroup returns the pattern strings for a specific group.
func (s *State) PatternsForGroup(groupName string) []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var result []string
	for _, p := range s.patterns {
		if p.Group == groupName {
			result = append(result, p.Pattern)
		}
	}
	return result
}

// RemovePattern removes a glob pattern from the watch list.
// Returns true if the pattern was found and removed.
func (s *State) RemovePattern(absPattern, groupName string) bool {
	var removed *GlobPattern
	func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		for i, p := range s.patterns {
			if p.Pattern == absPattern && p.Group == groupName {
				removed = p
				s.patterns = append(s.patterns[:i], s.patterns[i+1:]...)
				break
			}
		}
	}()

	if removed == nil {
		return false
	}

	s.walkDirsForPattern(removed, s.removeDirWatch)

	slog.Info("pattern removed", "pattern", absPattern, "group", groupName)
	s.mu.Lock()
	s.sendEvent(sseEvent{Name: "update", Data: "{}"})
	s.mu.Unlock()
	return true
}

// RestoreData represents the state to be persisted across restarts.
type RestoreData struct {
	Groups   map[string][]string `json:"groups"`
	Patterns map[string][]string `json:"patterns,omitempty"`
}

// WriteRestoreFile writes RestoreData to a temporary file and returns the path.
func WriteRestoreFile(data RestoreData) (string, error) {
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

// ExportState writes the current groups, file paths, and patterns to a temporary file and returns the path.
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

	if len(s.patterns) > 0 {
		data.Patterns = make(map[string][]string)
		for _, p := range s.patterns {
			data.Patterns[p.Group] = append(data.Patterns[p.Group], p.Pattern)
		}
	}

	return WriteRestoreFile(data)
}

func (s *State) walkDirsForPattern(gp *GlobPattern, fn func(string)) {
	if s.watcher == nil {
		return
	}
	if !gp.IsRecursive() {
		fn(gp.BaseDir)
		return
	}

	if err := filepath.WalkDir(gp.BaseDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			fn(path)
		}
		return nil
	}); err != nil {
		slog.Warn("failed to walk directories for pattern", "pattern", gp.Pattern, "base", gp.BaseDir, "error", err)
	}
}

func (s *State) removeDirWatch(dir string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if count, ok := s.watchedDirs[dir]; ok {
		count--
		if count <= 0 {
			delete(s.watchedDirs, dir)
			if s.watcher != nil {
				if err := s.watcher.Remove(dir); err != nil {
					slog.Warn("failed to remove directory watch", "dir", dir, "error", err)
				}
			}
		} else {
			s.watchedDirs[dir] = count
		}
	}
}

func (s *State) watchLoop() {
	for {
		select {
		case event, ok := <-s.watcher.Events:
			if !ok {
				return
			}
			ids := s.findIDsByPath(event.Name)
			if len(ids) > 0 {
				if event.Has(fsnotify.Write) || event.Has(fsnotify.Create) {
					slog.Info("file changed", "path", event.Name)
					s.notifyFileChanged(ids)
				}
				// Editors using atomic save (write-to-temp + rename) cause
				// the original inode to disappear, which removes the watch.
				// Re-add the watch so subsequent saves are still detected.
				if event.Has(fsnotify.Remove) || event.Has(fsnotify.Rename) {
					time.AfterFunc(100*time.Millisecond, func() {
						if err := s.watcher.Add(event.Name); err != nil {
							// File is actually gone — remove from file list
							slog.Info("file deleted, removing from list", "path", event.Name)
							for _, id := range ids {
								s.RemoveFile(id)
							}
						} else {
							slog.Info("re-watching file", "path", event.Name)
							s.notifyFileChanged(ids)
						}
					})
				}
			}
			if event.Has(fsnotify.Create) {
				s.handleCreateForGlobs(event.Name)
			}
		case err, ok := <-s.watcher.Errors:
			if !ok {
				return
			}
			slog.Warn("file watcher error", "error", err)
		}
	}
}

func (s *State) notifyFileChanged(ids []int) {
	for _, id := range ids {
		s.sendEvent(sseEvent{
			Name: "file-changed",
			Data: fmt.Sprintf(`{"id":%d}`, id),
		})
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
			slog.Warn("SSE event dropped (subscriber buffer full)", "event", e.Name)
		}
	}
}

func (s *State) watchDirsForPattern(gp *GlobPattern) {
	s.walkDirsForPattern(gp, s.addDirWatch)
}

func (s *State) addDirWatch(dir string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.watchedDirs[dir]++
	if s.watchedDirs[dir] == 1 && s.watcher != nil {
		if err := s.watcher.Add(dir); err != nil {
			slog.Warn("failed to watch directory", "path", dir, "error", err)
		}
	}
}

func (s *State) handleCreateForGlobs(path string) {
	s.mu.RLock()
	if len(s.patterns) == 0 {
		s.mu.RUnlock()
		return
	}
	patterns := make([]*GlobPattern, len(s.patterns))
	copy(patterns, s.patterns)
	s.mu.RUnlock()

	info, err := os.Stat(path)
	if err != nil {
		return
	}

	if info.IsDir() {
		watched := false
		for _, gp := range patterns {
			if !gp.IsRecursive() {
				continue
			}
			if !strings.HasPrefix(path, gp.BaseDir) {
				continue
			}
			if !watched {
				s.addDirWatch(path)
				// Scan directory contents for matching files
				filepath.WalkDir(path, func(p string, d os.DirEntry, err error) error { //nolint:errcheck
					if err != nil || d.IsDir() {
						return nil
					}
					s.matchAndAddFile(p, patterns)
					return nil
				})
				watched = true
			}
		}
		return
	}

	s.matchAndAddFile(path, patterns)
}

func (s *State) matchAndAddFile(path string, patterns []*GlobPattern) {
	dsPath := filepath.ToSlash(path)
	for _, gp := range patterns {
		matched, err := doublestar.Match(gp.PatternSlash, dsPath)
		if err != nil {
			continue
		}
		if matched {
			s.AddFile(path, gp.Group)
			slog.Info("auto-added file via glob", "path", path, "pattern", gp.Pattern, "group", gp.Group)
			return
		}
	}
}

type reorderFilesRequest struct {
	Group   string `json:"group"`
	FileIDs []int  `json:"fileIds"`
}

type moveFileRequest struct {
	Group string `json:"group"`
}

type addFileRequest struct {
	Path  string `json:"path"`
	Group string `json:"group"`
}

type patternRequest struct {
	Pattern string `json:"pattern"`
	Group   string `json:"group"`
}

type addPatternResponse struct {
	Matched int `json:"matched"`
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
	mux.HandleFunc("PUT /_/api/files/{id}/group", handleMoveFile(state))
	mux.HandleFunc("GET /_/api/groups", handleGroups(state))
	mux.HandleFunc("PUT /_/api/reorder", handleReorderFiles(state))
	mux.HandleFunc("GET /_/api/files/{id}/content", handleFileContent(state))
	mux.HandleFunc("GET /_/api/files/{id}/raw/{path...}", handleFileRaw(state))
	mux.HandleFunc("POST /_/api/files/open", handleOpenFile(state))
	mux.HandleFunc("POST /_/api/patterns", handleAddPattern(state))
	mux.HandleFunc("DELETE /_/api/patterns", handleRemovePattern(state))
	mux.HandleFunc("POST /_/api/restart", handleRestart(state))
	mux.HandleFunc("POST /_/api/shutdown", handleShutdown(state))
	mux.HandleFunc("GET /_/api/status", handleStatus(state))
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

		group, err := ResolveGroupName(req.Group)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
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

func handleMoveFile(state *State) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.Atoi(r.PathValue("id"))
		if err != nil {
			http.Error(w, "invalid file id", http.StatusBadRequest)
			return
		}
		var req moveFileRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		group, err := ResolveGroupName(req.Group)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := state.MoveFile(id, group); err != nil {
			http.Error(w, err.Error(), http.StatusConflict)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func handleReorderFiles(state *State) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req reorderFilesRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		group, err := ResolveGroupName(req.Group)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if !state.ReorderFiles(group, req.FileIDs) {
			http.Error(w, "invalid file IDs or group not found", http.StatusBadRequest)
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
			groupName = DefaultGroup
		}

		newEntry := state.AddFile(absPath, groupName)
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(newEntry); err != nil {
			slog.Error("failed to encode response", "error", err)
		}
	}
}

func handleAddPattern(state *State) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req patternRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		group, err := ResolveGroupName(req.Group)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		matched, err := state.AddPattern(req.Pattern, group)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(addPatternResponse{Matched: matched}); err != nil {
			slog.Error("failed to encode response", "error", err)
		}
	}
}

func handleRemovePattern(state *State) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req patternRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		group, err := ResolveGroupName(req.Group)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		if !state.RemovePattern(req.Pattern, group) {
			http.Error(w, "pattern not found", http.StatusNotFound)
			return
		}

		w.WriteHeader(http.StatusNoContent)
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

func handleShutdown(state *State) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
		select {
		case state.shutdownCh <- struct{}{}:
		default:
		}
	}
}

type statusGroup struct {
	Group
	Patterns []string `json:"patterns,omitempty"`
}

func handleStatus(state *State) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		groups := state.Groups()
		statusGroups := make([]statusGroup, len(groups))
		for i, g := range groups {
			statusGroups[i] = statusGroup{
				Group:    g,
				Patterns: state.PatternsForGroup(g.Name),
			}
		}

		resp := struct {
			Version  string        `json:"version"`
			Revision string        `json:"revision"`
			PID      int           `json:"pid"`
			Groups   []statusGroup `json:"groups"`
		}{
			Version:  version.Version,
			Revision: version.Revision,
			PID:      os.Getpid(),
			Groups:   statusGroups,
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			slog.Error("failed to encode status response", "error", err)
		}
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
