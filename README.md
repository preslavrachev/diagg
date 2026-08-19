# diagg

Zero-config C4 component diagrams from your Go codebase. Point it at your project, get architectural diagrams instantly.

```bash
diagg ./myproject
```

Generates PlantUML C4 diagrams showing components, dependencies, and interface implementations - no configuration needed.

## Why diagg?

Unlike tools that require manual wiring or extensive configuration, diagg infers everything from your code structure. It uses naming conventions (`*Service`, `*Repository`, `*Handler`, etc.) and full type analysis to detect relationships and interface implementations automatically. Works with projects containing multiple `main` packages (multiple binaries).

**What you get:**
- Solid lines for dependencies (struct fields, method parameters/returns, function body type usage)
- Dotted lines showing interface implementations
- Components grouped by package boundaries
- Automatic classification (Service, Repository, Handler, etc.)
- Visual hierarchy based on connectivity - heavily-used components rendered more prominently
- Multiple output formats: PlantUML C4 diagrams, interactive D3.js force-directed graphs, Excalidraw scenes, or machine-readable JSON

## Install

```bash
go install github.com/preslavrachev/diagg/cmd/diagg@latest
```

## Usage

```bash
diagg [directory]                     # Analyze and generate diagram.puml (PlantUML)
diagg -o output.puml -t "My App"      # Custom output file and title
diagg --format d3 -o diagram.html     # Generate interactive D3.js force-directed graph
diagg --format excalidraw             # Generate diagram.excalidraw for Excalidraw
diagg -P                              # Package-only mode from imports (A imports B => A -> B)
diagg -P --format d3                  # Interactive package-only graph
diagg --format json                   # Component-level JSON, printed to stdout
diagg -P --format json                # Package-level JSON, printed to stdout
diagg --format json -o out.json       # Write JSON to a file instead of stdout
diagg --debug                         # Print discovered packages/components (debug output)
```

`--format json` works exactly like the other formats - it respects `-P` for package-vs-component granularity - but defaults to stdout instead of a file when `-o`/`--output` isn't given, so it composes with `jq`, pre-commit hooks, and other pipelines:

```bash
diagg -P --format json | jq '.nodes[] | select(.role == "hub")'
```

**Structural checks (for CI / pre-commit):**
```bash
diagg check root-budget --max 5 --ignore cmd,config,database   # Fail if too many root-level packages
diagg check generic-names --deny model,core,utils,manager      # Flag generic/low-information package names
```
Both checks exit non-zero on violation (or 0 with `--warn-only`), so they can gate CI without extra tooling.

**Rendering the diagram:**
```bash
# PlantUML (static):
plantuml diagram.puml                # Local rendering (requires PlantUML installed)
# OR use the online renderer at http://www.plantuml.com/plantuml/uml/

# D3.js (interactive):
open diagram-d3.html                 # Opens in browser, no additional tools needed

# Excalidraw:
# Import diagram.excalidraw at https://excalidraw.com/
```

See [diagram.puml](diagram.puml) for an example of diagg analyzing itself.
