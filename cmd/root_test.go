package cmd

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/k1LoW/mo/internal/server"
)

func TestResolvePatterns_NoGlobChars(t *testing.T) {
	_, err := resolvePatterns([]string{"README.md"})
	if err == nil {
		t.Fatal("resolvePatterns should return error for pattern without glob chars")
	}
}

func TestResolvePatterns_Valid(t *testing.T) {
	patterns, err := resolvePatterns([]string{"**/*.md", "docs/*.md"})
	if err != nil {
		t.Fatalf("resolvePatterns returned error: %v", err)
	}
	if len(patterns) != 2 {
		t.Fatalf("got %d patterns, want 2", len(patterns))
	}
	for _, p := range patterns {
		if !filepath.IsAbs(p) {
			t.Errorf("pattern %q is not absolute", p)
		}
	}
}

func TestRun_UnwatchWithWatch(t *testing.T) {
	unwatchPatterns = []string{"**/*.md"}
	watchPatterns = []string{"**/*.md"}
	defer func() {
		unwatchPatterns = nil
		watchPatterns = nil
	}()

	err := run(rootCmd, nil)
	if err == nil {
		t.Fatal("run should return error when --unwatch and --watch are both specified")
	}
	want := "cannot use --unwatch with --watch"
	if err.Error() != want {
		t.Fatalf("got error %q, want %q", err.Error(), want)
	}
}

func TestRun_UnwatchWithArgs(t *testing.T) {
	unwatchPatterns = []string{"**/*.md"}
	defer func() { unwatchPatterns = nil }()

	err := run(rootCmd, []string{"README.md"})
	if err == nil {
		t.Fatal("run should return error when --unwatch and args are both specified")
	}
	want := "cannot use --unwatch with file arguments"
	if err.Error() != want {
		t.Fatalf("got error %q, want %q", err.Error(), want)
	}
}

func TestRun_Close(t *testing.T) {
	t.Run("without args returns error", func(t *testing.T) {
		closeFiles = true
		defer func() { closeFiles = false }()

		err := run(rootCmd, nil)
		if err == nil {
			t.Fatal("run should return error when --close is specified without file arguments")
		}
		want := "--close requires at least one file argument"
		if err.Error() != want {
			t.Fatalf("got error %q, want %q", err.Error(), want)
		}
	})

	t.Run("with watch returns error", func(t *testing.T) {
		closeFiles = true
		watchPatterns = []string{"**/*.md"}
		defer func() {
			closeFiles = false
			watchPatterns = nil
		}()

		err := run(rootCmd, []string{"README.md"})
		if err == nil {
			t.Fatal("run should return error when --close and --watch are both specified")
		}
		want := "cannot use --close with --watch"
		if err.Error() != want {
			t.Fatalf("got error %q, want %q", err.Error(), want)
		}
	})
}

func TestRun_WatchWithArgs(t *testing.T) {
	t.Run("with glob pattern", func(t *testing.T) {
		f := filepath.Join(t.TempDir(), "test.md")
		writeTestFile(t, f, []byte("# Test"))

		watchPatterns = []string{"**/*.md"}
		defer func() { watchPatterns = nil }()

		err := run(rootCmd, []string{f})
		if err == nil {
			t.Fatal("run should return error when --watch and args are both specified")
		}
		want := "cannot use --watch (-w) with file arguments"
		if err.Error() != want {
			t.Fatalf("got error %q, want %q", err.Error(), want)
		}
	})

	t.Run("without glob chars hints shell expansion", func(t *testing.T) {
		f1 := filepath.Join(t.TempDir(), "a.md")
		writeTestFile(t, f1, []byte("# A"))
		f2 := filepath.Join(t.TempDir(), "b.md")
		writeTestFile(t, f2, []byte("# B"))

		watchPatterns = []string{f1}
		defer func() { watchPatterns = nil }()

		err := run(rootCmd, []string{f2})
		if err == nil {
			t.Fatal("run should return error when --watch and args are both specified")
		}
		if !strings.Contains(err.Error(), "shell may have expanded") {
			t.Fatalf("error should hint shell expansion, got %q", err.Error())
		}
	})

	t.Run("non-existent file with watch returns file not found", func(t *testing.T) {
		watchPatterns = []string{"**/*.md"}
		defer func() { watchPatterns = nil }()

		err := run(rootCmd, []string{"nonexistent.md"})
		if err == nil {
			t.Fatal("run should return error for non-existent file")
		}
		if !strings.Contains(err.Error(), "file not found") {
			t.Fatalf("got error %q, want file not found error", err.Error())
		}
	})
}

