# Independent second-pass validation — November 1, 2026

**Verdict: request changes before accepting the repair plan as complete.**

Reviewed `fortune/classification-review-repairs` at
`cc564ad85fd9cfabb101729dd686ac2226863549`, comparing the changes since
`0ca94e4d4f9349a5df2ae1bf2cf42c15f64a5709` with the
[second-pass plan](../../../tasks/plan.md). The branch was already checked out.
No tracked application, test, plan, branch or commit was changed during this review.
Existing untracked documentation was preserved; only this review record was added.

## Confirmed progress

The implementation substantially addresses the first independent review. All
fifteen archived classification controls pass **unchanged**, and the archived
last-file cancellation control now returns `context.Canceled` as required.
The full tracked suite, race checks, vet, build and formatting checks also pass.

There is now a real component inventory, categorized dependency evidence, Go
call-identity analysis, field/span/listing/scope witnesses, validation before role
resolution, and six provenance-labelled repository snapshots. The corpus checks
assert kind and status per case and include mocked analysis-path degradation.
These improvements should be retained.

Nine further isolated classifier controls fail across the seven findings below.
They are concrete boundary cases within the existing plan, not a request for a
universal call graph or a new feature set. Selected counterexample counts do not
measure general classification accuracy.

## S1 — P1: a workspace declaration discards a supported root application

Locations: `internal/crawler/classify.go:208–219` and
`internal/crawler/resolve.go:293–301`.

`isWorkspace` treats any private npm package declaring workspaces as an aggregator.
`resolveRepository` then ignores its resolved root role. This conflates managing
workspace members with having no product of its own.

Reproduction: the root package has `private: true`, `workspaces: ["packages/*"]`,
runtime React, `index.html` and `src/App.tsx`. Its sole nested member is a library
with exports. The classifier correctly extracts the root frontend, then relegates
it to secondary and resolves the repository **library** from the nested member.

