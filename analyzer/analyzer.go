package analyzer

import (
	"go/ast"
	"go/types"
	"strings"

	"github.com/preslavrachev/diagg/config"
	"github.com/preslavrachev/diagg/parser"
)

// ComponentType represents the architectural role of a component
type ComponentType string

const (
	TypeService    ComponentType = "SERVICE"
	TypeRepository ComponentType = "REPOSITORY"
	TypeHandler    ComponentType = "HANDLER"
	TypeController ComponentType = "CONTROLLER"
	TypeClient     ComponentType = "CLIENT"
	TypeCache      ComponentType = "CACHE"
	TypeGateway    ComponentType = "GATEWAY"
	TypeMiddleware ComponentType = "MIDDLEWARE"
	TypeEntrypoint ComponentType = "ENTRYPOINT" // AIDEV-NOTE: main-package-type; represents application entry points
	TypeUnknown    ComponentType = "COMPONENT"
)

// AnalyzedComponent wraps a parsed component with architectural metadata
type AnalyzedComponent struct {
	Component    parser.Component
	Type         ComponentType
	Technology   string
	Dependencies []Dependency
	Implements   []InterfaceImplementation // Interfaces this component implements
	IsInterface  bool                      // True if this component is an interface
	Metrics      *ComponentMetrics         // Graph connectivity metrics for layout optimization
	Role         ComponentRole             // Architectural role based on connectivity
}

// Dependency represents a relationship between components
type Dependency struct {
	TargetName  string
	TargetType  ComponentType
	IsInterface bool // True if dependency is on an interface type
}

// InterfaceImplementation tracks which interfaces a component implements
type InterfaceImplementation struct {
	InterfaceName    string
	InterfacePackage string
}

// Analyzer infers component types and relationships.
// Config is read-only after initialization - safe to share across goroutines.
type Analyzer struct {
	config *config.Config
}

// NewAnalyzer creates a new Analyzer with the provided configuration.
// The config pointer is stored but never modified - it's read-only.
func NewAnalyzer(cfg *config.Config) *Analyzer {
	return &Analyzer{
		config: cfg,
	}
}

// Analyze processes parsed components and infers their architectural types
func (a *Analyzer) Analyze(components []parser.Component) []AnalyzedComponent {
	analyzed := make([]AnalyzedComponent, 0, len(components))

	// First pass: classify components and mark interfaces
	componentMap := make(map[string]*AnalyzedComponent)
	interfaceMap := make(map[string]*AnalyzedComponent) // Track interfaces separately

	for _, comp := range components {
		ac := AnalyzedComponent{
			Component:   comp,
			Type:        a.classifyComponent(comp),
			Technology:  a.inferTechnology(comp),
			IsInterface: comp.Kind == "interface",
		}
		analyzed = append(analyzed, ac)
		componentMap[comp.Name] = &analyzed[len(analyzed)-1]

		if ac.IsInterface {
			interfaceMap[comp.Name] = &analyzed[len(analyzed)-1]
		}
	}

	// Second pass: extract dependencies from struct fields
	for i := range analyzed {
		analyzed[i].Dependencies = a.extractDependencies(analyzed[i].Component, componentMap, interfaceMap)
	}

	// Third pass: calculate metrics and assign roles
	metrics := CalculateMetrics(analyzed)
	for i := range analyzed {
		compName := analyzed[i].Component.Name
		analyzed[i].Metrics = metrics[compName]
		analyzed[i].Role = ClassifyRole(metrics[compName], len(analyzed))
	}

	return analyzed
}

// classifyComponent determines the component type based on naming patterns
func (a *Analyzer) classifyComponent(comp parser.Component) ComponentType {
	// Check for entrypoint first (main package)
	if comp.Kind == "entrypoint" || comp.PackageName == "main" {
		return TypeEntrypoint
	}

	patterns := &a.config.Patterns

	// Check patterns in order - first match wins
	if patterns.Service.MatchString(comp.Name) {
		return TypeService
	}
	if patterns.Repository.MatchString(comp.Name) {
		return TypeRepository
	}
	if patterns.Handler.MatchString(comp.Name) {
		return TypeHandler
	}
	if patterns.Controller.MatchString(comp.Name) {
		return TypeController
	}
	if patterns.Client.MatchString(comp.Name) {
		return TypeClient
	}
	if patterns.Cache.MatchString(comp.Name) {
		return TypeCache
	}
	if patterns.Gateway.MatchString(comp.Name) {
		return TypeGateway
	}
	if patterns.Middleware.MatchString(comp.Name) {
		return TypeMiddleware
	}

	return TypeUnknown
}

// inferTechnology makes educated guesses about the technology used
func (a *Analyzer) inferTechnology(comp parser.Component) string {
	compType := a.classifyComponent(comp)

	// First, check field names for specific technology patterns
	for _, field := range comp.Fields {
		lower := strings.ToLower(field.TypeName)
		for pattern, tech := range a.config.TechnologyRules.DatabasePatterns {
			if strings.Contains(lower, pattern) {
				return tech
			}
		}
	}

	// Fall back to default technology for this component type
	if defaultTech, ok := a.config.TechnologyRules.DefaultByType[string(compType)]; ok {
		return defaultTech
	}

	return a.config.Defaults.UnknownTechnology
}

