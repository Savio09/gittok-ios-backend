# G17 independent security review — November 4, 2026

Reviewed baseline `82eb571` → current `d0bd691`, plus the repair contract sections D/E and T7–T11 and implementation report. Tests were read before implementation. Applied code-review-and-quality and security-and-hardening skills. No application, git state, shared fixture, or plan edits. No .env or secret files read. No real credential or live authenticated GitHub request used.

## Verdict

No concrete P1/P2 security findings in the reviewed scope.

The public-only client accepts anonymous or classic-PAT mode separately from feed authentication. Successful authenticated responses require explicit empty scope evidence before JSON decode; unsupported credential families, 401s, scope expansion, missing scope evidence, and cross-origin credential redirects fail with typed policy errors. Policy rejection propagates through preview acquisition and prevents both partial previews and cache writes. Cached previews require a fresh public metadata lookup and the stable-ID/canonical-name key, while failed final identity checks discard tree/file evidence. Public HTTP responses use fixed safe error messages and no-store headers.

## Independent adversarial evidence

Isolated overlay tests (no live source files):

- All 18 combinations of scope expansion, missing granted-scope evidence, and revocation at initial metadata, ref, tree, first content, second content, or final identity revalidation returned a typed policy error with an empty preview and zero cache bytes.
- A warmed persistent preview rejected later expanded scopes after exactly one metadata request and returned no cached preview.
- Redirects changing scheme, port, hostname, a deceptive host suffix, or host to github.com were terminal before any second request was sent. No provider redirect body entered the error.
- The independent cases pass under the race detector.

Existing maintained tests were also reviewed and run for duplicate scopes, absent versus empty header, unsupported token families, private/unknown visibility, rename/transfer/reused ID, startup rejection, cache headers, cancellation and server wiring.

Passed commands:

```sh
go test -count=1 ./internal/githubclient ./internal/analysis ./internal/config ./cmd/server
go test -race -count=1 -overlay /tmp/g17-security-review-20261104/overlay.json ./internal/analysis ./internal/githubclient -run '^TestSecurityOverlay' -v
```

The package run initially failed because the sandbox denied loopback listeners, and the race run initially hit denied Go build-cache access. Both were rerun with the required sandbox approval and passed. These are environment failures, not test failures.

For a reproducible artifact-based overlay, use this directory's `overlay.json` in place of the /tmp overlay above; the `.go.txt` files are mapped into virtual package test paths. They are not ordinary Go sources and do not enter `go test ./...`.

## Boundaries and residual uncertainty

This review covers credential/public-visibility isolation and its interactions with cache publication. The sibling cache/concurrency review owns exhaustive disk corruption, TTL/eviction, cancellation-generation, and admission mechanics. Provider-side visibility remains a lookup-time guarantee, as the contract specifies. Live authenticated behavior was not independently repeated here. This review does not establish strict total network-byte or CPU budgets (G19).

Provider policy was cross-checked against GitHub's primary documentation: [scope header semantics and no-scope public access](https://docs.github.com/en/apps/oauth-apps/building-oauth-apps/scopes-for-oauth-apps) and [token families](https://docs.github.com/en/authentication/keeping-your-account-and-data-secure/about-authentication-to-github).
