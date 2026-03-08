package server

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/k1LoW/donegroup"
)

func newTestState(t *testing.T) *State {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	s := &State{
		groups:      make(map[string]*Group),
		subscribers: make(map[chan sseEvent]struct{}),
		restartCh:   make(chan string, 1),
		shutdownCh:  make(chan struct{}, 1),
		watchedDirs: make(map[string]int),
	}
	_ = ctx
	return s
}

func TestReorderFiles(t *testing.T) {
	idA := FileID("/a.md")
	idB := FileID("/b.md")
	idC := FileID("/c.md")

	t.Run("reorders files successfully", func(t *testing.T) {
		s := newTestState(t)
		s.groups[DefaultGroup] = &Group{
			Name: DefaultGroup,
			Files: []*FileEntry{
				{ID: idA, Name: "a.md", Path: "/a.md"},
				{ID: idB, Name: "b.md", Path: "/b.md"},
				{ID: idC, Name: "c.md", Path: "/c.md"},
			},
		}

		ok := s.ReorderFiles(DefaultGroup, []string{idC, idA, idB})
		if !ok {
			t.Fatal("ReorderFiles returned false, want true")
		}

		files := s.groups[DefaultGroup].Files
		if files[0].ID != idC || files[1].ID != idA || files[2].ID != idB {
			t.Errorf("got order [%s, %s, %s], want [%s, %s, %s]", files[0].ID, files[1].ID, files[2].ID, idC, idA, idB)
		}
	})

	t.Run("returns false for unknown group", func(t *testing.T) {
		s := newTestState(t)
		ok := s.ReorderFiles("nonexistent", []string{idA})
		if ok {
			t.Fatal("ReorderFiles returned true for unknown group")
		}
	})

	t.Run("returns false for mismatched count", func(t *testing.T) {
		s := newTestState(t)
		s.groups[DefaultGroup] = &Group{
			Name: DefaultGroup,
			Files: []*FileEntry{
				{ID: idA, Name: "a.md", Path: "/a.md"},
				{ID: idB, Name: "b.md", Path: "/b.md"},
			},
		}

		ok := s.ReorderFiles(DefaultGroup, []string{idA})
		if ok {
			t.Fatal("ReorderFiles returned true for mismatched count")
		}
	})

	t.Run("returns false for unknown file ID", func(t *testing.T) {
		s := newTestState(t)
		s.groups[DefaultGroup] = &Group{
			Name: DefaultGroup,
			Files: []*FileEntry{
				{ID: idA, Name: "a.md", Path: "/a.md"},
				{ID: idB, Name: "b.md", Path: "/b.md"},
			},
		}

		ok := s.ReorderFiles(DefaultGroup, []string{idA, "nonexist"})
		if ok {
			t.Fatal("ReorderFiles returned true for unknown file ID")
		}
	})
}

