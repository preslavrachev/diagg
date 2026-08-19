package analyzer

import (
	"go/ast"
	"go/types"
	"strings"

	"github.com/preslavrachev/diagg/config"
	"github.com/preslavrachev/diagg/parser"
	"golang.org/x/tools/go/packages"
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

	// FreeFunctionReferences counts references to this component from free (non-method)
	// functions - e.g. a plain constructor caller, or a same-package function that wires
	// this component into another one. Excluded: a free function referencing this type as
	// its own declared return value while living in this component's own package (the
	// `func NewX() *X { return &X{} }` constructor shape) - see freeFunctionReferenceCounts.
	// It is tracked separately from Metrics.InDegree/Role/TotalDegree, which drive diagram layout and
	// visual hierarchy: these references have no corresponding Dependency edge (there is
	// no component to draw the edge from), so folding them into the graph metrics would
	// change a node's role/size with no visible relationship to explain why. Consumers
	// that want a usage signal for "is this exported type referenced anywhere" (e.g. an
	// unused-component check) should treat Metrics.InDegree + FreeFunctionReferences > 0
	// as "used".
	FreeFunctionReferences int
}

// Dependency represents a relationship between components
type Dependency struct {
	TargetName    string
	TargetPackage string // Package of the target component
	TargetType    ComponentType
	IsInterface   bool // True if dependency is on an interface type
}

// InterfaceImplementation tracks which interfaces a component implements
type InterfaceImplementation struct {
	InterfaceName    string
	InterfacePackage string
}

// qualifiedName returns a package-qualified identifier for map keys and cross-references.
// AIDEV-NOTE: qualified-name; used everywhere to prevent name collisions across packages
func qualifiedName(pkg, name string) string {
	if pkg == "" {
		return name
	}
	return pkg + "." + name
}

// QualifiedName returns the package-qualified name for this component.
func (ac *AnalyzedComponent) QualifiedName() string {
	return qualifiedName(ac.Component.PackageName, ac.Component.Name)
}

// QualifiedTarget returns the package-qualified name for this dependency target.
func (d *Dependency) QualifiedTarget() string {
	return qualifiedName(d.TargetPackage, d.TargetName)
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
		qn := qualifiedName(comp.PackageName, comp.Name)
		componentMap[qn] = &analyzed[len(analyzed)-1]

		if ac.IsInterface {
			interfaceMap[qn] = &analyzed[len(analyzed)-1]
		}
	}

	// Second pass: extract dependencies from struct fields
	for i := range analyzed {
		analyzed[i].Dependencies = a.extractDependencies(analyzed[i].Component, componentMap, interfaceMap)
	}

	// Third pass: calculate metrics and assign roles
	metrics := CalculateMetrics(analyzed)
	for i := range analyzed {
		qn := analyzed[i].QualifiedName()
		analyzed[i].Metrics = metrics[qn]
		analyzed[i].Role = ClassifyRole(metrics[qn], len(analyzed))
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
		target := a.lookupComponent(field.TypeName, componentMap)
		if target == nil {
			continue
		}

		qn := target.QualifiedName()
		if !seen[qn] {
			dep := Dependency{
				TargetName:    target.Component.Name,
				TargetPackage: target.Component.PackageName,
				TargetType:    target.Type,
				IsInterface:   field.IsInterface || target.IsInterface,
			}
			deps = append(deps, dep)
			seen[qn] = true
		}
	}

	return deps
}

