# Agent implementation brief: close the classification review findings

Prepared: October 30, 2026. Reviewed baseline:
`3bf15746e053a391e95d7752c9c97684187bb774`.
Revised: October 31, 2026 after inspecting the two reports in
`/Users/fdeclan/Public/gittok-research/` and checking their claims against the
included GitDiagram checkout at `7876ab0`. The GitTok baseline is unchanged.
Deliverable: implementation instructions for Declan's implementation agent.
This document does not claim the repairs have been implemented or approved.

## Copyable assignment

Implement the classification and evidence-handling repairs below in
`/Users/fdeclan/Public/gittok-ios-backend`. Start by checking current HEAD and
uncommitted work. Preserve existing review artifacts and other agents' edits.
Use the archived review cases to write maintained regression tests before each
behavior change. Implement the ordered slices, verify each slice, and report
which original finding each change closes. Continue through the assignment;
do not stop after producing another plan or fixing only nondeterminism.

Keep the existing HTTP/JSON contract intact. Internal crawler interfaces may
change when necessary; update all callers and tests together. Prefer the smallest
explicit evidence model that represents what was actually inspected. An honest
inconclusive classification is a valid result when evidence cannot support a
decision. Neither language popularity nor a filename is proof of runtime behavior.

## Read first

1. [Original review and reproduction instructions](../docs/reviews/2026-10-30-classification/README.md).
2. [Archived counterexamples](../docs/reviews/2026-10-30-classification/review_adversarial_test.go.txt)
   and [snapshot fixtures](../docs/reviews/2026-10-30-classification/review_fixtures.json).
3. [GitDiagram architecture study](../docs/reviews/2026-10-30-gitdiagram-architecture.md):
   borrow evidence provenance, explicit completeness and deterministic enforcement;
   its runtime-only exclusions and citation checks are not sufficient for GitTok.
4. `internal/crawler/classify.go`, `selectfiles.go`, and their tests;
   `internal/githubclient/tree.go` and `contents.go`;
   `cmd/evidencepack/main.go`; `internal/apimodel/claim.go`;
   `docs/api-contract.md`.
5. [October 31 research assessment](../docs/reviews/2026-10-31-research-plan-assessment.md):
   verified takeaways, corrections to the other agent's research, and the
   rationale for this revision. Read this before adopting its recommendations.

The archived tests are diagnostic evidence, not an instruction to copy every
expected label unquestioningly. Several intentionally supply incomplete
evidence. For those cases, an explicit inconclusive result may be the correct
fix. Explain such changes rather than weakening tests to preserve old behavior.

## Tracker and scope

Linear remains the working tracker. This brief supplies repair instructions
under existing work; no new tracker items or status updates were created by
the reviewing agent.

