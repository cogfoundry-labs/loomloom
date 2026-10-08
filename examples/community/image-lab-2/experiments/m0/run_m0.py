#!/usr/bin/env python3
"""M0 controls experiment harness (see m0-plan.md).

  dry-run   print every request, the cost estimate and a fingerprint; write the
            execution snapshot out/snapshots/<fingerprint>.json. Spends nothing.
  generate  --confirm <fingerprint>: read ONLY that snapshot and generate it.
  blind     build the shuffled blind-test workbook + key from the finished images.
  score     read the filled workbook and score it against the key.
  grids     build unblinded comparison grids for the qualitative review.

--out <dir> overrides the output folder (used for offline tests).
Needs XlsxWriter, openpyxl and Pillow for blind/score/grids; if they are not
installed, set IMAGELAB_M0_LIBS to a folder that contains them.
"""
from __future__ import annotations

import argparse
import base64
import hashlib
import importlib.util
import json
import os
import random
import sys
import threading
import time
from concurrent.futures import ThreadPoolExecutor
from pathlib import Path

HERE = Path(__file__).resolve().parent
ROOT = HERE.parent.parent                      # image-lab-2/
REF_FILE = HERE / "ref" / "mighty-product.png"
SEED = 20261007
CONCURRENCY = 6
POLL_SECONDS = 4
POLL_TIMEOUT = 480

NB = "google/gemini-2.5-flash-image"           # Nano Banana
SB = "openai/gpt-image-2.5-sunburst"           # GPT Image 2.5 Sunburst
REF_PRICE_NB = 0.039                           # measured earlier, reference call on Nano Banana

LIGHTING = {
    "soft daylight": "Soft, even, diffused daylight from a window, gentle shadows",
    "golden hour": "Warm low golden-hour sunlight from the side, long soft shadows, amber glow",
    "dramatic spotlight": "Hard directional spotlight with strong contrast and deep dark shadows, moody",
}
CAMERA = {
    "eye-level front": "Eye-level front view, the camera straight on at the product",
    "three-quarter high": "Three-quarter view from slightly above, looking down at about 30 degrees",
    "low angle": "Low angle from near counter level, looking up at the product",
}
TEXT_TEMPLATE = (
    "A clean commercial product photograph of a pastel mint-green pump dispenser bottle "
    "standing next to a cream-colored toothpaste refill bottle with a silver cap, on a pale "
    "stone bathroom counter. {camera}. {lighting}. Sharp focus, realistic, square "
    "composition, no text overlays."
)
REF_TEMPLATE = (
    "Use the reference image as the exact product: keep the cream refill bottle with its teal "
    "\"MIGHTY Toothpaste Refill\" label and the mint-green dispenser unchanged in shape, color, "
    "branding and proportions. Place them on a pale stone bathroom counter. {camera}. "
    "{lighting}. Commercial product photograph, realistic, square composition."
)
GROUPS = {  # key -> (leg, model, short name)
    "A-nb": ("A", NB, "Nano Banana"),
    "A-sb": ("A", SB, "GPT Image 2.5 Sunburst"),
    "B-nb": ("B", NB, "Nano Banana (reference)"),
}
TERMINAL = ("COMPLETED", "FAILED")


# --------------------------------------------------------------------------- #
def load_image_lab():
    spec = importlib.util.spec_from_file_location("image", ROOT / "scripts" / "image.py")
    mod = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(mod)
    return mod


def need_libs():
    libs = os.environ.get("IMAGELAB_M0_LIBS")
    if libs:
        sys.path.insert(0, libs)
    try:
        import openpyxl, PIL, xlsxwriter  # noqa: F401
    except ImportError as e:
        sys.exit(f"missing library ({e.name}). pip install XlsxWriter openpyxl Pillow, "
                 f"or set IMAGELAB_M0_LIBS to a folder that has them.")


def sha256_bytes(b: bytes) -> str:
    return hashlib.sha256(b).hexdigest()


def write_json_atomic(path: Path, data) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    tmp = path.with_suffix(path.suffix + ".tmp")
    tmp.write_text(json.dumps(data, indent=2, ensure_ascii=False), encoding="utf-8")
    os.replace(tmp, path)


