# Classification repair, second pass: preserve the progress and finish the evidence contract

Prepared October 31, 2026, after independent validation of
`fortune/classification-review-repairs` at
`0ca94e4d4f9349a5df2ae1bf2cf42c15f64a5709`.

**Status: implementation handoff; all tasks below remain pending.** This replaces
the [first-pass brief](classification-repair-plan-first-pass.md), preserved for
process documentation. Continue from the implemented branch. Acceptance requires
closing the remaining findings while preserving supported positive results.

## Assignment

Read the [independent validation report](../docs/reviews/2026-10-31-classification-repair-validation/README.md),
its [classifier controls](../docs/reviews/2026-10-31-classification-repair-validation/reviewer_crawler_test.go.txt),
[cancellation control](../docs/reviews/2026-10-31-classification-repair-validation/reviewer_githubclient_test.go.txt)
and [captured output](../docs/reviews/2026-10-31-classification-repair-validation/results.txt).
Check HEAD and uncommitted work before editing; preserve other agents' changes.
Account for any fixes made since the reviewed commit.

Implement the ordered tasks below. Reproduce each failure before fixing it,
promote the relevant controls into maintained tests with the fixing change, and
keep commits focused. Continue through acceptance; the original eleven
regressions alone do not finish this assignment. This document directs the next
implementing agent; the reviewing agent has changed documentation only.

## What the first repair achieved — keep these parts

The tracked suite, fresh race checks, vet and build passed at the reviewed commit.
The maintained versions of all eleven original counterexamples passed. Build on
these improvements rather than restarting the classifier.

| Useful change | First-pass evidence | What still needs strengthening |
|---|---|---|
| Explicit inspection, tree scope, read outcomes and unresolved revision identity | `6af1cd6`; `evidence.go`, evidence tests | Inference must consume truncation, parse and unavailable states |
| Deterministic manifest choice and actual source paths | `195c67d`; inventory tests | Retain independent components instead of one manifest per basename |
| Go AST/module parsing, JSON dependency categories, TOML and YAML parsers | `2c5191e`, `33092b4`, `7259f88`, `e1e63b0` | Syntax must support the exact observation; categories must survive inference |
| Primary/secondary candidate handling | `d9bda62`; resolution tests | Use owning components and artifacts beyond directory-name ranks |
| Shared selector and interruptible retry backoff | `5e356e2`; command/client tests | Preserve omitted evidence and propagate final-file cancellation |
| JS tooling distinction, conventional Rust targets and modern Python scripts | `ec90532`, `259a847`, `7d6e520` | Preserve real positives while correcting unsupported premises |
| Maintained regressions and mocked acquisition-to-classification path | `9a9545b`, `b1b9a9c` | Broaden boundary controls and complete repository acceptance |

The [first remediation report](../docs/reviews/2026-10-31-classification-repair-results.md)
records implementation results. Independent review reproduced **15 unsupported
resolved classification verdicts and one swallowed cancellation** beyond those
tests. These selected adversarial cases do not estimate general accuracy. Some
behaviors predate the repair; they still leave the agreed work open.

## Scope and recommended design

Retain the public HTTP/JSON contract and existing kind enum. Internal types may
change with all callers updated. Keep classification pure and offline; acquisition
supplies a bounded inspection. The live feed stays metadata-only via
`preview.FromMetadata` until later preview-pipeline work.

Keep four responsibilities explicit:

1. **Inspection:** what was listed, selected, fetched and parsed, with original
   paths, revision identity, read outcomes and bounds.
2. **Component inventory:** which manifests, sources and targets belong together.
3. **Observations:** supported declarations/constructs, source witnesses and
   dependency categories. A type declaration, call and running process differ.
4. **Resolution:** which validated observations support primary/secondary roles
   and which conclusions require unavailable evidence.

