package generator

import (
	"fmt"
	"io"
	"strings"

	"github.com/preslavrachev/diagg/analyzer"
	"github.com/preslavrachev/diagg/config"
)

// PlantUMLGenerator generates C4 Component diagrams in PlantUML format.
// Config is read-only after initialization.
type PlantUMLGenerator struct {
	title  string
	config *config.Config
	opts   generatorOptions
}

// NewPlantUMLGenerator creates a new PlantUML generator.
// The config pointer is stored but never modified - it's read-only.
func NewPlantUMLGenerator(title string, cfg *config.Config, options ...Option) *PlantUMLGenerator {
	if title == "" {
		title = cfg.Defaults.DiagramTitle
	}
	opts := defaultOptions()
	for _, opt := range options {
		opt(&opts)
	}

	return &PlantUMLGenerator{
		title:  title,
		config: cfg,
		opts:   opts,
	}
}

// Generate writes the PlantUML diagram to the writer
func (g *PlantUMLGenerator) Generate(components []analyzer.AnalyzedComponent, w io.Writer) error {
	if g.opts.viewMode == ViewModePackage {
		return g.generatePackageView(components, w)
	}

	return g.generateComponentView(components, w)
}

func (g *PlantUMLGenerator) generateComponentView(components []analyzer.AnalyzedComponent, w io.Writer) error {
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

	// Define custom styling tags for visual hierarchy
	if err := g.writeStyleTags(w); err != nil {
		return fmt.Errorf("writing style tags: %w", err)
	}

	if _, err := fmt.Fprintf(w, "title %s\n\n", g.title); err != nil {
		return fmt.Errorf("writing title: %w", err)
	}

	// Sort components by connectivity (hubs first for better layout)
	sortedComponents := g.sortByConnectivity(components)

	// Group components by package
	packageMap := g.groupByPackage(sortedComponents)

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

func (g *PlantUMLGenerator) generatePackageView(components []analyzer.AnalyzedComponent, w io.Writer) error {
	graph := buildViewGraph(components, ViewModePackage, g.config.Defaults.PackageFallback, g.opts)

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

	for _, node := range graph.Nodes {
		id := sanitizeID(node.ID)
		if _, err := fmt.Fprintf(w, "Component(%s, \"%s\", \"Go\", \"Package\")\n", id, node.Name); err != nil {
			return fmt.Errorf("writing package node %s: %w", node.Name, err)
		}
	}

	if _, err := fmt.Fprintln(w); err != nil {
		return err
	}

	for _, edge := range graph.Edges {
		if edge.Type != "dependency" {
			continue
		}

		if _, err := fmt.Fprintf(w, "Rel(%s, %s, \"imports\")\n", sanitizeID(edge.SourceID), sanitizeID(edge.TargetID)); err != nil {
			return fmt.Errorf("writing package relationship %s -> %s: %w", edge.SourceID, edge.TargetID, err)
		}
	}

	if _, err := fmt.Fprintln(w, "\n@enduml"); err != nil {
		return fmt.Errorf("writing footer: %w", err)
	}

	return nil
}

// writeStyleTags defines custom C4 tags for visual hierarchy
// AIDEV-NOTE: visual-hierarchy; defines tags for hub/central/leaf/entrypoint components
func (g *PlantUMLGenerator) writeStyleTags(w io.Writer) error {
	tags := []string{
		g.config.Styling.HubTag,
		g.config.Styling.CentralTag,
		g.config.Styling.LeafTag,
		g.config.Styling.EntrypointTag,
	}

	for _, tag := range tags {
		if _, err := fmt.Fprintln(w, tag); err != nil {
			return fmt.Errorf("writing style tag: %w", err)
		}
	}

	if _, err := fmt.Fprintln(w); err != nil {
		return err
	}

	return nil
}

// writeComponent writes a single component in C4 PlantUML format
func (g *PlantUMLGenerator) writeComponent(comp analyzer.AnalyzedComponent, w io.Writer) error {
	componentID := sanitizeID(comp.QualifiedName())
	description := g.generateDescription(comp)

	// Determine tag - ENTRYPOINT takes precedence over role-based tags
	tag := ""
	if comp.Type == analyzer.TypeEntrypoint {
		tag = "$tags=\"entrypoint\""
	} else {
		switch comp.Role {
		case analyzer.RoleHub:
			tag = "$tags=\"hub\""
		case analyzer.RoleCentral:
			tag = "$tags=\"central\""
		case analyzer.RoleLeaf:
			tag = "$tags=\"leaf\""
		}
	}

	// Use Component macro from C4-PlantUML
	// Format: Component(id, label, technology, description, tags)
	if tag != "" {
		_, err := fmt.Fprintf(w, "Component(%s, \"%s\", \"%s\", \"%s\", %s)\n",
			componentID,
			comp.Component.Name,
			comp.Technology,
			description,
			tag,
		)
		return err
	}

	// No special tag
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
	desc := &g.config.Descriptions

	switch comp.Type {
	case analyzer.TypeService:
		return desc.Service
	case analyzer.TypeRepository:
		return desc.Repository
	case analyzer.TypeHandler:
		return desc.Handler
	case analyzer.TypeController:
		return desc.Controller
	case analyzer.TypeClient:
		return desc.Client
	case analyzer.TypeCache:
		return desc.Cache
	case analyzer.TypeGateway:
		return desc.Gateway
	case analyzer.TypeMiddleware:
		return desc.Middleware
	case analyzer.TypeEntrypoint:
		return desc.Entrypoint
	default:
		pkgName := comp.Component.PackageName
		if pkgName == "" {
			pkgName = g.config.Defaults.PackageFallback
		}
		return fmt.Sprintf(desc.Unknown, pkgName)
	}
}

// writeRelationships writes the relationships (dependencies) for a component
// AIDEV-NOTE: directional-hints; uses Rel_D/Rel_U based on component roles for better layout
func (g *PlantUMLGenerator) writeRelationships(comp analyzer.AnalyzedComponent, w io.Writer) error {
	sourceID := sanitizeID(comp.QualifiedName())

	// Write dependency relationships with directional hints
	for _, dep := range comp.Dependencies {
		targetID := sanitizeID(dep.QualifiedTarget())

		// Choose relationship direction based on roles
		relFunc := g.chooseRelationshipDirection(comp.Role, dep)

		_, err := fmt.Fprintf(w, "%s(%s, %s, \"uses\")\n",
			relFunc,
			sourceID,
			targetID,
		)
		if err != nil {
			return fmt.Errorf("writing relationship to %s: %w", dep.TargetName, err)
		}
	}

	// Write interface implementation relationships (dotted lines)
	for _, impl := range comp.Implements {
		targetID := sanitizeID(impl.InterfacePackage + "." + impl.InterfaceName)

		// Use Rel_Back for dotted lines (C4-PlantUML convention)
		// Rel_Back renders as a dotted/dashed line going backwards
		_, err := fmt.Fprintf(w, "Rel_Back(%s, %s, \"implements\")\n",
			targetID,
			sourceID,
		)
		if err != nil {
			return fmt.Errorf("writing interface implementation to %s: %w", impl.InterfaceName, err)
		}
	}

	return nil
}

// chooseRelationshipDirection selects the appropriate directional relationship based on component roles
// Strategy: Push components AWAY from hubs to keep hubs central
func (g *PlantUMLGenerator) chooseRelationshipDirection(sourceRole analyzer.ComponentRole, dep analyzer.Dependency) string {
	// Determine target role from dependency type (approximation)
	targetRole := g.inferRoleFromType(dep.TargetType)

	// If target is a hub, push source DOWN (source appears above hub)
	if targetRole == analyzer.RoleHub {
		return "Rel_D"
	}

	// If source is a hub, push dependencies UP (dependencies appear below hub)
	if sourceRole == analyzer.RoleHub {
		return "Rel_U"
	}

	// If target is central, push source DOWN slightly
	if targetRole == analyzer.RoleCentral {
		return "Rel_D"
	}

	// If source is a leaf, dependencies should be below it
	if sourceRole == analyzer.RoleLeaf {
		return "Rel_D"
	}

	// Default: no directional hint
	return "Rel"
}

// inferRoleFromType makes an educated guess about a component's role based on its type
// This is used when we only have dependency info without full component context
func (g *PlantUMLGenerator) inferRoleFromType(compType analyzer.ComponentType) analyzer.ComponentRole {
	switch compType {
	case analyzer.TypeService:
		return analyzer.RoleHub // Services tend to be hubs
	case analyzer.TypeRepository, analyzer.TypeCache, analyzer.TypeClient:
		return analyzer.RoleCentral // Data layer tends to be central
	case analyzer.TypeHandler, analyzer.TypeController:
		return analyzer.RoleLeaf // Handlers are typically leaves
	default:
		return analyzer.RoleOrdinary
	}
}

// sortByConnectivity orders components by their connectivity (hubs first, then central, then others)
// This influences PlantUML's layout algorithm to place important components more centrally
// AIDEV-NOTE: layout-optimization; component ordering affects PlantUML auto-layout
func (g *PlantUMLGenerator) sortByConnectivity(components []analyzer.AnalyzedComponent) []analyzer.AnalyzedComponent {
	// Create a copy to avoid modifying the original slice
	sorted := make([]analyzer.AnalyzedComponent, len(components))
	copy(sorted, components)

	// Sort by role priority, then by total degree
	// Priority: Hub > Central > Ordinary > Leaf
	rolePriority := map[analyzer.ComponentRole]int{
		analyzer.RoleHub:      0,
		analyzer.RoleCentral:  1,
		analyzer.RoleOrdinary: 2,
		analyzer.RoleLeaf:     3,
	}

	// Simple bubble sort (fine for typical component counts)
	for i := 0; i < len(sorted); i++ {
		for j := i + 1; j < len(sorted); j++ {
			iPriority := rolePriority[sorted[i].Role]
			jPriority := rolePriority[sorted[j].Role]

			// Swap if j has higher priority (lower number)
			shouldSwap := false
			if jPriority < iPriority {
				shouldSwap = true
			} else if jPriority == iPriority {
				// Same role, sort by total degree (higher first)
				if sorted[j].Metrics != nil && sorted[i].Metrics != nil {
					shouldSwap = sorted[j].Metrics.TotalDegree > sorted[i].Metrics.TotalDegree
				}
			}

			if shouldSwap {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}

	return sorted
}

// groupByPackage organizes components by their package name
func (g *PlantUMLGenerator) groupByPackage(components []analyzer.AnalyzedComponent) map[string][]analyzer.AnalyzedComponent {
	pkgMap := make(map[string][]analyzer.AnalyzedComponent)

	for _, comp := range components {
		pkgName := comp.Component.PackageName
		if pkgName == "" {
			pkgName = g.config.Defaults.PackageFallback
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
