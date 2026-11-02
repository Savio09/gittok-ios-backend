package crawler

import (
	"path"
	"sort"
	"strings"

	"github.com/Savio09/gittok-ios-backend/internal/githubclient"
)

// SelectedFile is one file the crawler judges worth reading, together with the
// reason it was picked. Like Classification, it mirrors the project's evidence
// philosophy: a file is never chosen silently — the Reason records *why* it
// carries signal, so a reader (and the downstream classifier) can see the
// ranking's reasoning rather than trust a bare list.
type SelectedFile struct {
	Path   string // repo-root path, e.g. "cmd/app/main.go"
	Score  int    // heuristic weight; higher means more informative
	Reason string // why this file was selected
}

// SelectionBudget bounds how many files SelectFiles returns, so a huge or
// monorepo-shaped tree cannot blow the downstream content-fetch budget. A
// non-positive value means that dimension is unbounded.
//
//   - MaxFiles   the overall cap on selected files.
//   - MaxSource  a sub-cap on generic source files (matched only by extension),
//     so a repo's raw code can't crowd out manifests, docs and config.
//   - MaxPerBase a cap on files sharing a base name (e.g. the many package.json
//     files in a JS monorepo), to dedupe that kind of spam.
//   - MaxEntry   how many entry-point sources are reserved up front — see the
//     entry-source guarantee in SelectFiles.
type SelectionBudget struct {
	MaxFiles   int
	MaxSource  int
	MaxPerBase int
	MaxEntry   int
}

// DefaultSelectionBudget is the bound used for ingesting a repo for analysis.
// The values match the caps the evidencepack command was tuned with.
func DefaultSelectionBudget() SelectionBudget {
	return SelectionBudget{MaxFiles: 16, MaxSource: 6, MaxPerBase: 3, MaxEntry: 4}
}

// SelectFiles ranks a repository's file tree by how much project signal each
// file usually carries and returns the top files, capped by budget. It is a
// pure, network-free function so it can be unit-tested with synthetic trees:
// the input is the tree entries (paths plus the blob/tree type GetTree already
// provides), and the output is a ranked, capped []SelectedFile. It only decides
// what is worth reading; it never fetches content or makes a claim about truth.
//
// This is the canonical implementation of the heuristic the evidencepack
// command (cmd/evidencepack) first prototyped; that command keeps its own copy
// but could later call into here.
//
// # Ranking (#algorithms)
//
// Every blob is scored by a fixed tier order — dependency manifests highest,
// then contribution docs and READMEs, run/container config, root build config,
// CI workflows, likely entry points, and finally generic source as a fallback
// so a README-poor repo (where the code is the only evidence) still yields a
// useful set. Ties break toward shallower paths, then lexicographically, so the
// result is deterministic.
//
// # Entry-source guarantee (the "caddy gap")
//
// The classifier (see Classify) can only tell a server from a CLI by reading an
// entry source: a Go command that calls http.ListenAndServe is a backend, while
// the same package main under cmd/ with no server call is a CLI. Entry points
// score low (tier 50), so in a large or monorepo-shaped tree the many
// higher-scored manifests and docs would otherwise push the one cmd/x/main.go
// out of MaxFiles — and the classifier, never seeing ListenAndServe, would
// misclassify a caddy-style server as a CLI. To close that gap, SelectFiles
// reserves the top MaxEntry entry-point sources (main.go, cmd/*/main.go, and
// the analogous main/app/server/index files for Python, Rust, Node and Dart)
// *before* filling the remaining budget, so a repo's entry source is always
// present for the classifier to read. The reservation still counts against
// MaxFiles, so the result never exceeds the budget.
func SelectFiles(entries []githubclient.TreeEntry, budget SelectionBudget) []SelectedFile {
	type candidate struct {
		SelectedFile
		depth    int
		isSource bool
		isEntry  bool
	}

	var cands []candidate
	for _, e := range entries {
		if e.Type != "blob" {
			continue
		}
		fs := scoreTreeFile(e.Path)
		if fs.score == 0 {
			continue
		}
		cands = append(cands, candidate{
			SelectedFile: SelectedFile{Path: e.Path, Score: fs.score, Reason: fs.reason},
			depth:        strings.Count(e.Path, "/"),
			isSource:     fs.isSource,
			isEntry:      fs.isEntry,
		})
	}

	// Rank: score desc, then shallower paths first, then lexicographic — a total
	// order, so the selection is stable across runs.
	sort.SliceStable(cands, func(i, j int) bool {
		if cands[i].Score != cands[j].Score {
			return cands[i].Score > cands[j].Score
		}
		if cands[i].depth != cands[j].depth {
			return cands[i].depth < cands[j].depth
		}
		return cands[i].Path < cands[j].Path
	})

	selected := make([]bool, len(cands))
	var count, entryCount, sourceCount int
	perBase := map[string]int{}

	// take marks candidate i as selected if it fits every cap, updating the
	// counters. It is the single place the budget is enforced, so no selection
	// path can exceed a cap.
	take := func(i int) {
		if budget.MaxFiles > 0 && count >= budget.MaxFiles {
			return
		}
		base := strings.ToLower(path.Base(cands[i].Path))
		if budget.MaxPerBase > 0 && perBase[base] >= budget.MaxPerBase {
			return
		}
		if cands[i].isSource && budget.MaxSource > 0 && sourceCount >= budget.MaxSource {
			return
		}
		selected[i] = true
		count++
		perBase[base]++
		if cands[i].isSource {
			sourceCount++
		}
		if cands[i].isEntry {
			entryCount++
		}
	}

	// Pass 1 — reserve entry-point sources first (the caddy-gap guarantee),
	// bounded by MaxEntry so they can't starve manifests and docs.
	for i := range cands {
		if !cands[i].isEntry {
			continue
		}
		if budget.MaxEntry > 0 && entryCount >= budget.MaxEntry {
			break
		}
		take(i)
	}
	// Pass 2 — fill the remaining budget with the best of the rest.
	for i := range cands {
		if selected[i] {
			continue
		}
		take(i)
	}

	// Emit in ranked order (a reserved entry appears in its ranked position, not
	// hoisted to the top), so the returned slice reads as a ranking.
	out := make([]SelectedFile, 0, count)
	for i := range cands {
		if selected[i] {
			out = append(out, cands[i].SelectedFile)
		}
	}
	return out
}

