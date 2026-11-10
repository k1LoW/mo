// Package gitignore answers "is this path ignored by git?" by evaluating
// .gitignore, .git/info/exclude and the core.excludesFile of the user and
// system git config with go-git's pattern matcher.
//
// It reads those files directly rather than running the git command, so mo
// does not depend on git being installed, and browsing a directory never
// executes anything configured by that directory's repository (for example
// core.fsmonitor).
package gitignore

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/go-git/go-git/v5/plumbing/format/config"
	"github.com/go-git/go-git/v5/plumbing/format/gitignore"
	"github.com/go-git/go-git/v5/plumbing/format/index"
)

// Checker reports which paths below a root directory are ignored by git. A
// nil *Checker ignores nothing.
//
// The .gitignore of each directory is read the first time a path below it is
// queried and cached for the lifetime of the Checker, so edits made after
// that are not seen.
type Checker struct {
	root string // directory the checker was created for
	repo string // working tree root of the repository containing root
	base []gitignore.Pattern
	// tracked holds the slash-separated paths of the files in the index and
	// of every directory above them. git never reports a tracked path as
	// ignored, and does not skip a directory that still holds tracked files.
	tracked map[string]struct{}

	mu   sync.Mutex
	dirs map[string][]gitignore.Pattern
}

// NewChecker returns a Checker for the paths below root.
//
// It returns nil (nothing is ignored) when root is not in a git repository,
// or root itself is ignored. Pointing mo at an ignored directory explicitly
// means the user wants its contents.
func NewChecker(root string) *Checker {
	root, err := filepath.Abs(root)
	if err != nil {
		return nil
	}
	repo, gitDir, ok := findRepo(root)
	if !ok {
		return nil
	}
	c := &Checker{
		root:    root,
		repo:    repo,
		base:    loadBasePatterns(gitDir),
		tracked: loadTracked(gitDir),
		dirs:    make(map[string][]gitignore.Pattern),
	}
	if root != repo && c.match(root, true, repo) {
		return nil
	}
	return c
}

// Ignored reports whether path, or one of its parent directories below the
// root, is ignored.
func (c *Checker) Ignored(path string) bool {
	if c == nil {
		return false
	}
	path = filepath.Clean(path)
	info, err := os.Lstat(path)
	return c.match(path, err == nil && info.IsDir(), c.root)
}

// DirFS returns an fs.FS rooted at dir that hides entries ignored by c.
// When c is nil, it returns os.DirFS(dir) directly.
func (c *Checker) DirFS(dir string) fs.FS {
	fsys := os.DirFS(dir)
	if c == nil {
		return fsys
	}
	return &ignoreFS{FS: fsys, base: dir, checker: c}
}

// match reports whether path, or one of its parent directories strictly
// below from, is ignored. Parents are checked before path itself because git
// never looks inside an ignored directory, so a negation pattern cannot bring
// back a file below one, while go-git's Matcher alone would let it.
func (c *Checker) match(path string, isDir bool, from string) bool {
	rel, err := filepath.Rel(c.repo, path)
	if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return false
	}
	segs := strings.Split(rel, string(filepath.Separator))
	patterns := slices.Clone(c.base)
	dir := c.repo
	for i := range segs {
		patterns = append(patterns, c.dirPatterns(dir, segs[:i])...)
		dir = filepath.Join(dir, segs[i])
		last := i == len(segs)-1
		if _, ok := c.tracked[strings.Join(segs[:i+1], "/")]; ok {
			if last {
				return false
			}
			continue
		}
		if !pathWithin(dir, from) {
			continue
		}
		if gitignore.NewMatcher(patterns).Match(segs[:i+1], isDir || !last) {
			return true
		}
	}
	return false
}