func TestMergeGroups(t *testing.T) {
	t.Run("restored files come first, CLI files appended after", func(t *testing.T) {
		base := map[string][]string{"default": {"/a.md", "/b.md"}}
		additional := map[string][]string{"default": {"/c.md"}}
		got := mergeGroups(base, additional)
		want := []string{"/a.md", "/b.md", "/c.md"}
		if len(got["default"]) != len(want) {
			t.Fatalf("got %v, want %v", got["default"], want)
		}
		for i, v := range want {
			if got["default"][i] != v {
				t.Fatalf("got[%d] = %s, want %s", i, got["default"][i], v)
			}
		}
	})

	t.Run("deduplicates files in the same group", func(t *testing.T) {
		base := map[string][]string{"default": {"/a.md", "/b.md"}}
		additional := map[string][]string{"default": {"/b.md", "/c.md"}}
		got := mergeGroups(base, additional)
		want := []string{"/a.md", "/b.md", "/c.md"}
		if len(got["default"]) != len(want) {
			t.Fatalf("got %v, want %v", got["default"], want)
		}
		for i, v := range want {
			if got["default"][i] != v {
				t.Fatalf("got[%d] = %s, want %s", i, got["default"][i], v)
			}
		}
	})

	t.Run("merges different groups", func(t *testing.T) {
		base := map[string][]string{"docs": {"/doc.md"}}
		additional := map[string][]string{"default": {"/a.md"}}
		got := mergeGroups(base, additional)
		if len(got["docs"]) != 1 || got["docs"][0] != "/doc.md" {
			t.Fatalf("got docs=%v, want [/doc.md]", got["docs"])
		}
		if len(got["default"]) != 1 || got["default"][0] != "/a.md" {
			t.Fatalf("got default=%v, want [/a.md]", got["default"])
		}
	})

	t.Run("nil base returns additional only", func(t *testing.T) {
		additional := map[string][]string{"default": {"/a.md"}}
		got := mergeGroups(nil, additional)
		if len(got["default"]) != 1 || got["default"][0] != "/a.md" {
			t.Fatalf("got %v, want [/a.md]", got["default"])
		}
	})

	t.Run("nil additional returns base only", func(t *testing.T) {
		base := map[string][]string{"default": {"/a.md"}}
		got := mergeGroups(base, nil)
		if len(got["default"]) != 1 || got["default"][0] != "/a.md" {
			t.Fatalf("got %v, want [/a.md]", got["default"])
		}
	})

	t.Run("both nil returns nil", func(t *testing.T) {
		got := mergeGroups(nil, nil)
		if got != nil {
			t.Fatalf("got %v, want nil", got)
		}
	})
}

func TestBuildDeeplink(t *testing.T) {
	tests := []struct {
		addr      string
		groupName string
		fileID    string
		want      string
	}{
		{"localhost:6275", server.DefaultGroup, "abc12345", "http://localhost:6275/?file=abc12345"},
		{"localhost:6275", "design", "def67890", "http://localhost:6275/design?file=def67890"},
	}
	for _, tt := range tests {
		got := buildDeeplink(tt.addr, tt.groupName, tt.fileID)
		if got != tt.want {
			t.Errorf("buildDeeplink(%q, %q, %q) = %q, want %q", tt.addr, tt.groupName, tt.fileID, got, tt.want)
		}
	}
}