// extractDependencies identifies dependencies from struct fields
func (a *Analyzer) extractDependencies(
	comp parser.Component,
	componentMap map[string]*AnalyzedComponent,
	interfaceMap map[string]*AnalyzedComponent,
) []Dependency {
	var deps []Dependency
	seen := make(map[string]bool)

	for _, field := range comp.Fields {
		// Extract the base type name (remove package prefix)
		typeName := field.TypeName
		if idx := strings.LastIndex(typeName, "."); idx >= 0 {
			typeName = typeName[idx+1:]
		}

		// Check if this field references another component
		if target, exists := componentMap[typeName]; exists {
			// Avoid duplicates
			if !seen[typeName] {
				dep := Dependency{
					TargetName:  typeName,
					TargetType:  target.Type,
					IsInterface: field.IsInterface || target.IsInterface,
				}
				deps = append(deps, dep)
				seen[typeName] = true
			}
		}
	}

	return deps
}

// AnalyzeWithTypes performs analysis using type information from go/packages
// This enables detection of interface implementations and function body type usage
func (a *Analyzer) AnalyzeWithTypes(
	components []parser.Component,
	pkgTypeInfo *parser.PackageTypeInfo,
) []AnalyzedComponent {
	analyzed := make([]AnalyzedComponent, 0, len(components))

	// First pass: classify components
	componentMap := make(map[string]*AnalyzedComponent)
	interfaceMap := make(map[string]*types.Interface)
	typeMap := make(map[string]types.Type)

	for _, comp := range components {
		ac := AnalyzedComponent{
			Component:   comp,
			Type:        a.classifyComponent(comp),
			Technology:  a.inferTechnology(comp),
			IsInterface: comp.Kind == "interface",
		}
		analyzed = append(analyzed, ac)
		componentMap[comp.Name] = &analyzed[len(analyzed)-1]

		// Get the actual type from the appropriate package scope
		if pkg, ok := pkgTypeInfo.Packages[comp.PackageName]; ok {
			if obj := pkg.Scope().Lookup(comp.Name); obj != nil {
				typeMap[comp.Name] = obj.Type()
				if iface, ok := obj.Type().Underlying().(*types.Interface); ok {
					interfaceMap[comp.Name] = iface
				}
			}
		}
	}

	// Second pass: extract dependencies (including function body analysis)
	for i := range analyzed {
		analyzed[i].Dependencies = a.extractDependenciesWithTypes(
			analyzed[i].Component,
			componentMap,
			typeMap,
			pkgTypeInfo,
		)
	}

	// Third pass: detect interface implementations
	for i := range analyzed {
		if !analyzed[i].IsInterface {
			// Check if this type implements any interface
			typeName := analyzed[i].Component.Name
			if concreteType, ok := typeMap[typeName]; ok {
				// Need to check pointer receiver methods too
				ptrType := types.NewPointer(concreteType)

				for ifaceName, ifaceType := range interfaceMap {
					// Check both value and pointer receiver
					if types.Implements(concreteType, ifaceType) || types.Implements(ptrType, ifaceType) {
						analyzed[i].Implements = append(analyzed[i].Implements, InterfaceImplementation{
							InterfaceName:    ifaceName,
							InterfacePackage: analyzed[i].Component.PackageName,
						})
					}
				}
			}
		}
	}

	// Fourth pass: calculate metrics and assign roles
	metrics := CalculateMetrics(analyzed)
	for i := range analyzed {
		compName := analyzed[i].Component.Name
		analyzed[i].Metrics = metrics[compName]
		analyzed[i].Role = ClassifyRole(metrics[compName], len(analyzed))
	}

	return analyzed
}