# --------------------------------------------------------------------------- #
def build_rows() -> list[dict]:
    rows = []
    for key, (leg, model, _name) in GROUPS.items():
        n = 0
        for lk, lv in LIGHTING.items():
            for ck, cv in CAMERA.items():
                n += 1
                tmpl = TEXT_TEMPLATE if leg == "A" else REF_TEMPLATE
                rows.append({
                    "id": f"{key}-{n}", "group": key, "leg": leg, "model": model,
                    "lighting": lk, "camera": ck,
                    "prompt": tmpl.format(camera=CAMERA[ck], lighting=LIGHTING[lk]),
                    "reference": "refs/mighty-product.png" if leg == "B" else None,
                })
    return rows


def price_for(row: dict, catalog: dict) -> float:
    if row["leg"] == "B":
        return REF_PRICE_NB
    return float(catalog[row["model"]]["usd_per_image"])


def ref_path(row: dict) -> Path:
    """A row may name its own reference file (relative to experiments/m0); default is the Mighty photo."""
    return HERE / row["ref_file"] if row.get("ref_file") else REF_FILE


def make_snapshot(rows: list[dict], catalog: dict) -> dict:
    body_rows = []
    for r in rows:
        ref_sha = sha256_bytes(ref_path(r).read_bytes()) if r["leg"] == "B" and ref_path(r).exists() else None
        body_rows.append({k: r[k] for k in ("id", "group", "leg", "model", "lighting", "camera", "prompt")}
                         | {"aspect_ratio": "1:1" if catalog[r["model"]]["request"]["aspect_ratio_param"] else None,
                            "ref_file": r.get("ref_file"),
                            "reference_sha256": ref_sha,
                            "est_usd": price_for(r, catalog)})
    canon = json.dumps(body_rows, sort_keys=True, separators=(",", ":"))
    fp = sha256_bytes(canon.encode())[:12]
    return {"fingerprint": fp, "created_at": time.strftime("%Y-%m-%dT%H:%M:%S"),
            "estimated_usd": round(sum(r["est_usd"] for r in body_rows), 4), "rows": body_rows}


def request_body(row: dict, catalog: dict, with_image: bool) -> dict:
    body = {"model": row["model"], "prompt": row["prompt"]}
    if row.get("aspect_ratio"):
        body["aspect_ratio"] = row["aspect_ratio"]
    if row["leg"] == "B":
        if with_image:
            b64 = base64.b64encode(ref_path(row).read_bytes()).decode()
            body["image"] = "data:image/png;base64," + b64
        else:
            body["image"] = "<reference image: refs/mighty-product.png as base64 data URI>"
    return body


# --------------------------------------------------------------------------- #
def cmd_dry_run(a):
    il = load_image_lab()
    catalog = il.load_model_catalog()
    if not REF_FILE.exists():
        sys.exit(f"missing {REF_FILE}")
    rows = build_rows()
    snap = make_snapshot(rows, catalog)
    out = Path(a.out)
    write_json_atomic(out / "snapshots" / f"{snap['fingerprint']}.json", snap)
    print("M0 DRY RUN  (nothing is sent, nothing is spent)\n")
    print(f"{'id':<8}{'leg':<4}{'model':<34}{'lighting':<20}{'camera':<20}est")
    for r in snap["rows"]:
        print(f"{r['id']:<8}{r['leg']:<4}{r['model']:<34}{r['lighting']:<20}{r['camera']:<20}${r['est_usd']:.4f}")
    print("\nSample request bodies (image elided):")
    first_of_group = {}
    for r in snap["rows"]:
        first_of_group.setdefault(r["group"], r["id"])
    for gid in first_of_group.values():
        row = next(r for r in snap["rows"] if r["id"] == gid)
        print(f"\n  [{gid}] " + json.dumps(request_body(row, catalog, with_image=False), indent=2, ensure_ascii=False)
              .replace("\n", "\n  "))
    legs = {}
    for r in snap["rows"]:
        legs.setdefault(r["group"], []).append(r["est_usd"])
    print("\nEstimated cost:")
    for g, v in legs.items():
        print(f"  {GROUPS[g][2]:<26} {len(v)} images  ${sum(v):.4f}")
    print(f"  {'TOTAL':<26} {len(snap['rows'])} images  ${snap['estimated_usd']:.4f}")
    print("  (reference leg uses the price measured on an earlier test, $0.039/image; not guaranteed)")
    print(f"\nFingerprint: {snap['fingerprint']}")
    print(f"Snapshot:    {out / 'snapshots' / (snap['fingerprint'] + '.json')}")
    print(f"To spend, run: python {Path(sys.argv[0]).name} generate --confirm {snap['fingerprint']}")


