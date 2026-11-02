package crawler

import (
	"encoding/json"
	"path"
	"strings"

	"github.com/Savio09/gittok-ios-backend/internal/apimodel"
)

// Classification is the crawler's verdict on what kind of project a repository
// is, together with the evidence it rested on. It mirrors the project's claim
// philosophy (see apimodel.Claim): the kind is never asserted on its own — the
// Evidence names the manifest entries and file-tree facts that produced it, so
// a reader can see the reasoning rather than trust a bare label.
type Classification struct {
	Kind     apimodel.RepoKind
	Evidence []string
}

// Classify infers a repository's RepoKind from already-gathered evidence. It is
// a pure, network-free function so it can be unit-tested with synthetic inputs:
//
//   - language   the repo's primary language (GitHub's guess; may be empty),
//   - paths      the file tree as a flat list of blob/dir paths,
//   - manifests  a path->content map for the few files that carry the signal:
//     package.json, go.mod, Cargo.toml, pyproject.toml, setup.py,
//     pubspec.yaml, a Dockerfile, and — when available — a key entry
//     source file (e.g. main.go, cmd/x/main.go) whose package/imports
//     disambiguate a Go library from a Go binary or server.
//
// It never calls the GitHub API; the caller gathers the inputs first.
//
// # Precedence (#algorithms)
//
// A repo often carries several signals at once (a Next.js app also runs a Node
// server; a Go server also lives under cmd/). Classify picks the dominant kind
// by running detectors in a fixed order and returning the first match:
//
//	mobile → frontend → backend → cli → library → other
//
// The ordering encodes which signal is the most specific / most salient for a
// preview:
//
//   - mobile is first because react-native/expo apps also depend on react, so
//     they would otherwise be mistaken for a web frontend.
//   - frontend precedes backend so a client app that happens to ship a
//     Dockerfile is labelled by its user-facing surface, not its container.
//   - backend precedes cli so a server that exposes subcommands under cmd/ is
//     still a backend; cli is reserved for tools with no server signal.
//   - library is last among the positives: a repo that merely *could* be
//     imported but also produces a runnable artifact is classified by the
//     artifact (the more useful thing to preview). Pure importable code — a Go
//     module with no package main (e.g. gin-gonic/gin), an npm package with a
//     lib entry and no app — falls through to here.
func Classify(language string, paths []string, manifests map[string]string) Classification {
	ix := newRepoIndex(language, paths, manifests)

	detectors := []func(*repoIndex) (apimodel.RepoKind, []string, bool){
		detectMobile,
		detectFrontend,
		detectBackend,
		detectCLI,
		detectLibrary,
	}
	for _, detect := range detectors {
		if kind, ev, ok := detect(ix); ok {
			return Classification{Kind: kind, Evidence: ev}
		}
	}
	return Classification{
		Kind:     apimodel.KindOther,
		Evidence: []string{"no decisive frontend / backend / library / CLI / mobile signal in the manifests or file tree"},
	}
}

// repoIndex is the pre-parsed, cheap-to-query view of the inputs. Parsing the
// manifests once up front keeps the detectors readable and avoids re-walking
// the (potentially large) path list for every predicate.
type repoIndex struct {
	language  string            // lowercased primary language
	paths     []string          // original tree paths
	lowerPath []string          // lowercased tree paths, index-aligned with paths
	manifests map[string]string // original path -> content

	pkg        *packageJSON // parsed package.json, or nil
	goMod      string       // go.mod content, or ""
	cargo      string       // Cargo.toml content, or ""
	pyproject  string       // pyproject.toml content, or ""
	setupPy    string       // setup.py content, or ""
	pubspec    string       // pubspec.yaml content, or ""
	dockerfile string       // Dockerfile content, or ""
	goSources  []string     // contents of any *.go files handed in via manifests
}

