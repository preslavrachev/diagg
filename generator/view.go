package generator

import (
	"fmt"
	"maps"

	"github.com/preslavrachev/diagg/analyzer"
)

// ViewMode controls diagram granularity.
type ViewMode string

const (
	ViewModeComponent ViewMode = "component"
	ViewModePackage   ViewMode = "package"
)

type generatorOptions struct {
	viewMode          ViewMode
	packageImports    map[string][]string
	packageNameByPath map[string]string
}

func defaultOptions() generatorOptions {
	return generatorOptions{
		viewMode: ViewModeComponent,
	}
}

// Option configures generator behavior.
type Option func(*generatorOptions)

// WithViewMode sets diagram granularity.
func WithViewMode(mode ViewMode) Option {
	return func(opts *generatorOptions) {
		if mode == ViewModePackage {
			opts.viewMode = ViewModePackage
			return
		}
		opts.viewMode = ViewModeComponent
	}
}

// WithPackageImports provides package-level imports (source package path -> imported package paths).
func WithPackageImports(imports map[string][]string) Option {
	return func(opts *generatorOptions) {
		if imports == nil {
			return
		}

		copied := make(map[string][]string, len(imports))
		for source, targets := range imports {
			dst := make([]string, len(targets))
			copy(dst, targets)
			copied[source] = dst
		}
		opts.packageImports = copied
	}
}

// WithPackageNamesByPath provides display names for package paths.
func WithPackageNamesByPath(names map[string]string) Option {
	return func(opts *generatorOptions) {
		if names == nil {
			return
		}

		copied := make(map[string]string, len(names))
		maps.Copy(copied, names)
		opts.packageNameByPath = copied
	}
}

type viewNode struct {
	ID          string
	Name        string
	Package     string
	Type        string
	Technology  string
	Role        analyzer.ComponentRole
	Metrics     *analyzer.ComponentMetrics
	IsMainLike  bool
	IsInterface bool
}

type viewEdge struct {
	SourceID   string
	TargetID   string
	Type       string // dependency or implementation
	TargetType analyzer.ComponentType
}

type viewGraph struct {
	Nodes []viewNode
	Edges []viewEdge
}

func buildViewGraph(
	components []analyzer.AnalyzedComponent,
	mode ViewMode,
	packageFallback string,
	opts generatorOptions,
) viewGraph {
	if mode == ViewModePackage {
		return buildPackageViewGraph(components, packageFallback, opts)
	}
	return buildComponentViewGraph(components, packageFallback)
}

func buildComponentViewGraph(
	components []analyzer.AnalyzedComponent,
	packageFallback string,
) viewGraph {
	graph := viewGraph{
		Nodes: make([]viewNode, 0, len(components)),
		Edges: make([]viewEdge, 0),
	}

	for _, comp := range components {
		pkgName := normalizedPackageName(comp.Component.PackageName, packageFallback)
		graph.Nodes = append(graph.Nodes, viewNode{
			ID:          comp.QualifiedName(),
			Name:        comp.Component.Name,
			Package:     pkgName,
			Type:        string(comp.Type),
			Technology:  comp.Technology,
			Role:        comp.Role,
			Metrics:     comp.Metrics,
			IsMainLike:  comp.Type == analyzer.TypeEntrypoint,
			IsInterface: comp.IsInterface,
		})
	}

	for _, comp := range components {
		sourceID := comp.QualifiedName()
		for _, dep := range comp.Dependencies {
			graph.Edges = append(graph.Edges, viewEdge{
				SourceID:   sourceID,
				TargetID:   dep.QualifiedTarget(),
				Type:       "dependency",
				TargetType: dep.TargetType,
			})
		}

		for _, impl := range comp.Implements {
			graph.Edges = append(graph.Edges, viewEdge{
				SourceID: sourceID,
				TargetID: qualifiedName(impl.InterfacePackage, impl.InterfaceName),
				Type:     "implementation",
			})
		}
	}

	return graph
}

