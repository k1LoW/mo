<p align="center">
<br><br><br>
<img src="https://github.com/k1LoW/mo/raw/main/images/logo.svg" width="120" alt="mo">
<br><br><br>
</p>

# mo

[![build](https://github.com/k1LoW/mo/actions/workflows/ci.yml/badge.svg)](https://github.com/k1LoW/mo/actions/workflows/ci.yml) ![Coverage](https://raw.githubusercontent.com/k1LoW/octocovs/main/badges/k1LoW/mo/coverage.svg) ![Code to Test Ratio](https://raw.githubusercontent.com/k1LoW/octocovs/main/badges/k1LoW/mo/ratio.svg) ![Test Execution Time](https://raw.githubusercontent.com/k1LoW/octocovs/main/badges/k1LoW/mo/time.svg)

`mo` is a **M**arkdown viewer that **o**pens `.md` files in a browser.

## Features

- GitHub-flavored Markdown (tables, task lists, footnotes, etc.)
- Syntax highlighting ([Shiki](https://shiki.style/))
- [Mermaid](https://mermaid.js.org/) diagram rendering
- <img src="images/icons/theme-light.svg" width="16" height="16" alt="dark theme"> Dark / <img src="images/icons/theme-dark.svg" width="16" height="16" alt="light theme"> light theme
- <img src="images/icons/group.svg" width="16" height="16" alt="group"> File grouping
- <img src="images/icons/toc.svg" width="16" height="16" alt="toc"> Table of contents panel
- <img src="images/icons/view-flat.svg" width="16" height="16" alt="flat view"> Flat / <img src="images/icons/view-tree.svg" width="16" height="16" alt="tree view"> tree sidebar view with drag-and-drop reorder and file search
- YAML frontmatter display (collapsible metadata block)
- MDX file support (renders as Markdown, strips `import`/`export`, escapes JSX tags)
- <img src="images/icons/raw.svg" width="16" height="16" alt="raw"> Raw markdown view
- <img src="images/icons/copy.svg" width="16" height="16" alt="copy"> Copy content (Markdown / Text / HTML)
- <img src="images/icons/restart.svg" width="16" height="16" alt="restart"> Server restart with session preservation
- Auto session backup and restore
- Live-reload on save

## Install

**homebrew tap:**

```console
$ brew install k1LoW/tap/mo
```

**manually:**

Download binary from [releases page](https://github.com/k1LoW/mo/releases)

## Usage

``` console
$ mo README.md                          # Open a single file
$ mo README.md CHANGELOG.md docs/*.md   # Open multiple files
$ mo spec.md --target design            # Open in a named group
```

`mo` opens Markdown files in a browser with live-reload. When you save a file, the browser automatically reflects the changes.

### Single server, multiple files

By default, `mo` runs a single server on port `6275`. If a server is already running on the same port, subsequent `mo` invocations add files to the existing session instead of starting a new one.

``` console
$ mo README.md          # Starts a mo server in the background
$ mo CHANGELOG.md       # Adds the file to the running mo server
```

To run a completely separate session, use a different port:

``` console
$ mo draft.md -p 6276
```

![Multiple files with sidebar](images/multiple-files.png)

### Groups

Files can be organized into named groups using the `--target` (`-t`) flag. Each group gets its own URL path and sidebar.

``` console
$ mo spec.md --target design      # Opens at http://localhost:6275/design
$ mo api.md --target design       # Adds to the "design" group
$ mo notes.md --target notes      # Opens at http://localhost:6275/notes
```

![Group view](images/groups.png)

### Glob pattern watching

Use `--watch` (`-w`) to specify glob patterns. Matching files are opened automatically, and watched directories are monitored for new files.

``` console
$ mo --watch '**/*.md'                          # Watch and open all .md files recursively
$ mo --watch 'docs/**/*.md' --target docs       # Watch docs/ tree in "docs" group
$ mo --watch '*.md' --watch 'docs/**/*.md'      # Multiple patterns
```

`--watch` cannot be combined with file arguments. The `**` pattern matches directories recursively.

#### Removing watch patterns

Use `--unwatch` to stop watching a previously registered pattern. Files already added remain in the sidebar.

``` console
$ mo --unwatch '**/*.md'                              # Stop watching a pattern (default group)
$ mo --unwatch 'docs/**/*.md' --target docs            # Stop watching in a specific group
$ mo --unwatch '/Users/you/project/**/*.md'            # Stop watching by absolute path
```

Patterns are resolved to absolute paths before matching, so you can specify either a relative glob or the full path shown by `--status`.

### Sidebar view modes

The sidebar supports flat and tree view modes. Flat view shows file names only, while tree view displays the directory hierarchy.

| Flat | Tree |
|------|------|
| ![Flat view](images/sidebar-flat.png) | ![Tree view](images/sidebar-tree.png) |

### Starting and stopping

`mo` runs in the background by default — the command returns immediately, leaving the shell free for other work. This makes it easy to incorporate into scripts, tool chains, or LLM-driven workflows.

``` console
$ mo README.md
mo: serving at http://localhost:6275 (pid 12345)
$ # shell is available immediately
```

Use `--status` to check all running mo servers, and `--shutdown` to stop one:

``` console
$ mo --status              # Show all running mo servers
http://localhost:6275 (pid 12345, v0.12.0)
  default: 5 file(s)
    watching: /Users/you/project/src/**/*.md, /Users/you/project/*.md
  docs: 2 file(s)
    watching: /Users/you/project/docs/**/*.md

$ mo --shutdown            # Shut down the mo server on the default port
$ mo --shutdown -p 6276    # Shut down the mo server on a specific port
$ mo --restart             # Restart the mo server on the default port
```

If you need the mo server to run in the foreground (e.g. for debugging), use `--foreground`:

``` console
$ mo --foreground README.md
```

### Server restart

Click the <img src="images/icons/restart.svg" width="16" height="16" alt="restart"> restart button (bottom-right corner) or run `mo --restart` to restart the `mo` server process. The current session — all open files and groups — is preserved across the restart. This is useful when you have updated the `mo` binary and want to pick up the new version without re-opening your files.

### Session backup and restore

`mo` automatically saves session state (open files and watch patterns per group) when files are added or removed. If a `mo` server is stopped and you run `mo` with no arguments on the same port, the previous session is automatically restored.

``` console
$ mo README.md CHANGELOG.md       # Start with two files
$ mo --shutdown                   # Shut down the server
$ mo                              # Restores README.md and CHANGELOG.md
```

Use `--clear` to remove a saved session:

``` console
$ mo --clear                      # Clear saved session for the default port
$ mo --clear -p 6276              # Clear saved session for a specific port
```

### Flags

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--target` | `-t` | `default` | Group name |
| `--port` | `-p` | `6275` | Server port |
| `--open` | | | Always open browser |
| `--no-open` | | | Never open browser |
| `--status` | | | Show all running mo servers |
| `--watch` | `-w` | | Glob pattern to watch for matching files (repeatable) |
| `--unwatch` | | | Remove a watched glob pattern (repeatable) |
| `--shutdown` | | | Shut down the running mo server |
| `--restart` | | | Restart the running mo server |
| `--clear` | | | Clear saved session for the specified port |
| `--foreground` | | | Run mo server in foreground |

## Build

Requires Go and [pnpm](https://pnpm.io/).

``` console
$ make build
```

## References

- [yusukebe/gh-markdown-preview](https://github.com/yusukebe/gh-markdown-preview): GitHub CLI extension to preview Markdown looks like GitHub.
