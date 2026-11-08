# Source-grounded walkthroughs (SAV-52)

The quick source preview remains `GET /v1/repos/{owner}/{name}/preview`.
Walkthrough generation is an optional, separately configured background job.
It investigates one pinned public repository, writes an explanation of one
representative workflow, checks exact citations, then requests an independent
semantic review. Accepted statements alone form the story's prose. Page count
comes from accepted scenes, not a fixed template or a repository allowlist.

Source interpretation is always `inferred` / `not_executed`. This feature
does not run a target repository, create screenshots, or prove runtime behavior.
Azure observations are SAV-53 and require a separate isolated execution service.

## Local configuration

Keep `OPENAI_API_KEY` in the backend's local environment or ignored `.env` file.
Never put it in iOS, a prompt, a ticket, Git, or a screenshot. The Go server reads
environment variables; your local launcher must load `.env` into its environment.
A key alone does not enable generation: `WALKTHROUGH_ENABLED` defaults false.

Select a model and verify its current API token prices. The first accepted live
control on November 6, 2026 used `gpt-6.1-sol`, with `low` reasoning and standard
service tier. Its documented rates are $2 input / $10 output per million tokens,
with a $2.50 input cache-write rate. The example uses $2.50 for every input token
as a conservative ceiling; recorded local costs can exceed actual billing.
Source: [OpenAI model documentation](https://developers.openai.com/api/docs/models/gpt-6.1-sol).
Prices below use USD microdollars, so $1 is 1,000,000 micros. Requests explicitly
use `service_tier: default`; no hosted tools or regional endpoints are enabled.
`OPENAI_REASONING_EFFORT` is optional and omitted when empty. Choose an effort
supported by the selected model; GPT-6.1 Sol supports `low` but not `none`.

Example **non-secret** configuration, after selecting those prices:

```sh
OPENAI_MODEL=gpt-6.1-sol
OPENAI_REASONING_EFFORT=low
OPENAI_INPUT_MICROS_PER_MILLION=2500000
OPENAI_OUTPUT_MICROS_PER_MILLION=10000000
WALKTHROUGH_LEDGER_PATH=/absolute/private-directory/spend.json
WALKTHROUGH_JOURNAL_PATH=/absolute/private-directory/jobs.json
WALKTHROUGH_WINDOW_ID=capstone-demo
WALKTHROUGH_WINDOW_BUDGET_MICROS=2000000
WALKTHROUGH_MAX_JOB_MICROS=500000
WALKTHROUGH_MAX_REVISIONS=0
WALKTHROUGH_ENABLED=true
```

The example caps the entire operator-selected window at $2 and each generation
at $0.50; those are limits, not predicted charges. Requests reserve a conservative
upper bound before execution and settle against validated provider usage.
Unknown outcomes retain their maximum reservation. The configured prices must
match the selected model; this local ledger is not a substitute for provider
billing or a provider-level project spending limit.

Initialize accounting explicitly **once**, using the exact same path/window/limit:

```sh
go run ./cmd/walkthrough-ledger \
  -path /absolute/private-directory/spend.json \
  -window capstone-demo -budget-micros 2000000
```

The dedicated directory must be owned by the operator and mode 0700. Keep ledger
and journal on a persistent volume when hosting. Startup refuses absent, corrupt,
lost or mismatched accounting; it never assumes zero spent. Initialization refuses
existing accounting or its surviving control marker. Do not delete state or rename
windows to bypass a spending limit. Reconcile unknown charges against provider
usage before performing an intentional new accounting setup.

The private volume and its backups must be trusted. Restore ledger and marker
consistently and reconcile provider usage before resuming after restoration. A
local ledger cannot detect restoration of an older snapshot from the same ledger
or rollback of all control files. Legacy empty markers are refused; there is no
automatic migration that could erase prior accounting.

### Raising an existing authorized ceiling

After explicit owner authorization, change only the existing ceiling using the
operator command. Do not initialize a new window or delete historical entries:

```sh
go run ./cmd/walkthrough-ledger \
  -path /absolute/private-directory/spend.json \
  -window capstone-demo -increase-from-micros 2000000 \
  -budget-micros 50000000
```

The command checks the exact current ceiling, ownership, ledger digest and
strict increase before atomically persisting the change. Window identity,
charges, unsettled outcomes, replay protection and reservation limits remain.
Set `WALKTHROUGH_WINDOW_BUDGET_MICROS` to the new ceiling before restarting; a
stale configured ceiling is rejected. This command never runs automatically.

On November 7 the owner approved $50 total for the existing capstone window.
Live cold-generation checks used that ceiling and a $1/job limit, retaining two
prior unsettled outcomes. See [the core repair record](core-preview-repair.md).

## HTTP contract

```http
POST /v1/repos/{owner}/{name}/walkthrough
Content-Type: application/json

{}
```

Returns 202 for active jobs, or 200 for a reusable accepted story at the same
current public identity, pinned commit and policy version. Body:

```json
{"id":"opaque-32-character-id","state":"queued","poll_after_ms":1000}
```

Poll `GET /v1/walkthroughs/{id}`. States: `queued`, `reading`, `writing`,
`reviewing`, `ready`, `failed`, `interrupted`, `cancelled`, `unavailable`.
The response may include `stage` and a short `message`. A ready response includes
`preview` with the existing `RepoPreview` schema plus optional
`presentation: "walkthrough"`, `Section.id` and `Claim.id`. Narrative section
`body` contains accepted prose; its claims provide contextual citations and limits.
Unknown fields remain additive for existing clients. All responses are `no-store`.
No caller-provided model, prompt, URL, path, command or budget override is accepted.

Generation disabled: 503. Capacity exhausted: 503 with `Retry-After`. Invalid input:
400. Unknown/lost job or unavailable repository: 404. Generation/review failures
are terminal job states, not permission to fabricate a story. iOS retains the
source preview and shows a concise status when generation cannot finish.

## Bounds and recovery

One worker per process; the control files prevent concurrent processes using the
same state. Default job registry is 64 receipts with a 30-minute completion TTL.
One investigation owns a two-minute deadline, pinned commit and cumulative GitHub
quotas. Seed reads reserve half the files and bytes for targeted follow-up.
The initial model context prioritizes acquired code under separate 24 KiB/800
line per-file and 64 KiB/six-file total caps; docs/manifests get up to 40 lines.
These are ceilings: before any paid call, the exact final-writer request is
preflighted against configured pricing and budgets. If necessary, its projection
keeps a complete core code window where affordable, drops secondary code, and
clips remaining windows with truthful truncation flags. Acquired source and its
registry remain intact. If a minimal projection cannot fit, no model call occurs.
Targeted source tools retain their 8 KiB windows. These projections neither fetch
additional files nor renew investigation quotas.
Model calls have cumulative limits: 8 calls, 512 KiB serialized input, 12,000 output
tokens and the configured dollar cap. Review receives a reserved share of the
remaining generation budget. Exploration exposes `finish_reading`, followed by
a dedicated full-output `submit_story` turn. The submission tool is not exposed
on short exploratory turns. Requests have bounded bodies and are never
automatically replayed after an HTTP failure.

`WALKTHROUGH_MAX_REVISIONS` defaults to 1 and accepts only 0 or 1. A complete,
valid negative semantic review may trigger one constrained correction and a
fresh independent review when the remaining pair can be funded. Both distinct
call receipts are reserved before correction executes, including against the
global window. Transport, malformed output, mechanical citation and quota
failures do not trigger correction. Correction may replace wording, scene and
statement identifiers, and citations within the immutable already acquired source
registry. It cannot fetch new evidence or change repository identity. It must
pass the ordinary grounding/validation boundary and a fresh review that sees
neither prior feedback nor the rejected candidate. Only the accepted candidate
can be published. The final serialized review body is capped at 128 KiB and its
maximum cost reserved before correction starts. That conservative reservation
may not fit a small per-job dollar cap, especially with an expensive model; in
that case correction does not execute. The tested example explicitly sets 0 for
one writer/reviewer pass. Revision mode requires at least four total call slots.

Model citations select `file_id`, `start_line` and a schema-enforced `line_count`
of 1–10. The backend derives end lines and quotations from acquired bytes.
Models cannot supply an end line or fabricate a quotation at this boundary.

Disconnecting a polling client does not reset the job or its budgets. Identical
active generations coalesce by stable repository ID/canonical name/commit/policy.
Only a completed acquisition is reusable; a successful writer cannot upgrade
partial acquisition. Public visibility is rechecked before returning source prose.

Receipts are journaled locally; source and accepted prose remain in memory. On
restart, active jobs become `interrupted`, and previously ready bodies become
`unavailable`. They are never automatically regenerated. A missing ephemeral
journal makes old IDs unavailable; the independent persistent spending ledger
still retains their charges/reservations. This is deliberately a single-process
first slice, not distributed job infrastructure.

## Acceptance evidence

Tests and HTTP fixtures establish budgets, publication gates, recovery and client
behavior. They do not establish that a real model tells useful, true stories.
Live acceptance additionally requires authorized model runs on held-out repositories
of different types, checking every cited span and evaluating the complete purpose
and input → mechanism → outcome explanation. Until that evidence exists, SAV-52
must remain in progress. An automated independent reviewer is a further check,
not a semantic truth oracle.