func TestMoveFile(t *testing.T) {
	idA := FileID("/a.md")
	idB := FileID("/b.md")
	idC := FileID("/c.md")

	t.Run("moves file to existing group", func(t *testing.T) {
		s := newTestState(t)
		s.groups["src"] = &Group{
			Name:  "src",
			Files: []*FileEntry{{ID: idA, Name: "a.md", Path: "/a.md"}, {ID: idB, Name: "b.md", Path: "/b.md"}},
		}
		s.groups["dst"] = &Group{
			Name:  "dst",
			Files: []*FileEntry{{ID: idC, Name: "c.md", Path: "/c.md"}},
		}

		if err := s.MoveFile(idA, "dst"); err != nil {
			t.Fatalf("MoveFile returned error: %v", err)
		}

		if len(s.groups["src"].Files) != 1 || s.groups["src"].Files[0].ID != idB {
			t.Error("source group should have only file b")
		}
		if len(s.groups["dst"].Files) != 2 || s.groups["dst"].Files[1].ID != idA {
			t.Error("target group should have file a appended")
		}
	})

	t.Run("auto-creates target group", func(t *testing.T) {
		s := newTestState(t)
		s.groups["src"] = &Group{
			Name:  "src",
			Files: []*FileEntry{{ID: idA, Name: "a.md", Path: "/a.md"}, {ID: idB, Name: "b.md", Path: "/b.md"}},
		}

		if err := s.MoveFile(idA, "newgroup"); err != nil {
			t.Fatalf("MoveFile returned error: %v", err)
		}

		if _, ok := s.groups["newgroup"]; !ok {
			t.Fatal("target group should have been created")
		}
		if s.groups["newgroup"].Files[0].ID != idA {
			t.Error("target group should contain file a")
		}
	})

	t.Run("deletes empty source group", func(t *testing.T) {
		s := newTestState(t)
		s.groups["src"] = &Group{
			Name:  "src",
			Files: []*FileEntry{{ID: idA, Name: "a.md", Path: "/a.md"}},
		}
		s.groups["dst"] = &Group{
			Name:  "dst",
			Files: []*FileEntry{{ID: idB, Name: "b.md", Path: "/b.md"}},
		}

		if err := s.MoveFile(idA, "dst"); err != nil {
			t.Fatalf("MoveFile returned error: %v", err)
		}

		if _, ok := s.groups["src"]; ok {
			t.Error("empty source group should have been deleted")
		}
	})

	t.Run("returns error for duplicate path", func(t *testing.T) {
		s := newTestState(t)
		s.groups["src"] = &Group{
			Name:  "src",
			Files: []*FileEntry{{ID: idA, Name: "a.md", Path: "/a.md"}},
		}
		s.groups["dst"] = &Group{
			Name:  "dst",
			Files: []*FileEntry{{ID: idA, Name: "a.md", Path: "/a.md"}},
		}

		err := s.MoveFile(idA, "dst")
		if err == nil {
			t.Fatal("MoveFile should return error for duplicate path")
		}
	})

	t.Run("returns error for same group", func(t *testing.T) {
		s := newTestState(t)
		s.groups["src"] = &Group{
			Name:  "src",
			Files: []*FileEntry{{ID: idA, Name: "a.md", Path: "/a.md"}},
		}

		err := s.MoveFile(idA, "src")
		if err == nil {
			t.Fatal("MoveFile should return error for same group")
		}
	})

	t.Run("returns error for unknown file", func(t *testing.T) {
		s := newTestState(t)
		err := s.MoveFile("nonexist", "dst")
		if err == nil {
			t.Fatal("MoveFile should return error for unknown file")
		}
	})
}