// dirPatterns returns the patterns of the .gitignore in dir, whose path
// relative to the repository root is domain.
func (c *Checker) dirPatterns(dir string, domain []string) []gitignore.Pattern {
	c.mu.Lock()
	ps, ok := c.dirs[dir]
	c.mu.Unlock()
	if ok {
		return ps
	}
	// Like git, refuse a .gitignore that is a symlink, so a browsed tree
	// cannot point it at something like /dev/zero.
	ps = readPatterns(filepath.Join(dir, ".gitignore"), domain, true)
	c.mu.Lock()
	c.dirs[dir] = ps
	c.mu.Unlock()
	return ps
}

type ignoreFS struct {
	fs.FS
	base    string
	checker *Checker
}

func (f *ignoreFS) Open(name string) (fs.File, error) {
	if name != "." && f.checker.Ignored(filepath.Join(f.base, filepath.FromSlash(name))) {
		return nil, fs.ErrNotExist
	}
	return f.FS.Open(name)
}

func (f *ignoreFS) ReadDir(name string) ([]fs.DirEntry, error) {
	if name != "." && f.checker.Ignored(filepath.Join(f.base, filepath.FromSlash(name))) {
		return nil, fs.ErrNotExist
	}
	entries, err := fs.ReadDir(f.FS, name)
	if err != nil {
		return nil, err
	}
	dir := filepath.Join(f.base, filepath.FromSlash(name))
	return slices.DeleteFunc(entries, func(e fs.DirEntry) bool {
		return f.checker.match(filepath.Join(dir, e.Name()), e.IsDir(), f.checker.root)
	}), nil
}

func (f *ignoreFS) Stat(name string) (fs.FileInfo, error) {
	if name != "." && f.checker.Ignored(filepath.Join(f.base, filepath.FromSlash(name))) {
		return nil, fs.ErrNotExist
	}
	return fs.Stat(f.FS, name)
}

// findRepo walks up from dir to the nearest working tree root, and returns it
// with the repository's git directory.
func findRepo(dir string) (repo, gitDir string, ok bool) {
	for {
		dotGit := filepath.Join(dir, ".git")
		if info, err := os.Stat(dotGit); err == nil {
			if info.IsDir() {
				return dir, dotGit, true
			}
			// Linked worktrees and submodules have a .git file that points
			// at the real git directory.
			if gd, ok := readGitDirFile(dotGit); ok {
				return dir, gd, true
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", "", false
		}
		dir = parent
	}
}

func readGitDirFile(path string) (string, bool) {
	b, err := readRegularFile(path, false)
	if err != nil {
		return "", false
	}
	gd, ok := strings.CutPrefix(strings.TrimSpace(string(b)), "gitdir:")
	if !ok {
		return "", false
	}
	gd = filepath.FromSlash(strings.TrimSpace(gd))
	if !filepath.IsAbs(gd) {
		gd = filepath.Join(filepath.Dir(path), gd)
	}
	return gd, true
}

// loadBasePatterns returns the patterns that apply to the whole repository,
// lowest priority first.
func loadBasePatterns(gitDir string) []gitignore.Pattern {
	// A linked worktree shares info/exclude and config with the main
	// repository.
	commonDir := gitDir
	if b, err := readRegularFile(filepath.Join(gitDir, "commondir"), false); err == nil {
		cd := filepath.FromSlash(strings.TrimSpace(string(b)))
		if !filepath.IsAbs(cd) {
			cd = filepath.Join(gitDir, cd)
		}
		commonDir = cd
	}
	var ps []gitignore.Pattern
	if f := excludesFile(gitDir, commonDir); f != "" {
		ps = readPatterns(f, nil, false)
	}
	return append(ps, readPatterns(filepath.Join(commonDir, "info", "exclude"), nil, false)...)
}

// loadTracked reads the index of the repository whose git directory is
// gitDir. A linked worktree has its own index there, so commondir is not
// followed. When the index cannot be read every path counts as untracked.
func loadTracked(gitDir string) map[string]struct{} {
	name := filepath.Join(gitDir, "index")
	if info, err := os.Stat(name); err != nil || !info.Mode().IsRegular() {
		return nil
	}
	f, err := os.Open(name) //nolint:gosec
	if err != nil {
		return nil
	}
	defer f.Close()
	var idx index.Index
	if err := index.NewDecoder(bufio.NewReader(f)).Decode(&idx); err != nil {
		slog.Debug("skipping git index", "dir", gitDir, "error", err)
		return nil
	}
	tracked := make(map[string]struct{}, len(idx.Entries))
	for _, e := range idx.Entries {
		for p := e.Name; p != "."; p = path.Dir(p) {
			if _, ok := tracked[p]; ok {
				break
			}
			tracked[p] = struct{}{}
		}
	}
	return tracked
}

// excludesFile returns the ignore file git reads for core.excludesFile, or
// its default when unset.
//
// go-git's LoadGlobalPatterns is not used because it opens ~/.gitconfig
// through a billy filesystem rooted at "/", which cannot reach a Windows
// home directory, and it reads neither $XDG_CONFIG_HOME/git/config, the
// repository config nor includes, nor falls back to the default ignore file.
func excludesFile(gitDir, commonDir string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		slog.Debug("skipping global git excludes", "error", err)
		return ""
	}
	xdg := os.Getenv("XDG_CONFIG_HOME")
	if xdg == "" {
		xdg = filepath.Join(home, ".config")
	}
	// Later files take precedence, as in git.
	var excludes string
	cfgs := []string{
		"/etc/gitconfig",
		filepath.Join(xdg, "git", "config"),
		filepath.Join(home, ".gitconfig"),
		filepath.Join(commonDir, "config"),
	}
	for _, cfg := range cfgs {
		if v := coreExcludesFile(cfg, home, gitDir, 0); v != "" {
			excludes = v
		}
	}
	if excludes == "" {
		return filepath.Join(xdg, "git", "ignore")
	}
	return expandHome(excludes, home)
}

