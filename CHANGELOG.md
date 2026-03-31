# Changelog

## [v0.23.2](https://github.com/k1LoW/mo/compare/v0.23.1...v0.23.2) - 2026-03-31
### Fix bug 🐛
- feat: add smooth scroll for footnote references by @k1LoW in https://github.com/k1LoW/mo/pull/156

## [v0.23.1](https://github.com/k1LoW/mo/compare/v0.23.0...v0.23.1) - 2026-03-28
### Other Changes
- fix: open sidebar when search is activated by @k1LoW in https://github.com/k1LoW/mo/pull/152

## [v0.23.0](https://github.com/k1LoW/mo/compare/v0.22.1...v0.23.0) - 2026-03-28
### New Features 🎉
- feat: add full-text search results by @k1LoW in https://github.com/k1LoW/mo/pull/149
- chore: change page title format by @syumai in https://github.com/k1LoW/mo/pull/151

## [v0.22.1](https://github.com/k1LoW/mo/compare/v0.22.0...v0.22.1) - 2026-03-27
### Fix bug 🐛
- fix: Enable overscroll containment for Sidebar and TocPanel components by @orangekame3 in https://github.com/k1LoW/mo/pull/147
### Dependency Updates ⬆️
- chore(deps): bump the dependencies group in /internal/frontend with 8 updates by @dependabot[bot] in https://github.com/k1LoW/mo/pull/145

## [v0.22.0](https://github.com/k1LoW/mo/compare/v0.21.0...v0.22.0) - 2026-03-26
### New Features 🎉
- feat: add "Copy link" to file context menu by @k1LoW in https://github.com/k1LoW/mo/pull/140
- feat: persist TOC open/closed state per file in localStorage by @k1LoW in https://github.com/k1LoW/mo/pull/143

## [v0.21.0](https://github.com/k1LoW/mo/compare/v0.20.2...v0.21.0) - 2026-03-24
### Breaking Changes 🛠
- feat: support directory arguments to open .md files inside by @k1LoW in https://github.com/k1LoW/mo/pull/137
### New Features 🎉
- feat: add fullscreen zoom modal for images and Mermaid diagrams by @harakeishi in https://github.com/k1LoW/mo/pull/135

## [v0.20.2](https://github.com/k1LoW/mo/compare/v0.20.1...v0.20.2) - 2026-03-22
### Other Changes
- fix: prevent scroll chaining on content area by @orangekame3 in https://github.com/k1LoW/mo/pull/133

## [v0.20.1](https://github.com/k1LoW/mo/compare/v0.20.0...v0.20.1) - 2026-03-20
### Fix bug 🐛
- fix: clear running server state when using --clear by @k1LoW in https://github.com/k1LoW/mo/pull/131

## [v0.20.0](https://github.com/k1LoW/mo/compare/v0.19.0...v0.20.0) - 2026-03-20
### New Features 🎉
- feat: add --close option to remove files from CLI by @haru0017 in https://github.com/k1LoW/mo/pull/126
### Dependency Updates ⬆️
- chore(deps): bump the dependencies group with 2 updates by @dependabot[bot] in https://github.com/k1LoW/mo/pull/127
- chore(deps): bump the dependencies group in /internal/frontend with 8 updates by @dependabot[bot] in https://github.com/k1LoW/mo/pull/128

## [v0.19.0](https://github.com/k1LoW/mo/compare/v0.18.5...v0.19.0) - 2026-03-18
### New Features 🎉
- feat: add title toggle button for heading titles by @k1LoW in https://github.com/k1LoW/mo/pull/121
- feat: add Content-Security-Policy header by @k1LoW in https://github.com/k1LoW/mo/pull/125
- feat: add "Copy absolute path" to file context menu by @harakeishi in https://github.com/k1LoW/mo/pull/123

## [v0.18.5](https://github.com/k1LoW/mo/compare/v0.18.4...v0.18.5) - 2026-03-18
### Other Changes
- docs: add SECURITY.md with vulnerability reporting policy by @k1LoW in https://github.com/k1LoW/mo/pull/119

## [v0.18.4](https://github.com/k1LoW/mo/compare/v0.18.3...v0.18.4) - 2026-03-15
### Other Changes
- refactor: apply modernize linter suggestions by @k1LoW in https://github.com/k1LoW/mo/pull/113
- fix: update undici to 7.24.3 to resolve security vulnerabilities by @k1LoW in https://github.com/k1LoW/mo/pull/115