func TestHandleMoveFile(t *testing.T) {
	idA := FileID("/a.md")
	idB := FileID("/b.md")

	t.Run("moves file via HTTP", func(t *testing.T) {
		s := newTestState(t)
		s.groups["src"] = &Group{
			Name:  "src",
			Files: []*FileEntry{{ID: idA, Name: "a.md", Path: "/a.md"}},
		}
		s.groups["dst"] = &Group{
			Name:  "dst",
			Files: []*FileEntry{{ID: idB, Name: "b.md", Path: "/b.md"}},
		}

		handler := NewHandler(s)
		body, err := json.Marshal(moveFileRequest{Group: "dst"})
		if err != nil {
			t.Fatal(err)
		}
		req := httptest.NewRequest("PUT", fmt.Sprintf("/_/api/files/%s/group", idA), bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusNoContent {
			t.Fatalf("got status %d, want %d", rec.Code, http.StatusNoContent)
		}
		if len(s.groups["dst"].Files) != 2 {
			t.Error("target group should have 2 files")
		}
	})

	t.Run("returns 409 for duplicate path", func(t *testing.T) {
		s := newTestState(t)
		s.groups["src"] = &Group{
			Name:  "src",
			Files: []*FileEntry{{ID: idA, Name: "a.md", Path: "/a.md"}},
		}
		s.groups["dst"] = &Group{
			Name:  "dst",
			Files: []*FileEntry{{ID: idA, Name: "a.md", Path: "/a.md"}},
		}

		handler := NewHandler(s)
		body, err := json.Marshal(moveFileRequest{Group: "dst"})
		if err != nil {
			t.Fatal(err)
		}
		req := httptest.NewRequest("PUT", fmt.Sprintf("/_/api/files/%s/group", idA), bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusConflict {
			t.Fatalf("got status %d, want %d", rec.Code, http.StatusConflict)
		}
	})
}

func TestHandleShutdown(t *testing.T) {
	t.Run("returns 202 and signals shutdownCh", func(t *testing.T) {
		s := newTestState(t)
		handler := NewHandler(s)
		req := httptest.NewRequest("POST", "/_/api/shutdown", nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusAccepted {
			t.Fatalf("got status %d, want %d", rec.Code, http.StatusAccepted)
		}

		select {
		case <-s.ShutdownCh():
		default:
			t.Fatal("shutdownCh should have received a signal")
		}
	})

	t.Run("does not block on duplicate signal", func(t *testing.T) {
		s := newTestState(t)
		handler := NewHandler(s)

		for i := 0; i < 2; i++ {
			req := httptest.NewRequest("POST", "/_/api/shutdown", nil)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			if rec.Code != http.StatusAccepted {
				t.Fatalf("call %d: got status %d, want %d", i+1, rec.Code, http.StatusAccepted)
			}
		}
	})
}

func TestHandleRestart(t *testing.T) {
	idA := FileID("/a.md")

	t.Run("returns 202 and signals restartCh", func(t *testing.T) {
		s := newTestState(t)
		s.groups[DefaultGroup] = &Group{
			Name:  DefaultGroup,
			Files: []*FileEntry{{ID: idA, Name: "a.md", Path: "/a.md"}},
		}
		handler := NewHandler(s)
		req := httptest.NewRequest("POST", "/_/api/restart", nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusAccepted {
			t.Fatalf("got status %d, want %d", rec.Code, http.StatusAccepted)
		}

		select {
		case restoreFile := <-s.RestartCh():
			if restoreFile == "" {
				t.Fatal("restartCh should have received a non-empty restore file path")
			}
			t.Cleanup(func() {
				_ = os.Remove(restoreFile) //nostyle:handlerrors
			})
		default:
			t.Fatal("restartCh should have received a signal")
		}
	})

	t.Run("does not block on duplicate signal", func(t *testing.T) {
		s := newTestState(t)
		s.groups[DefaultGroup] = &Group{
			Name:  DefaultGroup,
			Files: []*FileEntry{{ID: idA, Name: "a.md", Path: "/a.md"}},
		}
		handler := NewHandler(s)

		for i := 0; i < 2; i++ {
			req := httptest.NewRequest("POST", "/_/api/restart", nil)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			if rec.Code != http.StatusAccepted {
				t.Fatalf("call %d: got status %d, want %d", i+1, rec.Code, http.StatusAccepted)
			}
		}

		// Drain restartCh and clean up the restore file from the first request
		select {
		case restoreFile := <-s.RestartCh():
			t.Cleanup(func() {
				_ = os.Remove(restoreFile) //nostyle:handlerrors
			})
		default:
		}
	})
}

func TestHandleReorderFiles(t *testing.T) {
	idA := FileID("/a.md")
	idB := FileID("/b.md")

	t.Run("reorders files via HTTP", func(t *testing.T) {
		s := newTestState(t)
		s.groups["docs"] = &Group{
			Name: "docs",
			Files: []*FileEntry{
				{ID: idA, Name: "a.md", Path: "/a.md"},
				{ID: idB, Name: "b.md", Path: "/b.md"},
			},
		}

		handler := NewHandler(s)
		body, err := json.Marshal(reorderFilesRequest{Group: "docs", FileIDs: []string{idB, idA}})
		if err != nil {
			t.Fatal(err)
		}
		req := httptest.NewRequest("PUT", "/_/api/reorder", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusNoContent {
			t.Fatalf("got status %d, want %d", rec.Code, http.StatusNoContent)
		}

		files := s.groups["docs"].Files
		if files[0].ID != idB || files[1].ID != idA {
			t.Errorf("got order [%s, %s], want [%s, %s]", files[0].ID, files[1].ID, idB, idA)
		}
	})

	t.Run("reorders files in group with slashes", func(t *testing.T) {
		s := newTestState(t)
		s.groups["api/docs"] = &Group{
			Name: "api/docs",
			Files: []*FileEntry{
				{ID: idA, Name: "a.md", Path: "/a.md"},
				{ID: idB, Name: "b.md", Path: "/b.md"},
			},
		}

		handler := NewHandler(s)
		body, err := json.Marshal(reorderFilesRequest{Group: "api/docs", FileIDs: []string{idB, idA}})
		if err != nil {
			t.Fatal(err)
		}
		req := httptest.NewRequest("PUT", "/_/api/reorder", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusNoContent {
			t.Fatalf("got status %d, want %d", rec.Code, http.StatusNoContent)
		}

		files := s.groups["api/docs"].Files
		if files[0].ID != idB || files[1].ID != idA {
			t.Errorf("got order [%s, %s], want [%s, %s]", files[0].ID, files[1].ID, idB, idA)
		}
	})

	t.Run("returns 400 for invalid group", func(t *testing.T) {
		s := newTestState(t)
		handler := NewHandler(s)
		body, err := json.Marshal(reorderFilesRequest{Group: "nonexistent", FileIDs: []string{idA}})
		if err != nil {
			t.Fatal(err)
		}
		req := httptest.NewRequest("PUT", "/_/api/reorder", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("got status %d, want %d", rec.Code, http.StatusBadRequest)
		}
	})

	t.Run("returns 400 for invalid JSON", func(t *testing.T) {
		s := newTestState(t)
		s.groups[DefaultGroup] = &Group{Name: DefaultGroup, Files: []*FileEntry{}}
		handler := NewHandler(s)
		req := httptest.NewRequest("PUT", "/_/api/reorder", bytes.NewReader([]byte("invalid")))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("got status %d, want %d", rec.Code, http.StatusBadRequest)
		}
	})
}

func TestAddPattern_InitialExpansion(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "a.md"), []byte("# A"), 0o600) //nolint:errcheck
	os.WriteFile(filepath.Join(dir, "b.md"), []byte("# B"), 0o600) //nolint:errcheck
	os.WriteFile(filepath.Join(dir, "c.txt"), []byte("text"), 0o600) //nolint:errcheck

	s := newTestState(t)
	pattern := filepath.Join(dir, "*.md")
	matched, err := s.AddPattern(pattern, DefaultGroup)
	if err != nil {
		t.Fatalf("AddPattern returned error: %v", err)
	}
	if matched != 2 {
		t.Fatalf("got matched=%d, want 2", matched)
	}

	groups := s.Groups()
	if len(groups) != 1 {
		t.Fatalf("got %d groups, want 1", len(groups))
	}
	if len(groups[0].Files) != 2 {
		t.Fatalf("got %d files, want 2", len(groups[0].Files))
	}
}

func TestAddPattern_Duplicate(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "a.md"), []byte("# A"), 0o600) //nolint:errcheck

	s := newTestState(t)
	pattern := filepath.Join(dir, "*.md")
	_, err := s.AddPattern(pattern, DefaultGroup)
	if err != nil {
		t.Fatalf("AddPattern returned error: %v", err)
	}

	matched, err := s.AddPattern(pattern, DefaultGroup)
	if err != nil {
		t.Fatalf("duplicate AddPattern returned error: %v", err)
	}
	if matched != 0 {
		t.Fatalf("duplicate AddPattern returned matched=%d, want 0", matched)
	}

	patterns := s.Patterns()
	if len(patterns) != 1 {
		t.Fatalf("got %d patterns, want 1", len(patterns))
	}
}

func TestAddPattern_InvalidBaseDir(t *testing.T) {
	s := newTestState(t)
	_, err := s.AddPattern("/nonexistent/dir/*.md", DefaultGroup)
	if err == nil {
		t.Fatal("AddPattern should return error for nonexistent base dir")
	}
}

func TestExportState_WithPatterns(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "a.md"), []byte("# A"), 0o600) //nolint:errcheck

	s := newTestState(t)
	pattern := filepath.Join(dir, "*.md")
	_, err := s.AddPattern(pattern, DefaultGroup)
	if err != nil {
		t.Fatalf("AddPattern returned error: %v", err)
	}

	restoreFile, err := s.ExportState()
	if err != nil {
		t.Fatalf("ExportState returned error: %v", err)
	}
	defer os.Remove(restoreFile)

	data, err := os.ReadFile(restoreFile)
	if err != nil {
		t.Fatalf("failed to read restore file: %v", err)
	}

	var rd RestoreData
	if err := json.Unmarshal(data, &rd); err != nil {
		t.Fatalf("failed to unmarshal restore data: %v", err)
	}

	if len(rd.Patterns) == 0 {
		t.Fatal("RestoreData.Patterns should not be empty")
	}
	pats, ok := rd.Patterns[DefaultGroup]
	if !ok || len(pats) != 1 || pats[0] != pattern {
		t.Fatalf("got patterns=%v, want [%s]", pats, pattern)
	}
}

