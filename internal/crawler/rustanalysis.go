package crawler

import (
	"strings"

	toml "github.com/pelletier/go-toml/v2"
)

// This file parses Cargo.toml so Rust targets are recognised by Cargo's own
// conventions (the classification review, Slice 6). The old substring checks for
// "[[bin]]" / "[lib]" missed the implicit targets Cargo infers from the file
// layout — src/main.rs, src/bin/*.rs and src/lib.rs — so an ordinary binary or
// library crate fell through to "other".

// cargoManifest is the slice of Cargo.toml the classifier reads: the package
// autodiscovery switches, explicit target tables, and dependency names.
type cargoManifest struct {
	Package *struct {
		Name     string `toml:"name"`
		Autobins *bool  `toml:"autobins"`
		Autolib  *bool  `toml:"autolib"`
	} `toml:"package"`
	Lib               *struct{}      `toml:"lib"`
	Bin               []struct{}     `toml:"bin"`
	Dependencies      map[string]any `toml:"dependencies"`
	DevDependencies   map[string]any `toml:"dev-dependencies"`
	BuildDependencies map[string]any `toml:"build-dependencies"`
	Workspace         *struct {
		Members []string `toml:"members"`
	} `toml:"workspace"`
}

// parseCargo parses Cargo.toml; parsed is false when the TOML is malformed.
func parseCargo(content string) (m cargoManifest, parsed bool) {
	if err := toml.Unmarshal([]byte(content), &m); err != nil {
		return cargoManifest{}, false
	}
	return m, true
}

// cargoAutobins / cargoAutolib report whether Cargo's implicit target discovery
// is on (the default) — a package may disable it with autobins/autolib = false.
func cargoAutobins(m cargoManifest) bool {
	return m.Package == nil || m.Package.Autobins == nil || *m.Package.Autobins
}
func cargoAutolib(m cargoManifest) bool {
	return m.Package == nil || m.Package.Autolib == nil || *m.Package.Autolib
}

// cargoDep returns the declared name of the first of wants found in
// [dependencies] (an exact, case-insensitive crate-name match, not a substring),
// or "". Iterating wants in fixed order keeps the result independent of map order.
func cargoDep(m cargoManifest, wants []string) string {
	for _, w := range wants {
		for _, name := range sortedKeys(m.Dependencies) {
			if strings.EqualFold(name, w) {
				return name
			}
		}
	}
	return ""
}
