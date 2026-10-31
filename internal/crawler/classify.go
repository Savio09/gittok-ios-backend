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
//	mobile → frontend → backend → cli → library → other
//
//   - mobile is first because react-native/expo apps also depend on react, so
//     they would otherwise be mistaken for a web frontend.
//   - frontend precedes backend so a client app that happens to ship a
//     Dockerfile is labelled by its user-facing surface, not its container.
//   - backend precedes cli so a server that exposes subcommands under cmd/ is
//     still a backend; cli is reserved for tools with no server signal.
//   - library is last among the positives: pure importable code — a Go module
//     with no package main (e.g. gin-gonic/gin), an npm package with a lib entry
//     and no app — falls through to here.
//
// When no detector finds decisive, inspected evidence, Classify returns
// KindOther with VerdictInconclusive and the limitations that explain why — an
// honest abstention, not a confident "other".
func Classify(insp Inspection) Classification {
	ix := newRepoIndex(insp)
	limits := ix.limitations()

	detectors := []func(*repoIndex) (apimodel.RepoKind, []Observation, bool){
		detectMobile,
		detectFrontend,
		detectBackend,
		detectCLI,
		detectLibrary,
	}
	for _, detect := range detectors {
		if kind, obs, ok := detect(ix); ok {
			return Classification{
				Kind:         kind,
				Status:       VerdictResolved,
				Observations: obs,
				Limitations:  limits,
			}
		}
	}
	limits = append(limits, "no decisive frontend / backend / library / CLI / mobile signal was found in the inspected manifests and file tree")
	return Classification{
		Kind:        apimodel.KindOther,
		Status:      VerdictInconclusive,
		Limitations: limits,
	}
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
	if c, p, ok := pickShallowest(insp.Files, "package.json"); ok {
		ix.pkgPath = p
		if ix.pkg = parsePackageJSON(c); ix.pkg == nil {
			ix.pkgBad = true
		}
	}
	ix.goMod, ix.goModPath = pick(insp.Files, "go.mod")
	ix.cargo, ix.cargoPath = pick(insp.Files, "cargo.toml")
	ix.pyproject, ix.pyprojPath = pick(insp.Files, "pyproject.toml")
	ix.setupPy, ix.setupPyPath = pick(insp.Files, "setup.py")
	if c, p, ok := pickShallowest(insp.Files, "pubspec.yaml"); ok {
		ix.pubspec, ix.pubspecPath = c, p
	} else if c, p, ok := pickShallowest(insp.Files, "pubspec.yml"); ok {
		ix.pubspec, ix.pubspecPath = c, p
	}
	ix.dockerfile, ix.dockerPath = pick(insp.Files, "dockerfile")

	for _, f := range insp.Files {
		if strings.HasSuffix(strings.ToLower(f.Path), ".go") {
			ix.goFiles = append(ix.goFiles, f)
		}
	}
	return ix
}

// pick returns the shallowest-path content and path for a manifest base name.
func pick(files []SourceFile, base string) (content, srcPath string) {
	c, p, _ := pickShallowest(files, base)
	return c, p
}

