#!/usr/bin/env python3
"""pack.py — recompress a built case-study folder's images for the web.

`build-exploration-page.py` is standard-library only, so it emits the run's
original PNGs. Before a case study is committed, run this once to swap them for
web-weight JPEG and rewrite every reference (index.html + exploration.json).

    python case-studies/pack.py case-studies/<slug>

Policy for every committed case study:
  - JPEG, **quality 95** (visually lossless for line-art + text; a full run
    lands ~3-4 MB, in the same range as redesign-lab/case-studies/)
  - progressive, optimized
  - full resolution kept; only downscaled if a side exceeds MAX_EDGE px

Needs Pillow (`pip install Pillow`). This is a maintainer tool — it is NOT part
of the skill runtime and nothing under scripts/ imports it.
"""
from __future__ import annotations

import sys
from pathlib import Path

QUALITY = 95
MAX_EDGE = 2048  # only downscale images larger than this on their long edge


def main() -> None:
    if len(sys.argv) != 2:
        sys.exit("usage: python case-studies/pack.py case-studies/<slug>")
    try:
        from PIL import Image
    except ImportError:
        sys.exit("pack.py needs Pillow:  pip install Pillow")

    folder = Path(sys.argv[1]).resolve()
    assets = folder / "assets"
    index = folder / "index.html"
    data = folder / "exploration.json"
    if not (assets.is_dir() and index.exists()):
        sys.exit(f"{folder} is not a built case-study folder (need assets/ + index.html)")

    pngs = sorted(assets.glob("*.png"))
    if not pngs:
        print("no PNG assets to pack (already done?)")
        return

    html = index.read_text(encoding="utf-8")
    js = data.read_text(encoding="utf-8") if data.exists() else None
    total = 0
    for png in pngs:
        im = Image.open(png).convert("RGB")
        longest = max(im.size)
        if longest > MAX_EDGE:
            s = MAX_EDGE / longest
            im = im.resize((round(im.width * s), round(im.height * s)), Image.LANCZOS)
        jpg = png.with_suffix(".jpg")
        im.save(jpg, format="JPEG", quality=QUALITY, optimize=True, progressive=True)
        total += jpg.stat().st_size
        png.unlink()
        html = html.replace(png.name, jpg.name)
        if js is not None:
            js = js.replace(png.name, jpg.name)
        print(f"  {png.name} -> {jpg.name}  {im.size[0]}x{im.size[1]}  {jpg.stat().st_size // 1024} KB")

    index.write_text(html, encoding="utf-8")
    if js is not None:
        data.write_text(js, encoding="utf-8")
    print(f"packed {len(pngs)} image(s) at JPEG q{QUALITY} - {total / 1_000_000:.2f} MB total")


if __name__ == "__main__":
    main()
