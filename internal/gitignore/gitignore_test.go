package gitignore

import (
	"os"
	"os/exec"
	"path/filepath"
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
		if err := os.WriteFile(filepath.Join(d, p), []byte(c), 0o644); err != nil {
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
	if IsIgnored(filepath.Join(d, "a.md")) {
		t.Error("a.md should not be ignored")
	}
}
