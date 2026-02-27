# Mermaid + Code Mixed

This file contains both mermaid diagrams and regular code blocks.

## Architecture

```mermaid
graph LR
    Browser -->|HTTP| GoServer
    GoServer -->|Read| FileSystem
    GoServer -->|Embed| StaticFiles
```

## Go Code

```go
func main() {
    fmt.Println("Hello, mo!")
}
```

## Class Diagram

```mermaid
classDiagram
    class Server {
        +State state
        +HandleFunc(pattern, handler)
        +ListenAndServe() error
    }
    class State {
        -sync.RWMutex mu
        -map groups
        +AddFile(path, group) FileEntry
        +GetGroups() []Group
    }
    Server --> State
```

## JSON Example

```json
{
  "name": "mo",
  "version": "0.1.0"
}
```

## Pie Chart

```mermaid
pie title Language Distribution
    "Go" : 40
    "TypeScript" : 35
    "CSS" : 15
    "Other" : 10
```
