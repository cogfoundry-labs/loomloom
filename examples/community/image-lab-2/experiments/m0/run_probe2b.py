#!/usr/bin/env python3
"""M0 round 2, step 2b: re-test `low angle` with an upright reference.

The upright reference is a generated image from probe 1 (products standing on a
counter, labels legible), saved as ref/mighty-upright.png. 3 images on Nano Banana:
  1. upright reference + strong low-angle wording
  2. upright reference + milder low-angle wording
  3. original tilted reference + milder wording

  python run_probe2b.py dry-run | generate --confirm <fp> | sheet
"""
import sys
from pathlib import Path

import run_m0 as m

HERE = Path(__file__).resolve().parent
OUT = HERE / "out-probe2b"

LIGHT = "Soft, even, diffused daylight from a window, gentle shadows"
STRONG = ("Extreme low angle: the camera sits on the counter surface pointing sharply upward, the products "
          "tower above the lens with strong perspective foreshortening, the ceiling or upper wall is "
          "visible behind them")
MILD = "Low camera position at counter level, looking slightly upward at the standing products"

TPL_UPRIGHT = (
    "The reference image shows the exact products standing upright on a counter. Keep the cream refill "
    "bottle with its teal \"MIGHTY Toothpaste Refill\" label and the mint-green dispenser unchanged in "
    "shape, color, branding and proportions, and keep them standing upright on the counter exactly as in "
    "the reference. Only the camera position changes: re-photograph the same scene from the camera angle "
    "below. Do not tilt, rotate or float the products. {camera}. {light}. Commercial product photograph, "
    "realistic, square composition."
)
TPL_TILTED = (
    "Use the reference image only to identify the product: keep the cream refill bottle with its teal "
    "\"MIGHTY Toothpaste Refill\" label and the mint-green dispenser unchanged in shape, color, branding "
    "and proportions. Re-photograph them from the camera angle described below. The products stand "
    "upright on a pale stone bathroom counter, exactly as they would in real life; do not copy the tilt, "
    "rotation, floating pose or viewpoint of the reference image. {camera}. {light}. "
    "Commercial product photograph, realistic, square composition."
)
m.GROUPS = {"p2-nb": ("B", m.NB, "Nano Banana (reference, probe 2b)")}


def build_rows():
    spec = [("upright ref + strong wording", "ref/mighty-upright.png", TPL_UPRIGHT, STRONG),
            ("upright ref + mild wording", "ref/mighty-upright.png", TPL_UPRIGHT, MILD),
            ("tilted ref + mild wording", "ref/mighty-product.png", TPL_TILTED, MILD)]
    return [{"id": f"p2-nb-{i}", "group": "p2-nb", "leg": "B", "model": m.NB,
             "lighting": "soft daylight", "camera": label, "ref_file": ref,
             "prompt": tpl.format(camera=cam, light=LIGHT), "reference": ref}
            for i, (label, ref, tpl, cam) in enumerate(spec, 1)]


m.build_rows = build_rows


def sheet():
    import json
    from PIL import Image, ImageDraw
    ledger = json.loads((OUT / "ledger.json").read_text(encoding="utf-8"))["tasks"]
    rows = build_rows()
    cell = 420
    s = Image.new("RGB", (cell * 4 + 10, cell + 36), "white")
    d = ImageDraw.Draw(s)
    refs = [("reference: upright (from probe 1)", HERE / "ref" / "mighty-upright.png")]
    x = 0
    d.text((x + 6, 10), refs[0][0], fill="black")
    im = Image.open(refs[0][1]).convert("RGB"); im.thumbnail((cell - 6, cell - 6)); s.paste(im, (x + 3, 36))
    for i, r in enumerate(rows, 1):
        x = i * cell
        d.text((x + 6, 10), r["camera"], fill="black")
        f = ledger.get(r["id"], {}).get("file")
        if f and Path(f).exists():
            im = Image.open(f).convert("RGB"); im.thumbnail((cell - 6, cell - 6)); s.paste(im, (x + 3, 36))
    p = OUT / "probe2b-sheet.png"
    s.save(p)
    print(f"sheet: {p}")


if __name__ == "__main__":
    if len(sys.argv) > 1 and sys.argv[1] == "sheet":
        m.need_libs()
        sheet()
    else:
        if "--out" not in sys.argv:
            sys.argv[1:1] = ["--out", str(OUT)]
        m.main()
