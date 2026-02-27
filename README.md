# mo

`mo` is a Markdown viewer that opens `.md` files in a browser.

## Usage

``` console
$ mo README.md                          # Open a single file
$ mo README.md CHANGELOG.md docs/*.md   # Open multiple files
$ mo spec.md --target design            # Open in a named group
$ mo api.md --port 8080                 # Use a custom port
```

If a server is already running on the same port, files are added to the existing session instead of starting a new one.

### Flags

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--target` | `-t` | `default` | Group name |
| `--port` | `-p` | `6275` | Server port |

## Build

Requires Go and [pnpm](https://pnpm.io/).

``` console
$ make build
```