// fileScore is the heuristic verdict for a single path: its weight, the reason
// it carries signal, and the two flags the budget caps act on.
type fileScore struct {
	score    int
	reason   string
	isSource bool // matched only by generic extension, not by a named rule
	isEntry  bool // a likely program entry point the classifier needs to read
}

// scoreTreeFile ranks one path by how much project signal it usually carries.
// The tiers mirror the evidencepack prototype; a score of 0 means "not worth
// reading on its own". Depth is subtracted within a tier so shallower (usually
// more canonical) files sort first.
func scoreTreeFile(p string) fileScore {
	base := strings.ToLower(path.Base(p))
	lower := strings.ToLower(p)
	depth := strings.Count(p, "/")

	// Dependency manifests & project definition — the strongest signal.
	switch base {
	case "package.json", "go.mod", "cargo.toml", "pyproject.toml",
		"requirements.txt", "gemfile", "pom.xml", "build.gradle",
		"composer.json", "pubspec.yaml", "setup.py":
		return fileScore{score: 100 - depth, reason: "dependency/project manifest (strongest signal)"}
	}
	// Contribution + licensing docs.
	if strings.HasPrefix(base, "contributing") {
		return fileScore{score: 90 - depth, reason: "contribution guide"}
	}
	if strings.HasPrefix(base, "readme") {
		return fileScore{score: 85 - depth, reason: "README"}
	}
	// Containerization / run config.
	switch base {
	case "dockerfile", "docker-compose.yml", "docker-compose.yaml",
		"makefile", ".env.example", "procfile":
		return fileScore{score: 80 - depth, reason: "containerization / run config"}
	}
	// Build/tooling config (root-level only, to avoid nested noise).
	if depth == 0 {
		switch base {
		case "tsconfig.json", "vite.config.ts", "vite.config.js",
			"next.config.js", "next.config.mjs", "webpack.config.js",
			"rollup.config.js", "babel.config.js", "nest-cli.json",
			"tailwind.config.js", "app.json":
			return fileScore{score: 70, reason: "root build config"}
		}
	}
	// CI workflows (take the shallow ones).
	if strings.HasPrefix(lower, ".github/workflows/") &&
		(strings.HasSuffix(lower, ".yml") || strings.HasSuffix(lower, ".yaml")) {
		return fileScore{score: 60 - depth, reason: "CI workflow"}
	}
	// Likely entry points — the source the classifier reads to tell a server
	// from a CLI. Flagged isEntry so the caddy-gap guarantee can reserve them.
	switch base {
	case "main.go", "main.py", "main.rs", "index.js", "index.ts",
		"app.js", "app.ts", "app.py", "server.go", "server.js", "main.dart":
		return fileScore{score: 50 - depth, reason: "likely entry point (source the classifier reads)", isEntry: true}
	}
	// Generic source as a fallback, below named entry points (50) but above
	// nothing, so a manifest-poor repo still offers its code as evidence.
	if isSourceFile(p) {
		return fileScore{score: 25, reason: "source file (fallback evidence)", isSource: true}
	}
	return fileScore{}
}

// isSourceFile reports whether a path is a source file in a common language,
// used as the fallback signal when a repo lacks manifests and docs.
func isSourceFile(p string) bool {
	switch strings.ToLower(path.Ext(p)) {
	case ".go", ".py", ".ts", ".tsx", ".js", ".jsx", ".rs",
		".dart", ".java", ".rb", ".kt", ".swift", ".c", ".cpp":
		return true
	}
	return false
}
