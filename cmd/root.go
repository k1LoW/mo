package cmd

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"maps"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/k1LoW/errors"

	"github.com/k1LoW/donegroup"
	"github.com/k1LoW/mo/internal/backup"
	"github.com/k1LoW/mo/internal/logfile"
	"github.com/k1LoW/mo/internal/server"
	"github.com/k1LoW/mo/version"
	"github.com/muesli/termenv"
	"github.com/pkg/browser"
	"github.com/spf13/cobra"
)

const (
	// probeTimeoutFast is used when a missing server is the normal case (e.g. first launch).
	probeTimeoutFast = 500 * time.Millisecond
	// probeTimeoutDefault is used when the server is expected to be running.
	probeTimeoutDefault = 2 * time.Second
)

var (
	target                       string
	port                         int
	bind                         string
	open                         bool
	noOpen                       bool
	restore                      string
	shutdownServer               bool
	restartServer                bool
	foreground                   bool
	statusServer                 bool
	watchPatterns                []string
	unwatchPatterns              []string
	closeFiles                   bool
	clearBackup                  bool
	jsonOutput                   bool
	dangerouslyAllowRemoteAccess bool
)

var rootCmd = &cobra.Command{
	Use:   "mo [flags] [FILE ...]",
	Short: "mo is a Markdown viewer that opens .md files in a browser.",
	Long: `mo is a Markdown viewer that opens .md files in a browser with live-reload.

It runs in the background, serving Markdown files using a built-in React SPA,
and automatically refreshes the browser when files are saved.

Examples:
  mo README.md                          Open a single file
  mo README.md CHANGELOG.md docs/*.md   Open multiple files
  mo spec.md --target design            Open in a named group
  mo draft.md --port 6276               Use a different port

Single Server, Multiple Files:
  By default, mo runs a single server on port 6275.
  If a mo server is already running on the same port, subsequent mo
  invocations add files to the existing session instead of starting a new one.

  $ mo README.md          # Starts a mo server in the background
  $ mo CHANGELOG.md       # Adds the file to the running mo server

  To run a completely separate session, use a different port:

  $ mo draft.md -p 6276

Groups:
  Files can be organized into named groups using the --target (-t) flag.
  Each group gets its own URL path (e.g., http://localhost:6275/design)
  and its own sidebar in the browser.

  $ mo spec.md --target design      # Opens at /design
  $ mo api.md --target design       # Adds to the "design" group
  $ mo notes.md --target notes      # Opens at /notes

  If no --target is specified, files are added to the "default" group.

Starting and Stopping:
  mo runs in the background by default. The command returns
  immediately, leaving the shell free for other work.

  $ mo README.md            # Starts mo in the background
  $ mo --status             # Shows all running mo servers
  $ mo --shutdown           # Shuts it down
  $ mo --restart            # Restarts it (preserving session)

  Use --foreground to keep the mo server in the foreground.

Session Restore:
  mo automatically saves session state. When starting a new server,
  the previous session is restored and merged with any specified files.

  $ mo README.md CHANGELOG.md    # Start with two files
  $ mo --shutdown                # Shut down the server
  $ mo                           # Restores README.md and CHANGELOG.md
  $ mo TODO.md                   # Restores previous session + adds TODO.md

  Use --clear to remove a saved session.

Live-Reload:
  mo watches all opened files for changes using filesystem notifications.
  When a file is saved, the browser automatically re-renders the content.

Supported Markdown Features:
  - GitHub Flavored Markdown (tables, task lists, strikethrough, autolinks)
  - Syntax-highlighted code blocks (via Shiki)
  - Mermaid diagrams
  - YAML frontmatter (displayed as a collapsible metadata block)
  - MDX files (rendered as Markdown with import/export stripped and JSX tags escaped)
  - Raw HTML

Glob Patterns:
  Use --watch (-w) to specify glob patterns. Matching directories are
  watched and new files are automatically added.
  Cannot be combined with file arguments.

  $ mo -w '**/*.md'                   Watch all .md files recursively
  $ mo -w 'docs/**/*.md' -t docs      Watch docs/ tree in "docs" group
  $ mo -w '*.md' -w 'docs/**/*.md'    Watch multiple patterns
  $ mo --unwatch '**/*.md'            Stop watching a pattern

WARNING: --bind with a non-loopback address:
  Binding to a non-localhost address (e.g. 0.0.0.0) exposes mo to the
  network without any authentication. Remote clients can read any file
  accessible by this user, browse the filesystem via glob patterns, and
  shut down the server. A confirmation prompt is shown before starting.`,
	Args:    cobra.ArbitraryArgs,
	RunE:    run,
	Version: version.Version,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.Flags().StringVarP(&target, "target", "t", server.DefaultGroup, "Tab group name")
	rootCmd.Flags().IntVarP(&port, "port", "p", 6275, "Server port")
	rootCmd.Flags().StringVarP(&bind, "bind", "b", "localhost", "Bind address (e.g. localhost, 0.0.0.0)")
	rootCmd.Flags().BoolVar(&open, "open", false, "Always open browser (even when adding to existing group)")
	rootCmd.Flags().BoolVar(&noOpen, "no-open", false, "Do not open browser automatically")
	rootCmd.MarkFlagsMutuallyExclusive("open", "no-open")
	rootCmd.Flags().BoolVar(&shutdownServer, "shutdown", false, "Shut down the running mo server on the specified port")
	rootCmd.Flags().BoolVar(&restartServer, "restart", false, "Restart the running mo server on the specified port")
	rootCmd.MarkFlagsMutuallyExclusive("shutdown", "restart")
	rootCmd.Flags().StringVar(&restore, "restore", "", "Restore state from file (internal use)")
	rootCmd.Flags().MarkHidden("restore") //nolint:errcheck
	rootCmd.Flags().BoolVar(&foreground, "foreground", false, "Run mo server in foreground (do not background)")
	rootCmd.Flags().BoolVar(&statusServer, "status", false, "Show status of all running mo servers")
	rootCmd.Flags().StringArrayVarP(&watchPatterns, "watch", "w", nil, "Glob pattern to watch for matching files (repeatable)")
	rootCmd.Flags().StringArrayVar(&unwatchPatterns, "unwatch", nil, "Remove a watched glob pattern (repeatable)")
	rootCmd.Flags().BoolVar(&closeFiles, "close", false, "Close files instead of opening them")
	rootCmd.Flags().BoolVar(&clearBackup, "clear", false, "Clear saved session for the specified port")
	rootCmd.Flags().BoolVar(&jsonOutput, "json", false, "Output structured data as JSON to stdout")
	rootCmd.Flags().BoolVar(&dangerouslyAllowRemoteAccess, "dangerously-allow-remote-access", false, "Allow remote access without authentication. Recommended only for trusted networks.")
}

