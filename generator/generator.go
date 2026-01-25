package generator

import (
	"io"

	"github.com/preslavrachev/diagg/analyzer"
)

// Generator defines the interface for diagram generators.
// All generators must implement this interface to convert analyzed components into visual diagrams.
type Generator interface {
	// Generate writes the diagram to the provided writer.
	// Returns an error if generation fails.
	Generate(components []analyzer.AnalyzedComponent, w io.Writer) error
}
