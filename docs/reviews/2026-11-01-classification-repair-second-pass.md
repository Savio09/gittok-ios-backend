# Classification repair, second pass — remediation record, November 1, 2026

Status: the tasks in the [second-pass brief](../../tasks/plan.md) are implemented
on `fortune/classification-review-repairs`, 19 commits on top of the reviewed
commit `0ca94e4` (head `dcafee3`). This record adds to — it does not replace —
the [first remediation report](2026-10-31-classification-repair-results.md) and
the [independent validation](2026-10-31-classification-repair-validation/README.md)
whose findings it answers. It is submitted for review; it does not declare the
plan, G15 or G16 accepted.

## 1. Findings → fixes → maintained regressions

Each row names the regression that rejects the unsupported result **and** the
counterpart that keeps the supported positive.

| Finding | Fixing commits | Code | Rejects the unsupported verdict | Keeps the supported positive |
|---|---|---|---|---|
| **R1** read/parse state discarded during inference | `f8568e6`, `a44c025`, `8ba0abb` | `inspect.go` (per-path read state, `TreeScope.Covers`); premise checks in `classify.go` (`goNoServerPremise`, `goNoCommandPremise`, `rustBinTarget`, npm/Python premises); blocks in `resolve.go` | `TestAbsencePremisesNeedInspectedScope` (all six reviewer R1 cases, each naming the missing path or region) | the complete counterpart in each of those six cases; `TestPartialFilesKeepTheirPositiveFacts` |
| **R2** one manifest per basename instead of a component inventory | `3b2f230`, `8ba0abb`, `895d390` | `inventory.go` (components keyed by owning root, every manifest kept with read state and categorised declarations); `resolve.go` (per-component roles, repository policy) | `TestNestedComponentsDoNotReplaceTheRootProduct` (both reviewer R2 fixtures); `TestNestedCandidatesWithoutARootProduct`; `TestNestedComparisonNeedsACompleteListing` | `TestBackendWithAdminUIResolvesBackend`, `TestRootLibraryWithExampleServerStaysLibrary`, `TestWorkspaceAppIsTheProductOverItsLibraries`, `TestInventory*`, `TestClassificationIsOrderStableAcrossComponents` |
| **R3** fabricated HTTP call evidence | `2fe45a6`, `05f3fa0` | `goanalysis.go` `scanServerCalls` (go/types against a minimal net/http stub) | `TestServerCallIdentity` (method on a local type, unused `http.Server{}` literal, shadowing, receivers and packages of unestablished identity), R3 cases in `TestGoCommentsDoNotCreateBackend` | direct, aliased, dot-imported and receiver calls in `TestServerCallIdentity`; `TestServerCallKeepsItsSpan`; `TestEveryReadCommandIsCited` |
| **R4** dependency categories flattened | `ba6e260`, `4553951` | `runtimeDep`, `pyServerObservation`, `parseRequirements`, `blockUninspectedPythonDependencies` | `TestJSCategoryGovernsMobileAndBackend` (dev-only express, peer-only expo); `TestPythonDependencyCategories` (docs extra, dynamic deps, unfollowed include, setup.py) | runtime express/expo cases in the same test; mandatory and requirements Flask cases; `TestJSToolingVsProduct` |
| **R5** EXPOSE decided the backend role | `a1ca055` | `detectBackend` (EXPOSE is supporting context only) | `TestDockerPortIsDeploymentEvidenceOnly` (port alone abstains; no wording claims listening or running) | same test with an independent server; the Dockerfile case in `TestClassify` |
| **R6** legacy `console_scripts` matched by substring | `efe8ba6`, `dcafee3` | `detectCLI`, `blockUninspectedPythonEntries` | `TestPythonEntryPointSources` (comment, string, legacy `entry_points`, GUI-only, dynamic scripts) | `[project.scripts]` case in the same test; `TestPythonEntryPointsAndTooling` |
| **R7** final-file cancellation swallowed | `559869f` | `githubclient/contents.go` | `TestCancellationDuringOnlyContentsRequest`, `…FinalContentsRequest`, `TestDeadlineDuringFinalContentsRequest`, `TestCancellationDuringRetryBackoff`, `TestAnalyzeCancellationOnFinalContentsRequest`, `TestCorpusPipelineFinalFileCancellation` — all synchronised on events, none on sleeps | 404 and other individual failures stay partial reads (`TestAnalyzeDegradesWhenDecisiveSourceUnfetched`, corpus 404 degradations) |
| **R8** no witness validation; corpus unfinished | `6f4d630`, `e4b87c1`, `5609a87`, `61d17d1`, `2481f2c` | `witness.go` (field/span/listing/scope witnesses re-derived from the inspection); `validateCandidates` in `resolve.go`; `internal/corpus`, `cmd/corpuscapture`, `testdata/corpus` | `TestInvalidWitnessesAreRejected` (invalid and unread paths, wrong values, missing fields, mismatched quotes, out-of-range spans, spans in a clipped file's unread region, matching words in a comment, an inspected but unrelated file, missing witnesses and spans); `TestRejectedEvidenceIsReResolved` | `TestValidWitnessesAreAccepted`; `TestEveryEmittedObservationHasAValidWitness`; the corpus positives in `TestCorpusSnapshots` and `TestCorpusThroughThePipeline` |

`0de32bb` fixes formatting the first pass left behind; it changes no behaviour.

## 2. Results

### Commands (head `dcafee3`, run after the last change)

| Command | Result |
|---|---|
| `go test -count=1 ./...` | pass — 9 packages with tests, 0 failures |
| `go test -race -count=1 ./...` | pass |
| `go vet ./...` | pass |
| `go build ./...` | pass |
| `gofmt -l .` | no files |
| `git diff --check` | pass |
| Reviewer controls, isolated `git archive` of HEAD (the validation's recipe) | 15/15 classifier controls pass; the final-file cancellation control passes |
| Reviewer controls at `0ca94e4` (baseline reproduction, Task 0) | 15/15 classifier controls fail; cancellation control fails — as reported |

Mock HTTP listeners ran on loopback; no check was blocked.

### Synthetic controls

The fifteen reviewer controls and the eleven original counterexamples pass as
maintained tests, alongside their supported counterparts. These are selected
boundary cases; they are not an accuracy measurement.

### Real-repository snapshots

Six repositories captured read-only at resolved commits (provenance and
reference judgments: `internal/crawler/testdata/corpus/README.md`). "Before"
is the reviewed commit `0ca94e4` run on the same snapshots; "after" is HEAD.

| Repository | Reference (complete-enough) | Before → after, complete-enough | Before → after, pipeline (selection only) |
|---|---|---|---|
| Plantly | mobile, resolved | mobile/resolved → mobile/resolved | mobile/resolved → mobile/resolved |
| Starship | cli, resolved | cli/resolved → cli/resolved | cli/resolved → cli/resolved |
| GitHub CLI | cli, resolved | cli/resolved → cli/resolved (now cites both `cmd/gh` and `cmd/gen-docs`) | cli/resolved → cli/resolved |
| Excalidraw | frontend, resolved | other/inconclusive → **frontend/resolved** | other/inconclusive → other/inconclusive (names the 9 unfetched manifests) |
| PocketBase | library, resolved (documented ambiguity) | library/resolved → library/resolved | **library/resolved → other/inconclusive** — the reviewed code concluded "library" while `cmd/serve.go` and `cmd/superuser.go` were unread, the R1 defect reproduced on real data |
| Face Recognition | other, inconclusive (outside the taxonomy) | other/inconclusive → other/inconclusive | other/inconclusive → other/inconclusive |

Counts at HEAD, kept separate per set (resolved / correct / incorrect / inconclusive):

| Set | Resolved | Correct | Incorrect | Inconclusive |
|---|---|---|---|---|
| Complete-enough snapshots (6) | 5 | 5 | 0 | 1 |
| Pipeline through the mocked analysis path (6) | 3 | 3 | 0 | 3 |
| Degradation mutations on snapshots (12) | 4 | 4 | 0 | 8 |
| Pipeline degradations through the real client (7) | each asserts its own kind and status; all pass | | | |
| Archived October 30 packs, replayed depth-limited (4) | 1 | 1 | 0 | 3 |

The pipeline abstentions are acquisition limits, not classifier errors: the
canonical selection fetched 3 of Excalidraw's 12 `package.json` files and none
of PocketBase's `cmd/*.go`, and each abstention names exactly those files.

### Live results

None. No live crawl was run against the production feed or any repository
beyond the read-only corpus capture; the live feed is unchanged.

## 3. Interface and dependency changes

No new module dependencies (`go/types` is standard library). No change to the
public HTTP/JSON contract (`internal/apimodel`), the live feed, `preview`,
`selection`, `store` or the server.

Internal changes, all callers updated:

- `TreeScope.MaxDepth` and `Covers`; `githubclient.FileTree.MaxDepth` — a
  depth-limited listing now proves absence exactly down to its bound.
- `ReadState` and a per-path inspection index (`inspect.go`).
- Component inventory (`inventory.go`): components, their manifests with read
  state, and categorised dependency declarations.
- `Observation` gains `Component`, `Role`, `Witness`, `Context`;
  `Classification` gains `Secondary`. Detectors return evidence groups.
- `Witness` and its validator (`witness.go`); validation runs before
  resolution and governs what the verdict and the pack may contain.
- `githubclient.ResolveRef` (used by corpus capture only); `GetFileContents`
  now returns context errors from the final request.
- `internal/corpus` and `cmd/corpuscapture`.

### Supported syntax (bounded) and its sources

| Ecosystem | Supported | Not supported (reported, never guessed) | Source consulted |
|---|---|---|---|
| Go | package clauses, imports and aliases, calls resolved by go/types to net/http `ListenAndServe`/`ListenAndServeTLS`/`Serve`/`ServeTLS` (functions and `*http.Server` methods); `go.mod` requires via `x/mod/modfile` | own-module imports are not followed (premise withheld); third-party servers other than the listed frameworks; commands outside the module root, `cmd/` and listed `main.go` (documented blind spot) | Go spec; `go/types`, `golang.org/x/mod/modfile` docs |
| npm | `dependencies` / `devDependencies` / `peerDependencies`, `bin`, `main`/`module`/`exports`, `private`, `workspaces` | — | npm `package.json` docs |
| Python | PEP 621 `[project]` (dependencies, optional groups, scripts, gui-scripts, `dynamic`); requirements PEP 508 lines, extras, markers, `name @ url` | `-r`/`-c` includes, URL/VCS lines, local paths, `dynamic` dependencies or entry points, all of `setup.py` | pyproject.toml specification; pip requirements file format (both re-read for this pass) |
| Rust | `[lib]`, `[[bin]]`, implicit `src/lib.rs`, `src/main.rs`, `src/bin/`, `autobins`/`autolib`, edition-2015 discovery defaults, `[dependencies]` | `[[example]]`/`[[test]]` targets, workspace inheritance | Cargo book, Cargo Targets (re-read for this pass) |
| Dart | `dependencies`/`dev_dependencies` `flutter` | pure-Dart library role (outside the implemented taxonomy) | pubspec format |
| Docker | `EXPOSE` as a deployment port | any role from a Dockerfile | Dockerfile reference |

Only the pyproject.toml specification, the pip requirements file format and
the Cargo targets chapter were re-read during this pass; the other sources are
the governing references for rules carried over from the first pass and were
not re-read. Re-reading those three found three discrepancies, fixed in
`dcafee3`: edition-2015 auto-discovery, pip comment and continuation rules, and
local-path requirements.

## 4. Process documentation entry

**What the first pass improved.** Deterministic manifest choice, real source
paths, parsers instead of substrings, an inspection model with tree scope and
read outcomes, a shared selector, cancellable backoff and maintained versions of
the original eleven counterexamples. All of that was kept and built on.

**What review exposed.** Independent validation reproduced fifteen unsupported
resolved verdicts and one swallowed cancellation. The common cause was that
each improvement stopped at the input: completeness was recorded and then
dropped, manifests were parsed and then collapsed to one per basename,
categories were kept and then ignored, and observations carried paths that
nothing checked. The first report's statement that all repairs were implemented
was broader than its evidence.

**Why the second pass addresses root causes.** Each fix is placed where the
first pass leaked: absence claims now name and check the scope they need; roles
are decided per component before the repository; Go call identity comes from
the type checker; categories and optional groups survive into the decision; and
every observation is validated against the inspection before it can count. The
real-repository corpus then tested the result on data, not only on synthetic
boundaries — and reproduced the original defect (PocketBase's pipeline verdict
at the reviewed commit) before showing its repair.

**What the corpus changed.** Two policy rules were added after seeing real
data, and are recorded as such: comparing nested components requires a
complete listing (found by replaying the abbreviated archived packs), and an
executable workspace member outranks library members (found with Excalidraw's
standalone `@excalidraw/utils`). Citing every Go command came from GitHub CLI.

**Evidence for acceptance.** The tables above; the commands in §2; and the
reviewer's own controls passing unmodified against HEAD.

## 5. Residual gaps (for review, not resolved here)

- **Acquisition coverage.** The canonical selection misses decisive files in
  crowded trees (Excalidraw's workspace manifests, PocketBase's `cmd/`), so the
  pipeline abstains there. Adaptive second reads remain deferred by the plan.
- **Python framework extensions.** A package that depends on Flask at runtime
  (a Flask extension) is reported as a backend; telling it from an application
  needs application-construct evidence, which is not implemented.
- **Workspace policy.** A library monorepo with one playground app outside the
  examples/docs/tooling hints is reported by that app.
- **PocketBase's kind** is a documented taxonomy ambiguity (framework structure
  versus backend product framing); the classifier reports the structure.
- **Scope witnesses** re-check that their files were consumed (and, for
  no-server premises, re-scan them); they do not re-derive every part of the
  extractor's reasoning.
- **Revision identity.** The corpus is captured at resolved commit SHAs; the
  analysis path itself still records a branch name as unresolved. Full SHA
  resolution in the analysis path remains separate work.
- **Scope of claims.** Selected repositories and controls, not a general
  accuracy measurement. These repairs do not complete G17–G19, full preview
  integration, runtime safety or the capstone's usefulness evaluation.
