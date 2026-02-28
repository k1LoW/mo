# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What is mo

`mo` is a CLI tool that opens Markdown files in a browser with live-reload. It runs a Go HTTP server that embeds a React SPA as a single binary. The Go module is `github.com/k1LoW/mo`.

## Build & Run

Requires Go and [pnpm](https://pnpm.io/). Node.js version is managed via `pnpm.executionEnv.nodeVersion` in `internal/frontend/package.json`.

```bash
# Full build (frontend + Go binary, with ldflags)
make build

# Dev: build frontend then run with args
make dev ARGS="testdata/basic.md"

# Frontend code generation only (called by make build/dev via go generate)
make generate

# Run tests
make test

# Run linters (golangci-lint + gostyle)
make lint

# CI target (install dev deps + generate + test)
make ci
```

## Architecture

**Go backend + embedded React SPA**, single binary.

- `cmd/root.go` — CLI entry point (Cobra). Handles single-instance detection: if a server is already running on the port, adds files via HTTP API instead of starting a new server.
- `internal/server/server.go` — HTTP server, state management (mutex-guarded), SSE for live-reload, file watcher (fsnotify). All API routes use `/_/` prefix to avoid collision with SPA route paths (group names).
- `internal/static/static.go` — `go:generate` runs the frontend build, then `go:embed` embeds the output from `internal/static/dist/`.
- `internal/frontend/` — Vite + React 19 + TypeScript + Tailwind CSS v4 SPA. Build output goes to `internal/static/dist/` (configured in `vite.config.ts`).
- `version/version.go` — Version info, updated by tagpr on release. Build embeds revision via ldflags.

## Frontend

- Package manager: **pnpm** (version specified in `internal/frontend/package.json` `packageManager` field)
- Markdown rendering: `react-markdown` + `remark-gfm` + `rehype-raw` + `rehype-slug` (heading IDs) + `@shikijs/rehype` (syntax highlighting) + `mermaid` (diagram rendering)
- SPA routing via `window.location.pathname` (no router library)
- Key components: `App.tsx` (routing/state), `Sidebar.tsx` (file list, resizable), `MarkdownViewer.tsx` (rendering + raw view toggle), `TocPanel.tsx` (table of contents, resizable), `ThemeToggle.tsx` (dark/light mode), `GroupDropdown.tsx` (group switcher)
- Custom hooks: `useSSE.ts` (SSE subscription with auto-reconnect), `useApi.ts` (typed API fetch wrappers), `useActiveHeading.ts` (scroll-based active heading tracking via IntersectionObserver)
- Theme: GitHub-style light/dark via CSS custom properties (`--color-gh-*`) in `styles/app.css`, toggled by `data-theme` attribute on `<html>`. UI components use Tailwind classes like `bg-gh-bg-sidebar`, `text-gh-text-secondary`, etc.
- Toggle button pattern: `RawToggle.tsx` and `TocToggle.tsx` follow the same style (`bg-transparent border border-gh-border rounded-md p-1.5 text-gh-text-secondary`). New toolbar buttons should match this pattern.

## Key Design Patterns

- **Single instance**: CLI probes `/_/api/groups` on the target port. If already running, pushes files via `POST /_/api/files` and exits.
- **File IDs, not paths**: Files get sequential integer IDs server-side. Paths are never exposed to the client (`json:"-"`).
- **Tab groups**: Files are organized into named groups. Group name maps to the URL path (e.g., `/design`). Default group name is `"default"`.
- **Live-reload via SSE**: fsnotify watches files; `file-changed` events trigger frontend to re-fetch content by file ID.
- **Resizable panels**: Both `Sidebar.tsx` (left) and `TocPanel.tsx` (right) use the same drag-to-resize pattern with localStorage persistence. Left sidebar uses `e.clientX`, right panel uses `window.innerWidth - e.clientX`.
- **Toolbar buttons in content area**: The toolbar column (ToC + Raw toggles) lives inside `MarkdownViewer.tsx`, positioned with `shrink-0 flex flex-col gap-2 -mr-4 -mt-4` to align with the header.

## API Conventions

All internal endpoints use `/_/api/` prefix and SSE uses `/_/events`. The `/_/` prefix avoids collisions with user-facing group name routes.

## CI/CD

- **CI**: golangci-lint (via reviewdog), gostyle, `make ci` (test + coverage), octocov
- **Release**: tagpr for automated tagging, goreleaser for cross-platform builds. The `go generate` step (frontend build) runs in goreleaser's `before.hooks`.
- **License check**: Trivy scans for license issues
- CI requires pnpm setup (`pnpm/action-setup`) before any Go build step because `go generate` triggers the frontend build.
