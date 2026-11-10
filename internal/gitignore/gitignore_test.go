package gitignore

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

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
			name:  "trailing /** does not ignore the directory itself",
			files: map[string]string{".gitignore": "build/**\n!build/keep.md\n", "build/keep.md": ""},
			rel:   "build/keep.md",
			want:  false,
		},
		{
			name:  "trailing /** ignores everything inside",
			files: map[string]string{".gitignore": "build/**\n", "build/sub/x.md": ""},
			rel:   "build/sub/x.md",
			want:  true,
		},
		{
			name:  "leading ** backtracks over repeated directory names",
			files: map[string]string{".gitignore": "**/docs/private.md\n", "docs/docs/private.md": ""},
			rel:   "docs/docs/private.md",
			want:  true,
		},
		{
			name:  "inner ** backtracks over repeated directory names",
			files: map[string]string{".gitignore": "a/**/b/c.md\n", "a/b/x/b/c.md": ""},
			rel:   "a/b/x/b/c.md",
			want:  true,
		},
		{
			name:  "** pattern is anchored to its .gitignore",
			files: map[string]string{"sub/.gitignore": "**/private.md\n", "private.md": ""},
			rel:   "private.md",
			want:  false,
		},
		{
			name:  "directory-only ** pattern ignores files below the directory",
			files: map[string]string{".gitignore": "**/cache/\n", "x/cache/a.md": ""},
			rel:   "x/cache/a.md",
			want:  true,
		},
		{
			name:  "braces are literal",
			files: map[string]string{".gitignore": "**/{draft,private}.md\n", "{draft,private}.md": ""},
			rel:   "{draft,private}.md",
			want:  true,
		},
		{
			name:  "braces are not alternation",
			files: map[string]string{".gitignore": "**/{draft,private}.md\n", "draft.md": ""},
			rel:   "draft.md",
			want:  false,
		},
		{
			name:  "re-including a directory does not re-include its files",
			files: map[string]string{".gitignore": "*\n!docs/\n", "docs/private.md": ""},
			rel:   "docs/private.md",
			want:  true,
		},
		{
			name:  "re-included directory itself is not ignored",
			files: map[string]string{".gitignore": "*\n!docs/\n", "docs/private.md": ""},
			rel:   "docs",
			want:  false,
		},
		{
			name:  "consecutive stars inside a segment act as one",
			files: map[string]string{".gitignore": "docs/**.md\n", "docs/private.md": ""},
			rel:   "docs/private.md",
			want:  true,
		},
		{
			name:  "negated character class excludes its members",
			files: map[string]string{".gitignore": "[!a]*.md\n", "a.md": ""},
			rel:   "a.md",
			want:  false,
		},
		{
			name:  "negated character class matches others",
			files: map[string]string{".gitignore": "[!a]*.md\n", "b.md": ""},
			rel:   "b.md",
			want:  true,
		},
		{
			name: "escaped trailing space is kept",
			// "x " itself is not created, since Windows drops trailing
			// spaces from file names.
			files: map[string]string{".gitignore": "x\\ \n"},
			rel:   "x ",
			want:  true,
		},
		{
			name:  "core.ignoreCase folds case",
			files: map[string]string{".git/config": "[core]\n\tignorecase = true\n", ".gitignore": "Secret.md\n", "secret.md": ""},
			rel:   "secret.md",
			want:  true,
		},
		{
			name:  "case matters without core.ignoreCase",
			files: map[string]string{".gitignore": "Secret.md\n", "secret.md": ""},
			rel:   "secret.md",
			want:  false,
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
		// The negation cannot bring back build/other.md, because its parent
		// stays excluded although it holds the tracked build/keep.md.
		".gitignore":     "build/\nsecret.md\n!build/other.md\n",
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

// writeIndex writes an index tracking names into d/.git/index, after letting
// mutate rewrite its bytes.
func writeIndex(t *testing.T, d string, names []string, mutate func([]byte) []byte) {
	t.Helper()
	idx := &index.Index{Version: 2}
	for _, name := range names {
		idx.Entries = append(idx.Entries, &index.Entry{Name: name, Mode: filemode.Regular})
	}
	var buf bytes.Buffer
	if err := index.NewEncoder(&buf).Encode(idx); err != nil {
		t.Fatal(err)
	}
	b := buf.Bytes()
	if mutate != nil {
		b = mutate(b)
	}
	if err := os.WriteFile(filepath.Join(d, ".git", "index"), b, 0o600); err != nil {
		t.Fatal(err)
	}
}

// withExtension inserts an extension with the given signature before the
// trailing checksum.
func withExtension(sig string) func([]byte) []byte {
	return func(b []byte) []byte {
		body := slices.Clone(b[:len(b)-20])
		body = append(body, sig...)
		body = append(body, 0, 0, 0, 0)
		return append(body, b[len(b)-20:]...)
	}
}

func TestChecker_SplitIndexInDirectoryWithGlobMetacharacters(t *testing.T) {
	isolateGitConfig(t)
	d := filepath.Join(t.TempDir(), "repo[1]")
	writeFiles(t, d, map[string]string{
		".git/sharedindex.0123": "",
		".gitignore":            "secret.md\n",
		"secret.md":             "",
	})
	writeIndex(t, d, []string{"secret.md"}, withExtension("link"))
	if c := NewChecker(d); c != nil {
		t.Error("a split index must disable filtering whatever the repository path looks like")
	}
}

func TestChecker_IndexDecodeFailures(t *testing.T) {
	tests := []struct {
		name   string
		mutate func([]byte) []byte
		shared bool
		// wantFilter reports whether filtering stays on. When it does, the
		// tracked secret.md must also be recognized as tracked.
		wantFilter bool
	}{
		{
			name: "skipHash leaves an all-zero checksum",
			mutate: func(b []byte) []byte {
				clear(b[len(b)-20:])
				return b
			},
			wantFilter: true,
		},
		{
			name:       "sparse index extension",
			mutate:     withExtension("sdir"),
			wantFilter: true,
		},
		{
			name:       "split index",
			mutate:     withExtension("link"),
			shared:     true,
			wantFilter: false,
		},
		{
			name: "corrupt checksum",
			mutate: func(b []byte) []byte {
				b[len(b)-1] ^= 0xff
				return b
			},
			wantFilter: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := setupRepo(t)
			writeIndex(t, d, []string{"secret.md"}, tt.mutate)
			if tt.shared {
				writeFiles(t, d, map[string]string{".git/sharedindex.0123": ""})
			}
			c := NewChecker(d)
			if got := c != nil; got != tt.wantFilter {
				t.Fatalf("filtering enabled = %v, want %v", got, tt.wantFilter)
			}
			if c.Ignored(filepath.Join(d, "secret.md")) {
				t.Error("tracked secret.md must not be ignored")
			}
		})
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

func TestChecker_ExcludesFileFromConfig(t *testing.T) {
	tests := []struct {
		name string
		// configs maps a path relative to the home directory, or to the
		// repository when prefixed with "repo:", to its content. "EXCLUDES"
		// is replaced with the excludes file path, and "REPO" with the
		// repository path.
		configs map[string]string
	}{
		{
			name:    "include",
			configs: map[string]string{".gitconfig": "[include]\n\tpath = ~/.gitconfig.local\n", ".gitconfig.local": "[core]\n\texcludesFile = EXCLUDES\n"},
		},
		{
			name:    "relative include",
			configs: map[string]string{".gitconfig": "[include]\n\tpath = conf/extra\n", "conf/extra": "[core]\n\texcludesFile = EXCLUDES\n"},
		},
		{
			name:    "includeIf gitdir",
			configs: map[string]string{".gitconfig": "[includeIf \"gitdir:REPO/\"]\n\tpath = ~/work.gitconfig\n", "work.gitconfig": "[core]\n\texcludesFile = EXCLUDES\n"},
		},
		{
			name:    "include before a later core setting",
			configs: map[string]string{".gitconfig": "[include]\n\tpath = ~/other.gitconfig\n[core]\n\texcludesFile = EXCLUDES\n", "other.gitconfig": "[core]\n\texcludesFile = ~/unused\n"},
		},
		{
			name:    "core setting before a later include",
			configs: map[string]string{".gitconfig": "[core]\n\texcludesFile = ~/unused\n[include]\n\tpath = ~/other.gitconfig\n", "other.gitconfig": "[core]\n\texcludesFile = EXCLUDES\n"},
		},
		{
			name:    "repository config",
			configs: map[string]string{"repo:.git/config": "[core]\n\texcludesFile = EXCLUDES\n"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			home := isolateGitConfig(t)
			d := t.TempDir()
			excludes := filepath.ToSlash(filepath.Join(home, "global-ignore"))
			writeFiles(t, home, map[string]string{"global-ignore": "*.tmp.md\n"})
			writeFiles(t, d, map[string]string{".git/HEAD": "", "x.tmp.md": ""})
			for p, c := range tt.configs {
				c = strings.ReplaceAll(c, "EXCLUDES", excludes)
				c = strings.ReplaceAll(c, "REPO", filepath.ToSlash(d))
				if rel, ok := strings.CutPrefix(p, "repo:"); ok {
					writeFiles(t, d, map[string]string{rel: c})
				} else {
					writeFiles(t, home, map[string]string{p: c})
				}
			}
			if !NewChecker(d).Ignored(filepath.Join(d, "x.tmp.md")) {
				t.Error("x.tmp.md should be ignored by core.excludesFile")
			}
		})
	}
}

func TestChecker_IncludeIfRelativeDirPrefix(t *testing.T) {
	for _, cond := range []string{"gitdir:~/work/", "gitdir:./work/"} {
		t.Run(cond, func(t *testing.T) {
			home := isolateGitConfig(t)
			writeFiles(t, home, map[string]string{
				".gitconfig":     "[includeIf \"" + cond + "\"]\n\tpath = ~/work.gitconfig\n",
				"work.gitconfig": "[core]\n\texcludesFile = " + filepath.ToSlash(filepath.Join(home, "global-ignore")) + "\n",
				"global-ignore":  "*.tmp.md\n",
			})
			d := filepath.Join(home, "work", "repo")
			writeFiles(t, d, map[string]string{".git/HEAD": "", "x.tmp.md": ""})
			if !NewChecker(d).Ignored(filepath.Join(d, "x.tmp.md")) {
				t.Errorf("%s should cover a repository below it", cond)
			}
		})
	}
}

func TestChecker_RelativeExcludesFile(t *testing.T) {
	isolateGitConfig(t)
	d := t.TempDir()
	writeFiles(t, d, map[string]string{
		".git/config":  "[core]\n\texcludesFile = ignore.rules\n",
		"ignore.rules": "*.tmp.md\n",
		"x.tmp.md":     "",
	})
	t.Chdir(t.TempDir())
	if !NewChecker(d).Ignored(filepath.Join(d, "x.tmp.md")) {
		t.Error("a relative core.excludesFile should be read from the checker root")
	}
}

func TestChecker_SelfIncludingConfigFinishes(t *testing.T) {
	isolateGitConfig(t)
	d := t.TempDir()
	cfg := "[core]\n\texcludesFile = " + filepath.ToSlash(filepath.Join(d, "rules")) + "\n"
	for range 10 {
		cfg += "[include]\n\tpath = config\n"
	}
	writeFiles(t, d, map[string]string{".git/config": cfg, "rules": "*.tmp.md\n", "x.tmp.md": ""})
	done := make(chan bool, 1)
	go func() { done <- NewChecker(d).Ignored(filepath.Join(d, "x.tmp.md")) }()
	select {
	case ignored := <-done:
		if !ignored {
			t.Error("x.tmp.md should be ignored by core.excludesFile")
		}
	case <-time.After(10 * time.Second):
		t.Fatal("loading a self-including config did not finish")
	}
}

func TestChecker_IncludeIfOtherGitDir(t *testing.T) {
	home := isolateGitConfig(t)
	writeFiles(t, home, map[string]string{
		".gitconfig":     "[includeIf \"gitdir:/nowhere/\"]\n\tpath = ~/work.gitconfig\n",
		"work.gitconfig": "[core]\n\texcludesFile = " + filepath.ToSlash(filepath.Join(home, "global-ignore")) + "\n",
		"global-ignore":  "*.tmp.md\n",
	})
	d := t.TempDir()
	writeFiles(t, d, map[string]string{".git/HEAD": "", "x.tmp.md": ""})
	if NewChecker(d).Ignored(filepath.Join(d, "x.tmp.md")) {
		t.Error("an includeIf for another gitdir must not apply")
	}
}

func TestChecker_EmptyExcludesFileDisablesIt(t *testing.T) {
	tests := []struct {
		name    string
		configs map[string]string
	}{
		{
			name: "repository config clears the user setting",
			configs: map[string]string{
				".gitconfig":       "[core]\n\texcludesFile = ~/global-ignore\n",
				"repo:.git/config": "[core]\n\texcludesFile =\n",
			},
		},
		{
			name:    "empty value skips the default file",
			configs: map[string]string{".gitconfig": "[core]\n\texcludesFile =\n", ".config/git/ignore": "*.tmp.md\n"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			home := isolateGitConfig(t)
			d := t.TempDir()
			writeFiles(t, home, map[string]string{"global-ignore": "*.tmp.md\n"})
			writeFiles(t, d, map[string]string{".git/HEAD": "", "x.tmp.md": ""})
			for p, c := range tt.configs {
				if rel, ok := strings.CutPrefix(p, "repo:"); ok {
					writeFiles(t, d, map[string]string{rel: c})
				} else {
					writeFiles(t, home, map[string]string{p: c})
				}
			}
			if NewChecker(d).Ignored(filepath.Join(d, "x.tmp.md")) {
				t.Error("an empty core.excludesFile must disable the excludes file")
			}
		})
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

func TestChecker_SymlinkedRootOutsideRepository(t *testing.T) {
	d := setupRepo(t)
	writeFiles(t, d, map[string]string{"docs/secret.md": "", "docs/a.md": ""})
	link := filepath.Join(t.TempDir(), "docs")
	if err := os.Symlink(filepath.Join(d, "docs"), link); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	c := NewChecker(link)
	if c == nil {
		t.Fatal("NewChecker(symlink into repository) returned nil")
	}
	if !c.Ignored(filepath.Join(link, "secret.md")) {
		t.Error("secret.md should be ignored through the symlinked root")
	}
	if c.Ignored(filepath.Join(link, "a.md")) {
		t.Error("a.md should not be ignored")
	}
	var visited []string
	err := fs.WalkDir(c.DirFS(link), ".", func(path string, _ fs.DirEntry, err error) error {
		visited = append(visited, path)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	if slices.Contains(visited, "secret.md") || !slices.Contains(visited, "a.md") {
		t.Errorf("visited = %v, want a.md without secret.md", visited)
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
