# Independent source and kind-proof review

Reviewed `82eb571..d0bd691` against contract B / T2–T4. No application, fixture, Git, or plan edits. Applied the code-review-and-quality workflow. Reproductions use a Go overlay; the supplied text is a test source, not a package file. No external requests were made.

## Required findings (all P2)

1. **Use an unambiguous JSON field-location key.** `/Users/fdeclan/Public/gittok-ios-backend/internal/crawler/locations.go:46` joins path segments with NUL, although JSON keys may contain an escaped NUL. With `package.json` containing `{"dependencies":{"react":"19"},"dependencies\u0000react":"UNRELATED"}` and listed `index.html`, both the React declaration claim and resolved frontend proof cite the unrelated second property. A multiline fixture proves the links point at line 3 instead of the real declaration on line 2. The semantic validator resolves the real nested dependency but compares against the same collided locator key, so this is an accepted false citation, not just a rejected/tampered fact. Use a structural or length-encoded key, including unambiguous descendant deletion. Repro: `TestIndependentSourceNULCollision`.

2. **Carry the consumed Go clauses required by the no-command premise.** `/Users/fdeclan/Public/gittok-ios-backend/internal/crawler/classify.go:649` records only `go.mod` in the new typed `AbsenceGoCommand` scope. For `go.mod = "module example.com/lib\n"`, `lib.go = "package lib\nfunc F() {}\n"`, and `cmd/tool/helper.go = "package helper\n"`, the builder displays library with sources go.mod, lib.go:1, and tree only; helper.go is absent. Yet goNoCommandPremise reads helper.go to establish its non-main clause, and making it unread changes the display to other. Tree paths cannot establish package declarations. Include every necessary inspected command/root clause in the scope and citations; the renderer at kindproof.go:252 must describe the source-content inference accurately. Repro: `TestIndependentSourceAbsencePremiseFiles`.

3. **Retain workspace/supporting-component selection provenance.** `/Users/fdeclan/Public/gittok-ios-backend/internal/crawler/resolve.go:402` exports only each retained product's role basis. A root `package.json = {"private":true,"workspaces":["apps/*"],"dependencies":{"sidecar":"*"}}`, `apps/web/package.json = {"name":"web","dependencies":{"react":"19"}}`, listed `apps/web/index.html`, and `apps/sidecar/package.json = {"name":"sidecar","dependencies":{"express":"5"}}` resolve frontend. The proof says web is the only product, citing only web's React field and the tree. The necessary root dependency declaration and sidecar name are not cited or retained. Removing just the omitted root dependency changes the result to other because independent frontend/backend products conflict. The generic supporting-path disclaimer does not supply those facts. Preserve and cite the workspace/root and dependency/name premises or withhold the display. Repro: `TestIndependentSourceWorkspacePremises`.

4. **Do not turn a truncated tree into a verified absence.** `/Users/fdeclan/Public/gittok-ios-backend/internal/preview/kindproof.go:260` renders `AbsenceNPMCommandAndShell` as the tree showing no app shell, although its scope is only complete manifest reads and withTree does not check tree coverage. With complete `package.json = {"main":"./index.js"}`, a recorded pinned tree request, and `Inspection.Scope.Truncated = true`, Build still displays library with that confident absence claim. An app shell can lie beyond truncation. This internal classifier behavior existed before the repair, but the new resolved proof and display gate fail the T4 requirement to withhold a kind whose required absence premise cannot be verified. Repro: `TestIndependentSourceIncompleteAbsence`.

## Evidence and coverage

- Independent overlay tests 1–4 fail as described. Tests 2 and 3 also demonstrate the omitted premise changes the result to `other`.
- Seven independently specified normal controls pass: quoted/grouped Go requirements and CRLF, effective duplicate requirement, clipped Go final token, malformed Go, escaped/multiline/nested JSON duplicate fields, replaced duplicate parent object, and malformed/clipped JSON.
- Inspected actual generated payloads from production-selected cli/cli, PocketBase, starship, and excalidraw snapshots: 43 positioned primary/additional sources independently matched raw inclusive line contents; no ordinary byte-location mismatches. PocketBase/excalidraw correctly abstain in these snapshots. All section evidence modes remain not_executed.
- Existing `go test -count=1 ./internal/apimodel ./internal/crawler ./internal/preview` passes. This does not catch the independent failures.
- Additive optional `additional_sources` serialization, unambiguous claim dedup keys, exact-span UTF-8 handling, ordinary field/span tamper rejection, and recorded pinned tree URL checks showed no additional concrete issue.
- Duplicate JSON object decoding merges typed map fields while generic validation uses replacement semantics; this causes conservative fact rejection/abstention and predates the repair, so it is not counted as a new finding.

## Reproduction

Copy `adversarial_test.go.txt` to a temporary Go file and create an overlay mapping `/Users/fdeclan/Public/gittok-ios-backend/internal/preview/source_independent_review_test.go` to it. This review's existing overlay is `/tmp/g17-sources-review-20261104/overlay.json`.

```
go test -overlay /tmp/g17-sources-review-20261104/overlay.json -run '^TestIndependentSource' -v ./internal/preview
```

The original output capture is in `output.txt`; later incomplete-tree and normal-position controls are in `edge-output.txt`. Temporary originals remain under `/tmp/g17-sources-review-20261104/`.
