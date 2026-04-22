package cmd

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/k1LoW/mo/internal/server"
)

func TestRun_UnwatchWithWatch(t *testing.T) {
	unwatchMode = true
	watchMode = true
	defer func() {
		unwatchMode = false
		watchMode = false
	}()

	err := run(rootCmd, []string{"**/*.md"})
	if err == nil {
		t.Fatal("run should return error when --unwatch and --watch are both specified")
	}
	want := "cannot use --unwatch with --watch"
	if err.Error() != want {
		t.Fatalf("got error %q, want %q", err.Error(), want)
	}
}

func TestRun_UnwatchWithoutArgs(t *testing.T) {
	unwatchMode = true
	defer func() { unwatchMode = false }()

	err := run(rootCmd, nil)
	if err == nil {
		t.Fatal("run should return error when --unwatch has no arguments")
	}
	want := "--unwatch requires a glob pattern or directory argument"
	if err.Error() != want {
		t.Fatalf("got error %q, want %q", err.Error(), want)
	}
}

func TestRun_UnwatchWithFileArgs(t *testing.T) {
	f := filepath.Join(t.TempDir(), "test.md")
	writeTestFile(t, f, []byte("# Test"))

	unwatchMode = true
	defer func() { unwatchMode = false }()

	err := run(rootCmd, []string{f})
	if err == nil {
		t.Fatal("run should return error when --unwatch is given file arguments")
	}
	if !strings.Contains(err.Error(), "not individual files") {
		t.Fatalf("got error %q, want hint about individual files", err.Error())
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
		watchMode = true
		defer func() {
			closeFiles = false
			watchMode = false
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

func TestRun_Watch(t *testing.T) {
	t.Run("no args errors", func(t *testing.T) {
		watchMode = true
		defer func() { watchMode = false }()

		err := run(rootCmd, nil)
		if err == nil {
			t.Fatal("run should return error when --watch has no pattern or directory argument")
		}
		if !strings.Contains(err.Error(), "requires a glob pattern or directory argument") {
			t.Fatalf("got error %q, want 'requires a glob pattern or directory argument'", err.Error())
		}
	})

	t.Run("only file args hints shell expansion", func(t *testing.T) {
		f1 := filepath.Join(t.TempDir(), "a.md")
		writeTestFile(t, f1, []byte("# A"))
		f2 := filepath.Join(t.TempDir(), "b.md")
		writeTestFile(t, f2, []byte("# B"))

		watchMode = true
		defer func() { watchMode = false }()

		err := run(rootCmd, []string{f1, f2})
		if err == nil {
			t.Fatal("run should return error when --watch is given only regular file arguments")
		}
		if !strings.Contains(err.Error(), "shell may have expanded") {
			t.Fatalf("error should hint shell expansion, got %q", err.Error())
		}
	})

	t.Run("non-existent arg returns file not found", func(t *testing.T) {
		watchMode = true
		defer func() { watchMode = false }()

		err := run(rootCmd, []string{"nonexistent.md"})
		if err == nil {
			t.Fatal("run should return error for non-existent file")
		}
		if !strings.Contains(err.Error(), "file not found") {
			t.Fatalf("got error %q, want file not found error", err.Error())
		}
	})
}

func TestRun_RecursiveRequiresArgs(t *testing.T) {
	recursive = true
	defer func() { recursive = false }()

	err := run(rootCmd, nil)
	if err == nil {
		t.Fatal("run should return error when --recursive is used without any argument")
	}
	want := "--recursive (-R) requires a directory argument"
	if err.Error() != want {
		t.Fatalf("got error %q, want %q", err.Error(), want)
	}
}

func TestResolveUnwatchArgs_GlobPattern(t *testing.T) {
	patterns, err := resolveUnwatchArgs([]string{"**/*.md", "docs/*.md"}, false, "", "default")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
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

func TestResolveUnwatchArgs_Directory(t *testing.T) {
	dir := t.TempDir()

	patterns, err := resolveUnwatchArgs([]string{dir}, false, "", "default")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(patterns) != 1 {
		t.Fatalf("got %d patterns, want 1", len(patterns))
	}
	want := filepath.Join(dir, "*.md")
	if patterns[0] != want {
		t.Errorf("got pattern %q, want %q", patterns[0], want)
	}
}

func TestResolveUnwatchArgs_FileReturnsError(t *testing.T) {
	f := filepath.Join(t.TempDir(), "test.md")
	writeTestFile(t, f, []byte("# Test"))

	_, err := resolveUnwatchArgs([]string{f}, false, "", "default")
	if err == nil {
		t.Fatal("expected error for file argument")
	}
	if !strings.Contains(err.Error(), "not individual files") {
		t.Fatalf("got error %q, want hint about individual files", err.Error())
	}
}

func TestResolveUnwatchArgs_RecursiveDirectory(t *testing.T) {
	dir := t.TempDir()

	// Set up a mock server that returns patterns for the group.
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := statusResponse{
			Groups: []struct {
				Name  string `json:"name"`
				Files []struct {
					Name string `json:"name"`
					ID   string `json:"id"`
					Path string `json:"path"`
				} `json:"files"`
				Patterns []string `json:"patterns,omitempty"`
			}{
				{
					Name: "default",
					Patterns: []string{
						filepath.Join(dir, "*.md"),
						filepath.Join(dir, "sub", "*.md"),
						filepath.Join(dir, "**/*.md"),
						"/other/path/*.md",
					},
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp) //nolint:errcheck
	})
	ts := httptest.NewServer(handler)
	defer ts.Close()
	addr := strings.TrimPrefix(ts.URL, "http://")

	patterns, err := resolveUnwatchArgs([]string{dir}, true, addr, "default")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(patterns) != 3 {
		t.Fatalf("got %d patterns, want 3: %v", len(patterns), patterns)
	}
	// Should NOT include /other/path/*.md
	for _, p := range patterns {
		if !strings.HasPrefix(p, dir) {
			t.Errorf("unexpected pattern %q not under %s", p, dir)
		}
	}
}

func TestResolveUnwatchArgs_RecursiveNoMatch(t *testing.T) {
	dir := t.TempDir()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := statusResponse{
			Groups: []struct {
				Name  string `json:"name"`
				Files []struct {
					Name string `json:"name"`
					ID   string `json:"id"`
					Path string `json:"path"`
				} `json:"files"`
				Patterns []string `json:"patterns,omitempty"`
			}{
				{
					Name:     "default",
					Patterns: []string{"/other/path/*.md"},
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp) //nolint:errcheck
	})
	ts := httptest.NewServer(handler)
	defer ts.Close()
	addr := strings.TrimPrefix(ts.URL, "http://")

	_, err := resolveUnwatchArgs([]string{dir}, true, addr, "default")
	if err == nil {
		t.Fatal("expected error when no patterns match under directory")
	}
	if !strings.Contains(err.Error(), "no watched patterns found under") {
		t.Fatalf("got error %q, want 'no watched patterns found under'", err.Error())
	}
}

func TestResolveUnwatchArgs_RecursiveDeletedDirectory(t *testing.T) {
	// Use a path that does not exist on disk.
	deletedDir := filepath.Join(t.TempDir(), "deleted")

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := statusResponse{
			Groups: []struct {
				Name  string `json:"name"`
				Files []struct {
					Name string `json:"name"`
					ID   string `json:"id"`
					Path string `json:"path"`
				} `json:"files"`
				Patterns []string `json:"patterns,omitempty"`
			}{
				{
					Name: "default",
					Patterns: []string{
						filepath.Join(deletedDir, "*.md"),
						filepath.Join(deletedDir, "**/*.md"),
					},
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp) //nolint:errcheck
	})
	ts := httptest.NewServer(handler)
	defer ts.Close()
	addr := strings.TrimPrefix(ts.URL, "http://")

	patterns, err := resolveUnwatchArgs([]string{deletedDir}, true, addr, "default")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(patterns) != 2 {
		t.Fatalf("got %d patterns, want 2: %v", len(patterns), patterns)
	}
}

func TestResolveUnwatchArgs_NonRecursiveDeletedDirectory(t *testing.T) {
	deletedDir := filepath.Join(t.TempDir(), "deleted")

	_, err := resolveUnwatchArgs([]string{deletedDir}, false, "", "default")
	if err == nil {
		t.Fatal("expected error for non-existent directory without -R")
	}
	if !strings.Contains(err.Error(), "path not found") {
		t.Fatalf("got error %q, want 'path not found'", err.Error())
	}
}

func TestResolveUnwatchArgs_RecursiveGroupNotFound(t *testing.T) {
	dir := t.TempDir()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := statusResponse{
			Groups: []struct {
				Name  string `json:"name"`
				Files []struct {
					Name string `json:"name"`
					ID   string `json:"id"`
					Path string `json:"path"`
				} `json:"files"`
				Patterns []string `json:"patterns,omitempty"`
			}{
				{
					Name:     "other",
					Patterns: []string{"/other/*.md"},
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp) //nolint:errcheck
	})
	ts := httptest.NewServer(handler)
	defer ts.Close()
	addr := strings.TrimPrefix(ts.URL, "http://")

	_, err := resolveUnwatchArgs([]string{dir}, true, addr, "default")
	if err == nil {
		t.Fatal("expected error when group does not exist")
	}
	if !strings.Contains(err.Error(), "group") || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("got error %q, want group not found error", err.Error())
	}
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

	files, patterns, err := resolveArgs([]string{dir}, false, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(patterns) != 0 {
		t.Fatalf("got %d patterns, want 0", len(patterns))
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

func TestResolveArgs_DirectoryNaturalOrder(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"i1.md", "i2.md", "i10.md", "i11.md"} {
		writeTestFile(t, filepath.Join(dir, name), []byte("# "+name))
	}

	files, _, err := resolveArgs([]string{dir}, false, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := []string{
		filepath.Join(dir, "i1.md"),
		filepath.Join(dir, "i2.md"),
		filepath.Join(dir, "i10.md"),
		filepath.Join(dir, "i11.md"),
	}
	if len(files) != len(want) {
		t.Fatalf("got %d files, want %d: %v", len(files), len(want), files)
	}
	for i := range want {
		if files[i] != want[i] {
			t.Errorf("files[%d] = %q, want %q", i, files[i], want[i])
		}
	}
}

func TestResolveArgs_DirectoryWithWatch(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, "a.md"), []byte("# A"))

	files, patterns, err := resolveArgs([]string{dir}, true, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(files) != 0 {
		t.Fatalf("got %d files, want 0", len(files))
	}
	if len(patterns) != 1 {
		t.Fatalf("got %d patterns, want 1", len(patterns))
	}
	want := filepath.Join(dir, "*.md")
	if patterns[0] != want {
		t.Errorf("got pattern %q, want %q", patterns[0], want)
	}
}

func TestResolveArgs_DirectoryWithWatchRecursive(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, "a.md"), []byte("# A"))

	files, patterns, err := resolveArgs([]string{dir}, true, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(files) != 0 {
		t.Fatalf("got %d files, want 0", len(files))
	}
	if len(patterns) != 1 {
		t.Fatalf("got %d patterns, want 1", len(patterns))
	}
	want := filepath.Join(dir, "**/*.md")
	if patterns[0] != want {
		t.Errorf("got pattern %q, want %q", patterns[0], want)
	}
}

func TestResolveArgs_DirectoryRecursive(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, "a.md"), []byte("# A"))
	sub := filepath.Join(dir, "sub")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(sub, "b.md"), []byte("# B"))
	writeTestFile(t, filepath.Join(sub, "c.txt"), []byte("text"))

	files, patterns, err := resolveArgs([]string{dir}, false, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(patterns) != 0 {
		t.Fatalf("got %d patterns, want 0", len(patterns))
	}
	if len(files) != 2 {
		t.Fatalf("got %d files, want 2: %v", len(files), files)
	}
	wantNested := filepath.Join(sub, "b.md")
	if !slices.Contains(files, wantNested) {
		t.Errorf("recursive expansion missed nested file %q in %v", wantNested, files)
	}
}

func TestResolveArgs_GlobPositional_WatchMode(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, "a.md"), []byte("# A"))
	pattern := filepath.Join(dir, "*.md")

	files, patterns, err := resolveArgs([]string{pattern}, true, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(files) != 0 {
		t.Fatalf("got %d files, want 0", len(files))
	}
	if len(patterns) != 1 {
		t.Fatalf("got %d patterns, want 1: %v", len(patterns), patterns)
	}
	if !filepath.IsAbs(patterns[0]) {
		t.Errorf("pattern %q is not absolute", patterns[0])
	}
}

func TestResolveArgs_GlobPositional_NonWatch(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, "a.md"), []byte("# A"))
	writeTestFile(t, filepath.Join(dir, "b.md"), []byte("# B"))
	pattern := filepath.Join(dir, "*.md")

	files, patterns, err := resolveArgs([]string{pattern}, false, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(patterns) != 0 {
		t.Fatalf("got %d patterns, want 0", len(patterns))
	}
	if len(files) != 2 {
		t.Fatalf("got %d files, want 2: %v", len(files), files)
	}
}

