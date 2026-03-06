# Changelog

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
