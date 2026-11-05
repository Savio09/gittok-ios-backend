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
	if ix.pkg != nil {
		if _, state := ix.pkg.commandEntry(); state == unknown {
			ix.block(apimodel.KindCLI, ix.pkgRef.path+" declares malformed or unsupported bin metadata; its command entries could not be established")
		}
	}
	ix.blockUninspectedPythonDependencies()
	ix.blockUninspectedPythonEntries()
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

// componentNameWitness is the field that declares componentName, with its
// verified location when one exists; the zero Witness when there is none.
func (ix *repoIndex) componentNameWitness() Witness {
	var w Witness
	switch {
	case ix.pkg != nil && ix.pkg.Name != "":
		w = Witness{Kind: WitnessField, Path: ix.pkgRef.path, Field: []string{"name"}, Value: ix.pkg.Name}
	case ix.modPath != "":
		w = Witness{Kind: WitnessField, Path: ix.goMod.path, Field: []string{"module"}, Value: ix.modPath}
	case ix.cargoOK && ix.cargoM.Package != nil && ix.cargoM.Package.Name != "":
		w = Witness{Kind: WitnessField, Path: ix.cargo.path, Field: []string{"package", "name"}, Value: ix.cargoM.Package.Name}
	case ix.pyHasProject && ix.pyProj.Project.Name != "":
		w = Witness{Kind: WitnessField, Path: ix.pyproj.path, Field: []string{"project", "name"}, Value: ix.pyProj.Project.Name}
	default:
		return w
	}
	if validateWitness(ix.st, w) != nil {
		return Witness{}
	}
	return ix.st.locate(w)
}

// isWorkspace reports whether the component's manifest aggregates other
// components, possibly alongside a product of its own: a private npm
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

// --- witnessed evidence ------------------------------------------------------

// evidenceGroup is a conjunction of observations that together support a role:
// a Go command and its server call, a binary target and its CLI parser. A
// candidate role is a disjunction of groups — any group whose observations all
// validate supports it. A group whose observations are all Context supports
// nothing on its own; it accompanies a supported role.
type evidenceGroup []Observation

// fieldObs, spanObs, listingObs and scopeObs build an observation together with
// the witness that ties it to the inspection (see witness.go).
func fieldObs(file string, field []string, value, detail string) Observation {
	return Observation{Path: file, Detail: detail, Witness: Witness{Kind: WitnessField, Path: file, Field: field, Value: value}}
}

func spanObs(file string, start, end int, quote, construct, detail string) Observation {
	return Observation{Path: file, Detail: detail, Witness: Witness{Kind: WitnessSpan, Path: file, Start: start, End: end, Quote: quote, Construct: construct}}
}

func listingObs(paths []string, construct, detail string) Observation {
	o := Observation{Detail: detail, Witness: Witness{Kind: WitnessListing, Paths: paths, Construct: construct}}
	if len(paths) == 1 {
		o.Path = paths[0]
	}
	return o
}

func scopeObs(file string, paths []string, construct string, depth int, absence AbsenceKind, detail string) Observation {
	return Observation{Path: file, Detail: detail, Absence: absence, Witness: Witness{Kind: WitnessScope, Paths: paths, Construct: construct, Depth: depth}}
}

func contextObs(o Observation) Observation {
	o.Context = true
	return o
}

// commandObs cites a Go command by its package clause.
func commandObs(g goFile) Observation {
	return spanObs(g.path, g.pkgStart, g.pkgEnd, "main", constructGoPackageMain, "package main (produces a binary)")
}

// firstRelPrefix returns the first owned path whose component-relative form
// starts with prefix, or "".
func (ix *repoIndex) firstRelPrefix(prefix string) string {
	for i, lp := range ix.lowerRel {
		if strings.HasPrefix(lp, prefix) {
			return ix.paths[i]
		}
	}
	return ""
}

// --- mobile -----------------------------------------------------------------

