# Code Blocks

## Go

```go
package main

import (
	"fmt"
	"net/http"
)

func handler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "Hello, %s!", r.URL.Path[1:])
}

func main() {
	http.HandleFunc("/", handler)
	http.ListenAndServe(":8080", nil)
}
```

## TypeScript

```typescript
interface FileEntry {
  name: string;
  id: number;
}

async function fetchFiles(): Promise<FileEntry[]> {
  const res = await fetch("/_/api/groups");
  const data = await res.json();
  return data.flatMap((g: { files: FileEntry[] }) => g.files);
}
```

## CSS

```css
.markdown-body {
  font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;
  line-height: 1.6;
  color: #24292e;
}

[data-theme="dark"] .markdown-body {
  color: #e6edf3;
  background-color: #0d1117;
}
```

## Shell

```bash
#!/bin/bash
set -euo pipefail

echo "Building mo..."
cd internal/frontend && pnpm run build
cd ../..
go build -o mo .
echo "Done!"
```

## JSON

```json
{
  "name": "mo",
  "dependencies": {
    "react": "^19.1.0",
    "react-markdown": "^10.1.0",
    "shiki": "^3.6.0"
  }
}
```

## SQL

```sql
CREATE TABLE files (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    path TEXT NOT NULL UNIQUE,
    group_name TEXT NOT NULL DEFAULT 'default',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

SELECT f.name, g.name AS group_name
FROM files f
JOIN groups g ON f.group_name = g.name
ORDER BY f.id;
```

## Plain Text (no language)

```
This is a plain code block without a language specified.
It should still be rendered in a monospace font.
```

## Inline Code

Use `go build -o mo .` to build the binary. The config is in `internal/server/server.go`.
