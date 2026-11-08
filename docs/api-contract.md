# GitTok API contract (v1)

The JSON the Go backend serves and the SwiftUI app consumes. The Go source of
truth is [`internal/apimodel`](../internal/apimodel); a complete, valid sample
is [`internal/apimodel/testdata/feed_example.json`](../internal/apimodel/testdata/feed_example.json),
kept in sync by a golden test. Code the app against that fixture.

## Design principle: presentation-agnostic

The app has several candidate layouts (full-screen reel, tap-through chapters,
triage card stack). They are all renderings of the **same data**, so this
contract describes the data, never a layout. No field encodes where something
is drawn, and the walkthrough is a **variable-length** list of sections, not a
fixed set of screens.

## The invariant: evidence + calibrated uncertainty

Every `Claim` names the `source` it came from, explains what that source shows,
and states the **limit** of what the evidence proves. Any artifact that could
imply the software ran (a video, a diagram, a generated walkthrough section)
carries an `evidence_mode` of `observed_running` or `not_executed`. The schema
makes it hard to assert more than the evidence supports.

## Endpoints

| Method | Path | Body / query | Returns |
|---|---|---|---|
| `GET` | `/v1/feed` | `?cursor=` (opaque, optional) | `Feed` |
| `GET` | `/v1/repos/{id}/preview` | — | `RepoPreview` |
| `POST` | `/v1/repos/{owner}/{name}/walkthrough` | empty JSON object | walkthrough job |
| `GET` | `/v1/walkthroughs/{id}` | — | walkthrough job |
| `GET` | `/v1/profile` | — | `Profile` |
| `PUT` | `/v1/profile` | `Profile` | `Profile` |
| `POST` | `/v1/events` | `Event` | `204 No Content` |

The feed, repository preview and optionally configured walkthrough endpoints are
implemented. Profile and events remain contracts for later tickets.

The production feed uses paginated public GitHub discovery and inexpensive
metadata previews; it does not analyze every repository on the page. See
[public discovery](public-discovery.md) for selection, caching and cursor behavior.
The optional job endpoints provide independently reviewed source narratives;
see the [walkthrough contract](visual-walkthrough-contract.md) and
[operations guide](walkthrough-operations.md) for states, configuration, recovery
and spending controls. Source previews remain available when generation is disabled.

### On-demand repository preview (G17)

`GET /v1/repos/owner%2Frepo/preview` (the generated `owner/name` ID as one
encoded segment) or, equivalently, `GET /v1/repos/owner/repo/preview`. An
analyzed preview uses bounded acquisition, commit-pinned file sources and
deterministic claims ranked by product relevance (at most 24 claims, 8
dependency claims, 2 lower-priority claims). A resolved `kind` is always led by
its `Why this is <kind>` explanation; otherwise `kind` is `other`. Stars,
license and last push are sourced `metrics`, not claims; a value GitHub did not
report is omitted, never zero. A final `decision` section titled "Coverage
notes" holds at most three short statements of what the inspection could not
cover. Generated sections are `not_executed`; omitted sections/fit/media have
not been established.

Every request reconfirms public visibility, stable repository ID and canonical
name with GitHub before answering. A completed inspection is reused for
`PREVIEW_CACHE_TTL` (default 30 minutes) as a captured snapshot: its file links
and metadata capture time are those of the original acquisition. Identical
concurrent requests share one acquisition. Responses carry
`Cache-Control: no-store`.

Credentials: public discovery, previews and walkthroughs use only
`GITHUB_PUBLIC_TOKEN` — a classic personal access token with no OAuth scopes,
verified at startup and on every response — or, when that is empty, anonymous
access. They cannot expose private repositories; the older trusted-token
command path is not exposed through these public handlers.

Errors retain `{ "error": "message" }`: 400 invalid ID; 404 inaccessible/private
or unknown visibility; 502 fatal upstream failure; 503 provider authentication,
permission or rate failure before usable metadata, a rejected public credential,
or exhausted capacity (with `Retry-After`); 504 acquisition deadline before
usable metadata. Failures after usable metadata may return a partial 200 preview
with coverage notes. Caller cancellation stops acquisition and response work.
The default 30-second acquisition deadline, physical API-attempt limits,
per-response and cumulative read limits, and file/tree limits are configured as
documented in [analysis limits](analysis-limits.md). They do not promise a strict
CPU-time limit or successful execution of the inspected software.

