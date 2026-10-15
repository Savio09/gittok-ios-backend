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
| `internal/githubclient` | GitHub API: metadata, file tree, contents; auth, rate limits, errors |
| `internal/crawler` | Classify repo type, select informative files, enforce limits |
| `internal/preview` | Build evidence-linked claims + calibrated uncertainty |
| `internal/selection` | Choose & rank repos for the feed (curated now; personalized later) |
| `internal/store` | Persist built previews (and later, user profiles) |
| `internal/feed` | Serve the feed JSON to the app |

## Develop

```sh
go build ./...        # build everything
go run ./cmd/server   # run the backend
go test ./...         # run tests
```

Requires Go 1.25+. Set `GITHUB_TOKEN` in your environment (see `.env`).
