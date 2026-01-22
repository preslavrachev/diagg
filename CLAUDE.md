This is a Go-based CLI tool that generates C4 component diagrams from Go codebases.

## Overview

By pointing this CLI at a Go project, it recursively parses Go source files, extracts structs and their relationships, infers component types based on naming conventions, and generates PlantUML C4 diagrams - all with zero configuration.

## Architecture

The tool follows a three-stage pipeline:

1. **Parser** (`parser/`) - Uses `go/ast` to parse `.go` files and extract struct definitions, fields, and type information
2. **Analyzer** (`analyzer/`) - Classifies components based on naming patterns (e.g., `*Service`, `*Repository`, `*Handler`) and identifies dependencies from struct fields
3. **Generator** (`generator/`) - Renders PlantUML C4 component diagrams with proper relationships

## Package Structure

- `cmd/diagg/` - CLI entry point using urfave/cli
- `parser/` - AST-based Go code parser
- `analyzer/` - Component type inference and dependency extraction
- `generator/` - PlantUML C4 diagram generator

No `internal/` or `pkg/` directories - packages are reasonably named and organized at the root level.

## Component Detection Patterns

The analyzer uses regex patterns to classify components:
- `*Service` → SERVICE (business logic)
- `*Repository`, `*Repo`, `*Store` → REPOSITORY (data access)
- `*Handler` → HANDLER (HTTP handlers)
- `*Controller` → CONTROLLER (request controllers)
- `*Client` → CLIENT (external service clients)
- `*Cache` → CACHE (caching layer)
- `*Gateway` → GATEWAY (external gateways)
- `*Middleware` → MIDDLEWARE (HTTP middleware)

## Usage

```bash
diagg [directory]                    # Analyze current or specified directory
diagg -o output.puml -t "My App"    # Custom output file and title
```

## Design Philosophy

Unlike go-structurizr (which requires extensive configuration and manual wiring), diagg prioritizes:
- Zero configuration - just point and shoot
- Automatic inference from naming conventions
- Static analysis via AST parsing (no need to import/compile the target code)
- Simple, opinionated defaults that produce useful diagrams immediately