func run(cmd *cobra.Command, args []string) error {
	if !foreground || restore != "" {
		logCleanup, err := logfile.Setup(port)
		if err != nil {
			slog.Warn("failed to setup log file, using stderr", "error", err)
		} else {
			defer logCleanup()
		}
	}

	bind = strings.Trim(bind, "[]")
	addr := net.JoinHostPort(bind, strconv.Itoa(port))

	if clearBackup {
		wasServerRunning := false
		if _, err := probeServer(addr, probeTimeoutFast); err == nil {
			wasServerRunning = true
		}
		hasBackup := backup.Exists(port)

		if !wasServerRunning && !hasBackup {
			fmt.Fprintf(os.Stderr, "mo: no saved session for port %d\n", port)
			return nil
		}
		fmt.Fprintf(os.Stderr, "mo: clear saved session for port %d? [Y/n] ", port)
		scanner := bufio.NewScanner(os.Stdin)
		if !scanner.Scan() {
			fmt.Fprintln(os.Stderr, "mo: canceled")
			return nil
		}
		ans := strings.TrimSpace(scanner.Text())
		if ans != "" && strings.ToLower(ans) != "y" && strings.ToLower(ans) != "yes" {
			fmt.Fprintln(os.Stderr, "mo: canceled")
			return nil
		}

		if wasServerRunning {
			// Shut down the running server, wait for it to stop,
			// then restart with an empty state.
			if err := doShutdown(addr); err != nil {
				return err
			}
			if err := waitForServerDown(addr); err != nil {
				return err
			}
		}

		if hasBackup {
			if err := backup.Remove(port); err != nil {
				return err
			}
		}

		if wasServerRunning {
			// Restart the server with an empty state.
			if _, err := spawnNewProcess(addr, ""); err != nil {
				return err
			}
			fmt.Fprintf(os.Stderr, "mo: cleared session and restarted server on port %d\n", port)
		} else {
			fmt.Fprintf(os.Stderr, "mo: cleared saved session for port %d\n", port)
		}
		return nil
	}

	if statusServer {
		return doStatus()
	}

	if shutdownServer {
		return doShutdown(addr)
	}

	if restartServer {
		return doRestart(addr)
	}

	if len(unwatchPatterns) > 0 {
		if len(watchPatterns) > 0 {
			return fmt.Errorf("cannot use --unwatch with --watch")
		}
		if len(args) > 0 {
			return fmt.Errorf("cannot use --unwatch with file arguments")
		}

		resolved, err := resolvePatterns(unwatchPatterns)
		if err != nil {
			return err
		}

		resolvedTarget, err := server.ResolveGroupName(target)
		if err != nil {
			return fmt.Errorf("invalid target group name %q: %w", target, err)
		}

		return doUnwatch(addr, resolved, resolvedTarget)
	}

	if closeFiles {
		if len(watchPatterns) > 0 {
			return fmt.Errorf("cannot use --close with --watch")
		}
		if len(args) == 0 {
			return fmt.Errorf("--close requires at least one file argument")
		}

		resolvedTarget, err := server.ResolveGroupName(target)
		if err != nil {
			return fmt.Errorf("invalid target group name %q: %w", target, err)
		}

		closedPaths, err := doClose(addr, args, resolvedTarget)
		if len(closedPaths) > 0 {
			names := displayNames(closedPaths)
			for _, name := range names {
				fmt.Printf("  %s\n", name)
			}
			fmt.Fprintf(os.Stderr, "mo: closed %d file(s) from http://%s\n", len(closedPaths), addr)
		}
		return err
	}

	if restore != "" {
		filesByGroup, patternsByGroup, uploadedFiles, err := loadRestoreData(restore)
		if err != nil {
			return fmt.Errorf("failed to restore state: %w", err)
		}
		return startServer(cmd.Context(), addr, filesByGroup, patternsByGroup, uploadedFiles)
	}

	resolved, err := server.ResolveGroupName(target)
	if err != nil {
		return fmt.Errorf("invalid target group name %q: %w", target, err)
	}
	target = resolved

	if len(watchPatterns) > 0 && len(args) > 0 {
		hasGlob := slices.ContainsFunc(watchPatterns, func(p string) bool {
			return hasGlobChars(p)
		})
		if !hasGlob {
			return fmt.Errorf("cannot use --watch (-w) with file arguments\n(hint: the shell may have expanded the glob pattern; quote it to prevent expansion, e.g. -w '**/*.md')")
		}
		return fmt.Errorf("cannot use --watch (-w) with file arguments")
	}

	patterns, err := resolvePatterns(watchPatterns)
	if err != nil {
		return err
	}

	files, err := resolveFiles(args)
	if err != nil {
		return err
	}

	// When no files or patterns are specified and a server is already
	// running, just open the browser and exit.
	if len(files) == 0 && len(patterns) == 0 {
		if _, err := probeServer(addr, probeTimeoutDefault); err == nil {
			openBrowser(addr)
			return nil
		}
	}

	if (len(files) > 0 || len(patterns) > 0) && tryAddToExisting(addr, files, patterns) {
		return nil
	}

	filesByGroup := map[string][]string{target: files}
	var patternsByGroup map[string][]string
	if len(patterns) > 0 {
		patternsByGroup = map[string][]string{target: patterns}
	}

	// Restore backup and merge with specified files/patterns
	var rd server.RestoreData
	if err := backup.Load(port, &rd); err != nil {
		slog.Warn("failed to load backup", "error", err)
	}
	restoredFiles, restoredPatterns, restoredUploads := filterValidRestoreData(&rd)
	var uploadedFiles []server.UploadedFileData
	if len(restoredFiles) > 0 || len(restoredPatterns) > 0 || len(restoredUploads) > 0 {
		slog.Info("restoring session from backup", "port", port)
		fmt.Fprintf(os.Stderr, "mo: restoring previous session for port %d\n", port)
		filesByGroup = mergeGroups(restoredFiles, filesByGroup)
		patternsByGroup = mergeGroups(restoredPatterns, patternsByGroup)
		uploadedFiles = restoredUploads
	}

	// Prompt only when actually starting a new server (not adding to existing one).
	if !isLoopbackBind(bind) {
		slog.Warn("binding to non-loopback address", "bind", bind, "dangerously-allow-remote-access", dangerouslyAllowRemoteAccess)
	}
	if !isLoopbackBind(bind) && !dangerouslyAllowRemoteAccess {
		o := termenv.NewOutput(os.Stderr)
		c := func(s string) termenv.Style { return o.String(s).Foreground(o.Color("208")) }
		fmt.Fprintln(os.Stderr, c("SECURITY WARNING:").Bold(),
			c(fmt.Sprintf("Binding to %s instead of localhost. mo has no authentication -- remote clients can:", bind)))
		fmt.Fprintln(os.Stderr, c("  - Read any file accessible by this user"))
		fmt.Fprintln(os.Stderr, c("  - Browse the filesystem via glob patterns"))
		fmt.Fprintln(os.Stderr, c("  - Shut down or restart the server"))
		fmt.Fprintf(os.Stderr, "Continue? [y/N] ")
		scanner := bufio.NewScanner(os.Stdin)
		if !scanner.Scan() {
			if err := scanner.Err(); err != nil {
				return err
			}
			fmt.Fprintln(os.Stderr, "mo: canceled")
			return nil
		}
		ans := strings.ToLower(strings.TrimSpace(scanner.Text()))
		if ans != "y" && ans != "yes" {
			fmt.Fprintln(os.Stderr, "mo: canceled")
			return nil
		}
	}

	if foreground {
		return startServer(cmd.Context(), addr, filesByGroup, patternsByGroup, uploadedFiles)
	}
	return startBackground(addr, filesByGroup, patternsByGroup, uploadedFiles)
}

