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
// that bounded the decision.
//
// # Absence premises
//
// Several roles rest on something being absent — a command with no server call,
// a crate with no binary target, a package with no command entry. Each such
// premise names the inspected scope it needs (see the per-detector comments) and
// is only used when that scope was actually inspected: a file read in full and
// parsed, or a listing that covers the paths in question. When the scope was not
// inspected the premise is withheld and a limitation names the missing path or
// region. A limitation never validates an unsupported verdict on its own.
//
// # Precedence (#algorithms)
//
// A repo often carries several signals at once (a Next.js app also runs a Node
// server; a Go server also lives under cmd/). Every detector runs and each hit
// becomes a candidate; the candidate whose evidence sits in the most primary
// component scope wins, and detector order — mobile → frontend → backend → cli
// → library — breaks ties within a scope:
//
//   - mobile before frontend: react-native/expo apps also depend on react.
//   - frontend before backend: a client app that ships a Dockerfile is labelled
//     by its user-facing surface, not its container.
//   - backend before cli: a server that exposes subcommands is still a server.
//   - library last: an importable package that also produces a runnable
//     artifact is classified by the artifact.
//
// When no detector finds decisive, inspected evidence, Classify returns
// KindOther with VerdictInconclusive and the limitations that explain why — an
// honest abstention, not a confident "other".
func Classify(insp Inspection) Classification {
	ix := newRepoIndex(insp)

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
	// Limitations are gathered after the detectors ran, so they include the
	// premises each detector had to withhold.
	limits := ix.limitations()
	if len(candidates) == 0 {
		limits = append(limits, "no decisive frontend / backend / library / CLI / mobile signal was found in the inspected manifests and file tree")
		return Classification{Kind: apimodel.KindOther, Status: VerdictInconclusive, Limitations: limits}
	}

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

	// Withhold the verdict when something at least as primary as the chosen
	// candidate could not be inspected — an unreadable manifest or an unread Go
	// entry point. It could itself define the primary product, so picking a
	// less-primary signal would be a guess.
	if s, ok := ix.unresolvedScope(); ok && primaryScope >= s {
		for _, c := range candidates {
			for _, o := range c.obs {
				if o.Path != "" {
					limits = append(limits, "candidate ("+string(c.kind)+"): "+o.Path+": "+o.Detail)
				}
			}
		}
		limits = append(limits, "evidence at least as primary as every candidate could not be inspected; verdict withheld")
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

// manifestRef is one manifest the repository's primary product declares, with
// what acquisition managed to do with it. Content is set only when the manifest
// was read (in full or in part).
type manifestRef struct {
	path    string
	kind    manifestKind
	state   ReadState
	reason  string
	content string
}

func (m manifestRef) present() bool  { return m.path != "" }
func (m manifestRef) readable() bool { return m.state == StateRead || m.state == StateTruncated }
func (m manifestRef) complete() bool { return m.state == StateRead }

// repoIndex is the pre-parsed, cheap-to-query view of an Inspection. Each
// manifest is chosen deterministically among every listed, read or failed
// instance (most primary scope, then shallowest, then lexical), keeping its read
// state — so a failed root manifest stays visible instead of being silently
// replaced by a nested one.
type repoIndex struct {
	st        *inspIndex
	language  string   // lowercased primary language
	paths     []string // original tree paths
	lowerPath []string // lowercased tree paths, index-aligned with paths

	pkgRef       manifestRef
	pkg          *packageJSON // parsed package.json, or nil
	pkgBad       bool         // read but would not parse
	goMod        manifestRef
	modPath      string // module path declared by go.mod
	goModOK      bool   // go.mod parsed
	cargo        manifestRef
	cargoM       cargoManifest
	cargoOK      bool
	pyproj       manifestRef
	pyProj       pyProject
	pyHasProject bool
	pyOK         bool
	setupPy      manifestRef
	pubspec      manifestRef
	docker       manifestRef
	reqs         manifestRef

	pyDepSource map[string]string // normalised python dep name -> the file it was declared in
	goFiles     []goFile          // every read Go source, path-sorted

	notes []string // premises the detectors had to withhold, in detection order
}

func newRepoIndex(insp Inspection) *repoIndex {
	st := indexInspection(insp)
	ix := &repoIndex{
		st:        st,
		language:  strings.ToLower(strings.TrimSpace(insp.Language)),
		paths:     insp.Paths,
		lowerPath: make([]string, len(insp.Paths)),
	}
	for i, p := range insp.Paths {
		ix.lowerPath[i] = strings.ToLower(p)
	}

	ix.pkgRef = pickManifest(st, "package.json")
	if ix.pkgRef.readable() {
		if ix.pkg = parsePackageJSON(ix.pkgRef.content); ix.pkg == nil {
			ix.pkgBad = true
		}
	}
	ix.goMod = pickManifest(st, "go.mod")
	if ix.goMod.readable() {
		ix.modPath = goModulePath(ix.goMod.content)
		ix.goModOK = ix.modPath != ""
	}
	ix.cargo = pickManifest(st, "cargo.toml")
	if ix.cargo.readable() {
		ix.cargoM, ix.cargoOK = parseCargo(ix.cargo.content)
	}
	ix.pyproj = pickManifest(st, "pyproject.toml")
	if ix.pyproj.readable() {
		ix.pyProj, ix.pyHasProject, ix.pyOK = parsePyProject(ix.pyproj.content)
	}
	ix.setupPy = pickManifest(st, "setup.py")
	ix.pubspec = pickManifest(st, "pubspec.yaml", "pubspec.yml")
	ix.docker = pickManifest(st, "dockerfile")
	ix.reqs = pickManifest(st, "requirements.txt")

	// setup.py is deliberately not parsed for dependencies — its install_requires
	// is dynamic Python we never execute.
	ix.pyDepSource = buildPyDepSource(ix.pyProj, ix.pyproj.path, ix.reqs.content, ix.reqs.path)

	for _, f := range insp.Files {
		if strings.HasSuffix(strings.ToLower(f.Path), ".go") {
			ix.goFiles = append(ix.goFiles, analyzeGoFile(f))
		}
	}
	sort.Slice(ix.goFiles, func(i, j int) bool { return ix.goFiles[i].path < ix.goFiles[j].path })
	return ix
}

// pickManifest chooses, among every listed, read or unavailable path whose base
// name is one of bases, the one that best represents the repository's primary
// product: the most primary component scope (root over a nested component, both
// over example/docs/tooling), then the shallowest path, then the lexically first.
// The order is total, so the choice does not depend on input order. The chosen
// manifest keeps its read state even when it could not be read.
func pickManifest(x *inspIndex, bases ...string) manifestRef {
	var best manifestRef
	bestScope, bestDepth, found := 0, 0, false
	consider := func(p string) {
		b := strings.ToLower(path.Base(p))
		match := false
		for _, w := range bases {
			if b == w {
				match = true
			}
		}
		if !match {
			return
		}
		s, d := scopeRank(p), strings.Count(p, "/")
		if found && (s > bestScope || (s == bestScope && d > bestDepth) ||
			(s == bestScope && d == bestDepth && p >= best.path)) {
			return
		}
		found, bestScope, bestDepth = true, s, d
		state, reason := x.state(p)
		best = manifestRef{path: p, state: state, reason: reason}
		if f, ok := x.files[p]; ok {
			best.content = f.Content
		}
	}
	for _, p := range x.paths {
		consider(p)
	}
	for p := range x.files {
		consider(p)
	}
	for p := range x.unavail {
		consider(p)
	}
	return best
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

// scopeRank is a coarse, directory-derived guess at a path's role in the
// repository, used to prefer the primary product over an example, the docs site
// or tooling. Folder names are hints, not a component model.
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

// manifests lists the chosen manifests in a fixed order, for limitations.
func (ix *repoIndex) manifests() []manifestRef {
	return []manifestRef{ix.pkgRef, ix.goMod, ix.cargo, ix.pyproj, ix.setupPy, ix.pubspec, ix.reqs, ix.docker}
}

// note records a premise a detector had to withhold, naming the missing scope.
func (ix *repoIndex) note(s string) { ix.notes = append(ix.notes, s) }

// limitations gathers the honest caveats on any verdict: manifests that could
// not be read or parsed, a bounded listing, unread Go entry points, and every
// premise a detector had to withhold. Order is fixed and duplicates are dropped.
func (ix *repoIndex) limitations() []string {
	var out []string
	for _, m := range ix.manifests() {
		if m.present() && !m.readable() {
			out = append(out, ix.st.describe(m.path)+"; its declarations are unknown")
		}
	}
	if ix.pkgBad {
		out = append(out, "package.json at "+ix.pkgRef.path+" could not be parsed; its signals were not used")
	}
	if ix.pyproj.readable() && !ix.pyOK {
		out = append(out, "pyproject.toml at "+ix.pyproj.path+" could not be parsed; its signals were not used")
	}
	if ix.cargo.readable() && !ix.cargoOK {
		out = append(out, "Cargo.toml at "+ix.cargo.path+" could not be parsed; its signals were not used")
	}
	if ix.goMod.readable() && !ix.goModOK {
		out = append(out, "go.mod at "+ix.goMod.path+" could not be parsed; its signals were not used")
	}
	if ix.st.scope.Truncated {
		out = append(out, "the file listing was truncated; a signal absent from it cannot be treated as absent from the repository")
	}
	if ix.st.scope.DepthLimited {
		out = append(out, "the file listing was depth-limited; files below the depth cap were not listed, so a deeper entry point may exist")
	}
	if unread := ix.unreadGoEntries(); len(unread) > 0 {
		out = append(out, "Go entry point(s) present but their source was not inspected ("+strings.Join(unread, ", ")+"); a server could not be distinguished from a command")
	}
	out = append(out, ix.notes...)
	return dedupe(out)
}

// unresolvedScope is the most primary scope holding evidence that could define
// the primary product but could not be inspected: an unreadable manifest, or a
// listed Go entry point whose source was not read.
func (ix *repoIndex) unresolvedScope() (int, bool) {
	best, ok := 0, false
	consider := func(p string) {
		if s := scopeRank(p); !ok || s < best {
			best, ok = s, true
		}
	}
	for _, m := range ix.manifests() {
		if m.present() && !m.readable() {
			consider(m.path)
		}
	}
	for _, p := range ix.unreadGoEntries() {
		consider(p)
	}
	return best, ok
}

// unreadGoEntries lists primary-scope main.go paths that appear in the tree but
// whose content was not read. It only reports when the repo looks like Go.
func (ix *repoIndex) unreadGoEntries() []string {
	if !ix.isGo() {
		return nil
	}
	var unread []string
	for _, p := range ix.paths {
		if strings.ToLower(path.Base(p)) == "main.go" && scopeRank(p) <= 1 && !ix.isRead(p) {
			unread = append(unread, p)
		}
	}
	sort.Strings(unread)
	return unread
}

func (ix *repoIndex) isRead(p string) bool {
	s, _ := ix.st.state(p)
	return s == StateRead || s == StateTruncated
}

// hasPrefix reports whether any tree path starts with the given (lowercased)
// prefix — used for directory ("android/") checks.
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
				ev = append(ev, Observation{Path: ix.pkgRef.path, Detail: "depends on " + quoteDep(dep, v) + " (mobile framework)"})
			}
		}
	}
	// Flutter projects declare a Flutter SDK dependency in pubspec.yaml — checked
	// from the parsed dependency tables, so the word "flutter" in a comment does
	// not count.
	if ix.pubspec.readable() && pubspecUsesFlutter(ix.pubspec.content) {
		ev = append(ev, Observation{Path: ix.pubspec.path, Detail: "declares a Flutter SDK dependency (mobile)"})
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

// runtimeWebFramework returns the first web framework declared as a runtime
// dependency, with its version.
func (ix *repoIndex) runtimeWebFramework() (name, version string) {
	if ix.pkg == nil {
		return "", ""
	}
	for _, f := range webFrameworks {
		if v, cat, ok := ix.pkg.lookupDep(f); ok && cat == depRuntime {
			return f, v
		}
	}
	return "", ""
}

func detectFrontend(ix *repoIndex) (apimodel.RepoKind, []Observation, bool) {
	// A web framework establishes an application only as a *runtime* dependency.
	// A build tool in devDependencies (vite) or a peer dependency (a UI
	// library's react) does not make the package a frontend product.
	framework, version := ix.runtimeWebFramework()
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
	ev := []Observation{{Path: ix.pkgRef.path, Detail: "depends on " + quoteDep(framework, version) + " (web framework)"}, entry}
	if ix.pkg.Private {
		ev = append(ev, Observation{Path: ix.pkgRef.path, Detail: `"private": true (not a published package)`})
	}
	return apimodel.KindFrontend, ev, true
}

// frontendEntry finds a primary-component app entry: an index.html, a src/App
// root component, a src/main bootstrap, or an app build config (vite/next/…).
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

	// Go: a server needs both a command (a read file whose package clause is
	// main) and a server signal (a server call in read source, or a declared
	// server framework).
	if cmd, ok := ix.goCommand(); ok {
		if serverObs, ok := ix.goServerObservation(); ok {
			ev = append(ev, Observation{Path: cmd.path, Detail: "package main (produces a binary)"}, serverObs)
		}
	}
	// Node server frameworks.
	if ix.pkg != nil {
		for _, dep := range nodeServerDeps {
			if v := ix.pkg.dep(dep); v != "" {
				ev = append(ev, Observation{Path: ix.pkgRef.path, Detail: "depends on " + quoteDep(dep, v) + " (server framework)"})
				break
			}
		}
	}
	// Python web frameworks, matched against parsed dependency names.
	if obs, ok := ix.pyServerObservation(); ok {
		ev = append(ev, obs)
	}
	// Rust: a binary target that depends on a web framework.
	if bin, _ := ix.rustBinTarget(); bin == yes {
		if dep := cargoDep(ix.cargoM, rustServerDeps); dep != "" {
			ev = append(ev, Observation{Path: ix.cargo.path, Detail: "a binary target depends on \"" + dep + "\" (web framework)"})
		}
	}
	// Containerized service that declares a network port. The wording stays a
	// declaration: EXPOSE records a port the container intends to publish.
	if ix.docker.readable() {
		if port := dockerExposedPort(ix.docker.content); port != "" {
			ev = append(ev, Observation{Path: ix.docker.path, Detail: "declares EXPOSE " + port + " (a container network port)"})
		}
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

	// Go: a command wired to a CLI framework, or a command shown to start no
	// server. The latter is an absence premise; see goNoServerPremise for the
	// scope it requires.
	if cmd, ok := ix.goCommand(); ok {
		mainObs := Observation{Path: cmd.path, Detail: "package main (produces a binary)"}
		if dep := ix.goDep(goCLIDeps); dep != "" {
			ev = append(ev, mainObs, Observation{Path: ix.goMod.path, Detail: "requires \"" + dep + "\" (CLI framework)"})
		} else if _, server := ix.goServerObservation(); !server {
			if ok, why := ix.goNoServerPremise(cmd); ok {
				ev = append(ev, mainObs, Observation{Path: cmd.path, Detail: "the command's package was inspected in full and contains no server call (produces a command)"})
			} else {
				for _, w := range why {
					ix.note("cannot conclude the command starts no server: " + w)
				}
			}
		}
	}
	// npm: a package that installs a command exposes a "bin" field.
	if ix.pkg != nil && len(ix.pkg.Bin) > 0 && string(ix.pkg.Bin) != "null" {
		ev = append(ev, Observation{Path: ix.pkgRef.path, Detail: `declares a "bin" field (installs a command)`})
	}
	// Python: command entry points produce commands — a modern [project.scripts]
	// table, or the legacy console_scripts in setup.py/setup.cfg. GUI entry
	// points ([project.gui-scripts]) are deliberately not treated as a CLI.
	if ix.pyHasProject && len(ix.pyProj.Project.Scripts) > 0 {
		ev = append(ev, Observation{Path: ix.pyproj.path, Detail: "defines [project.scripts] (installs a command)"})
	} else if py, src := ix.pyText(); py != "" && strings.Contains(py, "console_scripts") {
		ev = append(ev, Observation{Path: src, Detail: "defines console_scripts (installs a command)"})
	}
	// Rust: a binary target. A CLI-parser dependency is cited when present; a
	// binary with no web-framework dependency is a command only when the full
	// Cargo.toml was read, so the dependency's absence is real.
	if bin, _ := ix.rustBinTarget(); bin == yes {
		cliDep, serverDep := cargoDep(ix.cargoM, rustCLIDeps), cargoDep(ix.cargoM, rustServerDeps)
		switch {
		case cliDep != "":
			ev = append(ev, Observation{Path: ix.cargo.path, Detail: "a binary target depends on \"" + cliDep + "\" (CLI parser)"})
		case serverDep != "":
			// a server binary; detectBackend owns it
		case ix.cargo.complete():
			ev = append(ev, Observation{Path: ix.cargo.path, Detail: "a binary target with no web-framework dependency (produces a command)"})
		default:
			ix.note("cannot conclude the binary target is a command: " + ix.st.describe(ix.cargo.path) + "; a web-framework dependency may lie in the unread remainder")
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

	// Go: an importable package, read from source, and no command at the
	// conventional command locations (see goNoCommandPremise).
	if ix.isGo() && ix.goMod.present() {
		if _, isCmd := ix.goCommand(); !isCmd {
			pkgFile, importable := ix.goImportablePackage()
			ok, why := ix.goNoCommandPremise()
			switch {
			case !importable:
				ix.note("no importable Go package clause was inspected" + ix.unreadGoSample() + "; the module's packages are unknown")
			case !ok:
				for _, w := range why {
					ix.note("cannot conclude the Go module has no command: " + w)
				}
			default:
				ev = append(ev,
					Observation{Path: ix.goMod.path, Detail: "declares module " + ix.modPath + "; no package main at the conventional command locations (module root, cmd/)"},
					Observation{Path: pkgFile.path, Detail: "declares package " + pkgFile.pkg + " (importable)"})
			}
		}
	}
	// npm: a lib entry (main/module/exports) and no app entry or bin. Both
	// absences are premises: "no bin" needs the full package.json; "no app entry"
	// matters only when a runtime web framework is declared, and then needs a
	// listing that covers the conventional entry locations.
	if ix.pkg != nil {
		hasLibEntry := ix.pkg.Main != "" || ix.pkg.Module != "" ||
			(len(ix.pkg.Exports) > 0 && string(ix.pkg.Exports) != "null")
		hasBin := len(ix.pkg.Bin) > 0 && string(ix.pkg.Bin) != "null"
		if hasLibEntry && !hasBin && !ix.hasPrimaryAppShell() {
			ok := true
			if !ix.pkgRef.complete() {
				ix.note("cannot conclude the package installs no command: " + ix.st.describe(ix.pkgRef.path))
				ok = false
			}
			if fw, _ := ix.runtimeWebFramework(); fw != "" && !ix.st.covers(2) {
				ix.note("cannot rule out an app entry for runtime dependency \"" + fw + "\": the listing does not cover index.html, src/App or src/main")
				ok = false
			}
			if ok {
				ev = append(ev, Observation{Path: ix.pkgRef.path, Detail: `declares a lib entry ("main"/"module"/"exports") and no app or bin (importable package)`})
			}
		}
	}
	// Rust: a library target and no binary target. The binary's absence needs
	// the full Cargo.toml and a listing that covers Cargo's implicit targets.
	if ix.rustHasLibTarget() {
		switch bin, why := ix.rustBinTarget(); bin {
		case no:
			ev = append(ev, Observation{Path: ix.cargo.path, Detail: "a library target and no binary target (importable crate)"})
		case unknown:
			ix.note("a library target is present, but " + why)
		}
	}
	// Python: real packaging metadata — a [project] table or a setup.py — with no
	// command entry points. "No command entry" needs the manifest read in full.
	if src, ok := ix.pyPackageSource(); ok && !ix.pyHasCommandEntry() {
		ref := ix.pyproj
		if src != ix.pyproj.path {
			ref = ix.setupPy
		}
		if ref.complete() {
			ev = append(ev, Observation{Path: src, Detail: "packaging metadata with no command entry points (importable package)"})
		} else {
			ix.note("cannot conclude the package declares no command entry: " + ix.st.describe(src) + "; entry points may lie in the unread remainder")
		}
	}

	if len(ev) > 0 {
		return apimodel.KindLibrary, ev, true
	}
	return "", nil, false
}

// --- language / Go helpers ---------------------------------------------------

func (ix *repoIndex) isGo() bool {
	if ix.language == "go" || ix.goMod.present() {
		return true
	}
	for _, lp := range ix.lowerPath {
		if path.Base(lp) == "go.mod" {
			return true
		}
	}
	return false
}

func isGoSource(p string) bool {
	lp := strings.ToLower(p)
	return strings.HasSuffix(lp, ".go") && !strings.HasSuffix(lp, "_test.go")
}

// primaryGoFiles are the read Go sources that belong to the primary product —
// root or an ordinary subdirectory (cmd/, internal/, …) — excluding example,
// docs and tooling scopes.
func (ix *repoIndex) primaryGoFiles() []goFile {
	var out []goFile
	for _, g := range ix.goFiles {
		if scopeRank(g.path) <= 1 {
			out = append(out, g)
		}
	}
	return out
}

func (ix *repoIndex) goFileAt(p string) (goFile, bool) {
	for _, g := range ix.goFiles {
		if g.path == p {
			return g, true
		}
	}
	return goFile{}, false
}

// goCommand returns a primary-scope, non-test Go source whose package clause is
// main. The clause is read from the file itself, so a clipped file still counts
// as a command — the clause is a positive fact.
func (ix *repoIndex) goCommand() (goFile, bool) {
	for _, g := range ix.primaryGoFiles() {
		if !g.test && g.pkg == "main" {
			return g, true
		}
	}
	return goFile{}, false
}

// goNoServerPremise reports whether "this command starts no server" is backed by
// inspected source. Its scope is the command's own package and the module's
// declared requirements:
//
//   - every non-test Go file listed in (or read from) the command's directory was
//     read in full and parsed;
//   - go.mod was read in full and parsed, so no server framework requirement is
//     hiding in it;
//   - the command imports no package from its own module, since imports are not
//     followed and a server could be started there.
//
// Third-party packages other than the recognised server frameworks are not
// inspected; that bound is part of the documented policy.
func (ix *repoIndex) goNoServerPremise(cmd goFile) (bool, []string) {
	var why []string
	dir := path.Dir(cmd.path)
	seen := map[string]bool{}
	check := func(p string) {
		if seen[p] || path.Dir(p) != dir || !isGoSource(p) {
			return
		}
		seen[p] = true
		g, read := ix.goFileAt(p)
		switch {
		case !read:
			why = append(why, ix.st.describe(p)+"; it may contain a server call")
		case !g.complete:
			why = append(why, ix.st.describe(p)+"; a server call may lie in the unread remainder")
		case !g.parsed:
			why = append(why, p+" could not be parsed ("+g.parseErr+"); a server call cannot be ruled out")
		}
	}
	for _, p := range ix.paths {
		check(p)
	}
	for _, g := range ix.goFiles {
		check(g.path)
	}
	if !ix.goMod.complete() || !ix.goModOK {
		why = append(why, "go.mod was not read in full and parsed; a server framework requirement cannot be ruled out")
	}
	if ix.modPath != "" {
		for _, g := range ix.goFiles {
			if path.Dir(g.path) != dir || g.test {
				continue
			}
			for _, im := range g.imports {
				if im == ix.modPath || strings.HasPrefix(im, ix.modPath+"/") {
					why = append(why, g.path+" imports "+im+" from its own module, which was not followed; a server started there cannot be ruled out")
				}
			}
		}
	}
	return len(why) == 0, why
}

// goImportablePackage returns a read, primary-scope, non-test Go source whose
// package is importable from outside the module: not main and not under an
// internal/ or cmd/ directory.
func (ix *repoIndex) goImportablePackage() (goFile, bool) {
	for _, g := range ix.primaryGoFiles() {
		if g.test || g.pkg == "" || g.pkg == "main" || strings.HasSuffix(g.pkg, "_test") {
			continue
		}
		dir := "/" + path.Dir(g.path) + "/"
		if strings.Contains(dir, "/internal/") || strings.HasPrefix(dir, "/cmd/") {
			continue
		}
		return g, true
	}
	return goFile{}, false
}

// goNoCommandPremise reports whether "this module has no command" is backed by
// inspected evidence. Go places a command's package in its own directory, and by
// convention commands live at the module root or under cmd/<name>/. The premise
// therefore requires:
//
//   - the module root's package clause to be known from a read file (or the
//     listing to show no root Go files at all);
//   - the listing to cover cmd/<name>/ and every Go file listed under cmd/ to
//     have been read;
//   - every listed main.go to have been read (enforced by the unread-entry veto).
//
// A command placed in some other directory, under a file name other than
// main.go, is outside this bounded scope; that blind spot is documented here
// rather than closed by fetching every file.
func (ix *repoIndex) goNoCommandPremise() (bool, []string) {
	var why []string
	rootListed, rootKnown, rootExample := false, false, ""
	for _, p := range ix.goCandidatePaths() {
		if path.Dir(p) != "." {
			continue
		}
		rootListed = true
		if g, ok := ix.goFileAt(p); ok && g.pkg != "" {
			rootKnown = true
		} else if rootExample == "" {
			rootExample = p
		}
	}
	switch {
	case rootListed && !rootKnown:
		why = append(why, "the module root's package clause was not inspected ("+ix.st.describe(rootExample)+"); the root could be a command")
	case !rootListed && !ix.st.covers(1):
		why = append(why, "the listing may omit Go files at the module root; the root could be a command")
	}
	if !ix.st.covers(3) {
		why = append(why, "the listing does not cover cmd/<name>/; a command there cannot be ruled out")
	}
	for _, p := range ix.goCandidatePaths() {
		if strings.HasPrefix(strings.ToLower(p), "cmd/") && !ix.isRead(p) {
			why = append(why, ix.st.describe(p)+"; it may be a command")
		}
	}
	return len(why) == 0, dedupe(why)
}

// goCandidatePaths lists every primary-scope, non-test Go source that is listed
// or read, path-sorted.
func (ix *repoIndex) goCandidatePaths() []string {
	set := map[string]bool{}
	for _, p := range ix.paths {
		if isGoSource(p) && scopeRank(p) <= 1 {
			set[p] = true
		}
	}
	for _, g := range ix.goFiles {
		if isGoSource(g.path) && scopeRank(g.path) <= 1 {
			set[g.path] = true
		}
	}
	out := make([]string, 0, len(set))
	for p := range set {
		out = append(out, p)
	}
	sort.Strings(out)
	return out
}

// unreadGoSample names a few unread Go sources, for a limitation.
func (ix *repoIndex) unreadGoSample() string {
	var unread []string
	for _, p := range ix.goCandidatePaths() {
		if !ix.isRead(p) {
			unread = append(unread, p)
		}
	}
	if len(unread) == 0 {
		return ""
	}
	if len(unread) > 3 {
		unread = append(unread[:3], "…")
	}
	return " (listed but not read: " + strings.Join(unread, ", ") + ")"
}

// goServerObservation returns evidence that a Go command starts a server: a read
// source with a server call (recognised from the AST, not a substring), or a
// declared server-framework dependency in go.mod. Both are positive facts, so
// they may come from a partial file.
func (ix *repoIndex) goServerObservation() (Observation, bool) {
	for _, g := range ix.primaryGoFiles() {
		if starts, _ := goStartsHTTPServer(g.content); starts {
			return Observation{Path: g.path, Detail: "calls http.ListenAndServe (starts an HTTP server)"}, true
		}
	}
	if dep := ix.goDep(goServerDeps); dep != "" {
		return Observation{Path: ix.goMod.path, Detail: "requires \"" + dep + "\" (server framework)"}, true
	}
	return Observation{}, false
}

// goDep returns the first of wants required by go.mod (parsed with the official
// module-file parser, so a module named only in a comment never counts).
func (ix *repoIndex) goDep(wants []string) string {
	if !ix.goMod.readable() {
		return ""
	}
	return goRequiresAny(ix.goMod.content, wants)
}

// --- Rust helpers ------------------------------------------------------------

// tristate is a premise's answer when its scope may not have been inspected.
type tristate int

const (
	unknown tristate = iota
	yes
	no
)

// rustBinTarget reports whether the primary crate has a binary target: yes from
// an explicit [[bin]] table or Cargo's implicit src/main.rs / src/bin/*.rs; no
// only when the full Cargo.toml was read and the listing covers those implicit
// locations; unknown otherwise, with the reason.
func (ix *repoIndex) rustBinTarget() (tristate, string) {
	if !ix.cargo.readable() || !ix.cargoOK {
		return unknown, "Cargo.toml was not read and parsed"
	}
	if len(ix.cargoM.Bin) > 0 {
		return yes, ""
	}
	auto := cargoAutobins(ix.cargoM)
	if auto {
		if _, ok := ix.firstPrimaryPath(func(lp string) bool {
			return lp == "src/main.rs" || (strings.HasPrefix(lp, "src/bin/") && strings.HasSuffix(lp, ".rs"))
		}); ok {
			return yes, ""
		}
	}
	if !ix.cargo.complete() {
		return unknown, ix.st.describe(ix.cargo.path) + "; a [[bin]] table may lie in the unread remainder"
	}
	if auto && !ix.st.covers(3) {
		return unknown, "the listing does not cover src/main.rs and src/bin/, so a binary target cannot be ruled out"
	}
	return no, ""
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
	if ix.pyproj.readable() {
		return ix.pyproj.content, ix.pyproj.path
	}
	return ix.setupPy.content, ix.setupPy.path
}

// pyPackageSource reports the file that marks the repo as a real Python package
// — a pyproject.toml with a [project] table, or a setup.py — and whether one was
// found. A pyproject with only [tool.*] tables does not count.
func (ix *repoIndex) pyPackageSource() (string, bool) {
	if ix.pyHasProject {
		return ix.pyproj.path, true
	}
	if ix.setupPy.readable() {
		return ix.setupPy.path, true
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
	for line := range strings.SplitSeq(dockerfile, "\n") {
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

// dedupe drops repeated strings, keeping the first occurrence's position.
func dedupe(in []string) []string {
	seen := make(map[string]bool, len(in))
	out := in[:0:0]
	for _, s := range in {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}

// --- package.json ------------------------------------------------------------

type packageJSON struct {
	Private          bool              `json:"private"`
	Main             string            `json:"main"`
	Module           string            `json:"module"`
	Exports          json.RawMessage   `json:"exports"`
	Bin              json.RawMessage   `json:"bin"`
	Workspaces       json.RawMessage   `json:"workspaces"`
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
// distinct so a role decision can tell a runtime dependency (evidence of what
// the product uses) from a build-time dev dependency (evidence of how it is
// built) — the two must not be conflated.
type depCategory string

const (
	depRuntime depCategory = "dependencies"
	depDev     depCategory = "devDependencies"
	depPeer    depCategory = "peerDependencies"
)

// lookupDep returns the declared version and the category a dependency was found
// in, checking runtime, then dev, then peer. ok is false when the name is absent.
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
