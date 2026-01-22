package analyzer

import (
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
}

// Dependency represents a relationship between components
type Dependency struct {
	TargetName string
	TargetType ComponentType
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

	// First pass: classify components
	componentMap := make(map[string]*AnalyzedComponent)
	for _, comp := range components {
		ac := AnalyzedComponent{
			Component:  comp,
			Type:       a.classifyComponent(comp),
			Technology: a.inferTechnology(comp),
		}
		analyzed = append(analyzed, ac)
		componentMap[comp.Name] = &analyzed[len(analyzed)-1]
	}

	// Second pass: extract dependencies from struct fields
	for i := range analyzed {
		analyzed[i].Dependencies = a.extractDependencies(analyzed[i].Component, componentMap)
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
func (a *Analyzer) extractDependencies(comp parser.Component, componentMap map[string]*AnalyzedComponent) []Dependency {
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
				deps = append(deps, Dependency{
					TargetName: typeName,
					TargetType: target.Type,
				})
				seen[typeName] = true
			}
		}
	}

	return deps
}
