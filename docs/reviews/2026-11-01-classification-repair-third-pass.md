# Classification repair implementation — November 1, 2026

Status: implemented and verified for S1–S7 in the
[independent second-pass validation](2026-11-01-classification-repair-validation/README.md).
Started at `cc564ad85fd9cfabb101729dd686ac2226863549`; final code commit is
`992e67704ec2b5de0653818fdef4358af1d463f6` on
`fortune/classification-review-repairs`. The user explicitly authorized both
planning and implementation, extending the earlier review-only assignment.

The [execution plan](../../tasks/plan.md) records scope, dependencies and checks.
The [previous plan](../../tasks/classification-repair-plan-second-pass.md), prior
reviews, original failures and captured corpus remain unchanged historical inputs.

## Finding → implementation → maintained evidence

| Finding | Commit | Change | Tests and supported counterparts |
|---|---|---|---|
| S1: workspace discarded its own root product | `3b7e51f` | Workspace roots participate in member comparison when they have a supported role; empty aggregators still defer | `TestWorkspaceRootParticipatesInProductResolution`: root frontend/backend/CLI, aggregator/member app, competing executable roles |
| S2: no-server scope did not prove complete package coverage | `3b01193` | Shared `validateGoPackageListing` checks directory coverage, all known siblings and package-main identity; extraction and witness validation both use it | `TestNoServerPremiseRequiresPackageListingCoverage`, `TestNoServerWitnessCannotOmitPackageSiblings`, `TestNoServerWitnessNeedsACommandPackage`; complete/depth-covered commands remain positive |
| S3: read bytes substituted for known package clauses | `3b01193` | Conventional command paths require an established package clause before asserting no command | `TestLibraryNeedsKnownConventionalCommandClauses`: clipped/malformed unknown clauses abstain, known non-main clauses remain usable even in clipped files |
| S4: test-only HTTP calls established runtime product role | `f3cd3a3` | Server observation extraction excludes `_test.go`; source selection remains eligible to retain tests for other analysis | `TestProductRoleExcludesTestOnlyServerCalls`: same-package and external test packages versus a real runtime server call |
| S5: inaccurate Cargo target/discovery bounds | `1b63bb8` | Shared exact implicit-bin layout predicate; target witnesses anchor to their crate; absence covers multi-file entry depth; virtual roots and unknown clipped discovery settings do not create targets | `TestCargoBinaryEntriesAndListingBounds`, `TestImplicitCargoWitnessNeedsItsOwningCrate`, `TestClippedCargoCannotInventAutodiscoveryDefaults`; direct/multi-file/disabled-discovery positives |
| S6: module substrings manufactured framework identity | `bd3b0f6` | Match canonical framework families via `module.SplitPathVersion`, including explicitly supported gopkg.in aliases; retain declared path | `TestFrameworkRequirementsUseCanonicalModuleIdentities`: real versioned Gin/Echo/Chi/Fiber/Cobra/CLI modules versus wrong host, suffix lookalike and child module |
| S7: empty or malformed npm bin installed a command | `8b1fc79` | Shared typed bin interpretation for CLI and library inference; executable field/value witnesses; malformed metadata blocks unsupported absence | `TestNPMBinNeedsMeaningfulExecutableEntries`: empty/null/string/map, invalid shapes, named limitations and accurate no-command wording |
| Review follow-up: normalized Go versions were presented as declared values | `992e677` | One source-preserving module parser used by inventory, extraction and field validation; invalid semantic versions remain rejected | Noncanonical and build-metadata cases in module-identity tests; `TestMalformedModuleVersionsCannotEstablishAFramework` |

Regression tests were observed failing before the corresponding fixes. Supported
counterparts accompany them; the implementation does not pass by returning
inconclusive for every repository. Existing tests were preserved; one valid
no-server witness fixture was corrected to use a complete safe package rather
than omit its known siblings. The old module helper assertion now expects a full
canonical identity instead of the previous partial substring.

## Verification

[Captured verification output](2026-11-01-classification-repair-implementation/verification.txt)
contains the standard commands and unchanged archived-control replay.

| Check | Result |
|---|---|
| `go test -count=1 ./...` | Pass |
| `go test -race -count=1 ./...` | Pass |
| `go vet ./...` | Pass |
| `go build ./...` | Pass |
| `git diff --check`; changed Go-file formatting | Pass |
| Original eleven maintained review regressions | Pass within the full suite |
| Fifteen earlier archived classifier controls, unchanged | 15 pass |
| Nine November 1 archived classifier controls, unchanged | 9 pass |
| Archived final-file cancellation control, unchanged | Pass |

