package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/preslavrachev/diagg/analyzer"
	"github.com/preslavrachev/diagg/config"
	"github.com/preslavrachev/diagg/generator"
	"github.com/preslavrachev/diagg/parser"
	"github.com/urfave/cli/v2"
)

func main() {
	app := &cli.App{
		Name:  "diagg",
		Usage: "Generate UML and C4 diagrams from Go code",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "output",
				Aliases: []string{"o"},
				Value:   "", // Will use config default if empty
				Usage:   "Output file path for the diagram",
			},
			&cli.StringFlag{
				Name:    "title",
				Aliases: []string{"t"},
				Value:   "",
				Usage:   "Diagram title",
			},
			&cli.StringFlag{
				Name:    "format",
				Aliases: []string{"f"},
				Value:   "plantuml",
				Usage:   "Output format: plantuml or d3",
			},
			&cli.BoolFlag{
				Name:    "package-links-only",
				Aliases: []string{"P"},
				Usage:   "Collapse component dependencies to package-level import links",
			},
			&cli.BoolFlag{
				Name:  "debug",
				Usage: "Print debug information about parsed packages and components",
			},
		},
		Action: runDiagg,
	}

	if err := app.Run(os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func runDiagg(c *cli.Context) error {
	// Get the target directory (default to current directory)
	targetDir := "."
	if c.NArg() > 0 {
		targetDir = c.Args().Get(0)
	}

	// Resolve to absolute path
	absPath, err := filepath.Abs(targetDir)
	if err != nil {
		return fmt.Errorf("resolving path: %w", err)
	}

	// Check if directory exists
	info, err := os.Stat(absPath)
	if err != nil {
		return fmt.Errorf("accessing directory: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("%s is not a directory", absPath)
	}

	fmt.Printf("Analyzing Go code in: %s\n", absPath)

	// Initialize configuration with sensible defaults
	cfg := config.New()

	// Step 1: Parse Go files with type information for interface detection
	p := parser.NewParser()
	components, pkgTypeInfo, err := p.ParseDirectoryWithTypes(absPath)
	if err != nil {
		return fmt.Errorf("parsing directory: %w", err)
	}

	if len(components) == 0 {
		return fmt.Errorf("no Go components found in %s", absPath)
	}

	fmt.Printf("Found %d components\n", len(components))
	if c.Bool("debug") {
		printIncludedEntities(components, pkgTypeInfo)
	}

	// Step 2: Analyze components with type information (enables interface detection)
	a := analyzer.NewAnalyzer(cfg)
	analyzed := a.AnalyzeWithTypes(components, pkgTypeInfo)

	// Count by type
	typeCounts := make(map[analyzer.ComponentType]int)
	for _, comp := range analyzed {
		typeCounts[comp.Type]++
	}

	fmt.Println("\nComponent breakdown:")
	for compType, count := range typeCounts {
		fmt.Printf("  %s: %d\n", compType, count)
	}

	// Step 3: Generate diagram
	format := c.String("format")
	outputPath := c.String("output")
	if outputPath == "" {
		// Set default output file based on format
		switch format {
		case "d3":
			outputPath = "diagram.html"
		default:
			outputPath = cfg.Defaults.OutputFile
		}
	}

	viewMode := generator.ViewModeComponent
	if c.Bool("package-links-only") {
		viewMode = generator.ViewModePackage
	}

	title := c.String("title")
	if title == "" {
		if viewMode == generator.ViewModePackage {
			title = fmt.Sprintf("Package Diagram - %s", filepath.Base(absPath))
		} else {
			title = fmt.Sprintf("Component Diagram - %s", filepath.Base(absPath))
		}
	}

	outFile, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("creating output file: %w", err)
	}
	defer outFile.Close()

	// Select generator based on format
	genOptions := []generator.Option{
		generator.WithViewMode(viewMode),
		generator.WithPackageImports(pkgTypeInfo.PackageImports),
		generator.WithPackageNamesByPath(packageNamesByPath(pkgTypeInfo)),
	}

	var gen generator.Generator
	switch format {
	case "d3":
		gen = generator.NewD3Generator(title, cfg, genOptions...)
	case "plantuml":
		gen = generator.NewPlantUMLGenerator(title, cfg, genOptions...)
	default:
		return fmt.Errorf("unknown format: %s (supported: plantuml, d3)", format)
	}

	if err := gen.Generate(analyzed, outFile); err != nil {
		return fmt.Errorf("generating diagram: %w", err)
	}

	fmt.Printf("\nDiagram written to: %s\n", outputPath)

	// Format-specific instructions
	switch format {
	case "plantuml":
		fmt.Println("\nTo render the diagram:")
		fmt.Printf("  plantuml %s\n", outputPath)
		fmt.Println("  OR visit: http://www.plantuml.com/plantuml/uml/")
	case "d3":
		fmt.Println("\nTo view the diagram:")
		fmt.Printf("  open %s\n", outputPath)
	}

	return nil
}

func printIncludedEntities(components []parser.Component, pkgTypeInfo *parser.PackageTypeInfo) {
	type packageSummary struct {
		Name  string
		Path  string
		Count int
	}

	packageCounts := make(map[string]*packageSummary)
	for _, comp := range components {
		key := comp.PackagePath + "::" + comp.PackageName
		if summary, ok := packageCounts[key]; ok {
			summary.Count++
			continue
		}

		packageCounts[key] = &packageSummary{
			Name:  comp.PackageName,
			Path:  comp.PackagePath,
			Count: 1,
		}
	}

	packages := make([]packageSummary, 0)
	if pkgTypeInfo != nil {
		for pkgPath, pkg := range pkgTypeInfo.LoadedPackagesByPath {
			count := 0
			key := pkgPath + "::" + pkg.Name
			if summary, ok := packageCounts[key]; ok {
				count = summary.Count
			}
			packages = append(packages, packageSummary{
				Name:  pkg.Name,
				Path:  pkgPath,
				Count: count,
			})
		}
	} else {
		for _, summary := range packageCounts {
			packages = append(packages, *summary)
		}
	}

	sort.Slice(packages, func(i, j int) bool {
		if packages[i].Path == packages[j].Path {
			return packages[i].Name < packages[j].Name
		}
		return packages[i].Path < packages[j].Path
	})

	fmt.Printf("\nDebug: Included packages (%d)\n", len(packages))
	for _, pkg := range packages {
		fmt.Printf("  - %s (%s) components=%d\n", pkg.Path, pkg.Name, pkg.Count)
	}

	sortedComponents := append([]parser.Component(nil), components...)
	sort.Slice(sortedComponents, func(i, j int) bool {
		if sortedComponents[i].PackagePath == sortedComponents[j].PackagePath {
			if sortedComponents[i].PackageName == sortedComponents[j].PackageName {
				return sortedComponents[i].Name < sortedComponents[j].Name
			}
			return sortedComponents[i].PackageName < sortedComponents[j].PackageName
		}
		return sortedComponents[i].PackagePath < sortedComponents[j].PackagePath
	})

	fmt.Printf("\nDebug: Included components (%d)\n", len(sortedComponents))
	for _, comp := range sortedComponents {
		fmt.Printf("  - %s.%s [%s] (%s)\n", comp.PackageName, comp.Name, comp.Kind, comp.PackagePath)
	}
	fmt.Println()
}

func packageNamesByPath(pkgTypeInfo *parser.PackageTypeInfo) map[string]string {
	if pkgTypeInfo == nil {
		return nil
	}

	names := make(map[string]string, len(pkgTypeInfo.LoadedPackagesByPath))
	for pkgPath, pkg := range pkgTypeInfo.LoadedPackagesByPath {
		names[pkgPath] = pkg.Name
	}
	return names
}
