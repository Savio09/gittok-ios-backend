package crawler

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strings"

	"golang.org/x/mod/modfile"
)

// This file holds the syntax-aware Go analysis the classifier relies on, so a
// dependency or a server call is recognised from the actual program structure
// rather than from a substring. The classification review (Slice 3a) showed the
// old string matching turning a `// ListenAndServe` comment into a server and a
// `// no longer uses …/gin` comment into a dependency; parsing closes both.

// parseGo parses Go source into an AST. It returns the file (which may be a
// partial tree when the source was clipped) and whether parsing fully
// succeeded. A positive signal found in a partial tree is real; an *absence* is
// only trustworthy when parsed is true.
func parseGo(src string) (file *ast.File, parsed bool) {
	f, err := parser.ParseFile(token.NewFileSet(), "", src, parser.SkipObjectResolution)
	return f, err == nil
}

// goDeclaresPackageMain reports whether the source's package clause is `main`.
// It reads the parsed package name, so `package main` inside a comment or string
// never counts.
func goDeclaresPackageMain(src string) bool {
	f, _ := parseGo(src)
	return f != nil && f.Name != nil && f.Name.Name == "main"
}

// goStartsHTTPServer reports whether the source contains a real call that starts
// an HTTP server — http.ListenAndServe / ListenAndServeTLS, or a composite
// literal http.Server{…} — and whether the file parsed cleanly. Only AST call
// and literal nodes count, so a mention in a comment or string does not. A false
// result is trustworthy only when parsed is true: a clipped, unparseable file
// cannot prove the absence of a server call.
func goStartsHTTPServer(src string) (starts, parsed bool) {
	f, ok := parseGo(src)
	if f == nil {
		return false, false
	}
	ast.Inspect(f, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.CallExpr:
			if sel, isSel := node.Fun.(*ast.SelectorExpr); isSel {
				switch sel.Sel.Name {
				case "ListenAndServe", "ListenAndServeTLS":
					starts = true
				}
			}
		case *ast.CompositeLit:
			if sel, isSel := node.Type.(*ast.SelectorExpr); isSel && sel.Sel.Name == "Server" {
				if pkg, isID := sel.X.(*ast.Ident); isID && pkg.Name == "http" {
					starts = true
				}
			}
		}
		return !starts // stop walking once a server signal is found
	})
	return starts, ok
}

// goModRequiredPaths returns the module paths required by a go.mod, parsed with
// the official module-file parser so comments, the module directive and
// replace/exclude blocks are handled correctly — a module named only in a
// comment is never returned. A go.mod that will not parse yields no paths.
func goModRequiredPaths(goMod string) []string {
	f, err := modfile.Parse("go.mod", []byte(goMod), nil)
	if err != nil {
		return nil
	}
	paths := make([]string, 0, len(f.Require))
	for _, r := range f.Require {
		paths = append(paths, r.Mod.Path)
	}
	return paths
}

// goRequiresAny returns the first of wants (partial module paths such as
// "gin-gonic/gin") that is a substring of some required module path, or "".
func goRequiresAny(goMod string, wants []string) string {
	paths := goModRequiredPaths(goMod)
	for _, w := range wants {
		for _, p := range paths {
			if strings.Contains(p, w) {
				return w
			}
		}
	}
	return ""
}
