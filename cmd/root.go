package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/k1LoW/donegroup"
	"github.com/k1LoW/mo/internal/server"
	"github.com/k1LoW/mo/version"
	"github.com/pkg/browser"
	"github.com/spf13/cobra"
)

var (
	target string
	port   int
)

var rootCmd = &cobra.Command{
	Use:   "mo [flags] [FILE ...]",
	Short: "mo is a Markdown viewer that opens .md files in a browser.",
	Long: `mo is a Markdown viewer that opens .md files in a browser with live-reload.

It starts an HTTP server, renders Markdown files using a built-in React SPA,
and automatically refreshes the browser when files are saved.

Examples:
  mo README.md                          Open a single file
  mo README.md CHANGELOG.md docs/*.md   Open multiple files
  mo spec.md --target design            Open in a named group
  mo draft.md --port 6276               Use a different port

Single Server, Multiple Files:
  By default, mo runs a single server process on port 6275.
  If a server is already running on the same port, subsequent mo invocations
  add files to the existing session instead of starting a new one.

  $ mo README.md          # Starts a server and opens the browser
  $ mo CHANGELOG.md       # Adds the file to the running server

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

Live-Reload:
  mo watches all opened files for changes using filesystem notifications.
  When a file is saved, the browser automatically re-renders the content.

Supported Markdown Features:
  - GitHub Flavored Markdown (tables, task lists, strikethrough, autolinks)
  - Syntax-highlighted code blocks (via Shiki)
  - Mermaid diagrams
  - Raw HTML`,
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
	rootCmd.Flags().StringVarP(&target, "target", "t", "default", "Tab group name")
	rootCmd.Flags().IntVarP(&port, "port", "p", 6275, "Server port")
}

func run(cmd *cobra.Command, args []string) error {
	files, err := resolveFiles(args)
	if err != nil {
		return err
	}

	addr := fmt.Sprintf("localhost:%d", port)

	if len(files) > 0 && tryAddToExisting(addr, files) {
		return nil
	}

	return startServer(cmd.Context(), addr, files)
}

func resolveFiles(args []string) ([]string, error) {
	var files []string
	for _, arg := range args {
		absPath, err := filepath.Abs(arg)
		if err != nil {
			return nil, fmt.Errorf("cannot resolve path %s: %w", arg, err)
		}
		if _, err := os.Stat(absPath); err != nil {
			return nil, fmt.Errorf("file not found: %s", absPath)
		}
		files = append(files, absPath)
	}
	return files, nil
}

func tryAddToExisting(addr string, files []string) bool {
	client := &http.Client{Timeout: 500 * time.Millisecond}

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

	for _, f := range files {
		body, err := json.Marshal(map[string]string{
			"path":  f,
			"group": target,
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to marshal request for %s: %v\n", f, err)
			continue
		}
		resp, err := client.Post(
			fmt.Sprintf("http://%s/_/api/files", addr),
			"application/json",
			bytes.NewReader(body),
		)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to add file %s: %v\n", f, err)
			continue
		}
		resp.Body.Close()
	}

	fmt.Fprintf(os.Stderr, "Added %d file(s) to existing server at %s\n", len(files), addr)

	if isNewGroup {
		url := fmt.Sprintf("http://%s/%s", addr, target)
		if err := browser.OpenURL(url); err != nil {
			fmt.Fprintf(os.Stderr, "Could not open browser: %v\n", err)
		}
	}

	return true
}

func startServer(ctx context.Context, addr string, files []string) error {
	state := server.NewState()

	for _, f := range files {
		state.AddFile(f, target)
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

	sigCtx, stop := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	ctx, cancel := donegroup.WithCancel(sigCtx)
	defer func() {
		cancel()
		if err := donegroup.WaitWithTimeout(ctx, 5*time.Second); err != nil {
			fmt.Fprintf(os.Stderr, "shutdown: %v\n", err)
		}
	}()

	if err := donegroup.Cleanup(ctx, func() error {
		state.CloseAllSubscribers()
		return srv.Shutdown(context.Background())
	}); err != nil {
		return fmt.Errorf("failed to register cleanup: %w", err)
	}

	go func() {
		fmt.Fprintf(os.Stderr, "Serving on http://%s\n", addr)
		if err := srv.Serve(ln); err != nil && err != http.ErrServerClosed {
			fmt.Fprintf(os.Stderr, "server error: %v\n", err)
		}
	}()

	url := fmt.Sprintf("http://%s", addr)
	if target != "default" {
		url = fmt.Sprintf("%s/%s", url, target)
	}
	if err := browser.OpenURL(url); err != nil {
		fmt.Fprintf(os.Stderr, "Could not open browser: %v\n", err)
	}

	<-ctx.Done()
	fmt.Fprintln(os.Stderr, "\nShutting down...")

	return nil
}
