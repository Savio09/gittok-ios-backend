package crawler

import (
	"encoding/json"
	"path"
	"sort"
	"strings"

	"github.com/Savio09/gittok-ios-backend/internal/apimodel"
)

// Classify infers a repository's RepoKind from already-gathered evidence. It is
// a pure, network-free function so it can be unit-tested with synthetic inputs:
// the caller assembles an Inspection (the file tree, the files it managed to
// read, and the scope that bounded the fetch), and Classify reasons only over
// that. It never calls the GitHub API.
//
// It returns a structured Classification (see evidence.go): the Kind, a Status
// that is honest about abstention, the Observations that produced the verdict
// (each tied to the actual source path it was read from), and the Limitations
// that bounded the decision. The cardinal rule from the classification review is
// that a *missing* signal is never silently a negative fact: if the tree was
// truncated or an entry point's source was not read, that is recorded as a
// limitation and the verdict abstains rather than guessing.
//
// # Precedence (#algorithms)
//
// A repo often carries several signals at once (a Next.js app also runs a Node
// server; a Go server also lives under cmd/). Classify picks the dominant kind
// by running detectors in a fixed order and returning the first match:
//
//		mobile → frontend → backend → cli → library → other
//
//	  - mobile is first because react-native/expo apps also depend on react, so
//	    they would otherwise be mistaken for a web frontend.
//	  - frontend precedes backend so a client app that happens to ship a
//	    Dockerfile is labelled by its user-facing surface, not its container.
//	  - backend precedes cli so a server that exposes subcommands under cmd/ is
//	    still a backend; cli is reserved for tools with no server signal.
//	  - library is last among the positives: pure importable code — a Go module
//	    with no package main (e.g. gin-gonic/gin), an npm package with a lib entry
//	    and no app — falls through to here.
//
// When no detector finds decisive, inspected evidence, Classify returns
// KindOther with VerdictInconclusive and the limitations that explain why — an
// honest abstention, not a confident "other".
func Classify(insp Inspection) Classification {
	ix := newRepoIndex(insp)
	limits := ix.limitations()

	// Run every detector and keep each hit as a candidate tagged with the
	// component scope of its evidence, rather than taking the first match. The
	// detectors are listed in precedence order (the tie-breaker among candidates
	// in the same scope).
	detectors := []func(*repoIndex) (apimodel.RepoKind, []Observation, bool){
		detectMobile,
		detectFrontend,
		detectBackend,
		detectCLI,
		detectLibrary,
	}
	type candidate struct {
		kind  apimodel.RepoKind
		obs   []Observation
		scope int
	}
	var candidates []candidate
	for _, detect := range detectors {
		if kind, obs, ok := detect(ix); ok {
			candidates = append(candidates, candidate{kind: kind, obs: obs, scope: candidateScope(obs)})
		}
	}
	if len(candidates) == 0 {
		limits = append(limits, "no decisive frontend / backend / library / CLI / mobile signal was found in the inspected manifests and file tree")
		return Classification{Kind: apimodel.KindOther, Status: VerdictInconclusive, Limitations: limits}
	}

	// The primary product is whatever has evidence in the most primary component
	// scope (lowest rank). A signal from a less primary component — an admin UI
	// under ui/, a server under examples/ — cannot override it; it is recorded as
	// a secondary component instead. Detectors already ran in precedence order,
	// so the first candidate at the primary scope wins the tie.
	primaryScope := candidates[0].scope
	for _, c := range candidates[1:] {
		if c.scope < primaryScope {
			primaryScope = c.scope
		}
	}
	chosen := 0
	for i := range candidates {
		if candidates[i].scope == primaryScope {
			chosen = i
			break
		}
	}

	// Abstain when a Go entry point at least as primary as the chosen candidate
	// was never inspected: it could itself be the primary product (a backend
	// whose decisive source we did not read), so picking a less-primary signal
	// such as an admin UI would be a guess.
	if unread := ix.unreadGoEntries(); len(unread) > 0 && primaryScope >= minScopeOfPaths(unread) {
		for _, c := range candidates {
			for _, o := range c.obs {
				if o.Path != "" {
					limits = append(limits, "candidate ("+string(c.kind)+"): "+o.Path+": "+o.Detail)
				}
			}
		}
		limits = append(limits, "an uninspected Go entry point could be the primary product; verdict withheld")
		return Classification{Kind: apimodel.KindOther, Status: VerdictInconclusive, Limitations: limits}
	}

	for i := range candidates {
		if i == chosen {
			continue
		}
		for _, o := range candidates[i].obs {
			if o.Path != "" {
				limits = append(limits, "secondary component ("+string(candidates[i].kind)+"): "+o.Path+": "+o.Detail)
			}
		}
	}
	return Classification{
		Kind:         candidates[chosen].kind,
		Status:       VerdictResolved,
		Observations: candidates[chosen].obs,
		Limitations:  limits,
	}
}

