package generator

import (
	"testing"

	"github.com/preslavrachev/diagg/analyzer"
	"github.com/preslavrachev/diagg/config"
	"github.com/preslavrachev/diagg/parser"
)

func TestD3Generator_PackageModeUsesImportGraphForPackagesWithoutTypes(t *testing.T) {
	cfg := config.New()
	gen := NewD3Generator(
		"Package Import Mode",
		cfg,
		WithViewMode(ViewModePackage),
		WithPackageImports(map[string][]string{
			"test/cmd/server": {"test/router"},
			"test/router":     {},
		}),
		WithPackageNamesByPath(map[string]string{
			"test/cmd/server": "main",
			"test/router":     "router",
		}),
	)

	components := []analyzer.AnalyzedComponent{
		{
			Component: parser.Component{
				Name:        "server",
				PackageName: "main",
				PackagePath: "test/cmd/server",
				Kind:        "entrypoint",
			},
			Type: analyzer.TypeEntrypoint,
		},
	}

	graph := gen.buildGraph(components)

	nodes := make(map[string]bool)
	for _, node := range graph.Nodes {
		nodes[node.ID] = true
	}

	if !nodes["test/cmd/server"] {
		t.Fatal("expected test/cmd/server node in package graph")
	}
	if !nodes["test/router"] {
		t.Fatal("expected test/router node in package graph")
	}

	foundEdge := false
	for _, link := range graph.Links {
		if link.Source == "test/cmd/server" && link.Target == "test/router" && link.Type == "dependency" {
			foundEdge = true
			break
		}
	}
	if !foundEdge {
		t.Fatalf("expected dependency edge test/cmd/server -> test/router, got links: %+v", graph.Links)
	}
}

// Regression test: before package metrics were computed, all package nodes had the
// same base size in D3 package mode, regardless of connectivity.
func TestD3Generator_PackageModeScalesNodeSizeByConnectivity(t *testing.T) {
	cfg := config.New()
	gen := NewD3Generator(
		"Package Connectivity",
		cfg,
		WithViewMode(ViewModePackage),
		WithPackageImports(map[string][]string{
			"test/a":    {"test/core"},
			"test/b":    {"test/core"},
			"test/core": {"test/util"},
			"test/util": {},
		}),
	)

	graph := gen.buildGraph(nil)

	sizeByID := make(map[string]int)
	for _, node := range graph.Nodes {
		sizeByID[node.ID] = node.Size
	}

	coreSize, coreFound := sizeByID["test/core"]
	utilSize, utilFound := sizeByID["test/util"]
	if !coreFound || !utilFound {
		t.Fatalf("expected nodes for test/core and test/util, got: %+v", sizeByID)
	}

	if coreSize <= utilSize {
		t.Fatalf("expected high-connectivity package test/core (size=%d) to be larger than test/util (size=%d)", coreSize, utilSize)
	}
}
