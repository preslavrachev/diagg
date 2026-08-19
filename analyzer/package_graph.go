package analyzer

import (
	"path/filepath"
	"sort"
	"strings"

	"github.com/preslavrachev/diagg/parser"
	"golang.org/x/tools/go/packages"
)

// PackageNode describes one local Go package for structural checks (root-package
// budgets, generic-name detection, forbidden imports, etc.).
type PackageNode struct {
	Path        string   // Full import path
	Name        string   // Package name (identifier used in "package X")
	Dir         string   // Filesystem directory of the package
	RootLevel   bool     // True if the package sits directly under the module root
	Components  int      // Number of structs/interfaces found in the package
	Imports     []string // Import paths of local packages this package imports
	InDegree    int
	OutDegree   int
	TotalDegree int
	Role        ComponentRole
}

// PackageGraph is a package-level view of a Go module, suitable for JSON export
// and structural checks.
type PackageGraph struct {
	ModulePath string
	Packages   []PackageNode
}

// BuildPackageGraph assembles a PackageGraph from parsed components and the
// type/import information collected while loading the module.
// AIDEV-NOTE: package-graph; feeds --format json and the `check` subcommands
func BuildPackageGraph(components []parser.Component, pkgTypeInfo *parser.PackageTypeInfo) PackageGraph {
	graph := PackageGraph{}
	if pkgTypeInfo == nil {
		return graph
	}

	graph.ModulePath = pkgTypeInfo.ModulePath

	componentCounts := make(map[string]int, len(pkgTypeInfo.LoadedPackagesByPath))
	for _, comp := range components {
		componentCounts[comp.PackagePath]++
	}

	nodesByPath := make(map[string]*PackageNode, len(pkgTypeInfo.LoadedPackagesByPath))
	for pkgPath, pkg := range pkgTypeInfo.LoadedPackagesByPath {
		node := &PackageNode{
			Path:       pkgPath,
			Name:       pkg.Name,
			Dir:        packageDir(pkg),
			RootLevel:  isRootLevel(graph.ModulePath, pkgPath),
			Components: componentCounts[pkgPath],
		}
		nodesByPath[pkgPath] = node
	}

	for pkgPath, imports := range pkgTypeInfo.PackageImports {
		node, ok := nodesByPath[pkgPath]
		if !ok {
			continue
		}
		node.Imports = append([]string(nil), imports...)
		node.OutDegree = len(imports)
		for _, target := range imports {
			if targetNode, ok := nodesByPath[target]; ok {
				targetNode.InDegree++
			}
		}
	}

	nodes := make([]PackageNode, 0, len(nodesByPath))
	for _, node := range nodesByPath {
		node.TotalDegree = node.InDegree + node.OutDegree
		nodes = append(nodes, *node)
	}

	metrics := &ComponentMetrics{}
	for i := range nodes {
		metrics.InDegree = nodes[i].InDegree
		metrics.OutDegree = nodes[i].OutDegree
		metrics.TotalDegree = nodes[i].TotalDegree
		nodes[i].Role = ClassifyRole(metrics, len(nodes))
	}

	sort.Slice(nodes, func(i, j int) bool { return nodes[i].Path < nodes[j].Path })
	graph.Packages = nodes

	return graph
}

// packageDir returns the directory containing a loaded package's source files.
func packageDir(pkg *packages.Package) string {
	if pkg == nil || len(pkg.GoFiles) == 0 {
		return ""
	}
	return filepath.Dir(pkg.GoFiles[0])
}

// isRootLevel reports whether pkgPath sits directly under modulePath, i.e. has
// exactly one path segment beyond the module root (e.g. "module/foo", not
// "module/foo/bar").
func isRootLevel(modulePath, pkgPath string) bool {
	if modulePath == "" {
		return false
	}
	rel := strings.TrimPrefix(pkgPath, modulePath+"/")
	if rel == pkgPath {
		// pkgPath does not start with modulePath/ - either it's the module root
		// itself or an external package; neither counts as a root-level product
		// package.
		return false
	}
	return !strings.Contains(rel, "/")
}