// mergeGroups merges base and additional group maps, with base entries first.
// Entries from additional that already exist in base for the same group are skipped.
func mergeGroups(base, additional map[string][]string) map[string][]string {
	if len(base) == 0 && len(additional) == 0 {
		return nil
	}
	merged := make(map[string][]string)
	for group, items := range base {
		merged[group] = append(merged[group], items...)
	}
	for group, items := range additional {
		seen := make(map[string]struct{}, len(merged[group]))
		for _, v := range merged[group] {
			seen[v] = struct{}{}
		}
		for _, v := range items {
			if _, ok := seen[v]; !ok {
				merged[group] = append(merged[group], v)
				seen[v] = struct{}{}
			}
		}
	}
	return merged
}

// filterValidRestoreData validates restore data by checking that file paths still exist.
func filterValidRestoreData(rd *server.RestoreData) (map[string][]string, map[string][]string, []server.UploadedFileData) {
	filesByGroup := make(map[string][]string)
	for group, paths := range rd.Groups {
		for _, p := range paths {
			if _, err := os.Stat(p); err != nil {
				slog.Info("skipping missing file from backup", "path", p)
				continue
			}
			filesByGroup[group] = append(filesByGroup[group], p)
		}
	}

	patternsByGroup := make(map[string][]string)
	maps.Copy(patternsByGroup, rd.Patterns)

	return filesByGroup, patternsByGroup, rd.UploadedFiles
}

