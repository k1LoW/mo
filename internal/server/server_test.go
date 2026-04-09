package server

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
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

var (
	testIDa = FileID("/a.md")
	testIDb = FileID("/b.md")
	testIDc = FileID("/c.md")
)

func newTestState(t *testing.T) *State {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	s := &State{
		groups:             make(map[string]*Group),
		subscribers:        make(map[chan sseEvent]struct{}),
		restartCh:          make(chan string, 1),
		shutdownCh:         make(chan struct{}, 1),
		watchedDirs:        make(map[string]int),
		fileChangeDebounce: defaultFileChangeDebounce,
		fileChangeTimers:   make(map[string]*time.Timer),
	}
	_ = ctx
	return s
}

func TestReorderFiles(t *testing.T) {
	idA := testIDa
	idB := testIDb
	idC := testIDc

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
	idA := testIDa
	idB := testIDb
	idC := testIDc

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

		if err := s.MoveFile(idA, "src", "dst"); err != nil {
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

		if err := s.MoveFile(idA, "src", "newgroup"); err != nil {
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

		if err := s.MoveFile(idA, "src", "dst"); err != nil {
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

		err := s.MoveFile(idA, "src", "dst")
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

		err := s.MoveFile(idA, "src", "src")
		if err == nil {
			t.Fatal("MoveFile should return error for same group")
		}
	})

	t.Run("returns error for unknown file", func(t *testing.T) {
		s := newTestState(t)
		err := s.MoveFile("nonexist", "src", "dst")
		if err == nil {
			t.Fatal("MoveFile should return error for unknown file")
		}
	})
}

func TestHandleMoveFile(t *testing.T) {
	idA := testIDa
	idB := testIDb

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
		req := httptest.NewRequest("PUT", fmt.Sprintf("/_/api/groups/src/files/%s/group", idA), bytes.NewReader(body))
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
		req := httptest.NewRequest("PUT", fmt.Sprintf("/_/api/groups/src/files/%s/group", idA), bytes.NewReader(body))
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

		for i := range 2 {
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
	idA := testIDa

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

		for i := range 2 {
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
	idA := testIDa
	idB := testIDb

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
		body, err := json.Marshal(reorderFilesRequest{FileIDs: []string{idB, idA}})
		if err != nil {
			t.Fatal(err)
		}
		req := httptest.NewRequest("PUT", "/_/api/groups/docs/reorder", bytes.NewReader(body))
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
		body, err := json.Marshal(reorderFilesRequest{FileIDs: []string{idB, idA}})
		if err != nil {
			t.Fatal(err)
		}
		req := httptest.NewRequest("PUT", "/_/api/groups/api%2Fdocs/reorder", bytes.NewReader(body))
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
		body, err := json.Marshal(reorderFilesRequest{FileIDs: []string{idA}})
		if err != nil {
			t.Fatal(err)
		}
		req := httptest.NewRequest("PUT", "/_/api/groups/nonexistent/reorder", bytes.NewReader(body))
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
		req := httptest.NewRequest("PUT", "/_/api/groups/default/reorder", bytes.NewReader([]byte("invalid")))
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
	os.WriteFile(filepath.Join(dir, "a.md"), []byte("# A"), 0o600)   //nolint:errcheck
	os.WriteFile(filepath.Join(dir, "b.md"), []byte("# B"), 0o600)   //nolint:errcheck
	os.WriteFile(filepath.Join(dir, "c.txt"), []byte("text"), 0o600) //nolint:errcheck

	s := newTestState(t)
	pattern := filepath.Join(dir, "*.md")
	entries, err := s.AddPattern(pattern, DefaultGroup)
	if err != nil {
		t.Fatalf("AddPattern returned error: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("got matched=%d, want 2", len(entries))
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

	entries, err := s.AddPattern(pattern, DefaultGroup)
	if err != nil {
		t.Fatalf("duplicate AddPattern returned error: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("duplicate AddPattern returned matched=%d, want 0", len(entries))
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
	if !s.RemoveFile(fileID, groups[0].Name) {
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

	var resp AddPatternResponse
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
	wg.Go(func() {
		handler.ServeHTTP(rec, req)
		pw.Close()
	})
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

func TestScheduleFileChanged_DebouncesDuplicateEvents(t *testing.T) {
	s := newTestState(t)
	s.fileChangeDebounce = 20 * time.Millisecond

	tmpFile := filepath.Join(t.TempDir(), "debounce.md")
	s.groups[DefaultGroup] = &Group{
		Name:  DefaultGroup,
		Files: []*FileEntry{{ID: FileID(tmpFile), Name: "debounce.md", Path: tmpFile}},
	}

	ch := s.Subscribe()
	defer s.Unsubscribe(ch)

	s.scheduleFileChanged(tmpFile)
	s.scheduleFileChanged(tmpFile)
	s.scheduleFileChanged(tmpFile)

	var changedEvents int
	deadline := time.After(300 * time.Millisecond)
	for changedEvents < 1 {
		select {
		case e := <-ch:
			if e.Name == eventFileChanged {
				changedEvents++
			}
		case <-deadline:
			t.Fatal("timed out waiting for debounced file-changed event")
		}
	}

	select {
	case e := <-ch:
		if e.Name == eventFileChanged {
			t.Fatal("received duplicate file-changed event after debounce window")
		}
	case <-time.After(80 * time.Millisecond):
	}
}

func TestSendEvent_ConcurrentWithUnsubscribeDoesNotPanic(t *testing.T) {
	s := newTestState(t)
	ch := s.Subscribe()

	done := make(chan struct{})
	panicCh := make(chan any, 1)

	go func() {
		defer close(done)
		defer func() {
			if r := recover(); r != nil {
				panicCh <- r
			}
		}()
		for range 100 {
			s.sendEvent(sseEvent{Name: eventFileChanged, Data: "{}"})
		}
	}()

	for range 100 {
		s.Unsubscribe(ch)
		ch = s.Subscribe()
	}
	s.Unsubscribe(ch)

	<-done

	select {
	case r := <-panicCh:
		t.Fatalf("sendEvent panicked during concurrent unsubscribe: %v", r)
	default:
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

	tmpFile := filepath.Join(t.TempDir(), "test-a.md")
	os.WriteFile(tmpFile, []byte("# A"), 0o600) //nolint:errcheck

	var mu sync.Mutex
	var saved []RestoreData
	s.EnableBackup(ctx, func(data RestoreData) {
		mu.Lock()
		saved = append(saved, data)
		mu.Unlock()
	})

	if _, err := s.AddFile(tmpFile, DefaultGroup); err != nil {
		t.Fatal(err)
	}

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
	if len(paths) != 1 || paths[0] != tmpFile {
		t.Fatalf("got paths=%v, want [%s]", paths, tmpFile)
	}
}

func TestEnableBackup_FinalSaveOnCancel(t *testing.T) {
	ctx, cancel := donegroup.WithCancel(context.Background())

	s := NewState(ctx)

	tmpFile := filepath.Join(t.TempDir(), "test-b.md")
	os.WriteFile(tmpFile, []byte("# B"), 0o600) //nolint:errcheck

	var mu sync.Mutex
	var saved []RestoreData
	s.EnableBackup(ctx, func(data RestoreData) {
		mu.Lock()
		saved = append(saved, data)
		mu.Unlock()
	})

	if _, err := s.AddFile(tmpFile, DefaultGroup); err != nil {
		t.Fatal(err)
	}

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
	if len(paths) != 1 || paths[0] != tmpFile {
		t.Fatalf("got paths=%v, want [%s]", paths, tmpFile)
	}
}

func TestEnableBackup_ReflectsLatestState(t *testing.T) {
	ctx, cancel := donegroup.WithCancel(context.Background())
	defer cancel()

	s := NewState(ctx)

	dir := t.TempDir()
	tmpC := filepath.Join(dir, "test-c.md")
	tmpD := filepath.Join(dir, "test-d.md")
	os.WriteFile(tmpC, []byte("# C"), 0o600) //nolint:errcheck
	os.WriteFile(tmpD, []byte("# D"), 0o600) //nolint:errcheck

	var mu sync.Mutex
	var saved []RestoreData
	s.EnableBackup(ctx, func(data RestoreData) {
		mu.Lock()
		saved = append(saved, data)
		mu.Unlock()
	})

	if _, err := s.AddFile(tmpC, DefaultGroup); err != nil {
		t.Fatal(err)
	}
	if _, err := s.AddFile(tmpD, DefaultGroup); err != nil {
		t.Fatal(err)
	}

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

func TestAddFile_RejectsBinaryFile(t *testing.T) {
	s := newTestState(t)

	dir := t.TempDir()

	// Binary file (contains NUL bytes)
	binFile := filepath.Join(dir, "image.png")
	os.WriteFile(binFile, []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 0x00}, 0o600) //nolint:errcheck

	_, err := s.AddFile(binFile, DefaultGroup)
	if err == nil {
		t.Fatal("expected error for binary file, got nil")
	}
	if !errors.Is(err, ErrBinaryFile) {
		t.Fatalf("expected ErrBinaryFile, got: %v", err)
	}

	// Text file should succeed
	txtFile := filepath.Join(dir, "readme.md")
	os.WriteFile(txtFile, []byte("# Hello"), 0o600) //nolint:errcheck

	entry, err := s.AddFile(txtFile, DefaultGroup)
	if err != nil {
		t.Fatalf("unexpected error for text file: %v", err)
	}
	if entry == nil {
		t.Fatal("expected non-nil entry for text file")
	}

	// Non-existent file should not error
	_, err = s.AddFile(filepath.Join(dir, "nonexistent.md"), DefaultGroup)
	if err != nil {
		t.Fatalf("unexpected error for non-existent file: %v", err)
	}
}

func TestAddFile_RejectsNonRegularFile(t *testing.T) {
	s := newTestState(t)

	dir := t.TempDir()

	entry, err := s.AddFile(dir, DefaultGroup)
	if err == nil {
		t.Fatal("expected error for non-regular (directory) path, got nil")
	}
	if entry != nil {
		t.Fatalf("expected nil entry for non-regular (directory) path, got %#v", entry)
	}
}

func TestAddUploadedFile(t *testing.T) {
	t.Run("adds uploaded file to group", func(t *testing.T) {
		s := newTestState(t)
		entry := s.AddUploadedFile("test.md", "# Hello", DefaultGroup)

		if entry.Name != "test.md" {
			t.Fatalf("got name %q, want %q", entry.Name, "test.md")
		}
		if !entry.Uploaded {
			t.Fatal("entry should be marked as uploaded")
		}
		if entry.Path != "" {
			t.Fatalf("uploaded file should have empty path, got %q", entry.Path)
		}
		if entry.content != "# Hello" {
			t.Fatal("uploaded file content mismatch")
		}
		if len(s.groups[DefaultGroup].Files) != 1 {
			t.Fatalf("got %d files, want 1", len(s.groups[DefaultGroup].Files))
		}
	})

	t.Run("deduplicates by content", func(t *testing.T) {
		s := newTestState(t)
		e1 := s.AddUploadedFile("a.md", "# Same", DefaultGroup)
		e2 := s.AddUploadedFile("b.md", "# Same", DefaultGroup)

		if e1.ID != e2.ID {
			t.Fatal("same content should produce same ID")
		}
		if len(s.groups[DefaultGroup].Files) != 1 {
			t.Fatalf("got %d files, want 1 (dedup)", len(s.groups[DefaultGroup].Files))
		}
	})

	t.Run("different content gets different IDs", func(t *testing.T) {
		s := newTestState(t)
		e1 := s.AddUploadedFile("a.md", "# A", DefaultGroup)
		e2 := s.AddUploadedFile("b.md", "# B", DefaultGroup)

		if e1.ID == e2.ID {
			t.Fatal("different content should produce different IDs")
		}
		if len(s.groups[DefaultGroup].Files) != 2 {
			t.Fatalf("got %d files, want 2", len(s.groups[DefaultGroup].Files))
		}
	})

	t.Run("allows same content in different groups", func(t *testing.T) {
		s := newTestState(t)
		e1 := s.AddUploadedFile("a.md", "# Same", "src")
		e2 := s.AddUploadedFile("b.md", "# Same", "dst")

		if e1.ID != e2.ID {
			t.Fatal("same content should produce same ID")
		}
		if e1 == e2 {
			t.Fatal("different groups should create separate entries")
		}
		if len(s.groups["src"].Files) != 1 {
			t.Fatalf("src group should have 1 file, got %d", len(s.groups["src"].Files))
		}
		if len(s.groups["dst"].Files) != 1 {
			t.Fatalf("dst group should have 1 file, got %d", len(s.groups["dst"].Files))
		}
	})
}

func TestHandleAddFile_RejectsBinaryFile(t *testing.T) {
	dir := t.TempDir()

	t.Run("returns 400 for binary file", func(t *testing.T) {
		s := newTestState(t)
		handler := NewHandler(s)

		binFile := filepath.Join(dir, "image.png")
		os.WriteFile(binFile, []byte{0x89, 0x50, 0x4e, 0x47, 0x00}, 0o600) //nolint:errcheck

		body, err := json.Marshal(addFileRequest{Path: binFile})
		if err != nil {
			t.Fatal(err)
		}
		req := httptest.NewRequest("POST", "/_/api/groups/default/files", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("got status %d, want %d", rec.Code, http.StatusBadRequest)
		}
	})

	t.Run("returns 200 for text file", func(t *testing.T) {
		s := newTestState(t)
		handler := NewHandler(s)

		txtFile := filepath.Join(dir, "readme.md")
		os.WriteFile(txtFile, []byte("# Hello"), 0o600) //nolint:errcheck

		body, err := json.Marshal(addFileRequest{Path: txtFile})
		if err != nil {
			t.Fatal(err)
		}
		req := httptest.NewRequest("POST", "/_/api/groups/default/files", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("got status %d, want %d", rec.Code, http.StatusOK)
		}

		var entry FileEntry
		if err := json.NewDecoder(rec.Body).Decode(&entry); err != nil {
			t.Fatal(err)
		}
		if entry.Name != "readme.md" {
			t.Fatalf("got name %q, want %q", entry.Name, "readme.md")
		}
	})
}

func TestHandleUploadFile(t *testing.T) {
	t.Run("uploads file via HTTP", func(t *testing.T) {
		s := newTestState(t)
		handler := NewHandler(s)

		body, err := json.Marshal(uploadFileRequest{Name: "test.md", Content: "# Hello"})
		if err != nil {
			t.Fatal(err)
		}
		req := httptest.NewRequest("POST", "/_/api/groups/default/files/upload", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("got status %d, want %d", rec.Code, http.StatusOK)
		}

		var entry FileEntry
		if err := json.NewDecoder(rec.Body).Decode(&entry); err != nil {
			t.Fatal(err)
		}
		if !entry.Uploaded {
			t.Fatal("response should have uploaded=true")
		}
		if entry.Name != "test.md" {
			t.Fatalf("got name %q, want %q", entry.Name, "test.md")
		}
	})

	t.Run("returns 413 for oversized content", func(t *testing.T) {
		s := newTestState(t)
		handler := NewHandler(s)

		oversized := strings.Repeat("x", 10<<20+1) // 10MB + 1 byte
		body, err := json.Marshal(uploadFileRequest{Name: "big.md", Content: oversized})
		if err != nil {
			t.Fatal(err)
		}
		req := httptest.NewRequest("POST", "/_/api/groups/default/files/upload", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusRequestEntityTooLarge {
			t.Fatalf("got status %d, want %d", rec.Code, http.StatusRequestEntityTooLarge)
		}
	})

	t.Run("returns 400 for missing name", func(t *testing.T) {
		s := newTestState(t)
		handler := NewHandler(s)

		body, err := json.Marshal(uploadFileRequest{Name: "", Content: "# Hello"})
		if err != nil {
			t.Fatal(err)
		}
		req := httptest.NewRequest("POST", "/_/api/groups/default/files/upload", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("got status %d, want %d", rec.Code, http.StatusBadRequest)
		}
	})
}

func TestUploadedFileContent(t *testing.T) {
	t.Run("serves uploaded file content", func(t *testing.T) {
		s := newTestState(t)
		entry := s.AddUploadedFile("test.md", "# Uploaded Content", DefaultGroup)

		handler := NewHandler(s)
		req := httptest.NewRequest("GET", fmt.Sprintf("/_/api/groups/default/files/%s/content", entry.ID), nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("got status %d, want %d", rec.Code, http.StatusOK)
		}

		var resp fileContentResponse
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatal(err)
		}
		if resp.Content != "# Uploaded Content" {
			t.Fatalf("got content %q, want %q", resp.Content, "# Uploaded Content")
		}
		if resp.BaseDir != "" {
			t.Fatalf("uploaded file should have empty baseDir, got %q", resp.BaseDir)
		}
	})

	t.Run("returns 404 for raw assets of uploaded file", func(t *testing.T) {
		s := newTestState(t)
		entry := s.AddUploadedFile("test.md", "# Hello", DefaultGroup)

		handler := NewHandler(s)
		req := httptest.NewRequest("GET", fmt.Sprintf("/_/api/groups/default/files/%s/raw/image.png", entry.ID), nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Fatalf("got status %d, want %d", rec.Code, http.StatusNotFound)
		}
	})

	t.Run("returns 400 for open relative file of uploaded file", func(t *testing.T) {
		s := newTestState(t)
		entry := s.AddUploadedFile("test.md", "# Hello", DefaultGroup)

		handler := NewHandler(s)
		body, err := json.Marshal(openFileRequest{FileID: entry.ID, Path: "./other.md"})
		if err != nil {
			t.Fatal(err)
		}
		req := httptest.NewRequest("POST", "/_/api/groups/default/files/open", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("got status %d, want %d", rec.Code, http.StatusBadRequest)
		}
	})
}

func TestSearch(t *testing.T) {
	t.Run("searches file contents within a group", func(t *testing.T) {
		s := newTestState(t)
		dir := t.TempDir()

		readme := filepath.Join(dir, "README.md")
		guide := filepath.Join(dir, "GUIDE.md")
		if err := os.WriteFile(readme, []byte("# Intro\ncache warmup\nmore cache\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(guide, []byte("# Guide\nnothing here\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := s.AddFile(readme, DefaultGroup); err != nil {
			t.Fatal(err)
		}
		if _, err := s.AddFile(guide, DefaultGroup); err != nil {
			t.Fatal(err)
		}
		s.AddUploadedFile("upload.md", "# Upload\ncache line\n", "docs")

		handler := NewHandler(s)
		req := httptest.NewRequest("GET", "/_/api/search?q=cache&group=default&limit=10&context=1", nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("got status %d, want %d", rec.Code, http.StatusOK)
		}

		var resp searchResponse
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatal(err)
		}
		if resp.Total != 2 {
			t.Fatalf("got total %d, want 2", resp.Total)
		}
		if len(resp.Results) != 1 {
			t.Fatalf("got %d results, want 1", len(resp.Results))
		}
		if resp.Results[0].FileName != "README.md" {
			t.Fatalf("got file %q, want README.md", resp.Results[0].FileName)
		}
		if got := resp.Results[0].Matches[0].Heading; got != "Intro" {
			t.Fatalf("got heading %q, want Intro", got)
		}
		if got := resp.Results[0].Matches[0].Before; len(got) != 1 || got[0] != "# Intro" {
			t.Fatalf("got before %#v, want [\"# Intro\"]", got)
		}
	})

	t.Run("ignores headings inside fenced code blocks", func(t *testing.T) {
		s := newTestState(t)
		dir := t.TempDir()

		f := filepath.Join(dir, "fenced.md")
		content := "# Real Heading\n```\n# Fake Heading\nfind me\n```\n# After Fence\nfind me again\n"
		if err := os.WriteFile(f, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := s.AddFile(f, DefaultGroup); err != nil {
			t.Fatal(err)
		}

		handler := NewHandler(s)
		req := httptest.NewRequest("GET", "/_/api/search?q=find+me&group=default&context=0", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("got status %d, want %d", rec.Code, http.StatusOK)
		}

		var resp searchResponse
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatal(err)
		}
		if len(resp.Results) != 1 || len(resp.Results[0].Matches) != 2 {
			t.Fatalf("expected 1 result with 2 matches, got %d results", len(resp.Results))
		}
		// First match is inside the code block — heading should be "Real Heading", not "Fake Heading"
		if got := resp.Results[0].Matches[0].Heading; got != "Real Heading" {
			t.Fatalf("got heading %q for match inside code block, want \"Real Heading\"", got)
		}
		// Second match is after the fence closes — heading should be "After Fence"
		if got := resp.Results[0].Matches[1].Heading; got != "After Fence" {
			t.Fatalf("got heading %q for match after fence, want \"After Fence\"", got)
		}
	})

	t.Run("strips closing hashes from headings", func(t *testing.T) {
		s := newTestState(t)
		dir := t.TempDir()

		f := filepath.Join(dir, "closing.md")
		content := "# Title ###\nfind me\n"
		if err := os.WriteFile(f, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := s.AddFile(f, DefaultGroup); err != nil {
			t.Fatal(err)
		}

		handler := NewHandler(s)
		req := httptest.NewRequest("GET", "/_/api/search?q=find+me&group=default&context=0", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		var resp searchResponse
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatal(err)
		}
		if len(resp.Results) != 1 || len(resp.Results[0].Matches) != 1 {
			t.Fatalf("expected 1 result with 1 match, got %d results", len(resp.Results))
		}
		if got := resp.Results[0].Matches[0].Heading; got != "Title" {
			t.Fatalf("got heading %q, want \"Title\"", got)
		}
	})

	t.Run("returns 400 for missing query", func(t *testing.T) {
		s := newTestState(t)
		handler := NewHandler(s)

		req := httptest.NewRequest("GET", "/_/api/search?group=default", nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("got status %d, want %d", rec.Code, http.StatusBadRequest)
		}
	})

	t.Run("returns 404 for unknown group", func(t *testing.T) {
		s := newTestState(t)
		handler := NewHandler(s)

		req := httptest.NewRequest("GET", "/_/api/search?q=cache&group=missing", nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Fatalf("got status %d, want %d", rec.Code, http.StatusNotFound)
		}
	})
}

func TestMoveUploadedFile(t *testing.T) {
	t.Run("moves uploaded file between groups", func(t *testing.T) {
		s := newTestState(t)
		entry := s.AddUploadedFile("test.md", "# Hello", "src")

		if err := s.MoveFile(entry.ID, "src", "dst"); err != nil {
			t.Fatalf("MoveFile returned error: %v", err)
		}

		if _, ok := s.groups["src"]; ok {
			t.Error("empty source group should have been deleted")
		}
		if len(s.groups["dst"].Files) != 1 {
			t.Fatalf("got %d files in dst, want 1", len(s.groups["dst"].Files))
		}
		if !s.groups["dst"].Files[0].Uploaded {
			t.Error("moved file should still be marked as uploaded")
		}
	})

}

func TestSnapshotRestoreDataWithUploads(t *testing.T) {
	t.Run("includes uploaded files in snapshot", func(t *testing.T) {
		s := newTestState(t)

		dir := t.TempDir()
		fsFile := filepath.Join(dir, "fs.md")
		os.WriteFile(fsFile, []byte("# FS"), 0o600) //nolint:errcheck
		if _, err := s.AddFile(fsFile, DefaultGroup); err != nil {
			t.Fatal(err)
		}
		s.AddUploadedFile("upload.md", "# Uploaded", DefaultGroup)

		s.mu.RLock()
		data := s.snapshotRestoreData()
		s.mu.RUnlock()

		// Filesystem file should be in Groups, not in UploadedFiles
		if len(data.Groups[DefaultGroup]) != 1 {
			t.Fatalf("got %d paths, want 1", len(data.Groups[DefaultGroup]))
		}
		if data.Groups[DefaultGroup][0] != fsFile {
			t.Fatalf("got path %q, want %q", data.Groups[DefaultGroup][0], fsFile)
		}

		// Uploaded file should be in UploadedFiles
		if len(data.UploadedFiles) != 1 {
			t.Fatalf("got %d uploaded files, want 1", len(data.UploadedFiles))
		}
		if data.UploadedFiles[0].Name != "upload.md" {
			t.Fatalf("got name %q, want %q", data.UploadedFiles[0].Name, "upload.md")
		}
		if data.UploadedFiles[0].Content != "# Uploaded" {
			t.Fatalf("got content %q, want %q", data.UploadedFiles[0].Content, "# Uploaded")
		}
		if data.UploadedFiles[0].Group != DefaultGroup {
			t.Fatalf("got group %q, want %q", data.UploadedFiles[0].Group, DefaultGroup)
		}
	})
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

func TestDirMove(t *testing.T) {
	ctx, cancel := donegroup.WithCancel(context.Background())
	defer cancel()

	s := NewState(ctx)

	dir := t.TempDir()
	subDir := filepath.Join(dir, "docs")
	if err := os.MkdirAll(subDir, 0o755); err != nil {
		t.Fatal(err)
	}
	f1 := filepath.Join(subDir, "a.md")
	f2 := filepath.Join(subDir, "b.md")
	os.WriteFile(f1, []byte("# A"), 0o600) //nolint:errcheck
	os.WriteFile(f2, []byte("# B"), 0o600) //nolint:errcheck

	pattern := filepath.Join(dir, "**", "*.md")
	entries, err := s.AddPattern(pattern, DefaultGroup)
	if err != nil {
		t.Fatalf("AddPattern returned error: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("got %d entries, want 2", len(entries))
	}

	oldID1 := entries[0].ID
	oldID2 := entries[1].ID

	newDir := filepath.Join(dir, "docs-renamed")
	if err := os.Rename(subDir, newDir); err != nil {
		t.Fatal(err)
	}

	deadline := time.After(5 * time.Second)
	for {
		select {
		case <-deadline:
			t.Fatal("timed out waiting for stale files to be removed")
		default:
		}
		if s.FindFile(oldID1, DefaultGroup) == nil && s.FindFile(oldID2, DefaultGroup) == nil {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func TestExtractTitle(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    string
	}{
		{"h1", "# Hello World", "Hello World"},
		{"h2", "## Second Level", "Second Level"},
		{"h3", "### Third Level", "Third Level"},
		{"with leading blank lines", "\n\n# Title After Blanks", "Title After Blanks"},
		{"with frontmatter-like content", "---\ntitle: foo\n---\n# Real Title", "Real Title"},
		{"no heading", "Just some text\nwithout headings", ""},
		{"empty", "", ""},
		{"heading with extra spaces", "#   Spaced Out   ", "Spaced Out"},
		{"bare hash", "#", ""},
		{"bare hashes", "###", ""},
		{"no space after hash", "#not-a-heading\n# Real Title", "Real Title"},
		{"heading inside fenced code block", "```\n# Not A Title\n```\n# Real Title", "Real Title"},
		{"heading inside tilde fence", "~~~\n# Not A Title\n~~~\n# Real Title", "Real Title"},
		{"only heading inside fence", "```\n# Only In Fence\n```", ""},
		{"only first heading", "# First\n# Second", "First"},
		{"tab after hash", "#\tTab Title", "Tab Title"},
		{"tab indented line", "\t# Not A Heading\n# Real Title", "Real Title"},
		{"mixed spaces and tab indented line", "   \t# Not A Heading\n# Real Title", "Real Title"},
		{"seven hashes", "####### Not A Heading\n# Real Title", "Real Title"},
		{"longer fence not closed by shorter", "````\n# Not A Title\n```\n````\n# Real Title", "Real Title"},
		{"tilde fence not closed by backtick", "~~~\n# Not A Title\n```\n~~~\n# Real Title", "Real Title"},
		{"closing hash sequence", "# Title ###", "Title"},
		{"closing hash sequence with spaces", "# Title   ## ", "Title"},
		{"hash only closing", "# ###", ""},
		{"closing hashes not preceded by space", "# Title#", "Title#"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractTitle(tt.content)
			if got != tt.want {
				t.Errorf("extractTitle() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestExtractTitleFromFile(t *testing.T) {
	dir := t.TempDir()

	t.Run("reads title from file", func(t *testing.T) {
		f := filepath.Join(dir, "with-title.md")
		os.WriteFile(f, []byte("# File Title\nSome content"), 0o600) //nolint:errcheck
		got, ok := extractTitleFromFile(f)
		if !ok {
			t.Fatal("expected ok=true")
		}
		if got != "File Title" {
			t.Errorf("got %q, want %q", got, "File Title")
		}
	})

	t.Run("returns empty for file without heading", func(t *testing.T) {
		f := filepath.Join(dir, "no-title.md")
		os.WriteFile(f, []byte("No heading here"), 0o600) //nolint:errcheck
		got, ok := extractTitleFromFile(f)
		if !ok {
			t.Fatal("expected ok=true")
		}
		if got != "" {
			t.Errorf("got %q, want empty", got)
		}
	})

	t.Run("returns ok=false for nonexistent file", func(t *testing.T) {
		_, ok := extractTitleFromFile(filepath.Join(dir, "nope.md"))
		if ok {
			t.Error("expected ok=false for nonexistent file")
		}
	})
}

func TestAddFile_ExtractsTitle(t *testing.T) {
	s := newTestState(t)

	dir := t.TempDir()
	f := filepath.Join(dir, "doc.md")
	os.WriteFile(f, []byte("# My Document\nContent here"), 0o600) //nolint:errcheck

	entry, err := s.AddFile(f, DefaultGroup)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if entry.Title != "My Document" {
		t.Errorf("Title = %q, want %q", entry.Title, "My Document")
	}
}

func TestAddFile_NoHeading(t *testing.T) {
	s := newTestState(t)

	dir := t.TempDir()
	f := filepath.Join(dir, "doc.md")
	os.WriteFile(f, []byte("No heading here"), 0o600) //nolint:errcheck

	entry, err := s.AddFile(f, DefaultGroup)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if entry.Title != "" {
		t.Errorf("Title = %q, want empty", entry.Title)
	}
}

func TestAddUploadedFile_ExtractsTitle(t *testing.T) {
	s := newTestState(t)

	entry := s.AddUploadedFile("note.md", "## Uploaded Note\nBody", DefaultGroup)
	if entry.Title != "Uploaded Note" {
		t.Errorf("Title = %q, want %q", entry.Title, "Uploaded Note")
	}
}

func TestNotifyFileChangedByPath_UpdatesTitle(t *testing.T) {
	s := newTestState(t)

	dir := t.TempDir()
	f := filepath.Join(dir, "evolving.md")
	os.WriteFile(f, []byte("# Original Title"), 0o600) //nolint:errcheck

	entry, err := s.AddFile(f, DefaultGroup)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if entry.Title != "Original Title" {
		t.Fatalf("initial Title = %q, want %q", entry.Title, "Original Title")
	}

	// Update file content and trigger the production code path.
	os.WriteFile(f, []byte("# Updated Title"), 0o600) //nolint:errcheck
	s.notifyFileChangedByPath(f)

	if entry.Title != "Updated Title" {
		t.Errorf("Title = %q, want %q", entry.Title, "Updated Title")
	}
}

func TestHandleGroups_IncludesTitle(t *testing.T) {
	s := newTestState(t)

	dir := t.TempDir()
	f := filepath.Join(dir, "titled.md")
	os.WriteFile(f, []byte("# API Reference\nEndpoints..."), 0o600) //nolint:errcheck

	_, err := s.AddFile(f, DefaultGroup)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	handler := NewHandler(s)
	req := httptest.NewRequest("GET", "/_/api/groups", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	var groups []Group
	if err := json.NewDecoder(rec.Body).Decode(&groups); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(groups) != 1 || len(groups[0].Files) != 1 {
		t.Fatalf("expected 1 group with 1 file, got %d groups", len(groups))
	}
	if groups[0].Files[0].Title != "API Reference" {
		t.Errorf("Title in API response = %q, want %q", groups[0].Files[0].Title, "API Reference")
	}
}

func TestCSPHeader(t *testing.T) {
	s := newTestState(t)
	handler := NewHandler(s)

	routes := []struct {
		method string
		path   string
	}{
		{"GET", "/"},
		{"GET", "/_/api/status"},
	}

	for _, rt := range routes {
		req := httptest.NewRequest(rt.method, rt.path, nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		csp := rec.Header().Get("Content-Security-Policy")
		if csp == "" {
			t.Errorf("%s %s: Content-Security-Policy header not set", rt.method, rt.path)
			continue
		}
		for _, directive := range []string{
			"default-src 'self'",
			"script-src 'self' 'unsafe-eval'",
			"style-src 'self' 'unsafe-inline'",
			"img-src 'self' https: data:",
			"font-src 'self' data:",
			"connect-src 'self'",
			"object-src 'none'",
			"base-uri 'self'",
			"form-action 'self'",
			"frame-ancestors 'none'",
		} {
			if !strings.Contains(csp, directive) {
				t.Errorf("%s %s: CSP missing directive %q\ngot: %s", rt.method, rt.path, directive, csp)
			}
		}
	}
}

func TestRemoveFileNotFound(t *testing.T) {
	s := newTestState(t)
	s.groups[DefaultGroup] = &Group{
		Name:  DefaultGroup,
		Files: []*FileEntry{{ID: testIDa, Name: "a.md", Path: "/a.md"}},
	}

	ok := s.RemoveFile("nonexistent-id", DefaultGroup)
	if ok {
		t.Error("RemoveFile returned true for nonexistent file ID")
	}

	// Verify original file is still there
	if len(s.groups[DefaultGroup].Files) != 1 {
		t.Error("expected files to be unchanged")
	}
}