func TestRemovePattern(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "a.md"), []byte("# A"), 0o600) //nolint:errcheck

	t.Run("removes existing pattern", func(t *testing.T) {
		s := newTestState(t)
		pattern := filepath.Join(dir, "*.md")
		_, err := s.AddPattern(pattern, DefaultGroup)
		if err != nil {
			t.Fatalf("AddPattern returned error: %v", err)
		}

		ok := s.RemovePattern(pattern, DefaultGroup)
		if !ok {
			t.Fatal("RemovePattern returned false, want true")
		}

		patterns := s.Patterns()
		if len(patterns) != 0 {
			t.Fatalf("got %d patterns, want 0", len(patterns))
		}
	})

	t.Run("returns false for non-existent pattern", func(t *testing.T) {
		s := newTestState(t)
		ok := s.RemovePattern("/nonexistent/*.md", DefaultGroup)
		if ok {
			t.Fatal("RemovePattern returned true for non-existent pattern")
		}
	})

	t.Run("returns false for wrong group", func(t *testing.T) {
		s := newTestState(t)
		pattern := filepath.Join(dir, "*.md")
		_, err := s.AddPattern(pattern, DefaultGroup)
		if err != nil {
			t.Fatalf("AddPattern returned error: %v", err)
		}

		ok := s.RemovePattern(pattern, "other")
		if ok {
			t.Fatal("RemovePattern returned true for wrong group")
		}
	})

	t.Run("files remain after pattern removal", func(t *testing.T) {
		s := newTestState(t)
		pattern := filepath.Join(dir, "*.md")
		_, err := s.AddPattern(pattern, DefaultGroup)
		if err != nil {
			t.Fatalf("AddPattern returned error: %v", err)
		}

		groups := s.Groups()
		fileCount := len(groups[0].Files)
		if fileCount == 0 {
			t.Fatal("should have files before removal")
		}

		s.RemovePattern(pattern, DefaultGroup)

		groups = s.Groups()
		if len(groups) != 1 || len(groups[0].Files) != fileCount {
			t.Fatal("files should remain after pattern removal")
		}
	})
}