// minScopeOfPaths is the most primary (lowest) scope among the given paths, or 0
// when there are none.
func minScopeOfPaths(paths []string) int {
	best := -1
	for _, p := range paths {
		if s := scopeRank(p); best == -1 || s < best {
			best = s
		}
	}
	if best == -1 {
		return 0
	}
	return best
}

// candidateScope is the component scope a candidate speaks for: the most primary
// (lowest) scope among its evidence paths. Whole-tree structural observations
// (no path) do not pull the scope toward root, so a nested component's manifest
// still reads as nested; a candidate with only structural evidence is treated as
// primary (scope 0).
func candidateScope(obs []Observation) int {
	best := -1
	for _, o := range obs {
		if o.Path == "" {
			continue
		}
		if s := scopeRank(o.Path); best == -1 || s < best {
			best = s
		}
	}
	if best == -1 {
		return 0
	}
	return best
}

// repoIndex is the pre-parsed, cheap-to-query view of an Inspection. Parsing the
// manifests once up front keeps the detectors readable. Each manifest is chosen
// deterministically (shallowest path, then lexical) and its path is retained so
// an observation can cite the file it actually came from.
type repoIndex struct {
	language  string   // lowercased primary language
	paths     []string // original tree paths
	lowerPath []string // lowercased tree paths, index-aligned with paths

	scope       TreeScope
	unavailable []Unavailable

	pkg         *packageJSON // parsed root-most package.json, or nil
	pkgPath     string       // path the package.json was read from
	pkgBad      bool         // a package.json was present but would not parse
	goMod       string
	goModPath   string
	cargo       string
	cargoPath   string
	pyproject   string
	pyprojPath  string
	setupPy     string
	setupPyPath string
	pubspec     string
	pubspecPath string
	dockerfile  string
	dockerPath  string
	goFiles     []SourceFile // Go sources whose content was read, with their paths

	requirements     string // requirements.txt content, or ""
	requirementsPath string
	pyProj           pyProject         // parsed pyproject.toml (Project nil if absent)
	pyHasProject     bool              // a [project] table was present
	pyBad            bool              // a pyproject.toml was present but would not parse
	pyDepSource      map[string]string // normalised python dep name -> the file it was declared in

	cargoM   cargoManifest // parsed Cargo.toml
	cargoOK  bool          // Cargo.toml parsed
	cargoBad bool          // a Cargo.toml was present but would not parse
}

func newRepoIndex(insp Inspection) *repoIndex {
	ix := &repoIndex{
		language:    strings.ToLower(strings.TrimSpace(insp.Language)),
		paths:       insp.Paths,
		lowerPath:   make([]string, len(insp.Paths)),
		scope:       insp.Scope,
		unavailable: insp.Unavailable,
	}
	for i, p := range insp.Paths {
		ix.lowerPath[i] = strings.ToLower(p)
	}

	// Choose each manifest deterministically from the read files: the shallowest
	// path wins (root is preferred), ties broken lexically. Files are already
	// path-sorted by the Inspection constructors, so this is stable regardless of
	// how the content was gathered.
	if c, p, ok := pickPrimary(insp.Files, "package.json"); ok {
		ix.pkgPath = p
		if ix.pkg = parsePackageJSON(c); ix.pkg == nil {
			ix.pkgBad = true
		}
	}
	ix.goMod, ix.goModPath = pick(insp.Files, "go.mod")
	ix.cargo, ix.cargoPath = pick(insp.Files, "cargo.toml")
	ix.pyproject, ix.pyprojPath = pick(insp.Files, "pyproject.toml")
	ix.setupPy, ix.setupPyPath = pick(insp.Files, "setup.py")
	if c, p, ok := pickPrimary(insp.Files, "pubspec.yaml"); ok {
		ix.pubspec, ix.pubspecPath = c, p
	} else if c, p, ok := pickPrimary(insp.Files, "pubspec.yml"); ok {
		ix.pubspec, ix.pubspecPath = c, p
	}
	ix.dockerfile, ix.dockerPath = pick(insp.Files, "dockerfile")
	ix.requirements, ix.requirementsPath = pick(insp.Files, "requirements.txt")

	// Parse Python declarations once. setup.py is deliberately not parsed for
	// dependencies — its install_requires is dynamic Python we never execute.
	if ix.pyproject != "" {
		var parsed bool
		ix.pyProj, ix.pyHasProject, parsed = parsePyProject(ix.pyproject)
		ix.pyBad = !parsed
	}
	ix.pyDepSource = buildPyDepSource(ix.pyProj, ix.pyprojPath, ix.requirements, ix.requirementsPath)

	if ix.cargo != "" {
		ix.cargoM, ix.cargoOK = parseCargo(ix.cargo)
		ix.cargoBad = !ix.cargoOK
	}

	for _, f := range insp.Files {
		if strings.HasSuffix(strings.ToLower(f.Path), ".go") {
			ix.goFiles = append(ix.goFiles, f)
		}
	}
	return ix
}