# --------------------------------------------------------------------------- #
class Ledger:
    def __init__(self, path: Path):
        self.path, self.lock = path, threading.Lock()
        self.data = json.loads(path.read_text(encoding="utf-8")) if path.exists() else {"tasks": {}}

    def update(self, tid: str, **kw):
        with self.lock:
            rec = self.data["tasks"].setdefault(tid, {})
            rec.update(kw)
            write_json_atomic(self.path, self.data)

    def get(self, tid: str) -> dict:
        with self.lock:
            return dict(self.data["tasks"].get(tid, {}))


def cmd_generate(a):
    il = load_image_lab()
    out = Path(a.out)
    snap_path = out / "snapshots" / f"{a.confirm}.json"
    if not snap_path.exists():
        sys.exit(f"no snapshot for fingerprint {a.confirm!r}. Run dry-run and use its fingerprint.")
    snap = json.loads(snap_path.read_text(encoding="utf-8"))
    canon = json.dumps([{k: v for k, v in r.items()} for r in snap["rows"]], sort_keys=True, separators=(",", ":"))
    if sha256_bytes(canon.encode())[:12] != snap["fingerprint"] or snap["fingerprint"] != a.confirm:
        sys.exit("snapshot does not match its fingerprint; refusing to run.")
    for r in snap["rows"]:
        if r["leg"] == "B":
            p = ref_path(r)
            if not p.exists() or sha256_bytes(p.read_bytes()) != r["reference_sha256"]:
                sys.exit(f"reference image {p.name} changed since the snapshot was approved; refusing to run.")
    catalog = il.load_model_catalog()
    tok = il.token()
    ledger = Ledger(out / "ledger.json")
    abort = threading.Event()
    why = {"reason": ""}
    (out / "images").mkdir(parents=True, exist_ok=True)

    def work(r):
        tid = r["id"]
        rec = ledger.get(tid)
        if rec.get("status") in ("COMPLETED", "UNKNOWN") or abort.is_set() and not rec.get("rid"):
            return                                   # hard rule: UNKNOWN is never auto-retried
        if not rec.get("rid"):
            body = request_body(r, catalog, with_image=True)
            st, resp, err = il._req("POST", "/tasks/generations", tok, body)
            sub = time.time()
            rid = ((resp or {}).get("data") or {}).get("request_id")
            base = {"submitted_at": sub, "body_sha256": sha256_bytes(json.dumps(body, sort_keys=True).encode()),
                    "model": r["model"]}
            if rid:
                ledger.update(tid, rid=rid, status="SUBMITTING", **base)       # request id recorded first
            elif st in (0, 504):
                ledger.update(tid, status="UNKNOWN", error=err or f"HTTP {st}", **base)
                print(f"  {tid}: UNKNOWN (no response; may or may not be billed)", flush=True)
                return
            else:
                if st in (401, 402, 403):
                    why["reason"] = "an authentication or balance rejection"
                    abort.set()
                ledger.update(tid, status="FAILED", error=json.dumps(resp)[:200] or err, **base)
                print(f"  {tid}: FAILED at submit (HTTP {st})", flush=True)
                return
        rec = ledger.get(tid)
        deadline = time.time() + POLL_TIMEOUT
        while time.time() < deadline:
            _, resp, err = il._req("GET", f"/tasks/generations/{rec['rid']}", tok)
            d = (resp or {}).get("data") or {}
            status = d.get("status", "")
            if d.get("cost") is not None:
                ledger.update(tid, cost=float(d["cost"]))
                spent = sum(float(t.get("cost") or 0) for t in ledger.data["tasks"].values())
                if spent > a.max_usd and not abort.is_set():
                    why["reason"] = f"budget guard (--max-usd ${a.max_usd:.2f}); tasks already running still billed"
                    abort.set()                      # budget guard: no NEW submissions
                    print(f"  budget guard: ${spent:.4f} exceeds --max-usd ${a.max_usd:.2f}; "
                          f"no further tasks will be submitted", flush=True)
            if status == "COMPLETED":
                urls = (d.get("data") or {}).get("image_urls") or []
                dest = out / "images" / f"{tid}.png"
                try:
                    il._download(urls[0], dest)
                    ledger.update(tid, status="COMPLETED", file=str(dest), seconds=round(time.time() - rec["submitted_at"], 1))
                    print(f"  {tid}: done  ${float(d.get('cost') or 0):.4f}", flush=True)
                except Exception as e:  # noqa: BLE001
                    ledger.update(tid, status="COMPLETED", file=None, error=f"download: {e}")
                    print(f"  {tid}: completed but download failed: {e}", flush=True)
                return
            if status == "FAILED":
                ledger.update(tid, status="FAILED", error=d.get("fail_reason", ""))
                print(f"  {tid}: FAILED ({d.get('fail_reason', '')})", flush=True)
                return
            time.sleep(POLL_SECONDS)
        print(f"  {tid}: still running after {POLL_TIMEOUT}s; re-run generate to resume polling", flush=True)

    print(f"GENERATE  {len(snap['rows'])} images, concurrency {CONCURRENCY}, est ${snap['estimated_usd']:.4f}", flush=True)
    with ThreadPoolExecutor(max_workers=CONCURRENCY) as ex:
        list(ex.map(work, snap["rows"]))
    tasks = ledger.data["tasks"]
    done = sum(t.get("status") == "COMPLETED" for t in tasks.values())
    total = sum(float(t.get("cost") or 0) for t in tasks.values())
    print(f"\nDONE  {done}/{len(snap['rows'])} completed, actual total ${total:.4f} "
          f"(estimated ${snap['estimated_usd']:.4f})", flush=True)
    for g in GROUPS:
        gs = [t for k, t in tasks.items() if k.startswith(g + "-")]
        print(f"  {GROUPS[g][2]:<26} ${sum(float(t.get('cost') or 0) for t in gs):.4f}", flush=True)
    if abort.is_set():
        print(f"NOTE: stopped submitting after {why['reason']}.", flush=True)


