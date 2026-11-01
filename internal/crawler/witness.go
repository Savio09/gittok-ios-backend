package crawler

import (
	"encoding/json"
	"fmt"
	"path"
	"strings"

	toml "github.com/pelletier/go-toml/v2"
	"gopkg.in/yaml.v3"
)

// This file defines source witnesses and validates them against the inspection
// (the classification review, R8). A real path or a matching quote establishes
// provenance, not support: every witness is checked by re-deriving the claimed
// declaration or construct from the inspected bytes, and an observation whose
// witness fails is rejected before it can affect a verdict or reach the pack.

// WitnessKind says how an observation is tied to the inspection.
type WitnessKind string

const (
	// WitnessField: a field of a parsed manifest, addressed by its path segments
	// (["dependencies", "react"], ["require", "github.com/spf13/cobra"]) and, when
	// the field has a scalar value, that declared value.
	WitnessField WitnessKind = "field"
	// WitnessSpan: a byte range [Start, End) of an original file, holding the
	// named Construct (and Quote, when given, exactly).
	WitnessSpan WitnessKind = "span"
	// WitnessListing: a fact about the file tree, established by the listed
	// Paths and checked by the named Construct.
	WitnessListing WitnessKind = "listing"
	// WitnessScope: the basis of an absence premise — the Paths that must have
	// been read in full (and, for some Constructs, re-checked) for the premise.
	WitnessScope WitnessKind = "scope"
)

// Constructs a span, listing or scope witness may claim. Each is re-derived from
// the inspection by the validator.
const (
	constructGoPackageMain   = "go.package-main"    // span: the package clause naming main
	constructGoPackageName   = "go.package"         // span: a package clause (Quote holds the name)
	constructGoServerCall    = "go.server-call"     // span: a call resolved to a net/http server entry point
	constructDockerExpose    = "dockerfile.expose"  // span: an EXPOSE instruction
	constructNativeShells    = "native-shells"      // listing: android/ and ios/ directories
	constructIndexHTML       = "entry.index-html"   // listing: an index.html
	constructSrcApp          = "entry.src-app"      // listing: a src/App root component
	constructSrcMain         = "entry.src-main"     // listing: a src/main bootstrap
	constructAppConfig       = "entry.app-config"   // listing: a web-app build config
	constructRustBinTarget   = "cargo.implicit-bin" // listing: src/main.rs or src/bin/*.rs
	constructRustLibTarget   = "cargo.implicit-lib" // listing: src/lib.rs
	constructGoNoServer      = "go.no-server"       // scope: the command's package, read, parsed, no server call
	constructCompleteReads   = "complete-reads"     // scope: manifests read in full
	constructCoveringListing = "covering-listing"   // scope: complete reads plus a listing covering Depth segments
)

// Witness ties an observation to the inspection. Which fields are meaningful
// depends on Kind; see the constants above.
type Witness struct {
	Kind      WitnessKind
	Path      string   // the file a field or span witness reads
	Field     []string // field witness: path segments in the parsed manifest
	Value     string   // field witness: the declared scalar value, when it has one
	Start     int      // span witness: byte offsets in the original file
	End       int
	Quote     string   // span witness: optional; must equal the bytes at [Start, End)
	Construct string   // span/listing/scope witness: what the evidence must be
	Paths     []string // listing/scope witness: the paths it rests on
	Depth     int      // scope witness: the listing must cover paths this many segments deep
}

// String renders the witness for evidence and limitations.
func (w Witness) String() string {
	switch w.Kind {
	case WitnessField:
		return w.Path + " field " + strings.Join(w.Field, ".")
	case WitnessSpan:
		return fmt.Sprintf("%s bytes %d–%d (%s)", w.Path, w.Start, w.End, w.Construct)
	case WitnessListing, WitnessScope:
		return string(w.Kind) + " " + w.Construct + " over " + strings.Join(w.Paths, ", ")
	}
	return "no witness"
}

// validateWitness checks an observation's witness against the inspection and
// returns nil only when the inspected content supports the claimed declaration
// or construct. A matching path or quote alone is never enough.
func validateWitness(st *inspIndex, w Witness) error {
	switch w.Kind {
	case WitnessField:
		return validateField(st, w)
	case WitnessSpan:
		return validateSpan(st, w)
	case WitnessListing:
		return validateListing(st, w)
	case WitnessScope:
		return validateScope(st, w)
	case "":
		return fmt.Errorf("the observation carries no witness")
	}
	return fmt.Errorf("unknown witness kind %q", w.Kind)
}

// consumed returns the bytes of p the analyzer actually read.
func consumed(st *inspIndex, p string) (string, error) {
	f, ok := st.files[p]
	if !ok {
		s, _ := st.state(p)
		return "", fmt.Errorf("%s was not read (%s)", p, s)
	}
	return f.Content, nil
}

