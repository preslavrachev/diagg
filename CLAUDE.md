This is a Go-based CLI tool that generates C4 component diagrams from Go codebases.

## Overview

By pointing this CLI at a Go project, it recursively parses Go source files, extracts structs and their relationships, infers component types based on naming conventions, and generates PlantUML C4 diagrams - all with zero configuration.

## Architecture

The tool follows a three-stage pipeline:

1. **Parser** (`parser/`) - Uses `golang.org/x/tools/go/packages` to load Go packages with full type information, extracting structs, interfaces, and their relationships
2. **Analyzer** (`analyzer/`) - Classifies components based on naming patterns (e.g., `*Service`, `*Repository`, `*Handler`), identifies dependencies from struct fields and method signatures (parameters and return types), and detects interface implementations
3. **Generator** (`generator/`) - Renders PlantUML C4 component diagrams with proper relationships (solid lines for dependencies, dotted lines for interface implementations)

## Package Structure

- `cmd/diagg/` - CLI entry point using urfave/cli
- `parser/` - Go package loader with type information support
- `analyzer/` - Component type inference, dependency extraction, and interface implementation detection
- `generator/` - PlantUML C4 diagram generator

No `internal/` or `pkg/` directories - packages are reasonably named and organized at the root level.

## Analysis Modes

The analyzer supports two modes:

1. **Basic mode** (`Analyze()`) - Fast AST-only analysis, good for simple dependency graphs
2. **Type-aware mode** (`AnalyzeWithTypes()`) - Full type information, detects interface implementations and cross-package relationships

Most users should use type-aware mode for accurate diagrams. See [docs/INTERFACE_DETECTION.md](docs/INTERFACE_DETECTION.md) for implementation details.

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
- Full type-aware analysis using `go/packages` for accurate interface detection
- Simple, opinionated defaults that produce useful diagrams immediately

## Dependency Detection

The analyzer detects dependencies from multiple sources:
- **Struct fields** - Direct composition/aggregation relationships
- **Method parameters** - Dependencies passed into behavior
- **Method return types** - Dependencies produced by behavior

This ensures that components like `PlantUMLGenerator` correctly show dependencies on `AnalyzedComponent` (via method parameters) even when they don't store them as fields.

## Development Notes

- **AIDEV anchors** - Used throughout for AI-assisted development. Grep for `AIDEV-NOTE:`, `AIDEV-TODO:`, or `AIDEV-QUESTION:`
  - Key anchors: `method-deps`, `method-extraction`, `test-cross-pkg-deps`
- **Testing strategy** - TDD approach with comprehensive test coverage:
  - Cross-package dependency detection ([analyzer_test.go:14](analyzer/analyzer_test.go#L14))
  - Interface implementation detection ([analyzer_test.go:70](analyzer/analyzer_test.go#L70))
  - Method parameter dependency detection ([analyzer_test.go:154](analyzer/analyzer_test.go#L154))
- **Tests use table-driven patterns** with well-named constants
- **Error handling** uses `fmt.Errorf("context: %w", err)` for proper error chains
- **Example output**: [diagram.puml](diagram.puml) shows the tool's own component structure