// lookupComponent resolves a type name (possibly package-qualified like "pkgb.B")
// to an AnalyzedComponent in the map keyed by qualifiedName.
func (a *Analyzer) lookupComponent(typeName string, componentMap map[string]*AnalyzedComponent) *AnalyzedComponent {
	// Strip pointer/slice prefixes
	clean := strings.TrimPrefix(typeName, "*")
	clean = strings.TrimPrefix(clean, "[]")

	// Try direct qualified lookup (e.g. "pkgb.B" matches key "pkgb.B")
	if target, ok := componentMap[clean]; ok {
		return target
	}

	// Extract bare name and search all entries
	bareName := clean
	if idx := strings.LastIndex(clean, "."); idx >= 0 {
		bareName = clean[idx+1:]
	}

	// If the field has a package prefix, try that specific qualified name
	if bareName != clean {
		// clean is already "pkg.Name", tried above
		// Fall through to scan
	}

	// Scan for a unique match by bare name (backward compat for single-package cases)
	var match *AnalyzedComponent
	matches := 0
	for _, comp := range componentMap {
		if comp.Component.Name == bareName {
			match = comp
			matches++
		}
	}
	if matches == 1 {
		return match
	}

	return nil
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
		qn := qualifiedName(comp.PackageName, comp.Name)
		componentMap[qn] = &analyzed[len(analyzed)-1]

		// Get the actual type from the appropriate package scope.
		pkg, ok := pkgTypeInfo.PackagesByPath[comp.PackagePath]
		if !ok {
			pkg, ok = pkgTypeInfo.Packages[comp.PackageName]
		}
		if ok {
			if obj := pkg.Scope().Lookup(comp.Name); obj != nil {
				typeMap[qn] = obj.Type()
				if iface, ok := obj.Type().Underlying().(*types.Interface); ok {
					interfaceMap[qn] = iface
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
			qn := analyzed[i].QualifiedName()
			if concreteType, ok := typeMap[qn]; ok {
				// Need to check pointer receiver methods too
				ptrType := types.NewPointer(concreteType)

				for ifaceQN, ifaceType := range interfaceMap {
					// Check both value and pointer receiver
					if types.Implements(concreteType, ifaceType) || types.Implements(ptrType, ifaceType) {
						// Extract package and name from the qualified key
						ifacePkg := ""
						ifaceName := ifaceQN
						if idx := strings.LastIndex(ifaceQN, "."); idx >= 0 {
							ifacePkg = ifaceQN[:idx]
							ifaceName = ifaceQN[idx+1:]
						}
						analyzed[i].Implements = append(analyzed[i].Implements, InterfaceImplementation{
							InterfaceName:    ifaceName,
							InterfacePackage: ifacePkg,
						})
					}
				}
			}
		}
	}

	// Fourth pass: calculate metrics and assign roles. Free-function usage is recorded
	// separately (FreeFunctionReferences) rather than folded in here, since it has no
	// Dependency edge to back it - see the AnalyzedComponent.FreeFunctionReferences doc.
	metrics := CalculateMetrics(analyzed)
	freeFunctionRefs := a.freeFunctionReferenceCounts(componentMap, pkgTypeInfo)
	for i := range analyzed {
		qn := analyzed[i].QualifiedName()
		analyzed[i].Metrics = metrics[qn]
		analyzed[i].Role = ClassifyRole(metrics[qn], len(analyzed))
		analyzed[i].FreeFunctionReferences = freeFunctionRefs[qn]
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
		target := a.lookupComponent(typeName, componentMap)
		if target == nil {
			return
		}

		qn := target.QualifiedName()
		if !seen[qn] {
			// Determine if it's an interface dependency
			isInterface := false
			if fieldType, ok := typeMap[qn]; ok {
				_, isInterface = fieldType.Underlying().(*types.Interface)
			}

			dep := Dependency{
				TargetName:    target.Component.Name,
				TargetPackage: target.Component.PackageName,
				TargetType:    target.Type,
				IsInterface:   isInterface || target.IsInterface,
			}
			deps = append(deps, dep)
			seen[qn] = true
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
	pkg, ok := pkgTypeInfo.LoadedPackagesByPath[comp.PackagePath]
	if !ok {
		pkg, ok = pkgTypeInfo.LoadedPackages[comp.PackageName]
	}
	if ok {
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
					walkFuncBodyTypeUsage(funcDecl.Body, pkg.TypesInfo, addDep)
				}

				return true
			})
		}
	}

	return deps
}

// walkFuncBodyTypeUsage extracts type usage from a function body (composite literals,
// call return types, type assertions, explicit var declarations) and reports each
// referenced named type to addDep. Shared by extractDependenciesWithTypes (methods)
// and freeFunctionReferenceCounts (package-level functions) so both sources of type usage
// are detected identically.
func walkFuncBodyTypeUsage(body *ast.BlockStmt, typesInfo *types.Info, addDep func(string)) {
	ast.Inspect(body, func(bodyNode ast.Node) bool {
		// Look for composite literals (e.g., MyType{}, &MyType{})
		if compLit, ok := bodyNode.(*ast.CompositeLit); ok {
			if typeInfo, hasType := typesInfo.Types[compLit]; hasType {
				addDepFromType(typeInfo.Type, addDep)
			}
		}

		// Look for function calls and check their return types
		// AIDEV-NOTE: constructor-detection; catches markdown.NewStreamingParser() by return type
		if callExpr, ok := bodyNode.(*ast.CallExpr); ok {
			if typeInfo, hasType := typesInfo.Types[callExpr]; hasType {
				addDepFromType(typeInfo.Type, addDep)
			}
		}

		// Look for type assertions (e.g., x.(MyType))
		if typeAssert, ok := bodyNode.(*ast.TypeAssertExpr); ok {
			if typeInfo, hasType := typesInfo.Types[typeAssert.Type]; hasType {
				addDepFromType(typeInfo.Type, addDep)
			}
		}

		// Look for variable declarations with explicit types
		if valSpec, ok := bodyNode.(*ast.ValueSpec); ok {
			if valSpec.Type != nil {
				if typeInfo, hasType := typesInfo.Types[valSpec.Type]; hasType {
					addDepFromType(typeInfo.Type, addDep)
				}
			}
		}

		return true
	})
}