// buildPyDepSource maps each normalised Python dependency name to the file it
// was declared in (pyproject.toml preferred over requirements.txt), so a server
// dependency can be both detected and cited to its actual source.
func buildPyDepSource(proj pyProject, projPath, reqs, reqsPath string) map[string]string {
	m := map[string]string{}
	for _, n := range pyProjectDependencyNames(proj) {
		if _, ok := m[n]; !ok {
			m[n] = projPath
		}
	}
	for _, n := range pyRequirementNames(reqs) {
		if _, ok := m[n]; !ok {
			m[n] = reqsPath
		}
	}
	return m
}

// pick returns the primary content and path for a manifest base name.
func pick(files []SourceFile, base string) (content, srcPath string) {
	c, p, _ := pickPrimary(files, base)
	return c, p
}

// pickPrimary returns the content and path of the manifest that best represents
// the repository's primary product among all files whose base name matches
// (case-insensitively). Selection is deterministic and order-independent: it
// prefers the manifest in the most primary component scope (root over a nested
// component, and both over an example/docs/tooling scope), then the shallowest
// path, then the lexically first. This keeps a nested example's manifest from
// supplying the primary signal while still choosing it when it is the only one.
func pickPrimary(files []SourceFile, base string) (content, srcPath string, ok bool) {
	base = strings.ToLower(base)
	bestScope, bestDepth := 0, 0
	for _, f := range files {
		if strings.ToLower(path.Base(f.Path)) != base {
			continue
		}
		s := scopeRank(f.Path)
		d := strings.Count(f.Path, "/")
		if !ok || s < bestScope ||
			(s == bestScope && d < bestDepth) ||
			(s == bestScope && d == bestDepth && f.Path < srcPath) {
			bestScope, bestDepth, content, srcPath, ok = s, d, f.Content, f.Path, true
		}
	}
	return content, srcPath, ok
}

// componentScope is a coarse, directory-derived guess at a manifest's role in
// the repository, used only to prefer the primary product's manifest over a
// manifest that clearly belongs to an example, the docs site or tooling. Folder
// names are hints, not a full component model — the deeper role arbitration
// lives in the role-resolution step, not here.
func scopeRank(p string) int {
	if !strings.Contains(p, "/") {
		return 0 // root: the primary product's manifest usually sits here
	}
	first := strings.ToLower(p[:strings.IndexByte(p, '/')])
	switch first {
	case "examples", "example", "demo", "demos", "sample", "samples", "fixtures", "testdata":
		return 4
	case "docs", "doc", "website", "www", "site":
		return 3
	case "tools", "tool", "scripts", "script", ".github", "build", "ci":
		return 2
	default:
		return 1 // some other subdirectory / component
	}
}

// limitations gathers the honest caveats on any verdict: manifests that would
// not parse, a bounded listing, and Go entry points that were present in the
// tree but whose source was never read (so a server could not be told from a
// command). These are what keep a missing signal from becoming a negative fact.
func (ix *repoIndex) limitations() []string {
	var out []string
	if ix.pkgBad {
		out = append(out, "package.json at "+ix.pkgPath+" could not be parsed; its signals were not used")
	}
	if ix.pyBad {
		out = append(out, "pyproject.toml at "+ix.pyprojPath+" could not be parsed; its signals were not used")
	}
	if ix.cargoBad {
		out = append(out, "Cargo.toml at "+ix.cargoPath+" could not be parsed; its signals were not used")
	}
	if ix.scope.Truncated {
		out = append(out, "the file listing was truncated; a signal absent from it cannot be treated as absent from the repository")
	}
	if ix.scope.DepthLimited {
		out = append(out, "the file listing was depth-limited; files below the depth cap were not listed, so a deeper entry point may exist")
	}
	if unread := ix.unreadGoEntries(); len(unread) > 0 {
		out = append(out, "Go entry point(s) present but their source was not inspected ("+strings.Join(unread, ", ")+"); a server could not be distinguished from a command")
	}
	return out
}

