# G19: bounded repository analysis

Ticket: [SAV-25](https://linear.app/savios-workspace/issue/SAV-25/g19-enforce-hard-limits-files-size-api-calls-time-budget).

## Audit and contract

Existing acquisition retains at most 5,000 tree entries (depth 12), selects 16
files, and retains 64 KiB per file / 512 KiB total. Retries, cancellation,
public credential isolation, identity revalidation, admission and cache outcome
rules already exist. Successful response decoding and physical API attempts
are not bounded; limits are hard-coded; metadata and acquisition renew timeouts.

Preserve selection defaults and calibrated unknown/partial evidence. Configure
positive limits at service construction / server startup. Count every HTTP
attempt, including retries and redirects, across metadata, pinned reads and
identity revalidation. Stop reading oversized responses before allocating their
full body; stream tree entries and decode only retained file bytes. Use one
absolute acquisition deadline from before metadata, also when coalescing work.
Deadline/provider/call-budget failures remain non-cacheable; completed declared
tree/file caps retain existing cacheability. Public identity checks fail closed.

## Ordered work

1. Add failing tests for physical call quotas, capped response reads, streaming
   large trees, bounded base64 decoding, and cancellation/retry exhaustion.
   Implement client budgets without changing credential checks.
2. Add failing tests for custom service budgets and a shared deadline. Wire
   typed limits through acquisition, public previews and startup configuration.
3. Document environment settings and coverage/cache behavior. Run focused tests,
   full Go tests, race, vet and build. Send the artifact and contract for an
   independent root-agent review, then commit the tested slices locally.

## Verification

All provider tests use controlled contexts, in-process transports or httptest;
no real GitHub traffic or credentials. Include adversarial tree/file bodies,
failed-file batches, retries, redirects, exhausted quotas, private/reused IDs,
coalescing and cache regressions. Go parsers are synchronous: finite input caps
bound local work; context checkpoints prevent starting new local stages after
the acquisition deadline. Any remaining wall-clock preemption limitation must
be explicit in the final report.

## Sources

- https://pkg.go.dev/io@go1.25.5#LimitReader
- https://pkg.go.dev/net/http@go1.25.5#RoundTripper
- https://pkg.go.dev/context@go1.25.5#WithDeadline
- https://pkg.go.dev/encoding/base64@go1.25.5#NewDecoder

## Execution and review — November 6

Client slice ac66cf0 and review repair d9de36d add physical attempt quotas,
bounded response/tree/base64 reads, fresh HTTP/1 connections and cancellable
production backoff timers. Service slice c6e671d wires configurable limits and
one absolute deadline through acquisition, coalescing and local interpretation.

Fresh client review identified hidden transport retries, reading a tree tail
after filling its cap, integer-overflow limits, and leaking the overflow sentinel
to JSON decoding. A second pass found surviving HTTP/2 ALPN/handlers. These were
fixed; a real in-process HTTP/2-capable TLS server now verifies fresh HTTP/1
requests and the exact quota. The final bounded rereview found no remaining
contract-breaking issue in that repair.

Fresh service review found that classification started later parsers after
cancellation. Context checkpoints now stop later manifest/Go parser calls, with
a deterministic checkpoint regression; final bounded rereview found no remaining
contract violation. Existing stricter admission deadlines remain documented.

Root independently verified the final source with full uncached Go tests, full
race tests, vet, build and diff checks. All passed, using local transports and
httptest only. Full logs and the process record are saved in
[the shared G19 record](../../docs/2026-11-06-g19-hard-limits/README.md).
Synchronous parsers already running may finish beyond the deadline: this is
cooperative cancellation with finite input caps, not process CPU preemption.
