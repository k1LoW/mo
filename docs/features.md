# Hosted mode features

These features extend `mo` beyond local-only usage, making it a bit more suitable for hosted or shared environments where multiple users may access the same instance over a network.

Theres a lot of other work to do like authentication and group ACLs.

At present, an assumption is users run it in a trusted environment (ie. own secured infrastructure).

## UI controls

- **`--read-only`** — Shorthand that enables `--no-restart`, `--no-delete`, and `--no-file-move`. Prevents viewers from modifying files read by `mo`
- **`--no-restart`** — Disable server restart via UI.
- **`--no-delete`** — Disable file removal via UI.
- **`--no-file-move`** — Disable moving files between groups via UI.

## Sharing

- **`--shareable`** — Expose document links in the address bar (`?file=<id>` or `?filename=<name>`). Enables the share button and raw share button in the toolbar.
- **`--true-filenames`** — Use human-readable filenames in URLs (`?filename=foo.md`) instead of hash IDs. Falls back to hash ID when filenames are not unique within a group.
- **Raw file sharing** — A toolbar button copies a direct link to the raw file content (`/_/api/files/{id}/raw`), served as `text/plain`. Gated by `--shareable`.

## UI features

- **`--newfile-no-autoselect`** — When new files appear (e.g. from `--watch` patterns), they appear in the sidebar without forcing the viewer away from the file they are reading.
- **File timestamps** — Three-mode toggle in the sidebar header cycles between off, relative time ("5m ago"), and absolute time ("Mar 15 02:36").
- **Tree collapse/expand** — Toggle button in the sidebar header to collapse or expand all directories in tree view.

## Configuration file

All flags can be set via a YAML config file (`--config`). See [`config.example.yaml`](config.example.yaml) for the full reference. Useful when running on a server (or configmap on k8s)