// unreadGoEntries lists main.go-style entry paths that appear in the tree but
// whose content was not read. It only reports when the repo looks like Go, so
// an unrelated main.go elsewhere does not raise a spurious caveat.
func (ix *repoIndex) unreadGoEntries() []string {
	if !ix.isGo() {
		return nil
	}
	read := map[string]bool{}
	for _, f := range ix.goFiles {
		read[f.Path] = true
	}
	var unread []string
	for _, p := range ix.paths {
		if strings.ToLower(path.Base(p)) == "main.go" && scopeRank(p) <= 1 && !read[p] {
			unread = append(unread, p)
		}
	}
	sort.Strings(unread)
	return unread
}

// hasBase reports whether any tree path has the given (lowercased) base name.
func (ix *repoIndex) hasBase(name string) bool {
	name = strings.ToLower(name)
	for _, p := range ix.lowerPath {
		if path.Base(p) == name {
			return true
		}
	}
	return false
}

// hasPrefix reports whether any tree path starts with the given (lowercased)
// prefix — used for directory ("android/") and stem ("src/app.") checks.
func (ix *repoIndex) hasPrefix(prefix string) bool {
	prefix = strings.ToLower(prefix)
	for _, p := range ix.lowerPath {
		if strings.HasPrefix(p, prefix) {
			return true
		}
	}
	return false
}

// --- mobile -----------------------------------------------------------------

func detectMobile(ix *repoIndex) (apimodel.RepoKind, []Observation, bool) {
	var ev []Observation
	if ix.pkg != nil {
		for _, dep := range []string{"react-native", "expo"} {
			if v := ix.pkg.dep(dep); v != "" {
				ev = append(ev, Observation{Path: ix.pkgPath, Detail: "depends on " + quoteDep(dep, v) + " (mobile framework)"})
			}
		}
	}
	// Flutter projects declare a Flutter SDK dependency in pubspec.yaml — checked
	// from the parsed dependency tables, so the word "flutter" in a comment does
	// not count.
	if ix.pubspec != "" && pubspecUsesFlutter(ix.pubspec) {
		ev = append(ev, Observation{Path: ix.pubspecPath, Detail: "declares a Flutter SDK dependency (mobile)"})
	}
	// Native shells side by side are a strong mobile signal on their own.
	if ix.hasPrefix("android/") && ix.hasPrefix("ios/") {
		ev = append(ev, Observation{Detail: "android/ and ios/ app directories both present (native mobile shells)"})
	}
	if len(ev) > 0 {
		return apimodel.KindMobile, ev, true
	}
	return "", nil, false
}

// --- frontend ----------------------------------------------------------------

// webFrameworks are the dependencies that mark a web-app (not a published lib).
var webFrameworks = []string{
	"react", "react-dom", "vue", "svelte", "@sveltejs/kit", "@angular/core",
	"next", "nuxt", "vite", "solid-js", "preact", "gatsby", "astro",
}

func detectFrontend(ix *repoIndex) (apimodel.RepoKind, []Observation, bool) {
	if ix.pkg == nil {
		return "", nil, false
	}
	// A web framework establishes an application only as a *runtime* dependency.
	// A build tool in devDependencies (vite) or a peer dependency (a UI
	// library's react) does not make the package a frontend product.
	var framework, frameworkVer string
	for _, f := range webFrameworks {
		if v, cat, ok := ix.pkg.lookupDep(f); ok && cat == depRuntime {
			framework, frameworkVer = f, v
			break
		}
	}
	if framework == "" {
		return "", nil, false
	}
	// The app entry must live in the primary component, so a nested example or
	// demo does not turn a library into a frontend. A dev/start script alone is
	// not an app entry. Without a primary app entry, defer to detectLibrary.
	entry, ok := ix.frontendEntry()
	if !ok {
		return "", nil, false
	}

	ev := []Observation{{Path: ix.pkgPath, Detail: "depends on " + quoteDep(framework, frameworkVer) + " (web framework)"}, entry}
	if ix.pkg.Private {
		ev = append(ev, Observation{Path: ix.pkgPath, Detail: `"private": true (not a published package)`})
	}
	return apimodel.KindFrontend, ev, true
}

