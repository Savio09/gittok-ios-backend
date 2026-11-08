# Classification repair validation — October 31, 2026

**Recommendation: request another repair pass before accepting the plan as complete.**

Reviewed `fortune/classification-review-repairs` at
`0ca94e4d4f9349a5df2ae1bf2cf42c15f64a5709`, against original baseline
`3bf15746e053a391e95d7752c9c97684187bb774` and the
[implementation brief](../../../tasks/plan.md). The branch was already checked out.
The working tree had no tracked changes; existing untracked review documents and
the plan were preserved. No application code, tests in the active suite, commits,
branches, Linear issues or Notion pages were changed during this validation.

The existing suite, race checks, vet, build and tracked diff whitespace check
passed. The maintained eleven original counterexamples pass. These are real
improvements, particularly deterministic manifest choice, preservation of actual
paths, parsed comments versus declarations, explicit inspection state, the shared
selector, and cancellation during retry backoff.

However, isolated reviewer tests reproduced **15 unsupported resolved
classification verdicts and one swallowed cancellation**. They target boundary
conditions; their failure count is not a representative accuracy measurement.
Some behaviors predate these commits. They remain open because the repair plan
explicitly covered their root causes. The implementation report's statement that
all repairs are implemented is broader than the evidence supports.

## Findings

### R1 — P1: completeness is carried into the input but discarded during inference

Locations: `internal/crawler/classify.go:224–240,339–379,590–605,640–672,738–747`;
`internal/crawler/goanalysis.go:22–32`.

`SourceFile.Truncated` and `Inspection.Unavailable` exist, but selected manifests
become plain strings and the general unavailable list is never consulted after
index construction. Only unread paths named `main.go` get a dedicated veto.
Go parse success is discarded when deciding that a source is readable and that
no server was found. Rust and Python library fallbacks also make negative claims
without checking the relevant tree/file completeness.

Reproduced cases:

| Input | Actual verdict | Why unsupported |
|---|---|---|
| Root `package.json` fetch fails; only an Expo example is read | resolved mobile; no failure limitation | The unavailable root product was replaced by its example |
| `cmd/api/main.go` clipped after `package main` | resolved CLI; no truncation limitation | The missing remainder could contain the server call |
| Same entry has an unclosed function | resolved CLI; no parse limitation | Parsing failure was treated as absence of a server |
| Rust library source in a truncated tree | resolved library, “no binary target” | An unlisted binary remains possible; adding a caveat does not validate that negative claim |
| Python manifest clipped after `[project]` metadata | resolved library, “no command entry points” | Scripts may be in the missing suffix |
| Complete Go tree lists unread `app.go`, no `main.go` | resolved library, “no package main in the tree” | Go entry files can have any filename; listing names does not inspect package clauses |

Required repair: retain per-component parse and read states through extraction
and resolution. Each absence claim needs a defined, sufficiently inspected scope.
Keep positive observations from partial files when valid, but withhold conclusions
whose decisive negative premise is uninspected. Record the actual missing or
clipped source. Do not fix this by abstaining on every repository: preserve
supported positives and distinguish bounded uncertainty from verified absence.

### R2 — P1: one manifest per basename still replaces a component inventory

Locations: `internal/crawler/classify.go:171–240,295–332,477–513,686–709`.

The plan required retaining relevant manifests and their owning components.
`repoIndex` instead keeps one `pkg`, one `goMod`, etc. `pickPrimary` deterministically
discards the rest. `scopeRank` examines only the first directory segment, and
ordinary subdirectories are all considered primary. Frontend entries and Go
sources are then searched outside the selected manifest's component. Choosing the
minimum scope across observations cannot repair that earlier mixing.

Reproduced:

- A root React library with exports and an independent `ui/package.json` plus
  `ui/index.html` becomes frontend. Its observation combines the root dependency
  with the nested app's entry; the nested manifest is discarded.
- A root Go library with a separate `packages/demo/go.mod` and demo server becomes
  backend. The nested module's `package main` is attributed to the root product's
  role because `packages/` is not on the top-level exclusion list.

