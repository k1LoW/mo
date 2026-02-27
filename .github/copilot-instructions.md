# Copilot Instructions for mo (Markdown Opener)

## What is mo

`mo` is a CLI tool that opens Markdown files in a browser with live-reload. It runs a Go HTTP server that embeds a React SPA and serves rendered Markdown. The Go module is `github.com/k1LoW/mo`.

## Build & Run

```bash
# Full build (frontend + Go binary)
make build

# Dev: build frontend then run with args
make dev ARGS="testdata/basic.md"

# Frontend code generation only (called by make build/dev via go generate)
go generate ./internal/static/

# Go binary only (requires frontend already built)
go build -o mo .
```

There are currently no Go tests or linters configured.

## Architecture

**Go backend + embedded React SPA**, single binary.

- `cmd/root.go` — CLI entry point (cobra). Handles instance detection: if a server is already running on the port, adds files via HTTP API instead of starting a new one.
- `internal/server/server.go` — HTTP server, state management, SSE for live-reload, file watcher (fsnotify). All API routes use `/_/` prefix to avoid collision with SPA route paths (group names).
- `internal/static/static.go` — `go:generate` runs the frontend build, then `go:embed` embeds the output from `internal/static/dist/`.
- `internal/frontend/` — Vite 7 + React 19 + TypeScript + Tailwind CSS v4 SPA. Build output goes to `internal/static/dist/` (configured in `vite.config.ts`).

## API Conventions

All internal API endpoints are under `/_/api/` and SSE under `/_/events`. The `/_/` prefix is intentional to avoid collisions with user-facing group name routes (e.g., `/mygroup`).

## Frontend

- Located in `internal/frontend/`, uses **pnpm** as the package manager.
- React 19, TypeScript, Tailwind CSS v4, Vite 7.
- Markdown rendering: `react-markdown` + `remark-gfm` + `rehype-raw` + `@shikijs/rehype` (syntax highlighting) + `mermaid` (diagram rendering).
- SPA routing via `window.location.pathname` (no router library).
- Key components: `App.tsx` (routing/state), `Sidebar.tsx` (file tabs), `MarkdownViewer.tsx` (rendering), `ThemeToggle.tsx` (dark/light mode).
- Custom hooks: `useSSE.ts` (SSE subscription), `useApi.ts` (API calls).

## Key Patterns

- **Single instance design**: The CLI checks if a server is already running on the target port before starting a new one. If running, it sends files via `POST /_/api/files` and exits.
- **File IDs**: Files are assigned sequential integer IDs on the server side. The frontend references files by ID, not path (paths are never exposed to the client).
- **Tab groups**: Files are organized into named groups (default: "default"). Group name maps to the URL path.
