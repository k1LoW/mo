package server

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
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
	Name     string `json:"name"`
	ID       string `json:"id"`
	Path     string `json:"path"`
	Uploaded bool   `json:"uploaded,omitempty"`
	content  string // in-memory content for uploaded files
}

// FileID generates a deterministic file ID from an absolute path.
// The ID is the first 8 characters of the SHA-256 hex digest.
func FileID(absPath string) string {
	h := sha256.Sum256([]byte(absPath))
	return hex.EncodeToString(h[:])[:8]
}

type Group struct {
	Name  string       `json:"name"`
	Files []*FileEntry `json:"files"`
}

type sseEvent struct {
	Name string // SSE event name
	Data string // SSE data payload (JSON)
}

const (
	eventUpdate      = "update"
	eventFileChanged = "file-changed"
)

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
	subscribers map[chan sseEvent]struct{}
	subMu       sync.RWMutex
	watcher     *fsnotify.Watcher
	restartCh   chan string
	shutdownCh  chan struct{}
	patterns    []*GlobPattern
	watchedDirs map[string]int // directory → reference count

	fileChangeDebounce time.Duration
	fileChangeTimers   map[string]*time.Timer

	backupCh     chan struct{}     // dirty signal (buffered, size 1)
	backupSaveFn func(RestoreData) // backup write callback
	backupDone   chan struct{}     // closed when backupLoop exits

	noRestart bool
	noDelete  bool
	noFileMove    bool
	shareable bool
}

// Configure sets server behaviour flags. Call before serving.
func (s *State) Configure(noRestart, noDelete, noFileMove, shareable bool) {
	s.noRestart = noRestart
	s.noDelete = noDelete
	s.noFileMove = noFileMove
	s.shareable = shareable
}

const defaultFileChangeDebounce = 200 * time.Millisecond

func NewState(ctx context.Context) *State {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		slog.Warn("failed to create file watcher", "error", err)
	}

	s := &State{
		groups:             make(map[string]*Group),
		subscribers:        make(map[chan sseEvent]struct{}),
		watcher:            w,
		restartCh:          make(chan string, 1),
		shutdownCh:         make(chan struct{}, 1),
		watchedDirs:        make(map[string]int),
		fileChangeDebounce: defaultFileChangeDebounce,
		fileChangeTimers:   make(map[string]*time.Timer),
	}

	if w != nil {
		donegroup.Go(ctx, func() error {
			s.watchLoop()
			return nil
		})
	}

	return s
}

// ErrBinaryFile is returned when a file is detected as binary.
var ErrBinaryFile = errors.New("binary file is not supported")

// isBinaryFile checks whether the file at the given path is binary
// by reading the first 8KB and looking for NUL bytes (same heuristic as Git).
func isBinaryFile(path string) (bool, error) {
	fi, err := os.Stat(path)
	if err != nil {
		return false, err
	}
	if !fi.Mode().IsRegular() {
		return false, fmt.Errorf("not a regular file: %s", path)
	}
	f, err := os.Open(path) //nolint:gosec
	if err != nil {
		return false, err
	}
	defer f.Close()
	var buf [8192]byte
	n, err := f.Read(buf[:])
	if err != nil && !errors.Is(err, io.EOF) {
		return false, err
	}
	if n == 0 {
		return false, nil
	}
	return bytes.IndexByte(buf[:n], 0) >= 0, nil
}