func validateField(st *inspIndex, w Witness) error {
	if len(w.Field) == 0 {
		return fmt.Errorf("field witness on %s names no field", w.Path)
	}
	content, err := consumed(st, w.Path)
	if err != nil {
		return err
	}
	tree, err := manifestTree(w.Path, content)
	if err != nil {
		return fmt.Errorf("%s could not be parsed: %v", w.Path, err)
	}
	got, ok := resolveField(tree, w.Field)
	if !ok {
		return fmt.Errorf("%s has no field %s", w.Path, strings.Join(w.Field, "."))
	}
	if w.Value != "" && got != w.Value {
		return fmt.Errorf("%s field %s is %q, not %q", w.Path, strings.Join(w.Field, "."), got, w.Value)
	}
	return nil
}

func validateSpan(st *inspIndex, w Witness) error {
	if w.Construct == "" {
		return fmt.Errorf("span witness on %s names no construct", w.Path)
	}
	if w.Start == 0 && w.End == 0 {
		return fmt.Errorf("the %s witness on %s omits its required source span", w.Construct, w.Path)
	}
	content, err := consumed(st, w.Path)
	if err != nil {
		return err
	}
	if w.Start < 0 || w.End <= w.Start || w.End > len(content) {
		return fmt.Errorf("span %d–%d lies outside the %d bytes of %s that were read", w.Start, w.End, len(content), w.Path)
	}
	if w.Quote != "" && content[w.Start:w.End] != w.Quote {
		return fmt.Errorf("the quote does not match %s bytes %d–%d", w.Path, w.Start, w.End)
	}
	switch w.Construct {
	case constructGoPackageMain, constructGoPackageName:
		g := analyzeGoFile(st.files[w.Path])
		if g.pkgStart != w.Start || g.pkgEnd != w.End || (w.Construct == constructGoPackageMain && g.pkg != "main") {
			return fmt.Errorf("%s bytes %d–%d are not its package clause %q", w.Path, w.Start, w.End, w.Construct)
		}
	case constructGoServerCall:
		for _, c := range analyzeGoFile(st.files[w.Path]).serverCalls {
			if c.start == w.Start && c.end == w.End {
				return nil
			}
		}
		return fmt.Errorf("%s bytes %d–%d hold no call resolved to a net/http server entry point", w.Path, w.Start, w.End)
	case constructDockerExpose:
		fields := strings.Fields(content[w.Start:w.End])
		if len(fields) < 2 || !strings.EqualFold(fields[0], "EXPOSE") {
			return fmt.Errorf("%s bytes %d–%d are not an EXPOSE instruction", w.Path, w.Start, w.End)
		}
	default:
		return fmt.Errorf("unknown span construct %q", w.Construct)
	}
	return nil
}

func validateListing(st *inspIndex, w Witness) error {
	if len(w.Paths) == 0 {
		return fmt.Errorf("listing witness %s names no paths", w.Construct)
	}
	for _, p := range w.Paths {
		if s, _ := st.state(p); s == StateNotListed {
			return fmt.Errorf("%s is not in the listing", p)
		}
	}
	lower := make([]string, len(w.Paths))
	for i, p := range w.Paths {
		lower[i] = strings.ToLower(p)
	}
	some := func(pred func(string) bool) bool {
		for _, l := range lower {
			if pred(l) {
				return true
			}
		}
		return false
	}
	var ok bool
	switch w.Construct {
	case constructNativeShells:
		ok = some(func(l string) bool { return strings.Contains("/"+l, "/android/") }) &&
			some(func(l string) bool { return strings.Contains("/"+l, "/ios/") })
	case constructIndexHTML:
		ok = some(func(l string) bool { return path.Base(l) == "index.html" })
	case constructSrcApp:
		ok = some(func(l string) bool { return strings.HasPrefix(path.Base(l), "app.") && path.Base(path.Dir(l)) == "src" })
	case constructSrcMain:
		ok = some(func(l string) bool {
			return strings.HasPrefix(path.Base(l), "main.") && path.Base(path.Dir(l)) == "src"
		})
	case constructAppConfig:
		ok = some(isAppConfig)
	case constructRustBinTarget, constructRustLibTarget:
		return validateImplicitCargoTarget(st, w)
	default:
		return fmt.Errorf("unknown listing construct %q", w.Construct)
	}
	if !ok {
		return fmt.Errorf("the listed paths %v do not establish %s", w.Paths, w.Construct)
	}
	return nil
}

