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
| `GET` | `/v1/profile` | — | `Profile` |
| `PUT` | `/v1/profile` | `Profile` | `Profile` |
| `POST` | `/v1/events` | `Event` | `204 No Content` |

Endpoints are defined here but implemented in later tickets; this ticket fixes
the shapes below.

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

### Claim (the backbone)
- `label` — short category/headline
- `source` — `{ kind?: file | search | metadata | api, ref, lines? }`
- `excerpt?` — the exact snippet the claim rests on
- `explanation` — what the excerpt shows
- `supports?` — what the evidence establishes
- `limit?` — what it does **not** establish (e.g. "does not prove runtime behavior")
- `confidence` — `confident | uncertain`
- `stance?` — fit framing: `match | stretch | caution | unknown`

`confidence` (trust in the evidence) and `stance` (bearing on the reader's fit)
are independent: a claim can be a confident caution or an uncertain match.

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
