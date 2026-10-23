// Package gitignore answers "is this path ignored by git?" by delegating to
// the git command, so .gitignore, .git/info/exclude and the global ignore file
// are all honored.
package gitignore

import (
	"bytes"
	"log/slog"
	"os/exec"
	"path/filepath"
	"strings"
)

// Checker reports which paths below a root directory are ignored by git. It
// is built from a single `git ls-files` call, so it is cheap to query for
// every entry of a large tree. A nil *Checker ignores nothing.
type Checker struct {
	root    string
	ignored map[string]struct{}
}

// NewChecker lists the untracked, ignored entries below root. Ignored
// directories are reported as a whole, so a directory such as node_modules
// costs one entry regardless of its size.
//
// It returns nil (nothing is ignored) when git is unavailable, root is not in
// a git repository, or root itself is ignored: pointing mo at an ignored
// directory explicitly means the user wants its contents.
func NewChecker(root string) *Checker {
	root = filepath.Clean(root)
	if IsIgnored(root) {
		return nil
	}
	cmd := exec.Command("git", "-C", root, "ls-files", "-o", "-i", "--exclude-standard", "--directory", "-z") //nolint:gosec
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		slog.Debug("skipping gitignore filtering", "dir", root, "error", strings.TrimSpace(stderr.String()))
		return nil
	}
	ignored := make(map[string]struct{})
	for rel := range strings.SplitSeq(string(out), "\x00") {
		rel = strings.TrimSuffix(rel, "/")
		if rel == "" {
			continue
		}
		ignored[filepath.Join(root, filepath.FromSlash(rel))] = struct{}{}
	}
	return &Checker{root: root, ignored: ignored}
}

// Ignored reports whether path, or one of its parent directories below the
// root, is ignored.
func (c *Checker) Ignored(path string) bool {
	if c == nil || len(c.ignored) == 0 {
		return false
	}
	for p := filepath.Clean(path); p != c.root; {
		if _, ok := c.ignored[p]; ok {
			return true
		}
		parent := filepath.Dir(p)
		if parent == p {
			break
		}
		p = parent
	}
	return false
}

// IsIgnored reports whether a single path is ignored by git, including when
// one of its parent directories is ignored. It returns false when git is
// unavailable or path is not in a git repository.
func IsIgnored(path string) bool {
	err := exec.Command("git", "-C", filepath.Dir(path), "check-ignore", "-q", "--", path).Run() //nolint:gosec
	return err == nil
}
