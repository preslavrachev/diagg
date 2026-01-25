package generator

import (
	"bytes"
	"strings"
	"testing"

	"github.com/preslavrachev/diagg/analyzer"
	"github.com/preslavrachev/diagg/config"
	"github.com/preslavrachev/diagg/parser"
)

func TestD3Generator_Generate(t *testing.T) {
	cfg := config.New()
	gen := NewD3Generator("Test Diagram", cfg)

	components := []analyzer.AnalyzedComponent{
		{
			Component: parser.Component{
				Name:        "UserService",
				PackageName: "service",
			},
			Type: analyzer.TypeService,
			Role: analyzer.RoleHub,
			Dependencies: []analyzer.Dependency{
				{TargetName: "UserRepository", TargetType: analyzer.TypeRepository},
			},
			Metrics: &analyzer.ComponentMetrics{
				InDegree:    2,
				OutDegree:   1,
				TotalDegree: 3,
			},
		},
		{
			Component: parser.Component{
				Name:        "UserRepository",
				PackageName: "repository",
			},
			Type: analyzer.TypeRepository,
			Role: analyzer.RoleCentral,
			Metrics: &analyzer.ComponentMetrics{
				InDegree:    1,
				OutDegree:   0,
				TotalDegree: 1,
			},
		},
	}

	var buf bytes.Buffer
	err := gen.Generate(components, &buf)
	if err != nil {
		t.Fatalf("Generate() failed: %v", err)
	}

	output := buf.String()

	// Verify HTML structure
	expectedElements := []string{
		"<!DOCTYPE html>",
		"<html>",
		"<title>Test Diagram</title>",
		"<script src=\"https://d3js.org/d3.v7.min.js\"></script>",
		"<h1>Test Diagram</h1>",
		"<svg id=\"graph\">",
		"const data =",
		"\"id\":\"UserService\"",
		"\"id\":\"UserRepository\"",
		"\"source\":\"UserService\"",
		"\"target\":\"UserRepository\"",
		"d3.forceSimulation",
	}

	for _, expected := range expectedElements {
		if !strings.Contains(output, expected) {
			t.Errorf("Output missing expected element: %q", expected)
		}
	}
}

func TestD3Generator_NodeSizing(t *testing.T) {
	cfg := config.New()
	gen := NewD3Generator("Node Size Test", cfg)

	components := []analyzer.AnalyzedComponent{
		{
			Component: parser.Component{Name: "HubComponent", PackageName: "pkg"},
			Type:      analyzer.TypeService,
			Metrics:   &analyzer.ComponentMetrics{TotalDegree: 10},
		},
		{
			Component: parser.Component{Name: "LeafComponent", PackageName: "pkg"},
			Type:      analyzer.TypeRepository,
			Metrics:   &analyzer.ComponentMetrics{TotalDegree: 1},
		},
	}

	graph := gen.buildGraph(components)

	// Hub should be larger than leaf
	hubSize := 0
	leafSize := 0
	for _, node := range graph.Nodes {
		if node.Name == "HubComponent" {
			hubSize = node.Size
		}
		if node.Name == "LeafComponent" {
			leafSize = node.Size
		}
	}

	if hubSize <= leafSize {
		t.Errorf("Hub size (%d) should be larger than leaf size (%d)", hubSize, leafSize)
	}

	// Base size is 10, scaling factor is 2
	expectedHubSize := 10 + (10 * 2)
	if hubSize != expectedHubSize {
		t.Errorf("Hub size = %d, want %d", hubSize, expectedHubSize)
	}

	expectedLeafSize := 10 + (1 * 2)
	if leafSize != expectedLeafSize {
		t.Errorf("Leaf size = %d, want %d", leafSize, expectedLeafSize)
	}
}

func TestD3Generator_InterfaceImplementations(t *testing.T) {
	cfg := config.New()
	gen := NewD3Generator("Interface Test", cfg)

	components := []analyzer.AnalyzedComponent{
		{
			Component: parser.Component{Name: "ConcreteService", PackageName: "service"},
			Type:      analyzer.TypeService,
			Implements: []analyzer.InterfaceImplementation{
				{InterfaceName: "ServiceInterface"},
			},
		},
		{
			Component: parser.Component{Name: "ServiceInterface", PackageName: "service"},
			Type:      analyzer.TypeUnknown,
		},
	}

	graph := gen.buildGraph(components)

	// Should have implementation link
	foundImplementationLink := false
	for _, link := range graph.Links {
		if link.Source == "ConcreteService" && link.Target == "ServiceInterface" && link.Type == "implementation" {
			foundImplementationLink = true
			break
		}
	}

	if !foundImplementationLink {
		t.Error("Expected implementation link not found in graph")
	}
}
