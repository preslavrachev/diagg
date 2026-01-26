package parser

import (
	"os"
	"path/filepath"
	"testing"
)

const (
	testModulePath = "github.com/testorg/testapp"
)

// TestExtractMainComponent_MultipleEntrypoints verifies that multiple main packages
// are distinguished by their package path, not collapsed into a single "Main" component.
// AIDEV-NOTE: test-multiple-mains; critical for multi-binary Go applications
func TestExtractMainComponent_MultipleEntrypoints(t *testing.T) {
	testCases := []struct {
		name            string
		setupProject    func(t *testing.T, root string)
		wantEntrypoints map[string][]string // component name -> expected dependencies
	}{
		{
			name: "single main in cmd/diagg",
			setupProject: func(t *testing.T, root string) {
				createGoMod(t, root, testModulePath)
				createMainPackage(t, root, "cmd/diagg", []string{
					"github.com/testorg/testapp/parser",
					"github.com/testorg/testapp/analyzer",
				})
				createDummyPackage(t, root, "parser", "Parser")
				createDummyPackage(t, root, "analyzer", "Analyzer")
			},
			wantEntrypoints: map[string][]string{
				"diagg": {"Parser", "Analyzer"},
			},
		},
		{
			name: "multiple mains: server and worker",
			setupProject: func(t *testing.T, root string) {
				createGoMod(t, root, testModulePath)

				// Server uses parser and config
				createMainPackage(t, root, "cmd/server", []string{
					"github.com/testorg/testapp/parser",
					"github.com/testorg/testapp/config",
				})

				// Worker uses analyzer and config
				createMainPackage(t, root, "cmd/worker", []string{
					"github.com/testorg/testapp/analyzer",
					"github.com/testorg/testapp/config",
				})

				createDummyPackage(t, root, "parser", "Parser")
				createDummyPackage(t, root, "analyzer", "Analyzer")
				createDummyPackage(t, root, "config", "Config")
			},
			wantEntrypoints: map[string][]string{
				"server": {"Parser", "Config"},
				"worker": {"Analyzer", "Config"},
			},
		},
		{
			name: "three binaries with different dependencies",
			setupProject: func(t *testing.T, root string) {
				createGoMod(t, root, testModulePath)

				createMainPackage(t, root, "cmd/api", []string{
					"github.com/testorg/testapp/handler",
				})

				createMainPackage(t, root, "cmd/worker", []string{
					"github.com/testorg/testapp/processor",
				})

				createMainPackage(t, root, "cmd/migrator", []string{
					"github.com/testorg/testapp/db",
				})

				createDummyPackage(t, root, "handler", "Handler")
				createDummyPackage(t, root, "processor", "Processor")
				createDummyPackage(t, root, "db", "DB")
			},
			wantEntrypoints: map[string][]string{
				"api":      {"Handler"},
				"worker":   {"Processor"},
				"migrator": {"DB"},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tmpDir := t.TempDir()

			tc.setupProject(t, tmpDir)

			parser := NewParser()
			components, _, err := parser.ParseDirectoryWithTypes(tmpDir)
			if err != nil {
				t.Fatalf("ParseDirectoryWithTypes failed: %v", err)
			}

			// Extract entrypoint components
			entrypoints := make(map[string]*Component)
			for i := range components {
				comp := &components[i]
				if comp.Kind == "entrypoint" {
					entrypoints[comp.Name] = comp
				}
			}

			// Verify we found the expected number of entrypoints
			if len(entrypoints) != len(tc.wantEntrypoints) {
				t.Errorf("got %d entrypoints, want %d", len(entrypoints), len(tc.wantEntrypoints))
				t.Logf("found entrypoints: %v", mapKeys(entrypoints))
			}

			// Verify each entrypoint has correct name and dependencies
			for wantName, wantDeps := range tc.wantEntrypoints {
				comp, found := entrypoints[wantName]
				if !found {
					t.Errorf("missing entrypoint %q", wantName)
					continue
				}

				// Check component name
				if comp.Name != wantName {
					t.Errorf("entrypoint name: got %q, want %q", comp.Name, wantName)
				}

				// Check package name is "main"
				if comp.PackageName != "main" {
					t.Errorf("%s: package name: got %q, want %q", wantName, comp.PackageName, "main")
				}

				// Extract dependency names from Fields (abusing Fields to store deps)
				gotDeps := make([]string, len(comp.Fields))
				for i, field := range comp.Fields {
					gotDeps[i] = field.TypeName
				}

				// Verify dependencies
				if !stringSlicesEqual(gotDeps, wantDeps) {
					t.Errorf("%s dependencies: got %v, want %v", wantName, gotDeps, wantDeps)
				}
			}
		})
	}
}

