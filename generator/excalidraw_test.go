package generator

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/preslavrachev/diagg/analyzer"
	"github.com/preslavrachev/diagg/config"
	"github.com/preslavrachev/diagg/parser"
)

func TestExcalidrawGenerator_Generate(t *testing.T) {
	cfg := config.New()
	gen := NewExcalidrawGenerator("Test Diagram", cfg)

	components := []analyzer.AnalyzedComponent{
		{
			Component: parser.Component{
				Name:        "UserService",
				PackageName: "service",
			},
			Type:       analyzer.TypeService,
			Technology: "Go",
			Role:       analyzer.RoleHub,
			Dependencies: []analyzer.Dependency{
				{TargetName: "UserRepository", TargetPackage: "repository", TargetType: analyzer.TypeRepository},
			},
			Metrics: &analyzer.ComponentMetrics{TotalDegree: 3},
		},
		{
			Component: parser.Component{
				Name:        "UserRepository",
				PackageName: "repository",
			},
			Type:       analyzer.TypeRepository,
			Technology: "Database",
			Role:       analyzer.RoleCentral,
			Metrics:    &analyzer.ComponentMetrics{TotalDegree: 1},
		},
	}

	var buf bytes.Buffer
	if err := gen.Generate(components, &buf); err != nil {
		t.Fatalf("Generate() failed: %v", err)
	}

	var scene excalidrawFile
	if err := json.Unmarshal(buf.Bytes(), &scene); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}

	if scene.Type != "excalidraw" {
		t.Fatalf("scene type = %q, want excalidraw", scene.Type)
	}

	assertExcalidrawText(t, scene.Elements, "Test Diagram")
	assertExcalidrawText(t, scene.Elements, "service")
	assertExcalidrawText(t, scene.Elements, "repository")
	assertExcalidrawText(t, scene.Elements, "UserService\nSERVICE / Go")
	assertExcalidrawText(t, scene.Elements, "UserRepository\nREPOSITORY / Database")

	assertExcalidrawArrow(t, scene.Elements, "dependency-service-userservice-repository-userrepository", "solid")
}

func TestExcalidrawGenerator_InterfaceImplementations(t *testing.T) {
	cfg := config.New()
	gen := NewExcalidrawGenerator("Interface Test", cfg)

	components := []analyzer.AnalyzedComponent{
		{
			Component: parser.Component{Name: "ConcreteService", PackageName: "service"},
			Type:      analyzer.TypeService,
			Implements: []analyzer.InterfaceImplementation{
				{InterfaceName: "ServiceInterface", InterfacePackage: "service"},
			},
		},
		{
			Component:   parser.Component{Name: "ServiceInterface", PackageName: "service"},
			Type:        analyzer.TypeUnknown,
			IsInterface: true,
		},
	}

	var buf bytes.Buffer
	if err := gen.Generate(components, &buf); err != nil {
		t.Fatalf("Generate() failed: %v", err)
	}

	var scene excalidrawFile
	if err := json.Unmarshal(buf.Bytes(), &scene); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}

	assertExcalidrawText(t, scene.Elements, "ServiceInterface\ninterface / COMPONENT")
	assertExcalidrawArrow(t, scene.Elements, "implementation-service-concreteservice-service-serviceinterface", "dashed")
}

func assertExcalidrawText(t *testing.T, elements []excalidrawElement, text string) {
	t.Helper()

	for _, element := range elements {
		if element.Type == "text" && element.Text == text {
			return
		}
	}

	t.Fatalf("missing text element %q", text)
}

func assertExcalidrawArrow(t *testing.T, elements []excalidrawElement, id string, strokeStyle string) {
	t.Helper()

	for _, element := range elements {
		if element.ID == id && element.Type == "arrow" && element.StrokeStyle == strokeStyle {
			if element.EndArrowhead != "arrow" {
				t.Fatalf("arrow %q endArrowhead = %q, want arrow", id, element.EndArrowhead)
			}
			return
		}
	}

	t.Fatalf("missing %s arrow %q", strokeStyle, id)
}
