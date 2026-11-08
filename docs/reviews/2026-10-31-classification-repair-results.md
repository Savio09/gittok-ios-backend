# Classification repair — remediation results, October 31, 2026

Status: the repairs in [the implementation brief](../../tasks/plan.md) are
implemented on branch `fortune/classification-review-repairs` (13 commits on top
of baseline `3bf1574`). This record adds remediation results to the historical
[October 30 review](2026-10-30-classification/README.md); it does not rewrite the
captured failures. It claims only what the local test suite verifies — no live
GitHub crawl, no runtime execution of any target repository, and no change to the
public API or the live `GET /v1/feed` path (which remains metadata-only via
`preview.FromMetadata` until the preview pipeline is built).

## 1. Finding → fix → regression test

| Review finding | Fix (commit) | Code | Maintained regression |
|---|---|---|---|
| P1 nondeterministic manifest selection | Slice 2 `195c67d` | `classify.go` `pickPrimary`/`scopeRank` | `inventory_test.go` (1000-run + shuffled `DeepEqual`) |
| P1 nested component overrides repo purpose | Slice 4 `d9bda62` | `classify.go` `Classify` candidate resolution, `candidateScope` | `resolve_test.go`; `review_regression_test.go` (`go_library_example…`) |
| P1 textual match → fact (Go) | Slice 3a `2c5191e` | `goanalysis.go` (`go/parser`, `x/mod/modfile`) | `goanalysis_test.go`; counterexamples `go_comment…`, `go_dependency_comment…` |
| P1 textual match → fact (Python name) | Slice 3c `7259f88` | `pyanalysis.go` (TOML, requirements) | `pyanalysis_test.go`; `python_project_name_is_not_dependency` |
| P1 textual match → fact (Dart) | Slice 3d `e1e63b0` | `dartanalysis.go` (YAML) | `dartanalysis_test.go`; `flutter_comment_is_not_sdk` |
| P1 missing inspection → negative evidence | Slice 1 `6af1cd6` | `evidence.go` `Inspection`/`TreeScope`; `classify.go` limitations + abstention | `evidence_test.go` |
| P2 ecosystem conventions missed (Rust) | Slice 6 `259a847` | `rustanalysis.go`; `rustHasBin/LibTarget` | `rustrole_test.go`; `rust_implicit_cli/library` |
| P2 ecosystem conventions missed (Python) | Slice 7 `7d6e520` | `classify.go` `[project.scripts]`, `pyPackageSource` | `pyrole_test.go`; `python_modern_cli`, `python_tooling_is_not_library`, `python_requirements_backend` |
| P2 tooling mistaken for frontend (JS) | Slice 5 `ec90532` | `classify.go` runtime-dep + primary-scope entry | `jsrole_test.go`; `npm_cli_with_vite_build_tool` |
| Provenance loss | Slices 1–2 | `Observation.Path`; `pickPrimary` keeps original path | `TestObservationCitesActualNestedPath`, `TestNestedManifestDoesNotOverwriteRootProvenance` |
| Divergent selector / acquisition evidence | Slice 8 `5e356e2` | `cmd/evidencepack` → `crawler.SelectFiles` + `Classify`; `client.go` `waitBackoff` | `cmd/evidencepack/main_test.go`, `backoff_cancel_test.go` |
| Integration handoff unproven | Slice 9 `b1b9a9c` | `cmd/evidencepack` `analyze()` | `integration_test.go`, `negative_controls_test.go` |

All eleven archived counterexamples pass as maintained tests
(`review_regression_test.go`). Two resolve to `KindOther` with an explanation
(tooling-only pyproject; comment-only Flutter in a plain Dart library) because an
honest abstention is more defensible than the archived positive label there.

## 2. Command results (local, this branch)

```
go test ./...        # ok (all packages)
go test -race ./...  # ok (all packages)
go vet ./...         # clean
go build ./...       # clean
git diff --check     # clean
```

Acceptance corpus (`TestAcceptanceCorpusSeparatesCorrectnessFromAbstention`):
`resolved=4 correct=4 inconclusive=2` of 6 fixtures. Correctness-among-resolved
and the abstention count are reported as separate measures, never merged, and
neither is substituted by a citation or coverage count.

## 3. Before / after (behaviour)

- Before: a comment or a project name could become a dependency or a server; a
  nested example or admin UI could set the repo kind; an unread file or truncated
  tree read as a negative fact; manifest selection varied run to run.
- After: dependencies and constructs come from parsers (`go/ast`, `modfile`,
  TOML, YAML); the primary kind is the most-primary component's, with others
  recorded as secondary; a bounded or failed fetch produces a limitation and, when
  decisive, an abstention; selection is deterministic and component-scoped.

## 4. Dependencies and interface changes

- New module requirements (resolved from the local cache, Go 1.25-compatible):
  `golang.org/x/mod v0.37.0`, `github.com/pelletier/go-toml/v2 v2.2.4`,
  `gopkg.in/yaml.v3 v3.0.1`. Each is a maintained, standard parser used in place
  of substring matching, per the brief.
- Internal interface change: `crawler.Classify` now takes a `crawler.Inspection`
  (tree scope + per-file read status + revision) and returns a structured
  `Classification` (`Kind`, `Status`, `Observations`, `Limitations`) with an
  `Evidence()` renderer. Only test and `cmd/evidencepack` callers existed; all
  updated together. `githubclient.FileTree` gains `DepthLimited`.
- Public HTTP/JSON contract (`internal/apimodel`) unchanged.

## 5. Remaining limitations / explicitly not done

- No live crawl was run; results are from deterministic fixtures. A sanitized
  real-repo corpus (Excalidraw, PocketBase, GitHub CLI, Starship; Plantly, Face
  Recognition) extracted from the checked-in packs remains follow-up; the current
  corpus uses representative synthetic fixtures for each archetype plus incomplete
  cases.
- The live feed still classifies via the older metadata heuristic; wiring this
  pipeline into the feed is the preview-pipeline work (G17–G19), not this repair.
- Deferred per the brief: a cross-language reference resolver, adaptive
  second-round crawl, raw-CDN/hash recovery, ETag caching, per-claim retrieval,
  SSE/cost endpoints, a storage/quota system, and commit-SHA resolution (a branch
  ref is recorded as unresolved, not pinned).

This does not declare G15/G16 complete or accepted; it reports the implemented
repairs for owner review.