# --------------------------------------------------------------------------- #
def load_done(out: Path, snap_rows: dict) -> list[dict]:
    ledger = json.loads((out / "ledger.json").read_text(encoding="utf-8"))["tasks"]
    items = []
    for tid, t in ledger.items():
        if t.get("status") == "COMPLETED" and t.get("file") and Path(t["file"]).exists():
            items.append({"id": tid, **snap_rows[tid], "file": t["file"]})
    return items


def newest_snapshot_rows(out: Path) -> dict:
    snaps = sorted((out / "snapshots").glob("*.json"), key=lambda p: p.stat().st_mtime)
    if not snaps:
        sys.exit("no snapshot found")
    return {r["id"]: r for r in json.loads(snaps[-1].read_text(encoding="utf-8"))["rows"]}


def cmd_blind(a):
    need_libs()
    import xlsxwriter
    from PIL import Image
    out = Path(a.out)
    items = load_done(out, newest_snapshot_rows(out))
    if not items:
        sys.exit("no completed images found")
    random.Random(SEED).shuffle(items)
    key = {}
    wb = xlsxwriter.Workbook(str(out / "m0-blind-test.xlsx"))
    ws = wb.add_worksheet("Blind test")
    info = wb.add_worksheet("Read me")
    head = wb.add_format({"bold": True, "bg_color": "#E8F0EC", "border": 1, "text_wrap": True, "valign": "top"})
    cell = wb.add_format({"border": 1, "valign": "vcenter"})
    wrap = wb.add_format({"text_wrap": True, "valign": "top"})
    info.set_column(0, 0, 100)
    for i, line in enumerate([
        "M0 blind test",
        "For each image, choose which LIGHTING and which CAMERA angle it was meant to show, from the dropdowns.",
        "If the image shows the Mighty product (cream refill bottle + mint dispenser with Mighty branding), also rate "
        "'Product recognizably the same?' (Yes / Partly / No). Leave that column blank for generic images.",
        "Use Notes for anything odd (side effects, artifacts, changed scene). Save the file when done.",
        "Do not open m0-key.json until you have finished.",
    ]):
        info.write(i, 0, line, wrap)
    heads = ["Code", "Image", "Lighting?", "Camera?", "Product recognizably the same?", "Notes"]
    for c, h in enumerate(heads):
        ws.write(0, c, h, head)
    ws.set_column(0, 0, 8); ws.set_column(1, 1, 36); ws.set_column(2, 3, 22)
    ws.set_column(4, 4, 24); ws.set_column(5, 5, 40)
    for n, it in enumerate(items, start=1):
        code = f"S-{n:02d}"
        key[code] = {k: it[k] for k in ("id", "group", "leg", "model", "lighting", "camera")}
        r = n
        ws.set_row(r, 190)
        ws.write(r, 0, code, cell)
        ws.write(r, 1, "", cell)
        thumb = out / "thumbs" / f"{code}.jpg"          # small copy: keeps the workbook light
        thumb.parent.mkdir(parents=True, exist_ok=True)
        im = Image.open(it["file"]).convert("RGB")
        im.thumbnail((480, 480))
        im.save(thumb, quality=88)
        scale = 240 / max(im.size)
        ws.insert_image(r, 1, str(thumb), {"x_scale": scale, "y_scale": scale, "x_offset": 4, "y_offset": 4, "object_position": 1})
        for c in (2, 3, 4, 5):
            ws.write(r, c, "", cell)
    last = len(items)
    none_opt = "none of the options"                # lets the reviewer say "matches no option"
    ws.data_validation(1, 2, last, 2, {"validate": "list", "source": list(LIGHTING) + [none_opt]})
    ws.data_validation(1, 3, last, 3, {"validate": "list", "source": list(CAMERA) + [none_opt]})
    ws.data_validation(1, 4, last, 4, {"validate": "list", "source": ["Yes", "Partly", "No"], "ignore_blank": True})
    ws.freeze_panes(1, 0)
    wb.close()
    write_json_atomic(out / "m0-key.json", key)
    print(f"blind workbook: {out / 'm0-blind-test.xlsx'}  ({len(items)} images)")
    print(f"key (do not open until scored): {out / 'm0-key.json'}")