func TestResolveArgs_EmptyDirectory(t *testing.T) {
	dir := t.TempDir()

	_, _, err := resolveArgs([]string{dir}, false, false)
	if err == nil {
		t.Fatal("expected error for empty directory")
	}
	if !strings.Contains(err.Error(), "no .md files") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestStdinName(t *testing.T) {
	tests := []struct {
		name    string
		content string
	}{
		{"simple content", "# Hello World"},
		{"empty content", ""},
		{"japanese content", "# 日本語テスト"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stdinName(tt.content)
			if !strings.HasPrefix(got, "stdin-") {
				t.Errorf("stdinName(%q) = %q, want prefix 'stdin-'", tt.content, got)
			}
			if !strings.HasSuffix(got, ".md") {
				t.Errorf("stdinName(%q) = %q, want suffix '.md'", tt.content, got)
			}
			// "stdin-" (6) + hash (7) + ".md" (3) = 16
			if len(got) != 16 {
				t.Errorf("stdinName(%q) = %q (len %d), want len 16", tt.content, got, len(got))
			}
		})
	}

	t.Run("same content produces same name", func(t *testing.T) {
		a := stdinName("# Hello")
		b := stdinName("# Hello")
		if a != b {
			t.Errorf("same content gave different names: %q vs %q", a, b)
		}
	})

	t.Run("different content produces different name", func(t *testing.T) {
		a := stdinName("# A")
		b := stdinName("# B")
		if a == b {
			t.Errorf("different content gave same name: %q", a)
		}
	})
}