## [v0.18.3](https://github.com/k1LoW/mo/compare/v0.18.2...v0.18.3) - 2026-03-13
### Dependency Updates ⬆️
- chore(deps): bump the dependencies group with 2 updates by @dependabot[bot] in https://github.com/k1LoW/mo/pull/111
- chore(deps): bump the dependencies group in /internal/frontend with 10 updates by @dependabot[bot] in https://github.com/k1LoW/mo/pull/112
### Other Changes
- fix: debounce watcher file change events by @k1LoW in https://github.com/k1LoW/mo/pull/109

## [v0.18.2](https://github.com/k1LoW/mo/compare/v0.18.1...v0.18.2) - 2026-03-11
### Other Changes
- feat: reject binary files in AddFile by @k1LoW in https://github.com/k1LoW/mo/pull/107
- feat: syntax highlight non-markdown text files with Shiki by @k1LoW in https://github.com/k1LoW/mo/pull/108

## [v0.18.1](https://github.com/k1LoW/mo/compare/v0.18.0...v0.18.1) - 2026-03-10
### Other Changes
- feat: add --skip-bind-address-confirmation flag to bypass non-loopback bind prompt by @110y in https://github.com/k1LoW/mo/pull/103
- fix: rename --skip-bind-address-confirmation to --dangerously-allow-remote-access by @k1LoW in https://github.com/k1LoW/mo/pull/105

## [v0.18.0](https://github.com/k1LoW/mo/compare/v0.17.0...v0.18.0) - 2026-03-10
### New Features 🎉
- feat: add wide/narrow layout toggle by @k1LoW in https://github.com/k1LoW/mo/pull/101
### Other Changes
- fix: add accessibility attributes to all toggle buttons by @k1LoW in https://github.com/k1LoW/mo/pull/102

## [v0.17.0](https://github.com/k1LoW/mo/compare/v0.16.3...v0.17.0) - 2026-03-10
### New Features 🎉
- Proposal: add --bind flag to specify listen address by @110y in https://github.com/k1LoW/mo/pull/96
- feat: add security warning with confirmation prompt for non-localhost --bind by @k1LoW in https://github.com/k1LoW/mo/pull/98

## [v0.16.3](https://github.com/k1LoW/mo/compare/v0.16.2...v0.16.3) - 2026-03-10
### Fix bug 🐛
- fix: render Mermaid charts using actual container width by @k1LoW in https://github.com/k1LoW/mo/pull/95
### Other Changes
- refactor: replace eslint with oxlint/oxfmt by @k1LoW in https://github.com/k1LoW/mo/pull/93

## [v0.16.2](https://github.com/k1LoW/mo/compare/v0.16.1...v0.16.2) - 2026-03-09
### Fix bug 🐛
- Remove stale files when a watched directory is moved by @110y in https://github.com/k1LoW/mo/pull/91

## [v0.16.1](https://github.com/k1LoW/mo/compare/v0.16.0...v0.16.1) - 2026-03-09
### New Features 🎉
- feat: preserve scroll position across live-reload and server restart by @k1LoW in https://github.com/k1LoW/mo/pull/90
### Other Changes
- feat: add ESLint to frontend and fix all lint errors by @k1LoW in https://github.com/k1LoW/mo/pull/88

## [v0.16.0](https://github.com/k1LoW/mo/compare/v0.15.2...v0.16.0) - 2026-03-08
### Breaking Changes 🛠
- feat: display deeplinks when adding files via CLI by @k1LoW in https://github.com/k1LoW/mo/pull/86
- feat: route processable output to stdout and add --json flag by @k1LoW in https://github.com/k1LoW/mo/pull/87
### New Features 🎉
- feat: add LaTeX/math rendering support with KaTeX by @ysaito8015 in https://github.com/k1LoW/mo/pull/84
- feat: add drag-and-drop file addition from OS file manager by @k1LoW in https://github.com/k1LoW/mo/pull/85
### Other Changes
- feat: use deterministic hash-based file IDs for deep linking by @k1LoW in https://github.com/k1LoW/mo/pull/81

## [v0.15.2](https://github.com/k1LoW/mo/compare/v0.15.1...v0.15.2) - 2026-03-07
### Other Changes
- fix: always restore backup when starting a new server by @k1LoW in https://github.com/k1LoW/mo/pull/80

## [v0.15.1](https://github.com/k1LoW/mo/compare/v0.15.0...v0.15.1) - 2026-03-06

## [v0.15.0](https://github.com/k1LoW/mo/compare/v0.14.1...v0.15.0) - 2026-03-06
### New Features 🎉
- feat: add auto-backup and restore for sessions by @k1LoW in https://github.com/k1LoW/mo/pull/76

## [v0.14.1](https://github.com/k1LoW/mo/compare/v0.14.0...v0.14.1) - 2026-03-06
### Fix bug 🐛
- fix: fix group dropdown not showing when no default group exists by @k1LoW in https://github.com/k1LoW/mo/pull/75

