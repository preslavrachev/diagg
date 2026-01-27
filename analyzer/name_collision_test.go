package analyzer

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/preslavrachev/diagg/config"
	"github.com/preslavrachev/diagg/parser"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// AIDEV-NOTE: name-collision-tests; validates same-named types across packages are distinguished

func TestSameNamedTypes_BothRetainedInAnalysis(t *testing.T) {
	files := twoHandlerPackages()
	files["consumer/consumer.go"] = `package consumer
import (
	"test/auth"
	"test/billing"
)
type AuthConsumer struct {
	h *auth.Handler
}
type BillingConsumer struct {
	h *billing.Handler
}`

	dir := createCollisionTestdata(t, files)
	analyzed := analyzeCollisionTestdata(t, dir)

	authConsumer := findByNameAndPackage(analyzed, "AuthConsumer", "consumer")
	billingConsumer := findByNameAndPackage(analyzed, "BillingConsumer", "consumer")
	require.NotNil(t, authConsumer, "AuthConsumer should exist")
	require.NotNil(t, billingConsumer, "BillingConsumer should exist")

	assert.Len(t, authConsumer.Dependencies, 1, "AuthConsumer should have 1 dependency")
	assert.Len(t, billingConsumer.Dependencies, 1, "BillingConsumer should have 1 dependency")

	// Both auth.Handler and billing.Handler must survive the componentMap — no overwrites.
	var handlerCount int
	for _, c := range analyzed {
		if c.Component.Name == "Handler" {
			handlerCount++
		}
	}
	assert.Equal(t, 2, handlerCount, "both auth.Handler and billing.Handler must be in the output")
}

func TestSameNamedTypes_DependencyResolvesToImportedPackage(t *testing.T) {
	files := twoHandlerPackages()
	files["app/server.go"] = `package app
import "test/auth"
type Server struct {
	auth *auth.Handler
}`

	dir := createCollisionTestdata(t, files)
	analyzed := analyzeCollisionTestdata(t, dir)

	server := findByNameAndPackage(analyzed, "Server", "app")
	require.NotNil(t, server, "Server should exist")
	require.Len(t, server.Dependencies, 1, "Server should have exactly 1 dependency")

	dep := server.Dependencies[0]
	assert.Equal(t, "Handler", dep.TargetName)
	assert.Equal(t, "auth", dep.TargetPackage,
		"dependency should resolve to auth.Handler, not billing.Handler")
}

func TestSameNamedTypes_QualifiedNamesAreUnique(t *testing.T) {
	files := twoHandlerPackages()
	files["consumer/consumer.go"] = `package consumer
import (
	"test/auth"
	"test/billing"
)
type AuthConsumer struct {
	h *auth.Handler
}
type BillingConsumer struct {
	h *billing.Handler
}`

	dir := createCollisionTestdata(t, files)
	analyzed := analyzeCollisionTestdata(t, dir)

	seen := make(map[string]bool)
	for _, comp := range analyzed {
		qn := comp.QualifiedName()
		assert.False(t, seen[qn], "duplicate qualified name %q — would produce duplicate IDs downstream", qn)
		seen[qn] = true
	}
}

// createCollisionTestdata writes Go source files to a temp directory and returns its path.
func createCollisionTestdata(t *testing.T, files map[string]string) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "diagg-collision-test-*")
	require.NoError(t, err)
	t.Cleanup(func() { os.RemoveAll(dir) })

	for path, content := range files {
		fullPath := filepath.Join(dir, path)
		require.NoError(t, os.MkdirAll(filepath.Dir(fullPath), 0o755))
		require.NoError(t, os.WriteFile(fullPath, []byte(content), 0o644))
	}
	return dir
}

// analyzeCollisionTestdata parses and analyzes a temp directory with full type info.
func analyzeCollisionTestdata(t *testing.T, dir string) []AnalyzedComponent {
	t.Helper()
	p := parser.NewParser()
	components, pkgTypeInfo, err := p.ParseDirectoryWithTypes(dir)
	require.NoError(t, err)

	a := NewAnalyzer(config.New())
	return a.AnalyzeWithTypes(components, pkgTypeInfo)
}

// findByNameAndPackage returns the first component matching name+package, or nil.
func findByNameAndPackage(analyzed []AnalyzedComponent, name, pkg string) *AnalyzedComponent {
	for i := range analyzed {
		if analyzed[i].Component.Name == name && analyzed[i].Component.PackageName == pkg {
			return &analyzed[i]
		}
	}
	return nil
}

// twoHandlerPackages is the shared testdata for tests that need auth.Handler and billing.Handler.
func twoHandlerPackages() map[string]string {
	return map[string]string{
		"auth/handler.go": `package auth
type Handler struct{}
func (h *Handler) Authenticate() {}`,
		"billing/handler.go": `package billing
type Handler struct{}
func (h *Handler) Charge() {}`,
		"go.mod": `module test
go 1.25`,
	}
}