func TestGroupPersistsWithPatternsAfterFileRemoval(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "a.md"), []byte("# A"), 0o600) //nolint:errcheck

	s := newTestState(t)
	pattern := filepath.Join(dir, "*.md")
	_, err := s.AddPattern(pattern, DefaultGroup)
	if err != nil {
		t.Fatalf("AddPattern returned error: %v", err)
	}

	groups := s.Groups()
	if len(groups) != 1 || len(groups[0].Files) != 1 {
		t.Fatalf("expected 1 group with 1 file, got %d groups", len(groups))
	}

	// Remove the only file — group should persist because pattern remains.
	fileID := groups[0].Files[0].ID
	if !s.RemoveFile(fileID) {
		t.Fatal("RemoveFile returned false")
	}

	groups = s.Groups()
	if len(groups) != 1 {
		t.Fatal("group should persist when patterns remain")
	}
	if len(groups[0].Files) != 0 {
		t.Fatalf("expected 0 files, got %d", len(groups[0].Files))
	}
	if len(s.PatternsForGroup(DefaultGroup)) != 1 {
		t.Fatal("pattern should still be registered")
	}

	// Now remove the pattern — group should be deleted.
	s.RemovePattern(pattern, DefaultGroup)
	groups = s.Groups()
	if len(groups) != 0 {
		t.Fatal("group should be deleted when no files and no patterns remain")
	}
}

func TestRemoveDirWatch(t *testing.T) {
	s := newTestState(t)

	t.Run("decrements ref count", func(t *testing.T) {
		s.mu.Lock()
		s.watchedDirs["/some/dir"] = 2
		s.mu.Unlock()

		s.removeDirWatch("/some/dir")

		s.mu.RLock()
		count := s.watchedDirs["/some/dir"]
		s.mu.RUnlock()

		if count != 1 {
			t.Fatalf("watchedDirs count=%d, want 1", count)
		}
	})

	t.Run("deletes entry at zero", func(t *testing.T) {
		s.mu.Lock()
		s.watchedDirs["/another/dir"] = 1
		s.mu.Unlock()

		s.removeDirWatch("/another/dir")

		s.mu.RLock()
		_, exists := s.watchedDirs["/another/dir"]
		s.mu.RUnlock()

		if exists {
			t.Fatal("watchedDirs entry should be deleted at zero")
		}
	})

	t.Run("no-op for unknown dir", func(t *testing.T) {
		s.removeDirWatch("/unknown/dir")
		// Should not panic
	})
}