## [v0.14.0](https://github.com/k1LoW/mo/compare/v0.13.1...v0.14.0) - 2026-03-06
### New Features 🎉
- feat: add --restart flag by @k1LoW in https://github.com/k1LoW/mo/pull/71
- Add file search filtering to sidebar by @harakeishi in https://github.com/k1LoW/mo/pull/72
- feat: reload all browser tabs on server restart by @k1LoW in https://github.com/k1LoW/mo/pull/70
### Fix bug 🐛
- fix: render code blocks without language using Shiki and copy button by @babarot in https://github.com/k1LoW/mo/pull/73

## [v0.13.1](https://github.com/k1LoW/mo/compare/v0.13.0...v0.13.1) - 2026-03-06
### New Features 🎉
- feat: add `--unwatch` flag to remove watched glob patterns by @k1LoW in https://github.com/k1LoW/mo/pull/65
### Dependency Updates ⬆️
- chore(deps): bump aquasecurity/trivy-action from 0.34.1 to 0.34.2 in the dependencies group by @dependabot[bot] in https://github.com/k1LoW/mo/pull/66
- chore(deps): bump the dependencies group in /internal/frontend with 4 updates by @dependabot[bot] in https://github.com/k1LoW/mo/pull/67

## [v0.13.0](https://github.com/k1LoW/mo/compare/v0.12.0...v0.13.0) - 2026-03-05
### New Features 🎉
- feat: add `--watch` (`-w`) flag for glob pattern directory watching by @k1LoW in https://github.com/k1LoW/mo/pull/64

## [v0.12.0](https://github.com/k1LoW/mo/compare/v0.11.4...v0.12.0) - 2026-03-04
### New Features 🎉
- feat: support YAML frontmatter in Markdown files by @k1LoW in https://github.com/k1LoW/mo/pull/60
- feat: support MDX files by @k1LoW in https://github.com/k1LoW/mo/pull/62

## [v0.11.4](https://github.com/k1LoW/mo/compare/v0.11.3...v0.11.4) - 2026-03-04
### Other Changes
- Include frontend dependency licenses in CREDITS by @k1LoW in https://github.com/k1LoW/mo/pull/59

## [v0.11.3](https://github.com/k1LoW/mo/compare/v0.11.2...v0.11.3) - 2026-03-03
### Other Changes
- fix: avoid appending /default to URL when adding files to existing server by @k1LoW in https://github.com/k1LoW/mo/pull/56

## [v0.11.2](https://github.com/k1LoW/mo/compare/v0.11.1...v0.11.2) - 2026-03-03
### New Features 🎉
- fix: handle atomic saves and improve live-reload reliability by @k1LoW in https://github.com/k1LoW/mo/pull/54

## [v0.11.1](https://github.com/k1LoW/mo/compare/v0.11.0...v0.11.1) - 2026-03-03
### New Features 🎉
- Allow slashes in --target group names and validate invalid characters by @k1LoW in https://github.com/k1LoW/mo/pull/53

## [v0.11.0](https://github.com/k1LoW/mo/compare/v0.10.1...v0.11.0) - 2026-03-02
### Breaking Changes 🛠
- feat: write logs to rotating files under XDG_STATE_HOME by @k1LoW in https://github.com/k1LoW/mo/pull/46
- feat: run mo in background by default by @k1LoW in https://github.com/k1LoW/mo/pull/50
### New Features 🎉
- feat: add --close flag to gracefully shut down a running mo server by @k1LoW in https://github.com/k1LoW/mo/pull/47
- feat: add --status flag to show all running mo servers by @k1LoW in https://github.com/k1LoW/mo/pull/51
### Other Changes
- Rename --close flag to --shutdown by @k1LoW in https://github.com/k1LoW/mo/pull/49

## [v0.10.1](https://github.com/k1LoW/mo/compare/v0.10.0...v0.10.1) - 2026-03-02
### Other Changes
- feat: update tree view toggle icon to file-tree style by @k1LoW in https://github.com/k1LoW/mo/pull/45

## [v0.10.0](https://github.com/k1LoW/mo/compare/v0.9.0...v0.10.0) - 2026-03-02
### New Features 🎉
- feat: add drag-and-drop file reordering in sidebar by @k1LoW in https://github.com/k1LoW/mo/pull/41
- feat: add move file to another group via kebab menu by @k1LoW in https://github.com/k1LoW/mo/pull/42
- feat: add flat/tree view toggle for sidebar by @k1LoW in https://github.com/k1LoW/mo/pull/43

