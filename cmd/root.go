package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/k1LoW/donegroup"
	"github.com/k1LoW/mo/internal/logfile"
	"github.com/k1LoW/mo/internal/server"
	"github.com/k1LoW/mo/version"
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
	target         string
	port           int
	open           bool
	noOpen         bool
	restore        string
	shutdownServer bool
	foreground     bool
	statusServer   bool
	watchPatterns   []string
	unwatchPatterns []string
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

  Use --foreground to keep the mo server in the foreground.

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
  $ mo --unwatch '**/*.md'            Stop watching a pattern`,
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
	rootCmd.Flags().BoolVar(&open, "open", false, "Always open browser (even when adding to existing group)")
	rootCmd.Flags().BoolVar(&noOpen, "no-open", false, "Do not open browser automatically")
	rootCmd.MarkFlagsMutuallyExclusive("open", "no-open")
	rootCmd.Flags().BoolVar(&shutdownServer, "shutdown", false, "Shut down the running mo server on the specified port")
	rootCmd.Flags().StringVar(&restore, "restore", "", "Restore state from file (internal use)")
	rootCmd.Flags().MarkHidden("restore") //nolint:errcheck
	rootCmd.Flags().BoolVar(&foreground, "foreground", false, "Run mo server in foreground (do not background)")
	rootCmd.Flags().BoolVar(&statusServer, "status", false, "Show status of all running mo servers")
	rootCmd.Flags().StringArrayVarP(&watchPatterns, "watch", "w", nil, "Glob pattern to watch for matching files (repeatable)")
	rootCmd.Flags().StringArrayVar(&unwatchPatterns, "unwatch", nil, "Remove a watched glob pattern (repeatable)")
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

	addr := fmt.Sprintf("localhost:%d", port)

	if statusServer {
		return doStatus()
	}

	if shutdownServer {
		return doShutdown(addr)
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

	if restore != "" {
		filesByGroup, patternsByGroup, err := loadRestoreData(restore)
		if err != nil {
			return fmt.Errorf("failed to restore state: %w", err)
		}
		return startServer(cmd.Context(), addr, filesByGroup, patternsByGroup)
	}

	resolved, err := server.ResolveGroupName(target)
	if err != nil {
		return fmt.Errorf("invalid target group name %q: %w", target, err)
	}
	target = resolved

	if len(watchPatterns) > 0 && len(args) > 0 {
		hasGlob := false
		for _, p := range watchPatterns {
			if hasGlobChars(p) {
				hasGlob = true
				break
			}
		}
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

	if (len(files) > 0 || len(patterns) > 0) && tryAddToExisting(addr, files, patterns) {
		return nil
	}

	filesByGroup := map[string][]string{target: files}
	var patternsByGroup map[string][]string
	if len(patterns) > 0 {
		patternsByGroup = map[string][]string{target: patterns}
	}

	if foreground {
		return startServer(cmd.Context(), addr, filesByGroup, patternsByGroup)
	}
	return startBackground(addr, filesByGroup, patternsByGroup)
}

func loadRestoreData(path string) (map[string][]string, map[string][]string, error) {
	data, err := os.ReadFile(path) //nolint:gosec
	if err != nil {
		return nil, nil, err
	}
	os.Remove(path)

	var rd server.RestoreData
	if err := json.Unmarshal(data, &rd); err != nil {
		return nil, nil, err
	}
	return rd.Groups, rd.Patterns, nil
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
	client, err := probeServer(addr, probeTimeoutFast)
	if err != nil {
		return false
	}

	resp, err := client.Get(fmt.Sprintf("http://%s/_/api/groups", addr))
	if err != nil {
		return false
	}

	var existingGroups []struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&existingGroups); err != nil {
		resp.Body.Close()
		return false
	}
	resp.Body.Close()

	isNewGroup := true
	for _, g := range existingGroups {
		if g.Name == target {
			isNewGroup = false
			break
		}
	}

	postItems(client, addr, "/_/api/files", "path", target, files)
	postItems(client, addr, "/_/api/patterns", "pattern", target, patterns)

	added := len(files) + len(patterns)
	slog.Info("added to existing server", "files", len(files), "patterns", len(patterns), "addr", addr)
	fmt.Fprintf(os.Stderr, "mo: added %d item(s) to http://%s\n", added, addr)

	if isNewGroup || open {
		openBrowser(addr)
	}

	return true
}

func postItems(client *http.Client, addr, endpoint, key, group string, items []string) {
	for _, item := range items {
		body, err := json.Marshal(map[string]string{
			key:     item,
			"group": group,
		})
		if err != nil {
			slog.Warn("failed to marshal request", key, item, "error", err)
			continue
		}
		resp, err := client.Post(
			fmt.Sprintf("http://%s%s", addr, endpoint),
			"application/json",
			bytes.NewReader(body),
		)
		if err != nil {
			slog.Warn("failed to post item", key, item, "error", err)
			continue
		}
		resp.Body.Close()
	}
}

// probeServer checks that a mo server is running on addr by calling
// GET /_/api/status and validating the response contains a version field.
func probeServer(addr string, timeout ...time.Duration) (*http.Client, error) {
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

	var status struct {
		Version string `json:"version"`
		PID     int    `json:"pid"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil || status.Version == "" {
		return nil, fmt.Errorf("server on %s is not a mo instance", addr)
	}
	return client, nil
}

