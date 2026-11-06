# GitTok — Backend

Go backend for **GitTok**, a native iOS app that presents public open-source
GitHub repositories as a vertical feed of short, **evidence-grounded previews**,
helping a GitHub-fluent developer decide whether a repo is worth investigating
further ("inspect") or not ("skip").

Minerva University capstone (CP193). Sprint 1 target: **Nov 7, 2026**.

## What the backend does

Given a GitHub repository, it gathers evidence (metadata, file tree, key files),
builds a structured preview where **every claim links to the source** it came
from — and marks claims whose evidence is insufficient or inferred — then serves
a personalized feed to the iOS app over HTTP/JSON.

## Layout

Each package owns one concern (separation of concerns):

| Package | Responsibility |
|---|---|
| `cmd/server` | Entry point; wires packages together and serves HTTP |
| `internal/config` | Load and validate runtime configuration from the environment |
| `internal/apimodel` | The JSON contract served to the app (feed, previews, claims, profile) |
| `internal/githubclient` | GitHub API: metadata, file tree, contents; auth, rate limits, errors |
| `internal/crawler` | Classify repo type, select informative files, enforce limits |
| `internal/preview` | Build evidence-linked claims + calibrated uncertainty |
| `internal/selection` | Choose & rank repos for the feed (curated now; personalized later) |
| `internal/store` | Persist built previews (and later, user profiles) |
| `internal/feed` | Serve the feed JSON to the app |

## API contract

The JSON served to the app is documented in [`docs/api-contract.md`](docs/api-contract.md),
defined in Go under `internal/apimodel`, with a canonical sample at
`internal/apimodel/testdata/feed_example.json`.

## Develop

```sh
go build ./...        # build everything
go test ./...         # run tests

cp .env.example .env  # then fill in GITHUB_TOKEN
export GITHUB_TOKEN=ghp_...   # or source your .env
go run ./cmd/server   # run the backend (defaults to :8080)

curl localhost:8080/healthz   # -> {"status":"ok"}
curl localhost:8080/v1/feed   # -> live feed of the curated repos (see API contract)
```

The feed currently serves live GitHub **metadata** for a curated set of repos
(`internal/selection`); the evidence layer (claims, fit, walkthrough) is filled
in by later tickets.

Requires Go 1.25+. Configuration is read from the environment (see
`.env.example`): `GITHUB_TOKEN` is **required** and the server refuses to
start without it; `PORT` is optional and defaults to `8080`.

Repository acquisition has configurable file, byte, API-attempt and time caps;
see [analysis limits](docs/analysis-limits.md) for settings and partial-result
behavior. The defaults inspect up to 16 files / 512 KiB in a 30-second budget.
