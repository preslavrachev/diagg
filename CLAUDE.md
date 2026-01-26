This is a Go-based CLI tool that generates C4 component diagrams from Go codebases.

## Overview

By pointing this CLI at a Go project, it recursively parses Go source files, extracts structs and their relationships, infers component types based on naming conventions, and generates PlantUML C4 diagrams - all with zero configuration.

## Architecture

The tool follows a three-stage pipeline:

1. **Parser** (`parser/`) - Uses `golang.org/x/tools/go/packages` to load Go packages with full type information, extracting structs, interfaces, and their relationships
2. **Analyzer** (`analyzer/`) - Classifies components based on naming patterns (e.g., `*Service`, `*Repository`, `*Handler`), identifies dependencies from struct fields, method signatures (parameters and return types), and function body type usage, and detects interface implementations
3. **Generator** (`generator/`) - Renders diagrams in multiple formats:
   - PlantUML C4 component diagrams with proper relationships (solid lines for dependencies, dotted lines for interface implementations)
   - D3.js force-directed interactive graphs with package clustering

## Package Structure

- `cmd/diagg/` - CLI entry point using urfave/cli
- `parser/` - Go package loader with type information support
- `analyzer/` - Component type inference, dependency extraction (including function body analysis), and interface implementation detection
- `generator/` - Diagram generators (PlantUML C4, D3.js force-directed)
- `config/` - Configuration system for component patterns, styling, and technology inference

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
- **Function body type usage** - Local variables, constructor calls, type assertions (added 2026-01)
  - Detects dependencies from patterns like `parser := markdown.NewStreamingParser()`
  - Analyzes return types of function calls rather than just searching for "NewXXX" patterns
  - Filters out standard library types to reduce noise

This comprehensive detection ensures accurate dependency graphs even when types are instantiated locally within functions rather than stored as fields.

## Connectivity Metrics

The analyzer includes a metrics system ([analyzer/metrics.go](analyzer/metrics.go)) that calculates graph-theoretic properties for layout optimization:

- **In-degree** - How many components depend on this component
- **Out-degree** - How many components this component depends on
- **Role classification** - Components are categorized by connectivity patterns:
  - `hub` - High in-degree (many depend on it) - typically core abstractions/interfaces
  - `leaf` - High out-degree, low in-degree (depends on many, few depend on it) - typically application entry points
  - `central` - High total connectivity - coordination/orchestration components
  - `ordinary` - Standard connectivity levels

These metrics drive visual hierarchy in the generated diagrams - hubs are positioned centrally, leaves at the periphery.

## Output Formats

The tool supports multiple output formats via the generator interface:

1. **PlantUML C4** (default) - `diagram.puml`
   - C4 component diagram with package boundaries
   - Visual hierarchy based on connectivity metrics
   - Solid lines for dependencies, dotted for interface implementations

2. **D3.js** - `diagram-d3.html`
   - Interactive force-directed graph
   - Package clustering with translucent hulls
   - Draggable nodes, zoom/pan support

See [generator/plantuml.go](generator/plantuml.go) and [generator/d3.go](generator/d3.go) for implementation details.

## Development Notes

- **AIDEV anchors** - Used throughout for AI-assisted development. Grep for `AIDEV-NOTE:`, `AIDEV-TODO:`, or `AIDEV-QUESTION:`
  - Key anchors: `graph-metrics`, `method-deps`, `function-body-analysis`, `stdlib-filter`, `visual-hierarchy`
- **Testing strategy** - TDD approach with comprehensive test coverage:
  - Cross-package dependency detection ([analyzer_test.go:14](analyzer/analyzer_test.go#L14))
  - Interface implementation detection ([analyzer_test.go:70](analyzer/analyzer_test.go#L70))
  - Method parameter dependency detection ([analyzer_test.go:154](analyzer/analyzer_test.go#L154))
  - Function body dependency extraction - BDD-style tests ([function_body_deps_test.go](analyzer/function_body_deps_test.go))
- **Tests use table-driven patterns** with well-named constants for unit tests, BDD-style Given-When-Then for behavioral tests
- **Error handling** uses `fmt.Errorf("context: %w", err)` for proper error chains
- **Example output**: [diagram.puml](diagram.puml) shows the tool's own component structure
