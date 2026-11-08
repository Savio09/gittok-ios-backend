# G17 external review: source accuracy versus product readiness

Date: November 4, 2026. Reviewed baseline: `82eb571` on the G17 branch.
The user supplied another agent's findings after running repository previews.
This investigation checks those findings and defines follow-up work. Application
behavior has not been changed in this investigation.

The [November 2 report](../2026-11-02-g17-claim-extraction.md) correctly records
30 supported claims in its two held-out cases. The user's reviewer independently
reports 34 accurate file-backed claims in its evaluation. That source bar does
not establish that the selected facts are useful or that the endpoint can support
a demo. Calling the feature complete without emphasizing those limitations was
too broad. Keep the original report as the record of what was actually checked.

## Independently reproduced failures

The archived Go tests use the existing captured CLI/PocketBase evidence and only
metadata actually present in those snapshots. They make no network requests and
do not add manually fetched files. Because the external review used different
inputs/metadata, counts need not be identical; the same failure mechanisms occur.

| Check | Observed baseline behavior | Required correction |
|---|---|---|
| Product relevance | CLI produces 9 helper declarations alongside 9 other declarations; Cobra is omitted | Prefer classification evidence and product component facts; subordinate tooling, tests, examples and indirect/dev dependencies |
| Walkthrough relevance | The only highlighted HTTP-call construct comes from `script/api-host-gateway/gateway/main.go` | Do not let helper constructs stand in for the product walkthrough |
| Declared positions | 20 selected PocketBase declaration claims omit line ranges, including UI build/dev scripts | Use available parser offsets; preserve whole-file fallback only where positions are genuinely unavailable |
| Kind explanation | The card reports CLI without a visible explanation tying that kind to its evidence | Reserve a supported explanation, preserving every necessary premise and citation |
| Displayed limitations | CLI's displayed diagnostics are 1,126 bytes and contain internal paths/jargon | Generate short user-facing coverage statements; retain detailed diagnostics internally |
| Repeat cost | Two identical preview calls make 4 metadata reads, 2 tree reads and 2 content batches, plus ref resolutions | Cache completed results and coalesce concurrent work |

All five reproduction tests fail as expected on the baseline. Their source is
preserved as `.go.txt` files so ordinary project tests remain runnable. See the
[recorded results](results.txt) and the
[repair execution plan](../../../tasks/g17-review-repairs.md).

## Root causes

`internal/preview/claims.go` interleaves fact-kind/component buckets without
product priority. Root and `.github` manifests receive equal turns, and ties use
lexical order. Dependency category and the classifier's own decisive evidence do
not affect ranking. The 24-claim cap therefore preserves alphabetical artifacts
instead of the most useful explanation.

`build.go` sets the kind from `crawler.Classify`, but `candidates` never consumes
that verdict's evidence. The badge and visible claims follow different selection
paths. A kind claim must preserve multi-source premises, rather than quoting an
arbitrary field and asserting that it alone proves the classification.

`sources.go` emits lines only for span witnesses. Dependency/script facts carry
field witnesses. The existing Go module parser exposes `Require.Syntax` with
positions; exact Go module locations are available. JSON script fields need a
location-preserving pass through the original JSON. This is a presentation gap,
not evidence that those declarations lacked sources.

`build.go` copies technical classifier limitations directly into section text.
`analysis.Service.Preview` always calls `Acquire`; it has no result cache.

## Quota and safe deployment direction

With the default 16 selected files, acquisition can make 20 requests before
retries: metadata, ref resolution, tree, 16 contents reads, and identity recheck.
Actual counts depend on selected files and failures. GitHub documents
[60 anonymous requests/hour per originating IP and normally 5,000 authenticated requests/hour per user](https://docs.github.com/en/rest/using-the-rest-api/rate-limits-for-the-rest-api).
Render documents [shared outbound ranges within a region](https://render.com/docs/outbound-ip-addresses).
Three cold previews/hour is consequently a plausible upper bound with an otherwise
unused anonymous quota, not a dependable demo capacity.

Caching alone saves repeat acquisition but does not fix cold-request capacity or
a shared-IP quota already consumed by others. The repair plan therefore combines
bounded reusable caching with an optional separate public-only credential. GitHub
documents [no-scope tokens as read-only public access and exposes OAuth scopes in response headers](https://docs.github.com/en/apps/oauth-apps/building-oauth-apps/scopes-for-oauth-apps).
The proposed first credential support is conservative: a classic PAT with no scopes,
verified on startup and on every successful response before decoding data. Missing
or privileged scope evidence fails closed. A fine-grained/app/user-token integration
needs its own permission model; the broad feed token is not silently reused.

Cache requirements include current public-visibility/stable-ID checks before
serving, a builder-version key, TTL/size limits, concurrent-request coalescing,
and cancellation when the last waiter leaves. Do not serve stale/private results
when visibility cannot be established. Disk persistence on Render must be stated
explicitly: an ephemeral service filesystem is not a durable cache across redeploys.

This investigation does not implement token handling, caching or private repository
authorization. Those proposed contract changes require implementation and security
verification before any demo-readiness claim.

A fresh-context adversarial review of the repair design also identified boundaries
that must be explicit: policy failures must propagate as terminal typed errors;
nil-error partial results must not become automatically cacheable; canonical names
must be checked after renames/transfers; listing/absence classification premises
need their own truthful source representation; HTTP caches must not bypass visibility
checks; simultaneous distinct work must be bounded; and every scope-header value
and credential-bearing redirect origin must be validated. Those findings were
incorporated into the plan. This is design reconciliation, not runtime verification.

## Remediation order and evidence required

Use the linked plan's ordered slices: establish failing usefulness controls, add
source locations, expose classification proof, repair ranking, simplify displayed
limits, add safe public credentials, add caching/coalescing, then replay/audit all
evaluation repositories and run full verification. The product-ranking rules must
depend on inspected evidence/layout, not repository names or a study whitelist.

Future validation must report two separate outcomes: source accuracy and useful
coverage. Include demo request counts for cold, warm and concurrent requests and
negative privacy/cache controls. Preserve this external review and the failed
baseline tests in the final repair process record rather than rewriting history.
