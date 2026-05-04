package main

import (
	"fmt"
	"os"
	"path/filepath"

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
				Usage:   "Output format: plantuml, d3, or excalidraw",
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
		case "excalidraw":
			outputPath = "diagram.excalidraw"
		default:
			outputPath = cfg.Defaults.OutputFile
		}
	}

	title := c.String("title")
	if title == "" {
		title = fmt.Sprintf("Component Diagram - %s", filepath.Base(absPath))
	}

	outFile, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("creating output file: %w", err)
	}
	defer outFile.Close()

	// Select generator based on format
	var gen generator.Generator
	switch format {
	case "d3":
		gen = generator.NewD3Generator(title, cfg)
	case "excalidraw":
		gen = generator.NewExcalidrawGenerator(title, cfg)
	case "plantuml":
		gen = generator.NewPlantUMLGenerator(title, cfg)
	default:
		return fmt.Errorf("unknown format: %s (supported: plantuml, d3, excalidraw)", format)
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
	case "excalidraw":
		fmt.Println("\nTo view the diagram:")
		fmt.Println("  Import the .excalidraw file at https://excalidraw.com/")
	}

	return nil
}
