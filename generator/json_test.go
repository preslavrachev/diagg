package generator

import (
	"bytes"
	"encoding/json"
	"sort"
	"testing"

	"github.com/preslavrachev/diagg/analyzer"
	"github.com/preslavrachev/diagg/config"
	"github.com/preslavrachev/diagg/parser"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// AIDEV-NOTE: test-json-generator; validates JSONGenerator has the same component/package
// view-mode capability as PlantUMLGenerator/D3Generator/ExcalidrawGenerator

func TestJSONGenerator_ComponentView(t *testing.T) {
	cfg := config.New()
	gen := NewJSONGenerator("Test Diagram", cfg)

	components := []analyzer.AnalyzedComponent{
		{
			Component: parser.Component{Name: "UserService", PackageName: "service"},
			Type:      analyzer.TypeService,
			Role:      analyzer.RoleHub,
			Dependencies: []analyzer.Dependency{
				{TargetName: "UserRepository", TargetPackage: "repository", TargetType: analyzer.TypeRepository},
			},
			Metrics: &analyzer.ComponentMetrics{InDegree: 2, OutDegree: 1, TotalDegree: 3},
		},
		{
			Component: parser.Component{Name: "UserRepository", PackageName: "repository"},
			Type:      analyzer.TypeRepository,
			Role:      analyzer.RoleCentral,
			Metrics:   &analyzer.ComponentMetrics{InDegree: 1, OutDegree: 0, TotalDegree: 1},
		},
	}

	var buf bytes.Buffer
	require.NoError(t, gen.Generate(components, &buf))

	var doc jsonGraphDoc
	require.NoError(t, json.Unmarshal(buf.Bytes(), &doc))

	assert.Equal(t, "Test Diagram", doc.Title)
	assert.Equal(t, "component", doc.ViewMode)
	require.Len(t, doc.Nodes, 2)
	require.Len(t, doc.Edges, 1)

	assert.Equal(t, "service.UserService", doc.Edges[0].Source)
	assert.Equal(t, "repository.UserRepository", doc.Edges[0].Target)
	assert.Equal(t, "dependency", doc.Edges[0].Type)

	var hub jsonGraphNode
	for _, n := range doc.Nodes {
		if n.ID == "service.UserService" {
			hub = n
		}
	}
	assert.Equal(t, "hub", hub.Role)
	assert.Equal(t, 3, hub.TotalDegree)
}

// TestJSONGenerator_PackageView ensures -P (package-links-only) works for JSON
// exactly like it does for the other renderers, via WithViewMode.
func TestJSONGenerator_PackageView(t *testing.T) {
	cfg := config.New()
	gen := NewJSONGenerator("Test Diagram", cfg,
		WithViewMode(ViewModePackage),
		WithPackageImports(map[string][]string{
			"example.com/app/web":     {"example.com/app/service"},
			"example.com/app/service": {},
		}),
	)

	var buf bytes.Buffer
	require.NoError(t, gen.Generate(nil, &buf))

	var doc jsonGraphDoc
	require.NoError(t, json.Unmarshal(buf.Bytes(), &doc))

	assert.Equal(t, "package", doc.ViewMode)
	require.Len(t, doc.Nodes, 2)
	require.Len(t, doc.Edges, 1)
	assert.Equal(t, "example.com/app/web", doc.Edges[0].Source)
	assert.Equal(t, "example.com/app/service", doc.Edges[0].Target)
}

// TestJSONGenerator_DeterministicOrder ensures nodes/edges are emitted in a
// stable order even though the package view is built from Go maps
// (buildImportBasedPackageViewGraph), so CI diffs/snapshots of `--format
// json` output stay quiet across runs on an unchanged codebase.
func TestJSONGenerator_DeterministicOrder(t *testing.T) {
	cfg := config.New()
	gen := NewJSONGenerator("Test Diagram", cfg,
		WithViewMode(ViewModePackage),
		WithPackageImports(map[string][]string{
			"example.com/app/web":     {"example.com/app/service", "example.com/app/model"},
			"example.com/app/service": {"example.com/app/model", "example.com/app/cache"},
			"example.com/app/model":   {},
			"example.com/app/cache":   {},
			"example.com/app/auth":    {"example.com/app/model"},
		}),
	)

	var buf bytes.Buffer
	require.NoError(t, gen.Generate(nil, &buf))

	var doc jsonGraphDoc
	require.NoError(t, json.Unmarshal(buf.Bytes(), &doc))

	nodeIDs := make([]string, len(doc.Nodes))
	for i, n := range doc.Nodes {
		nodeIDs[i] = n.ID
	}
	assert.True(t, sort.StringsAreSorted(nodeIDs), "nodes must be sorted by id, got %v", nodeIDs)

	for i := 1; i < len(doc.Edges); i++ {
		prev, cur := doc.Edges[i-1], doc.Edges[i]
		less := prev.Source < cur.Source ||
			(prev.Source == cur.Source && prev.Target < cur.Target) ||
			(prev.Source == cur.Source && prev.Target == cur.Target && prev.Type <= cur.Type)
		assert.True(t, less, "edges must be sorted by (source, target, type); %+v before %+v", prev, cur)
	}
}
