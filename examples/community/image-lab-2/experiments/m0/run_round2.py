#!/usr/bin/env python3
"""M0 formal round 2: revised vocabulary, full 3x3 grids, blind test.

Text-only on GPT Image 2.5 Sunburst and Nano Banana 2, reference on Nano Banana.
Camera `low angle` uses a milder wording when a reference is present (M0 finding).

  R2_MODE=pricecheck python run_round2.py dry-run|generate   # 1 image, price check
  python run_round2.py dry-run | generate --confirm <fp> | blind | score | grids
"""
import os
import sys
from pathlib import Path

import run_m0 as m

HERE = Path(__file__).resolve().parent
N2 = os.environ.get("R2_SECOND_MODEL", "google/gemini-3.1-flash-image")   # second text model
MODE = os.environ.get("R2_MODE", "full")
SECOND_NAME = os.environ.get("R2_SECOND_NAME", "Nano Banana 2")

m.LIGHTING = {
    "soft daylight": "Soft, even, diffused daylight from a window, gentle shadows",
    "golden hour": ("Deep orange-amber golden-hour sunlight from low on the horizon at the side, "
                    "a strong warm orange color cast over the whole scene, glowing orange rim light "
                    "on the edges, long soft shadows"),
    "dramatic spotlight": ("Hard directional spotlight from the side at counter height, strong contrast "
                           "and deep dark shadows, moody"),
}
m.CAMERA = {
    "eye-level front": "Eye-level front view, the camera straight on at the product",
    "three-quarter high": ("Three-quarter view from clearly above: the camera is raised and looks down "
                           "at about 35 degrees, so the top surface of the counter is clearly visible"),
    "low angle": ("Extreme low angle: the camera sits on the counter surface pointing sharply upward, "
                  "the products tower above the lens with strong perspective foreshortening, "
                  "the ceiling or upper wall is visible behind them"),
}
CAMERA_REF = dict(m.CAMERA)                      # per-mode wording: milder low angle with a reference
CAMERA_REF["low angle"] = "Low camera position at counter level, looking slightly upward at the standing products"
m.REF_TEMPLATE = (
    "Use the reference image only to identify the product: keep the cream refill bottle with its teal "
    "\"MIGHTY Toothpaste Refill\" label and the mint-green dispenser unchanged in shape, color, branding "
    "and proportions. Re-photograph them from the camera angle described below. The products stand "
    "upright on a pale stone bathroom counter, exactly as they would in real life; do not copy the tilt, "
    "rotation, floating pose or viewpoint of the reference image. {camera}. {lighting}. "
    "Commercial product photograph, realistic, square composition."
)
if MODE == "pricecheck":
    m.GROUPS = {"c-n2": ("A", N2, "Nano Banana 2 (price check)")}
    DEFAULT_OUT = HERE / "out-r2-pricecheck"
else:
    m.GROUPS = {
        "r2-sb": ("A", m.SB, "GPT Image 2.5 Sunburst"),
        "r2-n2": ("A", N2, SECOND_NAME),
        "r2-nb": ("B", m.NB, "Nano Banana (reference)"),
    }
    DEFAULT_OUT = HERE / "out-r2"


def build_rows():
    rows = []
    for key, (leg, model, _name) in m.GROUPS.items():
        n = 0
        for lk in (m.LIGHTING if MODE == "full" else ["soft daylight"]):
            for ck in (m.CAMERA if MODE == "full" else ["eye-level front"]):
                n += 1
                cam = (CAMERA_REF if leg == "B" else m.CAMERA)[ck]
                tmpl = m.TEXT_TEMPLATE if leg == "A" else m.REF_TEMPLATE
                rows.append({"id": f"{key}-{n}", "group": key, "leg": leg, "model": model,
                             "lighting": lk, "camera": ck,
                             "prompt": tmpl.format(camera=cam, lighting=m.LIGHTING[lk]),
                             "reference": "refs/mighty-product.png" if leg == "B" else None})
    return rows


m.build_rows = build_rows

if __name__ == "__main__":
    if "--out" not in sys.argv:
        sys.argv[1:1] = ["--out", str(DEFAULT_OUT)]
    m.main()