// maxIncludeDepth is git's limit on nested config includes.
const maxIncludeDepth = 10

// coreExcludesFile returns core.excludesFile as set by the config file at
// path or by the files it includes, applying settings and includes in the
// order they appear so that the last one wins, as in git.
func coreExcludesFile(path, home, gitDir string, depth int) string {
	if depth > maxIncludeDepth {
		return ""
	}
	b, err := readRegularFile(path, false)
	if err != nil {
		return ""
	}
	var v string
	include := func(inc string) {
		inc = expandHome(inc, home)
		if !filepath.IsAbs(inc) {
			inc = filepath.Join(filepath.Dir(path), inc)
		}
		if iv := coreExcludesFile(inc, home, gitDir, depth+1); iv != "" {
			v = iv
		}
	}
	// go-git's decoder merges repeated sections, which loses where an
	// [include] sits relative to [core]. Decoding one section at a time keeps
	// that order.
	for _, chunk := range splitSections(b) {
		cfg := config.New()
		if err := config.NewDecoder(bytes.NewReader(chunk)).Decode(cfg); err != nil {
			slog.Debug("skipping unreadable git config section", "path", path, "error", err)
			continue
		}
		for _, s := range cfg.Sections {
			switch {
			case s.IsName("core"):
				for _, o := range s.Options {
					if o.IsKey("excludesfile") {
						v = o.Value
					}
				}
			case s.IsName("include"):
				for _, o := range s.Options {
					if o.IsKey("path") {
						include(o.Value)
					}
				}
			case s.IsName("includeIf"):
				for _, ss := range s.Subsections {
					if !gitDirMatches(ss.Name, path, home, gitDir) {
						continue
					}
					for _, o := range ss.Options {
						if o.IsKey("path") {
							include(o.Value)
						}
					}
				}
			}
		}
	}
	return v
}

