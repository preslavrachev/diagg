# Interface Detection Implementation Plan

## Current State
The analyzer uses `go/ast` (AST parsing only) which cannot:
- Resolve types across packages
- Determine if a type implements an interface
- Track interface method signatures

## Desired State
Detect and visualize interface contracts:
```
[B] ──uses──▶ [Storage]
                  ↑
                  │ implements (dotted)
                  │
                [C]
```

## Required Changes

### 1. Data Model Extensions

#### `analyzer.Dependency` needs:
```go
type Dependency struct {
    TargetName  string
    TargetType  ComponentType
    IsInterface bool          // NEW: true if dependency is on an interface
    ViaInterface string       // NEW: interface name if indirect
}
```

#### `analyzer.AnalyzedComponent` needs:
```go
type InterfaceImplementation struct {
    InterfaceName string
    InterfacePackage string
}

type AnalyzedComponent struct {
    Component    parser.Component
    Type         ComponentType
    Technology   string
    Dependencies []Dependency
    Implements   []InterfaceImplementation  // NEW: interfaces this component implements
    IsInterface  bool                       // NEW: true if this component is an interface
}
```

### 2. Parser Changes

Switch from `go/ast` to `golang.org/x/tools/go/packages`:

```go
import "golang.org/x/tools/go/packages"

func (p *Parser) ParseDirectory(root string) ([]Component, error) {
    cfg := &packages.Config{
        Mode: packages.NeedName |
              packages.NeedTypes |
              packages.NeedTypesInfo |
              packages.NeedSyntax,
        Dir: root,
    }

    pkgs, err := packages.Load(cfg, "./...")
    // Now we have type information available
}
```

### 3. Analyzer Enhancements

```go
func (a *Analyzer) detectInterfaceImplementations(
    components []Component,
    typeInfo *types.Info,
) map[string][]InterfaceImplementation {

    for _, comp := range components {
        // Get the types.Type for this component
        obj := typeInfo.Uses[comp.astIdent]

        // For each interface in the codebase
        for _, iface := range interfaces {
            // Check if component implements interface
            if types.Implements(obj.Type(), iface.Type()) {
                // Record implementation
            }
        }
    }
}
```

### 4. Generator Updates (PlantUML C4)

```plantuml
' Solid line for direct dependencies
Rel(B, Storage, "uses")

' Dotted line for interface implementation
Rel(C, Storage, "implements", $lineStyle="dotted")
```

## Implementation Complexity

**High**. This is not a trivial change because:

1. **go/packages is heavy** - requires go.mod, full module context
2. **Type resolution is slow** - compiles the entire module graph
3. **Error handling complexity** - type errors in target code break analysis
4. **Cross-module dependencies** - need to handle vendor/, replace directives

## Alternative: Annotation-Based Approach

If full type-checking is too heavyweight, consider:

```go
// @implements pkgb.Storage
type C struct {
    FilePath string
}
```

Pros:
- Keeps AST-only parsing (fast)
- Explicit contracts (documentation)
- No type-checking overhead

Cons:
- Requires manual annotation
- Can get out of sync with code

## Implementation Status

✅ **COMPLETED** - Interface detection is fully implemented using `go/packages`.

The test in `analyzer_test.go::TestAnalyze_InterfaceImplementation` validates the implementation and passes.

### What Was Implemented

- Parser migrated to `golang.org/x/tools/go/packages` with full type information
- `AnalyzedComponent` extended with `Implements` and `IsInterface` fields
- `Dependency` extended with `IsInterface` field
- `AnalyzeWithTypes()` method added for type-aware analysis
- Interface implementation detection via `types.Implements()`
- Cross-package dependency and interface detection working

### Usage

```go
parser := parser.NewParser()
components, typeInfo, err := parser.ParseDirectoryWithTypes("./myproject")
analyzer := analyzer.NewAnalyzer()
analyzed := analyzer.AnalyzeWithTypes(components, typeInfo)
```

This document is kept for historical reference of the design decisions made during implementation.
