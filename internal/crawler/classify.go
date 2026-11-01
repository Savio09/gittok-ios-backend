package crawler

import (
	"encoding/json"
	"path"
	"sort"
	"strconv"
	"strings"

	"github.com/Savio09/gittok-ios-backend/internal/apimodel"
)

// This file holds the per-component layer of classification: a view of one
// component (see inventory.go) and the detectors that extract role evidence from
// it. Paths inside a component are interpreted relative to its root, so a nested
// app's "src/main.rs" or "index.html" is its own, never the repository's.
//
// # Absence premises
//
// Several roles rest on something being absent — a command with no server call,
// a crate with no binary target, a package with no command entry. Each such
// premise names the inspected scope it needs (see the per-detector comments) and
// is only used when that scope was actually inspected: a file read in full and
// parsed, or a listing that covers the paths in question. When the scope was not
// inspected the premise is withheld, a limitation names the missing path or
// region, and a block records the highest role the uninspected evidence could
// have produced (see resolve.go for how blocks affect the verdict).

// manifestRef is one manifest a component owns, with what acquisition managed to
// do with it. Content is set only when the manifest was read (in full or part).
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

// block records evidence that could not be inspected and the highest-precedence
// role it could have established, so resolution can tell whether it could change
// a verdict.
type block struct {
	role   apimodel.RepoKind
	reason string
}

// repoIndex is the pre-parsed, cheap-to-query view of one component.
type repoIndex struct {
	st       *inspIndex
	comp     *component
	isRoot   bool
	language string   // lowercased primary language of the repository
	paths    []string // paths the component owns, original case
	lowerRel []string // lowercased paths relative to the component root, index-aligned
	base     int      // path segments in the component root

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
	goWork       manifestRef

	goFiles []goFile // read Go sources the component owns, path-sorted

	notes  []string // limitations, in the order they were found
	blocks []block  // uninspected evidence and the role it could have established
}

func newRepoIndex(st *inspIndex, comp *component, language string) *repoIndex {
	ix := &repoIndex{
		st:       st,
		comp:     comp,
		isRoot:   comp.root == "",
		language: strings.ToLower(strings.TrimSpace(language)),
		paths:    comp.paths,
		lowerRel: make([]string, len(comp.paths)),
		base:     segments(comp.root),
	}
	for i, p := range comp.paths {
		ix.lowerRel[i] = strings.ToLower(comp.rel(p))
	}

	ix.pkgRef = comp.manifest(kindNPM)
	if ix.pkgRef.readable() {
		if ix.pkg = parsePackageJSON(ix.pkgRef.content); ix.pkg == nil {
			ix.pkgBad = true
		}
	}
	ix.goMod = comp.manifest(kindGoMod)
	if ix.goMod.readable() {
		ix.modPath = goModulePath(ix.goMod.content)
		ix.goModOK = ix.modPath != ""
	}
	ix.cargo = comp.manifest(kindCargo)
	if ix.cargo.readable() {
		ix.cargoM, ix.cargoOK = parseCargo(ix.cargo.content)
	}
	ix.pyproj = comp.manifest(kindPyProject)
	if ix.pyproj.readable() {
		ix.pyProj, ix.pyHasProject, ix.pyOK = parsePyProject(ix.pyproj.content)
	}
	ix.setupPy = comp.manifest(kindSetupPy)
	ix.pubspec = comp.manifest(kindPubspec)
	ix.docker = comp.manifest(kindDocker)
	ix.reqs = comp.manifest(kindRequirements)
	ix.goWork = comp.manifest(kindGoWork)

	for _, p := range comp.paths {
		if f, ok := st.files[p]; ok && strings.HasSuffix(strings.ToLower(p), ".go") {
			ix.goFiles = append(ix.goFiles, analyzeGoFile(f))
		}
	}
	sort.Slice(ix.goFiles, func(i, j int) bool { return ix.goFiles[i].path < ix.goFiles[j].path })

	ix.blockUninspectedManifests()
	ix.blockUninspectedPythonDependencies()
	if unread := ix.unreadGoEntries(); len(unread) > 0 {
		ix.block(apimodel.KindBackend, "Go entry point(s) present but their source was not inspected ("+strings.Join(unread, ", ")+"); a server could not be distinguished from a command")
	}
	return ix
}