// freeFunctionReferenceCounts walks package-level (non-method) function bodies across
// every loaded package and counts references to known components. This is a usage
// signal, not a graph in-degree: see AnalyzedComponent.FreeFunctionReferences for why
// it is kept out of Metrics/Role.
//
// AIDEV-NOTE: free-function-deps; extractDependenciesWithTypes only walks methods
// (isMethodOfComp above), so a component reachable only through a free function -
// a plain constructor caller, a helper, anything without a receiver - previously got
// no inbound edge and read as unused even when genuinely referenced. main packages
// already get equivalent whole-package coverage via parser.extractMainComponent; this
// mirrors that for every other package, but feeds metrics directly instead of
// synthesizing a visible graph node, so it does not change diagram output.
//
// AIDEV-NOTE: exclude-self-construction, not same-package; a function that both lives
// in a type's package AND declares that same type as one of its own return values is
// that type's constructor (the `func NewX() *X { return &X{} }` shape) - referencing
// the type inside its own body is definitional, not usage evidence. A blanket
// same-package skip is too broad: it would also hide a different free function in the
// same package that genuinely wires two components together (e.g.
// `func BuildService() *Service { repo := NewRepo(); return &Service{repo: repo} }`),
// since BuildService's own return type (*Service) doesn't match Repo, so Repo is not
// self-referential from BuildService's point of view and must still be counted.
//
// AIDEV-NOTE: dual-package-map; mirrors the LoadedPackagesByPath -> LoadedPackages
// fallback in extractDependenciesWithTypes (comp.PackagePath / comp.PackageName), since
// a hand-built PackageTypeInfo (tests, other callers) may only populate one of the two
// maps. Both maps are merged here, deduped by the *packages.Package itself so a package
// present in both is only walked once, and each package's own PkgPath field (not the
// map key) is used for identity - LoadedPackages is keyed by package name, not path.
func (a *Analyzer) freeFunctionReferenceCounts(componentMap map[string]*AnalyzedComponent, pkgTypeInfo *parser.PackageTypeInfo) map[string]int {
	counts := make(map[string]int)
	seenPkg := make(map[*packages.Package]bool)

	pkgs := make([]*packages.Package, 0, len(pkgTypeInfo.LoadedPackagesByPath)+len(pkgTypeInfo.LoadedPackages))
	for _, pkg := range pkgTypeInfo.LoadedPackagesByPath {
		pkgs = append(pkgs, pkg)
	}
	for _, pkg := range pkgTypeInfo.LoadedPackages {
		pkgs = append(pkgs, pkg)
	}

	for _, pkg := range pkgs {
		// main packages are already fully covered by parser.extractMainComponent,
		// which walks every type expression in the package, not just func main().
		if pkg.Name == "main" || seenPkg[pkg] {
			continue
		}
		seenPkg[pkg] = true

		if pkg.TypesInfo == nil {
			continue
		}

		pkgPath := pkg.PkgPath

		for _, syntax := range pkg.Syntax {
			ast.Inspect(syntax, func(n ast.Node) bool {
				funcDecl, ok := n.(*ast.FuncDecl)
				if !ok || funcDecl.Recv != nil || funcDecl.Body == nil {
					return true
				}

				// Collect the function's own declared return types, so its own
				// constructor pattern can be excluded without excluding the whole package.
				ownReturnTypes := make(map[string]bool)
				if funcDecl.Type.Results != nil {
					collect := func(typeName string) { ownReturnTypes[typeName] = true }
					for _, field := range funcDecl.Type.Results.List {
						if typeInfo, hasType := pkg.TypesInfo.Types[field.Type]; hasType {
							addDepFromType(typeInfo.Type, collect)
						}
					}
				}

				addDep := func(typeName string) {
					target := a.lookupComponent(typeName, componentMap)
					if target == nil {
						return
					}
					if target.Component.PackagePath == pkgPath && ownReturnTypes[typeName] {
						return
					}
					counts[target.QualifiedName()]++
				}

				walkFuncBodyTypeUsage(funcDecl.Body, pkg.TypesInfo, addDep)
				return true
			})
		}
	}

	return counts
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

	// AIDEV-NOTE: multi-return-constructor; a call expression's type is a *types.Tuple
	// when the callee returns more than one value (the idiomatic `(*Repo, error)` shape),
	// so it must be unpacked before the *types.Named check below, or the whole call is
	// silently dropped and constructors following Go's own error-return convention are
	// invisible to dependency/usage detection.
	if tuple, ok := t.(*types.Tuple); ok {
		for v := range tuple.Variables() {
			addDepFromType(v.Type(), addDep)
		}
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
