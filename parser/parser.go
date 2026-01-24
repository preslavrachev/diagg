package parser

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/tools/go/packages"
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
	Name        string
	TypeName    string
	IsPointer   bool
	IsSlice     bool
	IsInterface bool // True if field type is an interface
}

// Method represents a method on a type
type Method struct {
	Name       string
	Parameters []string
	Returns    []string
}

// InterfaceMethod represents a method signature in an interface
type InterfaceMethod struct {
	Name       string
	Parameters []string
	Returns    []string
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

// PackageTypeInfo contains type information for analyzing interfaces
type PackageTypeInfo struct {
	Packages map[string]*types.Package // Package name -> package
	TypeInfo *types.Info               // Combined type information
}

// ParseDirectoryWithTypes parses a directory using go/packages for full type information
// This enables interface implementation detection but is slower than AST-only parsing
func (p *Parser) ParseDirectoryWithTypes(root string) ([]Component, *PackageTypeInfo, error) {
	cfg := &packages.Config{
		Mode: packages.NeedName |
			packages.NeedTypes |
			packages.NeedTypesInfo |
			packages.NeedSyntax |
			packages.NeedFiles,
		Dir: root,
	}

	// Load all packages in the directory
	pkgs, err := packages.Load(cfg, "./...")
	if err != nil {
		return nil, nil, fmt.Errorf("loading packages: %w", err)
	}

	// Check for errors in loaded packages
	if packages.PrintErrors(pkgs) > 0 {
		return nil, nil, fmt.Errorf("packages contain errors")
	}

	var components []Component
	pkgTypeInfo := &PackageTypeInfo{
		Packages: make(map[string]*types.Package),
	}

	for _, pkg := range pkgs {
		if pkg.Types == nil || pkg.TypesInfo == nil {
			continue
		}

		// Store package for interface checking
		pkgTypeInfo.Packages[pkg.Name] = pkg.Types

		// Store type info from first package
		if pkgTypeInfo.TypeInfo == nil {
			pkgTypeInfo.TypeInfo = pkg.TypesInfo
		}

		// Extract components from each package
		for _, syntax := range pkg.Syntax {
			pkgComponents := p.extractComponentsFromAST(syntax, pkg)
			components = append(components, pkgComponents...)
		}
	}

	return components, pkgTypeInfo, nil
}

// extractComponentsFromAST extracts components from an AST with type information
func (p *Parser) extractComponentsFromAST(file *ast.File, pkg *packages.Package) []Component {
	var components []Component
	packageName := pkg.Name
	packagePath := pkg.PkgPath

	ast.Inspect(file, func(n ast.Node) bool {
		genDecl, ok := n.(*ast.GenDecl)
		if !ok || genDecl.Tok != token.TYPE {
			return true
		}

		for _, spec := range genDecl.Specs {
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
				component.Fields = p.extractFieldsWithTypeInfo(t, pkg)
				// AIDEV-NOTE: method-extraction; extract methods for this type from the package
				component.Methods = p.extractMethodsForType(typeSpec.Name.Name, pkg)
			case *ast.InterfaceType:
				component.Kind = "interface"
				// Extract interface methods if needed
			}

			components = append(components, component)
		}
		return true
	})

	return components
}

// extractFieldsWithTypeInfo extracts fields with type information
func (p *Parser) extractFieldsWithTypeInfo(structType *ast.StructType, pkg *packages.Package) []Field {
	var fields []Field

	if structType.Fields == nil {
		return fields
	}

	for _, field := range structType.Fields.List {
		if len(field.Names) == 0 {
			continue
		}

		for _, name := range field.Names {
			if !ast.IsExported(name.Name) {
				continue
			}

			f := Field{
				Name: name.Name,
			}

			// Extract type information
			f.TypeName, f.IsPointer, f.IsSlice = p.extractTypeInfo(field.Type)

			// Check if field type is an interface using type information
			if pkg.TypesInfo != nil {
				if tv, ok := pkg.TypesInfo.Types[field.Type]; ok {
					// Unwrap pointer/slice to get underlying type
					underlyingType := tv.Type
					if ptr, ok := underlyingType.(*types.Pointer); ok {
						underlyingType = ptr.Elem()
					}
					if slice, ok := underlyingType.(*types.Slice); ok {
						underlyingType = slice.Elem()
					}

					// Check if it's an interface type
					if _, ok := underlyingType.Underlying().(*types.Interface); ok {
						f.IsInterface = true
					}
				}
			}

			fields = append(fields, f)
		}
	}

	return fields
}

// extractMethodsForType extracts methods defined on a named type
// AIDEV-NOTE: method-extraction; scans all function declarations for methods with receiver matching typeName
func (p *Parser) extractMethodsForType(typeName string, pkg *packages.Package) []Method {
	var methods []Method

	for _, syntax := range pkg.Syntax {
		ast.Inspect(syntax, func(n ast.Node) bool {
			funcDecl, ok := n.(*ast.FuncDecl)
			if !ok || funcDecl.Recv == nil {
				return true
			}

			// Check if this method belongs to our type
			for _, recv := range funcDecl.Recv.List {
				recvTypeName := p.extractReceiverTypeName(recv.Type)
				if recvTypeName == typeName && ast.IsExported(funcDecl.Name.Name) {
					method := Method{
						Name:       funcDecl.Name.Name,
						Parameters: p.extractFunctionParams(funcDecl.Type.Params),
						Returns:    p.extractFunctionResults(funcDecl.Type.Results),
					}
					methods = append(methods, method)
				}
			}
			return true
		})
	}

	return methods
}

// extractReceiverTypeName extracts type name from receiver (handles *T and T)
func (p *Parser) extractReceiverTypeName(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.StarExpr:
		if ident, ok := t.X.(*ast.Ident); ok {
			return ident.Name
		}
	}
	return ""
}

// extractFunctionParams extracts parameter types from function signature
func (p *Parser) extractFunctionParams(fieldList *ast.FieldList) []string {
	if fieldList == nil {
		return nil
	}

	var params []string
	for _, param := range fieldList.List {
		typeName := p.formatType(param.Type)
		params = append(params, typeName)
	}
	return params
}

// extractFunctionResults extracts return types from function signature
func (p *Parser) extractFunctionResults(fieldList *ast.FieldList) []string {
	if fieldList == nil {
		return nil
	}

	var results []string
	for _, result := range fieldList.List {
		typeName := p.formatType(result.Type)
		results = append(results, typeName)
	}
	return results
}

// formatType converts ast.Expr to string representation of type
func (p *Parser) formatType(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.StarExpr:
		return "*" + p.formatType(t.X)
	case *ast.ArrayType:
		return "[]" + p.formatType(t.Elt)
	case *ast.SelectorExpr:
		if ident, ok := t.X.(*ast.Ident); ok {
			return ident.Name + "." + t.Sel.Name
		}
	case *ast.InterfaceType:
		return "interface{}"
	case *ast.MapType:
		return "map[" + p.formatType(t.Key) + "]" + p.formatType(t.Value)
	}
	return "unknown"
}
