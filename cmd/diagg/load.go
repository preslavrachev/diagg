package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/preslavrachev/diagg/analyzer"
	"github.com/preslavrachev/diagg/parser"
)

// loadPackageGraph resolves targetDir, parses it with type information, and
// builds a package-level graph. Shared by the default `diagg` diagram command
// and the `check` subcommands.
func loadPackageGraph(targetDir string) (analyzer.PackageGraph, error) {
	absPath, err := filepath.Abs(targetDir)
	if err != nil {
		return analyzer.PackageGraph{}, fmt.Errorf("resolving path: %w", err)
	}

	info, err := os.Stat(absPath)
	if err != nil {
		return analyzer.PackageGraph{}, fmt.Errorf("accessing directory: %w", err)
	}
	if !info.IsDir() {
		return analyzer.PackageGraph{}, fmt.Errorf("%s is not a directory", absPath)
	}

	p := parser.NewParser()
	components, pkgTypeInfo, err := p.ParseDirectoryWithTypes(absPath)
	if err != nil {
		return analyzer.PackageGraph{}, fmt.Errorf("parsing directory: %w", err)
	}

	return analyzer.BuildPackageGraph(components, pkgTypeInfo), nil
}