Archived controls ran against an isolated `git archive` of the final code. No
target repository code, model call, deployment or production feed crawl was run.
Mock HTTP tests ran on loopback with environment permission. The full suite's
offline corpus and mocked pipeline checks retain their per-fixture kind/status
expectations: five resolved/one inconclusive for complete-enough snapshots, three
resolved/three inconclusive for the default selection pipeline. These are matches
to documented reference judgments, not general accuracy estimates.

## Decisions and official sources

- A workspace root can use its member packages while containing its own program;
  workspace membership alone is not proof of an empty aggregator. The project
  policy compares supported root/member products and abstains on incompatible
  executable roles. [npm workspaces](https://docs.npmjs.com/cli/v11/using-npm/workspaces/).
- Ordinary Go builds omit `_test.go`; test-only HTTP syntax cannot establish
  runtime product role. [Go 1.25.5 build documentation](https://pkg.go.dev/cmd/go@go1.25.5#hdr-Compile_packages_and_dependencies).
- Cargo's documented entry layouts distinguish a multi-file binary's `main.rs`
  from helpers and require sufficient listing depth to exclude it. Virtual
  manifests have no root package. [Package layout](https://doc.rust-lang.org/cargo/guide/project-layout.html),
  [virtual workspaces](https://doc.rust-lang.org/cargo/reference/workspaces.html#virtual-workspace).
- Framework matching uses whole module families and documented major suffixes.
  The pinned `golang.org/x/mod v0.37.0` source was read to confirm
  `SplitPathVersion`, `VersionFixer` and the default version canonicalization.
  The parser adapter keeps supported source spellings without accepting invalid
  versions. [Go module paths](https://go.dev/ref/mod#major-version-suffixes),
  [module parser](https://pkg.go.dev/golang.org/x/mod/modfile#Parse).
- npm executable metadata is interpreted as an executable string or a
  command-to-file mapping. Empty declarations establish no command; unsupported
  shapes establish uncertainty. [npm bin](https://docs.npmjs.com/cli/v11/configuring-npm/package-json/#bin).

No new dependencies or public HTTP/JSON fields were added. Changes are confined
to internal classification/evidence logic, maintained tests and documentation.

## Adversarial review and process record

The implementation followed planning, official-source verification, failing-test
reproduction, incremental implementation, git save points, documentation and code
review skills. Behavioral disproof attempts used TDD. Fresh-context reviewers
also challenged workspace policy, Go package scopes, Cargo targets and manifest
interpretation before those decisions stood.

The Go-scope reviewer found that a complete set of non-main or mixed-package
sources could still masquerade as a command-package witness. This was actionable:
package-main identity checks and rejection cases were added before the commit.
The manifest reviewer found the existing parser silently normalized declared
versions. This was actionable under the provenance contract and led to the shared
source-preserving parser plus raw-version tests. Subsequent reviews found no
remaining concrete contract violations in those changes. An unconfirmed Cargo
filename corner case was not treated as a demonstrated defect. The bounded
review stopped after the substantive issues were resolved.

An optional external cross-model opinion was offered, as required by the
doubt-driven skill. No external CLI or external model service was invoked;
verification consists of tests and internal fresh-context reviews. The skill's
referenced shared definition-of-done file was unavailable locally; the explicit
project verification commands and acceptance criteria above were used.

The earlier agent's inventory, parsers and corpus were retained. This pass fixes
the places where a valid source fact was overextended into a product role or an
absence claim. The history now includes the original review, both repair passes,
the independent counterexamples and these verified corrections, suitable for
process documentation without erasing unsuccessful iterations.

## Remaining bounds

These seven findings are closed against their stated controls and paired
positives. Broad repository taxonomy, arbitrary Go call graphs, all Cargo build
validation and unsupported dynamic metadata remain outside the bounded analyzer.
Default selection still abstains for Excalidraw and PocketBase; Face Recognition
remains outside the implemented taxonomy. The feed still uses its metadata
heuristic, and the analysis path still labels branch revisions unresolved.

No issue statuses, Notion pages, deployment state or public API were changed.
This record does not declare the full preview pipeline, G17–G19, runtime safety
or the capstone usefulness evaluation complete.
