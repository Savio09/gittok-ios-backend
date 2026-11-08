# G18 backend execution

## Contract and scope

Add optional `evidence_status` to claims and metrics (`supported`, `inferred`, `unknown`); add optional `kind_evidence_status` and `evidence_mode` to repository previews. Existing JSON field types, stance and confidence vocabulary remain unchanged. Missing status stays omitted for legacy round trips; generated results are explicit. Evidence status describes support for a statement at its stated scope; it is independent of runtime execution and reader fit.

Builder declarations, literal constructs and attributed GitHub/README statements are supported. Repository-kind classifications are inferred with legacy confidence uncertain; undecided kinds are unknown. Metadata-feed kind heuristics are inferred or unknown, never supported. All generated previews and sections remain not_executed. Partial reads keep validated facts and coverage notes without invented unknown claims. Builder version 4 invalidates older previews.

Keep the metadata feed's legacy last_commit key for compatibility, correcting its pushed_at label to since last push. Preserve acquisition bounds, source provenance and privacy gates. No external writes, commits, secrets reads or unrelated cleanup.

## Ordered increments and verification

1. Add maintained failing regressions for generated evidence semantics, optional JSON compatibility, deduplication/validation status, metadata feed, partial cases and v3 cache invalidation.
2. Add the enum and optional fields; populate direct facts, metadata metrics, classification and preview status/runtime fields. Run focused tests to green.
3. Refresh maintained illustrative JSON examples and export actual builder/feed response fixtures under /tmp/gittok-g18-contract/ for Swift client integration.
4. Review diff and validate full Go suite, race detector, vet and build. Record actual outcomes and limitations here.

## Decisions and authoritative references

Go 1.25.5 from go.mod; standard library encoding/json omitempty preserves old optional-field encodings: https://pkg.go.dev/encoding/json@go1.25.5#Marshal. No framework-specific behavior or new dependencies are needed. Existing exact candidate validation already compares every Claim field through reflect.DeepEqual; explicit status-forgery regressions will protect it.

## Execution log

- Initial state: HEAD 3cc4c0e; pre-existing docs/api-contract.md edits and untracked G17 docs/reviews/tasks preserved.

- RED: focused maintained G18 tests failed on missing explicit statuses/mode, confident classification, forged-status validation/dedup acceptance, pushed_at wording, and reuse of persisted version-3 output. Optional legacy JSON test passed before implementation.
- GREEN: minimal enum/schema/rendering changes passed the focused tests. ValidateClaim already uses full Claim equality; status forgery tests now reject mutations. Deduplication explicitly includes status.
- Captured G17 previews refreshed only for additive evidence fields and inferred classification confidence; source evidence and acquisition captures unchanged.
- Added exact production HTTP feed output regression using recorded glow/rack metadata. This protects inferred metadata kinds and the legacy pushed-at key with truthful label.
- Final verification: go test ./..., go test -race ./..., go vet ./..., go build ./..., git diff --check all PASS. The initial sandbox-only full test attempt was blocked by Go-cache permissions and local listener restrictions; the authorized escalated run passed.
- Actual JSON fixtures exported to /tmp/gittok-g18-contract/: preview.inferred.json (CLI), pocketbase__pocketbase.preview.json, charmbracelet__glow.preview.json, rack__rack.preview.json, preview.unknown.json, preview.partial.json, feed.metadata.json, and feed.captured.json. The captured feed is the exact HTTP handler response with two recorded public repositories; other examples are marshalled Build/FromMetadata outputs. Synthetic any/repo examples are explicitly contract cases, not live observations.
- Scope limit: feed.Service has no pagination implementation and emits no next_cursor. The fixture preserves that actual behavior; a client pagination simulation is separate. Metadata heuristic accuracy is unchanged and now labeled inferred. No target code was executed or new live repository acquisition performed.
- Independent review delegated to the coordinating parent. No commits, pushes, Linear updates, secrets reads or pre-existing documentation edits made by this implementation.

### Independent review repair: metadata metric presence

- Confirmed P2: FromMetadata previously promoted Go zero/default fields or mismatched/null provider values to supported metrics.
- RED: TestEvidenceMetadataMetricsRequireMatchingProviderFields reproduced ten failing omission cases covering missing/null/mismatched stars, license and pushed_at plus NOASSERTION license; explicit reported-zero and matching-value controls passed.
- GREEN: shared reportedMetadataValue predicate now gates analyzed and metadata-feed metrics; only matching non-null retained provider fields can yield supported. Unsupported metrics are omitted. Existing feed display formatting, ordering and legacy pushed_at key/label are preserved. Synthetic metric expectations now supply honest retained provider fields.
- Regenerated maintained G18 examples and actual /tmp/gittok-g18-contract/feed.captured.json; valid captured outputs retain equivalent metric semantics. All post-repair checks passed: go test ./..., go test -race ./..., go vet ./..., go build ./..., git diff --check.