func loadRestoreData(path string) (map[string][]string, map[string][]string, []server.UploadedFileData, error) {
	data, err := os.ReadFile(path) //nolint:gosec
	if err != nil {
		return nil, nil, nil, err
	}
	os.Remove(path)

	var rd server.RestoreData
	if err := json.Unmarshal(data, &rd); err != nil {
		return nil, nil, nil, err
	}
	return rd.Groups, rd.Patterns, rd.UploadedFiles, nil
}

func isLoopbackBind(bind string) bool {
	if bind == "localhost" {
		return true
	}
	ip := net.ParseIP(bind)
	return ip != nil && ip.IsLoopback()
}

func hasGlobChars(s string) bool {
	return strings.ContainsAny(s, "*?[")
}

func resolvePatterns(patterns []string) ([]string, error) {
	var resolved []string
	for _, pat := range patterns {
		if !hasGlobChars(pat) {
			return nil, fmt.Errorf("pattern %q does not contain glob characters (* ? [); use file arguments instead", pat)
		}
		abs, err := filepath.Abs(pat)
		if err != nil {
			return nil, fmt.Errorf("cannot resolve pattern %q: %w", pat, err)
		}
		resolved = append(resolved, abs)
	}
	return resolved, nil
}

func resolveFiles(args []string) ([]string, error) {
	var files []string
	for _, arg := range args {
		absPath, err := filepath.Abs(arg)
		if err != nil {
			return nil, fmt.Errorf("cannot resolve path %s: %w", arg, err)
		}
		if stat, err := os.Stat(absPath); err != nil {
			return nil, fmt.Errorf("file not found: %s", absPath)
		} else if stat.IsDir() {
			return nil, fmt.Errorf("%s is a directory", absPath)
		}
		files = append(files, absPath)
	}
	return files, nil
}

func tryAddToExisting(addr string, files []string, patterns []string) bool {
	result, err := probeServer(addr, probeTimeoutFast)
	if err != nil {
		return false
	}

	isNewGroup := !slices.Contains(result.groups, target)

	var deeplinks []deeplinkEntry
	deeplinks = append(deeplinks, postFiles(result.client, addr, target, files)...)
	deeplinks = append(deeplinks, postPatterns(result.client, addr, target, patterns)...)

	added := len(files) + len(patterns)
	slog.Info("added to existing server", "files", len(files), "patterns", len(patterns), "addr", addr)
	emitServeOutput(addr, deeplinks, false)
	fmt.Fprintf(os.Stderr, "mo: added %d item(s) to http://%s\n", added, addr)

	if isNewGroup || open {
		openBrowser(addr)
	}

	return true
}

func postFiles(client *http.Client, addr, group string, files []string) []deeplinkEntry {
	var entries []deeplinkEntry
	for _, f := range files {
		body, err := json.Marshal(map[string]string{
			"path":  f,
			"group": group,
		})
		if err != nil {
			slog.Warn("failed to marshal request", "path", f, "error", err)
			continue
		}
		resp, err := client.Post(
			fmt.Sprintf("http://%s/_/api/files", addr),
			"application/json",
			bytes.NewReader(body),
		)
		if err != nil {
			slog.Warn("failed to post file", "path", f, "error", err)
			continue
		}
		if resp.StatusCode != http.StatusOK {
			slog.Warn("failed to add file", "path", f, "status", resp.StatusCode)
			resp.Body.Close()
			continue
		}
		var entry server.FileEntry
		if err := json.NewDecoder(resp.Body).Decode(&entry); err != nil {
			slog.Warn("failed to decode file response", "error", err)
			resp.Body.Close()
			continue
		}
		resp.Body.Close()
		entries = append(entries, deeplinkEntry{
			URL:  buildDeeplink(addr, group, entry.ID),
			Path: entry.Path,
		})
	}
	return entries
}

func postPatterns(client *http.Client, addr, group string, patterns []string) []deeplinkEntry {
	var entries []deeplinkEntry
	for _, pat := range patterns {
		body, err := json.Marshal(map[string]string{
			"pattern": pat,
			"group":   group,
		})
		if err != nil {
			slog.Warn("failed to marshal request", "pattern", pat, "error", err)
			continue
		}
		resp, err := client.Post(
			fmt.Sprintf("http://%s/_/api/patterns", addr),
			"application/json",
			bytes.NewReader(body),
		)
		if err != nil {
			slog.Warn("failed to post pattern", "pattern", pat, "error", err)
			continue
		}
		if resp.StatusCode != http.StatusOK {
			slog.Warn("failed to add pattern", "pattern", pat, "status", resp.StatusCode)
			resp.Body.Close()
			continue
		}
		var patResp server.AddPatternResponse
		if err := json.NewDecoder(resp.Body).Decode(&patResp); err != nil {
			slog.Warn("failed to decode pattern response", "error", err)
			resp.Body.Close()
			continue
		}
		resp.Body.Close()
		for _, f := range patResp.Files {
			entries = append(entries, deeplinkEntry{
				URL:  buildDeeplink(addr, group, f.ID),
				Path: f.Path,
			})
		}
	}
	return entries
}