Use a small documented policy and a bounded supported syntax subset. Do not fix
component bugs by rearranging detector order or adding repository-name rules.
Do not replace unsupported certainty with blanket abstention.

Retain the [GitDiagram study](../docs/reviews/2026-10-30-gitdiagram-architecture.md)
and [research assessment](../docs/reviews/2026-10-31-research-plan-assessment.md)
boundaries: a real path or matching quote establishes provenance, not semantic
support; static inspection does not establish execution. Keep setup/contribution
evidence eligible instead of copying architecture-only source exclusions.

Defer model generation, target execution, universal reference resolution, adaptive
second crawling, raw-CDN recovery, ETag caching, SSE, new storage, Redis/Postgres,
personalization, deployment and SwiftUI changes. Full commit-SHA resolution remains
separate; never label an unresolved branch as an immutable revision.

## Execution order and tracking

Run **0 → 1 → 2 → 3 → checkpoint A → 4 → 5 → 6 → checkpoint B →
7 → 8 → 9 → checkpoint C → 10 → 11 → final verification**.

Each task should fit a focused session and normally touch two to five files.
Split larger tasks into compiling, tested substeps. Task 8 is independent and may
move earlier. Keep shared evidence/component interface changes sequential.

Linear remains the project tracker. This file supplies implementation detail and
an ordered index, not a second status database. No external records were changed.
This documentation request does not authorize publishing notes or changing issues.

