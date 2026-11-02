package crawler

import (
	"path"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// This file builds the component inventory: which manifests, sources and
// dependency declarations belong together. The classification review (R2)
// showed one-manifest-per-basename selection discarding independent components
// and then mixing a root manifest's dependencies with a nested app's entry. The
// inventory keeps every manifest, keyed by the component that owns it, so later
// steps can reason about each component before deciding the repository's role.
//
// # Supported boundaries
//
// A component is the repository root, or a directory holding a component-
// defining manifest: package.json, go.mod, Cargo.toml, pyproject.toml, setup.py
// or pubspec.yaml. Each path belongs to the deepest component whose directory
// contains it. requirements.txt, go.work and Dockerfiles do not start a component;
// they belong to the component that owns their directory.
//
// Not modelled (documented boundary cases): Bazel/Pants/Nx project files,
// Gradle/Maven modules, Python packages without packaging metadata, and Go
// packages that live under a directory with a different manifest kind. Those
// paths are owned by the nearest supported component and treated as its files.

// manifestKind is a manifest format the inventory recognises.
type manifestKind string

const (
	kindNPM          manifestKind = "package.json"
	kindGoMod        manifestKind = "go.mod"
	kindGoWork       manifestKind = "go.work"
	kindCargo        manifestKind = "Cargo.toml"
	kindPyProject    manifestKind = "pyproject.toml"
	kindSetupPy      manifestKind = "setup.py"
	kindRequirements manifestKind = "requirements.txt"
	kindPubspec      manifestKind = "pubspec.yaml"
	kindDocker       manifestKind = "Dockerfile"
)

// manifestKindOf returns the manifest kind a path's base name denotes.
func manifestKindOf(p string) (manifestKind, bool) {
	switch strings.ToLower(path.Base(p)) {
	case "package.json":
		return kindNPM, true
	case "go.mod":
		return kindGoMod, true
	case "go.work":
		return kindGoWork, true
	case "cargo.toml":
		return kindCargo, true
	case "pyproject.toml":
		return kindPyProject, true
	case "setup.py":
		return kindSetupPy, true
	case "requirements.txt":
		return kindRequirements, true
	case "pubspec.yaml", "pubspec.yml":
		return kindPubspec, true
	case "dockerfile":
		return kindDocker, true
	}
	return "", false
}

// definesComponent reports whether a manifest of this kind declares its own
// component (a package, module or crate) rather than describing its directory's.
func (k manifestKind) definesComponent() bool {
	switch k {
	case kindNPM, kindGoMod, kindCargo, kindPyProject, kindSetupPy, kindPubspec:
		return true
	}
	return false
}

// depDecl is one dependency a manifest declares, keeping the table it was
// declared in so a role decision can tell a runtime dependency from a
// development, peer, build or optional one.
type depDecl struct {
	Name     string // as declared; Python names are PEP 503-normalised
	Spec     string // version range or requirement specifier, when declared
	Category string // the declaring table, e.g. "devDependencies", "project.optional-dependencies.docs"
	Field    string // dotted manifest field path, e.g. "devDependencies.express"
	Source   string // the manifest's path
}

// component is one independently declared unit of the repository.
type component struct {
	root      string        // "" for the repository root; otherwise the directory, original case
	manifests []manifestRef // every manifest the component owns, path-sorted, with read state
	paths     []string      // every listed, read or failed path it owns, path-sorted
	deps      []depDecl     // declarations parsed from its readable manifests, in manifest order
}

// owns reports whether p lies inside the component's directory.
func (c *component) owns(p string) bool {
	return c.root == "" || p == c.root || strings.HasPrefix(p, c.root+"/")
}

// rel returns p relative to the component's directory.
func (c *component) rel(p string) string {
	if c.root == "" {
		return p
	}
	return strings.TrimPrefix(p, c.root+"/")
}

// manifest returns the component's own manifest of the given kind: the one in
// its directory if present, otherwise the shallowest primary-scope one it owns.
func (c *component) manifest(k manifestKind) manifestRef {
	var best manifestRef
	for _, m := range c.manifests {
		if m.kind != k || scopeRank(c.rel(m.path)) > 1 {
			continue
		}
		if !best.present() || strings.Count(m.path, "/") < strings.Count(best.path, "/") {
			best = m
		}
	}
	return best
}

// buildInventory partitions every known path — listed, read or failed — into
// components. The result is sorted by component root ("" first) and each
// component's paths, manifests and declarations are in a fixed order, so the
// same inspection yields the same inventory however it was constructed.
func buildInventory(x *inspIndex) []*component {
	known := map[string]bool{}
	for _, p := range x.paths {
		known[p] = true
	}
	for p := range x.files {
		known[p] = true
	}
	for p := range x.unavail {
		known[p] = true
	}
	all := make([]string, 0, len(known))
	for p := range known {
		all = append(all, p)
	}
	sort.Strings(all)

	roots := map[string]bool{"": true}
	for _, p := range all {
		if k, ok := manifestKindOf(p); ok && k.definesComponent() {
			roots[dirOf(p)] = true
		}
	}
	comps := make([]*component, 0, len(roots))
	for r := range roots {
		comps = append(comps, &component{root: r})
	}
	sort.Slice(comps, func(i, j int) bool { return comps[i].root < comps[j].root })

	// Deepest owner first, so a nested component claims its own subtree.
	byDepth := append([]*component(nil), comps...)
	sort.SliceStable(byDepth, func(i, j int) bool { return len(byDepth[i].root) > len(byDepth[j].root) })
	for _, p := range all {
		for _, c := range byDepth {
			if !c.owns(p) {
				continue
			}
			c.paths = append(c.paths, p)
			if k, ok := manifestKindOf(p); ok {
				state, reason := x.state(p)
				m := manifestRef{path: p, kind: k, state: state, reason: reason}
				if f, ok := x.files[p]; ok {
					m.content = f.Content
				}
				c.manifests = append(c.manifests, m)
			}
			break
		}
	}
	for _, c := range comps {
		for _, m := range c.manifests {
			if m.readable() {
				c.deps = append(c.deps, manifestDeps(m)...)
			}
		}
	}
	return comps
}

// dirOf is path.Dir with the repository root spelled "".
func dirOf(p string) string {
	if d := path.Dir(p); d != "." {
		return d
	}
	return ""
}

// manifestDeps parses the dependency declarations a readable manifest makes. A
// manifest that will not parse yields none; its parse failure is reported by
// the classifier, not hidden here. setup.py and Dockerfiles declare nothing the
// inventory can read without executing or interpreting them.
func manifestDeps(m manifestRef) []depDecl {
	var out []depDecl
	add := func(category, name, spec string) {
		out = append(out, depDecl{Name: name, Spec: spec, Category: category, Field: category + "." + name, Source: m.path})
	}
	switch m.kind {
	case kindNPM:
		if p := parsePackageJSON(m.content); p != nil {
			for _, t := range []struct {
				cat depCategory
				m   map[string]string
			}{{depRuntime, p.Dependencies}, {depDev, p.DevDependencies}, {depPeer, p.PeerDependencies}} {
				for _, n := range sortedKeys(t.m) {
					add(string(t.cat), n, t.m[n])
				}
			}
		}
	case kindGoMod:
		if f, err := parseGoModSource(m.path, m.content); err == nil {
			for _, r := range f.Require {
				cat := "require"
				if r.Indirect {
					cat = "require (indirect)"
				}
				add(cat, r.Mod.Path, r.Mod.Version)
			}
		}
	case kindCargo:
		if c, ok := parseCargo(m.content); ok {
			for _, t := range []struct {
				cat string
				m   map[string]any
			}{{"dependencies", c.Dependencies}, {"dev-dependencies", c.DevDependencies}, {"build-dependencies", c.BuildDependencies}} {
				for _, n := range sortedKeys(t.m) {
					add(t.cat, n, tomlSpec(t.m[n]))
				}
			}
		}
	case kindPyProject:
		if p, _, ok := parsePyProject(m.content); ok && p.Project != nil {
			for _, spec := range p.Project.Dependencies {
				if n := pyDistName(spec); n != "" {
					add("project.dependencies", n, spec)
				}
			}
			for _, g := range sortedKeys(p.Project.OptionalDependencies) {
				for _, spec := range p.Project.OptionalDependencies[g] {
					if n := pyDistName(spec); n != "" {
						add("project.optional-dependencies."+g, n, spec)
					}
				}
			}
		}
	case kindRequirements:
		for _, line := range pyRequirementLines(m.content) {
			add("requirements", pyDistName(line), line)
		}
	case kindPubspec:
		var p pubspec
		if yaml.Unmarshal([]byte(m.content), &p) == nil {
			for _, n := range sortedKeys(p.Dependencies) {
				add("dependencies", n, "")
			}
			for _, n := range sortedKeys(p.DevDependencies) {
				add("dev_dependencies", n, "")
			}
		}
	}
	return out
}

// tomlSpec renders a Cargo dependency value: the version string, or the table's
// "version" key; "" when neither is declared (e.g. a path or git dependency).
func tomlSpec(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case map[string]any:
		if s, ok := t["version"].(string); ok {
			return s
		}
	}
	return ""
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
