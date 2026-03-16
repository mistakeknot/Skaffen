package repomap

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
)

// TagDef represents a symbol definition.
type TagDef struct {
	File  string // relative path
	Name  string // symbol name
	Line  int    // definition line
	Kind  string // "func", "type", "method"
	Scope string // receiver type for methods (e.g. "*Foo")
}

// RefEdge represents a cross-file reference.
type RefEdge struct {
	SrcFile string // file containing the reference
	DstFile string // file containing the definition
	Symbol  string // name being referenced
}

// ExtractGoTags parses Go files under root and returns definitions and
// cross-file reference edges. Skips test files, vendor, hidden dirs.
func ExtractGoTags(root string, maxFiles int) ([]TagDef, []RefEdge) {
	if maxFiles <= 0 {
		maxFiles = 200
	}
	var defs []TagDef
	var edges []RefEdge
	fileCount := 0

	// Phase 1: collect all definitions by package
	pkgDefs := make(map[string]map[string]string) // dir → symbol → file

	filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			if d != nil && d.IsDir() {
				base := d.Name()
				if strings.HasPrefix(base, ".") || base == "vendor" || base == "testdata" || base == "node_modules" {
					return filepath.SkipDir
				}
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		if fileCount >= maxFiles {
			return filepath.SkipAll
		}
		fileCount++

		rel, _ := filepath.Rel(root, path)
		dir := filepath.Dir(rel)
		fset := token.NewFileSet()
		f, parseErr := parser.ParseFile(fset, path, nil, 0)
		if parseErr != nil {
			return nil
		}

		if pkgDefs[dir] == nil {
			pkgDefs[dir] = make(map[string]string)
		}

		for _, decl := range f.Decls {
			switch dd := decl.(type) {
			case *ast.GenDecl:
				for _, spec := range dd.Specs {
					if ts, ok := spec.(*ast.TypeSpec); ok && ts.Name.IsExported() {
						defs = append(defs, TagDef{
							File: rel, Name: ts.Name.Name,
							Line: fset.Position(ts.Pos()).Line, Kind: "type",
						})
						pkgDefs[dir][ts.Name.Name] = rel
					}
				}
			case *ast.FuncDecl:
				if dd.Name.IsExported() {
					td := TagDef{
						File: rel, Name: dd.Name.Name,
						Line: fset.Position(dd.Pos()).Line,
					}
					if dd.Recv != nil && len(dd.Recv.List) > 0 {
						td.Kind = "method"
						td.Scope = formatRecvType(dd.Recv.List[0].Type)
					} else {
						td.Kind = "func"
					}
					defs = append(defs, td)
					pkgDefs[dir][dd.Name.Name] = rel
				}
			}
		}
		return nil
	})

	// Phase 2: collect cross-file references via SelectorExpr in function bodies
	fileCount = 0
	filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			if d != nil && d.IsDir() {
				base := d.Name()
				if strings.HasPrefix(base, ".") || base == "vendor" || base == "testdata" || base == "node_modules" {
					return filepath.SkipDir
				}
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		if fileCount >= maxFiles {
			return filepath.SkipAll
		}
		fileCount++

		rel, _ := filepath.Rel(root, path)
		fset := token.NewFileSet()
		f, parseErr := parser.ParseFile(fset, path, nil, 0)
		if parseErr != nil {
			return nil
		}

		// Build import alias map: alias -> package path
		imports := make(map[string]string)
		for _, imp := range f.Imports {
			impPath := strings.Trim(imp.Path.Value, `"`)
			// Use the last path component as the default alias
			parts := strings.Split(impPath, "/")
			alias := parts[len(parts)-1]
			if imp.Name != nil {
				alias = imp.Name.Name
			}
			imports[alias] = impPath
		}

		// Walk function bodies for SelectorExpr (pkg.Symbol calls)
		for _, decl := range f.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				sel, ok := n.(*ast.SelectorExpr)
				if !ok {
					return true
				}
				ident, ok := sel.X.(*ast.Ident)
				if !ok {
					return true
				}
				// Check if this is a package-qualified reference
				pkgPath, isImport := imports[ident.Name]
				if !isImport {
					return true
				}
				// Try to resolve to a known definition
				parts := strings.Split(pkgPath, "/")
				for dir, syms := range pkgDefs {
					dirParts := strings.Split(dir, string(filepath.Separator))
					if len(dirParts) > 0 && dirParts[len(dirParts)-1] == parts[len(parts)-1] {
						if defFile, ok := syms[sel.Sel.Name]; ok && defFile != rel {
							edges = append(edges, RefEdge{
								SrcFile: rel,
								DstFile: defFile,
								Symbol:  sel.Sel.Name,
							})
						}
					}
				}
				return true
			})
		}
		return nil
	})

	return defs, edges
}

// formatRecvType extracts the receiver type name from an AST expression,
// preserving the pointer indicator (e.g. "*Foo").
func formatRecvType(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.StarExpr:
		return "*" + formatRecvType(t.X)
	case *ast.Ident:
		return t.Name
	default:
		return "?"
	}
}

// FormatMap renders a list of definitions as text for the system prompt.
// Definitions are grouped by directory. If maxChars > 0, output is truncated.
func FormatMap(defs []TagDef, maxChars int) string {
	if len(defs) == 0 {
		return ""
	}

	// Group by directory, preserving order of first appearance
	type pkgInfo struct {
		dir     string
		symbols []string
	}
	pkgMap := make(map[string]*pkgInfo)
	var order []string

	for _, d := range defs {
		dir := filepath.Dir(d.File)
		if pkgMap[dir] == nil {
			pkgMap[dir] = &pkgInfo{dir: dir}
			order = append(order, dir)
		}
		var sym string
		switch d.Kind {
		case "method":
			sym = fmt.Sprintf("func (%s) %s()", d.Scope, d.Name)
		case "func":
			sym = fmt.Sprintf("func %s()", d.Name)
		case "type":
			sym = fmt.Sprintf("type %s", d.Name)
		}
		pkgMap[dir].symbols = append(pkgMap[dir].symbols, sym)
	}

	var b strings.Builder
	b.WriteString("Repository Map (ranked by relevance)\n")
	b.WriteString(strings.Repeat("=", 40) + "\n\n")

	for _, dir := range order {
		info := pkgMap[dir]
		fmt.Fprintf(&b, "%s/\n", info.dir)
		seen := make(map[string]bool)
		for _, s := range info.symbols {
			if !seen[s] {
				seen[s] = true
				fmt.Fprintf(&b, "  %s\n", s)
			}
		}
		b.WriteString("\n")
		if maxChars > 0 && b.Len() > maxChars {
			break
		}
	}

	return strings.TrimRight(b.String(), "\n")
}
