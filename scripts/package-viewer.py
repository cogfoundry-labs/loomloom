#!/usr/bin/env python3
"""Package the workbench's build:viewer output. No checkout paths or credentials are embedded."""
import argparse
import hashlib
import json
from pathlib import Path
from zipfile import ZipFile, ZipInfo, ZIP_DEFLATED

parser = argparse.ArgumentParser(description=__doc__)
parser.add_argument('build_directory', type=Path)
args = parser.parse_args()
root = args.build_directory.resolve()
files = {p.relative_to(root).as_posix(): p.read_bytes() for p in sorted(root.rglob('*')) if p.is_file() and not p.is_symlink()}
if 'viewer.html' not in files or not any(p.endswith('.js') for p in files):
    parser.error('expected workbench build:viewer output')
if any(not (p == 'viewer.html' or p.startswith('assets/')) for p in files):
    parser.error('unexpected files in viewer build')
manifest = {'protocol': 'local-workflow-preview/v1', 'files': {p: hashlib.sha256(v).hexdigest() for p, v in files.items()}}
files['manifest.json'] = (json.dumps(manifest, sort_keys=True, indent=2) + '\n').encode()
out = Path(__file__).resolve().parents[1] / 'src/cli/internal/viewerassets/viewer.zip'
with ZipFile(out, 'w', compression=ZIP_DEFLATED) as archive:
    for name, content in sorted(files.items()):
        info = ZipInfo(name, date_time=(1980, 1, 1, 0, 0, 0))
        info.compress_type = ZIP_DEFLATED
        info.external_attr = 0o100644 << 16
        archive.writestr(info, content)
print(f'Packaged {len(files) - 1} viewer assets ({out.stat().st_size} bytes)')
