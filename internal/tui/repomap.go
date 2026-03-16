package tui

import (
	"sort"
	"strings"

	"github.com/mistakeknot/Skaffen/internal/repomap"
)

const (
	maxMapFiles  = 100 // max files to parse
	maxMapOutput = 8000 // max output characters
)

// generateRepoMap walks a directory tree, parses Go files, and produces
// a structural overview showing packages with their exported types and functions.
// This is a thin wrapper around the repomap package.
func generateRepoMap(root string) string {
	defs, _ := repomap.ExtractGoTags(root, maxMapFiles)
	if len(defs) == 0 {
		return ""
	}

	// Group by directory (matching the original alphabetical output)
	type pkgInfo struct {
		dir     string
		symbols []string
	}
	pkgMap := make(map[string]*pkgInfo)
	for _, d := range defs {
		dir := dirOf(d.File)
		if pkgMap[dir] == nil {
			pkgMap[dir] = &pkgInfo{dir: dir}
		}
		var sym string
		switch d.Kind {
		case "method":
			sym = "func (" + d.Scope + ") " + d.Name + "()"
		case "func":
			sym = "func " + d.Name + "()"
		case "type":
			sym = "type " + d.Name
		}
		pkgMap[dir].symbols = append(pkgMap[dir].symbols, sym)
	}

	// Sort packages alphabetically (original behavior)
	dirs := make([]string, 0, len(pkgMap))
	for d := range pkgMap {
		dirs = append(dirs, d)
	}
	sort.Strings(dirs)

	var b strings.Builder
	b.WriteString("Repository Map\n")
	b.WriteString(strings.Repeat("=", 40) + "\n\n")

	for _, dir := range dirs {
		info := pkgMap[dir]
		if len(info.symbols) == 0 {
			continue
		}
		// Deduplicate
		seen := make(map[string]bool)
		var unique []string
		for _, s := range info.symbols {
			if !seen[s] {
				seen[s] = true
				unique = append(unique, s)
			}
		}
		b.WriteString(dir + "/\n")
		for _, s := range unique {
			b.WriteString("  " + s + "\n")
		}
		b.WriteString("\n")
		if b.Len() > maxMapOutput {
			b.WriteString("... (truncated)\n")
			break
		}
	}

	return strings.TrimRight(b.String(), "\n")
}

// dirOf returns filepath.Dir without importing path/filepath to keep this thin.
func dirOf(path string) string {
	i := strings.LastIndexByte(path, '/')
	if i < 0 {
		return "."
	}
	return path[:i]
}
