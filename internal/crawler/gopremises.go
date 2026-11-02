package crawler

import (
	"fmt"
	"sort"
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
