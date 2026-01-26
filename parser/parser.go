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
		// Skip embedded fields (fields without names)
		if len(field.Names) == 0 {
			continue
		}

		for _, name := range field.Names {
			// AIDEV-NOTE: dependency-tracking; include ALL fields (exported and unexported) for accurate dependency graphs
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

		// AIDEV-NOTE: main-package-detection; create synthetic component for main packages
		if pkg.Name == "main" {
			mainComponent := p.extractMainComponent(pkg)
			if mainComponent != nil {
				components = append(components, *mainComponent)
			}
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
			// AIDEV-NOTE: dependency-tracking; include ALL fields (exported and unexported) for accurate dependency graphs
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
				// AIDEV-NOTE: dependency-tracking; include ALL methods (exported and unexported) for accurate dependency graphs
				if recvTypeName == typeName {
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

// extractMainComponent creates a synthetic component for main packages
// AIDEV-NOTE: main-entrypoint; extracts all types referenced by walking the type info
// AIDEV-NOTE: multiple-mains; uses actual file location to distinguish between multiple main packages
func (p *Parser) extractMainComponent(pkg *packages.Package) *Component {
	if pkg.Name != "main" {
		return nil
	}

	// Find a file that contains func main() to determine the actual location
	var mainFilePath string
	for _, syntax := range pkg.Syntax {
		ast.Inspect(syntax, func(n ast.Node) bool {
			if funcDecl, ok := n.(*ast.FuncDecl); ok {
				if funcDecl.Name.Name == "main" && funcDecl.Recv == nil {
					// Found func main() - extract file path
					if pos := pkg.Fset.Position(funcDecl.Pos()); pos.IsValid() {
						mainFilePath = pos.Filename
						return false // stop searching
					}
				}
			}
			return true
		})
		if mainFilePath != "" {
			break
		}
	}

	// Derive component name from the directory containing main.go
	// Examples:
	//   /path/to/project/cmd/server/main.go -> "server"
	//   /path/to/project/cmd/worker/main.go -> "worker"
	//   /path/to/project/main.go -> "Main"
	//   /some/weird/structure/foo/bar/main.go -> "bar"
	var componentName string
	if mainFilePath != "" {
		// Get the directory containing the main file
		dir := filepath.Dir(mainFilePath)
		componentName = filepath.Base(dir)

		// Check if this is the module root by looking for go.mod in this directory
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			// main.go is at module root
			componentName = "Main"
		} else if componentName == "." || componentName == "/" {
			// Other root indicators
			componentName = "Main"
		}
	} else {
		// Fallback: no main function found (unusual), use package path
		componentName = filepath.Base(pkg.PkgPath)
		if componentName == "main" || componentName == "." {
			componentName = "Main"
		}
	}

	// Create synthetic component representing the main package
	component := Component{
		Name:        componentName,
		PackageName: "main",
		PackagePath: pkg.PkgPath,
		Kind:        "entrypoint", // Special kind for main packages
		Fields:      []Field{},    // Main doesn't have fields, but we'll populate with dependencies
		Methods:     []Method{},
	}

	// Track unique type dependencies
	depSet := make(map[string]bool)

	// Extract module path from main package path
	// For "github.com/user/project/cmd/server" -> "github.com/user/project"
	// For "github.com/user/project" -> "github.com/user/project"
	modulePath := pkg.PkgPath
	if idx := strings.Index(modulePath, "/cmd/"); idx != -1 {
		modulePath = modulePath[:idx]
	}

	// Helper to extract named types from a type
	var extractTypes func(t types.Type)
	extractTypes = func(t types.Type) {
		if t == nil {
			return
		}

		switch typ := t.(type) {
		case *types.Named:
			// This is a named type (struct, interface, etc.)
			if obj := typ.Obj(); obj != nil {
				if objPkg := obj.Pkg(); objPkg != nil {
					pkgPath := objPkg.Path()
					// Only include types from our project (same module)
					if strings.HasPrefix(pkgPath, modulePath) && objPkg.Name() != "main" {
						fullName := objPkg.Name() + "." + obj.Name()
						depSet[fullName] = true
					}
				}
			}
		case *types.Pointer:
			extractTypes(typ.Elem())
		case *types.Slice:
			extractTypes(typ.Elem())
		case *types.Map:
			extractTypes(typ.Key())
			extractTypes(typ.Elem())
		case *types.Signature:
			// Extract from parameters and results
			if typ.Params() != nil {
				for i := 0; i < typ.Params().Len(); i++ {
					extractTypes(typ.Params().At(i).Type())
				}
			}
			if typ.Results() != nil {
				for i := 0; i < typ.Results().Len(); i++ {
					extractTypes(typ.Results().At(i).Type())
				}
			}
		}
	}

	// Walk through all expressions in the package and extract their types
	if pkg.TypesInfo != nil {
		for expr, typeAndValue := range pkg.TypesInfo.Types {
			_ = expr // we don't need the expression itself
			extractTypes(typeAndValue.Type)
		}
	}

	// Convert dependencies to Field entries (abuse Fields to store dependencies)
	for dep := range depSet {
		parts := strings.Split(dep, ".")
		if len(parts) == 2 {
			component.Fields = append(component.Fields, Field{
				Name:     parts[1], // Type name (e.g., "Parser")
				TypeName: parts[1], // Same as name
			})
		}
	}

	return &component
}