func validateImplicitCargoTarget(st *inspIndex, w Witness) error {
	if kind, _ := manifestKindOf(w.Path); kind != kindCargo {
		return fmt.Errorf("an implicit Cargo target needs its owning Cargo.toml")
	}
	content, err := consumed(st, w.Path)
	if err != nil {
		return err
	}
	m, parsed := parseCargo(content)
	if !parsed {
		return fmt.Errorf("the owning manifest %s could not be parsed", w.Path)
	}
	bin := w.Construct == constructRustBinTarget
	if (bin && (!cargoBinDiscoveryKnown(m, st.complete(w.Path)) || !cargoAutobins(m))) ||
		(!bin && (!cargoLibDiscoveryKnown(m, st.complete(w.Path)) || !cargoAutolib(m))) {
		return fmt.Errorf("%s does not establish enabled target autodiscovery", w.Path)
	}
	root := dirOf(w.Path)
	for _, p := range w.Paths {
		rel := p
		if root != "" {
			if !strings.HasPrefix(p, root+"/") {
				return fmt.Errorf("%s is outside the owning crate %s", p, w.Path)
			}
			rel = strings.TrimPrefix(p, root+"/")
		}
		if (bin && !cargoImplicitBin(rel)) || (!bin && rel != "src/lib.rs") {
			return fmt.Errorf("%s is not the claimed implicit target of %s", p, w.Path)
		}
	}
	return nil
}

func validateScope(st *inspIndex, w Witness) error {
	if len(w.Paths) == 0 {
		return fmt.Errorf("scope witness %s names no paths", w.Construct)
	}
	for _, p := range w.Paths {
		if !st.complete(p) {
			return fmt.Errorf("the premise needs %s read in full, but %s", p, st.describe(p))
		}
	}
	switch w.Construct {
	case constructCompleteReads:
		return nil
	case constructCoveringListing:
		if !st.covers(w.Depth) {
			return fmt.Errorf("the premise needs a listing that covers paths %d segments deep, which this listing does not", w.Depth)
		}
		return nil
	case constructGoNoServer:
		module := ""
		for _, p := range w.Paths {
			if k, _ := manifestKindOf(p); k == kindGoMod {
				module = goModulePath(st.files[p].Content)
			}
		}
		if module == "" {
			return fmt.Errorf("the no-server premise needs the module's go.mod read in full and parsed")
		}
		for _, p := range w.Paths {
			if !isGoSource(p) {
				continue // go.mod belongs to the premise's basis but holds no calls
			}
			g := analyzeGoFile(st.files[p])
			for _, im := range g.imports {
				if im == module || strings.HasPrefix(im, module+"/") {
					return fmt.Errorf("%s imports %s from its own module, which the premise does not cover", p, im)
				}
			}
			switch {
			case !g.parsed:
				return fmt.Errorf("%s does not parse, so it cannot show the absence of a server call", p)
			case len(g.serverCalls) > 0:
				return fmt.Errorf("%s contains a server call at line %d", p, g.serverCalls[0].line)
			case len(g.uncertain) > 0:
				return fmt.Errorf("%s has a server-like call of unestablished identity", p)
			}
		}
		return validateGoPackageListing(st, w.Paths)
	}
	return fmt.Errorf("unknown scope construct %q", w.Construct)
}

// manifestTree parses a manifest into a generic tree for field resolution. go.mod
// and requirements.txt, which are not tree formats, are mapped onto
// {"module": path, "require": {path: version}} and {"requirements": {name: spec}}.
func manifestTree(p, content string) (map[string]any, error) {
	tree := map[string]any{}
	switch k, _ := manifestKindOf(p); k {
	case kindNPM:
		err := json.Unmarshal([]byte(content), &tree)
		return tree, err
	case kindCargo, kindPyProject:
		err := toml.Unmarshal([]byte(content), &tree)
		return tree, err
	case kindPubspec:
		err := yaml.Unmarshal([]byte(content), &tree)
		return tree, err
	case kindGoMod:
		f, err := parseGoModSource(p, content)
		if err != nil {
			return nil, err
		}
		req := map[string]any{}
		for _, r := range f.Require {
			req[r.Mod.Path] = r.Mod.Version
		}
		tree["require"] = req
		if f.Module != nil {
			tree["module"] = f.Module.Mod.Path
		}
		return tree, nil
	case kindRequirements:
		specs, _ := parseRequirements(content)
		req := map[string]any{}
		for _, s := range specs {
			req[pyDistName(s)] = s
		}
		tree["requirements"] = req
		return tree, nil
	}
	return nil, fmt.Errorf("%s is not a manifest format with fields", p)
}

// resolveField walks a parsed manifest by path segments. Inside an array of
// PEP 508 strings (Python dependency lists) a segment selects the element whose
// distribution name it is; inside an array of tables (Cargo's [[bin]]) the
// array's presence is the field. Scalars resolve to their string form;
// composites resolve to "" (present, no scalar value).
func resolveField(tree map[string]any, segs []string) (string, bool) {
	var node any = tree
	for _, seg := range segs {
		switch n := node.(type) {
		case map[string]any:
			v, ok := n[seg]
			if !ok {
				return "", false
			}
			node = v
		case []any:
			found := false
			for _, el := range n {
				if s, ok := el.(string); ok && pyDistName(s) == pyDistName(seg) {
					node, found = s, true
					break
				}
			}
			if !found {
				return "", false
			}
		default:
			return "", false
		}
	}
	switch v := node.(type) {
	case string:
		return v, true
	case bool, int, int64, float64:
		return fmt.Sprint(v), true
	case nil:
		return "", false
	}
	return "", true
}
