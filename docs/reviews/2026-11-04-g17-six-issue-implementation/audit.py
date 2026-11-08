"""Offline audit against consumed snapshot bytes, independent of Go renderers."""

import json
from pathlib import Path


artifact = Path(__file__).resolve().parent
root = artifact.parents[2]
references = file_references = claims_checked = 0
for output in sorted((artifact / "previews").glob("*.json")):
    name = output.name
    if name in {"charmbracelet__glow.json", "rack__rack.json"}:
        raw = json.loads((root / "internal/analysis/testdata/g17" / name).read_text())
        files = {f["Path"]: f["Content"] for f in raw["consumed"]["Files"]}
        sha = raw["revision"]["Ref"]
    else:
        raw = json.loads((root / "internal/crawler/testdata/corpus" / name).read_text())
        files = {p: f["content"] for p, f in raw["files"].items() if p in raw["selected"]}
        sha = raw["resolved_sha"]
    preview = json.loads(output.read_text())
    claims = [c for s in preview["sections"] for c in s.get("claims", [])]
    assert len(claims) <= 24, name
    assert all(s["evidence_mode"] == "not_executed" for s in preview["sections"]), name
    deps = 0
    checked = 0
    for claim in claims:
        label = claim["label"]
        dependency = (
            label.startswith("Declares ")
            and not label.startswith("Declares command ")
            and label not in {"Declares binary target table", "Declares library target table"}
        )
        deps += dependency
        for i, src in enumerate([claim["source"]] + claim.get("additional_sources", [])):
            references += 1
            if src["kind"] != "file":
                continue
            checked += 1
            file_references += 1
            assert src["ref"] in files, (name, src)
            content = files[src["ref"]]
            assert "/blob/" + sha + "/" in src["url"], (name, src)
            quoted = content
            if src.get("lines"):
                a, b = map(int, src["lines"].split("-"))
                lines = content.split("\n")
                assert 1 <= a <= b <= len(lines), (name, src)
                quoted = "\n".join(lines[a - 1:b])
                assert src["url"].endswith(f"#L{a}-L{b}"), (name, src)
            if i == 0 and claim.get("excerpt"):
                assert claim["excerpt"] in quoted, (name, label, src)
            if i == 0 and dependency:
                assert label[9:] in quoted, (name, label, src)
    assert deps <= 8, (name, deps)
    notes = [s.get("body", "") for s in preview["sections"] if s["kind"] == "decision"]
    assert len(notes) <= 1
    assert all(len(n.split("\n")) <= 3 and len(n) <= 500 for n in notes), name
    if name == "excalidraw__excalidraw.json":
        assert any(c["label"] == "Declares react" and c["source"]["ref"] == "excalidraw-app/package.json" for c in claims)
    if name == "cli__cli.json":
        assert any(c["label"] == "Declares github.com/spf13/cobra" for c in claims)
        assert not any(c["label"] == "Contains net/http call" and c["source"]["ref"].startswith("script/") for c in claims)
    claims_checked += len(claims)
    print(f"{name}: kind={preview['kind']}, claims={len(claims)}, dependencies={deps}, file references={checked}; PASS")

assert len(list((artifact / "previews").glob("*.json"))) == 8
print(f"PASS: {claims_checked} claims, {references} total references, {file_references} file references checked across 8 snapshots")