func doShutdown(addr string) error {
	client, err := probeServer(addr)
	if err != nil {
		return err
	}

	resp, err := client.Post(fmt.Sprintf("http://%s/_/api/shutdown", addr), "application/json", nil)
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

func doUnwatch(addr string, patterns []string, groupName string) error {
	client, err := probeServer(addr)
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

		resp, err := client.Do(req) //nolint:gosec // URL is constructed from local addr, not user-supplied
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

type statusResponse struct {
	Version  string `json:"version"`
	Revision string `json:"revision"`
	PID      int    `json:"pid"`
	Groups   []struct {
		Name  string `json:"name"`
		Files []struct {
			Name string `json:"name"`
			ID   int    `json:"id"`
			Path string `json:"path"`
		} `json:"files"`
		Patterns []string `json:"patterns,omitempty"`
	} `json:"groups"`
}

func doStatus() error {
	ports := discoverPorts()
	if len(ports) == 0 {
		fmt.Fprintln(os.Stderr, "mo: no mo server found")
		return nil
	}

	client := &http.Client{Timeout: 2 * time.Second}
	found := false

	for i, p := range ports {
		addr := fmt.Sprintf("localhost:%d", p)
		resp, err := client.Get(fmt.Sprintf("http://%s/_/api/status", addr))
		if err != nil {
			fmt.Fprintf(os.Stderr, "http://%s (stopped)\n", addr)
			if i < len(ports)-1 {
				fmt.Fprintln(os.Stderr)
			}
			found = true
			continue
		}

		var status statusResponse
		if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
			resp.Body.Close()
			continue
		}
		resp.Body.Close()
		found = true

		ver := status.Version
		if status.Revision != "" {
			ver += " " + status.Revision
		}
		fmt.Fprintf(os.Stderr, "http://%s (pid %d, %s)\n", addr, status.PID, ver)
		for _, g := range status.Groups {
			fmt.Fprintf(os.Stderr, "  %s: %d file(s)\n", g.Name, len(g.Files))
			if len(g.Patterns) > 0 {
				fmt.Fprintf(os.Stderr, "    watching: %s\n", strings.Join(g.Patterns, ", "))
			}
		}
		if i < len(ports)-1 {
			fmt.Fprintln(os.Stderr)
		}
	}

	if !found {
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

func startServer(ctx context.Context, addr string, filesByGroup map[string][]string, patternsByGroup map[string][]string) error {
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

	for group, files := range filesByGroup {
		for _, f := range files {
			state.AddFile(f, group)
		}
	}

	for group, pats := range patternsByGroup {
		for _, pat := range pats {
			if _, err := state.AddPattern(pat, group); err != nil {
				slog.Warn("failed to add pattern", "pattern", pat, "error", err)
			}
		}
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

	_, p, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, fmt.Errorf("cannot parse addr: %w", err)
	}

	cmd := exec.Command(binPath, "--port", p, "--no-open", "--foreground", "--restore", restoreFile) //nolint:gosec
	setSysProcAttr(cmd)
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start new process: %w", err)
	}

	slog.Info("new process started", "pid", cmd.Process.Pid) //nolint:gosec // PID is from our own child process
	return cmd.Process, nil
}

func startBackground(addr string, filesByGroup map[string][]string, patternsByGroup map[string][]string) error {
	restoreFile, err := server.WriteRestoreFile(server.RestoreData{Groups: filesByGroup, Patterns: patternsByGroup})
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

	if err := waitForReady(addr, 10*time.Second); err != nil {
		return fmt.Errorf("%w (pid %d)", err, pid)
	}

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

func waitForReady(addr string, timeout time.Duration) error {
	client := &http.Client{Timeout: 500 * time.Millisecond}
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		resp, err := client.Get(fmt.Sprintf("http://%s/_/api/status", addr))
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return nil
			}
		}
		time.Sleep(50 * time.Millisecond)
	}

	return fmt.Errorf("server did not become ready within %s (check log file for details)", timeout)
}
