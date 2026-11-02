package crawler

import (
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"path"
	"strconv"
	"strings"
	"sync"

	"golang.org/x/mod/module"
)

// This file holds the syntax-aware Go analysis the classifier relies on, so a
// dependency or a server call is recognised from the actual program structure
// rather than from a substring. The classification review (Slice 3a) showed the
// old string matching turning a `// ListenAndServe` comment into a server and a
// `// no longer uses …/gin` comment into a dependency; parsing closes both.

// goFile is what the classifier knows about one read Go source: its package
// clause, whether it was read in full and parsed, and what it imports. The
// package clause sits at the top of the file, so it is usually known even for a
// clipped or broken file; a *positive* fact read from a partial file is real,
// but an absence inside it is only trustworthy when complete and parsed.
type goFile struct {
	path     string
	pkg      string // package clause; "" when even the clause is unreadable
	pkgStart int    // byte offsets of the package name in the original file
	pkgEnd   int
	test     bool   // *_test.go
	complete bool   // content read in full
	parsed   bool   // the whole file parsed
	parseErr string // first parse error, when !parsed
	imports  []string
	content  string

	serverCalls []goServerCall // HTTP server-starting calls whose identity resolved to net/http
	uncertain   []string       // server-like calls whose identity could not be established
}

// analyzeGoFile parses one read source into a goFile.
func analyzeGoFile(f SourceFile) goFile {
	g := goFile{
		path:     f.Path,
		test:     strings.HasSuffix(strings.ToLower(f.Path), "_test.go"),
		complete: !f.Truncated,
		content:  f.Content,
	}
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, f.Path, f.Content, parser.SkipObjectResolution)
	if err == nil {
		g.parsed = true
	} else {
		g.parseErr = strings.SplitN(err.Error(), "\n", 2)[0]
	}
	if file != nil {
		if file.Name != nil {
			g.pkg = file.Name.Name
			g.pkgStart, g.pkgEnd = fset.Position(file.Name.Pos()).Offset, fset.Position(file.Name.End()).Offset
		}
		for _, im := range file.Imports {
			if ip, uerr := strconv.Unquote(im.Path.Value); uerr == nil {
				g.imports = append(g.imports, ip)
			}
		}
		g.serverCalls, g.uncertain = scanServerCalls(fset, file, f.Content)
	}
	if g.pkg == "" { // a broken body can still have a readable package clause
		pfset := token.NewFileSet()
		if pf, _ := parser.ParseFile(pfset, "", f.Content, parser.PackageClauseOnly); pf != nil && pf.Name != nil {
			g.pkg = pf.Name.Name
			g.pkgStart, g.pkgEnd = pfset.Position(pf.Name.Pos()).Offset, pfset.Position(pf.Name.End()).Offset
		}
	}
	return g
}

// goModulePath returns the module path a go.mod declares, or "" if it will not
// parse or declares none.
func goModulePath(goMod string) string {
	f, err := parseGoModSource("go.mod", goMod)
	if err != nil || f.Module == nil {
		return ""
	}
	return f.Module.Mod.Path
}

// goServerCall is one call expression, found in source, that the type checker
// resolved to a net/http server-starting function or *http.Server method. It is
// static syntax: it shows the program contains the call, not that it ran.
type goServerCall struct {
	start, end int    // byte offsets of the whole call expression in the original file
	line       int    // 1-based line of the call
	callee     string // the callee as written, e.g. "http.ListenAndServe" or "srv.ListenAndServe"
	method     bool   // a method on *http.Server rather than a package function
}

// serverCallNames are the net/http entry points that start serving HTTP, as
// package functions and as methods on *http.Server.
var serverCallNames = []string{"ListenAndServe", "ListenAndServeTLS", "Serve", "ServeTLS"}

func isServerCallName(name string) bool {
	for _, n := range serverCallNames {
		if n == name {
			return true
		}
	}
	return false
}

// netHTTPStub is a minimal stand-in for the part of net/http that starts a
// server: the four package functions above and a Server type carrying the same
// four methods. With it the type checker can resolve call identity — imports,
// aliases, dot imports, shadowing, receivers declared as *http.Server — without
// loading the standard library. Parameters are variadic `any`, so argument
// mismatches never hide a call.
var netHTTPStub = sync.OnceValue(func() *types.Package {
	pkg := types.NewPackage("net/http", "http")
	errT := types.Universe.Lookup("error").Type()
	anyT := types.Universe.Lookup("any").Type()
	sig := func(recv *types.Var) *types.Signature {
		params := types.NewTuple(types.NewParam(token.NoPos, pkg, "args", types.NewSlice(anyT)))
		results := types.NewTuple(types.NewParam(token.NoPos, pkg, "", errT))
		return types.NewSignatureType(recv, nil, nil, params, results, true)
	}
	server := types.NewNamed(types.NewTypeName(token.NoPos, pkg, "Server", nil), types.NewStruct(nil, nil), nil)
	pkg.Scope().Insert(server.Obj())
	recv := types.NewVar(token.NoPos, pkg, "srv", types.NewPointer(server))
	for _, name := range serverCallNames {
		pkg.Scope().Insert(types.NewFunc(token.NoPos, pkg, name, sig(nil)))
		server.AddMethod(types.NewFunc(token.NoPos, pkg, name, sig(recv)))
	}
	pkg.MarkComplete()
	return pkg
})