// maxRoleOf is the highest-precedence role a manifest of this kind could
// establish on its own, used to bound what an unreadable manifest could change.
func maxRoleOf(k manifestKind) apimodel.RepoKind {
	switch k {
	case kindNPM, kindPubspec:
		return apimodel.KindMobile
	default:
		return apimodel.KindBackend
	}
}

// blockUninspectedManifests records each role-bearing manifest that could not be
// read or parsed: its declarations are unknown, so the roles it could establish
// cannot be ruled out.
func (ix *repoIndex) blockUninspectedManifests() {
	for _, m := range []manifestRef{ix.pkgRef, ix.goMod, ix.cargo, ix.pyproj, ix.setupPy, ix.pubspec, ix.reqs} {
		if m.present() && !m.readable() {
			ix.block(maxRoleOf(m.kind), ix.st.describe(m.path)+"; its declarations are unknown")
		}
	}
	unparsed := func(m manifestRef, ok bool) {
		if m.readable() && !ok {
			ix.block(maxRoleOf(m.kind), string(m.kind)+" at "+m.path+" could not be parsed; its signals were not used")
		}
	}
	unparsed(ix.pkgRef, !ix.pkgBad)
	unparsed(ix.pyproj, ix.pyOK)
	unparsed(ix.cargo, ix.cargoOK)
	unparsed(ix.goMod, ix.goModOK)
}

// scope is the directory-hint rank of a path within this component (see
// scopeRank): 0 at the component root, 1 in an ordinary subdirectory, higher in
// tooling, docs or example directories.
func (ix *repoIndex) scope(p string) int { return scopeRank(ix.comp.rel(p)) }

// covers reports whether the listing would show a path with n segments below
// the component root.
func (ix *repoIndex) covers(n int) bool { return ix.st.covers(ix.base + n) }

// note records a limitation; block records one that hides a role.
func (ix *repoIndex) note(s string) { ix.notes = append(ix.notes, s) }
func (ix *repoIndex) block(role apimodel.RepoKind, reason string) {
	ix.blocks = append(ix.blocks, block{role: role, reason: reason})
	ix.note(reason)
}

// limitations returns the component's limitations, in order, without repeats.
func (ix *repoIndex) limitations() []string { return dedupe(ix.notes) }

// componentName is the name the component's manifest declares for it, used to
// recognise when another component depends on this one.
func (ix *repoIndex) componentName() string {
	switch {
	case ix.pkg != nil && ix.pkg.Name != "":
		return ix.pkg.Name
	case ix.modPath != "":
		return ix.modPath
	case ix.cargoOK && ix.cargoM.Package != nil && ix.cargoM.Package.Name != "":
		return ix.cargoM.Package.Name
	case ix.pyHasProject && ix.pyProj.Project.Name != "":
		return pyDistName(ix.pyProj.Project.Name)
	}
	return ""
}

// isWorkspace reports whether the component's manifest declares a workspace that
// aggregates other components rather than a product of its own: a private npm
// package with "workspaces", a Cargo virtual manifest ([workspace] without
// [package]), or a go.work file with no module of its own.
func (ix *repoIndex) isWorkspace() bool {
	if ix.pkg != nil && ix.pkg.Private && len(ix.pkg.Workspaces) > 0 && string(ix.pkg.Workspaces) != "null" {
		return true
	}
	if ix.cargoOK && ix.cargoM.Workspace != nil && ix.cargoM.Package == nil {
		return true
	}
	return ix.goWork.present() && !ix.goMod.present()
}

// blockUninspectedPythonDependencies records Python dependency declarations the
// bounded parser does not read, so an empty or partial list can never stand in
// for "declares no web framework": dependencies declared dynamic in pyproject,
// requirement includes, constraints and URL/VCS requirements, and a setup.py
// whose install_requires is executable Python that is never evaluated.
func (ix *repoIndex) blockUninspectedPythonDependencies() {
	if ix.pyHasProject && ix.pyProj.dynamicDependencies() {
		ix.block(apimodel.KindBackend, ix.pyproj.path+" declares its dependencies dynamic; they are computed at build time and were not evaluated")
	}
	if ix.reqs.readable() {
		_, unsupported := parseRequirements(ix.reqs.content)
		for _, u := range unsupported {
			ix.block(apimodel.KindBackend, ix.reqs.path+" "+u)
		}
	}
	if ix.setupPy.present() && !ix.pyHasProject {
		ix.block(apimodel.KindBackend, ix.setupPy.path+" is executable Python; its install_requires was not evaluated, so its dependencies are unknown")
	}
}