type deeplinkEntry struct {
	URL  string
	Path string // absolute file path (empty for uploaded files)
	Name string // display name fallback when Path is empty
}

// JSON output types

type jsonFileEntry struct {
	URL  string `json:"url"`
	Name string `json:"name"`
	Path string `json:"path"`
}

type jsonServeOutput struct {
	URL   string          `json:"url"`
	Files []jsonFileEntry `json:"files"`
}

type jsonStatusGroupEntry struct {
	Name     string   `json:"name"`
	Files    int      `json:"files"`
	Patterns []string `json:"patterns,omitempty"`
}

type jsonStatusEntry struct {
	URL      string                 `json:"url"`
	Status   string                 `json:"status"`
	PID      int                    `json:"pid,omitempty"`
	Version  string                 `json:"version,omitempty"`
	Revision string                 `json:"revision,omitempty"`
	Groups   []jsonStatusGroupEntry `json:"groups,omitempty"`
}

func writeJSON(v any) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		slog.Warn("failed to write JSON output", "error", err)
	}
}

func deeplinksToJSON(entries []deeplinkEntry) []jsonFileEntry {
	if len(entries) == 0 {
		return []jsonFileEntry{}
	}
	names := deeplinkDisplayNames(entries)
	result := make([]jsonFileEntry, len(entries))
	for i, e := range entries {
		result[i] = jsonFileEntry{URL: e.URL, Name: names[i], Path: e.Path}
	}
	return result
}

func buildDeeplink(addr, groupName, fileID string) string {
	if groupName == server.DefaultGroup {
		return fmt.Sprintf("http://%s/?file=%s", addr, fileID)
	}
	return fmt.Sprintf("http://%s/%s?file=%s", addr, groupName, fileID)
}

// displayNames computes short display names for file paths, adding parent
// directory components as needed to distinguish files with the same base name.
func displayNames(paths []string) []string {
	names := make([]string, len(paths))
	// Track remaining parent path for each entry
	dirs := make([]string, len(paths))
	for i, p := range paths {
		names[i] = filepath.Base(p)
		dirs[i] = filepath.Dir(p)
	}

	for {
		dupes := make(map[string][]int)
		for i, n := range names {
			dupes[n] = append(dupes[n], i)
		}
		changed := false
		for _, indices := range dupes {
			if len(indices) <= 1 {
				continue
			}
			for _, idx := range indices {
				// Stop expanding when we've reached the filesystem root
				if dirs[idx] == filepath.Dir(dirs[idx]) {
					continue
				}
				parent := filepath.Base(dirs[idx])
				names[idx] = filepath.Join(parent, names[idx])
				dirs[idx] = filepath.Dir(dirs[idx])
				changed = true
			}
		}
		if !changed {
			break
		}
	}
	return names
}

// deeplinkDisplayNames computes display names for deeplink entries,
// using Name as fallback when Path is empty (uploaded files).
func deeplinkDisplayNames(entries []deeplinkEntry) []string {
	var pathEntries []string
	for _, e := range entries {
		if e.Path != "" {
			pathEntries = append(pathEntries, e.Path)
		} else {
			pathEntries = append(pathEntries, e.Name)
		}
	}
	return displayNames(pathEntries)
}

func printDeeplinks(entries []deeplinkEntry) {
	if len(entries) == 0 {
		return
	}
	names := deeplinkDisplayNames(entries)
	for i, e := range entries {
		fmt.Printf("  %s  %s\n", e.URL, names[i])
	}
}

// emitServeOutput writes the serve result (server URL + deeplinks) to stdout.
// In JSON mode it emits a single JSON object; in text mode it prints the URL and deeplinks.
func emitServeOutput(addr string, deeplinks []deeplinkEntry, printURL bool) {
	if jsonOutput {
		writeJSON(jsonServeOutput{
			URL:   fmt.Sprintf("http://%s", addr),
			Files: deeplinksToJSON(deeplinks),
		})
	} else {
		if printURL {
			fmt.Fprintf(os.Stdout, "http://%s\n", addr)
		}
		printDeeplinks(deeplinks)
	}
}

type probeResult struct {
	client *http.Client
	groups []string
}

