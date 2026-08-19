package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/preslavrachev/diagg/analyzer"
	"github.com/preslavrachev/diagg/parser"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// AIDEV-NOTE: test-json-format; validates maybeWritePackageGraphJSON in isolation from CLI plumbing

// TestMaybeWritePackageGraphJSON_NoComponents ensures format "json" succeeds
// even when no structs/interfaces were found (only funcs), since package-level
// JSON only needs the import graph. This is the regression case for the old
// bug where runDiagg's "no components found" guard ran before the JSON branch.
func TestMaybeWritePackageGraphJSON_NoComponents(t *testing.T) {
	outputPath := filepath.Join(t.TempDir(), "packages.json")

	handled, err := maybeWritePackageGraphJSON(
		"json", true, outputPath,
		nil, // no components at all
		&parser.PackageTypeInfo{ModulePath: "example.com/nostructs"},
	)

	require.True(t, handled)
	require.NoError(t, err)

	data, err := os.ReadFile(outputPath)
	require.NoError(t, err)
	assert.Contains(t, string(data), `"module": "example.com/nostructs"`)
}

func TestMaybeWritePackageGraphJSON_NotJSONFormat(t *testing.T) {
	handled, err := maybeWritePackageGraphJSON("plantuml", true, "", nil, nil)
	assert.False(t, handled)
	assert.NoError(t, err)
}

func TestMaybeWritePackageGraphJSON_RequiresPackageLinksOnly(t *testing.T) {
	handled, err := maybeWritePackageGraphJSON("json", false, "", nil, nil)
	assert.True(t, handled)
	assert.ErrorContains(t, err, "--package-links-only")
}

func TestWritePackageGraphJSON(t *testing.T) {
	graph := analyzer.PackageGraph{
		ModulePath: "example.com/app",
		Packages: []analyzer.PackageNode{
			{Path: "example.com/app/web", Name: "web", RootLevel: true, Role: analyzer.RoleHub},
		},
	}

	var buf bytes.Buffer
	require.NoError(t, writePackageGraphJSON(graph, &buf))

	var decoded packageGraphJSON
	require.NoError(t, json.Unmarshal(buf.Bytes(), &decoded))
	assert.Equal(t, "example.com/app", decoded.Module)
	require.Len(t, decoded.Packages, 1)
	assert.Equal(t, "hub", decoded.Packages[0].Role)
	assert.Equal(t, []string{}, decoded.Packages[0].Imports, "nil imports should serialize as an empty array, not null")
}
