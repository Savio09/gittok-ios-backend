# Independent cache/concurrency review — November 4, 2026

Reviewed base `82eb571` → `d0bd691`, repair contract §E/T9–T11 and implementation
record, actual cache/coalescer/acquisition/HTTP code, and maintained regression
tests. Applied code-review-and-quality; this is the delegated fresh-context
adversarial reviewer, without nested reviewers or external model CLIs.

Two concrete P2 findings. Both are gaps in the new T9/T10 repair, rather than a
claim that the uncached baseline previously had bounded/coalesced requests.
No app, git state, plans or shared fixtures were edited.

## P2: Bound visibility work before making the upstream request

Anchor: `internal/analysis/preview.go:31` (admission occurs only at line 58).

Every caller starts `metadata` before any admission decision. That method has its
own 30-second deadline and uses the HTTP client/retry pipeline. Distinct keys,
same-key followers beyond their limit, and warm requests can all create arbitrarily
many simultaneous upstream calls outside the group's two acquisitions/64 waiters.
Overload therefore does not fail fast: it first pays a GitHub request and can wait
for a slow/retrying provider. This bypasses the intended process resource bound and
can consume the very quota R6 was intended to protect.

Deterministic actual-HTTP repro: configure active/per-key/total limits to 1, hold
the accepted request at its tree endpoint, and launch 80 more callers. All 80
reach and block simultaneously at the fake GitHub metadata endpoint while group
state remains active=1/waiters=1. After releasing their metadata, all are rejected
with `ErrOverloaded`. Raising 80 raises the amount of unadmitted external work.

Fix direction: apply a finite request/metadata admission bound before contacting
GitHub, retaining the mandatory public-visibility check for every served hit and
the verified-identity grouping for acquisitions. The precise design can keep
visibility and acquisition budgets distinct; neither should be unbounded.

## P2: Recheck the cache when a caller becomes the acquisition leader

Anchors: `internal/analysis/preview.go:44–45`, `:58`; completion retires the group
at `internal/analysis/coalesce.go:111–114`.

The outer cache lookup and flight admission are separate operations. Caller B
can observe a miss, pause, and reach admission only after caller A stores a valid
snapshot and retires the completed flight. B starts another full acquisition:
the selected `run` closure unconditionally calls `acquireFrom`, with no second
cache lookup. A concurrent identical request can thus repeat all ref/tree/content
work and replace a usable snapshot, defeating the advertised shared pipeline.

Deterministic actual-HTTP repro uses an overlay synchronization hook immediately
after B's outer cache miss. A completes and stores; B resumes and becomes the new
leader. For an empty tree, endpoint counts are metadata=4/ref=2/tree=2; the expected
shared/cached path is metadata=3/ref=1/tree=1. A real selected tree adds its entire
contents batch again. No identity, expiration or acquisition behavior is changed
by the hook.

Fix direction: after becoming a new flight's leader, recheck the same verified
identity in cache before acquiring. Return that original snapshot as a cache hit;
avoid rewriting it with zero/new capture metadata or extending its original TTL.

## Verification and scope

Artifacts in this directory:

- `cache_review_overlay_test.go.txt`: two deterministic channel-barrier regressions.
- `preview_instrumented.go.txt`: exact production preview.go plus one conditional
  synchronization hook after the outer lookup. It is loaded only through overlay.
- `overlay.json`: absolute mapping; no live Go source/test files are added.

From repository root, with loopback permission:

```sh
go test -race -overlay docs/reviews/2026-11-04-g17-swarm-review/cache/overlay.json -run TestSwarm -count=20 -v -timeout 60s ./internal/analysis
```

Both cases fail with their intended behavioral assertions in all 20 repetitions;
no data race is reported. Representative output:

```text
TestSwarmAdmissionBoundsPreAdmissionHTTP
  saturated limits: active=1 waiters=1; 80 additional metadata HTTP requests are simultaneously blocked upstream
  admission limit=1 allowed 80 extra upstream visibility requests
TestSwarmCacheMissRechecksAfterWinningFlight
  HTTP endpoint counts: metadata=4 refs=2 trees=2
  two simultaneous same-key misses ran 2 acquisitions despite first storing a valid cache entry
```

Existing cache/identity/expiry/coalescing/cancellation/generation/admission tests
were also independently rerun:

```sh
go test -race -count=1 -run 'TestCache|TestCachedPreviews|TestOnlyCacheable|TestExpiredPreviews|TestSimultaneous|TestAFollower|TestTheLast|TestALate|TestAdmission|TestShared' ./internal/analysis
```

Result: PASS (`ok …/internal/analysis 1.347s`). Their passing assertions do not
cover either interleaving above.

No additional concrete issue found in the examined TTL/default storage eviction,
bounded reads, restart reuse, malformed-JSON/oversized rejection, identity and
visibility hit gates, follower cancellation or retired-generation cache-write
guard. File reads/expiry/identity are rechecked on hits; cached previews retain
their captured metadata and source links. Strict total provider bytes/CPU and
multi-process cache coordination remain explicitly outside this repair's scope.
