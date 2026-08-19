package main

import (
	"testing"

	"github.com/preslavrachev/diagg/analyzer"
	"github.com/stretchr/testify/assert"
)

// AIDEV-NOTE: test-check-commands; validates root-budget and generic-names filtering logic

func TestFilterRootBudget(t *testing.T) {
	const module = "example.com/app"
	graph := analyzer.PackageGraph{
		ModulePath: module,
		Packages: []analyzer.PackageNode{
			{Path: module + "/canvas", RootLevel: true},
			{Path: module + "/organization", RootLevel: true},
			{Path: module + "/web", RootLevel: true},
			{Path: module + "/phase/planning", RootLevel: false},
		},
	}

	ignore := toSet([]string{"web", "cmd"})
	counted, ignored := filterRootBudget(graph, ignore)

	assert.Equal(t, []string{module + "/canvas", module + "/organization"}, pathsOf(counted))
	assert.Equal(t, []string{module + "/web"}, pathsOf(ignored))
}

func TestFilterGenericNames(t *testing.T) {
	graph := analyzer.PackageGraph{
		Packages: []analyzer.PackageNode{
			{Path: "example.com/app/model", Name: "model"},
			{Path: "example.com/app/canvas", Name: "canvas"},
			{Path: "example.com/app/utils", Name: "utils"},
		},
	}

	flagged := filterGenericNames(graph, toSet(defaultGenericNames))

	assert.Equal(t, []string{"example.com/app/model", "example.com/app/utils"}, pathsOf(flagged))
}

// TestFilterGenericNames_DirectorySegment ensures a generic directory name is
// still flagged when the declared package identifier differs from it, e.g. a
// "model" directory containing "package datamodel".
func TestFilterGenericNames_DirectorySegment(t *testing.T) {
	graph := analyzer.PackageGraph{
		Packages: []analyzer.PackageNode{
			{Path: "example.com/app/model", Name: "datamodel"},
			{Path: "example.com/app/canvas", Name: "canvas"},
		},
	}

	flagged := filterGenericNames(graph, toSet([]string{"model"}))

	assert.Equal(t, []string{"example.com/app/model"}, pathsOf(flagged))
}

func TestSplitCSV(t *testing.T) {
	assert.Equal(t, []string{"a", "b", "c"}, splitCSV("a, b ,c"))
	assert.Empty(t, splitCSV(""))
}

func TestRootSegmentName(t *testing.T) {
	assert.Equal(t, "web", rootSegmentName("example.com/app", "example.com/app/web"))
}

func pathsOf(pkgs []analyzer.PackageNode) []string {
	paths := make([]string, len(pkgs))
	for i, pkg := range pkgs {
		paths[i] = pkg.Path
	}
	return paths
}
