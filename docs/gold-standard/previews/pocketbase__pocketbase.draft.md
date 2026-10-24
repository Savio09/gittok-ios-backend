# Gold-standard preview — `pocketbase/pocketbase` (DRAFT)

> **DRAFT for Declan to revise.** Written by Claude from the evidence pack
> (`docs/gold-standard/evidence/pocketbase__pocketbase.md`), strictly against files in that
> pack. Not executed. Revise as you actually use the repo. Track A (public), backend.

## 1. Identity
- **Repo:** pocketbase/pocketbase · **Kind:** backend service / Go framework
- **Primary language:** Go · **License:** MIT (`LICENSE.md`)
- **URL:** https://github.com/pocketbase/pocketbase · default branch `master`
- Stars ~61k; last pushed 2026-10-23 (actively maintained).

## 2. What it does
An open-source realtime backend that ships as a single executable — database, auth, file
storage, and a REST/realtime API — usable standalone or embedded as a Go framework.
*Evidence:* description ("Open Source realtime backend in 1 file"); `README.md`; topics
`authentication, backend, golang, realtime`; `examples/base/main.go` (embed-as-framework
entry); `pocketbase.go` (library root). **Confidence:** confident. **Execution:** not_executed.

## 3. How it works
- **Single-binary, zero-CGO SQLite.** Uses pure-Go `modernc.org/sqlite`, which is what makes
  the "one file" / no-native-deps story work. *Evidence:* `go.mod` require block.
- **HTTP API layer** in `apis/` — record CRUD, auth (`record_auth_*`), realtime
  (`realtime.go`), batch, file, backup, health, and middlewares (CORS, gzip, body-limit,
  rate-limit). *Evidence:* `apis/` tree.
- **Domain core** in `core/` — app lifecycle, DB access (`db*.go`), collection/record models,
  typed fields (`field_*.go`), migrations. *Evidence:* `core/` tree.
- **Extensibility:** an embedded JavaScript VM (`github.com/dop251/goja`) and `plugins/jsvm`
  let users script the backend; `plugins/migratecmd`, `plugins/ghupdate`. *Evidence:* `go.mod`;
  `plugins/` tree.
- **CLI** built on Cobra (`cmd/serve.go`, `cmd/superuser.go`). *Evidence:* `go.mod`
  (`spf13/cobra`); `cmd/` tree.
- **Admin UI** is a Vite-built web app embedded into the binary. *Evidence:* `ui/package.json`,
  `ui/vite.config.js`, `ui/embed.go`. **Confidence:** confident it's embedded; **uncertain**
  on the exact UI framework (did not read `ui/src`).
- **Auth/crypto:** JWT (`golang-jwt/jwt/v5`), OAuth2 (`golang.org/x/oauth2`), validation
  (`ozzo-validation`). *Evidence:* `go.mod`.

## 4. How to run / setup
- **As a framework:** `go run ./examples/base` then open the served admin URL. *Evidence:*
  `examples/base/main.go`; `README.md`. **Not verified by execution.**
- **Build/dev:** a `Makefile` is present (build/lint targets); release via `.goreleaser.yaml`;
  lint via `golangci.yml`. *Evidence:* those files. Requires Go 1.27 (`go.mod`).
- **Setup burden (inference):** low to run the example; higher to rebuild the admin UI (Node
  toolchain in `ui/`). **Confidence:** uncertain.

## 5. Contribution entry points
- `CONTRIBUTING.md` and `.github/SECURITY.md` present. *Evidence:* those files.
- **Very strong test culture** — nearly every source file has a sibling `_test.go`
  (e.g. `apis/record_crud_test.go`, `core/db_test.go`). A newcomer has clear examples to
  follow and a safety net. *Evidence:* `apis/`, `core/` trees.
- ⚠️ **Openness to PRs is NOT determinable from the tree.** PocketBase is known to be
  selective about external PRs; whether that's stated here must be read from `CONTRIBUTING.md`
  (content not in the pack). **Tag uncertain** and verify before claiming it's contribution-friendly.

## 6. Tech facts for fit
Go 1.27 · pure-Go SQLite (`modernc.org/sqlite`) · Cobra CLI · goja JS scripting · JWT/OAuth2
auth · realtime subscriptions (`apis/realtime.go`, `tools/subscriptions`) · embedded Vite admin
UI · heavy table-driven/\_test.go testing · official JS & Dart SDKs (`README.md`).

## 7. Calibrated uncertainty
- **confident / not_executed:** Go single-binary realtime backend; SQLite via modernc; apis/core
  split; JS-VM extensibility; Cobra CLI; embedded admin UI; thorough tests; MIT.
- **uncertain / not_executed:** admin-UI framework; run smoothness; PR-acceptance policy
  (read `CONTRIBUTING.md`).
- **Not determinable from evidence:** maintainer responsiveness; realistic first-PR difficulty.

## 8. Evidence index
`README.md` · `go.mod` · `pocketbase.go` · `examples/base/main.go` · `apis/` (tree) · `core/`
(tree) · `plugins/` · `cmd/` · `ui/` (`package.json`, `vite.config.js`, `embed.go`) · `Makefile`
· `.goreleaser.yaml` · `golangci.yml` · `CONTRIBUTING.md` · `.github/SECURITY.md` · topics/metadata.