func newRepoIndex(language string, paths []string, manifests map[string]string) *repoIndex {
	ix := &repoIndex{
		language:  strings.ToLower(strings.TrimSpace(language)),
		paths:     paths,
		lowerPath: make([]string, len(paths)),
		manifests: manifests,
	}
	for i, p := range paths {
		ix.lowerPath[i] = strings.ToLower(p)
	}
	for p, content := range manifests {
		switch strings.ToLower(path.Base(p)) {
		case "package.json":
			if ix.pkg == nil { // first one wins; root is preferred by the caller's ordering
				ix.pkg = parsePackageJSON(content)
			}
		case "go.mod":
			ix.goMod = content
		case "cargo.toml":
			ix.cargo = content
		case "pyproject.toml":
			ix.pyproject = content
		case "setup.py":
			ix.setupPy = content
		case "pubspec.yaml", "pubspec.yml":
			ix.pubspec = content
		case "dockerfile":
			ix.dockerfile = content
		}
		if strings.HasSuffix(strings.ToLower(p), ".go") {
			ix.goSources = append(ix.goSources, content)
		}
	}
	return ix
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

// firstPathWithBase returns the original-cased path whose base matches, for
// readable evidence; "" if none.
func (ix *repoIndex) firstPathWithBase(name string) string {
	name = strings.ToLower(name)
	for i, lp := range ix.lowerPath {
		if path.Base(lp) == name {
			return ix.paths[i]
		}
	}
	return ""
}

// --- mobile -----------------------------------------------------------------

func detectMobile(ix *repoIndex) (apimodel.RepoKind, []string, bool) {
	var ev []string
	if ix.pkg != nil {
		for _, dep := range []string{"react-native", "expo"} {
			if v := ix.pkg.dep(dep); v != "" {
				ev = append(ev, "package.json: depends on "+quoteDep(dep, v)+" (mobile framework)")
			}
		}
	}
	// Flutter projects are defined by a pubspec.yaml declaring the flutter SDK.
	if ix.pubspec != "" && strings.Contains(strings.ToLower(ix.pubspec), "flutter") {
		ev = append(ev, "pubspec.yaml: declares the Flutter SDK (mobile)")
	}
	// Native shells side by side are a strong mobile signal on their own.
	if ix.hasPrefix("android/") && (ix.hasPrefix("ios/") || ix.hasPrefix("ios ")) {
		ev = append(ev, "android/ and ios/ app directories both present (native mobile shells)")
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

func detectFrontend(ix *repoIndex) (apimodel.RepoKind, []string, bool) {
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

	// An app entry distinguishes a runnable frontend from a published UI
	// library (which ships only importable modules). index.html, a src/App.*
	// root component, or a src/main.* bootstrap are the usual markers.
	var entry string
	switch {
	case ix.hasBase("index.html"):
		entry = "index.html present (web app entry)"
	case ix.hasPrefix("src/app."):
		entry = ix.firstPathWithBaseStem("src/app.") + " present (root component)"
	case ix.hasPrefix("src/main."):
		entry = ix.firstPathWithBaseStem("src/main.") + " present (app bootstrap)"
	case ix.pkg.scripts("dev") != "" || ix.pkg.scripts("start") != "":
		entry = "package.json: has a dev/start script (runnable app)"
	}
	// A package that publishes a lib entry and has no app entry is a library,
	// not a frontend — defer to detectLibrary.
	if entry == "" {
		return "", nil, false
	}

	ev := []string{"package.json: depends on " + quoteDep(framework, frameworkVer) + " (web framework)", entry}
	if ix.pkg.Private {
		ev = append(ev, `package.json: "private": true (not a published package)`)
	}
	return apimodel.KindFrontend, ev, true
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

// --- backend -----------------------------------------------------------------

// Server frameworks keyed by language ecosystem. These are the dependencies a
// repo pulls in to *run* a server (not the frameworks themselves).
var (
	goServerDeps   = []string{"gin-gonic/gin", "labstack/echo", "gofiber/fiber", "go-chi/chi", "gorilla/mux"}
	nodeServerDeps = []string{"express", "fastify", "koa", "@nestjs/core", "@hapi/hapi", "hapi"}
	pyServerDeps   = []string{"fastapi", "flask", "django", "starlette", "uvicorn", "sanic", "aiohttp", "tornado"}
	rustServerDeps = []string{"axum", "actix-web", "rocket", "warp"}
)

func detectBackend(ix *repoIndex) (apimodel.RepoKind, []string, bool) {
	var ev []string

	// Go: only an actual command (package main) can be a backend *app*; an
	// importable module that merely uses net/http (e.g. gin itself) is a
	// library and must not match here.
	if ix.isGo() {
		if ok, where := ix.goMainPackage(); ok {
			if se := ix.goServerSignal(); se != "" {
				ev = append(ev, where, se)
			}
		}
	}
	// Node server frameworks.
	if ix.pkg != nil {
		for _, dep := range nodeServerDeps {
			if v := ix.pkg.dep(dep); v != "" {
				ev = append(ev, "package.json: depends on "+quoteDep(dep, v)+" (server framework)")
				break
			}
		}
	}
	// Python web frameworks.
	if py := ix.pyText(); py != "" {
		if dep := firstContains(py, pyServerDeps); dep != "" {
			ev = append(ev, ix.pySourceName()+": depends on \""+dep+"\" (web framework)")
		}
	}
	// Rust web frameworks (an app, i.e. with a [[bin]] target).
	if ix.cargo != "" && strings.Contains(ix.cargo, "[[bin]]") {
		if dep := firstContains(ix.cargo, rustServerDeps); dep != "" {
			ev = append(ev, "Cargo.toml: [[bin]] target depends on \""+dep+"\" (web framework)")
		}
	}
	// Containerized service that exposes a port. Frontend was already ruled
	// out above, so EXPOSE here is a safe backend signal.
	if port := dockerExposedPort(ix.dockerfile); port != "" {
		ev = append(ev, "Dockerfile: EXPOSE "+port+" (serves on a network port)")
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

func detectCLI(ix *repoIndex) (apimodel.RepoKind, []string, bool) {
	var ev []string

	// Go: a binary (package main) wired to a command framework.
	if ix.isGo() {
		if ok, where := ix.goMainPackage(); ok {
			if dep := ix.goDep(goCLIDeps); dep != "" {
				ev = append(ev, where, "go.mod: requires \""+dep+"\" (CLI framework)")
			} else if ix.hasPrefix("cmd/") {
				// A main package under cmd/ with no server signal is a command.
				ev = append(ev, where, "entry point under cmd/ with no server signal (produces a command)")
			}
		}
	}
	// npm: a package that installs a command exposes a "bin" field.
	if ix.pkg != nil && len(ix.pkg.Bin) > 0 && string(ix.pkg.Bin) != "null" {
		ev = append(ev, `package.json: declares a "bin" field (installs a command)`)
	}
	// Python: console_scripts entry points produce commands.
	if py := ix.pyText(); py != "" && strings.Contains(py, "console_scripts") {
		ev = append(ev, ix.pySourceName()+": defines console_scripts (installs a command)")
	}
	// Rust: a [[bin]] target using a CLI argument parser.
	if ix.cargo != "" && strings.Contains(ix.cargo, "[[bin]]") {
		if dep := firstContains(ix.cargo, rustCLIDeps); dep != "" {
			ev = append(ev, "Cargo.toml: [[bin]] target depends on \""+dep+"\" (CLI parser)")
		}
	}

	if len(ev) > 0 {
		return apimodel.KindCLI, ev, true
	}
	return "", nil, false
}

// --- library -----------------------------------------------------------------

func detectLibrary(ix *repoIndex) (apimodel.RepoKind, []string, bool) {
	var ev []string

	// Go: a module with no package main is importable-only (e.g. gin).
	if ix.isGo() && ix.goMod != "" {
		if ok, _ := ix.goMainPackage(); !ok {
			ev = append(ev, "go.mod present with no package main in the tree (importable Go module)")
		}
	}
	// npm: a lib entry (main/module/exports) and no app entry or bin.
	if ix.pkg != nil {
		hasLibEntry := ix.pkg.Main != "" || ix.pkg.Module != "" ||
			(len(ix.pkg.Exports) > 0 && string(ix.pkg.Exports) != "null")
		hasApp := ix.hasBase("index.html") || ix.hasPrefix("src/app.")
		hasBin := len(ix.pkg.Bin) > 0 && string(ix.pkg.Bin) != "null"
		if hasLibEntry && !hasApp && !hasBin {
			ev = append(ev, `package.json: declares a lib entry ("main"/"module"/"exports") and no app or bin (importable package)`)
		}
	}
	// Rust: a [lib] crate with no [[bin]] target.
	if ix.cargo != "" && strings.Contains(ix.cargo, "[lib]") && !strings.Contains(ix.cargo, "[[bin]]") {
		ev = append(ev, "Cargo.toml: declares [lib] and no [[bin]] (importable crate)")
	}
	// Python: packaging metadata with no console_scripts.
	if py := ix.pyText(); py != "" && !strings.Contains(py, "console_scripts") {
		ev = append(ev, ix.pySourceName()+": packaging metadata with no console_scripts (importable package)")
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

// goMainPackage reports whether the repo contains a Go command (package main),
// with a short description of where the signal came from. It prefers a content
// signal (a *.go source handed in declaring `package main`) and falls back to
// the file tree (a main.go anywhere), which is the usual entry-point name.
func (ix *repoIndex) goMainPackage() (bool, string) {
	for src := range ix.manifests {
		if !strings.HasSuffix(strings.ToLower(src), ".go") {
			continue
		}
		if declaresGoPackageMain(ix.manifests[src]) {
			return true, src + ": package main (produces a binary)"
		}
	}
	if p := ix.firstPathWithBase("main.go"); p != "" {
		return true, p + " present (Go command entry point)"
	}
	return false, ""
}

// goServerSignal returns evidence that a Go command starts a server, or "".
func (ix *repoIndex) goServerSignal() string {
	for _, src := range ix.goSources {
		if strings.Contains(src, "ListenAndServe") || strings.Contains(src, "http.Server{") {
			return "Go source calls http.ListenAndServe (starts an HTTP server)"
		}
	}
	if dep := ix.goDep(goServerDeps); dep != "" {
		return "go.mod: requires \"" + dep + "\" (server framework)"
	}
	return ""
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

func (ix *repoIndex) pyText() string {
	if ix.pyproject != "" {
		return ix.pyproject
	}
	return ix.setupPy
}

func (ix *repoIndex) pySourceName() string {
	if ix.pyproject != "" {
		return "pyproject.toml"
	}
	return "setup.py"
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
// so callers simply see "no package.json signal".
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
