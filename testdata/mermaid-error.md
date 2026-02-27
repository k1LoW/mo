# Error Handling Test

This file contains an invalid mermaid block to verify fallback behavior.

## Valid Diagram

```mermaid
flowchart LR
    A --> B --> C
```

## Invalid Diagram

```mermaid
this is not valid mermaid syntax !!!
    --> broken {{{
```

## Normal Text After

Regular markdown continues to render correctly after an invalid mermaid block.
