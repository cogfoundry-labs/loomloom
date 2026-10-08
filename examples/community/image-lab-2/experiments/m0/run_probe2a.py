#!/usr/bin/env python3
"""M0 round 2, step 2a: make an upright reference from the supplied (tilted) photo.

One image on Nano Banana. Afterwards `finish` copies it to ref/mighty-upright.png
(git-ignored) for the step 2b re-test.

  python run_probe2a.py dry-run
  python run_probe2a.py generate --confirm <fingerprint>
  python run_probe2a.py finish
"""
import shutil
import sys
from pathlib import Path

import run_m0 as m

HERE = Path(__file__).resolve().parent
OUT = HERE / "out-probe2a"

PROMPT = (
    "Using the reference image, show the same two products standing upright side by side on a plain "
    "white seamless studio background, in a straight-on front view, with soft even studio light. "
    "Keep the cream refill bottle with its teal \"MIGHTY Toothpaste Refill\" label and the mint-green "
    "dispenser unchanged in shape, color, branding and proportions. Both products stand perfectly "
    "vertical on the ground, not tilted, not rotated, not floating. Clean commercial product photograph, "
    "realistic, square composition."
)
m.GROUPS = {"u-nb": ("B", m.NB, "Nano Banana (upright reference)")}


def build_rows():
    return [{"id": "u-nb-1", "group": "u-nb", "leg": "B", "model": m.NB,
             "lighting": "studio", "camera": "front", "prompt": PROMPT, "reference": "refs/mighty-product.png"}]


m.build_rows = build_rows

if __name__ == "__main__":
    if len(sys.argv) > 1 and sys.argv[1] == "finish":
        from PIL import Image
        src = OUT / "images" / "u-nb-1.png"
        dst = HERE / "ref" / "mighty-upright.png"
        Image.open(src).convert("RGB").save(dst, optimize=True)
        print(f"saved {dst}")
    else:
        if "--out" not in sys.argv:
            sys.argv[1:1] = ["--out", str(OUT)]
        m.main()