func detectMobile(ix *repoIndex) (apimodel.RepoKind, []evidenceGroup) {
	var groups []evidenceGroup
	// A mobile framework establishes a mobile app only as a runtime dependency; a
	// development or peer declaration describes tooling or an integration.
	for _, dep := range []string{"react-native", "expo"} {
		if obs, ok := ix.runtimeDep(dep, "mobile framework"); ok {
			groups = append(groups, evidenceGroup{obs})
		}
	}
	// Flutter projects declare a Flutter SDK dependency in pubspec.yaml — read
	// from the parsed dependency tables, so the word in a comment does not count.
	if ix.pubspec.readable() {
		if table := pubspecFlutterTable(ix.pubspec.content); table != "" {
			groups = append(groups, evidenceGroup{fieldObs(ix.pubspec.path, []string{table, "flutter"}, "", "declares a Flutter SDK dependency in "+table+" (mobile)")})
		}
	}
	// Native shells side by side are a strong mobile signal on their own; the
	// witness is the listed paths that establish both directories.
	if android, ios := ix.firstRelPrefix("android/"), ix.firstRelPrefix("ios/"); android != "" && ios != "" {
		groups = append(groups, evidenceGroup{listingObs([]string{android, ios}, constructNativeShells, "android/ and ios/ app directories both present (native mobile shells)")})
	}
	return apimodel.KindMobile, groups
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

// runtimeDep returns a witnessed observation when name is a runtime dependency
// of the component's package.json. A declaration found only among
// devDependencies or peerDependencies is recorded as a note that names its
// table: it describes tooling or an integration a host provides, and does not
// by itself establish what the component is.
func (ix *repoIndex) runtimeDep(name, what string) (Observation, bool) {
	if ix.pkg == nil {
		return Observation{}, false
	}
	v, cat, ok := ix.pkg.lookupDep(name)
	switch {
	case !ok:
		return Observation{}, false
	case cat == depRuntime:
		return fieldObs(ix.pkgRef.path, []string{string(depRuntime), name}, v, "declares "+quoteDep(name, v)+" in dependencies ("+what+")"), true
	case cat == depDev:
		ix.note(ix.pkgRef.path + " declares " + quoteDep(name, v) + " only in devDependencies — development tooling, which does not establish a " + what + " in the product")
	default:
		ix.note(ix.pkgRef.path + " declares " + quoteDep(name, v) + " only in peerDependencies — an integration the host application provides, which does not establish a " + what + " in this package")
	}
	return Observation{}, false
}

func detectFrontend(ix *repoIndex) (apimodel.RepoKind, []evidenceGroup) {
	// A web framework establishes an application only as a *runtime* dependency.
	// A build tool in devDependencies (vite) or a peer dependency (a UI
	// library's react) does not make the package a frontend product.
	framework, version := ix.runtimeWebFramework()
	if framework == "" {
		return apimodel.KindFrontend, nil
	}
	// The app entry must live in this component's primary scope, so a nested
	// example or demo does not turn a library into a frontend. A dev/start
	// script alone is not an app entry. Without an entry, defer to detectLibrary.
	entry, ok := ix.frontendEntry()
	if !ok {
		return apimodel.KindFrontend, nil
	}
	groups := []evidenceGroup{{
		fieldObs(ix.pkgRef.path, []string{string(depRuntime), framework}, version, "declares "+quoteDep(framework, version)+" in dependencies (web framework)"),
		entry,
	}}
	if ix.pkg.Private {
		groups = append(groups, evidenceGroup{contextObs(fieldObs(ix.pkgRef.path, []string{"private"}, "true", `"private": true (not a published package)`))})
	}
	return apimodel.KindFrontend, groups
}

// frontendEntry finds an app entry in the component: an index.html, a src/App
// root component, a src/main bootstrap, or an app build config (vite/next/…),
// witnessed by the listed path.
func (ix *repoIndex) frontendEntry() (Observation, bool) {
	if p, ok := ix.firstPrimaryPath(func(lp string) bool { return path.Base(lp) == "index.html" }); ok {
		return listingObs([]string{p}, constructIndexHTML, "web app entry (index.html)"), true
	}
	if p, ok := ix.firstPrimaryPath(func(lp string) bool { return strings.HasPrefix(lp, "src/app.") }); ok {
		return listingObs([]string{p}, constructSrcApp, "root component (src/App)"), true
	}
	if p, ok := ix.firstPrimaryPath(func(lp string) bool { return strings.HasPrefix(lp, "src/main.") }); ok {
		return listingObs([]string{p}, constructSrcMain, "app bootstrap (src/main)"), true
	}
	if p, ok := ix.firstPrimaryPath(isAppConfig); ok {
		return listingObs([]string{p}, constructAppConfig, "app build configuration"), true
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
	goServerDeps   = []string{"github.com/gin-gonic/gin", "github.com/labstack/echo", "github.com/gofiber/fiber", "github.com/go-chi/chi", "github.com/gorilla/mux"}
	nodeServerDeps = []string{"express", "fastify", "koa", "@nestjs/core", "@hapi/hapi", "hapi"}
	pyServerDeps   = []string{"fastapi", "flask", "django", "starlette", "uvicorn", "sanic", "aiohttp", "tornado"}
	rustServerDeps = []string{"axum", "actix-web", "rocket", "warp"}
)

func detectBackend(ix *repoIndex) (apimodel.RepoKind, []evidenceGroup) {
	var groups []evidenceGroup

	// Go: a server needs both a command (a package clause naming main) and a
	// server signal (a resolved server call, or a declared server framework).
	if _, ok := ix.goCommand(); ok {
		if serverObs, ok := ix.goServerObservation(); ok {
			groups = append(groups, append(evidenceGroup(ix.commandObservations()), serverObs))
		}
	}
	// Node server frameworks, as runtime dependencies only.
	for _, dep := range nodeServerDeps {
		if obs, ok := ix.runtimeDep(dep, "server framework"); ok {
			groups = append(groups, evidenceGroup{obs})
			break
		}
	}
	// Python web frameworks, as mandatory or requirements declarations only.
	if obs, ok := ix.pyServerObservation(); ok {
		groups = append(groups, evidenceGroup{obs})
	}
	// Rust: a binary target that depends on a web framework.
	if bin, binObs, _ := ix.rustBinTarget(); bin == yes {
		if dep := cargoDep(ix.cargoM, rustServerDeps); dep != "" {
			groups = append(groups, evidenceGroup{binObs, fieldObs(ix.cargo.path, []string{"dependencies", dep}, "", "a binary target depends on \""+dep+"\" (web framework)")})
		}
	}
	// A Dockerfile's EXPOSE is deployment metadata: a port the image declares it
	// intends to publish, not evidence of what the product is or that anything
	// listens on it. It is kept as supporting context beside independent backend
	// evidence and otherwise surfaced as a note; it never decides the role.
	if ix.docker.readable() {
		if start, end, port := dockerExposeSpan(ix.docker.content); port != "" {
			if len(groups) > 0 {
				groups = append(groups, evidenceGroup{contextObs(spanObs(ix.docker.path, start, end, "", constructDockerExpose, "declares EXPOSE "+port+" (a deployment port the image declares; not observed listening)"))})
			} else {
				ix.note(ix.docker.path + " declares EXPOSE " + port + " — a deployment port the image declares, which does not by itself establish a backend")
			}
		}
	}
	return apimodel.KindBackend, groups
}

// --- cli ---------------------------------------------------------------------

var (
	goCLIDeps = []string{
		"github.com/spf13/cobra", "github.com/urfave/cli", "gopkg.in/urfave/cli.v1",
		"github.com/spf13/pflag", "github.com/alecthomas/kong", "github.com/alecthomas/kingpin", "gopkg.in/alecthomas/kingpin.v2",
	}
	rustCLIDeps = []string{"clap", "structopt", "argh"}
)

func detectCLI(ix *repoIndex) (apimodel.RepoKind, []evidenceGroup) {
	var groups []evidenceGroup

	// Go: a command wired to a CLI framework, or a command shown to start no
	// server. The latter is an absence premise; see goNoServerPremise for the
	// scope it requires, which its scope witness re-checks.
	if _, ok := ix.goCommand(); ok {
		if req, ver := ix.goRequirement(goCLIDeps); req != "" {
			groups = append(groups, append(evidenceGroup(ix.commandObservations()),
				fieldObs(ix.goMod.path, []string{"require", req}, ver, "requires \""+req+"\" (CLI framework)")))
		} else if _, server := ix.goServerObservation(); !server {
			// Every command must be shown to start no server, not just one.
			group, held := evidenceGroup{}, true
			for _, cmd := range ix.goCommands() {
				ok, why := ix.goNoServerPremise(cmd)
				for _, w := range why {
					ix.block(apimodel.KindBackend, "cannot conclude the command starts no server: "+w)
				}
				held = held && ok
				group = append(group, commandObs(cmd), scopeObs(cmd.path, ix.noServerScope(cmd), constructGoNoServer, 0, AbsenceGoServer,
					"the command's package was inspected in full and contains no server call (produces a command)"))
			}
			if held {
				groups = append(groups, group)
			}
		}
	}
	// npm: a supported executable declaration, not merely the presence of bin.
	if ix.pkg != nil {
		if cmd, state := ix.pkg.commandEntry(); state == yes {
			groups = append(groups, evidenceGroup{fieldObs(ix.pkgRef.path, cmd.field, cmd.target, `declares a "bin" field with an executable entry (installs a command)`)})
		}
	}
	// Python: a static [project.scripts] table installs commands. GUI entry
	// points ([project.gui-scripts]) are deliberately not a CLI. Legacy
	// entry_points in setup.py are never read as a command: setup.py is
	// executable Python that is not evaluated, so its entry points are recorded
	// as uninspected (see blockUninspectedPythonEntries), and a comment or string
	// that mentions console_scripts can never establish one.
	if ix.pyHasProject && len(ix.pyProj.Project.Scripts) > 0 {
		names := sortedKeys(ix.pyProj.Project.Scripts)
		groups = append(groups, evidenceGroup{fieldObs(ix.pyproj.path, []string{"project", "scripts", names[0]}, ix.pyProj.Project.Scripts[names[0]],
			"declares [project.scripts] entries "+strings.Join(names, ", ")+" (installs a command)")})
	}
	// Rust: a binary target. A CLI-parser dependency is cited when present; a
	// binary with no web-framework dependency is a command only when the full
	// Cargo.toml was read, so the dependency's absence is real.
	if bin, binObs, why := ix.rustBinTarget(); bin == unknown && ix.cargo.present() {
		ix.block(apimodel.KindBackend, why)
	} else if bin == yes {
		cliDep, serverDep := cargoDep(ix.cargoM, rustCLIDeps), cargoDep(ix.cargoM, rustServerDeps)
		switch {
		case cliDep != "":
			groups = append(groups, evidenceGroup{binObs, fieldObs(ix.cargo.path, []string{"dependencies", cliDep}, "", "a binary target depends on \""+cliDep+"\" (CLI parser)")})
		case serverDep != "":
			// a server binary; detectBackend owns it
		case ix.cargo.complete():
			groups = append(groups, evidenceGroup{binObs, scopeObs(ix.cargo.path, []string{ix.cargo.path}, constructCompleteReads, 0, AbsenceCargoWebFramework,
				"a binary target with no web-framework dependency (produces a command)")})
		default:
			ix.block(apimodel.KindBackend, "cannot conclude the binary target is a command: "+ix.st.describe(ix.cargo.path)+"; a web-framework dependency may lie in the unread remainder")
		}
	}
	return apimodel.KindCLI, groups
}

// --- library -----------------------------------------------------------------

func detectLibrary(ix *repoIndex) (apimodel.RepoKind, []evidenceGroup) {
	var groups []evidenceGroup

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
				groups = append(groups, evidenceGroup{
					fieldObs(ix.goMod.path, []string{"module"}, ix.modPath, "declares module "+ix.modPath),
					spanObs(pkgFile.path, pkgFile.pkgStart, pkgFile.pkgEnd, pkgFile.pkg, constructGoPackageName, "declares package "+pkgFile.pkg+" (importable)"),
					scopeObs(ix.goMod.path, ix.noCommandScope(), constructGoNoCommand, ix.base+3, AbsenceGoCommand,
						"no package main at the conventional command locations (module root, cmd/)"),
				})
			}
		}
	}
	// npm: a lib entry (main/module/exports) and no app entry or bin. Both
	// absences are premises: "no bin" needs the full package.json; "no app entry"
	// matters only when a runtime web framework is declared, and then needs a
	// listing that covers the conventional entry locations.
	if ix.pkg != nil {
		entryField := ""
		switch {
		case ix.pkg.Main != "":
			entryField = "main"
		case ix.pkg.Module != "":
			entryField = "module"
		case len(ix.pkg.Exports) > 0 && string(ix.pkg.Exports) != "null":
			entryField = "exports"
		}
		_, binState := ix.pkg.commandEntry()
		if entryField != "" && binState == no && !ix.hasPrimaryAppShell() {
			ok := true
			if !ix.pkgRef.complete() {
				ix.block(apimodel.KindCLI, "cannot conclude the package installs no command: "+ix.st.describe(ix.pkgRef.path))
				ok = false
			}
			// An app shell (index.html) may lie at any depth, so its absence
			// needs a complete listing, with or without a runtime framework.
			fw, _ := ix.runtimeWebFramework()
			if !ix.st.scope.Complete() {
				ix.block(apimodel.KindFrontend, "cannot rule out an app shell: the listing is incomplete, and index.html may lie beyond it")
				ok = false
			}
			if ok {
				premise := scopeObs(ix.pkgRef.path, []string{ix.pkgRef.path}, constructCompleteListing, 0, AbsenceNPMCommandAndShell, "declares no executable command entry in \"bin\" and the component has no app shell")
				if fw != "" {
					premise = scopeObs(ix.pkgRef.path, []string{ix.pkgRef.path}, constructCompleteListing, 0, AbsenceNPMCommandAndAppEntry, "declares no executable command entry in \"bin\" and the listing shows no app entry for runtime dependency \""+fw+"\"")
				}
				groups = append(groups, evidenceGroup{
					fieldObs(ix.pkgRef.path, []string{entryField}, "", `declares a lib entry ("`+entryField+`") and no app or executable command (importable package)`),
					premise,
				})
			}
		}
	}
	// Rust: a library target and no binary target. The binary's absence needs
	// the full Cargo.toml and a listing that covers Cargo's implicit targets.
	if libObs, ok := ix.rustLibTarget(); ok {
		switch bin, _, why := ix.rustBinTarget(); bin {
		case no:
			premise := scopeObs(ix.cargo.path, []string{ix.cargo.path}, constructCompleteReads, 0, AbsenceCargoBinary, "no binary target (importable crate)")
			if cargoAutobins(ix.cargoM) {
				premise = scopeObs(ix.cargo.path, []string{ix.cargo.path}, constructCoveringListing, ix.base+4, AbsenceCargoBinaryListing, "no [[bin]] table or implicit binary entry in a covering listing (importable crate)")
			}
			groups = append(groups, evidenceGroup{libObs, premise})
		case unknown:
			ix.block(apimodel.KindBackend, "a library target is present, but "+why)
		}
	}
	// Python: a static [project] table declaring no command or GUI entry points
	// is an importable package. "No entry points" needs the full pyproject.toml
	// and entry points that are not marked dynamic; a setup.py-only package's
	// entry points are never evaluated, so it is not concluded importable-only
	// (see blockUninspectedPythonEntries). A package with GUI entry points is an
	// application, not importable-only; the taxonomy has no desktop-app kind, so
	// it gets no role rather than a wrong one.
	if ix.pyHasProject && len(ix.pyProj.Project.Scripts) == 0 {
		switch {
		case len(ix.pyProj.Project.GUIScripts) > 0:
			ix.note(ix.pyproj.path + " declares [project.gui-scripts] entries " + strings.Join(sortedKeys(ix.pyProj.Project.GUIScripts), ", ") + " — a GUI application, which is neither a CLI nor an importable-only package")
		case !ix.pyproj.complete():
			ix.block(apimodel.KindCLI, "cannot conclude the package declares no command entry: "+ix.st.describe(ix.pyproj.path)+"; entry points may lie in the unread remainder")
		case !ix.pyProj.dynamicEntryPoints():
			groups = append(groups, evidenceGroup{
				fieldObs(ix.pyproj.path, []string{"project"}, "", "declares a static [project] table"),
				scopeObs(ix.pyproj.path, []string{ix.pyproj.path}, constructCompleteReads, 0, AbsencePythonEntryPoints, "with no command or GUI entry points (importable package)"),
			})
		}
	}
	return apimodel.KindLibrary, groups
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
	cmds := ix.goCommands()
	if len(cmds) == 0 {
		return goFile{}, false
	}
	return cmds[0], true
}