func TestReadStdin(t *testing.T) {
	t.Run("reads piped content", func(t *testing.T) {
		r, w, err := os.Pipe()
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { r.Close() })
		content := "# Test Document\n\nHello world."
		go func() {
			defer w.Close()
			if _, err := w.Write([]byte(content)); err != nil {
				t.Errorf("failed to write to pipe: %v", err)
			}
		}()

		name, got, err := readStdin(r)
		if err != nil {
			t.Fatal(err)
		}
		if got != content {
			t.Errorf("got content %q, want %q", got, content)
		}
		if !strings.HasPrefix(name, "stdin-") || !strings.HasSuffix(name, ".md") {
			t.Errorf("got name %q, want stdin-<hash>.md format", name)
		}
	})

	t.Run("exceeds max size", func(t *testing.T) {
		r, w, err := os.Pipe()
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { r.Close() })
		go func() {
			defer w.Close()
			// Write just over the limit
			buf := make([]byte, maxStdinSize+1)
			if _, err := w.Write(buf); err != nil {
				t.Errorf("failed to write to pipe: %v", err)
			}
		}()

		_, _, err = readStdin(r)
		if err == nil {
			t.Fatal("expected error for oversized stdin")
		}
		if !strings.Contains(err.Error(), "too large") {
			t.Errorf("got error %q, want 'too large' message", err.Error())
		}
	})

	t.Run("empty stdin", func(t *testing.T) {
		r, w, err := os.Pipe()
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { r.Close() })
		w.Close()

		name, got, err := readStdin(r)
		if err != nil {
			t.Fatal(err)
		}
		if got != "" {
			t.Errorf("got content %q, want empty", got)
		}
		if !strings.HasPrefix(name, "stdin-") {
			t.Errorf("got name %q, want stdin- prefix", name)
		}
	})
}

func TestResolveArgs_EmptyDirectoryWithWatch(t *testing.T) {
	dir := t.TempDir()

	files, patterns, err := resolveArgs([]string{dir}, true, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(files) != 0 {
		t.Fatalf("got %d files, want 0", len(files))
	}
	if len(patterns) != 1 {
		t.Fatalf("got %d patterns, want 1", len(patterns))
	}
}

func TestResolveArgs_MixedFilesAndDirs(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, "a.md"), []byte("# A"))

	singleFile := filepath.Join(t.TempDir(), "standalone.md")
	writeTestFile(t, singleFile, []byte("# Standalone"))

	files, patterns, err := resolveArgs([]string{dir, singleFile}, false, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(patterns) != 0 {
		t.Fatalf("got %d patterns, want 0", len(patterns))
	}
	if len(files) != 2 {
		t.Fatalf("got %d files, want 2: %v", len(files), files)
	}
}