// TestExtractMainComponent_FallbackNaming tests edge cases where package path
// doesn't follow cmd/name convention.
func TestExtractMainComponent_FallbackNaming(t *testing.T) {
	testCases := []struct {
		name         string
		pkgPath      string
		wantCompName string
	}{
		{
			name:         "standard cmd/server path",
			pkgPath:      "github.com/user/project/cmd/server",
			wantCompName: "server",
		},
		{
			name:         "nested cmd/tools/migrator",
			pkgPath:      "github.com/user/project/cmd/tools/migrator",
			wantCompName: "migrator",
		},
		{
			name:         "just main package at root",
			pkgPath:      "main",
			wantCompName: "Main", // fallback
		},
		{
			name:         "empty path",
			pkgPath:      ".",
			wantCompName: "Main", // fallback
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			createGoMod(t, tmpDir, "github.com/testuser/testproject")

			// Create a main.go that simulates the package path
			var mainDir string
			if tc.pkgPath == "main" || tc.pkgPath == "." {
				mainDir = tmpDir
			} else {
				// Extract path after module
				relPath := filepath.Base(tc.pkgPath)
				mainDir = filepath.Join(tmpDir, "cmd", relPath)
			}

			if err := os.MkdirAll(mainDir, 0755); err != nil {
				t.Fatalf("create main dir: %v", err)
			}

			mainGo := `package main

func main() {}
`
			if err := os.WriteFile(filepath.Join(mainDir, "main.go"), []byte(mainGo), 0644); err != nil {
				t.Fatalf("write main.go: %v", err)
			}

			parser := NewParser()
			components, _, err := parser.ParseDirectoryWithTypes(tmpDir)
			if err != nil {
				t.Fatalf("ParseDirectoryWithTypes failed: %v", err)
			}

			// Find the entrypoint
			var found *Component
			for i := range components {
				if components[i].Kind == "entrypoint" {
					found = &components[i]
					break
				}
			}

			if found == nil {
				t.Fatal("no entrypoint component found")
			}

			if found.Name != tc.wantCompName {
				t.Errorf("component name: got %q, want %q", found.Name, tc.wantCompName)
			}
		})
	}
}

// Helper functions

func createGoMod(t *testing.T, root, modulePath string) {
	t.Helper()
	content := "module " + modulePath + "\n\ngo 1.25\n"
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte(content), 0644); err != nil {
		t.Fatalf("create go.mod: %v", err)
	}
}

func createMainPackage(t *testing.T, root, relPath string, imports []string) {
	t.Helper()

	dir := filepath.Join(root, relPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("create directory %s: %v", dir, err)
	}

	var importBlock string
	if len(imports) > 0 {
		importBlock = "import (\n"
		for _, imp := range imports {
			importBlock += "\t\"" + imp + "\"\n"
		}
		importBlock += ")\n\n"
	}

	mainGo := `package main

` + importBlock + `func main() {
	// Reference imported types to ensure they appear in TypesInfo
`
	for _, imp := range imports {
		pkgName := filepath.Base(imp)
		typeName := capitalize(pkgName)
		mainGo += "\t_ = " + pkgName + "." + typeName + "{}\n"
	}

	mainGo += `}
`

	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(mainGo), 0644); err != nil {
		t.Fatalf("write main.go in %s: %v", dir, err)
	}
}

func createDummyPackage(t *testing.T, root, pkgName, typeName string) {
	t.Helper()

	dir := filepath.Join(root, pkgName)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("create directory %s: %v", dir, err)
	}

	content := "package " + pkgName + "\n\ntype " + typeName + " struct{}\n"
	if err := os.WriteFile(filepath.Join(dir, pkgName+".go"), []byte(content), 0644); err != nil {
		t.Fatalf("write %s.go: %v", pkgName, err)
	}
}

func capitalize(s string) string {
	if s == "" {
		return ""
	}
	// Special case for common acronyms
	switch s {
	case "db":
		return "DB"
	case "api":
		return "API"
	case "http":
		return "HTTP"
	case "url":
		return "URL"
	}

	// Convert first letter to uppercase
	first := s[0]
	if first >= 'a' && first <= 'z' {
		first = first - 32
	}
	return string(first) + s[1:]
}

func mapKeys[K comparable, V any](m map[K]V) []K {
	keys := make([]K, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

func stringSlicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}

	aSet := make(map[string]bool)
	for _, s := range a {
		aSet[s] = true
	}

	for _, s := range b {
		if !aSet[s] {
			return false
		}
	}

	return true
}
