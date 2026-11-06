# Repository analysis limits (G19 / SAV-25)

Public previews inspect arbitrary public repositories without executing their
code. `analysis.DefaultLimits()` supplies bounded defaults; configure a service
with `UseLimits` before sharing it. The server reads these environment settings
at startup and rejects zero, negative, malformed or excessive values.

| Environment setting | Default | Maximum | Scope |
| --- | --- | --- | --- |
| `ANALYSIS_TIMEOUT` | `30s` | `2m` | Absolute deadline, beginning before metadata |
| `ANALYSIS_MAX_TREE_ENTRIES` | `5000` | `10000` | Retained tree entries |
| `ANALYSIS_MAX_DEPTH` | `12` | `32` | Path segments in retained tree |
| `ANALYSIS_MAX_FILES` | `16` | `40` | Selected and attempted files |
| `ANALYSIS_MAX_FILE_BYTES` | `65536` | `262144` | Retained bytes per file |
| `ANALYSIS_MAX_TOTAL_BYTES` | `524288` | `2097152` | Retained bytes across selected files |
| `ANALYSIS_MAX_API_CALLS` | `80` | `128` | HTTP attempts for one analysis |
| `ANALYSIS_MAX_RESPONSE_BYTES` | `16777216` | `33554432` | Decoded HTTP bytes read per response |

Integers are decimal bytes/counts. The timeout uses Go duration syntax. Selection
keeps the existing source, entry-point and basename sub-caps, so raising the
overall file cap does not guarantee that many useful files. Lower file/byte caps
may leave too little evidence to infer a type; unknown is an expected result.

The API-attempt quota includes initial metadata, commit resolution, tree and
contents requests, identity revalidation, retries and redirects. A cache hit
still performs its separately bounded current visibility lookup. Coalesced
misses share their leader's remaining quota and absolute analysis deadline;
follower metadata checks are separate bounded operations. The shared work keeps
its own cancellation lifetime: a departing caller does not cancel followers.
Admission deadlines can impose a shorter bound than `ANALYSIS_TIMEOUT`.

Standard HTTP transports are copied and use fresh HTTP/1 connections, preventing
automatic transport retries from bypassing the attempt quota. This adds a
connection handshake per attempt. An injected custom RoundTripper must make at
most one network attempt per call and honor cancellation. Credential isolation,
scope vetting, same-origin credential redirects and final identity checks remain
in force. Startup credential verification is separate from a repository analysis.

Tree entries are streamed: filling the cap immediately closes the response and
marks the listing incomplete, even if its unread tail might be empty. Deep-only
trees, oversized entries/unknown JSON fields, missing Content-Length, compressed
bodies and oversized contents are also bounded by response bytes and time.
The reader consumes at most the response cap plus one detection byte; that byte
is never given to the JSON decoder. Base64 decoding retains at most the smaller
file/total allowance plus one byte to detect truncation.

A completed inspection clipped by declared tree/file caps retains the existing
cache eligibility. Provider, call-budget and deadline interruptions are not
cached. If the final identity check cannot run, file evidence is withheld. If
local interpretation expires, the preview retains sourced metadata and reports
the time limit with an unknown project type. Existing stored snapshots keep
their provenance and TTL; a limit change does not refresh or rewrite them.

Time limits cancel network requests and retry timers and are checked throughout
inventory, classification and fact collection, and between parser/rendering
stages. Local parsers and sorts are synchronous and receive only finitely capped
inputs; an individual parser/sort already running can finish after the deadline.
This is a cooperative time budget, not process-level CPU preemption. There are
no detached timeout workers continuing repository interpretation.

Commands using `analysis.NewService` receive the same defaults and may call
`UseLimits` programmatically; the environment settings above configure the server.
All regression checks use local synthetic transports/httptest, never real GitHub
credentials or traffic.

Go contracts used: [bounded reads](https://pkg.go.dev/io@go1.25.5#LimitReader),
[transport requests](https://pkg.go.dev/net/http@go1.25.5#RoundTripper),
[absolute deadlines](https://pkg.go.dev/context@go1.25.5#WithDeadline), and
[streaming base64](https://pkg.go.dev/encoding/base64@go1.25.5#NewDecoder).