// goCommands returns one read file per primary-scope command directory (a
// package clause naming main), path-sorted, so evidence cites every command the
// inspection saw rather than only the first.
func (ix *repoIndex) goCommands() []goFile {
	var out []goFile
	seen := map[string]bool{}
	for _, g := range ix.primaryGoFiles() {
		if dir := path.Dir(g.path); !g.test && g.pkg == "main" && !seen[dir] {
			seen[dir] = true
			out = append(out, g)
		}
	}
	return out
}

// commandObservations cites every read command.
func (ix *repoIndex) commandObservations() []Observation {
	var out []Observation
	for _, c := range ix.goCommands() {
		out = append(out, commandObs(c))
	}
	return out
}

// goNoServerPremise reports whether "this command starts no server" is backed by
// inspected source. Its scope is the command's own package and the module's
// declared requirements:
//
//   - the listing covers every file at the command directory's depth;
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
	if err := validateGoPackageListing(ix.st, ix.noServerScope(cmd)); err != nil {
		why = append(why, err.Error())
	}
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
	// Prefer the module root's own package — the one the module path imports.
	files := ix.primaryGoFiles()
	sort.SliceStable(files, func(i, j int) bool {
		return dirOf(files[i].path) == ix.comp.root && dirOf(files[j].path) != ix.comp.root
	})
	for _, g := range files {
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
//     have an established package clause;
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
		if strings.HasPrefix(strings.ToLower(ix.comp.rel(p)), "cmd/") {
			g, read := ix.goFileAt(p)
			switch {
			case !read:
				why = append(why, ix.st.describe(p)+"; it may be a command")
			case g.pkg == "":
				why = append(why, p+" has no established package clause; it may be a command")
			}
		}
	}
	return len(why) == 0, dedupe(why)
}