The npm documentation describes linked packages managed by a top-level root
package; it does not imply that package cannot itself contain an application.
See [npm workspaces](https://docs.npmjs.com/cli/v11/using-npm/workspaces/).

Repair: distinguish an aggregator with no supported root artifact from a workspace
root that declares its own artifact. Include valid root product evidence in the
resolution policy; do not discard it merely because it manages members. Preserve
the Excalidraw case where the root really is an aggregator. Pair root-app/member-
library, aggregator/member-app and genuinely mixed-product tests.

## S2 — P1: the no-server premise never proves the package listing is complete

Locations: `internal/crawler/classify.go:801–841,989–997` and
`internal/crawler/witness.go:246–275`.

`goNoServerPremise` checks all **known** command-package files were fully read,
but never checks the listing covers that package. Its scope witness rechecks only
the supplied paths, so it repeats the same incomplete basis.

Reproduction: a truncated tree contains `go.mod` and a fully read
`cmd/service/main.go` with an empty main. There is no positive CLI framework
declaration. The classifier resolves **CLI** and states “the command's package
was inspected in full and contains no server call.” An omitted sibling can hold
an `init` function that starts a server. A generic truncation caveat does not make
that package-completeness assertion true.

Repair: require listing coverage at the command directory's file depth before
using this negative premise. Make the witness verify the relevant package scope,
not merely that every path handed to it was read. Keep positive CLI-framework
and actual HTTP-call cases supported when they do not rely on this absence.

## S3 — P1: reading a command file is mistaken for inspecting its package clause

Location: `internal/crawler/classify.go:880–908`.

`goNoCommandPremise` accepts a `cmd/` file as inspected when `isRead` is true.
That includes clipped and malformed files whose package clause is unknown. It
does not require a known non-main clause for those files.

Reproduction: the root has a read importable `lib.go`. A listed
`cmd/service/entry.go` is either clipped at `package ` or contains `package ???`.
Both inputs resolve **library**, asserting “no package main at the conventional
command locations.” The classifier has not established that assertion and gives
no limitation naming the unknown command clause.

Repair: inspect the relevant package clauses before claiming no command. A clipped
file with an established non-main clause can still supply that positive fact;
an unread or unparseable clause must withhold the negative premise and name the
gap. This need not require reading every body or fetching every repository file.

## S4 — P1: test-only HTTP calls determine the product's backend role

Locations: `internal/crawler/classify.go:734–741,946–956`.

`goCommand` excludes `_test.go`, but `goServerObservation` scans all primary Go
files without excluding tests. Thus the main-package evidence can come from a
normal CLI while its supposed server evidence comes only from a test.

Reproduction: `cmd/tool/main.go` has an empty main, while
`cmd/tool/main_test.go` calls `http.ListenAndServe` inside `TestHTTP`. The result
is **backend**, with the test file as decisive HTTP evidence.

Repair: distinguish test observations from runtime product observations. Keep
tests eligible for contribution/quality evidence, but do not let a test-only call
establish the executable role. Pair this fixture with the same real call in the
runtime source so the supported backend still resolves.

## S5 — P2: Cargo implicit-target rules and depth coverage remain inaccurate

Locations: `internal/crawler/classify.go:1015–1036`,
`internal/crawler/classify.go:687–690`, `internal/crawler/witness.go:214–217`.

Two reproduced failures share this target model:

- `src/bin/tool/support.rs`, without `src/bin/tool/main.rs`, is treated as an
  implicit binary and produces **CLI**. An arbitrary nested module is not itself
  a binary entry.
- A depth-limited listing covering three segments shows `src/bin/tool` and
  `src/lib.rs` but can omit `src/bin/tool/main.rs` at depth four. The classifier
  nevertheless resolves **library**, asserting there is no binary target.

Cargo documents direct `src/bin/<name>.rs` entries and multi-file binaries whose
entry is `src/bin/<name>/main.rs`; other files there can be helper modules.
See [Cargo package layout](https://doc.rust-lang.org/cargo/guide/project-layout.html).

Repair: share the exact component-relative target rule between extraction and
witness validation, and require sufficient listing depth to exclude supported
multi-file entries. Test direct binaries, multi-file binaries, helper-only trees,
depth clipping and disabled autodiscovery.

## S6 — P2: module-path substrings still invent framework identity

Location: `internal/crawler/classify.go:969–981`.

`goRequirement` parses valid require entries, then matches them with
`strings.Contains`. Reproduction: a plain command requiring
`example.com/gin-gonic/gin-helper` resolves **backend**, citing that distinct
module as a server framework. The source field exists and its witness validates;
the inferred identity is still unsupported.

Repair: match documented canonical module identities with supported major-version
suffix handling. Unrecognized forks/lookalikes must not be silently relabelled
as the known framework. Pair lookalikes with actual Gin/Echo/Cobra module paths.

## S7 — P2: an empty npm bin map is treated as an installed command

Locations: `internal/crawler/classify.go:581–584,658–659`.

The raw JSON length check treats `"bin": {}` as executable metadata. Reproduction:
an exported library with that empty map resolves **CLI**, asserting it installs a
command. The generic field witness proves only that `bin` exists, not that it
declares any command.

The documented bin shape is a command-to-file map or a string for a single
executable. See [npm bin](https://docs.npmjs.com/cli/v11/configuring-npm/package-json/#bin).

Repair: interpret the supported field shapes and require at least one meaningful
executable entry. Share that interpretation between CLI extraction and the
library's no-bin premise. Keep string and nonempty-map positive fixtures; empty
maps must not establish a CLI, and malformed shapes need an explicit policy.

## Validation results and practical limits

| Independent check | Result |
|---|---|
| Fresh `go test -count=1 ./...` | Pass |
| Fresh `go test -race -count=1 ./...` | Pass |
| `go vet ./...` | Pass |
| `go build ./...` | Pass; rerun with module-cache access after sandbox cache warning |
| `git diff --check`; formatting in changed Go packages | Pass |
| Fifteen previous archived classifier controls, unchanged | 15 pass |
| Previous archived final-file cancellation control, unchanged | Pass |
| Additional second-pass classifier controls | 9 fail |

The six real snapshots are present with capture metadata and separate selected
versus extra files. The passing tests match the recorded per-case reference
judgments. Their complete-enough set resolves five and abstains on one; the
actual selection-only pipeline resolves three and abstains on three. This is
not a general accuracy estimate or proof of preview usefulness.

Excalidraw and PocketBase still abstain in the default pipeline because selection
misses decisive evidence; Face Recognition is outside the current taxonomy.
Those documented abstentions are not new false-verdict findings. PocketBase's
library reference is explicitly a structural taxonomy judgment with backend
product ambiguity, not a universally established label.

The implementation's [remediation record](../2026-11-01-classification-repair-second-pass.md)
also documents broader taxonomy limits and deferred revision/pipeline work. This
review does not require those to become a universal classifier. S1–S4 instead
prevent accepting the existing completeness and primary-role contract as closed.
S5–S7 are concrete supported-format corrections. Keep the current architecture
and fix these points with positive and negative regression pairs.

## Reproduction artifacts

- [Executed second-pass controls](reviewer_second_pass_test.go.txt)
- [Captured combined output](results.txt)
- [Previous controls and recipe](../2026-10-31-classification-repair-validation/README.md)

The controls ran in an isolated archive at
`/tmp/gittok-second-pass-review-dluokk2k`. To reproduce just the new failures,
extract the reviewed commit into a temporary directory, copy the `.go.txt` file
to `internal/crawler/reviewer_second_pass_test.go`, and run
`go test -count=1 -run TestSecondReviewBoundaries -v ./internal/crawler`.
No target repository code is executed and no network is needed once Go
dependencies are cached. The archived cancellation control needs loopback HTTP.

## Process documentation entry

The second pass generalized the first repair: component ownership, dependency
categories, source witnesses and real-repository fixtures now exist, and the
previous independent controls pass unchanged. Review then tested whether those
invariants held at adjacent boundaries: incomplete package listings, unreadable
package clauses, tests versus runtime files, workspace roots with their own apps,
and concrete manifest/target shapes. Nine further counterexamples failed.
The remaining issue is no longer a missing inventory or wholly absent validation;
it is that several resolution and premise rules still claim more than their
validated facts establish. Preserve both the successful second-pass results and
these failures in the process history. Complete focused repairs and rerun the
retained positives before recording acceptance.