func TestHandleRemovePattern(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "a.md"), []byte("# A"), 0o600) //nolint:errcheck

	t.Run("returns 204 on success", func(t *testing.T) {
		s := newTestState(t)
		pattern := filepath.Join(dir, "*.md")
		_, err := s.AddPattern(pattern, DefaultGroup)
		if err != nil {
			t.Fatal(err)
		}

		handler := NewHandler(s)
		body, err := json.Marshal(patternRequest{Pattern: pattern, Group: DefaultGroup})
		if err != nil {
			t.Fatal(err)
		}
		req := httptest.NewRequest("DELETE", "/_/api/patterns", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusNoContent {
			t.Fatalf("got status %d, want %d: %s", rec.Code, http.StatusNoContent, rec.Body.String())
		}
	})

	t.Run("returns 404 for unknown pattern", func(t *testing.T) {
		s := newTestState(t)
		handler := NewHandler(s)
		body, err := json.Marshal(patternRequest{Pattern: "/nonexistent/*.md", Group: DefaultGroup})
		if err != nil {
			t.Fatal(err)
		}
		req := httptest.NewRequest("DELETE", "/_/api/patterns", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Fatalf("got status %d, want %d", rec.Code, http.StatusNotFound)
		}
	})
}

func TestHandleStatus_PatternsInGroups(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "a.md"), []byte("# A"), 0o600) //nolint:errcheck

	s := newTestState(t)
	pattern := filepath.Join(dir, "*.md")
	_, err := s.AddPattern(pattern, DefaultGroup)
	if err != nil {
		t.Fatal(err)
	}

	handler := NewHandler(s)
	req := httptest.NewRequest("GET", "/_/api/status", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("got status %d, want %d", rec.Code, http.StatusOK)
	}

	var resp struct {
		Groups []struct {
			Name     string   `json:"name"`
			Patterns []string `json:"patterns,omitempty"`
		} `json:"groups"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(resp.Groups) != 1 {
		t.Fatalf("got %d groups, want 1", len(resp.Groups))
	}
	if len(resp.Groups[0].Patterns) != 1 || resp.Groups[0].Patterns[0] != pattern {
		t.Fatalf("got patterns=%v, want [%s]", resp.Groups[0].Patterns, pattern)
	}
}

func TestHandleAddPattern(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "a.md"), []byte("# A"), 0o600) //nolint:errcheck

	s := newTestState(t)
	handler := NewHandler(s)

	pattern := filepath.Join(dir, "*.md")
	body, err := json.Marshal(patternRequest{Pattern: pattern, Group: DefaultGroup})
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest("POST", "/_/api/patterns", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("got status %d, want %d: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var resp addPatternResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Matched != 1 {
		t.Fatalf("got matched=%d, want 1", resp.Matched)
	}
}

func TestHandleSSE_StartedEvent(t *testing.T) {
	s := newTestState(t)
	handler := handleSSE(s)

	pr, pw := io.Pipe()
	defer pr.Close()

	rec := &flushRecorder{pw: pw}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second) //nolint:gosec // cancel is called in t.Cleanup

	req := httptest.NewRequest(http.MethodGet, "/_/events", nil).WithContext(ctx)

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		handler.ServeHTTP(rec, req)
		pw.Close()
	}()
	t.Cleanup(func() {
		cancel()
		wg.Wait()
	})

	scanner := bufio.NewScanner(pr)
	var eventLine, dataLine string
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "event: ") {
			eventLine = line
		} else if strings.HasPrefix(line, "data: ") {
			dataLine = line
			break
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("scanner error while reading SSE stream: %v", err)
	}

	if eventLine != "event: started" {
		t.Fatalf("got event line %q, want %q", eventLine, "event: started")
	}

	wantData := fmt.Sprintf(`data: {"pid":%d}`, os.Getpid())
	if dataLine != wantData {
		t.Fatalf("got data line %q, want %q", dataLine, wantData)
	}
}

// flushRecorder implements http.ResponseWriter and http.Flusher,
// writing output to an io.Writer for streaming tests.
type flushRecorder struct {
	pw      io.Writer
	headers http.Header
}

func (f *flushRecorder) Header() http.Header {
	if f.headers == nil {
		f.headers = make(http.Header)
	}
	return f.headers
}

func (f *flushRecorder) Write(b []byte) (int, error) {
	return f.pw.Write(b)
}

func (f *flushRecorder) WriteHeader(_ int) {}

func (f *flushRecorder) Flush() {}

func TestEnableBackup_TriggersOnStateChange(t *testing.T) {
	ctx, cancel := donegroup.WithCancel(context.Background())
	defer cancel()

	s := NewState(ctx)

	var mu sync.Mutex
	var saved []RestoreData
	s.EnableBackup(ctx, func(data RestoreData) {
		mu.Lock()
		saved = append(saved, data)
		mu.Unlock()
	})

	s.AddFile("/tmp/test-a.md", DefaultGroup)

	// Wait for debounce (1s) + margin
	time.Sleep(1500 * time.Millisecond)

	mu.Lock()
	count := len(saved)
	mu.Unlock()

	if count == 0 {
		t.Fatal("backup callback should have been called after state change")
	}

	mu.Lock()
	last := saved[count-1]
	mu.Unlock()

	paths, ok := last.Groups[DefaultGroup]
	if !ok {
		t.Fatal("saved data should contain default group")
	}
	if len(paths) != 1 || paths[0] != "/tmp/test-a.md" {
		t.Fatalf("got paths=%v, want [/tmp/test-a.md]", paths)
	}
}

func TestEnableBackup_FinalSaveOnCancel(t *testing.T) {
	ctx, cancel := donegroup.WithCancel(context.Background())

	s := NewState(ctx)

	var mu sync.Mutex
	var saved []RestoreData
	s.EnableBackup(ctx, func(data RestoreData) {
		mu.Lock()
		saved = append(saved, data)
		mu.Unlock()
	})

	s.AddFile("/tmp/test-b.md", DefaultGroup)

	// Cancel immediately without waiting for debounce
	cancel()

	// Wait for the backupLoop goroutine to finish its final save
	<-s.backupDone

	mu.Lock()
	count := len(saved)
	mu.Unlock()

	if count == 0 {
		t.Fatal("backup callback should have been called on context cancellation")
	}

	mu.Lock()
	last := saved[count-1]
	mu.Unlock()

	paths, ok := last.Groups[DefaultGroup]
	if !ok {
		t.Fatal("final save should contain default group")
	}
	if len(paths) != 1 || paths[0] != "/tmp/test-b.md" {
		t.Fatalf("got paths=%v, want [/tmp/test-b.md]", paths)
	}
}

func TestEnableBackup_ReflectsLatestState(t *testing.T) {
	ctx, cancel := donegroup.WithCancel(context.Background())
	defer cancel()

	s := NewState(ctx)

	var mu sync.Mutex
	var saved []RestoreData
	s.EnableBackup(ctx, func(data RestoreData) {
		mu.Lock()
		saved = append(saved, data)
		mu.Unlock()
	})

	s.AddFile("/tmp/test-c.md", DefaultGroup)
	s.AddFile("/tmp/test-d.md", DefaultGroup)

	// Wait for debounce
	time.Sleep(1500 * time.Millisecond)

	mu.Lock()
	count := len(saved)
	if count == 0 {
		mu.Unlock()
		t.Fatal("backup callback should have been called after state change")
	}
	last := saved[count-1]
	mu.Unlock()

	paths := last.Groups[DefaultGroup]
	if len(paths) != 2 {
		t.Fatalf("got %d paths, want 2", len(paths))
	}
}

func TestFileID(t *testing.T) {
	id := FileID("/tmp/test.md")
	if len(id) != 8 {
		t.Fatalf("FileID should return 8-char string, got %q (len=%d)", id, len(id))
	}

	// Same path should always produce the same ID
	if FileID("/tmp/test.md") != id {
		t.Fatal("FileID should be deterministic")
	}

	// Different paths should produce different IDs
	if FileID("/tmp/other.md") == id {
		t.Fatal("FileID should differ for different paths")
	}
}
