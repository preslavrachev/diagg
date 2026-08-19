package analyzer

import (
	"testing"

	"github.com/preslavrachev/diagg/parser"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/tools/go/packages"
)

// TestBuildPackageGraph verifies that BuildPackageGraph correctly derives
// root-level status, component counts, and in/out degree for a synthetic
// module containing both root packages (web, model) and a nested package
// (phase/planning) reached through an intermediate root package (phase).
func TestBuildPackageGraph(t *testing.T) {
	const module = "example.com/app"

	pkgTypeInfo := &parser.PackageTypeInfo{
		ModulePath: module,
		LoadedPackagesByPath: map[string]*packages.Package{
			module + "/web": {
				Name:    "web",
				PkgPath: module + "/web",
				GoFiles: []string{"/repo/web/handler.go"},
			},
			module + "/phase": {
				Name:    "phase",
				PkgPath: module + "/phase",
				GoFiles: []string{"/repo/phase/phase.go"},
			},
			module + "/phase/planning": {
				Name:    "planning",
				PkgPath: module + "/phase/planning",
				GoFiles: []string{"/repo/phase/planning/planning.go"},
			},
			module + "/model": {
				Name:    "model",
				PkgPath: module + "/model",
				GoFiles: []string{"/repo/model/model.go"},
			},
		},
		PackageImports: map[string][]string{
			module + "/web":            {module + "/phase"},
			module + "/phase":          {module + "/phase/planning"},
			module + "/phase/planning": {},
			module + "/model":          {},
		},
	}

	components := []parser.Component{
		{Name: "Handler", PackagePath: module + "/web"},
		{Name: "Phase", PackagePath: module + "/phase"},
		{Name: "Plan", PackagePath: module + "/phase/planning"},
		{Name: "Step", PackagePath: module + "/phase/planning"},
	}

	graph := BuildPackageGraph(components, pkgTypeInfo)

	assert.Equal(t, module, graph.ModulePath)
	require.Len(t, graph.Packages, 4)

	byPath := make(map[string]PackageNode, len(graph.Packages))
	for _, node := range graph.Packages {
		byPath[node.Path] = node
	}

	web := byPath[module+"/web"]
	assert.True(t, web.RootLevel, "web should be root-level")
	assert.Equal(t, 1, web.Components)
	assert.Equal(t, []string{module + "/phase"}, web.Imports)
	assert.Equal(t, 0, web.InDegree)
	assert.Equal(t, 1, web.OutDegree)

	planning := byPath[module+"/phase/planning"]
	assert.False(t, planning.RootLevel, "phase/planning should not be root-level")
	assert.Equal(t, 2, planning.Components)
	assert.Equal(t, 1, planning.InDegree)
	assert.Equal(t, 0, planning.OutDegree)

	model := byPath[module+"/model"]
	assert.True(t, model.RootLevel)
	assert.Equal(t, 0, model.TotalDegree)
}

func TestBuildPackageGraph_NilTypeInfo(t *testing.T) {
	graph := BuildPackageGraph(nil, nil)
	assert.Empty(t, graph.ModulePath)
	assert.Empty(t, graph.Packages)
}

func TestIsRootLevel(t *testing.T) {
	tests := []struct {
		name       string
		modulePath string
		pkgPath    string
		want       bool
	}{
		{"root package", "example.com/app", "example.com/app/web", true},
		{"nested package", "example.com/app", "example.com/app/phase/planning", false},
		{"module root itself", "example.com/app", "example.com/app", false},
		{"empty module path", "", "example.com/app/web", false},
		{"external package", "example.com/app", "other.com/lib", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, isRootLevel(tt.modulePath, tt.pkgPath))
		})
	}
}