Required repair: build a lightweight inventory keyed by component root, retaining
all relevant manifests. Associate source, entry, dependency and target observations
with their owner before resolving the repository role. Keep secondary components
visible; abstain where primary-product evidence is genuinely ambiguous. Add
component-boundary tests beyond top-level directories named `examples/`.

### R3 — P1: the Go AST detector still fabricates HTTP call evidence

Locations: `internal/crawler/goanalysis.go:41–64`;
`internal/crawler/classify.go:738–742`.

Every selector call named `ListenAndServe` or `ListenAndServeTLS` is treated as
HTTP, regardless of the receiver or imports. A composite literal spelled
`http.Server{}` also counts as starting a server. Both branches generate the
same assertion: “calls http.ListenAndServe (starts an HTTP server).”

Reproduced: a CLI calling its own `logger{}.ListenAndServe()` becomes backend,
as does `func main() { _ = http.Server{} }`, which makes no listening call at all.
Both receive the fabricated HTTP call observation.

Required repair: recognize bounded supported patterns using actual imports and
aliases; resolve receiver identity where supported and otherwise record uncertainty.
Separate a type/literal declaration from a call expression. Describe static syntax
without claiming observed startup. Keep real direct/aliased HTTP call positives.

### R4 — P1: dependency categories are retained and then flattened again

Locations: `internal/crawler/classify.go:408–415,549–555,917–922`;
`internal/crawler/pyanalysis.go:73–92`.

Only the frontend detector checks the JS dependency category. Backend and mobile
use category-agnostic `dep()`. Python merges every optional dependency group into
the same list as mandatory dependencies, directly contradicting Slice 3c.

Reproduced:

- npm CLI with Express only in `devDependencies` → resolved backend.
- Exported package with Expo only in `peerDependencies` → resolved mobile.
- Python library with Flask only in `[project.optional-dependencies].docs` →
  resolved backend.

Required repair: preserve category and optional-group identity in observations
and apply the role policy consistently across detectors. Dev/peer/optional
declarations can describe tooling or integrations without establishing a primary
application. Keep actual runtime application positives and explicit secondary
capabilities separate.

### R5 — P2: Docker EXPOSE alone still establishes backend

Location: `internal/crawler/classify.go:570–579`.

`FROM alpine` plus `EXPOSE 8080`, with no application evidence, resolves backend.
The observation's revised wording correctly calls this a declared port, but the
role inference still makes that port decisive. Slice 3d explicitly excluded this.

Required repair: retain the port declaration as a deployment observation and
require independent supported product evidence before resolving backend.

### R6 — P2: legacy Python entry detection still matches comments

Locations: `internal/crawler/classify.go:612–618,816–823`.

`setup.py` containing `# no console_scripts declared here` and an ordinary
`setup(name='core')` resolves CLI with “defines console_scripts.” Dependencies use
parsers, but legacy entry-point extraction still uses `strings.Contains`.

Required repair: recognize only a deliberately bounded static declaration, without
executing Python, or report the dynamic declaration as uninspected. A comment,
string mention or unsupported dynamic expression must not establish a command.

### R7 — P2: cancellation on the last content request is swallowed

Location: `internal/githubclient/contents.go:86–93,123–124`.

`GetFileContents` converts a canceled fetch into a skipped-file record and
continues. Cancellation is returned only by the context check at the next loop
iteration. If the canceled request is the last/only path, it returns `nil` error.
The new integration test uses two paths and cancels during the first, so it misses
this boundary. This is an existing client behavior left open by the acquisition
repair, rather than a newly introduced line change.

Reproduced with a synchronized local mock server: one requested file; cancel
after the handler receives the request; result has a skipped file containing
“context canceled” and `err == nil`.

Required repair: propagate cancellation/deadline errors immediately from the fetch
and verify the final/only path, including the analyzer boundary. Preserve ordinary
404/individual failure degradation.

### R8 — P2: required acceptance evidence is unfinished