| Existing work | Instructions here |
|---|---|
| [G15 / SAV-21](https://linear.app/savios-workspace/issue/SAV-21) | Slices 1–7: classifier correctness, evidence and ecosystem conventions |
| [G16 / SAV-22](https://linear.app/savios-workspace/issue/SAV-22) | Slice 8: canonical selection and evidence handoff |
| [G37 / SAV-41](https://linear.app/savios-workspace/issue/SAV-41) and [G39 / SAV-43](https://linear.app/savios-workspace/issue/SAV-43) | Maintained unit/regression checks in every slice |
| [G38 / SAV-42](https://linear.app/savios-workspace/issue/SAV-42) | Slice 9's analysis-path test contributes evidence; it does not complete the future full preview integration ticket |

Do not mark a ticket complete solely because its code exists or its synthetic
tests pass. Do not change issue descriptions/statuses or publish process notes
unless the implementation task/session authorizes those external updates.

In scope: internal classification, inspection provenance, file-selection handoff,
tests, and documentation of results. Out of scope: LLM generation, a universal
call graph, video rendering, target-code execution, new infrastructure, profile
ranking, deployment, or redesigning the SwiftUI/public API. These are repairs
to G15/G16, not an implementation of the entire GitDiagram-inspired pipeline.

## Execution order and research boundaries

Keep the original slice IDs for traceability, but execute **1 → 2 → 3 → 4 → 8
→ Checkpoint A → 5 → 6 → 7 → Checkpoint B → 9**. Acquisition can erase the
completeness information introduced by the earlier slices, so Slice 8 belongs
inside the P1 repair phase, before ecosystem expansion.

The research suggests useful future work but does not change this repair into
a reference-resolver, generation, storage or deployment project. A deterministic
reference inventory can contain false positives and does not establish runtime
calls, relationship direction or claim truth. Keep candidate references separate
from parsed observations and role inferences. Do not port GitDiagram's regex
resolver as a substitute for the syntax-aware extraction required here.

Likewise, `observed_running` describes a particular execution under particular
conditions; it is not a global confidence upgrade for every repository claim.
Preserve the existing confidence/stance/execution-mode distinctions. Do not copy
the research's proposed replacement API fields or new error/streaming endpoints.

## Required invariants

- Identical evidence produces an identical verdict and deterministically ordered
  evidence/limitations, regardless of map insertion or tree order.
- Every positive observation identifies its actual source path and the inspected
  field, excerpt or source span. Never relabel a nested manifest as root.
- An existing/read path and a matching quote establish provenance, not semantic
  support. The parsed declaration or source construct must support the particular
  observation. Never automatically attach a nearby/importing file and upgrade a
  conclusion merely to fill a missing citation.
- Inspected source and full-tree knowledge remain distinct. A fetch failure,
  excluded depth, truncated tree/file, missing selection or parse failure cannot
  establish that a repository feature is absent.
- A dependency declaration, import, entry point, inferred project role and
  observed execution are distinct facts. Static classification never establishes
  that the software ran.
- Primary-product evidence is scoped to its component. An example, demo, docs
  site or admin UI does not automatically determine the whole repository kind.
- A removed/invalid source cannot leave a positive verdict with unsupported
  reasoning. Preserve valid observations while downgrading unresolved conclusions.
- Unknown/unsupported/ambiguous inputs can use existing `KindOther` at the
  presentation boundary, with an explicit internal status and explanation.
  Do not add new public enum values just to express uncertainty.

## Slice 1 — represent inspection completeness and structured evidence (P1)

Dependencies: none. Likely files: crawler evidence/result types and tests;
GitHub tree metadata and tests if needed. Keep each change to roughly 3–5 files.

Replace the lossy `language + paths + map[string]string` boundary with an
internal evidence input that can represent tree scope and per-file read status.
Retain original paths, available revision identity, returned text, truncation,
and skipped/unavailable/parse-failure reasons. Reuse existing GitHub result types
where sensible; do not create parallel copies with inconsistent semantics.

Distinguish bytes retrieved from bytes actually consumed by the analyzer, and
track the original offsets/ranges of any retained excerpts. If later inspection
reads a full file but shows only an index or excerpts to a model, those must
remain separate statuses. This repair does not need to add a model-facing index.

Return structured observations and explicit verdict status/limitations. Keep
human-readable evidence renderable from those fields. Do not invent a numeric
confidence score.

Acceptance:
- The same path listing with successful vs failed source reads can yield
  different completeness/limitations; failure is never silently treated as empty content.
- A depth-filtered or capped tree cannot justify a global “no main package”
  assertion. Existing `FileTree.Truncated` does not track depth filtering:
  add appropriate scope metadata or conservatively carry the configured scope.
- A Go module with an unread `cmd/service/main.go` is not declared CLI solely
  because no server call was inspected. Malformed/truncated JSON manifests
  produce visible limitations without panics.
- Evidence spans and quotes refer to the original file, not concatenated excerpt
  positions. An omitted region cannot be searched as if it had been inspected.

Verify focused evidence tests and `go test ./internal/crawler ./internal/githubclient`.

## Slice 2 — deterministic manifest inventory with component provenance (P1)

Depends on Slice 1. Likely files: crawler indexing/component helpers and tests.

Inventory all relevant manifests rather than collapsing each basename to one
global slot. Preserve original path/case for citations. Define deterministic
selection: explicit component scope, root preference where relevant, then depth
and lexical order as tie-breakers. Never depend on Go map iteration.

Identify lightweight boundaries from manifests/workspaces and directory
structure. Keep examples/docs/tooling distinguishable from the main product;
folder names are hints, not a complete semantic component detector.

Acceptance:
- A root library with a nested Expo example yields the same full result across
  shuffled input construction and repeated calls. The example cannot establish
  a mobile primary product.
- Multiple `go.mod`, `package.json`, Cargo and Docker files do not overwrite
  each other's provenance or supply arbitrary cross-component signals.
- Evidence from `apps/mobile/package.json` cites that exact path.

Verify focused determinism/provenance tests, including byte-equivalent normalized
results for input permutations, then the crawler suite.

## Slice 3 — extract supported declarations rather than matching words (P1)

Depends on Slices 1–2. Deliver as separate parser-sized changes (3a–3d), each
with its own tests; do not add every parser in one large commit.

**3a — Go:** use `go/parser` and syntax-aware dependency parsing. Distinguish
real package declarations/imports/call expressions from comments and strings;
handle import aliases. Detect supported server patterns conservatively, without
equating every server-related import with server execution. Associate findings
with the owning component and original source location.

**3b — JavaScript/TypeScript manifests:** retain dependency category
(runtime/dev/peer), scripts, executable entries and library exports separately.
Preserve manifest field references and distinguish declarations from role inference.

**3c — Python declarations:** parse supported TOML project/dependency fields and
requirements declarations. A project name, comment, description, URL, optional
extra or unrelated tooling section is not a runtime dependency. Never execute
or evaluate `setup.py`; unsupported dynamic declarations remain inconclusive.

**3d — Dart/container metadata:** check an actual Flutter SDK dependency rather
than the word “flutter.” An EXPOSE instruction records a declared container port;
it does not by itself establish a backend product or observed listening behavior.

Prefer existing/standard parsers. Where a maintained format parser is needed,
document its purpose and use official documentation; avoid writing an accidental
partial parser that silently claims full format support.

Acceptance:
- Commented `ListenAndServe`, commented removed Gin dependencies,
  `flask-utils` with no Flask dependency, and a comment excluding Flutter
  produce no corresponding positive observation.
- Equivalent real declarations still produce precise source-backed observations.
- Static evidence uses wording such as “declares a dependency” or “contains a
  call expression”; it never says “successfully starts/runs” without run evidence.
- A valid, inspected but unrelated file is rejected as support for a declaration
  or role it does not establish. Imports do not automatically establish invocation,
  and an observation cannot reverse the caller/callee represented by source syntax.

Verify focused parser tests after each sub-slice and then the crawler suite.
Read a newly used parser's official documentation before integrating it.

## Slice 4 — resolve the primary role using scoped positive evidence (P1)

Depends on Slices 1–3. Likely files: role resolution policy and classifier tests.
Keep observation extraction separate from role arbitration.

Replace global “first matching detector wins” with an explicit small resolution
policy. Compare relevant component/artifact signals. For a backend with a
supporting UI, the UI can remain a secondary component without overriding the
backend. A library containing runnable examples remains eligible as a library.
For genuinely mixed or insufficient evidence, return an inconclusive verdict
with supported candidates/limitations rather than inventing certainty.

Acceptance:
- A complete minimal backend + admin-UI fixture resolves backend; the UI evidence
  remains attributable to the UI component.
- A root library with an example server is not promoted to backend merely
  because the example declares `package main` and listens on a port.
- Removing decisive backend source from that fixture changes the result to
  insufficient/ambiguous where appropriate. The partial stored PocketBase pack
  must not be forced to backend by hardcoding its name.

Verify focused mixed-component and evidence-removal tests, then all crawler tests.

## Slice 8 — use one selector and preserve evidence through acquisition (P1/P2)

Depends on Slices 1–4; execute before Checkpoint A and the P2 slices.
Likely files: selector helpers/tests and evidence-pack command/tests. Separate
selector reuse, acquisition plumbing and cancellation fixes into sub-slices
8a–8c if needed to keep each change within roughly five files.

Have `cmd/evidencepack` call the canonical crawler selector rather than keep a
divergent copy. Feed complete paths, read outcomes and selection limits into
classification. Keep retrieval/orchestration separate from pure classification.

Entry reservation is not a guarantee of sufficient evidence. Selection or
fetch-budget exhaustion must stay visible downstream. Include decision-relevant
contribution/setup files; do not copy GitDiagram's architecture-only exclusions.
Use deterministic caps/diversity and component scope without building a universal
dependency crawler in this repair.

Keep selection budget, fetched bytes, parsed bytes and total collection time
distinct. Enforce the caller's total acquisition deadline through retries and
backoff, not just through each HTTP request. Record budget exhaustion as partial
inspection. The current GitHub client uses `time.Sleep` during retry backoff;
check that cancellation is interruptible instead of assuming a request context
also interrupts that sleep. Make the smallest client change needed if the
regression exposes it.

Give important component manifests and entry points a chance under a crowded
tree. Retain tests, contribution guidance and CI/setup artifacts as eligible
evidence where relevant; do not copy a one-manifest global cap. Report skipped
candidates and why they were skipped so a later bounded second read can build
on the same ledger. Adaptive import-following is deferred, not required here.

Acceptance:
- The command and tested selection path agree for the same tree/budget.
- A relevant source skipped by selection or fetching causes explicit
  incompleteness rather than a negative conclusion.
- Existing caps remain enforced; bounded inputs do not create unbounded retries
  or extra inspection. Tests cover source reservation under manifest pressure.
- A hanging response and cancellation during retry backoff stop collection
  promptly using deterministic synchronization/controlled waits in tests. No new
  inspection round begins after the total budget is exhausted.

Verify `go test ./internal/crawler ./cmd/evidencepack ./internal/githubclient`.
Do not overwrite historical checked-in packs while running this verification.

### Checkpoint A — P1 evidence and classifier repair

Run this checkpoint after Slice 8, before starting Slices 5–7. Show the four P1
findings mapped to regression tests and changed code. Verify
that unknown evidence is not being converted into negative evidence. Preserve
a clean build and passing current suites. Summarize remaining P2/integration
work and continue; this checkpoint is not permission to declare G15 complete.

## Slice 5 — separate JS tooling, libraries and executable products (P2)

Depends on Slice 4. Likely files: JS observation/resolution helpers and tests.

A Vite/React dev dependency or an arbitrary dev/start script cannot establish a
frontend app alone. Use executable entries, library exports, app configuration
and source/entry evidence in component context. Do not simply move CLI before
frontend globally: a legitimate product can provide both.

Acceptance:
- An npm package with `bin` and `vite build --watch` resolves CLI when the
  inspected artifact supports that interpretation.
- A published UI library with a Vite demo is not treated as a frontend product.
- The existing genuine React/Vite app and Expo mobile cases remain supported.

Verify focused JS role tests and the crawler suite.

## Slice 6 — support conventional Rust targets (P2)

Depends on Slice 4. Likely files: Rust target interpretation and tests.

Use Cargo's documented implicit `src/main.rs`, `src/bin/`, and `src/lib.rs`
targets alongside explicit target tables. Respect relevant autodiscovery
settings, explicit paths and workspace/component boundaries within the declared
supported subset. CLI parser dependency presence alone does not prove a binary.

Acceptance:
- Implicit binary + Clap fixture resolves CLI without requiring `[[bin]]`.
- Implicit library fixture resolves library without requiring `[lib]`.
- Explicit target overrides, disabled autodiscovery, and mixed lib/bin cases do
  not trigger fabricated absence claims; unsupported cases explain limitations.

Verify focused Rust tests and the crawler suite.

## Slice 7 — support modern Python entry points and backend inputs (P2)

Depends on Slice 3c and Slice 4. Likely files: Python entry-point/role helpers
and tests. Split requirements and entry points if the change exceeds one session.

Read `[project.scripts]` as command-entry metadata and distinguish
`[project.gui-scripts]`. Include actual `requirements.txt` declarations in
evidence. Packaging metadata does not automatically mean importable-only
library, and `[tool.ruff]` alone establishes no library or app role.
Bound and document handling of includes, constraints, environment markers,
extras and unsupported declarations; do not fetch remote includes or run tools.

Acceptance:
- A supported `[project.scripts]` fixture resolves CLI.
- A requirements-based Flask app with inspected app evidence resolves backend.
- Tooling-only pyproject remains inconclusive/other without claiming a library;
  unsupported syntax preserves limitations rather than guessing.

Verify focused Python tests and the crawler suite.

### Checkpoint B — supported conventions

Verify the original eleven counterexamples as maintained tests, with an explicit
explanation wherever inconclusive is more defensible than the archived expected
label. Test true-positive counterparts as well, so rejecting false positives
does not merely disable each detector. Supported complete fixtures must retain
their expected positive verdicts; returning inconclusive for every input does
not satisfy acceptance.

## Slice 9 — prove the analysis handoff and acceptance corpus

Depends on Slices 1–8. Likely files: analysis/command integration tests, sanitized
test fixtures and documentation. Split fixture collection from the integration
test if needed.

Build one deterministic mocked path through tree retrieval → selection → bounded
contents → evidence construction → classification. Include partial failures,
depth/entry caps, truncated manifests and cancellation. Keep all source requests
on the same resolved ref; record revision as unknown if only a branch name is
available, never label a branch string an immutable commit SHA. Full commit
resolution is a separate explicit hardening slice if needed.

Use sanitized, provenance-labelled snapshots for both sets:
- Live feed: Excalidraw, PocketBase, GitHub CLI and Starship.
- Study set: Excalidraw, PocketBase, Plantly and Face Recognition.

Expected labels must come from inspected artifacts and documented reference
judgments, not owner/repository allowlists. The original packs have abbreviated
trees and missing files. Keep those as missing-evidence regressions; use separate
complete-enough fixtures to establish positive classifications. Face Recognition
need not be labelled backend: a standalone ML/script project may legitimately
fall outside the current kind taxonomy.

Acceptance:
- Tests demonstrate both positive classifications and honest degradation when
  decisive evidence is removed. A test with no README still works on code evidence.
- A result contains inspectable provenance and limitations, without network
  access or credentials in the unit/integration fixture suite.
- Source validation can reject an invalid reference and downgrade its verdict.
  No full preview or runtime execution is claimed by this classification path.
- Add negative controls with an existing but irrelevant source, a mismatched
  quote, and an invalid/omitted source span. Provenance checks alone must not
  turn these into accepted supporting observations. Report classification
  correctness and inconclusive-case counts separately; do not use citation count
  or aggregate test coverage as a substitute for either.

Do not put full repository analysis on every `GET /v1/feed` request. The live
feed remains metadata-only until the preview pipeline is implemented; document
that boundary and the remaining older metadata heuristic. Do not declare this
repair to complete G17–G19 or the full feed→preview integration milestone.

## Final verification and handoff

Run the following once final code changes are complete, and rerun affected checks
after any subsequent fix:

```sh
go test ./...
go test -race ./...
go vet ./...
go build ./...
git diff --check
```

Use standard repository commands; permission to bind local test ports may be
needed for httptest. Never mask environment failures as passing verification.
Inspect the diff for accidental edits or changed historical review fixtures.

Return:
1. A finding-to-fix matrix with code locations and specific regression tests.
2. Actual command results, remaining limitations, and any unverified live behavior.
3. Before/after results for the acceptance corpus, separating deterministic fixture
   results from any optional live checks, with source revision/capture provenance.
4. Any documented dependency or internal interface changes and their rationale.
5. A process-documentation entry describing the correction and evidence, clearly
   distinguishing implemented repairs from proposed follow-up work. Save it locally
   if external publication has not been authorized.

Keep the October 30 review as a historical record. Add remediation results
rather than rewriting the old captured failures as if they never happened.
Do not claim universal classification accuracy, safe execution, or product
usefulness from these tests.

## Explicit deferrals and risks

A full claim-generation/evaluation system, revision-aware preview store,
background jobs and personalization remain future work. This brief must not
silently expand to cover them.

Specifically defer a cross-language reference resolver, adaptive second-round
crawl, raw-CDN retrieval/hash recovery, ETag caching, per-claim model retrieval,
SSE/cost endpoints, Redis quota/locks, a Postgres migration and universal runtime
execution. These require their own goals and measurements. Database choice is
not established by the research's observation that GitDiagram uses object storage.
Any future paid-generation budget must specify actual failure behavior and hard
bounds; a quota reservation estimate is not automatically a maximum spend limit.

Repository purpose can remain ambiguous even with complete source. Prefer a
small explainable policy and abstention over arbitrary ecosystem-specific
precedence rules. Preserve the public API while allowing honest internal
status. If a repair truly requires a public contract change, explain the
concrete incompatibility before changing that boundary.