// probeServer checks that a mo server is running on addr by calling
// GET /_/api/status and validating the response contains a version field.
func probeServer(addr string, timeout ...time.Duration) (*probeResult, error) {
	t := probeTimeoutDefault
	if len(timeout) > 0 {
		t = timeout[0]
	}
	client := &http.Client{Timeout: t}
	resp, err := client.Get(fmt.Sprintf("http://%s/_/api/status", addr))
	if err != nil {
		return nil, fmt.Errorf("no mo server found on %s", addr)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("server on %s returned %s", addr, resp.Status)
	}

	var status struct {
		Version string `json:"version"`
		PID     int    `json:"pid"`
		Groups  []struct {
			Name string `json:"name"`
		} `json:"groups"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil || status.Version == "" {
		return nil, fmt.Errorf("server on %s is not a mo instance", addr)
	}

	groups := make([]string, len(status.Groups))
	for i, g := range status.Groups {
		groups[i] = g.Name
	}
	return &probeResult{client: client, groups: groups}, nil
}

// waitForServerDown polls until the server on addr stops responding.
func waitForServerDown(addr string) error {
	const (
		interval = 100 * time.Millisecond
		timeout  = 5 * time.Second
	)
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if _, err := probeServer(addr, interval); err != nil {
			return nil
		}
		time.Sleep(interval)
	}
	return fmt.Errorf("server on %s did not shut down within %s", addr, timeout)
}

func doShutdown(addr string) error {
	result, err := probeServer(addr)
	if err != nil {
		return err
	}

	resp, err := result.client.Post(fmt.Sprintf("http://%s/_/api/shutdown", addr), "application/json", nil)
	if err != nil {
		return fmt.Errorf("failed to send shutdown request: %w", err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted {
		return fmt.Errorf("unexpected response from server: %s", resp.Status)
	}

	slog.Info("shutdown request sent", "addr", addr)
	fmt.Fprintf(os.Stderr, "mo: shutdown request sent to %s\n", addr)
	return nil
}

func doRestart(addr string) error {
	result, err := probeServer(addr)
	if err != nil {
		return err
	}

	resp, err := result.client.Post(fmt.Sprintf("http://%s/_/api/restart", addr), "application/json", nil)
	if err != nil {
		return fmt.Errorf("failed to send restart request: %w", err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted {
		return fmt.Errorf("unexpected response from server: %s", resp.Status)
	}

	slog.Info("restart request sent", "addr", addr)
	fmt.Fprintf(os.Stderr, "mo: restart request sent to %s\n", addr)
	return nil
}

func doUnwatch(addr string, patterns []string, groupName string) error {
	result, err := probeServer(addr)
	if err != nil {
		return err
	}

	for _, pat := range patterns {
		body, err := json.Marshal(map[string]string{
			"pattern": pat,
			"group":   groupName,
		})
		if err != nil {
			return fmt.Errorf("failed to marshal request: %w", err)
		}

		req, err := http.NewRequest(http.MethodDelete, fmt.Sprintf("http://%s/_/api/patterns", addr), bytes.NewReader(body))
		if err != nil {
			return fmt.Errorf("failed to create request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := result.client.Do(req) //nolint:gosec // URL is constructed from local addr, not user-supplied
		if err != nil {
			return fmt.Errorf("failed to send unwatch request: %w", err)
		}
		resp.Body.Close()

		if resp.StatusCode == http.StatusNotFound {
			return fmt.Errorf("watch pattern %q not found in group %q (use --status to see registered patterns)", pat, groupName)
		}
		if resp.StatusCode != http.StatusNoContent {
			return fmt.Errorf("unexpected response from server: %s", resp.Status)
		}

		slog.Info("pattern removed", "pattern", pat, "group", groupName)
		fmt.Fprintf(os.Stderr, "mo: unwatched %s\n", pat)
	}

	return nil
}

func doClose(addr string, paths []string, groupName string) ([]string, error) {
	result, err := probeServer(addr)
	if err != nil {
		return nil, err
	}

	resp, err := result.client.Get(fmt.Sprintf("http://%s/_/api/status", addr))
	if err != nil {
		return nil, fmt.Errorf("failed to get server status: %w", err)
	}
	defer resp.Body.Close()

	var status statusResponse
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		return nil, fmt.Errorf("failed to decode status: %w", err)
	}

	pathToID := make(map[string]string)
	for _, g := range status.Groups {
		if g.Name == groupName {
			for _, f := range g.Files {
				pathToID[f.Path] = f.ID
			}
			break
		}
	}

	var closedPaths []string
	var joinedErr error
	for _, p := range paths {
		absPath, err := filepath.Abs(p)
		if err != nil {
			joinedErr = errors.Join(joinedErr, fmt.Errorf("cannot resolve path %s: %w", p, err))
			continue
		}

		id, ok := pathToID[absPath]
		if !ok {
			joinedErr = errors.Join(joinedErr, fmt.Errorf("file %q not found in group %q (use --status to see files)", absPath, groupName))
			continue
		}

		req, err := http.NewRequest(http.MethodDelete,
			fmt.Sprintf("http://%s/_/api/files/%s", addr, id), nil)
		if err != nil {
			joinedErr = errors.Join(joinedErr, fmt.Errorf("failed to create request for %q: %w", absPath, err))
			continue
		}

		closeResp, err := result.client.Do(req)
		if err != nil {
			joinedErr = errors.Join(joinedErr, fmt.Errorf("failed to close %q: %w", absPath, err))
			continue
		}
		closeResp.Body.Close()

		if closeResp.StatusCode == http.StatusNotFound {
			joinedErr = errors.Join(joinedErr, fmt.Errorf("file %q not found", absPath))
			continue
		}
		if closeResp.StatusCode != http.StatusNoContent {
			joinedErr = errors.Join(joinedErr, fmt.Errorf("unexpected response for %q: %s", absPath, closeResp.Status))
			continue
		}

		slog.Info("file closed", "path", absPath, "id", id, "group", groupName)
		closedPaths = append(closedPaths, absPath)
	}

	return closedPaths, joinedErr
}

type statusResponse struct {
	Version  string `json:"version"`
	Revision string `json:"revision"`
	PID      int    `json:"pid"`
	Groups   []struct {
		Name  string `json:"name"`
		Files []struct {
			Name string `json:"name"`
			ID   string `json:"id"`
			Path string `json:"path"`
		} `json:"files"`
		Patterns []string `json:"patterns,omitempty"`
	} `json:"groups"`
}

func doStatus() error {
	ports := discoverPorts()
	if len(ports) == 0 {
		if jsonOutput {
			writeJSON([]jsonStatusEntry{})
		} else {
			fmt.Fprintln(os.Stderr, "mo: no mo server found")
		}
		return nil
	}

	client := &http.Client{Timeout: 2 * time.Second}
	found := false
	var jsonEntries []jsonStatusEntry

	for i, p := range ports {
		addr := fmt.Sprintf("localhost:%d", p)
		resp, err := client.Get(fmt.Sprintf("http://%s/_/api/status", addr))
		if err != nil {
			found = true
			if jsonOutput {
				jsonEntries = append(jsonEntries, jsonStatusEntry{
					URL:    fmt.Sprintf("http://%s", addr),
					Status: "stopped",
				})
			} else {
				fmt.Fprintf(os.Stdout, "http://%s (stopped)\n", addr)
				if i < len(ports)-1 {
					fmt.Fprintln(os.Stdout)
				}
			}
			continue
		}

		var status statusResponse
		if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
			resp.Body.Close()
			continue
		}
		resp.Body.Close()
		found = true

		if jsonOutput {
			entry := jsonStatusEntry{
				URL:      fmt.Sprintf("http://%s", addr),
				Status:   "running",
				PID:      status.PID,
				Version:  status.Version,
				Revision: status.Revision,
			}
			for _, g := range status.Groups {
				entry.Groups = append(entry.Groups, jsonStatusGroupEntry{
					Name:     g.Name,
					Files:    len(g.Files),
					Patterns: g.Patterns,
				})
			}
			jsonEntries = append(jsonEntries, entry)
		} else {
			ver := status.Version
			if status.Revision != "" {
				ver += " " + status.Revision
			}
			fmt.Fprintf(os.Stdout, "http://%s (pid %d, %s)\n", addr, status.PID, ver)
			for _, g := range status.Groups {
				fmt.Fprintf(os.Stdout, "  %s: %d file(s)\n", g.Name, len(g.Files))
				if len(g.Patterns) > 0 {
					fmt.Fprintf(os.Stdout, "    watching: %s\n", strings.Join(g.Patterns, ", "))
				}
			}
			if i < len(ports)-1 {
				fmt.Fprintln(os.Stdout)
			}
		}
	}

	if jsonOutput {
		if !found {
			jsonEntries = []jsonStatusEntry{}
		}
		writeJSON(jsonEntries)
	} else if !found {
		fmt.Fprintln(os.Stderr, "mo: no mo server found")
	}

	return nil
}

func discoverPorts() []int {
	dir, err := logfile.Dir()
	if err != nil {
		return nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}

	var ports []int
	for _, e := range entries {
		name := e.Name()
		// Match "mo-{port}.log"
		if !strings.HasPrefix(name, "mo-") || !strings.HasSuffix(name, ".log") {
			continue
		}
		// Exclude rotated backups like "mo-6275.log.1"
		raw := strings.TrimSuffix(strings.TrimPrefix(name, "mo-"), ".log")
		p, err := strconv.Atoi(raw)
		if err != nil {
			continue
		}
		ports = append(ports, p)
	}
	sort.Ints(ports)
	return ports
}

func startServer(ctx context.Context, addr string, filesByGroup map[string][]string, patternsByGroup map[string][]string, uploadedFiles []server.UploadedFileData) error {
	sigCtx, stop := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	ctx, cancel := donegroup.WithCancel(sigCtx)
	cleanedUp := false
	cleanup := func() {
		if cleanedUp {
			return
		}
		cleanedUp = true
		cancel()
		if err := donegroup.WaitWithTimeout(ctx, 5*time.Second); err != nil {
			slog.Error("shutdown error", "error", err)
		}
	}
	defer cleanup()

	state := server.NewState(ctx)

	state.EnableBackup(ctx, func(data server.RestoreData) {
		if err := backup.Save(port, data); err != nil {
			slog.Warn("failed to save backup", "error", err)
		}
	})

	var deeplinks []deeplinkEntry
	var totalFiles, skippedFiles int
	for group, files := range filesByGroup {
		for _, f := range files {
			totalFiles++
			entry, err := state.AddFile(f, group)
			if err != nil {
				skippedFiles++
				slog.Warn("skipping file", "path", f, "error", err)
				continue
			}
			deeplinks = append(deeplinks, deeplinkEntry{
				URL:  buildDeeplink(addr, group, entry.ID),
				Path: entry.Path,
			})
		}
	}
	var patternsAdded int
	for group, pats := range patternsByGroup {
		for _, pat := range pats {
			entries, err := state.AddPattern(pat, group)
			if err != nil {
				slog.Warn("failed to add pattern", "pattern", pat, "error", err)
				continue
			}
			patternsAdded++
			for _, entry := range entries {
				deeplinks = append(deeplinks, deeplinkEntry{
					URL:  buildDeeplink(addr, group, entry.ID),
					Path: entry.Path,
				})
			}
		}
	}

	for _, uf := range uploadedFiles {
		state.AddUploadedFile(uf.Name, uf.Content, uf.Group)
	}

	if totalFiles > 0 && skippedFiles == totalFiles && patternsAdded == 0 && len(uploadedFiles) == 0 {
		return fmt.Errorf("all %d file(s) were skipped", totalFiles)
	}

	handler := server.NewHandler(state)

	srv := &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
	}

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("cannot listen on %s: %w", addr, err)
	}

	emitServeOutput(addr, deeplinks, true)

	if err := donegroup.Cleanup(ctx, func() error {
		state.CloseAllSubscribers()
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCancel()
		return srv.Shutdown(shutdownCtx)
	}); err != nil {
		return fmt.Errorf("failed to register cleanup: %w", err)
	}

	go func() {
		slog.Info("serving", "url", fmt.Sprintf("http://%s", addr))
		if err := srv.Serve(ln); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "error", err)
		}
	}()

	openBrowser(addr)

	select {
	case <-ctx.Done():
		slog.Info("shutting down")
	case restoreFile := <-state.RestartCh():
		slog.Info("restarting")
		// Cleanup releases the port (CloseAllSubscribers + srv.Shutdown)
		// before we spawn the new process.
		cleanup()
		_, err := spawnNewProcess(addr, restoreFile)
		return err
	case <-state.ShutdownCh():
		slog.Info("shutting down (requested via API)")
	}

	return nil
}

func spawnNewProcess(addr string, restoreFile string) (*os.Process, error) {
	binPath, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("cannot find binary: %w", err)
	}

	h, p, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, fmt.Errorf("cannot parse addr: %w", err)
	}

	args := []string{"--port", p, "--bind", h, "--no-open", "--foreground"}
	if restoreFile != "" {
		args = append(args, "--restore", restoreFile)
	}
	cmd := exec.Command(binPath, args...) //nolint:gosec
	setSysProcAttr(cmd)
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start new process: %w", err)
	}

	slog.Info("new process started", "pid", cmd.Process.Pid) //nolint:gosec // PID is from our own child process
	return cmd.Process, nil
}

func startBackground(addr string, filesByGroup map[string][]string, patternsByGroup map[string][]string, uploadedFiles []server.UploadedFileData) error {
	restoreFile, err := server.WriteRestoreFile(server.RestoreData{Groups: filesByGroup, Patterns: patternsByGroup, UploadedFiles: uploadedFiles})
	if err != nil {
		return err
	}

	proc, err := spawnNewProcess(addr, restoreFile)
	if err != nil {
		os.Remove(restoreFile)
		return err
	}
	pid := proc.Pid
	// Detach so the child survives parent exit.
	if err := proc.Release(); err != nil {
		slog.Warn("failed to release process", "error", err)
	}

	status, err := waitForReady(addr, 10*time.Second)
	if err != nil {
		return fmt.Errorf("%w (pid %d)", err, pid)
	}

	var deeplinks []deeplinkEntry
	if status != nil {
		for _, g := range status.Groups {
			for _, f := range g.Files {
				deeplinks = append(deeplinks, deeplinkEntry{
					URL:  buildDeeplink(addr, g.Name, f.ID),
					Path: f.Path,
					Name: f.Name,
				})
			}
		}
	}
	emitServeOutput(addr, deeplinks, true)
	fmt.Fprintf(os.Stderr, "mo: serving at http://%s (pid %d)\n", addr, pid)

	openBrowser(addr)

	return nil
}

func openBrowser(addr string) {
	if noOpen {
		return
	}
	url := fmt.Sprintf("http://%s", addr)
	if target != server.DefaultGroup {
		url = fmt.Sprintf("%s/%s", url, target)
	}
	if err := browser.OpenURL(url); err != nil {
		slog.Warn("could not open browser", "error", err)
	}
}

func waitForReady(addr string, timeout time.Duration) (*statusResponse, error) {
	client := &http.Client{Timeout: 500 * time.Millisecond}
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		resp, err := client.Get(fmt.Sprintf("http://%s/_/api/status", addr))
		if err == nil {
			if resp.StatusCode == http.StatusOK {
				var status statusResponse
				if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
					resp.Body.Close()
					return nil, nil //nolint:nilerr // decode failure is non-fatal; server is ready
				}
				resp.Body.Close()
				return &status, nil
			}
			resp.Body.Close()
		}
		time.Sleep(50 * time.Millisecond)
	}

	return nil, fmt.Errorf("server did not become ready within %s (check log file for details)", timeout)
}
