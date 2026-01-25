package analyzer

import (
	"go/types"
	"regexp"
	"strings"

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

// Analyzer infers component types and relationships
type Analyzer struct {
	patterns map[ComponentType]*regexp.Regexp
}

// NewAnalyzer creates a new Analyzer with default patterns
func NewAnalyzer() *Analyzer {
	return &Analyzer{
		patterns: map[ComponentType]*regexp.Regexp{
			TypeService:    regexp.MustCompile(`(?i).*Service$`),
			TypeRepository: regexp.MustCompile(`(?i).*Repository$|.*Repo$|.*Store$`),
			TypeHandler:    regexp.MustCompile(`(?i).*Handler$`),
			TypeController: regexp.MustCompile(`(?i).*Controller$`),
			TypeClient:     regexp.MustCompile(`(?i).*Client$`),
			TypeCache:      regexp.MustCompile(`(?i).*Cache$`),
			TypeGateway:    regexp.MustCompile(`(?i).*Gateway$`),
			TypeMiddleware: regexp.MustCompile(`(?i).*Middleware$`),
		},
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
	// Check against all patterns
	for compType, pattern := range a.patterns {
		if pattern.MatchString(comp.Name) {
			return compType
		}
	}

	return TypeUnknown
}

// inferTechnology makes educated guesses about the technology used
func (a *Analyzer) inferTechnology(comp parser.Component) string {
	switch a.classifyComponent(comp) {
	case TypeService:
		return "Go"
	case TypeRepository:
		// Look for field names that suggest database type
		for _, field := range comp.Fields {
			lower := strings.ToLower(field.TypeName)
			if strings.Contains(lower, "sql") || strings.Contains(lower, "db") {
				return "SQL Database"
			}
			if strings.Contains(lower, "mongo") {
				return "MongoDB"
			}
			if strings.Contains(lower, "redis") {
				return "Redis"
			}
		}
		return "Database"
	case TypeCache:
		return "Cache"
	case TypeHandler, TypeController:
		return "HTTP"
	case TypeClient:
		// Check for gRPC or HTTP client patterns
		for _, field := range comp.Fields {
			if strings.Contains(strings.ToLower(field.TypeName), "grpc") {
				return "gRPC"
			}
		}
		return "HTTP Client"
	case TypeGateway:
		return "External System"
	case TypeMiddleware:
		return "HTTP Middleware"
	default:
		return "Go"
	}
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
// This enables detection of interface implementations
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

	// Second pass: extract dependencies
	for i := range analyzed {
		analyzed[i].Dependencies = a.extractDependenciesWithTypes(
			analyzed[i].Component,
			componentMap,
			typeMap,
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
// AIDEV-NOTE: method-deps; extracts dependencies from struct fields, method parameters, and return types
func (a *Analyzer) extractDependenciesWithTypes(
	comp parser.Component,
	componentMap map[string]*AnalyzedComponent,
	typeMap map[string]types.Type,
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

	return deps
}
