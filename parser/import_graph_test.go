package parser

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestParseDirectoryWithTypes_ImportGraphIncludesPackagesWithoutTypes(t *testing.T) {
	root := t.TempDir()
	createGoMod(t, root, "test")

	routerDir := filepath.Join(root, "router")
	if err := os.MkdirAll(routerDir, 0o755); err != nil {
		t.Fatalf("create router dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(routerDir, "router.go"), []byte("package router\n\nfunc New() int { return 1 }\n"), 0o644); err != nil {
		t.Fatalf("write router.go: %v", err)
	}

	serverDir := filepath.Join(root, "cmd", "server")
	if err := os.MkdirAll(serverDir, 0o755); err != nil {
		t.Fatalf("create server dir: %v", err)
	}
	mainSource := `package main

import "test/router"

func main() {
	_ = router.New()
}`
	if err := os.WriteFile(filepath.Join(serverDir, "main.go"), []byte(mainSource), 0o644); err != nil {
		t.Fatalf("write main.go: %v", err)
	}

	p := NewParser()
	components, pkgTypeInfo, err := p.ParseDirectoryWithTypes(root)
	if err != nil {
		t.Fatalf("ParseDirectoryWithTypes() failed: %v", err)
	}

	if pkgTypeInfo == nil {
		t.Fatal("pkgTypeInfo is nil")
	}

	if _, ok := pkgTypeInfo.LoadedPackagesByPath["test/router"]; !ok {
		t.Fatal("expected loaded package test/router to be present")
	}

	imports, ok := pkgTypeInfo.PackageImports["test/cmd/server"]
	if !ok {
		t.Fatal("expected import graph entry for test/cmd/server")
	}

	foundRouterImport := slices.Contains(imports, "test/router")
	if !foundRouterImport {
		t.Fatalf("expected test/cmd/server imports to include test/router, got: %v", imports)
	}

	hasRouterComponent := false
	for _, comp := range components {
		if comp.PackagePath == "test/router" {
			hasRouterComponent = true
			break
		}
	}
	if hasRouterComponent {
		t.Fatal("router package should not produce components when it has no type declarations")
	}
}
