This is a Go-based CLI tool that generates C4 component diagrams from Go codebases.

## Overview

By pointing this CLI at a Go project, it recursively parses Go source files, extracts structs and their relationships, infers component types based on naming conventions, and generates PlantUML C4 diagrams - all with zero configuration.

See [README.md](README.md) for user-facing usage and installation instructions.

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
  - `view.go` - `ViewMode` abstraction (`ViewModeComponent` / `ViewModePackage`) and `viewGraph` data model shared by all generators
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

## Dependency Detection

The analyzer detects dependencies from multiple sources:
- **Struct fields** - Direct composition/aggregation relationships
- **Method parameters** - Dependencies passed into behavior
- **Method return types** - Dependencies produced by behavior
- **Function body type usage** - Local variables, constructor calls, type assertions
  - Detects dependencies from patterns like `parser := markdown.NewStreamingParser()`
  - Analyzes return types of function calls rather than just searching for "NewXXX" patterns
  - Filters out standard library types to reduce noise
  - This walk only covers **methods** (functions with a receiver matching the component). A
    type referenced only from a free (package-level) function - e.g. a plain constructor
    caller - produces no `Dependency` edge; see `AnalyzedComponent.FreeFunctionReferences` below.

## Connectivity Metrics

The analyzer includes a metrics system ([analyzer/metrics.go](analyzer/metrics.go)) that calculates graph-theoretic properties for layout optimization:

- **Role classification** - `hub` (high in-degree), `leaf` (high out-degree, low in-degree), `central` (high total), `ordinary`

These metrics drive visual hierarchy in the generated diagrams. See [analyzer/metrics.go](analyzer/metrics.go) for thresholds.

### Free-Function Usage (`AnalyzedComponent.FreeFunctionReferences`)

`main` packages get full free-function coverage via a separate mechanism
(`parser.extractMainComponent`, which walks the whole package and produces real
`Dependency` edges). Every other package's free functions - constructors, helpers,
anything without a receiver - are walked by `Analyzer.freeFunctionReferenceCounts`
([analyzer/analyzer.go](analyzer/analyzer.go)), but deliberately **not** folded into
`Metrics.InDegree`/`Role`: there is no component to draw a `Dependency` edge from, so
merging the count into the graph metrics would change a node's role/visual weight with
no edge in the diagram to explain why. It is exposed as a separate
`FreeFunctionReferences int` field instead. Multi-return calls (`func NewRepo() (*Repo,
error)`, the idiomatic Go constructor shape) are unpacked via a `*types.Tuple` case in
`addDepFromType`, so the value half of the pair is still detected.

A free function that both lives in a type's own package AND declares that same type as
one of its own return values is that type's constructor (the `func NewX() *X { return
&X{} }` shape) - referencing the type inside its own body is excluded as definitional,
not usage evidence. This is narrower than a blanket same-package skip: a *different*
free function in the same package that wires two components together (e.g.
`func BuildService() *Service { repo := NewRepo(); return &Service{repo: repo} }`) still
counts as usage of `Repo`, since `BuildService`'s own return type is `*Service`, not
`*Repo`.

`FreeFunctionReferences` is **not currently surfaced** in any generator (PlantUML/D3/
Excalidraw/JSON) - it exists purely on `AnalyzedComponent` today, for a future consumer
(e.g. an unused-component check: `Metrics.InDegree + FreeFunctionReferences == 0`).

## Output Formats

The tool supports multiple output formats via the generator interface, with two view modes:

- **Component view** (default) - Individual structs/types as nodes
- **Package view** (`-P` flag) - Collapses to package-level import graph

Formats:
1. **PlantUML C4** (default) - `diagram.puml` - static C4 component diagram
2. **D3.js** - `diagram-d3.html` - interactive force-directed graph with package clustering
3. **Excalidraw** - `diagram.excalidraw` - editable Excalidraw scene
4. **JSON** (`--format json`) - machine-readable node/edge graph (id, name, package, type, role, degree metrics), supporting both view modes exactly like the other generators; see [generator/json.go](generator/json.go). Unlike the other formats, JSON defaults to stdout (not a file) when `--output` is unset, so it composes with `jq`/CI pipelines - all progress/debug logging is routed to stderr in that case (see the `json-stdout` AIDEV-NOTE in [cmd/diagg/main.go](cmd/diagg/main.go)).

See [generator/plantuml.go](generator/plantuml.go) and [generator/d3.go](generator/d3.go) for implementation details.

## Structural Checks

`diagg check` runs CI-friendly checks against the package graph, both built on [analyzer/package_graph.go](analyzer/package_graph.go):

- `diagg check root-budget --max N --ignore ...` - fails if root-level product/domain packages (direct children of the module root) exceed the budget, after excluding infrastructure/support packages
- `diagg check generic-names --deny ...` - flags packages whose declared name or import-path directory segment matches a generic-name deny list (e.g. `model`, `utils`, `manager`)

Both exit non-zero on violation unless `--warn-only` is passed. See [cmd/diagg/check.go](cmd/diagg/check.go).

## Development Notes

- **AIDEV anchors** - Used throughout for AI-assisted development. Grep for `AIDEV-NOTE:` in `*.go` files and `generator/templates/*.html` (D3.js anchors are in the template):
  - Core analysis: `function-body-analysis`, `method-deps`, `stdlib-filter`, `type-info-lookup`, `constructor-detection`, `qualified-name`
  - Metrics & layout: `graph-metrics`, `visual-hierarchy`, `directional-hints`, `layout-optimization`
  - Multi-entrypoint support: `main-package-detection`, `multiple-mains`, `main-entrypoint`
  - Visualization (in `templates/d3.html`): `package-clustering`, `package-hulls`
- **Testing strategy** - TDD approach with comprehensive test coverage:
  - Cross-package dependency detection ([analyzer_test.go:14](analyzer/analyzer_test.go#L14))
  - Interface implementation detection ([analyzer_test.go:70](analyzer/analyzer_test.go#L70))
  - Method parameter dependency detection ([analyzer_test.go:154](analyzer/analyzer_test.go#L154))
  - Function body dependency extraction - BDD-style tests ([function_body_deps_test.go](analyzer/function_body_deps_test.go))
  - Multiple main packages ([parser_test.go:15](parser/parser_test.go#L15))
- **Error handling** uses `fmt.Errorf("context: %w", err)` for proper error chains
- **Example output**: [diagram.puml](diagram.puml) shows the tool's own component structure
