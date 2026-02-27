# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What is mo

`mo` is a CLI tool that opens Markdown files in a browser with live-reload. It runs a Go HTTP server that embeds a React SPA as a single binary. The Go module is `github.com/k1LoW/mo`.

## Build & Run

Requires Go and [pnpm](https://pnpm.io/).

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

There are currently no Go tests or linters configured. TypeScript strict mode is enabled in `tsconfig.json`.

## Architecture

**Go backend + embedded React SPA**, single binary.

- `cmd/root.go` — CLI entry point (Cobra). Handles single-instance detection: if a server is already running on the port, adds files via HTTP API instead of starting a new server.
- `internal/server/server.go` — HTTP server, state management (mutex-guarded), SSE for live-reload, file watcher (fsnotify). All API routes use `/_/` prefix to avoid collision with SPA route paths (group names).
- `internal/static/static.go` — `go:generate` runs the frontend build, then `go:embed` embeds the output from `internal/static/dist/`.
- `internal/frontend/` — Vite 7 + React 19 + TypeScript + Tailwind CSS v4 SPA. Build output goes to `internal/static/dist/` (configured in `vite.config.ts`).

## Frontend

- Package manager: **pnpm**
- Markdown rendering: `react-markdown` + `remark-gfm` + `rehype-raw` + `@shikijs/rehype` (syntax highlighting) + `mermaid` (diagram rendering)
- SPA routing via `window.location.pathname` (no router library)
- Key components: `App.tsx` (routing/state), `Sidebar.tsx` (file list), `MarkdownViewer.tsx` (rendering), `ThemeToggle.tsx` (dark/light mode), `GroupDropdown.tsx` (group switcher)
- Custom hooks: `useSSE.ts` (SSE subscription with auto-reconnect), `useApi.ts` (typed API fetch wrappers)

## Key Design Patterns

- **Single instance**: CLI probes `/_/api/groups` on the target port. If already running, pushes files via `POST /_/api/files` and exits.
- **File IDs, not paths**: Files get sequential integer IDs server-side. Paths are never exposed to the client (`json:"-"`).
- **Tab groups**: Files are organized into named groups. Group name maps to the URL path (e.g., `/design`). Default group name is `"default"`.
- **Live-reload via SSE**: fsnotify watches files; `file-changed` events trigger frontend to re-fetch content by file ID.

## API Conventions

All internal endpoints use `/_/api/` prefix and SSE uses `/_/events`. The `/_/` prefix avoids collisions with user-facing group name routes.
