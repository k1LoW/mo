# Copilot Instructions for mo (Markdown Opener)

## What is mo

`mo` is a CLI tool that opens Markdown files in a browser with live-reload. It runs a Go HTTP server that embeds a React SPA as a single binary. The Go module is `github.com/k1LoW/mo`.

## Build & Run

Requires Go and [pnpm](https://pnpm.io/). Node.js version is managed via `pnpm.executionEnv.nodeVersion` in `internal/frontend/package.json`.

```bash
# Full build (frontend + Go binary, with ldflags)
make build

# Dev: build frontend then run with args (uses port 16275, foreground mode)
make dev ARGS="testdata/basic.md"

# Dev with tab groups (-t can only specify one group per invocation)
make dev ARGS="-t design testdata/basic.md"

# Frontend code generation only (called by make build/dev via go generate)
make generate

# Run all tests (frontend + Go)
make test

# Run a single frontend test (vitest)
cd internal/frontend && pnpm test src/utils/buildTree.test.ts

# Run Go tests only
go test ./...

# Run a single Go test
go test ./internal/server/ -run TestHandleFiles

# Run linters (oxlint for frontend, golangci-lint + gostyle for Go)
make lint

# CI target (install dev deps + generate + test)
make ci
```

### CLI Flags

- `--port` / `-p` — Server port (default: 6275)
- `--target` / `-t` — Tab group name (default: `"default"`)
- `--open` — Always open browser
- `--no-open` — Never open browser
- `--watch` / `-w` — Boolean flag that turns on watch mode; directory and glob positional arguments are registered as watch patterns
- `--unwatch` — Boolean flag that removes watched patterns; directory and glob positional arguments specify which patterns to unwatch (with `-R`, a directory removes all patterns under it)
- `--recursive` / `-R` — Recurse into subdirectories when a directory is given as an argument
- `--close` — Close files instead of opening them
- `--clear` — Clear saved session for the specified port
- `--status` — Show status of all running mo servers
- `--shutdown` — Shut down the running mo server
- `--restart` — Restart the running mo server
- `--foreground` — Run mo server in foreground (do not background)
- `--json` — Output structured data as JSON to stdout
- `--dangerously-allow-remote-access` — Allow remote access without authentication (trusted networks only)

## Architecture

**Go backend + embedded React SPA**, single binary.

- `cmd/root.go` — CLI entry point (Cobra). Handles single-instance detection: if a server is already running on the port, adds files via HTTP API instead of starting a new one.
- `internal/server/server.go` — HTTP server, state management (mutex-guarded), SSE for live-reload, file watcher (fsnotify). All API routes use `/_/` prefix to avoid collision with SPA route paths (group names).
- `internal/static/static.go` — `go:generate` runs the frontend build, then `go:embed` embeds the output from `internal/static/dist/`.
- `internal/frontend/` — Vite + React 19 + TypeScript + Tailwind CSS v4 SPA. Build output goes to `internal/static/dist/` (configured in `vite.config.ts`).
- `version/version.go` — Version info, updated by tagpr on release. Build embeds revision via ldflags.
- `testdata/` — Sample Markdown files (GFM, mermaid, math, alerts, etc.) and fixture projects for tests and dev. Reuse these for new test cases.

## API Conventions

All internal API endpoints are under `/_/api/` and SSE under `/_/events`. The `/_/` prefix is intentional to avoid collisions with user-facing group name routes (e.g., `/mygroup`).

Key endpoints:
- `GET /_/api/groups` — List all groups with files
- `POST /_/api/files` — Add file
- `DELETE /_/api/files/{id}` — Remove file
- `GET /_/api/files/{id}/content` — File content (markdown)
- `PUT /_/api/files/{id}/group` — Move file to another group
- `PUT /_/api/reorder` — Reorder files in a group (group name in body)
- `POST /_/api/files/open` — Open relative file link
- `POST /_/api/patterns` — Add glob watch pattern
- `DELETE /_/api/patterns` — Remove glob watch pattern
- `GET /_/api/status` — Server status (version, pid, groups with patterns)
- `GET /_/events` — SSE (event types: `update`, `file-changed`, `restart`)

## Frontend

- Located in `internal/frontend/`, uses **pnpm** as the package manager.
- React 19, TypeScript, Tailwind CSS v4.
- Markdown rendering: `react-markdown` + `remark-gfm` + `rehype-raw` + `rehype-slug` (heading IDs) + `rehype-sanitize` + `@shikijs/rehype` (syntax highlighting) + `mermaid` (diagram rendering) + `remark-math` + `rehype-katex` (math/LaTeX) + `rehype-github-alerts` (GitHub-style alerts) + `react-zoom-pan-pinch` (image zoom).
- SPA routing via `window.location.pathname` (no router library).
- Key components: `App.tsx` (routing/state), `Sidebar.tsx` (file list with flat/tree view, resizable, drag-and-drop reorder), `TreeView.tsx` (tree view with collapsible directories), `MarkdownViewer.tsx` (rendering + raw view toggle), `TocPanel.tsx` (table of contents, resizable), `GroupDropdown.tsx` (group switcher), `FileContextMenu.tsx` (shared kebab menu for file operations), `WidthToggle.tsx` (wide/narrow content width toggle).
- Custom hooks: `useSSE.ts` (SSE subscription with auto-reconnect), `useApi.ts` (typed API fetch wrappers), `useActiveHeading.ts` (scroll-based active heading tracking via IntersectionObserver).
- Theme: GitHub-style light/dark via CSS custom properties (`--color-gh-*`) in `styles/app.css`, toggled by `data-theme` attribute on `<html>`. UI components use Tailwind classes like `bg-gh-bg-sidebar`, `text-gh-text-secondary`, etc.
- Toggle button pattern: `RawToggle.tsx` and `TocToggle.tsx` follow the same style (`bg-transparent border border-gh-border rounded-md p-1.5 text-gh-text-secondary`). Header buttons (`ViewModeToggle`, `ThemeToggle`, `WidthToggle`, sidebar toggle) use `text-gh-header-text` instead. New buttons should match the appropriate variant.

## Key Patterns

- **Single instance design**: CLI probes `/_/api/status` on the target port via `probeServer()`. If already running, pushes files via `POST /_/api/files` and exits.
- **File IDs**: Files get deterministic string IDs derived from the SHA-256 hash of the absolute path (first 8 hex characters). IDs are stable across server restarts, enabling deep linking. The frontend primarily references files by ID. Absolute paths are available via `FileEntry.path` for display.
- **Tab groups**: Files are organized into named groups (default: "default"). Group name maps to the URL path.
- **Live-reload via SSE**: fsnotify watches files; `file-changed` events trigger frontend to re-fetch content by file ID.
- **State persistence**: Server state (files, groups, patterns) is backed up to `$XDG_STATE_HOME/mo/backup/mo-<port>.json` via `internal/backup`. When starting a new server, backup is always restored and merged with CLI-specified files/patterns (restored entries first, CLI entries appended, duplicates skipped). The backup file is only deleted when the CLI is invoked with `--clear`.
- **Glob pattern watching**: `--watch` enables watch mode; positional arguments that are globs or directories are registered as patterns, expanded to matching files, and monitored for new files via fsnotify directory watches. Patterns are stored with reference-counted directory watches (`watchedDirs map[string]int`). `--unwatch` is a boolean flag; positional arguments (globs or directories) determine which patterns to remove. With `-R`, a directory argument removes all registered patterns under that directory prefix. Groups persist as long as they have files or patterns.
- **Resizable panels**: Both `Sidebar.tsx` (left) and `TocPanel.tsx` (right) use the same drag-to-resize pattern with localStorage persistence. Left sidebar uses `e.clientX`, right panel uses `window.innerWidth - e.clientX`.
- **Toolbar buttons in content area**: The toolbar column (ToC + Raw toggles) lives inside `MarkdownViewer.tsx`, positioned with `shrink-0 flex flex-col gap-2 -mr-4 -mt-4` to align with the header.
- **Sidebar view modes**: Flat (default, with drag-and-drop reorder via dnd-kit) and tree (hierarchical directory view). View mode is persisted per-group in localStorage. Collapsed directory state is managed inside `TreeView` and also persisted per-group.
- **localStorage conventions**: All keys use `mo-` prefix (e.g., `mo-sidebar-width`, `mo-sidebar-viewmode`, `mo-sidebar-tree-collapsed`, `mo-theme`). Read patterns use `try/catch` around `JSON.parse` with fallback defaults.

## CI/CD

- **CI**: golangci-lint (via reviewdog), gostyle, `make ci` (test + coverage), octocov
- **Release**: tagpr for automated tagging, goreleaser for cross-platform builds. The `go generate` step (frontend build) runs in goreleaser's `before.hooks`.
- **License check**: Trivy scans for license issues
- CI requires pnpm setup (`pnpm/action-setup`) before any Go build step because `go generate` triggers the frontend build.
