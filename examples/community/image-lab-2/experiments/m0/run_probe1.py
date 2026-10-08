#!/usr/bin/env python3
"""M0 round 2, step 1: wording-only probe (see m0-plan.md section 10).

Reuses run_m0.py (dry-run / generate) with the revised wording. Not a blind test:
the reviewer looks at the images. 9 images:
  - 6 text-only on GPT Image 2.5 Sunburst: {soft daylight, golden hour} x 3 cameras
  - 3 with the (tilted) Mighty reference on Nano Banana: soft daylight x 3 cameras

  python run_probe1.py dry-run
  python run_probe1.py generate --confirm <fingerprint>
  python run_probe1.py sheet        # contact sheet of the results
"""
import sys
from pathlib import Path

import run_m0 as m

HERE = Path(__file__).resolve().parent

m.LIGHTING = {
    "soft daylight": "Soft, even, diffused daylight from a window, gentle shadows",
    "golden hour": ("Deep orange-amber golden-hour sunlight from low on the horizon at the side, "
                    "a strong warm orange color cast over the whole scene, glowing orange rim light "
                    "on the edges, long soft shadows"),
}
m.CAMERA = {
    "eye-level front": "Eye-level front view, the camera straight on at the product",
    "three-quarter high": ("Three-quarter view from clearly above: the camera is raised and looks down "
                           "at about 35 degrees, so the top surface of the counter is clearly visible"),
    "low angle": ("Extreme low angle: the camera sits on the counter surface pointing sharply upward, "
                  "the products tower above the lens with strong perspective foreshortening, "
                  "the ceiling or upper wall is visible behind them"),
}
m.REF_TEMPLATE = (
    "Use the reference image only to identify the product: keep the cream refill bottle with its teal "
    "\"MIGHTY Toothpaste Refill\" label and the mint-green dispenser unchanged in shape, color, branding "
    "and proportions. Re-photograph them from the camera angle described below. The products stand "
    "upright on a pale stone bathroom counter, exactly as they would in real life; do not copy the tilt, "
    "rotation, floating pose or viewpoint of the reference image. {camera}. {lighting}. "
    "Commercial product photograph, realistic, square composition."
)
m.GROUPS = {
    "p-sb": ("A", m.SB, "GPT Image 2.5 Sunburst (text, probe)"),
    "p-nb": ("B", m.NB, "Nano Banana (reference, probe)"),
}


def build_rows():
    rows = []
    n = 0
    for lk in ("soft daylight", "golden hour"):
        for ck in m.CAMERA:
            n += 1
            rows.append({"id": f"p-sb-{n}", "group": "p-sb", "leg": "A", "model": m.SB,
                         "lighting": lk, "camera": ck,
                         "prompt": m.TEXT_TEMPLATE.format(camera=m.CAMERA[ck], lighting=m.LIGHTING[lk]),
                         "reference": None})
    n = 0
    for ck in m.CAMERA:
        n += 1
        rows.append({"id": f"p-nb-{n}", "group": "p-nb", "leg": "B", "model": m.NB,
                     "lighting": "soft daylight", "camera": ck,
                     "prompt": m.REF_TEMPLATE.format(camera=m.CAMERA[ck], lighting=m.LIGHTING["soft daylight"]),
                     "reference": "refs/mighty-product.png"})
    return rows


m.build_rows = build_rows


def sheet(out: Path):
    import json
    from PIL import Image, ImageDraw
    ledger = json.loads((out / "ledger.json").read_text(encoding="utf-8"))["tasks"]
    snap = sorted((out / "snapshots").glob("*.json"), key=lambda p: p.stat().st_mtime)[-1]
    rows = {r["id"]: r for r in json.loads(snap.read_text(encoding="utf-8"))["rows"]}
    cell = 380
    cams = list(m.CAMERA)
    lines = [("p-sb", "soft daylight", "text / Sunburst: soft daylight"),
             ("p-sb", "golden hour", "text / Sunburst: golden hour"),
             ("p-nb", "soft daylight", "reference / Nano Banana: soft daylight")]
    sheet_img = Image.new("RGB", (190 + cell * 3, 36 + cell * 3), "white")
    d = ImageDraw.Draw(sheet_img)
    for ci, ck in enumerate(cams):
        d.text((190 + ci * cell + 8, 12), ck, fill="black")
    for li, (g, lk, label) in enumerate(lines):
        d.text((6, 36 + li * cell + cell // 2), label, fill="black")
        for ci, ck in enumerate(cams):
            tid = next((i for i, r in rows.items() if r["group"] == g and r["lighting"] == lk and r["camera"] == ck), None)
            f = ledger.get(tid, {}).get("file") if tid else None
            if f and Path(f).exists():
                im = Image.open(f).convert("RGB")
                im.thumbnail((cell - 6, cell - 6))
                sheet_img.paste(im, (190 + ci * cell + 3, 36 + li * cell + 3))
    p = out / "probe1-sheet.png"
    sheet_img.save(p)
    print(f"sheet: {p}")


if __name__ == "__main__":
    if "--out" not in sys.argv:
        sys.argv[1:1] = ["--out", str(HERE / "out-probe1")]
    if len(sys.argv) > 3 and sys.argv[3] == "sheet":
        m.need_libs()
        sheet(Path(sys.argv[2]))
    else:
        m.main()