Locations: `internal/crawler/evidence.go:128–137`;
`internal/crawler/negative_controls_test.go:14–49,55–116`;
`docs/reviews/2026-10-31-classification-repair-results.md:74–78`.

The report explicitly leaves the sanitized real-repository corpus for follow-up,
although Slice 9 requires both the feed and study sets. Six synthetic archetypes
do not establish performance on those repositories. The negative controls cover
a Go comment and an omitted source. There is no mismatched-quote or invalid/span
control, and observations carry no structured field/span/quote that a validator
checks against inspected content. The invented HTTP observations in R3 show why
path preservation is insufficient.

Required repair: complete the declared corpus with provenance and documented
reference judgments; preserve the original incomplete snapshots as degradation
cases. Add source-linked field/span evidence and the missing validation controls.
Report resolved correctness and abstentions separately, per fixture, and verify
the expected status of each case. Do not present the current synthetic results as
real-repository validation. Update the remediation report after those checks.

## Checks and archived pack replay

Fresh tracked-suite checks at the reviewed commit:

| Check | Result |
|---|---|
| `go test -count=1 ./...` | Pass |
| `go test -race -count=1 ./...` | Pass |
| `go vet ./...` | Pass |
| `go build ./...` | Pass; rerun with module-cache access after a sandbox cache warning |
| `git diff --check` | Pass |
| Reviewer classifier boundary controls | 15 failures, each an unsupported resolved verdict |
| Reviewer final-file cancellation control | Fails: nil error |

The four October 30 extracted pack inputs were also replayed with an explicit
depth-limited tree, since their stored trees are abbreviated. This is an
observational replay, not a positive-classification acceptance corpus:

| Archived input | Result |
|---|---|
| Face Recognition | other / inconclusive |
| Plantly | mobile / resolved |
| Excalidraw | other / inconclusive |
| PocketBase | other / inconclusive |

PocketBase's abstention is preferable to forcing backend from its missing decisive
source. Excalidraw's abstention needs complete-enough component-aware corpus
coverage before any conclusion about real-repository correctness. No live crawl,
model call or target-repository execution was performed. The live feed remains on
its documented older metadata heuristic; that deferred wiring is not a finding.

## Reproduce without changing the application working tree

Artifacts:

- [Classifier controls and pack replay](reviewer_crawler_test.go.txt)
- [Cancellation control](reviewer_githubclient_test.go.txt)
- [Captured output](results.txt)
- [Original archived inputs](../2026-10-30-classification/review_fixtures.json)

Reviewer tests were run against an isolated `git archive` of the reviewed commit
at `/tmp/gittok-repair-validation-iw7vbea4`. To recreate elsewhere:

1. Extract that exact commit to a temporary directory.
2. Copy the two `.go.txt` artifacts to `internal/crawler/reviewer_validation_test.go`
   and `internal/githubclient/reviewer_validation_test.go`, respectively.
3. Copy the original archived JSON to
   `internal/crawler/reviewer_archived_fixtures.json`.
4. Run `go test -count=1 -run TestReviewer -v ./internal/crawler ./internal/githubclient`.
   The GitHub-client control needs a local loopback HTTP listener. No GitHub token
   or external network access is needed once Go dependencies are available.

The classifier controls assert that a particular unsupported kind must not be
resolved. They allow a supported alternative or an honest inconclusive result,
so they do not impose a universal fallback label. The pack replay prints results
without asserting labels from abbreviated historical snapshots.

## Process documentation entry

After the original review and research assessment, the implementation agent
delivered 14 commits including its remediation record. Independent validation
confirmed the original minimal regressions and standard checks passed, then
challenged whether the repairs generalized to unread/truncated evidence,
component ownership, dependency categories and semantic source support. Fifteen
classification counterexamples and one cancellation boundary failed. This exposed
a recurring process issue: adding a parser or an uncertainty field does not prove
downstream inference respects its meaning. Follow-up acceptance should test those
boundaries directly and complete the repository corpus before recording the plan
as finished. Preserve the implementation history and these failures as evidence
of iterative evaluation rather than replacing the original results.