def cmd_score(a):
    need_libs()
    import openpyxl
    out = Path(a.out)
    key = json.loads((out / "m0-key.json").read_text(encoding="utf-8"))
    wb = openpyxl.load_workbook(out / "m0-blind-test.xlsx", read_only=True, data_only=True)
    ws = wb["Blind test"]
    ans = {}
    for row in ws.iter_rows(min_row=2, values_only=True):
        if row and row[0]:
            ans[str(row[0]).strip()] = [(str(x).strip().lower() if x is not None else "") for x in row[2:5]]
    ov_path = out / "m0-overrides.json"          # reviewer answers kept outside the workbook
    if ov_path.exists():
        for code, fields in json.loads(ov_path.read_text(encoding="utf-8")).items():
            row = ans.setdefault(code, ["", "", ""])
            for dim, val in fields.items():
                if dim in ("lighting", "camera"):
                    row[0 if dim == "lighting" else 1] = str(val).strip().lower()
    missing = [c for c, k in key.items() if not ans.get(c, ["", "", ""])[0] or not ans[c][1]]
    if missing:
        sys.exit(f"{len(missing)} images missing a Lighting or Camera answer: {', '.join(missing[:10])}...")
    stats = {g: {"lighting": [0, 0], "camera": [0, 0]} for g in GROUPS}
    outside = [c for c, a_ in ans.items() if a_[0].startswith("none") or a_[1].startswith("none")]
    prod = {"yes": 0, "partly": 0, "no": 0, "blank": 0}
    for code, k in key.items():
        la, ca, pa = ans[code]
        g = k["group"]
        stats[g]["lighting"][1] += 1; stats[g]["camera"][1] += 1
        stats[g]["lighting"][0] += la == k["lighting"].lower()
        stats[g]["camera"][0] += ca == k["camera"].lower()
        if k["leg"] == "B":
            prod[pa if pa in prod else "blank"] += 1
    print("M0 SCORE  (chance = 33%)\n")
    print(f"{'cell':<28}{'lighting':>10}{'camera':>10}")
    for g, s in stats.items():
        print(f"{GROUPS[g][2]:<28}{s['lighting'][0]:>7}/{s['lighting'][1]:<3}{s['camera'][0]:>7}/{s['camera'][1]:<3}")
    res = {}
    for dim in ("lighting", "camera"):
        tot = sum(s[dim][0] for s in stats.values()); n = sum(s[dim][1] for s in stats.values())
        cells_ok = all(s[dim][0] >= 6 for s in stats.values())
        res[dim] = (tot, n, cells_ok)
    c1 = all(t / n >= 0.8 and ok for t, n, ok in res.values())
    text_groups = [g for g, (leg, _m, _n) in GROUPS.items() if leg == "A"]
    c3 = len(text_groups) == 2 and all(
        abs(stats[text_groups[0]][d][0] - stats[text_groups[1]][d][0]) <= 2 for d in ("lighting", "camera"))
    yp = prod["yes"] + prod["partly"]
    c5 = yp >= 8 and prod["yes"] >= 5
    if outside:
        print(f"\nAnswered 'none of the options' (counted as not identified): {', '.join(outside)}")
    print(f"\nProduct rating (reference leg): yes {prod['yes']}, partly {prod['partly']}, no {prod['no']}, blank {prod['blank']}")
    print("\nCriteria")
    print(f"  1 identified >= 80% per dimension, each cell >= 6/9: {'PASS' if c1 else 'FAIL'}  "
          f"(lighting {res['lighting'][0]}/{res['lighting'][1]}, camera {res['camera'][0]}/{res['camera'][1]})")
    print(f"  2 differences match the intended dimension:          REVIEWER (see grids)")
    print(f"  3 models within 2 of 9 on the text leg:              {'PASS' if c3 else 'FAIL'}")
    print(f"  4 wording does not distort:                          REVIEWER (see grids)")
    print(f"  5 product stays recognizable (>=8 yes/partly, >=5 yes): {'PASS' if c5 else 'FAIL'}")
    write_json_atomic(out / "m0-score.json", {"stats": stats, "product": prod,
                                              "c1": c1, "c3": c3, "c5": c5})