func (s *State) AddFile(absPath, groupName string) (*FileEntry, error) {
	// Check for duplicates before doing any I/O.
	s.mu.RLock()
	if g, ok := s.groups[groupName]; ok {
		for _, f := range g.Files {
			if f.Path == absPath {
				s.mu.RUnlock()
				return f, nil
			}
		}
	}
	s.mu.RUnlock()

	bin, err := isBinaryFile(absPath)
	if err != nil {
		// If the file doesn't exist (yet), allow adding it.
		if !os.IsNotExist(err) {
			return nil, fmt.Errorf("failed to read file %s: %w", absPath, err)
		}
	} else if bin {
		return nil, fmt.Errorf("%s: %w", absPath, ErrBinaryFile)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	g, ok := s.groups[groupName]
	if !ok {
		g = &Group{Name: groupName}
		s.groups[groupName] = g
	}

	// Re-check after re-acquiring the lock.
	for _, f := range g.Files {
		if f.Path == absPath {
			return f, nil
		}
	}

	entry := &FileEntry{
		Name: filepath.Base(absPath),
		ID:   FileID(absPath),
		Path: absPath,
	}
	g.Files = append(g.Files, entry)

	if s.watcher != nil {
		if err := s.watcher.Add(absPath); err != nil {
			slog.Warn("failed to watch file", "path", absPath, "error", err)
		}
	}

	slog.Debug("file added", "path", absPath, "group", groupName, "id", entry.ID)

	s.sendEvent(sseEvent{Name: eventUpdate, Data: "{}"})
	return entry, nil
}

func (s *State) AddUploadedFile(name, content, groupName string) *FileEntry {
	s.mu.Lock()
	defer s.mu.Unlock()

	h := sha256.New()
	h.Write([]byte("upload:"))
	h.Write([]byte(content))
	id := "u" + hex.EncodeToString(h.Sum(nil))[:7]

	// Check all groups for an existing entry with the same ID
	for _, grp := range s.groups {
		for _, f := range grp.Files {
			if f.ID == id {
				return f
			}
		}
	}

	g, ok := s.groups[groupName]
	if !ok {
		g = &Group{Name: groupName}
		s.groups[groupName] = g
	}

	entry := &FileEntry{
		Name:     name,
		ID:       id,
		Uploaded: true,
		content:  content,
	}
	g.Files = append(g.Files, entry)

	slog.Debug("uploaded file added", "name", name, "group", groupName, "id", entry.ID)

	s.sendEvent(sseEvent{Name: eventUpdate, Data: "{}"})
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

func (s *State) FindFile(id string) *FileEntry {
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
func (s *State) FindGroupForFile(id string) string {
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

func (s *State) ReorderFiles(groupName string, fileIDs []string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	g, ok := s.groups[groupName]
	if !ok {
		return false
	}

	if len(fileIDs) != len(g.Files) {
		return false
	}

	idToFile := make(map[string]*FileEntry, len(g.Files))
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
	s.sendEvent(sseEvent{Name: eventUpdate, Data: "{}"})
	return true
}

func (s *State) MoveFile(id string, targetGroup string) error {
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

	// Check for duplicate in target group (by path for filesystem files, by ID for uploaded files)
	if tg, ok := s.groups[targetGroup]; ok {
		for _, f := range tg.Files {
			if file.Uploaded {
				if f.ID == file.ID {
					return fmt.Errorf("file %q already exists in group %q", file.Name, targetGroup)
				}
			} else {
				if f.Path == file.Path {
					return fmt.Errorf("file %q already exists in group %q", file.Name, targetGroup)
				}
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
	if len(sourceGroup.Files) == 0 && !s.groupHasPatterns(sourceGroupName) {
		delete(s.groups, sourceGroupName)
	}

	// Add to target group
	tg, ok := s.groups[targetGroup]
	if !ok {
		tg = &Group{Name: targetGroup}
		s.groups[targetGroup] = tg
	}
	tg.Files = append(tg.Files, file)

	s.sendEvent(sseEvent{Name: eventUpdate, Data: "{}"})
	return nil
}

func (s *State) RemoveFile(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	var removedPath string
	found := false
	for gName, g := range s.groups {
		for i, f := range g.Files {
			if f.ID == id {
				removedPath = f.Path
				g.Files = append(g.Files[:i], g.Files[i+1:]...)
				if len(g.Files) == 0 && !s.groupHasPatterns(gName) {
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

	slog.Info("file removed", "path", removedPath, "id", id) //nolint:gosec // G706: removedPath is from internal state, not direct user input

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

	s.sendEvent(sseEvent{Name: eventUpdate, Data: "{}"})
	return true
}

func (s *State) Subscribe() chan sseEvent {
	s.subMu.Lock()
	defer s.subMu.Unlock()

	ch := make(chan sseEvent, 16)
	s.subscribers[ch] = struct{}{}
	return ch
}

func (s *State) Unsubscribe(ch chan sseEvent) {
	s.subMu.Lock()
	defer s.subMu.Unlock()

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

	s.subMu.Lock()
	for ch := range s.subscribers {
		close(ch)
		delete(s.subscribers, ch)
	}
	s.subMu.Unlock()

	if s.watcher != nil {
		s.watcher.Close()
	}
	for path, timer := range s.fileChangeTimers {
		timer.Stop()
		delete(s.fileChangeTimers, path)
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
func (s *State) AddPattern(absPattern, groupName string) ([]*FileEntry, error) {
	// Use forward slashes for doublestar
	dsPattern := filepath.ToSlash(absPattern)
	base, relPat := doublestar.SplitPattern(dsPattern)
	base = filepath.FromSlash(base)

	info, err := os.Stat(base)
	if err != nil {
		return nil, fmt.Errorf("base directory %q does not exist: %w", base, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("base path %q is not a directory", base)
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
		// Ensure the group exists even if no files match yet.
		if _, ok := s.groups[groupName]; !ok {
			s.groups[groupName] = &Group{Name: groupName}
		}
		return gp, true
	}()
	if !added {
		return nil, nil
	}

	// Initial expansion
	matches, err := doublestar.Glob(os.DirFS(base), relPat, doublestar.WithFilesOnly())
	if err != nil {
		return nil, fmt.Errorf("glob expansion failed: %w", err)
	}

	var entries []*FileEntry
	for _, m := range matches {
		abs := filepath.Join(base, m)
		entry, err := s.AddFile(abs, groupName)
		if err != nil {
			slog.Warn("skipping file", "path", abs, "error", err)
			continue
		}
		entries = append(entries, entry)
	}

	s.watchDirsForPattern(gp)

	return entries, nil
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
	// Clean up empty group when last pattern is removed and no files remain.
	if g, ok := s.groups[groupName]; ok && len(g.Files) == 0 && !s.groupHasPatterns(groupName) {
		delete(s.groups, groupName)
	}
	s.sendEvent(sseEvent{Name: eventUpdate, Data: "{}"})
	s.mu.Unlock()
	return true
}

// UploadedFileData represents an uploaded file's content for persistence.
type UploadedFileData struct {
	Name    string `json:"name"`
	Content string `json:"content"`
	Group   string `json:"group"`
}

// RestoreData represents the state to be persisted across restarts.
type RestoreData struct {
	Groups        map[string][]string `json:"groups"`
	Patterns      map[string][]string `json:"patterns,omitempty"`
	UploadedFiles []UploadedFileData  `json:"uploadedFiles,omitempty"`
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
	return WriteRestoreFile(s.snapshotRestoreData())
}

// EnableBackup starts a background goroutine that periodically saves state
// via the provided callback when state changes are detected.
func (s *State) EnableBackup(ctx context.Context, saveFn func(RestoreData)) {
	s.backupCh = make(chan struct{}, 1)
	s.backupSaveFn = saveFn
	s.backupDone = make(chan struct{})
	donegroup.Go(ctx, func() error {
		defer close(s.backupDone)
		s.backupLoop(ctx)
		return nil
	})
}

// snapshotRestoreData creates a RestoreData snapshot of the current state.
// Caller must hold s.mu (at least RLock).
func (s *State) snapshotRestoreData() RestoreData {
	data := RestoreData{
		Groups: make(map[string][]string, len(s.groups)),
	}
	for name, g := range s.groups {
		paths := make([]string, 0, len(g.Files))
		for _, f := range g.Files {
			if f.Uploaded {
				data.UploadedFiles = append(data.UploadedFiles, UploadedFileData{
					Name:    f.Name,
					Content: f.content,
					Group:   name,
				})
				continue
			}
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

	return data
}

// markDirty signals that state has changed and a backup save is needed.
// Non-blocking: safe to call while holding s.mu.
func (s *State) markDirty() {
	if s.backupCh == nil {
		return
	}
	select {
	case s.backupCh <- struct{}{}:
	default:
	}
}

func (s *State) backupLoop(ctx context.Context) {
	const debounce = 1 * time.Second
	timer := time.NewTimer(debounce)
	timer.Stop()
	for {
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			s.saveBackup()
			return
		case _, ok := <-s.backupCh:
			if !ok {
				return
			}
			timer.Reset(debounce)
		case <-timer.C:
			s.saveBackup()
		}
	}
}

func (s *State) saveBackup() {
	if s.backupSaveFn == nil {
		return
	}
	s.mu.RLock()
	data := s.snapshotRestoreData()
	s.mu.RUnlock()
	s.backupSaveFn(data)
}

// groupHasPatterns reports whether the group has any registered watch patterns.
// Caller must hold s.mu.
func (s *State) groupHasPatterns(groupName string) bool {
	for _, p := range s.patterns {
		if p.Group == groupName {
			return true
		}
	}
	return false
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
			// Best-effort: still process this path so unwatch can decrement refcounts.
			fn(path)
			return fs.SkipDir
		}
		if d.IsDir() {
			fn(path)
		}
		return nil
	}); err != nil {
		// BaseDir may have been deleted; still clean up the base directory entry.
		fn(gp.BaseDir)
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
					s.scheduleFileChanged(event.Name)
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
							s.scheduleFileChanged(event.Name)
						}
					})
				}
			}
			if (event.Has(fsnotify.Rename) || event.Has(fsnotify.Remove)) && s.isWatchedDir(event.Name) {
				s.handleDirMove(event.Name)
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

func (s *State) scheduleFileChanged(absPath string) {
	if s.fileChangeDebounce <= 0 {
		s.notifyFileChangedByPath(absPath)
		return
	}

	s.mu.Lock()
	if timer, ok := s.fileChangeTimers[absPath]; ok {
		timer.Stop()
	}
	debounce := s.fileChangeDebounce
	var timer *time.Timer
	timer = time.AfterFunc(debounce, func() {
		s.mu.Lock()
		current, ok := s.fileChangeTimers[absPath]
		if ok && current == timer {
			delete(s.fileChangeTimers, absPath)
		}
		s.mu.Unlock()
		if ok && current == timer {
			s.notifyFileChangedByPath(absPath)
		}
	})
	s.fileChangeTimers[absPath] = timer
	s.mu.Unlock()
}

func (s *State) notifyFileChangedByPath(absPath string) {
	ids := s.findIDsByPath(absPath)
	if len(ids) == 0 {
		return
	}
	s.notifyFileChanged(ids)
}

func (s *State) notifyFileChanged(ids []string) {
	for _, id := range ids {
		b, err := json.Marshal(struct {
			ID string `json:"id"`
		}{ID: id})
		if err != nil {
			slog.Error("notifyFileChanged", "err", err)
			continue
		}
		s.sendEvent(sseEvent{
			Name: eventFileChanged,
			Data: string(b),
		})
	}
}

func (s *State) findIDsByPath(absPath string) []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var ids []string
	for _, g := range s.groups {
		for _, f := range g.Files {
			if f.Path == absPath {
				ids = append(ids, f.ID)
			}
		}
	}
	return ids
}

func (s *State) findIDsByPathPrefix(dirPath string) []string {
	prefix := dirPath + string(filepath.Separator)
	s.mu.RLock()
	defer s.mu.RUnlock()

	var ids []string
	for _, g := range s.groups {
		for _, f := range g.Files {
			if strings.HasPrefix(f.Path, prefix) {
				ids = append(ids, f.ID)
			}
		}
	}
	return ids
}

func (s *State) isWatchedDir(path string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, ok := s.watchedDirs[path]
	return ok
}

func (s *State) handleDirMove(dirPath string) {
	ids := s.findIDsByPathPrefix(dirPath)
	for _, id := range ids {
		slog.Info("removing stale file after directory move", "dir", dirPath, "id", id)
		s.RemoveFile(id)
	}
}

func (s *State) sendEvent(e sseEvent) {
	s.subMu.RLock()
	defer s.subMu.RUnlock()

	for ch := range s.subscribers {
		select {
		case ch <- e:
		default:
			slog.Warn("SSE event dropped (subscriber buffer full)", "event", e.Name)
		}
	}
	if e.Name == eventUpdate {
		s.markDirty()
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
			delete(s.watchedDirs, dir)
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
			if _, err := s.AddFile(path, gp.Group); err != nil {
				slog.Warn("skipping file", "path", path, "error", err)
				return
			}
			slog.Info("auto-added file via glob", "path", path, "pattern", gp.Pattern, "group", gp.Group)
			return
		}
	}
}

type reorderFilesRequest struct {
	Group   string   `json:"group"`
	FileIDs []string `json:"fileIds"`
}

type moveFileRequest struct {
	Group string `json:"group"`
}

type addFileRequest struct {
	Path  string `json:"path"`
	Group string `json:"group"`
}

type uploadFileRequest struct {
	Name    string `json:"name"`
	Content string `json:"content"`
	Group   string `json:"group"`
}

type patternRequest struct {
	Pattern string `json:"pattern"`
	Group   string `json:"group"`
}

// AddPatternResponse is the JSON response for the add-pattern endpoint.
type AddPatternResponse struct {
	Matched int          `json:"matched"`
	Files   []*FileEntry `json:"files,omitempty"`
}

type fileContentResponse struct {
	Content string `json:"content"`
	BaseDir string `json:"baseDir"`
}

type openFileRequest struct {
	FileID string `json:"fileId"`
	Path   string `json:"path"`
}

func NewHandler(state *State) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("POST /_/api/files", handleAddFile(state))
	mux.HandleFunc("POST /_/api/files/upload", handleUploadFile(state))
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
	mux.HandleFunc("GET /_/api/version", handleVersion(state))
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

		entry, err := state.AddFile(absPath, group)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(entry); err != nil {
			slog.Error("failed to encode response", "error", err)
		}
	}
}

func handleUploadFile(state *State) http.HandlerFunc {
	const maxRequestSize = 12 << 20 // 12MB (headroom for JSON envelope)
	const maxContentSize = 10 << 20 // 10MB
	return func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, maxRequestSize)
		var req uploadFileRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			var maxBytesErr *http.MaxBytesError
			if errors.As(err, &maxBytesErr) {
				http.Error(w, "file too large (max 10MB)", http.StatusRequestEntityTooLarge)
				return
			}
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		if len(req.Content) > maxContentSize {
			http.Error(w, "file too large (max 10MB)", http.StatusRequestEntityTooLarge)
			return
		}

		if req.Name == "" {
			http.Error(w, "missing file name", http.StatusBadRequest)
			return
		}

		group, err := ResolveGroupName(req.Group)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		entry := state.AddUploadedFile(req.Name, req.Content, group)
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(entry); err != nil {
			slog.Error("failed to encode response", "error", err)
		}
	}
}

func handleRemoveFile(state *State) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if state.noDelete {
			http.Error(w, "delete disabled", http.StatusForbidden)
			return
		}
		id := r.PathValue("id")
		if id == "" {
			http.Error(w, "missing file id", http.StatusBadRequest)
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
		if state.noFileMove {
			http.Error(w, "move disabled", http.StatusForbidden)
			return
		}
		id := r.PathValue("id")
		if id == "" {
			http.Error(w, "missing file id", http.StatusBadRequest)
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
		id := r.PathValue("id")
		if id == "" {
			http.Error(w, "missing file id", http.StatusBadRequest)
			return
		}

		entry := state.FindFile(id)
		if entry == nil {
			http.Error(w, "file not found", http.StatusNotFound)
			return
		}

		var resp fileContentResponse
		if entry.Uploaded {
			resp = fileContentResponse{
				Content: entry.content,
				BaseDir: "",
			}
		} else {
			content, err := os.ReadFile(entry.Path) //nolint:gosec // Path is server-managed, not user-supplied
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			resp = fileContentResponse{
				Content: string(content),
				BaseDir: filepath.Dir(entry.Path),
			}
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			slog.Error("failed to encode response", "error", err)
		}
	}
}

func handleFileRaw(state *State) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if id == "" {
			http.Error(w, "missing file id", http.StatusBadRequest)
			return
		}

		entry := state.FindFile(id)
		if entry == nil {
			http.Error(w, "file not found", http.StatusNotFound)
			return
		}

		if entry.Uploaded {
			http.Error(w, "raw assets not available for uploaded files", http.StatusNotFound)
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

		if entry.Uploaded {
			http.Error(w, "relative links not available for uploaded files", http.StatusBadRequest)
			return
		}

		absPath := filepath.Join(filepath.Dir(entry.Path), req.Path)
		absPath = filepath.Clean(absPath)

		if _, err := os.Stat(absPath); err != nil {
			if os.IsNotExist(err) {
				http.Error(w, fmt.Sprintf("file not found: %s", absPath), http.StatusNotFound)
			} else {
				http.Error(w, err.Error(), http.StatusBadRequest)
			}
			return
		}

		groupName := state.FindGroupForFile(req.FileID)
		if groupName == "" {
			groupName = DefaultGroup
		}

		newEntry, err := state.AddFile(absPath, groupName)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
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

		entries, err := state.AddPattern(req.Pattern, group)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(AddPatternResponse{Matched: len(entries), Files: entries}); err != nil {
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
		if state.noRestart {
			http.Error(w, "restart disabled", http.StatusForbidden)
			return
		}
		restoreFile, err := state.ExportState()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusAccepted)

		// Send restart signal after response is written
		select {
		case state.restartCh <- restoreFile:
		default:
			os.Remove(restoreFile) //nolint:errcheck
		}
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

func handleVersion(state *State) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		resp := map[string]any{
			"version":   version.Version,
			"revision":  version.Revision,
			"noRestart": state.noRestart,
			"noDelete":  state.noDelete,
			"noFileMove":    state.noFileMove,
			"shareable": state.shareable,
		}
		if err := json.NewEncoder(w).Encode(resp); err != nil {
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

		// Send server identity on connection
		fmt.Fprintf(w, "event: started\ndata: {\"pid\":%d}\n\n", os.Getpid())
		flusher.Flush()

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