| Existing issue | Work in this handoff |
|---|---|
| [G15 / SAV-21](https://linear.app/savios-workspace/issue/SAV-21) | Tasks 1–7, 9–10: classifier and evidence |
| [G16 / SAV-22](https://linear.app/savios-workspace/issue/SAV-22) | Tasks 1, 8, 11: acquisition evidence |
| [G37 / SAV-41](https://linear.app/savios-workspace/issue/SAV-41), [G39 / SAV-43](https://linear.app/savios-workspace/issue/SAV-43) | Regression verification throughout |
| [G38 / SAV-42](https://linear.app/savios-workspace/issue/SAV-42) | Task 11 contributes analysis integration evidence; full preview integration remains separate |

## Task 0 — establish the baseline

Dependencies: none. Scope: small; test harness and result notes.

Reproduce the archived controls against current HEAD. Adapt setup to current
internal interfaces without weakening the tested invariant. Keep historical pack
replay separate: it prints partial-input results, not positive acceptance.

Acceptance:

- Record starting commit, standard-suite results, reproduced failures and any
  intervening commit that already fixes a finding.
- Map R1–R8 to the tasks and retained test names. Do not change expected outcomes
  merely to match current behavior.
- Add maintained controls with their corresponding fixes; do not land a standalone
  intentionally failing normal-suite commit.

Verification: run the original suite and reviewer controls separately, recording
actual outcomes. No application behavior changes in this baseline step.

## Task 1 — make read/parse state govern absence claims (P1 / R1)

Dependencies: 0. Likely files: `evidence.go`, `classify.go`, `goanalysis.go`,
focused evidence tests. Scope: medium; split plumbing from ecosystem policies.

Keep selected sources as records with read, truncation and parse state. Define the
inspected scope needed by each negative premise. A complete listing is not parsed
source; appending a limitation does not validate an unsupported resolved verdict.
Prefer bounded positive artifact evidence over global absence assertions; do not
fetch every file to manufacture completeness. If an old positive test relied on
unread source, enrich its fixture with the required inspected evidence rather than
preserving an unsupported premise or weakening its assertion.

Acceptance:

- Clipped/malformed Go entries cannot establish CLI by absence of a server.
  A listing containing unread `app.go` cannot prove there is no `package main`.
- Truncated Rust trees and clipped Python manifests cannot establish “no binary”
  or “no scripts.” Failed root manifests stay visible and cannot silently promote
  examples to primary products.
- Retain actual positive facts from partial files and supported complete positives.
  Limitations name the material missing path/region or parse failure.

Verification: all six R1 boundary cases, paired complete/partial cases, then
`go test ./internal/crawler`.

## Task 2 — retain component boundaries in the inventory (P1 / R2)

Dependencies: 1. Likely files: component/index helper, `classify.go`, inventory
tests. Scope: medium.

Inventory relevant manifests by original path and owning component root. Retain
nested manifests even when a root one exists. Use supported manifest/workspace
structure and directory hints; document unsupported boundary cases.

Acceptance:

- Multiple package, Go, Cargo and container manifests remain inspectable with
  their owners; global basename slots do not discard independent evidence.
- Root library plus `ui/` app and root Go library plus `packages/demo/go.mod`
  retain separate sources, entries and dependency observations.
- Shuffled tree/file/map construction yields the same normalized inventory and
  ordered evidence, preserving original path/case and inspection state.

Verification: ownership and input-permutation tests, then crawler suite.

## Task 3 — resolve roles within those boundaries (P1 / R1, R2)

Dependencies: 2. Likely files: resolution helper, `classify.go`, resolution tests.
Scope: medium.

Extract candidates per component before resolving repository purpose. Do not
combine a root library's React dependency with an independent UI's entry or borrow
a nested module's main package. Root location identifies ownership but does not
automatically prove primary product purpose.

Acceptance:

- Both R2 fixtures stop promoting the independent nested artifact to primary.
  Secondary observations remain attributable to their owners.
- Complete backend/supporting-UI and library/example fixtures retain supported
  primary results. Genuinely unresolved primary candidates produce an explained
  inconclusive result rather than a detector-order winner.
- Removing decisive primary evidence changes the decision appropriately while
  retaining valid secondary observations.

Verification: mixed-component, evidence-removal and full-result ordering tests;
`go test ./internal/crawler ./cmd/evidencepack`.

### Checkpoint A — evidence ownership

R1/R2 controls and supported counterparts pass. Inspect representative outputs:
each negative premise needs sufficient inspection, and each combined observation
needs the right owner. Continue; this is not overall acceptance.

## Task 4 — recognize supported Go calls precisely (P1 / R3)

Dependencies: 3. Likely files: `goanalysis.go`, observation extraction, Go tests.
Scope: medium.

Bind supported package calls to real imports/aliases. Recognize receiver calls
only when HTTP identity is established within the bounded analysis; explain
unsupported identity instead of guessing. Retain original construct locations.

Acceptance:

- Custom `logger.ListenAndServe()` and unused `http.Server{}` do not produce HTTP
  listening-call evidence or independently establish backend.
- Real direct/aliased `net/http` calls retain precise static observations;
  unsupported receiver/shadowing patterns do not fabricate identity.
- Separate types/literals from calls. Describe inspected syntax without claiming
  successful startup. Existing comment/string negatives still pass.

Verification: paired Go positives/negatives and crawler suite.

## Task 5 — preserve JS dependency category during inference (P1 / R4)

Dependencies: 3. Likely files: JS extraction/detectors and package/role tests.
Scope: small to medium.

Apply category-aware reasoning to backend/mobile as well as frontend. Dev/peer
declarations can describe tooling or integration without establishing the primary
application. Relevant artifact evidence still matters; category alone is not a
universal classification rule.

Acceptance:

- Development-only Express cannot independently make an npm CLI backend; peer-only
  Expo cannot independently make an exported library mobile.
- Genuine Node backend, Expo mobile and frontend fixtures stay supported; npm
  CLI/Vite and UI-library/demo regressions remain fixed.
- Observations preserve actual manifest field/category and component identity.

Verification: backend/mobile/tooling positive pairs and crawler suite.

## Task 6 — preserve Python optional groups (P1 / R4)

Dependencies: 3. Likely files: `pyanalysis.go`, Python extraction/role tests.
Scope: small to medium.

Keep mandatory dependencies and named optional groups separate. Bound requirements
includes, markers and unsupported declarations explicitly; do not fetch remote
includes or execute packaging code.

Acceptance:

- Flask only in a docs extra cannot independently establish backend; its
  observation identifies the optional group.
- Supported runtime/requirements-based web applications retain their positive
  results with relevant inspected evidence; name/comment/tooling negatives remain.
- Unsupported syntax stays visible when material, rather than becoming an empty
  mandatory dependency set that justifies a negative conclusion.

Verification: optional/runtime pairs, requirements and Python role tests.

### Checkpoint B — semantic declarations

R3/R4 controls and supported counterparts pass. Inspect import identity,
observation wording and dependency categories. A parsed declaration does not
automatically establish the repository's role.

## Task 7 — separate container ports from product role (P2 / R5)

Dependencies: 3. Likely files: backend detector and container/role tests.
Scope: small.

Acceptance:

- `FROM alpine` plus `EXPOSE 8080` alone remains inconclusive about backend role.
- Preserve the deployment-port observation; an independently supported backend
  with the same Dockerfile still resolves backend.
- No wording claims the container was observed listening or successfully run.

Verification: Docker-only/real-backend pair and crawler suite.

## Task 8 — propagate cancellation at the final file (P2 / R7)

Dependencies: 0; independent of classifier work. Likely files: `contents.go`,
client tests, command integration tests. Scope: medium.

Acceptance:

- Cancellation/deadline expiry during the only/last content request returns the
  context error immediately, including through `analyze`; no next iteration is needed.
- Ordinary 404/individual failures remain partial reads. Existing file/byte caps
  and cancellable retry backoff remain supported.
- Tests synchronize on request receipt and controlled cancellation, covering
  only-file, final-file and retry-backoff cases without arbitrary sleeps.

Verification: R7 mock-server control, client and command integration suites.

## Task 9 — bound legacy Python entry interpretation (P2 / R6)

Dependencies: 6. Likely files: Python entry extraction, `classify.go`, Python tests.
Scope: small to medium.

Remove substring matching for `console_scripts`. Support a documented static
declaration subset or mark dynamic legacy entry metadata uninspected. Never
execute/evaluate `setup.py`. Unsupported metadata cannot prove “no commands.”

Acceptance:

- Comments and incidental strings mentioning `console_scripts` cannot establish CLI.
- Modern `[project.scripts]` positives remain and GUI entries stay distinct.
  Any retained legacy positive has actual static declaration evidence.
- Dynamic/unsupported inputs expose limitations and do not become confidently
  importable-only libraries by fallback.

Verification: comment/string/static/dynamic cases and crawler suite.

### Checkpoint C — reproduced behavior failures

All fifteen classifier controls and the cancellation control pass, with maintained
positive counterparts and original regressions. Source validation and repository
acceptance below are still required.

## Task 10 — validate source witnesses before using observations (P2 / R8)

Dependencies: 4–9. Split into two medium substeps: witness/validator plus tests,
then extractor/resolver/renderer integration. Update internal callers together.

Extend the observation model only as needed: original path/component plus a
resolvable parsed manifest field or original-file byte span. AST call observations
need the actual construct span. If a quote is retained, validate it against those
exact bytes. Tree-wide facts need an explicit scope basis, not a fabricated file.

Validate support against inspected content before it affects the verdict or
renders in the pack. Reject invalid support and re-resolve its candidate, retaining
unrelated valid observations. A matching substring alone cannot validate the
claimed declaration, call identity or relationship.

Acceptance:

- Witnesses resolve to consumed source, with original byte offsets and actual
  manifest fields/categories/values. Omitted regions and inspected-but-unrelated
  files cannot support fabricated facts.
- Reject invalid paths, mismatched quotes, out-of-range spans and missing required
  witnesses. Valid manifest-field witnesses need not invent spans; AST observations
  cannot pass with required construct spans omitted.
- Removing decisive valid support or rejecting invalid support downgrades the
  verdict appropriately. Decisions and rendered packs use the same validated facts.

Verification: witness mutation/semantic controls, original-offset tests and
`go test ./internal/crawler ./cmd/evidencepack`.

## Task 11 — complete repository and pipeline acceptance (P2 / R8)

Dependencies: 10. Split fixture collection/reference judgments from offline
analysis-path tests; each should fit a focused change.

### 11a — capture the corpus and reference judgments

Cover feed repositories Excalidraw, PocketBase, GitHub CLI and Starship, and study
repositories Excalidraw, PocketBase, Plantly and Face Recognition: six unique
repositories, with shared snapshots for overlapping members.

Store sanitized tree/content/read-outcome fixtures, relevant budgets, source URL,
capture date and verified revision where available. Record unresolved identity
honestly. If historical packs lack decisive source, collect missing artifacts
through bounded read-only inspection. Do not relabel abbreviated trees complete.

Acceptance:

- Each fixture has expected kind/status and a reference judgment grounded in
  inspected artifacts, including component rationale. No slug allowlist or
  presumed product reputation determines the verdict.
- Supported positive snapshots contain enough evidence; paired mutations remove,
  truncate or fail decisive evidence and specify degradation. Face Recognition
  may legitimately remain outside this taxonomy.
- Preserve original incomplete packs as separately labelled degradation cases.
  Tests require no network/credentials and fixtures contain no secrets.

Verification: inspect provenance and reference judgments before asserting labels.
Document taxonomy ambiguity instead of forcing a preconceived kind.

### 11b — prove the bounded analysis path

Drive metadata → tree → canonical selection → bounded contents → inspection →
component observations → source validation → resolution → pack rendering through
mock GitHub responses. Exercise meaningful corpus cases through this path;
classifier-only fixtures cannot prove acquisition preserves the evidence.

Acceptance:

- Cover no-README positives, failed root/decisive reads, tree/depth caps, truncated
  manifests, crowded selection and final-file cancellation. Selection/fetch
  omissions remain visible and cannot prove absence.
- Assert expected kind **and status per fixture**, source observations and material
  limitations. Decisive-source removal changes the verdict; rejected evidence
  cannot survive in the rendered pack.
- Report resolved, correct, incorrect and inconclusive counts separately, with
  per-case outcomes. All complete supported positives must retain expected results;
  blanket abstention cannot satisfy acceptance.

Verification: focused offline corpus and mocked-pipeline tests, then final checks.
Use one requested ref consistently; branch-only capture is not immutable/atomic.
Leave historical evidence packs unchanged.

## Final verification and handoff

After the final code change run:

```sh
go test -count=1 ./...
go test -race -count=1 ./...
go vet ./...
go build ./...
git diff --check
```

Rerun affected checks after subsequent fixes. Mock HTTP listeners may need
environment permission. Report blocked checks honestly. Inspect the diff for
accidental API/feed changes, historical fixture edits and unrelated work.

Return an updated remediation record containing:

1. R1–R8 → fixing commit → code location → maintained regression, including both
   invalid-evidence rejection and supported-positive preservation.
2. Actual command results and per-fixture before/after kind/status, resolved
   correctness and abstentions. Separate synthetic, real-snapshot and optional live
   results. Selected review failures are not a general accuracy measurement.
3. Internal interface/dependency changes, rationale and supported/unsupported
   syntax. Use official parser/ecosystem documentation for new interpretation rules.
4. A local process-documentation entry: what the first pass improved, what review
   exposed, why the next fixes address root causes, and evidence for acceptance.
   Preserve prior review results; do not rewrite failures as if they never occurred.

Do not mark this plan or G15/G16 accepted with missing required tasks, corpus
members, validation controls or checks. Document residual gaps for review.
These repairs do not complete G17–G19, full preview integration, runtime safety
or the capstone's usefulness evaluation.