func TestDisplayNames(t *testing.T) {
	t.Run("unique basenames stay short", func(t *testing.T) {
		paths := []string{"/a/README.md", "/b/CHANGELOG.md"}
		got := displayNames(paths)
		if got[0] != "README.md" || got[1] != "CHANGELOG.md" {
			t.Fatalf("got %v, want [README.md CHANGELOG.md]", got)
		}
	})

	t.Run("duplicate basenames get parent dir", func(t *testing.T) {
		paths := []string{"/project/docs/README.md", "/project/api/README.md"}
		got := displayNames(paths)
		want0 := filepath.Join("docs", "README.md")
		want1 := filepath.Join("api", "README.md")
		if got[0] != want0 || got[1] != want1 {
			t.Fatalf("got %v, want [%s %s]", got, want0, want1)
		}
	})

	t.Run("deeply nested duplicates get enough context", func(t *testing.T) {
		paths := []string{"/a/x/README.md", "/b/x/README.md"}
		got := displayNames(paths)
		want0 := filepath.Join("a", "x", "README.md")
		want1 := filepath.Join("b", "x", "README.md")
		if got[0] != want0 || got[1] != want1 {
			t.Fatalf("got %v, want [%s %s]", got, want0, want1)
		}
	})

	t.Run("identical paths do not loop forever", func(t *testing.T) {
		paths := []string{"/a/b/README.md", "/a/b/README.md"}
		got := displayNames(paths)
		if len(got) != 2 {
			t.Fatalf("got %d names, want 2", len(got))
		}
	})

	t.Run("single entry stays short", func(t *testing.T) {
		paths := []string{"/a/b/c/README.md"}
		got := displayNames(paths)
		if got[0] != "README.md" {
			t.Fatalf("got %v, want [README.md]", got)
		}
	})
}

func TestFilterValidRestoreData(t *testing.T) {
	t.Run("keeps only existing files", func(t *testing.T) {
		dir := t.TempDir()
		existing := filepath.Join(dir, "a.md")
		os.WriteFile(existing, []byte("# A"), 0o600) //nolint:errcheck
		missing := filepath.Join(dir, "missing.md")

		rd := &server.RestoreData{
			Groups: map[string][]string{
				"default": {existing, missing},
			},
		}

		filesByGroup, _, _ := filterValidRestoreData(rd)
		if len(filesByGroup["default"]) != 1 {
			t.Fatalf("got %d files, want 1", len(filesByGroup["default"]))
		}
		if filesByGroup["default"][0] != existing {
			t.Fatalf("got %s, want %s", filesByGroup["default"][0], existing)
		}
	})

	t.Run("omits group when all files missing", func(t *testing.T) {
		rd := &server.RestoreData{
			Groups: map[string][]string{
				"docs": {"/nonexistent/a.md", "/nonexistent/b.md"},
			},
		}

		filesByGroup, _, _ := filterValidRestoreData(rd)
		if _, ok := filesByGroup["docs"]; ok {
			t.Fatal("group with all missing files should not appear in result")
		}
	})

	t.Run("passes patterns through unchanged", func(t *testing.T) {
		rd := &server.RestoreData{
			Groups: map[string][]string{},
			Patterns: map[string][]string{
				"default": {"/some/path/*.md"},
			},
		}

		_, patternsByGroup, _ := filterValidRestoreData(rd)
		if len(patternsByGroup["default"]) != 1 {
			t.Fatalf("got %d patterns, want 1", len(patternsByGroup["default"]))
		}
		if patternsByGroup["default"][0] != "/some/path/*.md" {
			t.Fatalf("got %s, want /some/path/*.md", patternsByGroup["default"][0])
		}
	})

	t.Run("empty restore data returns empty results", func(t *testing.T) {
		rd := &server.RestoreData{}

		filesByGroup, patternsByGroup, _ := filterValidRestoreData(rd)
		if len(filesByGroup) != 0 {
			t.Fatalf("got %d groups, want 0", len(filesByGroup))
		}
		if len(patternsByGroup) != 0 {
			t.Fatalf("got %d pattern groups, want 0", len(patternsByGroup))
		}
	})
}