// frontendEntry finds a primary-component app entry: an index.html, a src/App
// root component, a src/main bootstrap, or an app build config (vite/next/…).
// Nested example/demo/docs paths are excluded, so a library's demo page does not
// read as the product's entry.
func (ix *repoIndex) frontendEntry() (Observation, bool) {
	if p, ok := ix.firstPrimaryPath(func(lp string) bool { return path.Base(lp) == "index.html" }); ok {
		return Observation{Path: p, Detail: "web app entry (index.html)"}, true
	}
	if p, ok := ix.firstPrimaryPath(func(lp string) bool { return strings.HasPrefix(lp, "src/app.") }); ok {
		return Observation{Path: p, Detail: "root component (src/App)"}, true
	}
	if p, ok := ix.firstPrimaryPath(func(lp string) bool { return strings.HasPrefix(lp, "src/main.") }); ok {
		return Observation{Path: p, Detail: "app bootstrap (src/main)"}, true
	}
	if p, ok := ix.firstPrimaryPath(isAppConfig); ok {
		return Observation{Path: p, Detail: "app build configuration"}, true
	}
	return Observation{}, false
}

// hasPrimaryAppShell reports whether a primary-component app shell (index.html
// or a src/App root component) exists — the stricter "this is an app" marker
// detectLibrary uses, which deliberately excludes a build config a library also
// ships.
func (ix *repoIndex) hasPrimaryAppShell() bool {
	if _, ok := ix.firstPrimaryPath(func(lp string) bool { return path.Base(lp) == "index.html" }); ok {
		return true
	}
	_, ok := ix.firstPrimaryPath(func(lp string) bool { return strings.HasPrefix(lp, "src/app.") })
	return ok
}

// firstPrimaryPath returns the first original-cased tree path at a primary
// component scope whose lowercased form satisfies pred.
func (ix *repoIndex) firstPrimaryPath(pred func(lower string) bool) (string, bool) {
	for i, lp := range ix.lowerPath {
		if scopeRank(ix.paths[i]) <= 1 && pred(lp) {
			return ix.paths[i], true
		}
	}
	return "", false
}

// isAppConfig reports whether a path is a web-app framework build config.
func isAppConfig(lower string) bool {
	switch path.Base(lower) {
	case "vite.config.ts", "vite.config.js", "next.config.js", "next.config.mjs",
		"nuxt.config.ts", "nuxt.config.js", "svelte.config.js", "astro.config.mjs",
		"angular.json":
		return true
	}
	return false
}

// --- backend -----------------------------------------------------------------

var (
	goServerDeps   = []string{"gin-gonic/gin", "labstack/echo", "gofiber/fiber", "go-chi/chi", "gorilla/mux"}
	nodeServerDeps = []string{"express", "fastify", "koa", "@nestjs/core", "@hapi/hapi", "hapi"}
	pyServerDeps   = []string{"fastapi", "flask", "django", "starlette", "uvicorn", "sanic", "aiohttp", "tornado"}
	rustServerDeps = []string{"axum", "actix-web", "rocket", "warp"}
)

func detectBackend(ix *repoIndex) (apimodel.RepoKind, []Observation, bool) {
	var ev []Observation

	// Go: a server needs both a command (package main) and a server signal. The
	// main package may be confirmed from read source or seen in the tree; the
	// server signal comes from read source or a declared server dependency.
	if ix.isGo() {
		if mainObs, ok := ix.goMainObservation(); ok {
			if serverObs, ok := ix.goServerObservation(); ok {
				ev = append(ev, mainObs, serverObs)
			}
		}
	}
	// Node server frameworks.
	if ix.pkg != nil {
		for _, dep := range nodeServerDeps {
			if v := ix.pkg.dep(dep); v != "" {
				ev = append(ev, Observation{Path: ix.pkgPath, Detail: "depends on " + quoteDep(dep, v) + " (server framework)"})
				break
			}
		}
	}
	// Python web frameworks, matched against parsed dependency names (not a
	// substring of the file), so a project named "flask-utils" is not a Flask dep.
	if obs, ok := ix.pyServerObservation(); ok {
		ev = append(ev, obs)
	}
	// Rust: a binary target (explicit or Cargo's implicit src/main.rs) that
	// depends on a web framework.
	if ix.rustHasBinTarget() {
		if dep := cargoDep(ix.cargoM, rustServerDeps); dep != "" {
			ev = append(ev, Observation{Path: ix.cargoPath, Detail: "a binary target depends on \"" + dep + "\" (web framework)"})
		}
	}
	// Containerized service that declares a network port. Frontend was already
	// ruled out above. The wording stays a declaration: EXPOSE records a port the
	// container intends to publish; it is not observed listening behaviour.
	if port := dockerExposedPort(ix.dockerfile); port != "" {
		ev = append(ev, Observation{Path: ix.dockerPath, Detail: "declares EXPOSE " + port + " (a container network port)"})
	}

	if len(ev) > 0 {
		return apimodel.KindBackend, ev, true
	}
	return "", nil, false
}

// --- cli ---------------------------------------------------------------------