// scopeRank is a coarse, directory-derived hint at a path's role, used to keep
// examples, docs and tooling from standing in for a product. It looks at the
// first path segment only; folder names are hints, not a component model.
func scopeRank(p string) int {
	if !strings.Contains(p, "/") {
		return 0
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
		return 1
	}
}

// unreadGoEntries lists primary-scope main.go paths the component owns whose
// content was not read.
func (ix *repoIndex) unreadGoEntries() []string {
	if !ix.isGo() {
		return nil
	}
	var unread []string
	for _, p := range ix.paths {
		if strings.ToLower(path.Base(p)) == "main.go" && ix.scope(p) <= 1 && !ix.isRead(p) {
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

// hasRelPrefix reports whether any owned path, relative to the component root,
// starts with the given (lowercased) prefix — used for "android/" and "ios/".
func (ix *repoIndex) hasRelPrefix(prefix string) bool {
	for _, lp := range ix.lowerRel {
		if strings.HasPrefix(lp, prefix) {
			return true
		}
	}
	return false
}

// firstPrimaryPath returns the first owned path at a primary scope whose
// lowercased component-relative form satisfies pred.
func (ix *repoIndex) firstPrimaryPath(pred func(lowerRel string) bool) (string, bool) {
	for i, lp := range ix.lowerRel {
		if ix.scope(ix.paths[i]) <= 1 && pred(lp) {
			return ix.paths[i], true
		}
	}
	return "", false
}

// --- mobile -----------------------------------------------------------------

func detectMobile(ix *repoIndex) (apimodel.RepoKind, []Observation, bool) {
	var ev []Observation
	// A mobile framework establishes a mobile app only as a runtime dependency; a
	// development or peer declaration describes tooling or an integration.
	for _, dep := range []string{"react-native", "expo"} {
		if obs, ok := ix.runtimeDep(dep, "mobile framework"); ok {
			ev = append(ev, obs)
		}
	}
	// Flutter projects declare a Flutter SDK dependency in pubspec.yaml — checked
	// from the parsed dependency tables, so the word "flutter" in a comment does
	// not count.
	if ix.pubspec.readable() && pubspecUsesFlutter(ix.pubspec.content) {
		ev = append(ev, Observation{Path: ix.pubspec.path, Detail: "declares a Flutter SDK dependency (mobile)"})
	}
	// Native shells side by side are a strong mobile signal on their own.
	if ix.hasRelPrefix("android/") && ix.hasRelPrefix("ios/") {
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

// runtimeDep returns an observation when name is a runtime dependency of the
// component's package.json, citing the declaring field. A declaration found
// only among devDependencies or peerDependencies is recorded as a note that
// names its table: it describes tooling or an integration a host provides, and
// does not by itself establish what the component is.
func (ix *repoIndex) runtimeDep(name, what string) (Observation, bool) {
	if ix.pkg == nil {
		return Observation{}, false
	}
	v, cat, ok := ix.pkg.lookupDep(name)
	switch {
	case !ok:
		return Observation{}, false
	case cat == depRuntime:
		return Observation{Path: ix.pkgRef.path, Detail: "declares " + quoteDep(name, v) + " in dependencies (" + what + ")"}, true
	case cat == depDev:
		ix.note(ix.pkgRef.path + " declares " + quoteDep(name, v) + " only in devDependencies — development tooling, which does not establish a " + what + " in the product")
	default:
		ix.note(ix.pkgRef.path + " declares " + quoteDep(name, v) + " only in peerDependencies — an integration the host application provides, which does not establish a " + what + " in this package")
	}
	return Observation{}, false
}

func detectFrontend(ix *repoIndex) (apimodel.RepoKind, []Observation, bool) {
	// A web framework establishes an application only as a *runtime* dependency.
	// A build tool in devDependencies (vite) or a peer dependency (a UI
	// library's react) does not make the package a frontend product.
	framework, version := ix.runtimeWebFramework()
	if framework == "" {
		return "", nil, false
	}
	// The app entry must live in this component's primary scope, so a nested
	// example or demo does not turn a library into a frontend. A dev/start
	// script alone is not an app entry. Without an entry, defer to detectLibrary.
	entry, ok := ix.frontendEntry()
	if !ok {
		return "", nil, false
	}
	ev := []Observation{{Path: ix.pkgRef.path, Detail: "declares " + quoteDep(framework, version) + " in dependencies (web framework)"}, entry}
	if ix.pkg.Private {
		ev = append(ev, Observation{Path: ix.pkgRef.path, Detail: `"private": true (not a published package)`})
	}
	return apimodel.KindFrontend, ev, true
}

// frontendEntry finds an app entry in the component: an index.html, a src/App
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

// hasPrimaryAppShell reports whether the component has an app shell (index.html
// or a src/App root component) — the stricter "this is an app" marker
// detectLibrary uses, which deliberately excludes a build config a library also
// ships.
func (ix *repoIndex) hasPrimaryAppShell() bool {
	if _, ok := ix.firstPrimaryPath(func(lp string) bool { return path.Base(lp) == "index.html" }); ok {
		return true
	}
	_, ok := ix.firstPrimaryPath(func(lp string) bool { return strings.HasPrefix(lp, "src/app.") })
	return ok
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
	// Node server frameworks, as runtime dependencies only: a development or peer
	// declaration describes tooling or an integration, not the product's server.
	for _, dep := range nodeServerDeps {
		if obs, ok := ix.runtimeDep(dep, "server framework"); ok {
			ev = append(ev, obs)
			break
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
					ix.block(apimodel.KindBackend, "cannot conclude the command starts no server: "+w)
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
			ix.block(apimodel.KindBackend, "cannot conclude the binary target is a command: "+ix.st.describe(ix.cargo.path)+"; a web-framework dependency may lie in the unread remainder")
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
				ix.block(apimodel.KindBackend, "no importable Go package clause was inspected"+ix.unreadGoSample()+"; the module's packages are unknown")
			case !ok:
				for _, w := range why {
					ix.block(apimodel.KindBackend, "cannot conclude the Go module has no command: "+w)
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
				ix.block(apimodel.KindCLI, "cannot conclude the package installs no command: "+ix.st.describe(ix.pkgRef.path))
				ok = false
			}
			if fw, _ := ix.runtimeWebFramework(); fw != "" && !ix.covers(2) {
				ix.block(apimodel.KindFrontend, "cannot rule out an app entry for runtime dependency \""+fw+"\": the listing does not cover index.html, src/App or src/main")
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
			ix.block(apimodel.KindBackend, "a library target is present, but "+why)
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
			ix.block(apimodel.KindCLI, "cannot conclude the package declares no command entry: "+ix.st.describe(src)+"; entry points may lie in the unread remainder")
		}
	}

	if len(ev) > 0 {
		return apimodel.KindLibrary, ev, true
	}
	return "", nil, false
}

// --- language / Go helpers ---------------------------------------------------

// isGo reports whether the component is a Go module (or, for the repository
// root, a Go repository without a module file).
func (ix *repoIndex) isGo() bool {
	return ix.goMod.present() || (ix.isRoot && ix.language == "go")
}

func isGoSource(p string) bool {
	lp := strings.ToLower(p)
	return strings.HasSuffix(lp, ".go") && !strings.HasSuffix(lp, "_test.go")
}

// primaryGoFiles are the read Go sources in the component's primary scope —
// excluding example, docs and tooling directories.
func (ix *repoIndex) primaryGoFiles() []goFile {
	var out []goFile
	for _, g := range ix.goFiles {
		if ix.scope(g.path) <= 1 {
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
		for _, u := range g.uncertain {
			why = append(why, p+" "+u)
		}
	}
	for _, p := range ix.paths {
		check(p)
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
		dir := "/" + path.Dir(ix.comp.rel(g.path)) + "/"
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
//   - every listed main.go to have been read (enforced by the unread-entry block).
//
// A command placed in some other directory, under a file name other than
// main.go, is outside this bounded scope; that blind spot is documented here
// rather than closed by fetching every file.
func (ix *repoIndex) goNoCommandPremise() (bool, []string) {
	var why []string
	rootListed, rootKnown, rootExample := false, false, ""
	for _, p := range ix.goCandidatePaths() {
		if dirOf(p) != ix.comp.root {
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
	case !rootListed && !ix.covers(1):
		why = append(why, "the listing may omit Go files at the module root; the root could be a command")
	}
	if !ix.covers(3) {
		why = append(why, "the listing does not cover cmd/<name>/; a command there cannot be ruled out")
	}
	for _, p := range ix.goCandidatePaths() {
		if strings.HasPrefix(strings.ToLower(ix.comp.rel(p)), "cmd/") && !ix.isRead(p) {
			why = append(why, ix.st.describe(p)+"; it may be a command")
		}
	}
	return len(why) == 0, dedupe(why)
}

// goCandidatePaths lists every primary-scope, non-test Go source the component
// owns, path-sorted.
func (ix *repoIndex) goCandidatePaths() []string {
	var out []string
	for _, p := range ix.paths {
		if isGoSource(p) && ix.scope(p) <= 1 {
			out = append(out, p)
		}
	}
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

// goServerObservation returns evidence that a Go component is a server: a read
// source containing a call the type checker resolved to a net/http server entry
// point (see scanServerCalls), or a declared server-framework dependency in
// go.mod. Both are positive static facts, so they may come from a partial file;
// neither says the server was observed running.
func (ix *repoIndex) goServerObservation() (Observation, bool) {
	for _, g := range ix.primaryGoFiles() {
		if len(g.serverCalls) > 0 {
			c := g.serverCalls[0]
			what := "net/http package function"
			if c.method {
				what = "method on *net/http.Server"
			}
			return Observation{Path: g.path, Detail: "contains a call expression " + c.callee + "(…) at line " + strconv.Itoa(c.line) + ", resolved to the " + what + " " + c.callee[strings.LastIndex(c.callee, ".")+1:] + " (static syntax, not observed running)"}, true
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

// rustBinTarget reports whether the crate has a binary target: yes from an
// explicit [[bin]] table or Cargo's implicit src/main.rs / src/bin/*.rs; no only
// when the full Cargo.toml was read and the listing covers those implicit
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
	if auto && !ix.covers(3) {
		return unknown, "the listing does not cover src/main.rs and src/bin/, so a binary target cannot be ruled out"
	}
	return no, ""
}

// rustHasLibTarget reports whether the crate has a library target, from an
// explicit [lib] table or — when autolib is on — Cargo's implicit src/lib.rs.
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

// pyPackageSource reports the file that marks the component as a real Python
// package — a pyproject.toml with a [project] table, or a setup.py — and whether
// one was found. A pyproject with only [tool.*] tables does not count.
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

// pyServerObservation returns evidence that the component requires a Python web
// framework at runtime: a declaration in [project].dependencies or
// requirements.txt, cited with its declaring field and specifier. A framework
// found only in an optional group ([project.optional-dependencies].<group>) is
// installed only on request — for docs, tests or an integration — so it is
// recorded as a note naming the group, not as role evidence. The framework list
// is iterated in fixed order, so the result never depends on map order.
func (ix *repoIndex) pyServerObservation() (Observation, bool) {
	for _, want := range pyServerDeps {
		name := pyDistName(want)
		for _, d := range ix.comp.deps {
			if d.Name != name {
				continue
			}
			switch {
			case d.Category == "project.dependencies" || d.Category == "requirements":
				return Observation{Path: d.Source, Detail: "declares \"" + d.Spec + "\" in " + d.Category + " (web framework)"}, true
			case strings.HasPrefix(d.Category, "project.optional-dependencies."):
				ix.note(d.Source + " declares \"" + d.Spec + "\" only in the optional group " + d.Category + " — an extra installed on request, which does not establish a web application")
			}
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
	Name             string            `json:"name"`
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