// stubImporter serves netHTTPStub for net/http and an empty package for every
// other import path, so a file type-checks in isolation and offline.
type stubImporter map[string]*types.Package

func (m stubImporter) Import(importPath string) (*types.Package, error) {
	if importPath == "net/http" {
		return netHTTPStub(), nil
	}
	if p, ok := m[importPath]; ok {
		return p, nil
	}
	name := path.Base(importPath)
	if i := strings.LastIndex(name, ".v"); i > 0 { // gopkg.in/yaml.v3 -> yaml
		name = name[:i]
	}
	name = strings.NewReplacer("-", "_", ".", "_").Replace(name)
	p := types.NewPackage(importPath, name)
	p.MarkComplete()
	m[importPath] = p
	return p, nil
}

// scanServerCalls type-checks one file against the stubs and returns the calls
// that resolve to net/http server entry points, plus the server-like calls —
// a callee named like one — whose identity could not be established. A method
// on a local type (logger{}.ListenAndServe()) resolves to that type and is
// neither; an http.Server composite literal is a value, not a call, and is
// neither. Type errors from the stubs or from other files are expected and
// ignored: only resolved identities count.
func scanServerCalls(fset *token.FileSet, file *ast.File, src string) (calls []goServerCall, uncertain []string) {
	info := &types.Info{
		Uses:       map[*ast.Ident]types.Object{},
		Selections: map[*ast.SelectorExpr]*types.Selection{},
	}
	conf := types.Config{Importer: stubImporter{}, Error: func(error) {}, FakeImportC: true}
	_, _ = conf.Check(file.Name.Name, fset, []*ast.File{file}, info)

	ast.Inspect(file, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		fun := ast.Unparen(call.Fun)
		var name string
		var obj types.Object
		method := false
		switch f := fun.(type) {
		case *ast.SelectorExpr:
			name = f.Sel.Name
			if sel, ok := info.Selections[f]; ok {
				obj, method = sel.Obj(), true
			} else {
				obj = info.Uses[f.Sel]
			}
		case *ast.Ident:
			name, obj = f.Name, info.Uses[f]
		default:
			return true
		}
		if !isServerCallName(name) {
			return true
		}
		start, end := fset.Position(call.Pos()), fset.Position(call.End())
		callee := src[fset.Position(fun.Pos()).Offset:fset.Position(fun.End()).Offset]
		fn, isFunc := obj.(*types.Func)
		switch {
		case isFunc && fn.Pkg() != nil && fn.Pkg().Path() == "net/http":
			calls = append(calls, goServerCall{start: start.Offset, end: end.Offset, line: start.Line, callee: callee, method: method})
		case obj == nil:
			uncertain = append(uncertain, "line "+strconv.Itoa(start.Line)+": "+callee+"(…) — its package or receiver type was not established, so it is neither counted as an HTTP server call nor ruled out")
		}
		return true
	})
	return calls, uncertain
}

// goModRequiredPaths returns the module paths required by a go.mod, parsed with
// the official module-file parser so comments, the module directive and
// replace/exclude blocks are handled correctly — a module named only in a
// comment is never returned. A go.mod that will not parse yields no paths.
func goModRequiredPaths(goMod string) []string {
	f, err := parseGoModSource("go.mod", goMod)
	if err != nil {
		return nil
	}
	paths := make([]string, 0, len(f.Require))
	for _, r := range f.Require {
		paths = append(paths, r.Mod.Path)
	}
	return paths
}

// goRequiresAny returns the first canonical module family in wants that is
// required, accounting for Go's major-version suffixes, or "".
func goRequiresAny(goMod string, wants []string) string {
	paths := goModRequiredPaths(goMod)
	for _, w := range wants {
		for _, p := range paths {
			if matchesGoModule(p, w) {
				return w
			}
		}
	}
	return ""
}

// Compare complete module identities, never host/path substrings or arbitrary
// submodules. SplitPathVersion also handles gopkg.in's .vN spelling.
// https://go.dev/ref/mod#major-version-suffixes
func matchesGoModule(required, canonical string) bool {
	prefix, _, ok := module.SplitPathVersion(required)
	family, _, known := module.SplitPathVersion(canonical)
	return ok && known && prefix == family
}
