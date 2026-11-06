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
