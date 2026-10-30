#!/usr/bin/env python3
"""Build an XPI from an explicit runtime allowlist, without local/test files."""
import json
from pathlib import Path
import zipfile

root = Path(__file__).resolve().parent
manifest = json.loads((root / 'manifest.json').read_text())
out = root.parent.parent / 'dist' / f'tayga-thunderbird-v{manifest["version"]}.xpi'
files = ['manifest.json', 'client.js', 'background.js', 'options.html', 'options.js',
         'rules.js', 'rules.html', 'rules-ui.js', 'rules.css', 'popup.html', 'popup.js', 'styles.css', 'icon.svg', 'ews.js', 'experiments/ews/api.js', 'experiments/ews/schema.json']
out.parent.mkdir(exist_ok=True)
with zipfile.ZipFile(out, 'w', compression=zipfile.ZIP_DEFLATED) as archive:
    for name in files:
        entry = zipfile.ZipInfo(name, (2026, 1, 1, 0, 0, 0))
        entry.compress_type = zipfile.ZIP_DEFLATED
        entry.external_attr = 0o644 << 16
        archive.writestr(entry, (root / name).read_bytes())
print(out)
