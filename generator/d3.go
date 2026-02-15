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
	opts   generatorOptions
}

// NewD3Generator creates a new D3 generator.
func NewD3Generator(title string, cfg *config.Config, options ...Option) *D3Generator {
	if title == "" {
		title = cfg.Defaults.DiagramTitle
	}

	opts := defaultOptions()
	for _, opt := range options {
		opt(&opts)
	}

	return &D3Generator{
		title:  title,
		config: cfg,
		opts:   opts,
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
	Main    bool   `json:"main"`
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
	view := buildViewGraph(components, g.opts.viewMode, g.config.Defaults.PackageFallback, g.opts)

	graph := d3Graph{
		Nodes: make([]d3Node, 0, len(view.Nodes)),
		Links: make([]d3Link, 0),
	}

	d3cfg := g.config.Styling.D3

	// Create nodes
	for _, node := range view.Nodes {
		size := d3cfg.BaseNodeSize
		if node.Metrics != nil {
			// Scale size based on total degree
			size = d3cfg.BaseNodeSize + (node.Metrics.TotalDegree * d3cfg.SizeScaleFactor)
		}

		d3node := d3Node{
			ID:      node.ID,
			Name:    node.Name,
			Type:    node.Type,
			Package: node.Package,
			Role:    string(node.Role),
			Size:    size,
			Main:    node.IsMainLike,
		}
		graph.Nodes = append(graph.Nodes, d3node)
	}

	// Create links
	for _, edge := range view.Edges {
		graph.Links = append(graph.Links, d3Link{
			Source: edge.SourceID,
			Target: edge.TargetID,
			Type:   edge.Type,
		})
	}

	return graph
}

type templateData struct {
	Title             string
	ViewMode          ViewMode
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
		ViewMode:          g.opts.viewMode,
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
