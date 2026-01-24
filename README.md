# diagg

Zero-config C4 component diagrams from your Go codebase. Point it at your project, get architectural diagrams instantly.

```bash
diagg ./myproject
```

Generates PlantUML C4 diagrams showing components, dependencies, and interface implementations - no configuration needed.

## Why diagg?

Unlike tools that require manual wiring or extensive configuration, diagg infers everything from your code structure. It uses naming conventions (`*Service`, `*Repository`, `*Handler`, etc.) and full type analysis to detect relationships and interface implementations automatically.

**What you get:**
- Solid lines for dependencies between components
- Dotted lines showing interface implementations
- Architectural layers based on naming patterns
- Cross-package relationship detection

## Install

```bash
go install github.com/preslavrachev/diagg/cmd/diagg@latest
```

## Usage

```bash
diagg [directory]                    # Analyze and output to stdout
diagg -o diagram.puml -t "My App"    # Custom output file and title
```

See [diagram.puml](diagram.puml) for an example of diagg analyzing itself.
