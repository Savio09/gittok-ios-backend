# Live GPT-6.1 Sol control — November 6, 2026

These are actual local HTTP results, not fixtures or sandbox observations.
Model: `gpt-6.1-sol`; reasoning: `low`; service tier: `default`; optional
corrections: 0. The unchanged limits were $2 per operator window, $0.50 per
job, 8 model calls, 512 KiB cumulative serialized input and 12,000 output tokens.
Input accounting uses the conservative $2.50/M cache-write ceiling; output $10/M.

| Repository | Result | Local accounted cost | Assessment |
|---|---|---|---|
| `chalk/strip-ansi` | Ready in 29.8 seconds; 2 settled model calls | $0.042028 | 18 exact citations, 10 claims, 4 scenes, 5 pinned files; reviewer-agent source-quality assessment passed |
| `charmbracelet/glow`, before context fitting | Initial writer preflight failed; 0 model calls | $0 | Preserved as `charmbracelet__glow.preflight.json` |
| `charmbracelet/glow`, after context fitting | Ready in 32.35 seconds; 4 settled model calls | $0.130476 | 22 exact citations, 9 claims, 4 scenes, 2 pinned files; reviewer-agent source-quality assessment passed |
| `charmbracelet/glow`, final code `bc01c8d` | Ready in 31.01 seconds; 4 settled model calls | $0.118259 | 20 exact citations, 8 claims, 4 scenes, 2 files; core workflow supported, one conditional wording defect |

The accepted story explains a concrete CSI-containing string through validation,
fast-path checks and regex replacement to a test-asserted result. It attributes
tests and comments, explains useful design choices and points to specific next
sources. All story claims remain `inferred` / `not_executed`.

Glow's story follows piped Markdown through stdin dispatch, source reading,
frontmatter-helper invocation, Glamour renderer configuration/Render, text sizing
and stdout. It explains destination-aware presentation and the common source
abstraction, and points to the TUI handoff and style/width controls. Helper
internals and the exact visual treatment of the sample remain explicitly
unestablished. A minor clarity note in the audit recommends repeating the
non-pipe premise beside the interactive no-argument branch; the complete story
already explains the earlier pipe check. No material factual defect was found.

The independent source audit fetched public pinned source separately from the
investigator. GitHub's public contents API with raw media type replaced a raw
host that suffered DNS/timeouts. No credential was supplied to the audit.
Semantic assessment was performed by AI reviewer agents using the product rubric,
not human study participants. Owner acceptance and participant evaluation remain
separate work; the reviewer is not a truth oracle.

The final-code artifact and audit use the `.final.json` / `.final.audit.json`
suffixes. Its width explanation omits an additional condition: terminal-width
detection requires stdout to be a terminal, as well as an unchanged width flag
and zero configured width. The source and workflow are useful, but this wording
is not fully entailed. The automated publication reviewer accepted it; the separate
reviewer agent caught it. This is evidence of the semantic gate's remaining
limitations, not an unqualified clean semantic pass.

Total accounting after these controls: $0.813610 charged or reserved, including
two earlier unknown outcomes that were retained. This is not a provider invoice.
Library and CLI successes do not establish universal repository coverage. Model
selection, source-context and submission fixes changed during this investigation;
this is not an isolated model-quality benchmark. See
[the process record](../../walkthrough-validation.md) for earlier failures and
superseded approaches.
