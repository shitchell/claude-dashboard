# Documentation

This directory contains documentation for Claude Dashboard.

## Contents

### Architecture

- **[architecture.md](architecture.md)** - Comprehensive architecture documentation including component descriptions, data flow, design patterns, and implementation details.

### Diagrams

Auto-generated diagrams from the Go AST. These are regenerated when source code changes.

| Diagram | Description |
|---------|-------------|
| [Class Diagram](diagrams/generated/classes.svg) | All structs, interfaces, methods, and their relationships |
| [Call Graph](diagrams/generated/callgraph.svg) | Function call relationships across packages |

**Source files:**
- `diagrams/generated/classes.puml` - PlantUML source
- `diagrams/generated/callgraph.gv` - GraphViz DOT source

### Guides

_Future home for usage guides, tutorials, and how-tos._

## Regenerating Diagrams

Diagrams are auto-generated from the Go AST using [goplantuml](https://github.com/jfeliu007/goplantuml) and [go-callvis](https://github.com/ofabry/go-callvis).

To regenerate manually:

```bash
./scripts/generate-docs.sh
```

To check if diagrams are stale:

```bash
./scripts/generate-docs.sh --check
```

### Prerequisites

Install the required tools:

```bash
go install github.com/jfeliu007/goplantuml/cmd/goplantuml@latest
go install github.com/ofabry/go-callvis@latest
```

PlantUML requires Java. The script will automatically download the PlantUML jar if needed.

## Git Hook

A pre-commit hook automatically regenerates diagrams when Go source files change. See `.githooks/pre-commit` for details.
