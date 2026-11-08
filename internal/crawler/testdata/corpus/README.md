# Classification acceptance corpus

Sanitized, provenance-labelled snapshots of the six repositories in GitTok's
live-feed and study sets, used by the offline classifier and pipeline tests.
Tests read these files only; they never touch the network or need credentials.

## How the snapshots were captured

`go run ./cmd/corpuscapture -out internal/crawler/testdata/corpus <owner/repo> …`
with a read-only GitHub token. For each repository the command:

1. read the metadata and resolved the default branch to the commit SHA it
   pointed at — every later read used that SHA, so each snapshot is one
   immutable revision rather than a moving branch;
2. listed the tree at that SHA under `DefaultTreeLimits` (5000 entries, depth 12);
3. ran the canonical `crawler.SelectFiles` with `DefaultSelectionBudget` and
   fetched exactly that selection under `DefaultContentLimits` — what the
   analysis path itself reads (`selected`);
4. fetched any **extras** listed in `extras.json`, each with a written reason —
   decisive files the selection did not reach, needed for the reference
   judgment. Extras are recorded apart from the selection, never merged into it;
5. withheld anything that looks like a credential (none was found).

| Repository | Ref | Resolved commit | Captured (UTC) | Entries | Selected | Extras | Note |
|---|---|---|---|---|---|---|---|
| Savio09/face-recognition | main | `90750d921bed3768bd59328109e90f66b593eb86` | 2026-11-01 | 3 | 2 | 0 | study set |
| Savio09/plantly | main | `81c15a40289784af8765f5b7485999d2aff8757d` | 2026-11-01 | 40 | 11 | 0 | study set |
| cli/cli | trunk | `9490d204a2ba95434153dedce99898ce9f281426` | 2026-11-01 | 2026 | 16 | 0 | feed set |
| excalidraw/excalidraw | master | `8bcdca4526867433f99fdfcb55a063668db0c9dc` | 2026-11-01 | 1471 | 16 | 9 | feed + study |
| pocketbase/pocketbase | master | `7f2773ec234b62d0b80ca47c508397cb3d0da3ea` | 2026-11-01 | 1107 | 14 | 2 | feed + study; archived upstream |
| starship/starship | main | `5e9e2c8c5f6dcb9c148c12dee1afd1166530b0e6` | 2026-11-01 | 1028 | 16 | 0 | feed set |

All six listings are complete (not truncated, not depth-limited). Every
requested file was returned in full: none exceeded the 64 KiB per-file cap and
none was skipped. Files keep their original paths and case.

`archived-2026-10-30-packs.json` is a verbatim copy of the inputs the October 30
review extracted from the historical evidence packs. Their trees are abbreviated
displays, so they are replayed as depth-limited degradation cases, never as a
positive-acceptance oracle.

## Reference judgments

Each judgment rests on the files named — not on a repository's name or
reputation — and says which component speaks for the repository and why. Two
results are recorded per snapshot: **complete-enough** (selection plus extras)
and **pipeline** (the selection alone, which is what `analyze` sees).

| Repository | Complete-enough | Pipeline | Rationale |
|---|---|---|---|
| Plantly | mobile, resolved | mobile, resolved | Root `package.json` declares `expo` and `react-native` in `dependencies`. |
| Starship | cli, resolved | cli, resolved | Cargo's implicit binary target `src/main.rs`; `clap` in `[dependencies]`; no web framework. |
| GitHub CLI | cli, resolved | cli, resolved | Two read commands, `cmd/gh/main.go` and `cmd/gen-docs/main.go`; `go.mod` requires `github.com/spf13/cobra`. The nested Go modules under `.github/` and the `script/` gateway command are tooling, not the product. |
| Excalidraw | frontend, resolved | other, inconclusive | Root is a private yarn workspace (an aggregator). `excalidraw-app` is a private app with React in `dependencies` and `excalidraw-app/index.html`. `packages/*` are libraries — `@excalidraw/excalidraw` and its internals are dependencies of the app; `@excalidraw/utils` is published alongside it. `examples/*` are supporting. One executable member beside libraries ⇒ the app is the product. **Pipeline:** the selection's per-basename cap fetched 3 of 12 `package.json` files, so the `packages/*` members are unresolved and the verdict abstains, naming each unfetched manifest. |
| PocketBase | library, resolved | other, inconclusive | The root module's own package is `package pocketbase` (importable); `cmd/serve.go` and `cmd/superuser.go` are `package cmd`, not commands; the only `package main` is `examples/base/main.go`, an example. **Ambiguity:** the project presents itself as a backend you run, and its release binary is built from `examples/base`; the repository's declared structure is a Go framework. The classifier reports the declared structure, so the reference is *library*, with the backend product framing recorded here rather than forced. **Pipeline:** the selection reserved its entry slots for `examples/base/main.go` and never fetched `cmd/*.go`, so "no command" cannot be concluded; the verdict abstains and names both files. |
| Face Recognition | other, inconclusive | other, inconclusive | No packaging metadata at all — a README and one OpenCV script. A standalone script sits outside the kind taxonomy; abstaining is the correct result, not a miss. |

## Degradation cases

The tests pair each supported snapshot with mutations that remove, truncate or
fail its decisive evidence (for example an unreadable `Cargo.toml` or
`excalidraw-app/package.json`, a truncated listing), and assert the specified
weaker result. Results are reported as resolved / correct / incorrect /
inconclusive counts per set, with per-case outcomes; these are a handful of
selected repositories, not a measurement of general accuracy.
