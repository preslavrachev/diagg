package generator

import (
	"fmt"
	"io"
	"strings"

	"github.com/preslavrachev/diagg/analyzer"
)

// PlantUMLGenerator generates C4 Component diagrams in PlantUML format
type PlantUMLGenerator struct {
	title string
}

// NewPlantUMLGenerator creates a new PlantUML generator
func NewPlantUMLGenerator(title string) *PlantUMLGenerator {
	if title == "" {
		title = "Component Diagram"
	}
	return &PlantUMLGenerator{
		title: title,
	}
}

// Generate writes the PlantUML diagram to the writer
func (g *PlantUMLGenerator) Generate(components []analyzer.AnalyzedComponent, w io.Writer) error {
	// Write header
	if _, err := fmt.Fprintln(w, "@startuml"); err != nil {
		return fmt.Errorf("writing header: %w", err)
	}
	if _, err := fmt.Fprintln(w, "!include https://raw.githubusercontent.com/plantuml-stdlib/C4-PlantUML/master/C4_Component.puml"); err != nil {
		return fmt.Errorf("writing C4 include: %w", err)
	}
	if _, err := fmt.Fprintln(w); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "title %s\n\n", g.title); err != nil {
		return fmt.Errorf("writing title: %w", err)
	}

	// Group components by package
	packageMap := g.groupByPackage(components)

	// Write components grouped by package
	for pkgName, pkgComponents := range packageMap {
		if err := g.writePackageBoundary(pkgName, pkgComponents, w); err != nil {
			return fmt.Errorf("writing package %s: %w", pkgName, err)
		}
	}

	if _, err := fmt.Fprintln(w); err != nil {
		return err
	}

	// Write relationships
	for _, comp := range components {
		if err := g.writeRelationships(comp, w); err != nil {
			return fmt.Errorf("writing relationships for %s: %w", comp.Component.Name, err)
		}
	}

	// Write footer
	if _, err := fmt.Fprintln(w, "\n@enduml"); err != nil {
		return fmt.Errorf("writing footer: %w", err)
	}

	return nil
}

// writeComponent writes a single component in C4 PlantUML format
func (g *PlantUMLGenerator) writeComponent(comp analyzer.AnalyzedComponent, w io.Writer) error {
	componentID := sanitizeID(comp.Component.Name)
	description := g.generateDescription(comp)

	// Use Component macro from C4-PlantUML
	// Format: Component(id, label, technology, description)
	_, err := fmt.Fprintf(w, "Component(%s, \"%s\", \"%s\", \"%s\")\n",
		componentID,
		comp.Component.Name,
		comp.Technology,
		description,
	)

	return err
}

// generateDescription creates a human-readable description for the component
func (g *PlantUMLGenerator) generateDescription(comp analyzer.AnalyzedComponent) string {
	switch comp.Type {
	case analyzer.TypeService:
		return "Business logic service"
	case analyzer.TypeRepository:
		return "Data access layer"
	case analyzer.TypeHandler:
		return "HTTP request handler"
	case analyzer.TypeController:
		return "Request controller"
	case analyzer.TypeClient:
		return "External service client"
	case analyzer.TypeCache:
		return "Caching layer"
	case analyzer.TypeGateway:
		return "External gateway"
	case analyzer.TypeMiddleware:
		return "Middleware component"
	default:
		return fmt.Sprintf("%s component", comp.Component.PackageName)
	}
}

// writeRelationships writes the relationships (dependencies) for a component
func (g *PlantUMLGenerator) writeRelationships(comp analyzer.AnalyzedComponent, w io.Writer) error {
	if len(comp.Dependencies) == 0 {
		return nil
	}

	sourceID := sanitizeID(comp.Component.Name)

	for _, dep := range comp.Dependencies {
		targetID := sanitizeID(dep.TargetName)

		// Format: Rel(from, to, label)
		_, err := fmt.Fprintf(w, "Rel(%s, %s, \"uses\")\n",
			sourceID,
			targetID,
		)
		if err != nil {
			return fmt.Errorf("writing relationship to %s: %w", dep.TargetName, err)
		}
	}

	return nil
}

// groupByPackage organizes components by their package name
func (g *PlantUMLGenerator) groupByPackage(components []analyzer.AnalyzedComponent) map[string][]analyzer.AnalyzedComponent {
	pkgMap := make(map[string][]analyzer.AnalyzedComponent)

	for _, comp := range components {
		pkgName := comp.Component.PackageName
		if pkgName == "" {
			pkgName = "main"
		}
		pkgMap[pkgName] = append(pkgMap[pkgName], comp)
	}

	return pkgMap
}

// writePackageBoundary writes a Container_Boundary grouping for a package
func (g *PlantUMLGenerator) writePackageBoundary(pkgName string, components []analyzer.AnalyzedComponent, w io.Writer) error {
	boundaryID := sanitizeID(pkgName + "_boundary")

	// Open boundary
	if _, err := fmt.Fprintf(w, "Container_Boundary(%s, \"%s\") {\n", boundaryID, pkgName); err != nil {
		return fmt.Errorf("writing boundary start: %w", err)
	}

	// Write components within this package
	for _, comp := range components {
		if err := g.writeComponent(comp, w); err != nil {
			return fmt.Errorf("writing component %s: %w", comp.Component.Name, err)
		}
	}

	// Close boundary
	if _, err := fmt.Fprintln(w, "}"); err != nil {
		return fmt.Errorf("writing boundary end: %w", err)
	}
	if _, err := fmt.Fprintln(w); err != nil {
		return err
	}

	return nil
}

// sanitizeID converts a component name to a valid PlantUML identifier
func sanitizeID(name string) string {
	// Replace invalid characters with underscores
	id := strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' {
			return r
		}
		return '_'
	}, name)

	// Ensure it doesn't start with a number
	if len(id) > 0 && id[0] >= '0' && id[0] <= '9' {
		id = "_" + id
	}

	return id
}
