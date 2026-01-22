package parser

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
)

// Component represents a Go type (struct, interface) found in the codebase
type Component struct {
	Name        string
	PackageName string
	PackagePath string
	Kind        string // "struct" or "interface"
	Fields      []Field
	Methods     []Method
}

// Field represents a struct field
type Field struct {
	Name      string
	TypeName  string
	IsPointer bool
	IsSlice   bool
}

// Method represents a method on a type
type Method struct {
	Name string
}

// Parser parses Go source files to extract components
type Parser struct {
	fset *token.FileSet
}

// NewParser creates a new Parser
func NewParser() *Parser {
	return &Parser{
		fset: token.NewFileSet(),
	}
}

// ParseDirectory recursively parses all Go files in a directory
func (p *Parser) ParseDirectory(root string) ([]Component, error) {
	var components []Component

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return fmt.Errorf("walking path %s: %w", path, err)
		}

		// Skip vendor and hidden directories
		if info.IsDir() {
			name := info.Name()
			if name == "vendor" || name == ".git" || strings.HasPrefix(name, ".") {
				return filepath.SkipDir
			}
			return nil
		}

		// Only process .go files (skip test files for now)
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}

		fileComponents, err := p.parseFile(path)
		if err != nil {
			// AIDEV-NOTE: Soft-fail on parse errors - don't stop entire analysis
			return nil
		}

		components = append(components, fileComponents...)
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("parsing directory: %w", err)
	}

	return components, nil
}

// parseFile parses a single Go file and extracts components
func (p *Parser) parseFile(path string) ([]Component, error) {
	file, err := parser.ParseFile(p.fset, path, nil, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("parsing file %s: %w", path, err)
	}

	var components []Component
	packageName := file.Name.Name

	// Extract package path from file path (basic heuristic)
	packagePath := extractPackagePath(path)

	ast.Inspect(file, func(n ast.Node) bool {
		switch decl := n.(type) {
		case *ast.GenDecl:
			if decl.Tok == token.TYPE {
				for _, spec := range decl.Specs {
					typeSpec, ok := spec.(*ast.TypeSpec)
					if !ok {
						continue
					}

					component := Component{
						Name:        typeSpec.Name.Name,
						PackageName: packageName,
						PackagePath: packagePath,
					}

					switch t := typeSpec.Type.(type) {
					case *ast.StructType:
						component.Kind = "struct"
						component.Fields = p.extractFields(t)
					case *ast.InterfaceType:
						component.Kind = "interface"
						// We could extract interface methods here if needed
					default:
						// Skip other type declarations (aliases, etc.)
						return true
					}

					components = append(components, component)
				}
			}
		}
		return true
	})

	return components, nil
}

// extractFields extracts field information from a struct type
func (p *Parser) extractFields(structType *ast.StructType) []Field {
	var fields []Field

	if structType.Fields == nil {
		return fields
	}

	for _, field := range structType.Fields.List {
		// Skip embedded fields or unexported fields
		if len(field.Names) == 0 {
			continue
		}

		for _, name := range field.Names {
			// Skip unexported fields
			if !ast.IsExported(name.Name) {
				continue
			}

			f := Field{
				Name: name.Name,
			}

			// Extract type information
			f.TypeName, f.IsPointer, f.IsSlice = p.extractTypeInfo(field.Type)

			fields = append(fields, f)
		}
	}

	return fields
}

// extractTypeInfo extracts type name from an ast.Expr
func (p *Parser) extractTypeInfo(expr ast.Expr) (typeName string, isPointer bool, isSlice bool) {
	switch t := expr.(type) {
	case *ast.StarExpr:
		// Pointer type
		typeName, _, isSlice = p.extractTypeInfo(t.X)
		isPointer = true
	case *ast.ArrayType:
		// Slice or array type
		if t.Len == nil { // Slice (no length)
			typeName, isPointer, _ = p.extractTypeInfo(t.Elt)
			isSlice = true
		}
	case *ast.Ident:
		// Simple type name
		typeName = t.Name
	case *ast.SelectorExpr:
		// Qualified type name (e.g., pkg.Type)
		if ident, ok := t.X.(*ast.Ident); ok {
			typeName = ident.Name + "." + t.Sel.Name
		}
	case *ast.InterfaceType:
		typeName = "interface{}"
	case *ast.MapType:
		typeName = "map"
	case *ast.ChanType:
		typeName = "chan"
	case *ast.FuncType:
		typeName = "func"
	}

	return
}

// extractPackagePath extracts package path from file path
// This is a simple heuristic - in a real tool you'd want to use go/packages
func extractPackagePath(filePath string) string {
	// Normalize path
	filePath = filepath.ToSlash(filePath)

	// Try to find "go.mod" or common patterns
	// For MVP, just use the directory name
	dir := filepath.Dir(filePath)
	return filepath.Base(dir)
}
