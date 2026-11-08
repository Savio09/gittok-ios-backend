# G17 implementation and process record

Implemented November 2, 2026 on
`fortune/sav-23-g17-extract-claims-link-each-to-source-filemetadata`.
Baseline: `40b17f65f8bb4ee516196d1c716ea4856e05110b`, with the classification
repairs already merged into main. Ticket:
[SAV-23 / G17](https://linear.app/savios-workspace/issue/SAV-23/g17-extract-claims-link-each-to-source-filemetadata).
The [execution plan](../../tasks/plan.md) and [design](../g17-claim-extraction.md)
describe the contracts. This record extends the
[previous repair process](2026-11-01-classification-repair-third-pass.md);
earlier reviews and unsuccessful iterations remain intact.

## Result

The application now has an on-demand evidence-backed preview path:

`GET /v1/repos/owner%2Frepo/preview`

The reusable analysis service fetches metadata, resolves a commit, selects bounded
files, retains inspection provenance and builds deterministic claims. The
evidence-pack command shares acquisition. The feed still performs metadata-only
fetching; it does not fan out into full analysis on scroll.

The builder consumes typed facts from existing parsers, with component ownership,
dependency category/version and optional context. It attributes descriptions and
README passages, emits exact representable quotes where positions are known, and
does not imply target execution, installed dependencies or personal fit. Unknown
ecosystems can yield metadata/documentation claims without a resolved product kind.
There are no repository-specific rules or new model/provider dependencies.

`Source.URL` is an optional additive API field. File URLs pin the inspected commit;
parsed declarations without reliable offsets omit excerpts/line ranges. Metadata
links identify a mutable provider resource and field, with capture time in claim
limits. Existing payloads still decode without the new field.

## Review findings that changed the implementation

Planning, source-driven implementation, test-driven development, incremental
delivery, interface design, security, doubt-driven review and code-quality review
were applied. Fresh-context reviewers challenged the contracts and implementation;
they were instructed to find failures rather than confirm the author's reasoning.
The user explicitly chose to continue with the current reviewer when offered a
cross-model opinion. No external model CLI was invoked.

| Finding | Correction and retained evidence |
|---|---|
| Public metadata followed by privileged reads can leak newly private contents | Public service constructs an anonymous GitHub client; the server token stays in the feed. Tests simulate visibility changing before ref resolution and reject trusted-token services at the public handler. |
| Contents API may dereference symlinks | Retain tree mode/object ID and exclude mode `120000` in canonical selection. Add a symlinked README/manifest selection regression. |
| JSON encoding can replace invalid UTF-8 and corrupt an exact quote | Omit unrepresentable excerpts and test exact text after JSON round-trip, including clipped multibyte input. |
| A truncated Go call can have invalid recovered AST offsets | Reproduced a real slice-bounds panic with an incomplete `http.ListenAndServe(` call; validate scanner/consumer bounds before quoting. Keep the malformed-source regression. |
| Dart versions and Cargo optional declarations lost context | Preserve supported pubspec version values and Cargo optional flags; validate the resulting facts against rederived source facts. |
| Per-witness parsing can make dense manifests excessively expensive | Cache parsed manifests and Go call spans per inspection. Runtime builds consume validated collections directly; standalone audit functions deliberately rederive evidence. |
| A real path can accompany unsupported wording | Claims come from typed fact templates; validation compares full supported wording/provenance. Mutation tests retain the path while forging runtime assertions, fields, URLs or quotes. |
| Per-file provider errors can leak through classifier limitations | Reproduced the leak in a failing test; replace provider diagnostic strings before constructing the shared inspection. Public fatal errors also retain a generic error shape. |
| Failed README reads or clipped non-role files can disappear from limitations | Add acquisition-wide skipped/clipped notices independently of classification. A README-failure/LICENSE-clipping regression preserves the gap. |
| Duplicate requirements can exhaust the cap before deduplication | Reproduced starvation of distinct declarations; deduplicate before component/family rounds and the 24-claim cap. |
| A redirected old slug can be reused for another repository | Switch subsequent requests to canonical identity; recheck stable repository ID before retaining resolved evidence. Failed checks discard the inspection and return metadata-only output. |
| Identity checks covering files alone miss tree-only evidence | Extend revalidation to every resolved inspection, including zero-content trees; retain a slug-reuse regression asserting paths/revision are discarded. The final focused review found no remaining defect in this resolution. |
| A hard-wrapped README line produces an incomplete description | Keep the entire short contiguous paragraph within the excerpt cap; Rack's source quote now covers lines 3–6. |

The initial proposal's visibility gate alone was insufficient; anonymous acquisition
is the substantive security change. ID revalidation also prevents two public
repositories from being mixed under one identity during rename/reuse. These checks
add a bounded metadata request, not an unbounded crawl or a new authorization flow.

Official documentation used for these boundaries:

- GitHub's [repository metadata endpoint](https://docs.github.com/en/rest/repos/repos#get-a-repository)
  and [commit endpoint](https://docs.github.com/en/rest/commits/commits#get-a-commit).
- GitHub's [permanent file links](https://docs.github.com/en/repositories/working-with-files/using-files/getting-permanent-links-to-files)
  and [Contents API symlink behavior](https://docs.github.com/en/rest/repos/contents#get-repository-content).
- Go 1.25.5 [ServeMux escaped-segment matching](https://pkg.go.dev/net/http@go1.25.5#ServeMux)
  and [JSON string encoding](https://pkg.go.dev/encoding/json@go1.25.5#Marshal).

## Held-out acceptance and independent source audit

Glow and Rack were selected before examining their output, outside the six-repo
study set. Rack is outside the supported manifest ecosystems. Live acquisition
used anonymous GitHub access and the production selector with no manual extras.
Retained [snapshots and previews](../../internal/analysis/testdata/g17/README.md)
are replayed by ordinary offline tests; recapture is explicitly opt-in.

| Repository | Pinned commit | Selected / consumed | Claims | Result |
|---|---|---|---|---|
| charmbracelet/glow | `8b0600fa445535b9c53194c0d4f7fe3cf700ef3e` | 16 / 16 | 24 | CLI; sourced descriptions, metadata and Go declarations |
| rack/rack | `da90464e3fa93e3cb49dc2d4ede798e6c6b70b4f` | 14 / 14 | 6 | Other; sourced descriptions/metadata and explicit classifier gap |

A separate reviewer checked every emitted claim against the actual retained fields
and consumed bytes without using the production validator as proof:

- **30/30 claims supported:** 11 metadata, 2 README, 17 Go dependency declarations.
- **19/19 file citations** identify consumed, untruncated files at the recorded
  repository/commit/path. Both README excerpts and line ranges match exactly.
- All 17 dependency names and versions match `go.mod`; 8 direct and 9 indirect
  declarations retain the correct category, including pseudo-versions.
- All sections use `not_executed`; no fabricated runtime, setup-success or fit claims.

The live snapshots were captured at 13:10 UTC before final repository-ID hardening,
so they lack the subsequently added ID field. Their raw evidence was preserved;
the final builder was replayed offline. Final canonical-ID/visibility behavior and
HTTP routing were verified with deterministic upstream integration tests. No IDs
were backfilled or invented. Future live recapture uses the final guarded path.

Coverage is reported separately from validity. Glow shows 17 of 67 dependency
declarations and discloses the claim cap; useful omitted declarations include
Cobra and Viper. Rack's Gemfile was consumed but is not parsed into dependency
claims; its gemspec was not selected. Rack's `other` result is abstention, not a
statement that it lacks library behavior. GitHub reported Rack's license as
`NOASSERTION`, so no license identification was fabricated. These two examples
meet ticket acceptance; they do not establish universal classification accuracy.

## Verification

All final code gates passed after the last behavioral change:

```text
go test -count=1 ./...           PASS
go test -race -count=1 ./...     PASS
go vet ./...                    PASS
go build ./...                  PASS
git diff --check                PASS
```

Focused verification additionally covered commit pinning, canonical redirects,
private/unknown visibility, public-to-private changes, repository-ID reuse with
files and with tree-only acquisition, malformed IDs, sanitized errors, parent
cancellation, own deadlines, retained partial facts, UTF-8 serialization, exact
multiline ranges, escaped paths, duplicate starvation, component fairness and
input-order/owner-renaming controls. The six existing crawler/corpus regressions
remain in the passing suite. Changed Go files are formatted.

One diagnostic benchmark on this Apple M4 Pro collected 2,000 declarations from a
manifest below the existing 64-KiB file cap: approximately 3.88 ms/op over three
iterations. This verifies the corrected path is practical for that case, not a
production latency or memory guarantee.

The first local attempts encountered sandbox restrictions on Go's normal cache,
loopback test servers and GitHub DNS. Cache-safe local commands or explicitly
approved test/network execution resolved them. A sandboxed build's metadata-cache
warning was followed by a clean final build with the required permissions. These
environment failures were not counted as passing checks. Git history is split
into focused local commits; no merge, deployment or external ticket update occurred.

## Residual limits and next tickets

The public preview route uses GitHub's lower anonymous quota to avoid privileged
private reads. Production quota management/caching and caller-scoped private
authorization remain future work; the token-bearing command remains a trusted
consumer. A failed identity check, including after the acquisition deadline, can
withhold previously read files and yield metadata-only output.

G18 should develop calibrated supported/inferred/unknown presentation beyond the
current confidence/limits fields. G19 should enforce configurable API/transfer/body
and CPU budgets: the current file/content/tree caps bound retained inputs, and the
30-second acquisition deadline does not bound every response allocation or pure
analysis CPU operation. More ecosystem parsers, architecture synthesis, rich setup
validation, executed artifacts and personalization were not silently added to G17.
Linear remains the designated tracker; its status was not changed automatically.
