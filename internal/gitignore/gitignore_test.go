package gitignore

import (
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/go-git/go-git/v5/plumbing/filemode"
	"github.com/go-git/go-git/v5/plumbing/format/index"
)

// isolateGitConfig keeps the developer's ~/.gitconfig (and its
// core.excludesFile) out of the test.
func isolateGitConfig(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	return home
}

func writeFiles(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for p, c := range files {
		path := filepath.Join(root, filepath.FromSlash(p))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(c), 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

func setupRepo(t *testing.T) string {
	t.Helper()
	isolateGitConfig(t)
	d := t.TempDir()
	if err := os.Mkdir(filepath.Join(d, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFiles(t, d, map[string]string{
		".gitignore":          "node_modules\nsecret.md\n",
		"a.md":                "",
		"secret.md":           "",
		"node_modules/x/b.md": "",
	})
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
		if got := c.Ignored(filepath.Join(d, filepath.FromSlash(tt.rel))); got != tt.want {
			t.Errorf("Ignored(%q) = %v, want %v", tt.rel, got, tt.want)
		}
	}
}

func TestChecker_Rules(t *testing.T) {
	tests := []struct {
		name  string
		files map[string]string
		rel   string
		want  bool
	}{
		{
			name:  "nested .gitignore applies below its directory",
			files: map[string]string{"docs/.gitignore": "draft.md\n", "docs/draft.md": ""},
			rel:   "docs/draft.md",
			want:  true,
		},
		{
			name:  "nested .gitignore does not apply outside its directory",
			files: map[string]string{"docs/.gitignore": "draft.md\n", "draft.md": ""},
			rel:   "draft.md",
			want:  false,
		},
		{
			name:  "directory-only pattern does not match a file",
			files: map[string]string{".gitignore": "build/\n", "build": ""},
			rel:   "build",
			want:  false,
		},
		{
			name:  "negation cannot re-include a file inside an ignored directory",
			files: map[string]string{".gitignore": "build/\n!build/keep.md\n", "build/keep.md": ""},
			rel:   "build/keep.md",
			want:  true,
		},
		{
			name:  "negation re-includes a file",
			files: map[string]string{".gitignore": "*.md\n!keep.md\n", "keep.md": ""},
			rel:   "keep.md",
			want:  false,
		},
		{
			name:  "info/exclude is honored",
			files: map[string]string{".git/info/exclude": "local.md\n", "local.md": ""},
			rel:   "local.md",
			want:  true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isolateGitConfig(t)
			d := t.TempDir()
			if err := os.Mkdir(filepath.Join(d, ".git"), 0o755); err != nil {
				t.Fatal(err)
			}
			writeFiles(t, d, tt.files)
			if got := NewChecker(d).Ignored(filepath.Join(d, filepath.FromSlash(tt.rel))); got != tt.want {
				t.Errorf("Ignored(%q) = %v, want %v", tt.rel, got, tt.want)
			}
		})
	}
}

func TestChecker_TrackedFilesAreNotIgnored(t *testing.T) {
	d := setupRepo(t)
	writeFiles(t, d, map[string]string{
		".gitignore":     "build/\nsecret.md\n",
		"build/keep.md":  "",
		"build/other.md": "",
		"secret.md":      "",
	})
	idx := &index.Index{Version: 2}
	for _, name := range []string{"build/keep.md", "secret.md"} {
		idx.Entries = append(idx.Entries, &index.Entry{Name: name, Mode: filemode.Regular})
	}
	f, err := os.Create(filepath.Join(d, ".git", "index"))
	if err != nil {
		t.Fatal(err)
	}
	if err := index.NewEncoder(f).Encode(idx); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	c := NewChecker(d)
	tests := []struct {
		rel  string
		want bool
	}{
		{"secret.md", false},
		{"build", false},
		{"build/keep.md", false},
		{"build/other.md", true},
	}
	for _, tt := range tests {
		if got := c.Ignored(filepath.Join(d, filepath.FromSlash(tt.rel))); got != tt.want {
			t.Errorf("Ignored(%q) = %v, want %v", tt.rel, got, tt.want)
		}
	}
}

func TestChecker_SymlinkedGitignoreIsRefused(t *testing.T) {
	d := setupRepo(t)
	writeFiles(t, d, map[string]string{"rules": "docs/\n", "docs/a.md": ""})
	if err := os.Remove(filepath.Join(d, ".gitignore")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(d, "rules"), filepath.Join(d, ".gitignore")); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	if NewChecker(d).Ignored(filepath.Join(d, "docs", "a.md")) {
		t.Error("a symlinked .gitignore must not be read")
	}
}

func TestChecker_GlobalExcludesFile(t *testing.T) {
	home := isolateGitConfig(t)
	excludes := filepath.Join(home, "global-ignore")
	writeFiles(t, home, map[string]string{
		".gitconfig":    "[core]\n\texcludesFile = " + filepath.ToSlash(excludes) + "\n",
		"global-ignore": "*.tmp.md\n",
	})
	d := t.TempDir()
	if err := os.Mkdir(filepath.Join(d, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFiles(t, d, map[string]string{"x.tmp.md": ""})
	if !NewChecker(d).Ignored(filepath.Join(d, "x.tmp.md")) {
		t.Error("x.tmp.md should be ignored by core.excludesFile")
	}
}

func TestChecker_DefaultGlobalIgnore(t *testing.T) {
	home := isolateGitConfig(t)
	writeFiles(t, home, map[string]string{".config/git/ignore": "*.tmp.md\n"})
	d := t.TempDir()
	if err := os.Mkdir(filepath.Join(d, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFiles(t, d, map[string]string{"x.tmp.md": ""})
	if !NewChecker(d).Ignored(filepath.Join(d, "x.tmp.md")) {
		t.Error("x.tmp.md should be ignored by $XDG_CONFIG_HOME/git/ignore")
	}
}

func TestChecker_LinkedWorktree(t *testing.T) {
	isolateGitConfig(t)
	main := t.TempDir()
	gitDir := filepath.Join(main, ".git", "worktrees", "wt")
	writeFiles(t, main, map[string]string{
		".git/info/exclude":           "local.md\n",
		".git/worktrees/wt/commondir": "../..\n",
	})
	wt := t.TempDir()
	writeFiles(t, wt, map[string]string{
		".git":       "gitdir: " + gitDir + "\n",
		".gitignore": "dist\n",
		"dist/x.md":  "",
		"local.md":   "",
		"a.md":       "",
	})
	c := NewChecker(wt)
	if c == nil {
		t.Fatal("NewChecker(worktree) returned nil")
	}
	if !c.Ignored(filepath.Join(wt, "dist", "x.md")) {
		t.Error("dist/x.md should be ignored by the worktree's .gitignore")
	}
	if !c.Ignored(filepath.Join(wt, "local.md")) {
		t.Error("local.md should be ignored by the main repository's info/exclude")
	}
	if c.Ignored(filepath.Join(wt, "a.md")) {
		t.Error("a.md should not be ignored")
	}
}

func TestChecker_IgnoredRootIsNotFiltered(t *testing.T) {
	d := setupRepo(t)
	root := filepath.Join(d, "node_modules")
	if c := NewChecker(root); c != nil {
		t.Errorf("NewChecker(ignored root) = %v, want nil", c)
	}
}

func TestChecker_SubdirectoryRoot(t *testing.T) {
	d := setupRepo(t)
	writeFiles(t, d, map[string]string{"docs/secret.md": "", "docs/a.md": ""})
	c := NewChecker(filepath.Join(d, "docs"))
	if c == nil {
		t.Fatal("NewChecker(subdirectory) returned nil")
	}
	if !c.Ignored(filepath.Join(d, "docs", "secret.md")) {
		t.Error("docs/secret.md should be ignored by the repository root's .gitignore")
	}
	if c.Ignored(filepath.Join(d, "docs", "a.md")) {
		t.Error("docs/a.md should not be ignored")
	}
}

func TestChecker_NotARepository(t *testing.T) {
	isolateGitConfig(t)
	c := NewChecker(t.TempDir())
	if c.Ignored("/anything") {
		t.Error("nil checker must ignore nothing")
	}
}

func TestChecker_SubrepoInsideIgnoredParentDir(t *testing.T) {
	parent := setupRepo(t)
	subrepo := filepath.Join(parent, "node_modules", "subrepo")
	if err := os.MkdirAll(filepath.Join(subrepo, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFiles(t, subrepo, map[string]string{
		".gitignore": "dist\n",
		"keep.md":    "# keep",
		"dist/x.md":  "# ignored in subrepo",
	})

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