// extractDependenciesWithTypes extracts dependencies using type information
// AIDEV-NOTE: method-deps; extracts dependencies from struct fields, method parameters, return types, and function bodies
func (a *Analyzer) extractDependenciesWithTypes(
	comp parser.Component,
	componentMap map[string]*AnalyzedComponent,
	typeMap map[string]types.Type,
	pkgTypeInfo *parser.PackageTypeInfo,
) []Dependency {
	var deps []Dependency
	seen := make(map[string]bool)

	// Helper to add a dependency if it references a known component
	addDep := func(typeName string) {
		// Extract the base type name (remove package prefix, pointers, slices)
		baseName := typeName
		baseName = strings.TrimPrefix(baseName, "*")
		baseName = strings.TrimPrefix(baseName, "[]")
		if idx := strings.LastIndex(baseName, "."); idx >= 0 {
			baseName = baseName[idx+1:]
		}

		// Check if this references another component
		if target, exists := componentMap[baseName]; exists {
			if !seen[baseName] {
				// Determine if it's an interface dependency
				isInterface := false
				if fieldType, ok := typeMap[baseName]; ok {
					_, isInterface = fieldType.Underlying().(*types.Interface)
				}

				dep := Dependency{
					TargetName:  baseName,
					TargetType:  target.Type,
					IsInterface: isInterface || target.IsInterface,
				}
				deps = append(deps, dep)
				seen[baseName] = true
			}
		}
	}

	// Extract dependencies from struct fields
	for _, field := range comp.Fields {
		addDep(field.TypeName)
	}

	// Extract dependencies from method signatures
	for _, method := range comp.Methods {
		// Check method parameters
		for _, param := range method.Parameters {
			addDep(param)
		}
		// Check return types
		for _, ret := range method.Returns {
			addDep(ret)
		}
	}

	// AIDEV-NOTE: function-body-analysis; extract type usage from function bodies (local vars, function calls)
	if pkg, ok := pkgTypeInfo.LoadedPackages[comp.PackageName]; ok {
		// Walk all function declarations in this package and find methods for this type
		for _, syntax := range pkg.Syntax {
			ast.Inspect(syntax, func(n ast.Node) bool {
				funcDecl, ok := n.(*ast.FuncDecl)
				if !ok {
					return true
				}

				// Check if this is a method on our component type
				isMethodOfComp := false
				if funcDecl.Recv != nil {
					for _, recv := range funcDecl.Recv.List {
						recvTypeName := extractReceiverTypeName(recv.Type)
						if recvTypeName == comp.Name {
							isMethodOfComp = true
							break
						}
					}
				}

				// Only analyze methods belonging to this component
				if !isMethodOfComp {
					return true
				}

				// Walk the function body and extract type usage
				// AIDEV-NOTE: type-info-lookup; use package-specific TypesInfo for accurate type resolution
				if funcDecl.Body != nil && pkg.TypesInfo != nil {
					ast.Inspect(funcDecl.Body, func(bodyNode ast.Node) bool {
						// Look for composite literals (e.g., MyType{}, &MyType{})
						if compLit, ok := bodyNode.(*ast.CompositeLit); ok {
							if typeInfo, hasType := pkg.TypesInfo.Types[compLit]; hasType {
								addDepFromType(typeInfo.Type, addDep)
							}
						}

						// Look for function calls and check their return types
						// AIDEV-NOTE: constructor-detection; catches markdown.NewStreamingParser() by return type
						if callExpr, ok := bodyNode.(*ast.CallExpr); ok {
							if typeInfo, hasType := pkg.TypesInfo.Types[callExpr]; hasType {
								addDepFromType(typeInfo.Type, addDep)
							}
						}

						// Look for type assertions (e.g., x.(MyType))
						if typeAssert, ok := bodyNode.(*ast.TypeAssertExpr); ok {
							if typeInfo, hasType := pkg.TypesInfo.Types[typeAssert.Type]; hasType {
								addDepFromType(typeInfo.Type, addDep)
							}
						}

						// Look for variable declarations with explicit types
						if valSpec, ok := bodyNode.(*ast.ValueSpec); ok {
							if valSpec.Type != nil {
								if typeInfo, hasType := pkg.TypesInfo.Types[valSpec.Type]; hasType {
									addDepFromType(typeInfo.Type, addDep)
								}
							}
						}

						return true
					})
				}

				return true
			})
		}
	}

	return deps
}

// extractReceiverTypeName extracts type name from receiver (handles *T and T)
func extractReceiverTypeName(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.StarExpr:
		if ident, ok := t.X.(*ast.Ident); ok {
			return ident.Name
		}
	}
	return ""
}

// addDepFromType extracts type name from types.Type and calls addDep
// AIDEV-NOTE: stdlib-filter; filters out standard library types to avoid noise
func addDepFromType(t types.Type, addDep func(string)) {
	if t == nil {
		return
	}

	// Unwrap pointers and slices
	for {
		switch typ := t.(type) {
		case *types.Pointer:
			t = typ.Elem()
		case *types.Slice:
			t = typ.Elem()
		default:
			goto done
		}
	}
done:

	// Extract named types
	if named, ok := t.(*types.Named); ok {
		obj := named.Obj()
		if obj == nil {
			return
		}

		// Filter out standard library types
		pkg := obj.Pkg()
		if pkg == nil {
			// Built-in types (int, string, error, etc.)
			return
		}

		// AIDEV-NOTE: stdlib-filter; skip stdlib packages
		// Stdlib packages don't have dots AND don't have slashes (e.g., "fmt", "errors")
		// Or they start with known stdlib prefixes (e.g., "golang.org/x/")
		pkgPath := pkg.Path()
		if !strings.Contains(pkgPath, "/") && !strings.Contains(pkgPath, ".") {
			// Standard library package without subdirs (e.g., "fmt", "context", "errors")
			return
		}
		if strings.HasPrefix(pkgPath, "golang.org/x/") {
			// Extended stdlib (e.g., "golang.org/x/sync")
			return
		}

		// Add dependency with package prefix
		typeName := pkg.Name() + "." + obj.Name()
		addDep(typeName)
	}
}