var (
	goCLIDeps   = []string{"spf13/cobra", "urfave/cli", "spf13/pflag", "alecthomas/kong", "alecthomas/kingpin"}
	rustCLIDeps = []string{"clap", "structopt", "argh"}
)

func detectCLI(ix *repoIndex) (apimodel.RepoKind, []Observation, bool) {
	var ev []Observation

	// Go: a binary (package main) wired to a command framework, or — only when
	// the entry source was actually inspected — a main package under cmd/ with no
	// server signal. The "no server signal" fallback must never fire on an
	// unread entry, or a server would be mislabelled a command.
	if ix.isGo() {
		if mainObs, ok := ix.goMainObservation(); ok {
			if dep := ix.goDep(goCLIDeps); dep != "" {
				ev = append(ev, mainObs, Observation{Path: ix.goModPath, Detail: "requires \"" + dep + "\" (CLI framework)"})
			} else if _, readable := ix.goMainReadablePath(); readable && ix.hasPrefix("cmd/") {
				if _, server := ix.goServerObservation(); !server {
					ev = append(ev, mainObs, Observation{Detail: "entry point under cmd/ with no server signal in the inspected source (produces a command)"})
				}
			}
		}
	}
	// npm: a package that installs a command exposes a "bin" field.
	if ix.pkg != nil && len(ix.pkg.Bin) > 0 && string(ix.pkg.Bin) != "null" {
		ev = append(ev, Observation{Path: ix.pkgPath, Detail: `declares a "bin" field (installs a command)`})
	}
	// Python: command entry points produce commands — a modern [project.scripts]
	// table, or the legacy console_scripts in setup.py/setup.cfg. GUI entry
	// points ([project.gui-scripts]) are deliberately not treated as a CLI.
	if ix.pyHasProject && len(ix.pyProj.Project.Scripts) > 0 {
		ev = append(ev, Observation{Path: ix.pyprojPath, Detail: "defines [project.scripts] (installs a command)"})
	} else if py, src := ix.pyText(); py != "" && strings.Contains(py, "console_scripts") {
		ev = append(ev, Observation{Path: src, Detail: "defines console_scripts (installs a command)"})
	}
	// Rust: a binary target (explicit or implicit). A CLI-parser dependency is
	// cited when present, but the binary target — not the parser alone — is what
	// makes it a command. A binary with a web framework was already taken as a
	// backend above, so reaching here means no server dependency.
	if ix.rustHasBinTarget() {
		if dep := cargoDep(ix.cargoM, rustCLIDeps); dep != "" {
			ev = append(ev, Observation{Path: ix.cargoPath, Detail: "a binary target depends on \"" + dep + "\" (CLI parser)"})
		} else if cargoDep(ix.cargoM, rustServerDeps) == "" {
			ev = append(ev, Observation{Path: ix.cargoPath, Detail: "a binary target with no web-framework dependency (produces a command)"})
		}
	}

	if len(ev) > 0 {
		return apimodel.KindCLI, ev, true
	}
	return "", nil, false
}

// --- library -----------------------------------------------------------------

func detectLibrary(ix *repoIndex) (apimodel.RepoKind, []Observation, bool) {
	var ev []Observation

	// Go: a module with no package main is importable-only (e.g. gin). The
	// absence of a main package is only conclusive when the listing is complete —
	// a truncated or depth-limited tree could hide one.
	if ix.isGo() && ix.goMod != "" {
		if _, ok := ix.goMainObservation(); !ok && ix.scope.Complete() {
			ev = append(ev, Observation{Path: ix.goModPath, Detail: "present with no package main in the tree (importable Go module)"})
		}
	}
	// npm: a lib entry (main/module/exports) and no app entry or bin.
	if ix.pkg != nil {
		hasLibEntry := ix.pkg.Main != "" || ix.pkg.Module != "" ||
			(len(ix.pkg.Exports) > 0 && string(ix.pkg.Exports) != "null")
		hasApp := ix.hasPrimaryAppShell()
		hasBin := len(ix.pkg.Bin) > 0 && string(ix.pkg.Bin) != "null"
		if hasLibEntry && !hasApp && !hasBin {
			ev = append(ev, Observation{Path: ix.pkgPath, Detail: `declares a lib entry ("main"/"module"/"exports") and no app or bin (importable package)`})
		}
	}
	// Rust: a library target (explicit [lib] or implicit src/lib.rs) and no
	// binary target — an importable crate.
	if ix.rustHasLibTarget() && !ix.rustHasBinTarget() {
		ev = append(ev, Observation{Path: ix.cargoPath, Detail: "a library target and no binary target (importable crate)"})
	}
	// Python: real packaging metadata — a [project] table or a setup.py — with no
	// command entry points is an importable package. A pyproject.toml carrying
	// only tool configuration ([tool.*]) is not packaging metadata and is not a
	// library.
	if src, ok := ix.pyPackageSource(); ok && !ix.pyHasCommandEntry() {
		ev = append(ev, Observation{Path: src, Detail: "packaging metadata with no command entry points (importable package)"})
	}

	if len(ev) > 0 {
		return apimodel.KindLibrary, ev, true
	}
	return "", nil, false
}

