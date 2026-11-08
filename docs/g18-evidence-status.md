# G18 evidence status

This records the G18 builder and fixture checks. The single-page feed described
in its export example is the historical fixture service; production now uses
[paginated public discovery](public-discovery.md). Optional reviewed narratives
are documented in [walkthrough operations](walkthrough-operations.md).

Repository previews now distinguish support for a statement, runtime execution, and reader fit. `evidence_status` is optional on Claim and Metric; `kind_evidence_status` and `evidence_mode` are optional on RepoPreview. Older payloads retain their shape when decoded and encoded. Generated results set the new fields explicitly. Confidence keeps its existing `confident` / `uncertain` vocabulary; Stance is unchanged.

| Value | Meaning | Generated examples |
| --- | --- | --- |
| supported | The cited source directly establishes the statement at its stated scope. | A parsed dependency declaration; a literal source construct; what a README author states; what GitHub metadata reports. |
| inferred | A conclusion drawn from evidence or a heuristic. | A repository kind inferred from cited premises; a metadata-feed kind inferred from language/topics. |
| unknown | The evidence does not determine the answer. | An undecided repository kind. |

A supported declaration does not establish installation, runtime use, reachability or repository-wide architecture. A supported README statement establishes what the author says. Existing Supports and Limit prose retains those boundaries. Classification claims emit inferred and legacy confidence uncertain, even when every cited premise is directly supported. Top-level kind status follows the emitted kind: inferred when the builder can state and cite the whole basis, otherwise unknown. Metadata-feed heuristics are inferred or unknown, never supported.

All generated previews and walkthrough sections emit evidence_mode not_executed. Evidence status never implies execution; a supported setup command was still not run. Fit stance is independent: a supported fact can be a caution. Optional fields use the standard Go string/omitempty contract, verified against [Go 1.25.5 encoding/json](https://pkg.go.dev/encoding/json@go1.25.5#Marshal). Empty/missing status makes no support guarantee. Clients should treat missing or unfamiliar statuses as unknown while displaying retained evidence.

Partial acquisition keeps only validated facts and existing plain coverage notes. It does not manufacture unknown claims to fill gaps. Builder version 4 prevents stored version-3 previews from being served under the new semantics. Claim audit equality and deduplication include EvidenceStatus.

Both feed and analyzed preview metrics require a matching, non-null retained provider metadata field before emitting supported. Missing fields never become a supported zero; an explicitly reported zero remains supported. Null, mismatched and NOASSERTION license values are omitted. Feed count/age formatting and metric order remain unchanged.

The metadata-feed metric sourced to pushed_at retains legacy key last_commit for compatibility, but its label is now since last push. The analyzed builder already uses last_push and last push reported by GitHub. Neither reports a verified commit time.

## Maintained checks and examples

The regressions in internal/preview/evidence_test.go, internal/apimodel/evidence_test.go, internal/feed/evidence_test.go and internal/analysis/cache_upgrade_test.go cover classification/direct-fact calibration, optional legacy JSON, fit/execution independence, status forgery and deduplication, unknown/partial output, truthful push labels and cache invalidation. Existing public captured preview goldens are refreshed without changing acquisition inputs.

`internal/preview/testdata/g18/` holds production-builder JSON examples. They include two captured corpus repositories plus explicit synthetic unknown/partial cases and a stable metadata-only contract case. Refresh with GITTOK_UPDATE_G18_EXAMPLES=1 only after auditing changes against the retained sources. The separate schema sample in internal/apimodel/testdata/feed_example.json illustrates optional schema capabilities; its hypothetical media is not output from the source-only builder.

The exact HTTP feed response for captured glow/rack metadata can be exported with:

```sh
GITTOK_G18_EXPORT_DIR=/tmp/gittok-g18-contract go test ./internal/feed -run TestEvidenceHTTPFeedFromCapturedMetadata -count=1
```

The exported response is feed.captured.json. Its relative push ages reflect the export clock. feed.Service currently returns a single assembled page with no cursor; pagination testing belongs to a clearly identified client simulation. No new acquisition or target execution is involved in these tests.

Final backend verification: go test ./..., go test -race ./..., go vet ./..., go build ./..., and git diff --check passed.
