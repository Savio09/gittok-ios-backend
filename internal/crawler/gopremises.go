package crawler

import (
	"fmt"
	"sort"
	"strings"
)

// validateGoPackageListing proves that a no-server witness covers one complete
// package's non-test files. Reading every supplied path is insufficient if the
// listing can omit a sibling or the witness left a known sibling out.
func validateGoPackageListing(st *inspIndex, paths []string) error {
	dir, hasPackage := "", false
	basis := make(map[string]bool, len(paths))
	for _, p := range paths {
		basis[p] = true
		if !isGoSource(p) {
			continue
		}
		if f, read := st.files[p]; read && analyzeGoFile(f).pkg != "main" {
			return fmt.Errorf("the no-server scope needs package main, but %s does not declare it", p)
		}
		if hasPackage && dirOf(p) != dir {
			return fmt.Errorf("the no-server scope mixes Go package directories")
		}
		dir, hasPackage = dirOf(p), true
	}
	if !hasPackage {
		return fmt.Errorf("the no-server scope needs a Go package source basis")
	}
	if !st.covers(segments(dir) + 1) {
		return fmt.Errorf("the listing does not cover all package files in %q; a sibling with a server call may be omitted", dir)
	}
	known := map[string]bool{}
	for _, p := range st.paths {
		known[p] = true
	}
	for p := range st.files {
		known[p] = true
	}
	for p := range st.unavail {
		known[p] = true
	}
	var omitted []string
	for p := range known {
		if isGoSource(p) && dirOf(p) == dir && !basis[p] {
			omitted = append(omitted, p)
		}
	}
	sort.Strings(omitted)
	if len(omitted) > 0 {
		return fmt.Errorf("the no-server scope omits package sibling %s", omitted[0])
	}
	return nil
}

// validateGoNoCommand re-derives "no package main at the module root or under
// cmd/" from the inspected bytes: go.mod read in full and parsed; a listing
// covering cmd/<name>/; every primary-scope Go source the module owns under cmd/
// named, read and declaring a package other than main; and, when the module
// root lists Go sources, at least one of them named, read and declaring a
// package other than main. A package clause is a positive fact, so a clipped
// file whose clause was read still counts.
func validateGoNoCommand(st *inspIndex, w Witness) error {
	gomod := w.Paths[0]
	if k, _ := manifestKindOf(gomod); k != kindGoMod || !st.complete(gomod) || goModulePath(st.files[gomod].Content) == "" {
		return fmt.Errorf("the no-command premise needs the module's go.mod read in full and parsed")
	}
	if !st.covers(w.Depth) {
		return fmt.Errorf("the listing does not cover cmd/<name>/, so a command there cannot be ruled out")
	}
	root := dirOf(gomod)
	var comp *component
	for _, c := range buildInventory(st) {
		if c.root == root {
			comp = c
		}
	}
	if comp == nil {
		return fmt.Errorf("no component is rooted at %s", gomod)
	}
	named := map[string]bool{}
	for _, p := range w.Paths[1:] {
		named[p] = true
	}
	rootListed, rootNamed := false, false
	for _, p := range comp.paths {
		rel := comp.rel(p)
		if !isGoSource(p) || scopeRank(rel) > 1 {
			continue
		}
		inCmd := strings.HasPrefix(strings.ToLower(rel), "cmd/")
		atRoot := dirOf(p) == root
		if atRoot {
			rootListed = true
		}
		if inCmd && !named[p] {
			return fmt.Errorf("the no-command premise omits %s under cmd/", p)
		}
		if named[p] {
			rootNamed = rootNamed || atRoot
			delete(named, p)
			if _, read := st.files[p]; !read {
				return fmt.Errorf("the no-command premise needs %s read, but %s", p, st.describe(p))
			}
			switch g := st.goFile(p); {
			case g.pkg == "":
				return fmt.Errorf("%s has no established package clause", p)
			case g.pkg == "main":
				return fmt.Errorf("%s declares package main", p)
			}
		}
	}
	if len(named) > 0 {
		return fmt.Errorf("the no-command premise names files outside the module's root and cmd/ sources")
	}
	if rootListed && !rootNamed {
		return fmt.Errorf("the no-command premise needs a read module-root package clause")
	}
	return nil
}
