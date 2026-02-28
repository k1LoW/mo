# Copilot Instructions for mo (Markdown Opener)

## What is mo

`mo` is a CLI tool that opens Markdown files in a browser with live-reload. It runs a Go HTTP server that embeds a React SPA and serves rendered Markdown. The Go module is `github.com/k1LoW/mo`.

## Build & Run

Requires Go and [pnpm](https://pnpm.io/). Node.js version is managed via `pnpm.executionEnv.nodeVersion` in `internal/frontend/package.json`.

```bash
# Full build (frontend + Go binary, with ldflags)
make build

# Dev: build frontend then run with args
make dev ARGS="testdata/basic.md"

# Frontend code generation only
make generate

# Run tests
make test

# Run linters (golangci-lint + gostyle)
make lint
```

## Architecture

**Go backend + embedded React SPA**, single binary.

- `cmd/root.go` — CLI entry point (Cobra). Handles instance detection: if a server is already running on the port, adds files via HTTP API instead of starting a new one.
- `internal/server/server.go` — HTTP server, state management (mutex-guarded), SSE for live-reload, file watcher (fsnotify). All API routes use `/_/` prefix to avoid collision with SPA route paths (group names).
- `internal/static/static.go` — `go:generate` runs the frontend build, then `go:embed` embeds the output from `internal/static/dist/`.
- `internal/frontend/` — Vite + React 19 + TypeScript + Tailwind CSS v4 SPA. Build output goes to `internal/static/dist/` (configured in `vite.config.ts`).
- `version/version.go` — Version info, updated by tagpr on release.

## API Conventions

All internal API endpoints are under `/_/api/` and SSE under `/_/events`. The `/_/` prefix is intentional to avoid collisions with user-facing group name routes (e.g., `/mygroup`).

## Frontend

- Located in `internal/frontend/`, uses **pnpm** as the package manager.
- React 19, TypeScript, Tailwind CSS v4.
- Markdown rendering: `react-markdown` + `remark-gfm` + `rehype-raw` + `rehype-slug` (heading IDs) + `@shikijs/rehype` (syntax highlighting) + `mermaid` (diagram rendering).
- SPA routing via `window.location.pathname` (no router library).
- Key components: `App.tsx` (routing/state), `Sidebar.tsx` (file list, resizable), `MarkdownViewer.tsx` (rendering + raw view toggle), `TocPanel.tsx` (table of contents, resizable), `ThemeToggle.tsx` (dark/light mode), `GroupDropdown.tsx` (group switcher).
- Custom hooks: `useSSE.ts` (SSE subscription with auto-reconnect), `useApi.ts` (typed API fetch wrappers), `useActiveHeading.ts` (scroll-based active heading tracking via IntersectionObserver).
- Theme: GitHub-style light/dark via CSS custom properties (`--color-gh-*`) in `styles/app.css`, toggled by `data-theme` attribute on `<html>`. UI components use Tailwind classes like `bg-gh-bg-sidebar`, `text-gh-text-secondary`, etc.
- Toggle button pattern: `RawToggle.tsx` and `TocToggle.tsx` follow the same style (`bg-transparent border border-gh-border rounded-md p-1.5 text-gh-text-secondary`). New toolbar buttons should match this pattern.

## Key Patterns

- **Single instance design**: The CLI checks if a server is already running on the target port before starting a new one. If running, it sends files via `POST /_/api/files` and exits.
- **File IDs**: Files are assigned sequential integer IDs on the server side. The frontend references files by ID, not path (paths are never exposed to the client).
- **Tab groups**: Files are organized into named groups (default: "default"). Group name maps to the URL path.
- **Live-reload via SSE**: fsnotify watches files; `file-changed` events trigger frontend to re-fetch content by file ID.
- **Resizable panels**: Both `Sidebar.tsx` (left) and `TocPanel.tsx` (right) use the same drag-to-resize pattern with localStorage persistence. Left sidebar uses `e.clientX`, right panel uses `window.innerWidth - e.clientX`.
- **Toolbar buttons in content area**: The toolbar column (ToC + Raw toggles) lives inside `MarkdownViewer.tsx`, positioned with `shrink-0 flex flex-col gap-2 -mr-4 -mt-4` to align with the header.