def cmd_grids(a):
    need_libs()
    from PIL import Image, ImageDraw
    out = Path(a.out)
    items = load_done(out, newest_snapshot_rows(out))
    cell = 360
    for g, (_leg, _model, name) in GROUPS.items():
        gi = {(i["lighting"], i["camera"]): i for i in items if i["group"] == g}
        if not gi:
            continue
        sheet = Image.new("RGB", (cell * 3 + 160, cell * 3 + 40), "white")
        d = ImageDraw.Draw(sheet)
        for ci, ck in enumerate(CAMERA):
            d.text((160 + ci * cell + 8, 12), ck, fill="black")
        for li, lk in enumerate(LIGHTING):
            d.text((8, 40 + li * cell + cell // 2), lk, fill="black")
            for ci, ck in enumerate(CAMERA):
                it = gi.get((lk, ck))
                if not it:
                    continue
                im = Image.open(it["file"]).convert("RGB")
                im.thumbnail((cell - 8, cell - 8))
                sheet.paste(im, (160 + ci * cell + 4, 40 + li * cell + 4))
        p = out / f"grid-{g}.png"
        sheet.save(p)
        print(f"grid: {p}  ({name}; rows = lighting, columns = camera)")


def main():
    ap = argparse.ArgumentParser(prog="run_m0.py", description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("--out", default=str(HERE / "out"))
    sub = ap.add_subparsers(dest="cmd", required=True)
    sub.add_parser("dry-run")
    g = sub.add_parser("generate"); g.add_argument("--confirm", required=True)
    g.add_argument("--max-usd", type=float, default=0.60,
                   help="stop submitting new tasks once actual cost exceeds this (default 0.60)")
    sub.add_parser("blind"); sub.add_parser("score"); sub.add_parser("grids")
    a = ap.parse_args()
    {"dry-run": cmd_dry_run, "generate": cmd_generate, "blind": cmd_blind,
     "score": cmd_score, "grids": cmd_grids}[a.cmd](a)


if __name__ == "__main__":
    main()