func TestDeeplinksToJSON(t *testing.T) {
	t.Run("empty entries returns empty slice", func(t *testing.T) {
		got := deeplinksToJSON(nil)
		if len(got) != 0 {
			t.Fatalf("got %d entries, want 0", len(got))
		}
	})

	t.Run("file entries with paths", func(t *testing.T) {
		entries := []deeplinkEntry{
			{URL: "http://localhost:6275/?file=abc", Path: "/home/user/README.md"},
			{URL: "http://localhost:6275/?file=def", Path: "/home/user/CHANGELOG.md"},
		}
		got := deeplinksToJSON(entries)
		if len(got) != 2 {
			t.Fatalf("got %d entries, want 2", len(got))
		}
		if got[0].URL != entries[0].URL {
			t.Errorf("got URL %q, want %q", got[0].URL, entries[0].URL)
		}
		if got[0].Name != "README.md" {
			t.Errorf("got Name %q, want %q", got[0].Name, "README.md")
		}
		if got[0].Path != entries[0].Path {
			t.Errorf("got Path %q, want %q", got[0].Path, entries[0].Path)
		}
	})

	t.Run("uploaded files with empty path use Name for display", func(t *testing.T) {
		entries := []deeplinkEntry{
			{URL: "http://localhost:6275/?file=abc", Path: "", Name: "uploaded.md"},
		}
		got := deeplinksToJSON(entries)
		if got[0].Name != "uploaded.md" {
			t.Errorf("got Name %q, want %q", got[0].Name, "uploaded.md")
		}
		if got[0].Path != "" {
			t.Errorf("got Path %q, want empty string", got[0].Path)
		}
	})
}

func TestDeeplinkDisplayNames(t *testing.T) {
	t.Run("uses Path when available", func(t *testing.T) {
		entries := []deeplinkEntry{
			{Path: "/a/README.md"},
			{Path: "/b/CHANGELOG.md"},
		}
		got := deeplinkDisplayNames(entries)
		if got[0] != "README.md" || got[1] != "CHANGELOG.md" {
			t.Fatalf("got %v, want [README.md CHANGELOG.md]", got)
		}
	})

	t.Run("falls back to Name when Path is empty", func(t *testing.T) {
		entries := []deeplinkEntry{
			{Path: "/a/README.md"},
			{Path: "", Name: "uploaded.md"},
		}
		got := deeplinkDisplayNames(entries)
		if got[0] != "README.md" || got[1] != "uploaded.md" {
			t.Fatalf("got %v, want [README.md uploaded.md]", got)
		}
	})

	t.Run("disambiguates duplicate names across path and uploaded", func(t *testing.T) {
		entries := []deeplinkEntry{
			{Path: "/a/docs/README.md"},
			{Path: "", Name: "README.md"},
		}
		got := deeplinkDisplayNames(entries)
		if got[0] == got[1] {
			t.Fatalf("names should differ but both are %q", got[0])
		}
	})
}

