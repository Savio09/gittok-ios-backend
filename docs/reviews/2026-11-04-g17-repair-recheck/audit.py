import json
from pathlib import Path

root = Path('/Users/fdeclan/Public/gittok-ios-backend')
artifact = Path(__file__).resolve().parent
for name in ['cli__cli', 'pocketbase__pocketbase', 'charmbracelet__glow', 'rack__rack']:
    if name in ['cli__cli', 'pocketbase__pocketbase']:
        raw = json.loads((root / 'internal/crawler/testdata/corpus' / (name+'.json')).read_text())
        files = {p: v['content'] for p, v in raw['files'].items() if p in raw['selected']}
        sha = raw['resolved_sha']
    else:
        raw = json.loads((root / 'internal/analysis/testdata/g17' / (name+'.json')).read_text())
        files = {v['Path']: v['Content'] for v in raw['consumed']['Files']}
        sha = raw['revision']['Ref']
    preview = json.loads((artifact / (name+'.json')).read_text())
    claims = [c for s in preview['sections'] for c in s.get('claims', [])]
    source_count = 0
    dep_count = 0
    for c in claims:
        if c['label'].startswith('Declares ') and not c['label'].startswith('Declares command '):
            dep_count += 1
        for i, src in enumerate([c['source']] + c.get('additional_sources', [])):
            source_count += 1
            if src['kind'] != 'file':
                continue
            content = files[src['ref']]
            assert '/blob/'+sha+'/' in src['url'], (name, src)
            if src.get('lines'):
                a, b = [int(x) for x in src['lines'].split('-')]
                lines = content.split('\n')
                assert 1 <= a <= b <= len(lines), (name, src)
                text = '\n'.join(lines[a-1:b])
                if i == 0 and c.get('excerpt'):
                    assert c['excerpt'] in text, (name, c['label'], src)
                if c['label'].startswith('Declares ') and not c['label'].startswith('Declares command '):
                    assert c['label'][9:] in text, (name, c['label'], text)
            if i == 0 and c.get('excerpt'):
                assert c['excerpt'] in content, (name, c['label'])
    notes = [s['body'] for s in preview['sections'] if s['kind'] == 'decision']
    assert len(notes) <= 1
    if notes:
        assert len(notes[0].split('\n')) <= 3 and len(notes[0]) <= 500
    assert dep_count <= 8
    assert all(s['evidence_mode'] == 'not_executed' for s in preview['sections'])
    print(f'{name}: kind={preview["kind"]}; claims={len(claims)}; sources={source_count}; deps={dep_count}; notes={notes}; direct byte/line and pinned-SHA checks passed')