// --- language / Go helpers ---------------------------------------------------

func (ix *repoIndex) isGo() bool {
	return ix.language == "go" || ix.goMod != "" || ix.hasBase("go.mod")
}

// primaryGoFiles are the read Go sources that belong to the primary product —
// root or an ordinary subdirectory (cmd/, internal/, …) — excluding example,
// docs and tooling scopes. A main or server call inside examples/ is a property
// of the example, not of the repository's primary kind.
func (ix *repoIndex) primaryGoFiles() []SourceFile {
	var out []SourceFile
	for _, f := range ix.goFiles {
		if scopeRank(f.Path) <= 1 {
			out = append(out, f)
		}
	}
	return out
}

// goMainReadable returns a primary-scope Go source actually read that declares
// `package main` (parsed, so a `package main` in a comment never counts), and
// whether one was found.
func (ix *repoIndex) goMainReadable() (SourceFile, bool) {
	for _, f := range ix.primaryGoFiles() {
		if goDeclaresPackageMain(f.Content) {
			return f, true
		}
	}
	return SourceFile{}, false
}

// goMainReadablePath is goMainReadable reduced to the path.
func (ix *repoIndex) goMainReadablePath() (string, bool) {
	f, ok := ix.goMainReadable()
	return f.Path, ok
}

// goMainObservation reports that the repo contains a Go command, citing the read
// source that declares `package main` when available, otherwise the main.go seen
// in the tree (whose source was not inspected — noted in the Detail and, for the
// absence case, in limitations()).
func (ix *repoIndex) goMainObservation() (Observation, bool) {
	if p, ok := ix.goMainReadablePath(); ok {
		return Observation{Path: p, Detail: "package main (produces a binary)"}, true
	}
	for _, p := range ix.paths {
		if strings.ToLower(path.Base(p)) == "main.go" && scopeRank(p) <= 1 {
			return Observation{Path: p, Detail: "Go command entry point present (source not inspected)"}, true
		}
	}
	return Observation{}, false
}

// goServerObservation returns evidence that a Go command starts a server: a read
// source with a real http.ListenAndServe call (recognised from the AST, not a
// substring, so a comment does not count), or a declared server-framework
// dependency in go.mod.
func (ix *repoIndex) goServerObservation() (Observation, bool) {
	for _, f := range ix.primaryGoFiles() {
		if starts, _ := goStartsHTTPServer(f.Content); starts {
			return Observation{Path: f.Path, Detail: "calls http.ListenAndServe (starts an HTTP server)"}, true
		}
	}
	if dep := ix.goDep(goServerDeps); dep != "" {
		return Observation{Path: ix.goModPath, Detail: "requires \"" + dep + "\" (server framework)"}, true
	}
	return Observation{}, false
}

// goDep returns the first of wants required by go.mod (parsed with the official
// module-file parser, so a module named only in a comment never counts).
func (ix *repoIndex) goDep(wants []string) string {
	return goRequiresAny(ix.goMod, wants)
}

// --- Rust helpers ------------------------------------------------------------

// rustHasBinTarget reports whether the primary crate has a binary target, from
// an explicit [[bin]] table or — when autobins is on — Cargo's implicit
// src/main.rs or src/bin/*.rs in the primary component.
func (ix *repoIndex) rustHasBinTarget() bool {
	if !ix.cargoOK {
		return false
	}
	if len(ix.cargoM.Bin) > 0 {
		return true
	}
	if !cargoAutobins(ix.cargoM) {
		return false
	}
	_, ok := ix.firstPrimaryPath(func(lp string) bool {
		return lp == "src/main.rs" || (strings.HasPrefix(lp, "src/bin/") && strings.HasSuffix(lp, ".rs"))
	})
	return ok
}

// rustHasLibTarget reports whether the primary crate has a library target, from
// an explicit [lib] table or — when autolib is on — Cargo's implicit src/lib.rs.
func (ix *repoIndex) rustHasLibTarget() bool {
	if !ix.cargoOK {
		return false
	}
	if ix.cargoM.Lib != nil {
		return true
	}
	if !cargoAutolib(ix.cargoM) {
		return false
	}
	_, ok := ix.firstPrimaryPath(func(lp string) bool { return lp == "src/lib.rs" })
	return ok
}

