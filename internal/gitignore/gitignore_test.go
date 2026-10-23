package gitignore

import (
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"testing"
)

func setupRepo(t *testing.T) string {
	t.Helper()
	d := t.TempDir()
	if err := exec.Command("git", "-C", d, "init", "-q").Run(); err != nil {
		t.Skip("git unavailable")
	}
	if err := os.MkdirAll(filepath.Join(d, "node_modules", "x"), 0o755); err != nil {
		t.Fatal(err)
	}
	for p, c := range map[string]string{
		".gitignore":          "node_modules\nsecret.md\n",
		"a.md":                "",
		"secret.md":           "",
		"node_modules/x/b.md": "",
	} {
		if err := os.WriteFile(filepath.Join(d, p), []byte(c), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return d
}

func TestChecker(t *testing.T) {
	d := setupRepo(t)
	c := NewChecker(d)
	tests := []struct {
		rel  string
		want bool
	}{
		{"a.md", false},
		{"secret.md", true},
		{"node_modules", true},
		{"node_modules/x/b.md", true},
	}
	for _, tt := range tests {
		if got := c.Ignored(filepath.Join(d, tt.rel)); got != tt.want {
			t.Errorf("Ignored(%q) = %v, want %v", tt.rel, got, tt.want)
		}
	}
}

func TestChecker_IgnoredRootIsNotFiltered(t *testing.T) {
	d := setupRepo(t)
	root := filepath.Join(d, "node_modules")
	if c := NewChecker(root); c != nil {
		t.Errorf("NewChecker(ignored root) = %v, want nil", c)
	}
}

func TestChecker_NotARepository(t *testing.T) {
	c := NewChecker(t.TempDir())
	if c.Ignored("/anything") {
		t.Error("nil checker must ignore nothing")
	}
}

func TestIsIgnored(t *testing.T) {
	d := setupRepo(t)
	if !IsIgnored(filepath.Join(d, "node_modules", "x", "b.md")) {
		t.Error("file below ignored directory should be ignored")
	}
	if !IsIgnored(filepath.Join(d, "node_modules")) {
		t.Error("ignored directory should be ignored")
	}
	if IsIgnored(filepath.Join(d, "a.md")) {
		t.Error("a.md should not be ignored")
	}
}

func TestChecker_SubrepoInsideIgnoredParentDir(t *testing.T) {
	parent := setupRepo(t)
	subrepo := filepath.Join(parent, "node_modules", "subrepo")
	if err := os.MkdirAll(filepath.Join(subrepo, "dist"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := exec.Command("git", "-C", subrepo, "init", "-q").Run(); err != nil {
		t.Fatal(err)
	}
	for p, c := range map[string]string{
		".gitignore": "dist\n",
		"keep.md":    "# keep",
		"dist/x.md":  "# ignored in subrepo",
	} {
		if err := os.WriteFile(filepath.Join(subrepo, p), []byte(c), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	if IsIgnored(subrepo) {
		t.Fatal("subrepo root should not be treated as ignored by parent repo")
	}
	c := NewChecker(subrepo)
	if c == nil {
		t.Fatal("NewChecker(subrepo) returned nil")
	}
	if c.Ignored(filepath.Join(subrepo, "keep.md")) {
		t.Error("keep.md should not be ignored")
	}
	if !c.Ignored(filepath.Join(subrepo, "dist", "x.md")) {
		t.Error("dist/x.md should be ignored by subrepo .gitignore")
	}
}

func TestChecker_DirFS(t *testing.T) {
	d := setupRepo(t)
	c := NewChecker(d)
	var visited []string
	err := fs.WalkDir(c.DirFS(d), ".", func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		visited = append(visited, path)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(visited, "a.md") {
		t.Errorf("visited = %v, want a.md", visited)
	}
	for _, excluded := range []string{"secret.md", "node_modules", "node_modules/x/b.md"} {
		if slices.Contains(visited, excluded) {
			t.Errorf("visited = %v, must not contain %s", visited, excluded)
		}
	}
}
