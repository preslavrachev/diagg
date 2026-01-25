package generator

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"html/template"
	"io"

	"github.com/preslavrachev/diagg/analyzer"
	"github.com/preslavrachev/diagg/config"
)

//go:embed templates/d3.html
var d3HTMLTemplate string

// D3Generator generates interactive force-directed graphs using D3.js.
type D3Generator struct {
	title  string
	config *config.Config
}

// NewD3Generator creates a new D3 generator.
func NewD3Generator(title string, cfg *config.Config) *D3Generator {
	if title == "" {
		title = cfg.Defaults.DiagramTitle
	}
	return &D3Generator{
		title:  title,
		config: cfg,
	}
}

// d3Node represents a node in the D3 force graph
type d3Node struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Type    string `json:"type"`
	Package string `json:"package"`
	Role    string `json:"role"`
	Size    int    `json:"size"` // Based on connectivity
}

// d3Link represents an edge in the D3 force graph
type d3Link struct {
	Source string `json:"source"`
	Target string `json:"target"`
	Type   string `json:"type"` // "dependency" or "implementation"
}

// d3Graph is the complete graph structure for D3
type d3Graph struct {
	Nodes []d3Node `json:"nodes"`
	Links []d3Link `json:"links"`
}

// Generate writes the D3 HTML diagram to the writer
func (g *D3Generator) Generate(components []analyzer.AnalyzedComponent, w io.Writer) error {
	// Build graph data structure
	graph := g.buildGraph(components)

	// Serialize to JSON
	graphJSON, err := json.Marshal(graph)
	if err != nil {
		return fmt.Errorf("marshaling graph data: %w", err)
	}

	// Write HTML with embedded D3.js visualization
	if err := g.writeHTML(string(graphJSON), w); err != nil {
		return fmt.Errorf("writing HTML: %w", err)
	}

	return nil
}

// buildGraph converts analyzed components into D3 graph structure
func (g *D3Generator) buildGraph(components []analyzer.AnalyzedComponent) d3Graph {
	graph := d3Graph{
		Nodes: make([]d3Node, 0, len(components)),
		Links: make([]d3Link, 0),
	}

	d3cfg := g.config.Styling.D3

	// Create nodes
	for _, comp := range components {
		size := d3cfg.BaseNodeSize
		if comp.Metrics != nil {
			// Scale size based on total degree
			size = d3cfg.BaseNodeSize + (comp.Metrics.TotalDegree * d3cfg.SizeScaleFactor)
		}

		node := d3Node{
			ID:      comp.Component.Name,
			Name:    comp.Component.Name,
			Type:    string(comp.Type),
			Package: comp.Component.PackageName,
			Role:    string(comp.Role),
			Size:    size,
		}
		graph.Nodes = append(graph.Nodes, node)
	}

	// Create links for dependencies
	for _, comp := range components {
		for _, dep := range comp.Dependencies {
			link := d3Link{
				Source: comp.Component.Name,
				Target: dep.TargetName,
				Type:   "dependency",
			}
			graph.Links = append(graph.Links, link)
		}

		// Create links for interface implementations (dotted style)
		for _, impl := range comp.Implements {
			link := d3Link{
				Source: comp.Component.Name,
				Target: impl.InterfaceName,
				Type:   "implementation",
			}
			graph.Links = append(graph.Links, link)
		}
	}

	return graph
}

type templateData struct {
	Title             string
	GraphJSON         template.JS
	ColorsJSON        template.JS
	LinkDistance      int
	ChargeStrength    int
	CollisionPadding  int
	CenteringStrength float64
	ArrowMarkerSize   int
	ArrowHeadDistance int
}

// writeHTML generates the complete HTML document with D3 visualization
func (g *D3Generator) writeHTML(graphJSON string, w io.Writer) error {
	d3cfg := g.config.Styling.D3

	// Serialize config values as JSON for embedding in JavaScript
	colorsJSON, _ := json.Marshal(d3cfg.PackageColors)

	tmpl, err := template.New("d3").Parse(d3HTMLTemplate)
	if err != nil {
		return fmt.Errorf("parsing template: %w", err)
	}

	data := templateData{
		Title:             g.title,
		GraphJSON:         template.JS(graphJSON),
		ColorsJSON:        template.JS(colorsJSON),
		LinkDistance:      d3cfg.LinkDistance,
		ChargeStrength:    d3cfg.ChargeStrength,
		CollisionPadding:  d3cfg.CollisionPadding,
		CenteringStrength: d3cfg.CenteringStrength,
		ArrowMarkerSize:   d3cfg.ArrowMarkerSize,
		ArrowHeadDistance: d3cfg.ArrowHeadDistance,
	}

	if err := tmpl.Execute(w, data); err != nil {
		return fmt.Errorf("executing template: %w", err)
	}

	return nil
}
