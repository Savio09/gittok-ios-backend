# G17: grounded preview claims

Status: implemented November 2, 2026. Planned November 1. Baseline: `main` at
`40b17f65f8bb4ee516196d1c716ea4856e05110b`.
Ticket: [SAV-23 / G17](https://linear.app/savios-workspace/issue/SAV-23/g17-extract-claims-link-each-to-source-filemetadata).
Historical execution order and acceptance checks:
[original plan](../tasks/g17-original-execution-plan.md).
Results and review corrections: [process record](reviews/2026-11-02-g17-claim-extraction.md).

This document records the deterministic G17 preview layer. Its references to
future budgets/synthesis describe the original ticket scope. The current
bounded acquisition and optional reviewed narrative layer are documented in
[analysis limits](analysis-limits.md) and [walkthrough operations](walkthrough-operations.md).

## Purpose and success

Turn selected repository evidence into useful preview claims. Every emitted claim
must cite a source actually acquired for that repository and support exactly what
the claim says. A plausible file path is insufficient. Ticket acceptance is
previews for at least two repositories where every claim cites a real source.
The stronger project requirement is that the same bounded pipeline accepts any
accessible GitHub repository, with useful partial output for unfamiliar layouts.
This does not promise full classification or architecture discovery everywhere.

The first implementation is deterministic: typed facts and fixed claim
templates, rather than generated prose that needs an unrestricted semantic judge.
Purpose comes from an explicitly attributed description or README excerpt.
Dependencies, scripts and source constructs supply implementation facts. No model
provider, model key or paid inference is required. A future synthesis layer would
have to preserve this evidence boundary; it is outside G17.

## Capability map and existing foundations

| ID | Responsibility | Existing foundation | Dependency direction |
|---|---|---|---|
| A | Acquire one bounded, commit-pinned inspection | `githubclient`, `cmd/evidencepack.analyze` | `analysis` → GitHub client, crawler |
| F | Extract and validate atomic facts | crawler parsers, component inventory, witnesses | crawler → inspection only |
| P | Build and validate preview claims | `apimodel.Claim`, `preview.FromMetadata` | preview → typed facts, metadata, API model |
| H | Serve an analyzed preview | documented preview route, existing server | HTTP → analysis service → preview |
| E | Demonstrate source integrity and generalization | maintained tests and six-repo corpus | tests → A/F/P/H |

Keep acquisition out of `preview`, and keep `preview` independent of `analysis`.
The command and HTTP handler consume the same analysis orchestration. Runtime
packages must not import `package main`. Retain `FromMetadata` for the inexpensive
feed; analysis is on demand through the preview route. Repository selection is a
separate concern: neither the analyzer nor the builder imports `selection.Curated`.

## A — source acquisition contract

The evidencepack workflow now lives in `internal/analysis` behind a narrow
injected interface using the existing client methods: `GetRepo`, `ResolveRef`,
`GetTree`, `GetFileContents`. Preserve command outputs and existing tests during
the extraction; apply pinning as a separate tested step.

After metadata acquisition, resolve the actual default branch once to its full
commit object ID. Validate the returned ID against the supported GitHub format;
reject empty or branch-like results. Do not silently substitute `main` for a
missing branch or call an unresolved branch immutable. Use the resolved ID for
all tree and content requests. The GitHub [commit endpoint](https://docs.github.com/en/rest/commits/commits#get-a-commit)
accepts refs; the existing `ResolveRef` method is the implementation starting point.
An empty repository can still produce a metadata preview without file citations.

Retain canonical owner/name, capture time, resolved commit, selected paths,
consumed contents, truncation flags and acquisition limitations in the internal
analysis result. Facts reference that result, never an unrelated checkout or a
later re-fetch. Distinguish directory listings from consumed files. Do not treat
an unread filename as proof of its contents.

For metadata, preserve field presence and the allowlisted provider value needed
by claims. The existing normalized `Repo` struct loses some presence/origin
information. A missing value must not become an asserted zero, empty license or
other fabricated fact. Do not retain/log full provider payloads or credentials.
Metadata represents what GitHub reported at capture time; it is not pinned to the
code commit. See the [repository endpoint](https://docs.github.com/en/rest/repos/repos#get-a-repository).

After metadata redirects, acquisition uses the canonical owner/name. Before
retaining resolved inspection evidence, the public service rechecks the stable
GitHub repository ID and public visibility. Failed revalidation discards tree,
revision, requested paths and contents, returning metadata with an explicit gap.
Identified repositories in the trusted command path also get identity checks;
trusted private access does not require public visibility. This additional bounded
metadata request guards against name reuse, including tree-only acquisition.
Links can still become unavailable after capture if a repository is renamed,
removed or made private.

The tree retains Git mode and object ID; canonical selection skips symlinks.
GitHub's [Contents API](https://docs.github.com/en/rest/repos/contents#get-repository-content)
can dereference symlinks, so target bytes must not masquerade as the symlink's
original file contents. Skipped/clipped selected files get sanitized limits
independently of whether they matter to the classifier.

Preserve existing selection, tree, content and retry limits. Local
acquisition deadline: 30 seconds, bounded by the caller's earlier deadline. This is
a tested policy default, not a measured latency guarantee. Existing content caps
bound retained bytes, not every response body or total transferred bytes. G19
owns complete configurable budget accounting and response-size enforcement.

## F — typed facts independent of role resolution

Expose a small fact collection/validation API from the crawler, reusing existing
parsers and witness checks. Preserve the `Classify` interface and repaired role
policy. Extract declarations even when they do not establish a product role;
do not scrape `Observation.Detail` strings or reimplement manifest parsing in
the preview package.

Each internal fact carries a typed kind/value, component, category/context and a
validated witness into the consumed inspection. Keep declared dependency name,
version and dependency class; distinguish development/test/peer/optional context.
Do not promote one workspace member's declaration into a repository-wide fact.

Initial fact families are deliberately bounded:

| Family | Permitted statement | Boundary |
|---|---|---|
| Description | “GitHub describes this repository as …” | Description is an author/provider statement, not verified behavior |
| README passage | “The README states …” with an exact short passage | Attribution does not certify setup or product behavior |
| Dependency | “`web/package.json` declares React in dependencies” | Declaration does not prove execution, usage or whole-repo architecture |
| Script/target | “This manifest declares a build script / binary target” | Declaration does not prove command success or a running product |
| Source construct | “This inspected non-test Go file calls …” | Syntax does not prove reachability or runtime behavior |
| Metadata | “GitHub reports …” for a present allowlisted field | Stars are not quality; `pushed_at` is not proof of a latest commit date |

Reuse supported ecosystem parsers; do not add a universal resolver. For unsupported
languages, available metadata and attributed documentation remain useful. No README,
malformed manifests, incomplete scans and inconclusive classification are ordinary
inputs. Return scoped limitations without making up claims to fill sections.

## P — public source and preview contract

Preserve the existing claim fields and source kinds in [the API contract](api-contract.md).
Additive change: optional `Source.URL` (`json:"url,omitempty"`).
`Ref` remains the file path or metadata field; `Lines` remains an optional `12-18`
range. A new URL lets the application open precisely the inspected revision without
changing the meaning of existing fields. Update contract examples and serialization
tests in the same slice; verify existing payloads still decode.

File URLs are generated internally from canonical identity, resolved commit and
escaped repository path, with a line fragment only when verified. GitHub documents
why branch URLs move and commit URLs are stable in [permanent file links](https://docs.github.com/en/repositories/working-with-files/using-files/getting-permanent-links-to-files).
Never trust a README-supplied URL as the provenance of inspected contents. Metadata
URLs point to the canonical provider resource and identify a field through `Ref`;
do not label metadata links immutable.

An emitted claim must pass all applicable checks:

1. Its source belongs to this acquired repository and inspection; file evidence
   belongs to the resolved commit. Metadata fields were actually present.
2. Its typed fact is revalidated against the corresponding parsed field or source
   construct. Merely finding matching text in some file is insufficient.
3. Any excerpt is an exact contiguous slice of consumed, representable UTF-8 bytes.
   Invalid UTF-8 excerpts are omitted because Go's [JSON encoder](https://pkg.go.dev/encoding/json@go1.25.5#Marshal)
   can replace invalid bytes. Any line
   range refers to those bytes and contains the cited construct; account for
   end-exclusive parser spans, multiline constructs and truncation.
4. Where parsers expose a value without reliable positions, omit optional excerpt
   and lines. Cite the pinned file and identify the parsed field in the explanation.
   Never present a reserialized value as an exact source quote.
5. Claim text comes from the approved template for its fact kind. A single witness
   cannot justify a multi-premise whole-repository conclusion. Emit atomic claims
   rather than flattening all classification observations into confident prose.

Build purpose/idea first when attributed purpose evidence exists, then appropriate
stack/how/activity/setup sections. Headline and section bodies must not add facts
beyond validated claims or explicitly attributed metadata. The analyzed `Kind`
comes from the crawler; inconclusive becomes `other`, not the legacy topic/language
heuristic. Preserve secondary-component context. Do not invent a purpose when absent.

Deduplicate and order deterministically by fact family, component, path and field
or span. Output caps: 24 claims, 1 KiB per excerpt. Choose exact excerpts
that fit; omit an overlong excerpt rather than alter its text or clip away the
construct that supports the claim. Prefer representative
purpose and component facts over a dependency dump. Record output clipping as a
limitation. Do not let input/API iteration order decide which facts survive.

Keep generated evidence mode `not_executed`; leave media, personal fit and stance
empty unless independently supported by a later capability. Do not copy illustrative
facts or `observed_running` flags from the fictional API golden sample. Supported
claims may be confident only within their explicitly stated scope. G18 owns the
broader supported/inferred/unknown policy; G17 still must disclose material gaps.

Represent gaps in an internal result and a plain-language limitations section.
Do not fabricate `SourceSearch` records: only use an uncertain sourced claim if
there is an actual recorded query/scope that supports it. A generic limitations
notice need not pretend to be a repository fact.

## H — application integration and failure behavior

Implement the existing documented `GET /v1/repos/{id}/preview` as a thin on-demand
adapter over the reusable analysis service. Existing generated IDs are `owner/name`;
encode that as one URL segment, e.g. `owner%2Frepo`. Go 1.25.5 ServeMux matches
escaped paths [segment by segment](https://pkg.go.dev/net/http@go1.25.5#ServeMux).
Test the real router and validate the decoded ID as exactly one owner/repository.
Accept no arbitrary host or filesystem path. The fictional bare `orbit` fixture
does not establish a real repository lookup scheme.

The feed uses a shared GitHub token and there is no caller authorization layer.
The public preview service creates an anonymous client, avoiding privileged reads
if an initially public repository becomes private. It serves only explicitly
public repositories; missing/ambiguous visibility fails closed. A token's ability
to fetch a private repository is not the caller's authorization to read it. Private
repository access through the core analyzer is possible only for a trusted caller;
public HTTP support requires a separate caller-scoped authorization design.

| Condition | Required behavior |
|---|---|
| Valid metadata and ordinary ref/tree/content failure | Return partial preview with sourced surviving facts and explicit limits |
| Unsupported language or inconclusive kind | Return available grounded facts; no whole-preview failure |
| No accessible repository, or private/unknown visibility at public endpoint | 404 without revealing private metadata |
| Malformed ID | 400 |
| Fatal upstream failure before usable metadata | 502; provider auth/rate failure 503; own deadline 504 |
| Parent context canceled | Stop further calls, return cancellation; no manufactured successful response |
| Own analysis deadline, parent still alive, metadata already available | Stop acquisition; return partial only from already retained and validated evidence |

Preserve the existing JSON error shape and sanitize upstream error details. The
partial/fatal distinction is based on whether usable evidence exists, never on
the presence of a non-nil error alone. Do not render provider authorization or
rate problems as a confirmed absence of repository files. Prevent private data
from escaping through partial results or logs.

Leave `GET /v1/feed` metadata-first: no analysis fanout on feed scrolling. No new
database, queue, persistent cache or personalization system is needed for this
ticket. The endpoint supplies an application consumer; it does not claim the
later full integration milestone is complete. Deployment throughput and global
request quotas remain separate from this bounded per-request implementation.

## E — evaluation and process evidence

Maintain the six existing corpus snapshots as classifier regressions. Their manual
extras and incomplete metadata provenance make them insufficient as the only G17
acceptance test. Do not overwrite historical gold-standard evidence packs.

Before tuning claim rules, choose and record at least two new public repositories
outside the study set, including one outside recognized manifest ecosystems. Run
the default production selector with no manual extra files. Save sanitized metadata
presence, commit ID, selected/consumed file evidence, limits and output in a new
G17 fixture/report location. Replay offline in ordinary tests; use an explicit
opt-in for future network recapture. At least two nonempty previews must pass a
claim-by-claim source audit, not just a self-reported validation flag.

Adversarial cases must include wrong-repository/commit/field citations, invented
quotes/lines, correct path with unsupported wording, missing/zero metadata,
unsupported layouts, nested products, test-only evidence, absent README, parse
failures, truncation, empty repositories, rate errors and cancellation. Include
owner/slug renaming and input-order permutation controls to detect fixture logic.
Repository documentation is untrusted data, including text addressed to an agent;
do not execute it or treat it as instructions.

Report source-validity and semantic-support results separately from useful claim
coverage, partial results and abstentions. Two examples demonstrate acceptance,
not universal accuracy. Record decisions, failed attempts, corrections and residual
limits in the G17 process report, linked to the existing repair history.
