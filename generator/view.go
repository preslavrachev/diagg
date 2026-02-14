package generator

import (
	"fmt"

	"github.com/preslavrachev/diagg/analyzer"
)

// ViewMode controls diagram granularity.
type ViewMode string

const (
	ViewModeComponent ViewMode = "component"
	ViewModePackage   ViewMode = "package"
)

type generatorOptions struct {
	viewMode ViewMode
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

type viewNode struct {
	ID         string
	Name       string
	Package    string
	Type       string
	Role       analyzer.ComponentRole
	Metrics    *analyzer.ComponentMetrics
	IsMainLike bool
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
) viewGraph {
	if mode == ViewModePackage {
		return buildPackageViewGraph(components, packageFallback)
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
			ID:         comp.QualifiedName(),
			Name:       comp.Component.Name,
			Package:    pkgName,
			Type:       string(comp.Type),
			Role:       comp.Role,
			Metrics:    comp.Metrics,
			IsMainLike: comp.Type == analyzer.TypeEntrypoint,
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
) viewGraph {
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

	return graph
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