// noCommandScope is the basis of goNoCommandPremise: go.mod, then every Go
// source under cmd/ and the read module-root sources whose package clauses it
// consulted, path-sorted. Its witness re-derives each clause (see
// validateGoNoCommand), so no premise rests on a tree name alone.
func (ix *repoIndex) noCommandScope() []string {
	var files []string
	for _, p := range ix.goCandidatePaths() {
		rel := ix.comp.rel(p)
		if strings.HasPrefix(strings.ToLower(rel), "cmd/") {
			files = append(files, p)
		} else if dirOf(p) == ix.comp.root {
			if g, ok := ix.goFileAt(p); ok && g.pkg != "" {
				files = append(files, p)
			}
		}
	}
	sort.Strings(files)
	return append([]string{ix.goMod.path}, files...)
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
// point (see scanServerCalls), witnessed by the call's exact span; or a declared
// server-framework requirement in go.mod, witnessed by the require field. Both
// are positive static facts, so they may come from a partial file; neither says
// the server was observed running.
func (ix *repoIndex) goServerObservation() (Observation, bool) {
	for _, g := range ix.primaryGoFiles() {
		// go build excludes _test.go; test calls are not product-role evidence.
		// https://pkg.go.dev/cmd/go@go1.25.5#hdr-Compile_packages_and_dependencies
		if g.test {
			continue
		}
		if len(g.serverCalls) > 0 {
			c := g.serverCalls[0]
			what := "net/http package function"
			if c.method {
				what = "method on *net/http.Server"
			}
			detail := "contains a call expression " + c.callee + "(…) at line " + strconv.Itoa(c.line) + ", resolved to the " + what + " " +
				c.callee[strings.LastIndex(c.callee, ".")+1:] + " (static syntax, not observed running)"
			return spanObs(g.path, c.start, c.end, g.content[c.start:c.end], constructGoServerCall, detail), true
		}
	}
	if req, ver := ix.goRequirement(goServerDeps); req != "" {
		return fieldObs(ix.goMod.path, []string{"require", req}, ver, "requires \""+req+"\" (server framework)"), true
	}
	return Observation{}, false
}

// goRequirement returns the full module path and version of the first go.mod
// requirement matching one of wants (canonical module families),
// parsed with the official module-file parser so a module named only in a
// comment never counts.
func (ix *repoIndex) goRequirement(wants []string) (modPath, version string) {
	if !ix.goMod.readable() {
		return "", ""
	}
	f, err := parseGoModSource(ix.goMod.path, ix.goMod.content)
	if err != nil {
		return "", ""
	}
	for _, w := range wants {
		for _, r := range f.Require {
			if matchesGoModule(r.Mod.Path, w) {
				return r.Mod.Path, r.Mod.Version
			}
		}
	}
	return "", ""
}

// noServerScope is the basis of the no-server premise: go.mod and every non-test
// Go file the component lists in the command's directory.
func (ix *repoIndex) noServerScope(cmd goFile) []string {
	paths := []string{ix.goMod.path}
	dir := path.Dir(cmd.path)
	for _, p := range ix.paths {
		if path.Dir(p) == dir && isGoSource(p) {
			paths = append(paths, p)
		}
	}
	return paths
}

// --- Rust helpers ------------------------------------------------------------

// tristate is a premise's answer when its scope may not have been inspected.
type tristate int

const (
	unknown tristate = iota
	yes
	no
)

// rustBinTarget reports whether the crate has a binary target: yes, with the
// witnessed observation, from an explicit [[bin]] table or Cargo's implicit
// src/main.rs, src/bin/<name>.rs or src/bin/<name>/main.rs; no only when the full
// Cargo.toml was read and the listing covers those locations; unknown otherwise.
func (ix *repoIndex) rustBinTarget() (tristate, Observation, string) {
	if !ix.cargo.readable() || !ix.cargoOK {
		return unknown, Observation{}, "Cargo.toml was not read and parsed"
	}
	if len(ix.cargoM.Bin) > 0 {
		return yes, fieldObs(ix.cargo.path, []string{"bin"}, "", "declares a [[bin]] target"), ""
	}
	if !cargoBinDiscoveryKnown(ix.cargoM, ix.cargo.complete()) {
		return unknown, Observation{}, ix.st.describe(ix.cargo.path) + "; binary autodiscovery settings are unknown"
	}
	auto := cargoAutobins(ix.cargoM)
	if auto {
		for _, p := range ix.paths {
			if cargoImplicitBin(ix.comp.rel(p)) {
				o := listingObs([]string{p}, constructRustBinTarget, p+" (Cargo's implicit binary target)")
				o.Witness.Path = ix.cargo.path
				return yes, o, ""
			}
		}
	}
	if !ix.cargo.complete() {
		return unknown, Observation{}, ix.st.describe(ix.cargo.path) + "; a [[bin]] table may lie in the unread remainder"
	}
	if auto && !ix.covers(4) {
		return unknown, Observation{}, "the listing does not cover src/main.rs and src/bin/, so a binary target cannot be ruled out"
	}
	return no, Observation{}, ""
}

// rustLibTarget returns the crate's library target, witnessed by an explicit
// [lib] table or — when autolib is on — Cargo's implicit src/lib.rs.
func (ix *repoIndex) rustLibTarget() (Observation, bool) {
	if !ix.cargoOK {
		return Observation{}, false
	}
	if ix.cargoM.Lib != nil {
		return fieldObs(ix.cargo.path, []string{"lib"}, "", "declares a [lib] target"), true
	}
	if !cargoLibDiscoveryKnown(ix.cargoM, ix.cargo.complete()) || !cargoAutolib(ix.cargoM) {
		return Observation{}, false
	}
	if p, ok := ix.firstPrimaryPath(func(lp string) bool { return lp == "src/lib.rs" }); ok {
		o := listingObs([]string{p}, constructRustLibTarget, p+" (Cargo's implicit library target)")
		o.Witness.Path = ix.cargo.path
		return o, true
	}
	return Observation{}, false
}

// --- Python helpers ----------------------------------------------------------

// blockUninspectedPythonEntries records Python entry-point metadata the parser
// does not read, so it can neither establish a command nor justify "no
// commands": entry points pyproject marks dynamic, and the entry_points of a
// setup.py-only package (executable Python that is never evaluated).
func (ix *repoIndex) blockUninspectedPythonEntries() {
	if ix.pyHasProject && ix.pyProj.dynamicEntryPoints() {
		ix.block(apimodel.KindCLI, ix.pyproj.path+" declares its entry points dynamic; they are computed at build time and were not evaluated")
	}
	if ix.setupPy.present() && !ix.pyHasProject {
		ix.block(apimodel.KindCLI, ix.setupPy.path+" may declare entry_points (e.g. console_scripts) in its setup() call, which is executable Python and was not evaluated")
	}
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
			field := append(strings.Split(d.Category, "."), d.Name)
			switch {
			case d.Category == "project.dependencies" || d.Category == "requirements":
				return fieldObs(d.Source, field, d.Spec, "declares \""+d.Spec+"\" in "+d.Category+" (web framework)"), true
			case strings.HasPrefix(d.Category, "project.optional-dependencies."):
				ix.note(d.Source + " declares \"" + d.Spec + "\" only in the optional group " + d.Category + " — an extra installed on request, which does not establish a web application")
			}
		}
	}
	return Observation{}, false
}

