package main

import (
	"fmt"

	"github.com/preslavrachev/diagg/generator"
)

// diagramPlan captures the decisions runDiagg needs before it invokes a
// generator: which view mode to render, whether output goes to stdout, and
// where a file would be written otherwise.
type diagramPlan struct {
	ViewMode   generator.ViewMode
	ToStdout   bool
	OutputPath string
}

// planDiagram derives a diagramPlan from CLI inputs and the parsed component
// count, with no *cli.Context and no I/O, so it can be unit tested directly.
//
// AIDEV-NOTE: package-view-no-components; package view is built entirely from
// pkgTypeInfo.PackageImports (see generator.buildPackageViewGraph), not parsed
// structs/interfaces, so a struct-free module (funcs/constants only) must still
// be able to render/export it. Only component view genuinely needs components.
func planDiagram(format string, packageLinksOnly bool, outputFlag string, componentCount int, defaultOutputFile string) (diagramPlan, error) {
	viewMode := generator.ViewModeComponent
	if packageLinksOnly {
		viewMode = generator.ViewModePackage
	}

	if componentCount == 0 && viewMode == generator.ViewModeComponent {
		return diagramPlan{}, fmt.Errorf("no Go components found")
	}

	// AIDEV-NOTE: json-stdout; json defaults to stdout when --output is unset, matching
	// the convention of CLI tools that emit machine-readable output (jq, kubectl -o json, ...).
	toStdout := format == "json" && outputFlag == ""

	outputPath := outputFlag
	if outputPath == "" && !toStdout {
		switch format {
		case "d3":
			outputPath = "diagram.html"
		case "excalidraw":
			outputPath = "diagram.excalidraw"
		default:
			outputPath = defaultOutputFile
		}
	}

	return diagramPlan{
		ViewMode:   viewMode,
		ToStdout:   toStdout,
		OutputPath: outputPath,
	}, nil
}