## Types

### Feed
- `items` — array of `RepoPreview`, in the order to show them.
- `next_cursor` — opaque string; omitted on the last page.

### RepoPreview
Top-level fields render the card face; `sections` carries the deeper walkthrough.
- `id`, `owner`, `name`, `slug` (`owner/name`)
- `kind` — `frontend | backend | library | cli | mobile | other`
- `language`, `headline`, `description`, `github_url`, `license`
- `media?` — `Media`
- `signals?` — array of `Signal` ("Why you're seeing this")
- `metrics?` — array of `Metric` (health/activity stat grid)
- `fit?` — `Fit`
- `sections?` — array of `Section`
- `presentation?` — `walkthrough` for the accepted narrative presentation; see
  the separate walkthrough contract for visual facts and ordered beats.

### Claim (the backbone)
- `label` — short category/headline
- `source` — `{ kind?: file | search | metadata | api, ref, lines?, url? }`
- `excerpt?` — the exact snippet the claim rests on
- `explanation` — what the excerpt shows
- `supports?` — what the evidence establishes
- `limit?` — what it does **not** establish (e.g. "does not prove runtime behavior")
- `confidence` — `confident | uncertain`
- `stance?` — fit framing: `match | stretch | caution | unknown`

`confidence` (trust in the evidence) and `stance` (bearing on the reader's fit)
are independent: a claim can be a confident caution or an uncertain match.

`source.url` is optional and additive. Generated file URLs use the inspected
commit ID, while `ref` remains the repository-relative path. `lines` is a
one-based inclusive range such as `12-18`; it and `excerpt` are omitted when the
parser cannot establish exact positions. Excerpts are exact consumed UTF-8 text,
never reserialized manifest values. Metadata URLs identify the provider resource;
`ref` names its field (for example `description` or `license.spdx_id`). Metadata
is mutable: generated claim limits record capture time separately from code revision.

`additional_sources?` — `[Source]`, optional and additive (omitted when empty).
A multi-premise claim cites its first premise in `source` and the rest here;
together they cover the claim's whole basis. Clients must render every
additional source: a client that shows only `source` hides part of the evidence.
Manifest declarations in `go.mod` and `package.json` cite their exact lines;
formats without a verified locator (TOML, YAML) cite the whole file.

The reserved claim `Why this is <kind>` leads an analyzed preview whose `kind`
is resolved. It is a classification from the inspected files, citing every
premise: consumed files (at their lines when known) and, for file-tree facts,
`{ kind: api, ref: <listed path or git/trees/<commit>>, url: <the recorded tree
request> }`. A bounded absence is stated within the files and tree it rests on,
never as a repository-wide absence. If any premise cannot be cited, `kind` is
`other` — an abstention, not a claim that the repository lacks a type.

### Section
- `kind` — `idea | how | stack | activity | setup | decision`
- `title`, `body?`
- `claims?` — evidence shown in the section
- `evidence_mode?` — `observed_running | not_executed` (e.g. a diagram inferred from code)

### Media
- `kind` — free label the app maps to a player (`narrated_video`, `muted_clip`, `demo`, …)
- `evidence_mode?`, `caption?`, `url?`

### Signal
- `value` — a repo attribute (e.g. `TypeScript`)
- `reason` — `you_know | you_want_to_learn | domain_match | popular`

### Metric
- `key`, `label`, `value` (a display string; `"Unclear"` is allowed)
- `source?`, `confidence?` — a metric can be uncertain like a claim

### Fit
- `summary?` — one-line human version
- `dimensions?` — array of `{ name, level (1–3), word }` scorecard rows

### Profile
- `identity` — `{ display_name?, github_login?, avatar_url? }`
- `known[]`, `learning[]`, `mix?` (`mostly_familiar | a_mix | mostly_new`)
- `domains[]`, `project_types[]`
- `experience?` (`never | small_prs | merged_changes | maintainer`)
- `goals[]`, `time_per_week?` (`under_2h | 2_to_5h | 5h_plus`)

### Event
- `repo_id`, `action` (`save | skip | inspect`)
- `reasons?[]` — optional "what decided it?" tags
- `at?` — RFC 3339 timestamp; server-set when omitted

## Versioning

The path is prefixed `/v1`. Additive fields (new optional fields, new enum
values the app can ignore) are not breaking. Removing or renaming a field, or
changing a type, means `/v2`.