func TestEmitServeOutput(t *testing.T) {
	entries := []deeplinkEntry{
		{URL: "http://localhost:6275/?file=abc", Path: "/home/user/README.md"},
	}

	t.Run("json mode outputs valid JSON", func(t *testing.T) {
		jsonOutput = true
		defer func() { jsonOutput = false }()

		r, w, err := os.Pipe()
		if err != nil {
			t.Fatal(err)
		}
		oldStdout := os.Stdout
		os.Stdout = w

		emitServeOutput("localhost:6275", entries, true)

		w.Close()
		os.Stdout = oldStdout

		var buf bytes.Buffer
		buf.ReadFrom(r) //nolint:errcheck

		var output jsonServeOutput
		if err := json.Unmarshal(buf.Bytes(), &output); err != nil {
			t.Fatalf("invalid JSON: %v\noutput: %s", err, buf.String())
		}
		if output.URL != "http://localhost:6275" {
			t.Errorf("got URL %q, want %q", output.URL, "http://localhost:6275")
		}
		if len(output.Files) != 1 {
			t.Fatalf("got %d files, want 1", len(output.Files))
		}
		if output.Files[0].Name != "README.md" {
			t.Errorf("got file name %q, want %q", output.Files[0].Name, "README.md")
		}
	})

	t.Run("text mode with printURL prints URL line", func(t *testing.T) {
		jsonOutput = false

		r, w, err := os.Pipe()
		if err != nil {
			t.Fatal(err)
		}
		oldStdout := os.Stdout
		os.Stdout = w

		emitServeOutput("localhost:6275", entries, true)

		w.Close()
		os.Stdout = oldStdout

		var buf bytes.Buffer
		buf.ReadFrom(r) //nolint:errcheck

		output := buf.String()
		if !strings.Contains(output, "http://localhost:6275\n") {
			t.Errorf("expected URL line in output, got %q", output)
		}
		if !strings.Contains(output, "README.md") {
			t.Errorf("expected deeplink in output, got %q", output)
		}
	})

	t.Run("text mode without printURL omits URL line", func(t *testing.T) {
		jsonOutput = false

		r, w, err := os.Pipe()
		if err != nil {
			t.Fatal(err)
		}
		oldStdout := os.Stdout
		os.Stdout = w

		emitServeOutput("localhost:6275", entries, false)

		w.Close()
		os.Stdout = oldStdout

		var buf bytes.Buffer
		buf.ReadFrom(r) //nolint:errcheck

		output := buf.String()
		if strings.Contains(output, "http://localhost:6275\n") {
			t.Errorf("URL line should not appear, got %q", output)
		}
		if !strings.Contains(output, "README.md") {
			t.Errorf("expected deeplink in output, got %q", output)
		}
	})

	t.Run("json mode with uploaded file keeps path empty", func(t *testing.T) {
		jsonOutput = true
		defer func() { jsonOutput = false }()

		uploaded := []deeplinkEntry{
			{URL: "http://localhost:6275/?file=xyz", Path: "", Name: "upload.md"},
		}

		r, w, err := os.Pipe()
		if err != nil {
			t.Fatal(err)
		}
		oldStdout := os.Stdout
		os.Stdout = w

		emitServeOutput("localhost:6275", uploaded, true)

		w.Close()
		os.Stdout = oldStdout

		var buf bytes.Buffer
		buf.ReadFrom(r) //nolint:errcheck

		var output jsonServeOutput
		if err := json.Unmarshal(buf.Bytes(), &output); err != nil {
			t.Fatalf("invalid JSON: %v", err)
		}
		if output.Files[0].Path != "" {
			t.Errorf("got Path %q, want empty string", output.Files[0].Path)
		}
		if output.Files[0].Name != "upload.md" {
			t.Errorf("got Name %q, want %q", output.Files[0].Name, "upload.md")
		}
	})
}