// splitSections splits a git config file at each section header.
func splitSections(b []byte) [][]byte {
	var chunks [][]byte
	var cur []byte
	for line := range bytes.Lines(b) {
		if bytes.HasPrefix(bytes.TrimSpace(line), []byte("[")) && len(cur) > 0 {
			chunks = append(chunks, cur)
			cur = nil
		}
		cur = append(cur, line...)
	}
	if len(cur) > 0 {
		chunks = append(chunks, cur)
	}
	return chunks
}

// gitDirMatches evaluates an includeIf "gitdir:" or "gitdir/i:" condition
// against gitDir. Other conditions such as onbranch are never met.
func gitDirMatches(cond, cfgPath, home, gitDir string) bool {
	pat, ok := strings.CutPrefix(cond, "gitdir:")
	fold := false
	if !ok {
		if pat, ok = strings.CutPrefix(cond, "gitdir/i:"); !ok {
			return false
		}
		fold = true
	}
	// Expanding ~/ and ./ goes through filepath.Join, which drops the
	// trailing slash that makes the condition cover every repository below.
	dirPrefix := strings.HasSuffix(pat, "/")
	switch {
	case strings.HasPrefix(pat, "~/"):
		pat = expandHome(pat, home)
	case strings.HasPrefix(pat, "./"):
		pat = filepath.Join(filepath.Dir(cfgPath), pat[2:])
	case !filepath.IsAbs(filepath.FromSlash(pat)):
		pat = "**/" + pat
	}
	pat = filepath.ToSlash(pat)
	if dirPrefix && !strings.HasSuffix(pat, "/") {
		pat += "/"
	}
	if strings.HasSuffix(pat, "/") {
		pat += "**"
	}
	target := filepath.ToSlash(gitDir)
	if fold {
		pat, target = strings.ToLower(pat), strings.ToLower(target)
	}
	matched, err := doublestar.Match(pat, target)
	return err == nil && matched
}

func expandHome(p, home string) string {
	if rest, ok := strings.CutPrefix(p, "~/"); ok {
		return filepath.Join(home, filepath.FromSlash(rest))
	}
	return filepath.FromSlash(p)
}

// readPatterns parses an ignore file. go-git's own reader is unexported, and
// its ReadPatterns walks the whole tree up front, which is what a lazily
// loading Checker avoids.
func readPatterns(path string, domain []string, nofollow bool) []gitignore.Pattern {
	b, err := readRegularFile(path, nofollow)
	if err != nil {
		return nil
	}
	var ps []gitignore.Pattern
	for line := range strings.Lines(string(b)) {
		line = strings.TrimRight(line, "\r\n")
		if strings.HasPrefix(line, "#") || strings.TrimSpace(line) == "" {
			continue
		}
		// In git a trailing "/**" matches everything inside a directory but
		// not the directory itself, so a later negation can still reach a
		// file below it. go-git's matcher also matches the directory, which
		// would make the parent check in match skip it, so require at least
		// one more segment.
		if trimmed := strings.TrimRight(line, " "); strings.HasSuffix(trimmed, "/**") {
			line = trimmed + "/*"
		}
		ps = append(ps, gitignore.ParsePattern(line, domain))
	}
	return ps
}

// maxFileSize bounds what is read from a single ignore or config file.
const maxFileSize = 1 << 20

// readRegularFile reads path only when it is a regular file, so that a device
// or a FIFO can neither exhaust memory nor block forever, and reads at most
// maxFileSize bytes of it. With nofollow, a symlink is refused rather than
// followed.
func readRegularFile(path string, nofollow bool) ([]byte, error) {
	stat := os.Stat
	if nofollow {
		stat = os.Lstat
	}
	info, err := stat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("%s is not a regular file", path)
	}
	f, err := os.Open(path) //nolint:gosec
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return io.ReadAll(io.LimitReader(f, maxFileSize))
}

func pathWithin(path, base string) bool {
	rel, err := filepath.Rel(base, path)
	return err == nil && rel != "." && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
