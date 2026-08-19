package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/preslavrachev/diagg/analyzer"
	"github.com/preslavrachev/diagg/parser"
)

// maybeWritePackageGraphJSON writes the package graph as JSON when format is
// "json", independent of whether any components (structs/interfaces) were
// found. Package-level JSON only needs the import graph, so this must run
// before any "no components found" guard - callers must check handled before
// applying such a guard.
func maybeWritePackageGraphJSON(
	format string,
	packageLinksOnly bool,
	outputPath string,
	components []parser.Component,
	pkgTypeInfo *parser.PackageTypeInfo,
) (handled bool, err error) {
	if format != "json" {
		return false, nil
	}

	if !packageLinksOnly {
		return true, fmt.Errorf("--format json requires --package-links-only (-P)")
	}

	if outputPath == "" {
		outputPath = "packages.json"
	}

	outFile, err := os.Create(outputPath)
	if err != nil {
		return true, fmt.Errorf("creating output file: %w", err)
	}
	defer outFile.Close()

	graph := analyzer.BuildPackageGraph(components, pkgTypeInfo)
	if err := writePackageGraphJSON(graph, outFile); err != nil {
		return true, fmt.Errorf("writing package graph JSON: %w", err)
	}

	fmt.Printf("\nPackage graph written to: %s\n", outputPath)
	return true, nil
}

// packageGraphJSON is the machine-readable shape written for `--format json`.
type packageGraphJSON struct {
	Module   string            `json:"module"`
	Packages []packageNodeJSON `json:"packages"`
}

type packageNodeJSON struct {
	Path        string   `json:"path"`
	Name        string   `json:"name"`
	Dir         string   `json:"dir"`
	RootLevel   bool     `json:"root_level"`
	Components  int      `json:"components"`
	Imports     []string `json:"imports"`
	InDegree    int      `json:"in_degree"`
	OutDegree   int      `json:"out_degree"`
	TotalDegree int      `json:"total_degree"`
	Role        string   `json:"role"`
}

func writePackageGraphJSON(graph analyzer.PackageGraph, w io.Writer) error {
	doc := packageGraphJSON{
		Module:   graph.ModulePath,
		Packages: make([]packageNodeJSON, 0, len(graph.Packages)),
	}

	for _, node := range graph.Packages {
		imports := node.Imports
		if imports == nil {
			imports = []string{}
		}
		doc.Packages = append(doc.Packages, packageNodeJSON{
			Path:        node.Path,
			Name:        node.Name,
			Dir:         node.Dir,
			RootLevel:   node.RootLevel,
			Components:  node.Components,
			Imports:     imports,
			InDegree:    node.InDegree,
			OutDegree:   node.OutDegree,
			TotalDegree: node.TotalDegree,
			Role:        string(node.Role),
		})
	}

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(doc)
}
