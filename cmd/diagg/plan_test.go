package main

import (
	"testing"

	"github.com/preslavrachev/diagg/generator"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// AIDEV-NOTE: test-plan; validates planDiagram, the pure decision function runDiagg
// delegates to for view mode, stdout-vs-file, and the components-required guard

func TestPlanDiagram_PackageViewJSON_NoComponents(t *testing.T) {
	// Regression: `diagg -P --format json` on a struct-free module (funcs/consts
	// only) must succeed, since package view is built from PackageImports, not
	// parsed structs/interfaces.
	plan, err := planDiagram("json", true, "", 0, "diagram.puml")

	require.NoError(t, err)
	assert.Equal(t, generator.ViewModePackage, plan.ViewMode)
	assert.True(t, plan.ToStdout)
	assert.Empty(t, plan.OutputPath)
}

func TestPlanDiagram_ComponentView_NoComponents(t *testing.T) {
	_, err := planDiagram("json", false, "", 0, "diagram.puml")
	assert.ErrorContains(t, err, "no Go components found")
}

func TestPlanDiagram_JSONWithOutputFlag_WritesToFile(t *testing.T) {
	plan, err := planDiagram("json", true, "out.json", 0, "diagram.puml")

	require.NoError(t, err)
	assert.False(t, plan.ToStdout)
	assert.Equal(t, "out.json", plan.OutputPath)
}

func TestPlanDiagram_NonJSONFormat_NeverGoesToStdout(t *testing.T) {
	plan, err := planDiagram("plantuml", true, "", 3, "diagram.puml")

	require.NoError(t, err)
	assert.False(t, plan.ToStdout)
	assert.Equal(t, "diagram.puml", plan.OutputPath)
}

func TestPlanDiagram_DefaultOutputPaths(t *testing.T) {
	tests := []struct {
		format string
		want   string
	}{
		{"plantuml", "diagram.puml"},
		{"d3", "diagram.html"},
		{"excalidraw", "diagram.excalidraw"},
	}

	for _, tt := range tests {
		t.Run(tt.format, func(t *testing.T) {
			plan, err := planDiagram(tt.format, false, "", 1, "diagram.puml")
			require.NoError(t, err)
			assert.Equal(t, tt.want, plan.OutputPath)
		})
	}
}