// pickShallowest returns the content and path of the file whose base name
// matches (case-insensitively), preferring the shallowest path then the
// lexically first, so the choice never depends on input order.
func pickShallowest(files []SourceFile, base string) (content, srcPath string, ok bool) {
	base = strings.ToLower(base)
	bestDepth := -1
	for _, f := range files {
		if strings.ToLower(path.Base(f.Path)) != base {
			continue
		}
		d := strings.Count(f.Path, "/")
		if bestDepth == -1 || d < bestDepth || (d == bestDepth && f.Path < srcPath) {
			bestDepth, content, srcPath, ok = d, f.Content, f.Path, true
		}
	}
	return content, srcPath, ok
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
		if strings.ToLower(path.Base(p)) == "main.go" && !read[p] {
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

// firstPathWithBaseStem returns the original-cased path whose lowercased form
// starts with stem (e.g. "src/app."), for readable evidence.
func (ix *repoIndex) firstPathWithBaseStem(stem string) string {
	stem = strings.ToLower(stem)
	for i, lp := range ix.lowerPath {
		if strings.HasPrefix(lp, stem) {
			return ix.paths[i]
		}
	}
	return stem + "*"
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
	// Flutter projects are defined by a pubspec.yaml declaring the flutter SDK.
	if ix.pubspec != "" && strings.Contains(strings.ToLower(ix.pubspec), "flutter") {
		ev = append(ev, Observation{Path: ix.pubspecPath, Detail: "declares the Flutter SDK (mobile)"})
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
	var framework, frameworkVer string
	for _, f := range webFrameworks {
		if v := ix.pkg.dep(f); v != "" {
			framework, frameworkVer = f, v
			break
		}
	}
	if framework == "" {
		return "", nil, false
	}

	// An app entry distinguishes a runnable frontend from a published UI library.
	var entry Observation
	switch {
	case ix.hasBase("index.html"):
		entry = Observation{Detail: "index.html present (web app entry)"}
	case ix.hasPrefix("src/app."):
		entry = Observation{Detail: ix.firstPathWithBaseStem("src/app.") + " present (root component)"}
	case ix.hasPrefix("src/main."):
		entry = Observation{Detail: ix.firstPathWithBaseStem("src/main.") + " present (app bootstrap)"}
	case ix.pkg.scripts("dev") != "" || ix.pkg.scripts("start") != "":
		entry = Observation{Path: ix.pkgPath, Detail: "has a dev/start script (runnable app)"}
	default:
		// No app entry: defer to detectLibrary.
		return "", nil, false
	}

	ev := []Observation{{Path: ix.pkgPath, Detail: "depends on " + quoteDep(framework, frameworkVer) + " (web framework)"}, entry}
	if ix.pkg.Private {
		ev = append(ev, Observation{Path: ix.pkgPath, Detail: `"private": true (not a published package)`})
	}
	return apimodel.KindFrontend, ev, true
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
	// Python web frameworks.
	if py, src := ix.pyText(); py != "" {
		if dep := firstContains(py, pyServerDeps); dep != "" {
			ev = append(ev, Observation{Path: src, Detail: "depends on \"" + dep + "\" (web framework)"})
		}
	}
	// Rust web frameworks (an app, i.e. with a [[bin]] target).
	if ix.cargo != "" && strings.Contains(ix.cargo, "[[bin]]") {
		if dep := firstContains(ix.cargo, rustServerDeps); dep != "" {
			ev = append(ev, Observation{Path: ix.cargoPath, Detail: "[[bin]] target depends on \"" + dep + "\" (web framework)"})
		}
	}
	// Containerized service that exposes a port. Frontend was already ruled out
	// above, so EXPOSE here is a safe backend signal.
	if port := dockerExposedPort(ix.dockerfile); port != "" {
		ev = append(ev, Observation{Path: ix.dockerPath, Detail: "EXPOSE " + port + " (serves on a network port)"})
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
	// Python: console_scripts entry points produce commands.
	if py, src := ix.pyText(); py != "" && strings.Contains(py, "console_scripts") {
		ev = append(ev, Observation{Path: src, Detail: "defines console_scripts (installs a command)"})
	}
	// Rust: a [[bin]] target using a CLI argument parser.
	if ix.cargo != "" && strings.Contains(ix.cargo, "[[bin]]") {
		if dep := firstContains(ix.cargo, rustCLIDeps); dep != "" {
			ev = append(ev, Observation{Path: ix.cargoPath, Detail: "[[bin]] target depends on \"" + dep + "\" (CLI parser)"})
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
		hasApp := ix.hasBase("index.html") || ix.hasPrefix("src/app.")
		hasBin := len(ix.pkg.Bin) > 0 && string(ix.pkg.Bin) != "null"
		if hasLibEntry && !hasApp && !hasBin {
			ev = append(ev, Observation{Path: ix.pkgPath, Detail: `declares a lib entry ("main"/"module"/"exports") and no app or bin (importable package)`})
		}
	}
	// Rust: a [lib] crate with no [[bin]] target.
	if ix.cargo != "" && strings.Contains(ix.cargo, "[lib]") && !strings.Contains(ix.cargo, "[[bin]]") {
		ev = append(ev, Observation{Path: ix.cargoPath, Detail: "declares [lib] and no [[bin]] (importable crate)"})
	}
	// Python: packaging metadata with no console_scripts.
	if py, src := ix.pyText(); py != "" && !strings.Contains(py, "console_scripts") {
		ev = append(ev, Observation{Path: src, Detail: "packaging metadata with no console_scripts (importable package)"})
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

// goMainReadablePath returns the path of a Go source actually read that declares
// `package main`, and whether one was found.
func (ix *repoIndex) goMainReadablePath() (string, bool) {
	for _, f := range ix.goFiles {
		if declaresGoPackageMain(f.Content) {
			return f.Path, true
		}
	}
	return "", false
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
		if strings.ToLower(path.Base(p)) == "main.go" {
			return Observation{Path: p, Detail: "Go command entry point present (source not inspected)"}, true
		}
	}
	return Observation{}, false
}

// goServerObservation returns evidence that a Go command starts a server: a read
// source calling http.ListenAndServe, or a declared server-framework dependency.
func (ix *repoIndex) goServerObservation() (Observation, bool) {
	for _, f := range ix.goFiles {
		if strings.Contains(f.Content, "ListenAndServe") || strings.Contains(f.Content, "http.Server{") {
			return Observation{Path: f.Path, Detail: "calls http.ListenAndServe (starts an HTTP server)"}, true
		}
	}
	if dep := ix.goDep(goServerDeps); dep != "" {
		return Observation{Path: ix.goModPath, Detail: "requires \"" + dep + "\" (server framework)"}, true
	}
	return Observation{}, false
}

// goDep returns the first of wants that appears in a go.mod require directive
// (ignoring the module line so the repo's own path never counts as a dep).
func (ix *repoIndex) goDep(wants []string) string {
	for _, line := range strings.Split(ix.goMod, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "module ") {
			continue
		}
		for _, w := range wants {
			if strings.Contains(trimmed, w) {
				return w
			}
		}
	}
	return ""
}

// --- Python helpers ----------------------------------------------------------

// pyText returns the preferred Python packaging text and the path it came from.
func (ix *repoIndex) pyText() (text, srcPath string) {
	if ix.pyproject != "" {
		return ix.pyproject, ix.pyprojPath
	}
	return ix.setupPy, ix.setupPyPath
}

// --- small pure utilities ----------------------------------------------------

// declaresGoPackageMain reports whether Go source declares `package main`,
// scanning line by line so a `package main` inside a string or comment on the
// same line as other text does not false-positive.
func declaresGoPackageMain(src string) bool {
	for _, line := range strings.Split(src, "\n") {
		if strings.TrimSpace(line) == "package main" {
			return true
		}
	}
	return false
}

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

// firstContains returns the first of wants found (case-insensitively) in text.
func firstContains(text string, wants []string) string {
	lower := strings.ToLower(text)
	for _, w := range wants {
		if strings.Contains(lower, strings.ToLower(w)) {
			return w
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

// dep returns the version range declared for name across the dependency,
// devDependency and peerDependency maps, or "" if absent. Peer dependencies are
// checked last so a real (dev)dependency version is preferred for evidence.
func (p *packageJSON) dep(name string) string {
	for _, m := range []map[string]string{p.Dependencies, p.DevDependencies, p.PeerDependencies} {
		if v, ok := m[name]; ok {
			return v
		}
	}
	return ""
}

// scripts returns the named npm script, or "".
func (p *packageJSON) scripts(name string) string {
	return p.Scripts[name]
}