// --- Python helpers ----------------------------------------------------------

// pyText returns the preferred Python packaging text and the path it came from.
func (ix *repoIndex) pyText() (text, srcPath string) {
	if ix.pyproject != "" {
		return ix.pyproject, ix.pyprojPath
	}
	return ix.setupPy, ix.setupPyPath
}

// pyPackageSource reports the file that marks the repo as a real Python package
// — a pyproject.toml with a [project] table, or a setup.py — and whether one was
// found. A pyproject with only [tool.*] tables does not count.
func (ix *repoIndex) pyPackageSource() (string, bool) {
	if ix.pyHasProject {
		return ix.pyprojPath, true
	}
	if ix.setupPy != "" {
		return ix.setupPyPath, true
	}
	return "", false
}

// pyHasCommandEntry reports whether the package installs a command, via a modern
// [project.scripts] table or legacy console_scripts.
func (ix *repoIndex) pyHasCommandEntry() bool {
	if ix.pyHasProject && len(ix.pyProj.Project.Scripts) > 0 {
		return true
	}
	py, _ := ix.pyText()
	return py != "" && strings.Contains(py, "console_scripts")
}

// pyServerObservation returns evidence that a declared Python dependency is a
// web framework, citing the file (pyproject.toml or requirements.txt) it was
// declared in. It iterates the framework list in fixed order so the result is
// deterministic regardless of map iteration.
func (ix *repoIndex) pyServerObservation() (Observation, bool) {
	for _, want := range pyServerDeps {
		if src, ok := ix.pyDepSource[pyDistName(want)]; ok {
			return Observation{Path: src, Detail: "declares a dependency on \"" + want + "\" (web framework)"}, true
		}
	}
	return Observation{}, false
}

// --- small pure utilities ----------------------------------------------------

// dockerExposedPort returns the first port from an EXPOSE directive, or "".
func dockerExposedPort(dockerfile string) string {
	for _, line := range strings.Split(dockerfile, "\n") {
		fields := strings.Fields(strings.TrimSpace(line))
		if len(fields) >= 2 && strings.EqualFold(fields[0], "EXPOSE") {
			return fields[1]
		}
	}
	return ""
}

// quoteDep formats a dependency and its version range for evidence, e.g.
// `"react" (^18.2.0)`; the version is omitted when unknown.
func quoteDep(name, version string) string {
	if version == "" {
		return `"` + name + `"`
	}
	return `"` + name + `" (` + version + ")"
}

// --- package.json ------------------------------------------------------------

type packageJSON struct {
	Private          bool              `json:"private"`
	Main             string            `json:"main"`
	Module           string            `json:"module"`
	Exports          json.RawMessage   `json:"exports"`
	Bin              json.RawMessage   `json:"bin"`
	Scripts          map[string]string `json:"scripts"`
	Dependencies     map[string]string `json:"dependencies"`
	DevDependencies  map[string]string `json:"devDependencies"`
	PeerDependencies map[string]string `json:"peerDependencies"`
}

// parsePackageJSON decodes package.json best-effort; malformed JSON yields nil
// so callers record a parse limitation rather than reading absent content as a
// negative fact.
func parsePackageJSON(content string) *packageJSON {
	var p packageJSON
	if err := json.Unmarshal([]byte(content), &p); err != nil {
		return nil
	}
	return &p
}

// depCategory names which dependency table a package was declared in, kept
// distinct so a later role decision can tell a runtime dependency (evidence of
// what the product uses) from a build-time dev dependency (evidence of how it is
// built) — the two must not be conflated.
type depCategory string

const (
	depRuntime depCategory = "dependencies"
	depDev     depCategory = "devDependencies"
	depPeer    depCategory = "peerDependencies"
)

// lookupDep returns the declared version and the category a dependency was found
// in, checking runtime, then dev, then peer, so a real (dev)dependency is
// preferred over a peer range. ok is false when the name is absent.
func (p *packageJSON) lookupDep(name string) (version string, cat depCategory, ok bool) {
	for _, e := range []struct {
		m   map[string]string
		cat depCategory
	}{
		{p.Dependencies, depRuntime},
		{p.DevDependencies, depDev},
		{p.PeerDependencies, depPeer},
	} {
		if v, found := e.m[name]; found {
			return v, e.cat, true
		}
	}
	return "", "", false
}

// dep returns the version range declared for name in any category, or "" if
// absent. It is the category-agnostic lookup; use lookupDep when the category
// (runtime vs dev vs peer) matters to the decision.
func (p *packageJSON) dep(name string) string {
	v, _, _ := p.lookupDep(name)
	return v
}
