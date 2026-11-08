"""Compare accepted visual stories with separately fetched, pinned public source.
No credentials, model calls, or target execution. Exactness is not semantic proof.
"""
import base64
import hashlib
import json
from pathlib import Path
import re
import sys
import urllib.parse
import urllib.request

artifact = Path(sys.argv[1])
output = Path(sys.argv[2])
output.mkdir(parents=True, exist_ok=True)
job = json.loads(artifact.read_text())["job"]
if job.get("state") != "ready":
    raise SystemExit("No accepted story to audit.")
preview = job["preview"]
cache, commits, errors = {}, set(), []
claim_ids, beat_ids = set(), set()
counts = {"claims": 0, "citations": 0, "scenes": len(preview.get("sections", []))}

def fetch_span(source):
    counts["citations"] += 1
    url = urllib.parse.urlsplit(source["url"])
    parts = [urllib.parse.unquote(p) for p in url.path.split("/")]
    if (url.scheme != "https" or url.netloc != "github.com" or len(parts) < 6
            or parts[1:3] != [preview["owner"], preview["name"]]
            or parts[3] != "blob" or not re.fullmatch("[0-9a-f]{40}", parts[4])):
        raise ValueError("Invalid pinned source URL")
    commit, path = parts[4], "/".join(parts[5:])
    if path != source["ref"]:
        raise ValueError("Path differs from URL")
    start, end = map(int, source["lines"].split("-"))
    if start < 1 or end < start or end - start >= 10 or url.fragment != f"L{start}-L{end}":
        raise ValueError("Invalid exact line range")
    commits.add(commit)
    key = commit + ":" + path
    if key not in cache:
        endpoint = ("https://api.github.com/repos/" + urllib.parse.quote(preview["owner"], safe="")
                    + "/" + urllib.parse.quote(preview["name"], safe="") + "/contents/"
                    + urllib.parse.quote(path, safe="/") + "?ref=" + commit)
        request = urllib.request.Request(endpoint, headers={"User-Agent": "gittok-source-audit",
                                                        "Accept": "application/vnd.github.raw+json"})
        with urllib.request.urlopen(request, timeout=25) as response:
            raw = response.read((2 << 20) + 1)
            content_type = response.headers.get("Content-Type", "")
        if len(raw) > 2 << 20:
            raise ValueError("Source audit file limit")
        if "application/json" in content_type:
            payload = json.loads(raw)
            if isinstance(payload, dict) and payload.get("encoding") == "base64":
                raw = base64.b64decode(payload["content"], validate=False)
        text = raw.decode("utf-8")
        lines = text.split("\n")
        actual_lines = [line + "\n" for line in lines[:-1]]
        if lines[-1]:
            actual_lines.append(lines[-1])
        filename = hashlib.sha256(key.encode()).hexdigest() + ".txt"
        (output / filename).write_bytes(raw)
        cache[key] = {"path": path, "commit": commit, "file": filename, "lines": actual_lines}
    lines = cache[key]["lines"]
    if end > len(lines):
        raise ValueError("Citation outside actual file")
    return "".join(lines[start - 1:end])

if preview.get("evidence_mode") != "not_executed" or preview.get("presentation") != "walkthrough":
    errors.append("Wrong presentation/provenance")
for section in preview.get("sections", []):
    if section.get("evidence_mode") != "not_executed" or section.get("media") is not None:
        errors.append("Section provenance/media mismatch")
    claims = section.get("claims", [])
    board = section.get("storyboard", {})
    if board.get("version") != 1 or len(board.get("beats", [])) != len(claims):
        errors.append("Missing complete ordered storyboard")
    for beat, claim in zip(board.get("beats", []), claims):
        if beat["id"] in beat_ids or claim["id"] in claim_ids or beat["claim_id"] != claim["id"]:
            errors.append("Duplicate/dangling/out-of-order identity")
        beat_ids.add(beat["id"])
        claim_ids.add(claim["id"])
    for claim in claims:
        counts["claims"] += 1
        try:
            spans = [fetch_span(s) for s in [claim["source"]] + claim.get("additional_sources", [])]
            if "\n\n".join(spans) != claim.get("excerpt"):
                raise ValueError("Quote differs from independent pinned source")
            if claim.get("evidence_status") != "inferred":
                raise ValueError("Wrong claim provenance")
            visual = claim["visual"]
            if not visual["cue"] or visual["cue"] not in claim["supports"]:
                raise ValueError("Invalid narration cue")
            if visual["kind"] == "source":
                expected = spans[0].removesuffix("\n").split("\n")[:6]
                if visual["title"] != claim["source"]["ref"] or visual["lines"] != expected:
                    raise ValueError("Code panel differs from actual first source lines")
        except Exception as error:
            errors.append({"claim_id": claim.get("id"), "error": str(error)})
if len(commits) != 1:
    errors.append("Source commits are mixed or absent")
registry = {key: {k: v for k, v in value.items() if k != "lines"} for key, value in cache.items()}
(output / "sources.json").write_text(json.dumps(registry, indent=2))
report = {"repository": preview["slug"], "job_id": job["id"], **counts, "unique_files": len(cache),
          "commits": sorted(commits), "errors": errors, "source_audit_passed": not errors,
          "spoken_words": sum(len(c.get("supports", "").split()) for s in preview.get("sections", []) for c in s.get("claims", [])),
          "semantic_quality": "Requires separate assessment; exact source matching alone does not establish meaning."}
(output / "audit.json").write_text(json.dumps(report, indent=2))
print(json.dumps(report, indent=2))
raise SystemExit(bool(errors))
