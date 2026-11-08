# Implementation plan: close the seven November 1 classification findings

Baseline: `cc564ad85fd9cfabb101729dd686ac2226863549` on
`fortune/classification-review-repairs`. Execution authorized November 1, 2026.
The [second-pass plan](classification-repair-plan-second-pass.md) and prior
reviews remain historical records. This plan governs the focused implementation.

## Outcome and scope

Close S1–S7 in the [independent review](../docs/reviews/2026-11-01-classification-repair-validation/README.md).
Keep the inspection model, component inventory, parsed observations, witness
validation, canonical selector and six-repository corpus. Preserve the public
API, metadata-only feed, acquisition budgets and deferred preview work.

Use planning, source-driven development, test-driven development, incremental
implementation, doubt-driven development, git workflow, documentation and code
review. Implement one tested behavior slice at a time. Add failing maintained
tests before each fix and retain positive counterparts. Commit focused changes
after affected suites pass; preserve pre-existing untracked documents.

Linear remains the designated tracker (G15/SAV-21, G16/SAV-22 and existing test
issues). This local plan supplies the execution checklist; no external issue or
Notion update is part of this implementation. No separate todo database is added.

## Verified sources and decisions

- [npm workspaces](https://docs.npmjs.com/cli/v11/using-npm/workspaces/):
  a root package can manage members and still declare its own application.
- [Go test packages, Go 1.25.5](https://pkg.go.dev/cmd/go@go1.25.5#hdr-Test_packages):
  test files participate in testing, not the ordinary application build.
- [Cargo package layout](https://doc.rust-lang.org/cargo/guide/project-layout.html):
  implicit binaries are `src/main.rs`, `src/bin/<name>.rs`, or
  `src/bin/<name>/main.rs`; nested helper modules are not executable entries.
- [Go major-version suffixes](https://go.dev/ref/mod#major-version-suffixes):
  recognize canonical module identities with supported major-version suffixes,
  rather than substring matching hosts or child modules.
- [npm bin](https://docs.npmjs.com/cli/v11/configuring-npm/package-json/#bin):
  interpret a string executable or a command-to-file map. Empty/malformed values
  must not manufacture a command.

Inspection coverage and role precedence are project policy, not framework facts.
Use bounded facts: listing completeness at the relevant directory, established
package clauses and supported product artifacts. Do not read every repository
file, execute target code or add a universal resolver.

## Ordered tasks

All tasks below were executed; the completion evidence is recorded at the end.

### 1. S1 — preserve workspace roots with their own artifact

Dependencies: none. Scope: small; resolution and role tests.

Separate workspace aggregation from the root's supported executable role. A
private workspace with its own frontend/backend/CLI/mobile artifact participates
in product resolution; an artifact-free aggregator still defers to members.
Keep genuinely ambiguous products inconclusive.

Acceptance: root app plus library resolves the app; aggregator plus member app
keeps the member; current workspace corpus and mixed-role cases remain supported.

Verification: fail the root-app regression before editing; run focused workspace
tests and the crawler suite. Review the root/aggregator policy before committing.

### 2. S2 — verify the whole command package behind no-server evidence

Dependencies: 1 for the sequential checkpoint; otherwise independent.
Scope: medium; Go premise extraction, witness validation and tests.

Require listing coverage at the package-file depth before using absence of HTTP
calls. A scope witness must enumerate the complete relevant non-test package
basis, not merely validate whichever paths the extractor supplied. Reject omitted
siblings and scopes without a package basis. Keep positive server/framework facts.

Acceptance: truncated package listing abstains; omitted sibling witnesses fail;
complete plain commands and positive CLI-framework cases retain results.
Limitations identify the relevant missing package scope.

Verification: fail classification and witness mutation controls first; focused
premise/witness tests, then crawler tests.

### 3. S3/S4 — inspect package clauses and exclude test-only role evidence

Dependencies: 2. Implement as separate tested substeps.
Scope: small per substep; Go premise/server extraction and regression tests.

For S3 require a known package clause where no-command inference depends on it.
A clipped or malformed unknown clause cannot establish an importable-only role.
Known non-main clauses can remain positive facts even when bodies are clipped.
For S4 keep test sources available for quality/contribution evidence but exclude
their server calls from runtime product-role inference.

Acceptance: clipped/malformed command clauses abstain with named limitations;
known non-main counterparts remain libraries; test-only HTTP calls leave a CLI
as CLI while the same call in runtime source still establishes backend.

Verification: red/green pairs per substep; crawler suite and analysis command tests.

### Checkpoint A — P1 contracts

All S1–S4 regressions pass with positive counterparts. Inspect actual primary,
secondary and limitation output. Validate the original fifteen controls remain
fixed. Get a bounded adversarial review of the changed policy/premises and
reconcile actionable findings before treating the checkpoint as complete.

### 4. S5 — share exact Cargo implicit-target semantics

Dependencies: checkpoint A. Scope: medium; Cargo helpers, detector, witness tests.

Use one component-relative binary-entry predicate for extraction and validation.
Bind implicit target witnesses to their owning crate. Account for four-segment
multi-file binary entries when asserting no binary exists; preserve autodiscovery
switches and explicit target declarations.

Acceptance: helper-only tree stays library; direct/multi-file entries stay CLI or
backend as supported; depth-three listing cannot rule out a multi-file binary;
disabled autodiscovery remains effective. Wrong-owner witnesses are rejected.

Verification: failing helper/depth/owner controls, positive layout pairs, then
crawler suite and corpus tests.

### 5. S6 — match canonical Go module identities

Dependencies: checkpoint A; execute after 4. Scope: small to medium; requirement
matching and tests.

Replace module-path substrings with canonical framework families, accounting for
Go major-version paths and supported gopkg.in spellings. Reuse matching between
the existing helpers rather than leaving divergent rules.

Acceptance: lookalike host/path/child modules do not establish framework identity;
actual Gin, Echo, Chi, Cobra and supported versioned CLI modules still match.
Preserve the full declared source path/version in observations.

Verification: red/green module-identity cases and Go/crawler tests.

### 6. S7 — interpret npm executable declarations by shape

Dependencies: checkpoint A; execute after 5. Scope: small to medium; package
metadata helper, role inference and tests.

Share bin interpretation between CLI extraction and no-bin library premises.
Recognize nonempty executable strings and nonempty command-to-file mappings.
An empty map declares no command. Malformed values cannot prove either a
command or its absence; record a material limitation.

Acceptance: exported library with empty bin stays library; valid string/map
entries retain CLI results; empty strings/maps and invalid values cannot generate
an installs-command observation. Malformed declarations do not silently justify
library-only inference.

Verification: shape and classification controls fail before implementation;
paired CLI/library cases, then crawler and command suites.

### Checkpoint B — retained behavior and source support

All nine archived November 1 classifier controls pass unchanged. All previous
fifteen controls and the final-file cancellation control still pass. Inspect
witnesses for the repaired scope/target decisions; arbitrary matching paths must
not establish the wrong fact. No repository-specific allowlists or blanket
abstention shortcuts.

### 7. Final validation, review and process record

Dependencies: 1–6. Scope: tests/documentation; no new product behavior.

Run after the final code change:

```sh
go test -count=1 ./...
go test -race -count=1 ./...
go vet ./...
go build ./...
git diff --check
```

Check formatting for changed Go files. Re-run affected checks only after new code
changes or a newly identified concern. Use loopback permissions where needed;
do not label blocked checks passed.

Inspect corpus kind/status outcomes and supported-positive retention. Document
default-pipeline abstentions separately from incorrect resolved verdicts.
Preserve the captured snapshots and reference judgments unless a demonstrated
error justifies a documented correction.

Complete an adversarial code review of the patch and address concrete findings.
Record S1–S7 → change → maintained test, actual command results, internal policy
changes and residual bounds in a new remediation/process document. Keep all old
review findings and plans. No deployment, remote push, external publication or
ticket acceptance is implied.

## Risks and controls

| Risk | Control |
|---|---|
| Workspace app fix hides a genuinely mixed repository | Test aggregator, root product and competing executable roles |
| Stronger absence rules make supported positives disappear | Pair each negative with positive artifact evidence and run corpus |
| Extraction and validation disagree | Shared scope/target/metadata interpretation plus mutated-witness tests |
| New metadata rules silently expand supported syntax | Document bounded shapes and preserve unknown states |
| Broad refactor obscures these repairs | Focused test/fix slices; no unrelated interface or infrastructure work |

## Execution record

Completed November 1, 2026. Final code: `992e67704ec2b5de0653818fdef4358af1d463f6`.

| Task | Result | Commit/evidence |
|---|---|---|
| 1 / S1 | Complete; root artifacts retained, competing products abstain | `3b7e51f`; workspace product tests |
| 2 / S2 | Complete; coverage, sibling and command-package witnesses checked | `3b01193`; Go scope tests |
| 3 / S3–S4 | Complete; known clauses required, test-only server calls excluded | `3b01193`, `f3cd3a3`; package-clause and test-role pairs |
| Checkpoint A | Pass | Crawler and analysis-command suites; fresh policy/scope review |
| 4 / S5 | Complete; shared Cargo layout, ownership and discovery bounds | `1b63bb8`; Cargo layout/witness cases |
| 5 / S6 | Complete; canonical module families and original version values | `bd3b0f6`, `992e677`; module identity/provenance cases |
| 6 / S7 | Complete; typed executable entries and honest unknown states | `8b1fc79`; npm bin shape pairs |
| Checkpoint B | Pass | All 24 archived classifier controls and cancellation control pass unchanged |
| 7 | Complete | Full tests, race checks, vet, build and whitespace/format checks pass; fresh reviews reconciled |

See the [implementation/process record](../docs/reviews/2026-11-01-classification-repair-third-pass.md)
and [captured verification](../docs/reviews/2026-11-01-classification-repair-implementation/verification.txt).
Only the seven reviewed fixes and their directly related provenance correction
are marked complete. Broader taxonomy, default selection abstentions and preview
usefulness remain separate evaluations; no external ticket was marked accepted.