func buildPackageViewGraph(
	components []analyzer.AnalyzedComponent,
	packageFallback string,
	opts generatorOptions,
) viewGraph {
	if len(opts.packageImports) > 0 {
		graph := buildImportBasedPackageViewGraph(opts)
		annotatePackageMetrics(&graph)
		return graph
	}

	graph := viewGraph{
		Nodes: make([]viewNode, 0),
		Edges: make([]viewEdge, 0),
	}

	packages := make(map[string]bool)
	edges := make(map[string]bool)

	for _, comp := range components {
		sourcePkg := normalizedPackageName(comp.Component.PackageName, packageFallback)
		packages[sourcePkg] = true

		for _, dep := range comp.Dependencies {
			targetPkg := normalizedPackageName(dep.TargetPackage, packageFallback)
			packages[targetPkg] = true
			if sourcePkg == targetPkg {
				continue
			}

			key := fmt.Sprintf("%s->%s", sourcePkg, targetPkg)
			if edges[key] {
				continue
			}

			graph.Edges = append(graph.Edges, viewEdge{
				SourceID: sourcePkg,
				TargetID: targetPkg,
				Type:     "dependency",
			})
			edges[key] = true
		}
	}

	for pkgName := range packages {
		graph.Nodes = append(graph.Nodes, viewNode{
			ID:      pkgName,
			Name:    pkgName,
			Package: pkgName,
			Type:    "PACKAGE",
			Role:    analyzer.RoleOrdinary,
		})
	}

	annotatePackageMetrics(&graph)
	return graph
}

func buildImportBasedPackageViewGraph(opts generatorOptions) viewGraph {
	graph := viewGraph{
		Nodes: make([]viewNode, 0, len(opts.packageImports)),
		Edges: make([]viewEdge, 0),
	}

	packages := make(map[string]bool)
	edges := make(map[string]bool)

	for source, imports := range opts.packageImports {
		packages[source] = true
		for _, target := range imports {
			packages[target] = true
			if source == target {
				continue
			}

			key := fmt.Sprintf("%s->%s", source, target)
			if edges[key] {
				continue
			}

			graph.Edges = append(graph.Edges, viewEdge{
				SourceID: source,
				TargetID: target,
				Type:     "dependency",
			})
			edges[key] = true
		}
	}

	for pkgPath := range packages {
		graph.Nodes = append(graph.Nodes, viewNode{
			ID:      pkgPath,
			Name:    packageDisplayName(pkgPath, opts.packageNameByPath),
			Package: pkgPath,
			Type:    "PACKAGE",
			Role:    analyzer.RoleOrdinary,
		})
	}

	return graph
}

func annotatePackageMetrics(graph *viewGraph) {
	if graph == nil || len(graph.Nodes) == 0 {
		return
	}

	indexByID := make(map[string]int, len(graph.Nodes))
	for i, node := range graph.Nodes {
		indexByID[node.ID] = i
		graph.Nodes[i].Metrics = &analyzer.ComponentMetrics{}
	}

	for _, edge := range graph.Edges {
		if edge.Type != "dependency" {
			continue
		}

		srcIdx, srcOK := indexByID[edge.SourceID]
		dstIdx, dstOK := indexByID[edge.TargetID]
		if !srcOK || !dstOK {
			continue
		}

		graph.Nodes[srcIdx].Metrics.OutDegree++
		graph.Nodes[dstIdx].Metrics.InDegree++
	}

	for i := range graph.Nodes {
		m := graph.Nodes[i].Metrics
		m.TotalDegree = m.InDegree + m.OutDegree
		graph.Nodes[i].Role = analyzer.ClassifyRole(m, len(graph.Nodes))
	}
}

func normalizedPackageName(pkgName, fallback string) string {
	if pkgName == "" {
		return fallback
	}
	return pkgName
}

func qualifiedName(pkg, name string) string {
	if pkg == "" {
		return name
	}
	return pkg + "." + name
}

func packageDisplayName(path string, names map[string]string) string {
	if names == nil {
		return path
	}

	name, ok := names[path]
	if !ok || name == "" {
		return path
	}
	return path + " (" + name + ")"
}