func TestWaitForServerDown(t *testing.T) {
	// Use a short timeout for tests.
	orig := waitForServerDownTimeout
	waitForServerDownTimeout = 500 * time.Millisecond
	t.Cleanup(func() { waitForServerDownTimeout = orig })

	t.Run("returns nil when server actually stops", func(t *testing.T) {
		callCount := 0
		stopCh := make(chan struct{}, 1)

		var srv *httptest.Server //nolint:staticcheck // declared before assignment so the closure can reference srv
		srv = newFakeMoServer(t, func(w http.ResponseWriter, r *http.Request) {
			callCount++
			if callCount >= 3 {
				select {
				case stopCh <- struct{}{}:
				default:
				}
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{"version": "test", "pid": 1, "groups": []any{}}) //nolint:errcheck
		})

		go func() {
			<-stopCh
			srv.Close()
		}()

		addr := strings.TrimPrefix(srv.URL, "http://")
		err := waitForServerDown(addr)
		if err != nil {
			t.Fatalf("expected nil error, got %v", err)
		}
	})

	t.Run("returns error on timeout", func(t *testing.T) {
		srv := newFakeMoServer(t, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{"version": "test", "pid": 1, "groups": []any{}}) //nolint:errcheck
		})

		addr := strings.TrimPrefix(srv.URL, "http://")
		err := waitForServerDown(addr)
		if err == nil {
			t.Fatal("expected timeout error, got nil")
		}
		if !strings.Contains(err.Error(), "did not shut down") {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

// newFakeMoServer creates an httptest server that handles /_/api/status with
// the provided handler, and /_/api/shutdown with a 202 response.
func newFakeMoServer(t *testing.T, statusHandler http.HandlerFunc) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("GET /_/api/status", statusHandler)
	mux.HandleFunc("POST /_/api/shutdown", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func TestIsLoopbackBind(t *testing.T) {
	tests := []struct {
		name string
		bind string
		want bool
	}{
		{"localhost", "localhost", true},
		{"127.0.0.1", "127.0.0.1", true},
		{"::1", "::1", true},
		{"127.0.0.2", "127.0.0.2", true},
		{"0.0.0.0", "0.0.0.0", false},
		{"::", "::", false},
		{"192.168.1.1", "192.168.1.1", false},
		{"10.0.0.1", "10.0.0.1", false},
		{"example.com", "example.com", false},
		{"empty", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isLoopbackBind(tt.bind)
			if got != tt.want {
				t.Errorf("isLoopbackBind(%q) = %v, want %v", tt.bind, got, tt.want)
			}
		})
	}
}

func writeTestFile(t *testing.T, path string, content []byte) {
	t.Helper()
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatalf("failed to write test file %s: %v", path, err)
	}
}

func TestResolveArgs_Directory(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, "a.md"), []byte("# A"))
	writeTestFile(t, filepath.Join(dir, "b.md"), []byte("# B"))
	writeTestFile(t, filepath.Join(dir, "c.txt"), []byte("text"))

	files, dirPatterns, err := resolveArgs([]string{dir}, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(dirPatterns) != 0 {
		t.Fatalf("got %d dirPatterns, want 0", len(dirPatterns))
	}
	if len(files) != 2 {
		t.Fatalf("got %d files, want 2: %v", len(files), files)
	}
	for _, f := range files {
		if !strings.HasSuffix(f, ".md") {
			t.Errorf("unexpected non-.md file: %s", f)
		}
	}
}

func TestResolveArgs_DirectoryWithWatch(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, "a.md"), []byte("# A"))

	files, dirPatterns, err := resolveArgs([]string{dir}, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(files) != 0 {
		t.Fatalf("got %d files, want 0", len(files))
	}
	if len(dirPatterns) != 1 {
		t.Fatalf("got %d dirPatterns, want 1", len(dirPatterns))
	}
	want := filepath.Join(dir, "*.md")
	if dirPatterns[0] != want {
		t.Errorf("got pattern %q, want %q", dirPatterns[0], want)
	}
}

func TestResolveArgs_EmptyDirectory(t *testing.T) {
	dir := t.TempDir()

	_, _, err := resolveArgs([]string{dir}, false)
	if err == nil {
		t.Fatal("expected error for empty directory")
	}
	if !strings.Contains(err.Error(), "no .md files") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestResolveArgs_EmptyDirectoryWithWatch(t *testing.T) {
	dir := t.TempDir()

	files, dirPatterns, err := resolveArgs([]string{dir}, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(files) != 0 {
		t.Fatalf("got %d files, want 0", len(files))
	}
	if len(dirPatterns) != 1 {
		t.Fatalf("got %d dirPatterns, want 1", len(dirPatterns))
	}
}

func TestResolveArgs_MixedFilesAndDirs(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, "a.md"), []byte("# A"))

	singleFile := filepath.Join(t.TempDir(), "standalone.md")
	writeTestFile(t, singleFile, []byte("# Standalone"))

	files, dirPatterns, err := resolveArgs([]string{dir, singleFile}, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(dirPatterns) != 0 {
		t.Fatalf("got %d dirPatterns, want 0", len(dirPatterns))
	}
	if len(files) != 2 {
		t.Fatalf("got %d files, want 2: %v", len(files), files)
	}
}
