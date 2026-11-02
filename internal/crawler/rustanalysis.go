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
		Edition  string `toml:"edition"`
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

// cargoAutobins / cargoAutolib report whether Cargo discovers binary / library
// targets from the file layout. Per the Cargo book (Target auto-discovery): an
// explicit autobins / autolib setting wins; otherwise discovery is on, except
// that in the 2015 edition — the edition Cargo assumes when none is declared —
// it is off as soon as any target is declared manually.
func cargoAutobins(m cargoManifest) bool {
	if m.Package == nil {
		return false // a virtual workspace has no root package to discover targets for
	}
	if m.Package != nil && m.Package.Autobins != nil {
		return *m.Package.Autobins
	}
	return cargoAutoDefault(m)
}

func cargoAutolib(m cargoManifest) bool {
	if m.Package == nil {
		return false
	}
	if m.Package != nil && m.Package.Autolib != nil {
		return *m.Package.Autolib
	}
	return cargoAutoDefault(m)
}

func cargoAutoDefault(m cargoManifest) bool {
	edition := ""
	if m.Package != nil {
		edition = m.Package.Edition
	}
	manual := m.Lib != nil || len(m.Bin) > 0
	return !(manual && (edition == "" || edition == "2015"))
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

// Cargo's implicit binary entries are direct files or main.rs in one named
// subdirectory, never arbitrary helper modules below src/bin/.
// https://doc.rust-lang.org/cargo/guide/project-layout.html
func cargoImplicitBin(p string) bool {
	parts := strings.Split(p, "/")
	return p == "src/main.rs" ||
		(len(parts) == 3 && parts[0] == "src" && parts[1] == "bin" && strings.HasSuffix(parts[2], ".rs")) ||
		(len(parts) == 4 && parts[0] == "src" && parts[1] == "bin" && parts[3] == "main.rs")
}

func cargoBinDiscoveryKnown(m cargoManifest, complete bool) bool {
	return complete || (m.Package != nil && m.Package.Autobins != nil)
}

func cargoLibDiscoveryKnown(m cargoManifest, complete bool) bool {
	return complete || (m.Package != nil && m.Package.Autolib != nil)
}