## [v0.9.0](https://github.com/k1LoW/mo/compare/v0.8.0...v0.9.0) - 2026-03-02
### New Features 🎉
- feat: add file remove feature by @k1LoW in https://github.com/k1LoW/mo/pull/36
- feat: add Open in new tab to sidebar kebab menu by @k1LoW in https://github.com/k1LoW/mo/pull/38
- feat: add restart server from Web UI by @k1LoW in https://github.com/k1LoW/mo/pull/39

## [v0.8.0](https://github.com/k1LoW/mo/compare/v0.7.0...v0.8.0) - 2026-03-01
### New Features 🎉
- feat: add copy buttons to Mermaid blocks by @k1LoW in https://github.com/k1LoW/mo/pull/34

## [v0.7.0](https://github.com/k1LoW/mo/compare/v0.6.0...v0.7.0) - 2026-03-01
### New Features 🎉
- feat: add copy button to code blocks by @k1LoW in https://github.com/k1LoW/mo/pull/32
### Other Changes
- test: improve frontend testing with colocation and component tests by @k1LoW in https://github.com/k1LoW/mo/pull/33

## [v0.6.0](https://github.com/k1LoW/mo/compare/v0.5.2...v0.6.0) - 2026-03-01
### New Features 🎉
- feat: add copy-to-clipboard button with format selection by @k1LoW in https://github.com/k1LoW/mo/pull/29
### Other Changes
- fix: differentiate ToC indentation for each heading level by @k1LoW in https://github.com/k1LoW/mo/pull/30

## [v0.5.2](https://github.com/k1LoW/mo/compare/v0.5.1...v0.5.2) - 2026-03-01

## [v0.5.1](https://github.com/k1LoW/mo/compare/v0.5.0...v0.5.1) - 2026-03-01
### Fix bug 🐛
- fix: resolve render loop caused by unstable references in ToC integration by @k1LoW in https://github.com/k1LoW/mo/pull/26

## [v0.5.0](https://github.com/k1LoW/mo/compare/v0.4.1...v0.5.0) - 2026-02-28
### New Features 🎉
- feat: add raw markdown view toggle by @k1LoW in https://github.com/k1LoW/mo/pull/22
- feat: add table of contents right panel by @k1LoW in https://github.com/k1LoW/mo/pull/24

## [v0.4.1](https://github.com/k1LoW/mo/compare/v0.4.0...v0.4.1) - 2026-02-28
### Fix bug 🐛
- fix: reject directory paths passed as file arguments by @matsuyoshi30 in https://github.com/k1LoW/mo/pull/20

## [v0.4.0](https://github.com/k1LoW/mo/compare/v0.3.2...v0.4.0) - 2026-02-28
### New Features 🎉
- feat: add --open and --no-open flags to control browser opening by @k1LoW in https://github.com/k1LoW/mo/pull/19

## [v0.3.2](https://github.com/k1LoW/mo/compare/v0.3.1...v0.3.2) - 2026-02-28
### Fix bug 🐛
- fix: serialize mermaid rendering to fix multiple diagrams by @k1LoW in https://github.com/k1LoW/mo/pull/15

## [v0.3.1](https://github.com/k1LoW/mo/compare/v0.3.0...v0.3.1) - 2026-02-27
### Other Changes
- refactor: improve donegroup usage for graceful shutdown by @k1LoW in https://github.com/k1LoW/mo/pull/13
- refactor: replace log and fmt.Fprintf(os.Stderr) with slog by @k1LoW in https://github.com/k1LoW/mo/pull/14

## [v0.3.0](https://github.com/k1LoW/mo/compare/v0.2.0...v0.3.0) - 2026-02-27
### New Features 🎉
- feat: support GitHub Alerts (admonitions) by @k1LoW in https://github.com/k1LoW/mo/pull/11

## [v0.2.0](https://github.com/k1LoW/mo/compare/v0.1.1...v0.2.0) - 2026-02-27
### New Features 🎉
- feat: show file path tooltip on sidebar hover by @k1LoW in https://github.com/k1LoW/mo/pull/8

## [v0.1.1](https://github.com/k1LoW/mo/compare/v0.1.0...v0.1.1) - 2026-02-27

## [v0.1.0](https://github.com/k1LoW/mo/commits/v0.1.0) - 2026-02-27
### Dependency Updates ⬆️
- chore(deps): bump pnpm/action-setup from 4.1.0 to 4.2.0 in the dependencies group by @dependabot[bot] in https://github.com/k1LoW/mo/pull/4
- chore(deps): bump the dependencies group in /internal/frontend with 3 updates by @dependabot[bot] in https://github.com/k1LoW/mo/pull/6