// --- small pure utilities ----------------------------------------------------

// dockerExposeSpan returns the byte span of the first EXPOSE instruction in a
// Dockerfile and the first port it declares; port is "" when there is none.
func dockerExposeSpan(dockerfile string) (start, end int, port string) {
	offset := 0
	for line := range strings.SplitSeq(dockerfile, "\n") {
		trimmed := strings.TrimSpace(line)
		if fields := strings.Fields(trimmed); len(fields) >= 2 && strings.EqualFold(fields[0], "EXPOSE") {
			s := offset + strings.Index(line, trimmed)
			return s, s + len(trimmed), fields[1]
		}
		offset += len(line) + 1
	}
	return 0, 0, ""
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
	Name                 string            `json:"name"`
	Private              bool              `json:"private"`
	Main                 string            `json:"main"`
	Module               string            `json:"module"`
	Exports              json.RawMessage   `json:"exports"`
	Bin                  json.RawMessage   `json:"bin"`
	Workspaces           json.RawMessage   `json:"workspaces"`
	Scripts              map[string]string `json:"scripts"`
	Dependencies         map[string]string `json:"dependencies"`
	DevDependencies      map[string]string `json:"devDependencies"`
	PeerDependencies     map[string]string `json:"peerDependencies"`
	OptionalDependencies map[string]string `json:"optionalDependencies"`
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
