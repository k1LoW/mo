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
	Use:     "mo [flags] [FILE ...]",
	Short:   "mo is a Markdown viewer that opens .md files in a browser.",
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

	return startServer(addr, files)
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
	json.NewDecoder(resp.Body).Decode(&existingGroups)
	resp.Body.Close()

	isNewGroup := true
	for _, g := range existingGroups {
		if g.Name == target {
			isNewGroup = false
			break
		}
	}

	for _, f := range files {
		body, _ := json.Marshal(map[string]string{
			"path":  f,
			"group": target,
		})
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

func startServer(addr string, files []string) error {
	state := server.NewState()

	for _, f := range files {
		state.AddFile(f, target)
	}

	handler := server.NewHandler(state)

	srv := &http.Server{
		Addr:    addr,
		Handler: handler,
	}

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("cannot listen on %s: %w", addr, err)
	}

	sigCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	ctx, cancel := donegroup.WithCancel(sigCtx)
	defer func() {
		cancel()
		if err := donegroup.WaitWithTimeout(ctx, 5*time.Second); err != nil {
			fmt.Fprintf(os.Stderr, "shutdown: %v\n", err)
		}
	}()

	donegroup.Cleanup(ctx, func() error {
		state.CloseAllSubscribers()
		return srv.Shutdown(context.Background())
	})

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
