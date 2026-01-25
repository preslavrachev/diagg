package analyzer

import (
	"path/filepath"
	"testing"

	"github.com/preslavrachev/diagg/config"
	"github.com/preslavrachev/diagg/parser"
)

// AIDEV-NOTE: test-cross-pkg-deps; validates cross-package dependency detection

// TestAnalyze_CrossPackageDependencies verifies that the analyzer correctly detects
// cross-package dependencies by ensuring component A is identified as depending on component B.
func TestAnalyze_CrossPackageDependencies(t *testing.T) {
	// Parse testdata: pkga.A depends on pkgb.B
	p := parser.NewParser()
	testdataPath, err := filepath.Abs("testdata")
	if err != nil {
		t.Fatalf("failed to get absolute path: %v", err)
	}

	components, err := p.ParseDirectory(testdataPath)
	if err != nil {
		t.Fatalf("ParseDirectory() failed: %v", err)
	}

	if len(components) < 2 {
		t.Fatalf("expected at least 2 components (A and B), got %d", len(components))
	}

	// Analyze
	cfg := config.New()
	analyzer := NewAnalyzer(cfg)
	analyzed := analyzer.Analyze(components)

	// Find component A
	var compA *AnalyzedComponent
	for i := range analyzed {
		if analyzed[i].Component.Name == "A" {
			compA = &analyzed[i]
			break
		}
	}

	if compA == nil {
		t.Fatal("component A not found")
	}

	// Verify A has dependency on B
	if len(compA.Dependencies) != 1 {
		t.Fatalf("A should have exactly 1 dependency, got %d", len(compA.Dependencies))
	}

	dep := compA.Dependencies[0]
	if dep.TargetName != "B" {
		t.Errorf("dependency target = %q, want %q", dep.TargetName, "B")
	}

	// Verify the dependency was detected across package boundaries
	// A is in pkga, B is in pkgb
	if compA.Component.PackageName == "pkga" {
		t.Logf("✓ Cross-package dependency detected: %s.A -> %s.B",
			compA.Component.PackageName, dep.TargetName)
	}
}

// TestAnalyze_InterfaceImplementation verifies that the analyzer can detect
// when a concrete type implements an interface defined in another package.
// Expected: B depends on Storage (interface), C implements Storage.
// In C4: B ──uses──▶ Storage (dotted line to C showing "implements Storage")
func TestAnalyze_InterfaceImplementation(t *testing.T) {
	// This test uses go/packages for full type information
	// 1. pkgb.B has field of type Storage (interface)
	// 2. pkgc.C implements Storage interface (implicitly)
	// 3. Analyzer should detect:
	//    - B depends on Storage interface
	//    - C implements Storage interface
	// 4. Generator should render:
	//    - Solid line: B -> Storage (uses)
	//    - Dotted line: C -> Storage (implements)

	p := parser.NewParser()
	testdataPath, err := filepath.Abs("testdata")
	if err != nil {
		t.Fatalf("failed to get absolute path: %v", err)
	}

	components, pkgTypeInfo, err := p.ParseDirectoryWithTypes(testdataPath)
	if err != nil {
		t.Fatalf("ParseDirectoryWithTypes() failed: %v", err)
	}

	if pkgTypeInfo == nil || pkgTypeInfo.TypeInfo == nil {
		t.Fatal("type info is nil")
	}

	// Analyze with type information
	cfg := config.New()
	analyzer := NewAnalyzer(cfg)
	analyzed := analyzer.AnalyzeWithTypes(components, pkgTypeInfo)

	// Build lookup map
	compMap := make(map[string]*AnalyzedComponent)
	for i := range analyzed {
		compMap[analyzed[i].Component.Name] = &analyzed[i]
	}

	t.Logf("Found %d components with type information", len(analyzed))

	// Verify B has dependency on Storage interface
	compB := compMap["B"]
	if compB == nil {
		t.Fatal("component B not found")
	}

	var storageDep *Dependency
	for i, dep := range compB.Dependencies {
		if dep.TargetName == "Storage" {
			storageDep = &compB.Dependencies[i]
			break
		}
	}

	if storageDep == nil {
		t.Error("B should have dependency on Storage interface")
	} else {
		t.Logf("✓ B depends on Storage interface")
		if !storageDep.IsInterface {
			t.Error("Storage dependency should be marked as interface")
		}
	}

	// Verify C implements Storage
	compC := compMap["C"]
	if compC == nil {
		t.Fatal("component C not found")
	}

	var implementsStorage bool
	for _, impl := range compC.Implements {
		if impl.InterfaceName == "Storage" {
			implementsStorage = true
			t.Logf("✓ C implements Storage interface")
			break
		}
	}

	if !implementsStorage {
		t.Error("C should be detected as implementing Storage interface")
	}
}

// TestAnalyze_MethodParameterDependencies verifies that the analyzer detects dependencies
// from method parameters and return types, not just struct fields.
// Example: If Generator has method Generate([]AnalyzedComponent), it depends on AnalyzedComponent.
func TestAnalyze_MethodParameterDependencies(t *testing.T) {
	const (
		genName      = "PlantUMLGenerator"
		analyzedName = "AnalyzedComponent"
	)

	p := parser.NewParser()
	projectRoot, err := filepath.Abs("..")
	if err != nil {
		t.Fatalf("failed to get project root: %v", err)
	}

	components, pkgTypeInfo, err := p.ParseDirectoryWithTypes(projectRoot)
	if err != nil {
		t.Fatalf("ParseDirectoryWithTypes() failed: %v", err)
	}

	cfg := config.New()
	analyzer := NewAnalyzer(cfg)
	analyzed := analyzer.AnalyzeWithTypes(components, pkgTypeInfo)

	// Build lookup map
	compMap := make(map[string]*AnalyzedComponent)
	for i := range analyzed {
		compMap[analyzed[i].Component.Name] = &analyzed[i]
	}

	// PlantUMLGenerator should exist
	gen := compMap[genName]
	if gen == nil {
		t.Fatalf("%s not found in analyzed components", genName)
	}

	// Debug: check if methods were parsed
	t.Logf("PlantUMLGenerator has %d methods", len(gen.Component.Methods))
	for _, method := range gen.Component.Methods {
		t.Logf("  Method: %s(params: %v) returns: %v", method.Name, method.Parameters, method.Returns)
	}

	// PlantUMLGenerator.Generate() takes []AnalyzedComponent as parameter
	// So it should have a dependency on AnalyzedComponent
	var foundDep bool
	for _, dep := range gen.Dependencies {
		if dep.TargetName == analyzedName {
			foundDep = true
			t.Logf("✓ %s depends on %s (via method parameter)", genName, analyzedName)
			break
		}
	}

	if !foundDep {
		t.Errorf("%s should have dependency on %s (method parameter), but none found.\nActual dependencies: %v",
			genName, analyzedName, gen.Dependencies)
	}
}
