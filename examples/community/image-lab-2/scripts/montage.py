"""Montage: a labelled grid of an experiment's finished images, one grid per value of a dimension (free, needs Pillow).

    python scripts/image.py montage --dir <experiment> --by direction [--cols 6] [--thumb 320] [--blind] [--out <folder>]

One JPEG per value of `--by` (for example one per creative direction), so the first look at a calibration batch takes one glance
per direction. `--blind` writes a single grid in a fixed shuffled order with neutral labels (S01...) and a key file that maps
them back, for scoring images without knowing which direction or model made them."""
from __future__ import annotations

import json
import random
import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))
import ledger as lg  # noqa: E402
import review as rv  # noqa: E402
import sheet as sh  # noqa: E402


class MontageError(Exception):
    pass


def _images(exp: Path, ledger: dict) -> list[dict]:
    """Completed attempts whose file is a safe image inside the experiment, with their row's values."""
    rows = {r["id"]: r for r in ledger["rows"]}
    out = []
    for a in ledger["attempts"]:
        if a.get("status") != "Completed":
            continue
        p = sh._safe_image(exp, a.get("file"))
        if p is None or not p.exists():
            continue
        row = rows.get(a["row_id"])
        out.append({"id": a.get("sample_id") or a["row_id"], "params": (row or {}).get("params", a.get("params", {})), "path": p,
                    "review": (row or {}).get("review")})
    return out


def _grid(items: list[dict], labels: list[str], cols: int, thumb: int, title: str | None):
    from PIL import Image, ImageDraw
    cells = []
    for it in items:
        im = Image.open(it["path"]).convert("RGB")
        im.thumbnail((thumb, int(thumb * 1.5)))
        cells.append(im)
    cw, ch = thumb, max(c.height for c in cells) + 18
    cols = max(1, min(cols, len(cells)))
    rows_n = -(-len(cells) // cols)
    head = 26 if title else 0
    sheet = Image.new("RGB", (cols * cw + 8, rows_n * ch + head + 8), (245, 243, 238))
    d = ImageDraw.Draw(sheet)
    if title:
        d.text((8, 6), title, fill=(30, 30, 30))
    for i, (im, label) in enumerate(zip(cells, labels)):
        x, y = 8 + (i % cols) * cw, head + 4 + (i // cols) * ch
        sheet.paste(im, (x, y + 14))
        d.text((x + 2, y), label, fill=(60, 60, 60))
    return sheet


READABLE = {"cols": 3, "thumb": 576}          # tiles big enough to read small type (a headline reads at 320 px, a supporting line does not)


def _natural(value) -> list:
    """'10 x' sorts after '2 x': numbers compare as numbers."""
    import re
    return [int(t) if t.isdigit() else t.lower() for t in re.split(r"(\d+)", str(value))]


def build(exp_dir, by: str | None = None, cols: int = 6, thumb: int = 320, blind: bool = False, out_dir=None, seed: int = 20261008,
          readable: bool = False) -> list[Path]:
    if readable:
        cols, thumb = READABLE["cols"], READABLE["thumb"]
    try:
        import PIL  # noqa: F401
    except ImportError as e:
        raise MontageError("montage needs Pillow (pip install Pillow)") from e
    exp = Path(exp_dir)
    if not (exp / "ledger.json").exists():
        raise MontageError(f"{exp} has no ledger.json")
    ledger = lg.load(exp / "ledger.json")
    names = ledger["experiment"]["dimensions"]
    items = _images(exp, ledger)
    if not items:
        raise MontageError("no finished images yet")
    out = Path(out_dir) if out_dir else exp / "montage"
    out.mkdir(parents=True, exist_ok=True)
    made: list[Path] = []
    if blind:
        random.Random(seed).shuffle(items)
        labels = [f"S{i:02d}" for i in range(1, len(items) + 1)]
        p = out / "blind.jpg"
        _grid(items, labels, cols, thumb, None).save(p, "JPEG", quality=88)
        key = {lab: {"id": it["id"], "params": it["params"]} for lab, it in zip(labels, items)}
        (out / "blind-key.json").write_text(json.dumps(key, indent=1, ensure_ascii=False), encoding="utf-8")
        return [p, out / "blind-key.json"]
    if by is not None and by not in names:
        raise MontageError(f"--by {by!r} is not a dimension of this experiment (they are: {', '.join(names)})")
    groups: dict = {}
    for it in items:
        groups.setdefault(it["params"].get(by, "all") if by else "all", []).append(it)
    used: set = set()
    for i, value in enumerate(sorted(groups, key=_natural), 1):                  # the groups in their own order (direction 1, 2, ... 10), not the order they first finished
        grp = sorted(groups[value], key=lambda it: _natural(it["id"]))
        safe = "".join(c if c.isalnum() or c in "-_" else "-" for c in str(value)).strip("-")[:40] or f"g{i}"
        name = f"{by or 'all'}-{safe}.jpg"                                        # the value already carries its number ("3 Architectural aisle")
        if name in used:
            name = f"{by or 'all'}-{i}-{safe}.jpg"
        used.add(name)
        p = out / name
        labels = [(it["id"] + (" " + rv.mark(it.get("review")) if it.get("review") else "")) for it in grp]
        _grid(grp, labels, cols, thumb, f"{by or 'all'}: {value}   ({len(grp)} images)").save(p, "JPEG", quality=88)
        made.append(p)
    return made


def add_args(ap) -> None:
    ap.add_argument("--dir", required=True, help="the experiment folder")
    ap.add_argument("--by", default=None, help="a dimension of the experiment: one grid per value of it (for example direction); without it, one grid of everything")
    ap.add_argument("--cols", type=int, default=6, help="images per row in each grid (default 6)")
    ap.add_argument("--thumb", type=int, default=320, help="thumbnail width in pixels")
    ap.add_argument("--blind", action="store_true", help="one shuffled grid with neutral labels S01... and a key file")
    ap.add_argument("--readable", action="store_true", help="bigger tiles (3 across, 576 px) so small type can be read; overrides --cols and --thumb")
    ap.add_argument("--out", default=None, help="output folder (default <dir>/montage)")


def run(a) -> int:
    try:
        made = build(a.dir, a.by, a.cols, a.thumb, a.blind, a.out, readable=a.readable)
    except MontageError as e:
        print(f"error: {e}", file=sys.stderr)
        return 2
    for p in made:
        print(f"montage: {p}")
    return 0
