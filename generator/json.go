package generator

import (
	"encoding/json"
	"io"
	"sort"

	"github.com/preslavrachev/diagg/analyzer"
	"github.com/preslavrachev/diagg/config"
)

// JSONGenerator emits the same component/package graph the other generators
// render, as machine-readable JSON. It supports both view modes exactly like
// PlantUMLGenerator, D3Generator, and ExcalidrawGenerator.
type JSONGenerator struct {
	title  string
	config *config.Config
	opts   generatorOptions
}

// NewJSONGenerator creates a new JSON generator.
func NewJSONGenerator(title string, cfg *config.Config, options ...Option) *JSONGenerator {
	if title == "" {
		title = cfg.Defaults.DiagramTitle
	}

	opts := defaultOptions()
	for _, opt := range options {
		opt(&opts)
	}

	return &JSONGenerator{
		title:  title,
		config: cfg,
		opts:   opts,
	}
}

type jsonGraphNode struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Package     string `json:"package"`
	Type        string `json:"type"`
	Technology  string `json:"technology,omitempty"`
	Role        string `json:"role"`
	InDegree    int    `json:"in_degree"`
	OutDegree   int    `json:"out_degree"`
	TotalDegree int    `json:"total_degree"`
	IsMainLike  bool   `json:"is_main_like,omitempty"`
	IsInterface bool   `json:"is_interface,omitempty"`
}

type jsonGraphEdge struct {
	Source     string `json:"source"`
	Target     string `json:"target"`
	Type       string `json:"type"`
	TargetType string `json:"target_type,omitempty"`
}

type jsonGraphDoc struct {
	Title    string          `json:"title"`
	ViewMode string          `json:"view_mode"`
	Nodes    []jsonGraphNode `json:"nodes"`
	Edges    []jsonGraphEdge `json:"edges"`
}

// Generate writes the component/package graph as JSON to the writer.
func (g *JSONGenerator) Generate(components []analyzer.AnalyzedComponent, w io.Writer) error {
	view := buildViewGraph(components, g.opts.viewMode, g.config.Defaults.PackageFallback, g.opts)

	doc := jsonGraphDoc{
		Title:    g.title,
		ViewMode: string(g.opts.viewMode),
		Nodes:    make([]jsonGraphNode, 0, len(view.Nodes)),
		Edges:    make([]jsonGraphEdge, 0, len(view.Edges)),
	}

	for _, node := range view.Nodes {
		n := jsonGraphNode{
			ID:          node.ID,
			Name:        node.Name,
			Package:     node.Package,
			Type:        node.Type,
			Technology:  node.Technology,
			Role:        string(node.Role),
			IsMainLike:  node.IsMainLike,
			IsInterface: node.IsInterface,
		}
		if node.Metrics != nil {
			n.InDegree = node.Metrics.InDegree
			n.OutDegree = node.Metrics.OutDegree
			n.TotalDegree = node.Metrics.TotalDegree
		}
		doc.Nodes = append(doc.Nodes, n)
	}

	for _, edge := range view.Edges {
		doc.Edges = append(doc.Edges, jsonGraphEdge{
			Source:     edge.SourceID,
			Target:     edge.TargetID,
			Type:       edge.Type,
			TargetType: string(edge.TargetType),
		})
	}

	// AIDEV-NOTE: json-stable-order; package view is built from Go maps
	// (buildImportBasedPackageViewGraph), so nodes/edges must be sorted here or
	// CI diffs/snapshots of --format json output would be noisy across runs.
	sort.Slice(doc.Nodes, func(i, j int) bool {
		return doc.Nodes[i].ID < doc.Nodes[j].ID
	})
	sort.Slice(doc.Edges, func(i, j int) bool {
		a, b := doc.Edges[i], doc.Edges[j]
		if a.Source != b.Source {
			return a.Source < b.Source
		}
		if a.Target != b.Target {
			return a.Target < b.Target
		}
		return a.Type < b.Type
	})

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(doc)
}
