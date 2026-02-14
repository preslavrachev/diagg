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
- Multiple output formats: PlantUML C4 diagrams or interactive D3.js force-directed graphs

## Install

```bash
go install github.com/preslavrachev/diagg/cmd/diagg@latest
```

## Usage

```bash
diagg [directory]                     # Analyze and generate diagram.puml (PlantUML)
diagg -o output.puml -t "My App"      # Custom output file and title
diagg --format d3 -o diagram.html     # Generate interactive D3.js force-directed graph
diagg -P                              # Package-only mode (A imports B => A -> B)
diagg --debug                         # Print discovered packages/components (debug output)
```

**Rendering the diagram:**
```bash
# PlantUML (static):
plantuml diagram.puml                # Local rendering (requires PlantUML installed)
# OR use the online renderer at http://www.plantuml.com/plantuml/uml/

# D3.js (interactive):
open diagram-d3.html                 # Opens in browser, no additional tools needed
```

See [diagram.puml](diagram.puml) for an example of diagg analyzing itself.
