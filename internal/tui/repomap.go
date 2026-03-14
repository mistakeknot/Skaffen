package tui

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const (
	maxMapFiles  = 100 // max files to parse
	maxMapOutput = 8000 // max output characters
)

// generateRepoMap walks a directory tree, parses Go files, and produces
// a structural overview showing packages with their exported types and functions.
func generateRepoMap(root string) string {
	pkgs := make(map[string][]string) // package path → symbols
	fileCount := 0

	filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // skip unreadable dirs
		}
		// Skip hidden dirs, vendor, testdata, node_modules
		if d.IsDir() {
			base := d.Name()
			if strings.HasPrefix(base, ".") || base == "vendor" || base == "testdata" || base == "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		if fileCount >= maxMapFiles {
			return filepath.SkipAll
		}
		fileCount++

		rel, _ := filepath.Rel(root, path)
		dir := filepath.Dir(rel)

		symbols := extractGoSymbols(path)
		if len(symbols) > 0 {
			pkgs[dir] = append(pkgs[dir], symbols...)
		}
		return nil
	})

	if len(pkgs) == 0 {
		return ""
	}

	// Sort packages
	dirs := make([]string, 0, len(pkgs))
	for d := range pkgs {
		dirs = append(dirs, d)
	}
	sort.Strings(dirs)

	var b strings.Builder
	b.WriteString("Repository Map\n")
	b.WriteString(strings.Repeat("=", 40) + "\n\n")

	for _, dir := range dirs {
		symbols := pkgs[dir]
		if len(symbols) == 0 {
			continue
		}
		// Deduplicate
		seen := make(map[string]bool)
		var unique []string
		for _, s := range symbols {
			if !seen[s] {
				seen[s] = true
				unique = append(unique, s)
			}
		}

		fmt.Fprintf(&b, "%s/\n", dir)
		for _, s := range unique {
			fmt.Fprintf(&b, "  %s\n", s)
		}
		b.WriteString("\n")

		if b.Len() > maxMapOutput {
			b.WriteString("... (truncated)\n")
			break
		}
	}

	return strings.TrimRight(b.String(), "\n")
}

// extractGoSymbols parses a Go file and returns exported symbols.
// Format: "type TypeName", "func FuncName()", "func (T) Method()"
func extractGoSymbols(path string) []string {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		return nil
	}

	var symbols []string
	for _, decl := range f.Decls {
		switch d := decl.(type) {
		case *ast.GenDecl:
			for _, spec := range d.Specs {
				switch s := spec.(type) {
				case *ast.TypeSpec:
					if s.Name.IsExported() {
						symbols = append(symbols, fmt.Sprintf("type %s", s.Name.Name))
					}
				}
			}
		case *ast.FuncDecl:
			if d.Name.IsExported() {
				if d.Recv != nil && len(d.Recv.List) > 0 {
					recv := formatRecv(d.Recv.List[0].Type)
					symbols = append(symbols, fmt.Sprintf("func (%s) %s()", recv, d.Name.Name))
				} else {
					symbols = append(symbols, fmt.Sprintf("func %s()", d.Name.Name))
				}
			}
		}
	}
	return symbols
}

// formatRecv extracts the receiver type name from an AST expression.
func formatRecv(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.StarExpr:
		return "*" + formatRecv(t.X)
	case *ast.Ident:
		return t.Name
	default:
		return "?"
	}
}
