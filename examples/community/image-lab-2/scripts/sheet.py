#!/usr/bin/env python3
"""Contact sheet for an Image Lab 2 experiment (design-v2.md, M5). No gateway call, spends nothing.

    python scripts/image.py sheet --dir <experiment> [--rows DIM] [--cols DIM] [--inline]
                                  [--out FILE] [--selected r001,r004-2]

Builds `contact-sheet.html` in the experiment folder from the ledger: every generated image as a
tile, arranged in a pivot of two dimensions (for example camera across, lighting down) so the
effect of each control can be compared at a glance, with cost, model, status and the exact prompt
one click away. The page is one static file with no external resources; the images are linked by
relative path (or embedded with --inline, which is refused for person experiments).

Everything that came from the ledger or the workbook (values, notes, errors) reaches the page as
JSON and is drawn with textContent, never as HTML, because workbook cells are user-supplied.
"""
from __future__ import annotations

import argparse
import base64
import json
import os
import re
import sys
import time
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))

import ledger as lg  # noqa: E402

SHEET_NAME = "contact-sheet.html"
INLINE_LIMIT = 16 * 1024 * 1024
MIME = {".png": "image/png", ".jpg": "image/jpeg", ".jpeg": "image/jpeg", ".webp": "image/webp"}


class SheetError(Exception):
    pass


def _plan(exp: Path) -> dict:
    p = exp / "plan.json"
    if not p.exists():
        return {}
    try:
        data = json.loads(p.read_text(encoding="utf-8"))
    except (OSError, ValueError):
        return {"_unreadable": True}
    return data if isinstance(data, dict) else {"_unreadable": True}


def _is_person(plan: dict, exp: Path | None = None) -> bool:
    """Fails closed: an unreadable plan, or any approved snapshot that flagged a person reference, counts."""
    if plan.get("_unreadable"):
        return True
    refs = plan.get("references", [])
    if isinstance(refs, list) and any(isinstance(r, dict) and (r.get("contains_person") or r.get("role") == "person")
                                      for r in refs):
        return True
    if exp is not None and (exp / "snapshots").is_dir():
        for p in (exp / "snapshots").glob("*.json"):
            try:
                snap = json.loads(p.read_text(encoding="utf-8"))
            except (OSError, ValueError):
                return True
            if any(ref.get("person") for r in snap.get("rows", []) for ref in r.get("references", [])):
                return True
    return False


_SCHEME = re.compile(r"^[A-Za-z][A-Za-z0-9+.-]*:")


def _safe_image(exp: Path, rel) -> Path | None:
    """The image file a ledger entry names, only if it is a relative path to an image inside the
    experiment folder (ledger text is user-controlled: no URLs, no `..`, no absolute paths)."""
    if not isinstance(rel, str) or not rel or _SCHEME.match(rel) or Path(rel).is_absolute():
        return None
    p = (exp / rel).resolve()
    try:
        p.relative_to(exp.resolve())
    except ValueError:
        return None
    return p if p.suffix.lower() in MIME else None


def _prompts(exp: Path, ledger: dict) -> dict:
    """{(batch no, row id): prompt} from the approved snapshots."""
    out = {}
    for b in ledger["batches"]:
        snap = exp / "snapshots" / f"{b['fingerprint']}.json"
        if snap.exists():
            for r in json.loads(snap.read_text(encoding="utf-8")).get("rows", []):
                out[(b["no"], r["id"])] = r.get("prompt")
    return out


def build_data(exp_dir, inline: bool = False, selected=(), out_dir=None) -> dict:
    exp = Path(exp_dir)
    out_dir = Path(out_dir) if out_dir else exp
    ledger = lg.load(exp / "ledger.json")
    plan = _plan(exp)
    person = _is_person(plan, exp)
    if inline and person:
        raise SheetError("this experiment uses a photo of a person: its results stay local, so the sheet cannot "
                         "be embedded into a shareable single file (--inline). The normal sheet links the images "
                         "next to it and is for you only.")
    names = ledger["experiment"]["dimensions"]
    prompts = _prompts(exp, ledger)
    dim_values = {}
    for d in names:
        planned = [v["value"] if isinstance(v, dict) else v for v in plan.get("dimensions", {}).get(d, [])]
        seen = [r["params"].get(d) for r in ledger["rows"] if r["status"] != "Removed" and r["params"].get(d)]
        dim_values[d] = planned + [v for v in dict.fromkeys(seen) if v not in planned]
    live = [a for a in ledger["attempts"] if not a.get("superseded")]   # not those rejected before billing and rerun
    models = sorted({a["model"] for a in live})
    dim_values["model"] = models
    rows, total_bytes = [], 0
    by_row: dict[str, list[dict]] = {}
    for a in ledger["attempts"]:
        if a.get("superseded"):
            continue                                       # rejected before billing and then run again
        by_row.setdefault(a["row_id"], []).append(a)
    for row in ledger["rows"]:
        atts = by_row.get(row["id"], [])
        if row["status"] == "Removed" and not atts:
            continue
        samples = []
        for a in sorted(atts, key=lambda x: (x["batch"], x["sample"])):
            src, problem = a.get("file"), None
            if src:
                f = _safe_image(exp, src)
                if f is None:
                    src, problem = None, "unsafe or unsupported image path in the ledger; not shown"
                elif not f.exists():
                    src, problem = None, "image file is missing"
                elif inline:
                    raw = f.read_bytes()
                    total_bytes += len(raw)
                    src = f"data:{MIME[f.suffix.lower()]};base64," + base64.b64encode(raw).decode()
                else:                                           # relative to wherever the page is written
                    src = os.path.relpath(f, out_dir.resolve()).replace("\\", "/")
            samples.append({
                "id": a["sample_id"], "batch": a["batch"], "status": a["status"], "model": a["model"],
                "params": a.get("params"),
                "file": src, "cost": a.get("cost"), "seconds": a.get("seconds"), "size": a.get("size_actual"),
                "error": problem or a.get("error") or a.get("note"), "prompt": prompts.get((a["batch"], a["row_id"]))})
        rows.append({"id": row["id"], "params": row["params"], "model": row.get("model"), "take": row.get("take", 1),
                     "status": row["status"], "notes": row.get("notes") or "", "samples": samples})
    if inline and total_bytes > INLINE_LIMIT:
        raise SheetError(f"the embedded images total {total_bytes / 1e6:.1f} MB, over the 16 MB limit for a "
                         f"single-file page; use the normal sheet (it links the images next to it)")
    done = [a for a in live if a["status"] == "Completed"]
    return {
        "brief": ledger["experiment"].get("brief"), "intent": ledger["experiment"].get("intent"),
        "fixed": plan.get("fixed") or {}, "visual_checks": plan.get("visual_checks") or [],
        "person": person, "inline": inline, "generated_at": time.strftime("%Y-%m-%d %H:%M"),
        "dims": names, "dim_values": dim_values, "rows": rows, "selected": list(selected),
        "stats": {"images": len(done), "samples": len(live),
                  "cost": round(sum(float(a.get("cost") or 0) for a in live), 4),
                  "batches": len(ledger["batches"]), "models": models,
                  "problems": sum(1 for a in live if a["status"] in ("Failed", "Blocked", "Unknown"))},
    }


def build_sheet(exp_dir, rows_dim=None, cols_dim=None, inline: bool = False, out=None, selected=()) -> Path:
    exp = Path(exp_dir)
    path = Path(out) if out else exp / SHEET_NAME
    data = build_data(exp, inline, selected, path.resolve().parent)
    axes = data["dims"] + ["model"]
    for name, val in (("rows", rows_dim), ("cols", cols_dim)):
        if val and val not in axes and val != "none":
            raise SheetError(f"--{name} {val!r} is not a dimension of this experiment ({', '.join(axes)})")
    data["default_rows"] = rows_dim or (data["dims"][0] if data["dims"] else "model")
    data["default_cols"] = cols_dim or (data["dims"][1] if len(data["dims"]) > 1 else "none")
    payload = (json.dumps(data, ensure_ascii=False).replace("<", "\\u003c").replace(">", "\\u003e")
               .replace("&", "\\u0026").replace("\u2028", "\\u2028").replace("\u2029", "\\u2029"))
    path.write_text(TEMPLATE.replace("/*__DATA__*/null", payload), encoding="utf-8")
    return path


TEMPLATE = r"""<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Image Lab contact sheet</title>
<style>
:root{--bg:#f6f5f1;--fg:#1c1c1a;--mut:#6b6a63;--card:#fff;--line:#d9d6cc;--acc:#1f7a5a;--bad:#b3261e;--warn:#9a6a00}
@media (prefers-color-scheme:dark){:root:not([data-theme="light"]){--bg:#161614;--fg:#ecebe5;--mut:#9a988e;--card:#1f1f1c;--line:#34332f;--acc:#4fbf97;--bad:#ff8a80;--warn:#e0b050}}
:root[data-theme="dark"]{--bg:#161614;--fg:#ecebe5;--mut:#9a988e;--card:#1f1f1c;--line:#34332f;--acc:#4fbf97;--bad:#ff8a80;--warn:#e0b050}
*{box-sizing:border-box}
body{margin:0;background:var(--bg);color:var(--fg);font:15px/1.45 system-ui,-apple-system,"Segoe UI",sans-serif}
main{max-width:1280px;margin:0 auto;padding:20px 16px 60px}
h1{font-size:22px;margin:0 0 4px}
.mut{color:var(--mut)}
.banner{border:1px solid var(--warn);color:var(--warn);padding:8px 12px;border-radius:6px;margin:12px 0;font-size:14px}
.facts{display:grid;grid-template-columns:repeat(auto-fit,minmax(220px,1fr));gap:12px;margin:14px 0}
.card{background:var(--card);border:1px solid var(--line);border-radius:8px;padding:10px 12px}
.card h2{font-size:12px;letter-spacing:.06em;text-transform:uppercase;color:var(--mut);margin:0 0 6px}
.card ul{margin:0;padding-left:18px}
.stats{display:flex;gap:18px;flex-wrap:wrap;margin:8px 0}
.stats b{font-size:20px;display:block}
.controls{display:flex;gap:14px;flex-wrap:wrap;align-items:center;margin:16px 0}
select,button{font:inherit;padding:5px 8px;border:1px solid var(--line);background:var(--card);color:var(--fg);border-radius:6px}
label{color:var(--mut);font-size:13px}
table{border-collapse:collapse;width:100%}
th,td{border:1px solid var(--line);vertical-align:top;padding:6px}
th{background:var(--card);text-align:left;font-weight:600;font-size:13px}
td.empty{color:var(--mut);text-align:center;font-size:12px}
.tiles{display:flex;flex-wrap:wrap;gap:8px}
.tile{width:168px;background:var(--card);border:1px solid var(--line);border-radius:6px;overflow:hidden;cursor:pointer}
.tile img{display:block;width:100%;aspect-ratio:1/1;object-fit:cover;background:#8884}
.tile.sel{outline:3px solid var(--acc)}
.tile .cap{padding:4px 6px;font-size:11.5px;line-height:1.3}
.tile .ph{display:flex;align-items:center;justify-content:center;aspect-ratio:1/1;font-size:12px;padding:8px;text-align:center;color:var(--bad);background:#8881}
.tag{display:inline-block;font-size:10.5px;padding:0 5px;border-radius:8px;border:1px solid var(--line);margin-right:3px;color:var(--mut)}
.rowkey{font-size:11px;color:var(--mut)}
#lb{position:fixed;inset:0;background:#000c;display:none;align-items:center;justify-content:center;padding:16px;z-index:9}
#lb.on{display:flex}
#lb .box{background:var(--card);max-width:1100px;width:100%;max-height:96vh;overflow:auto;border-radius:8px;display:grid;grid-template-columns:minmax(0,1.3fr) minmax(0,1fr)}
#lb img{width:100%;display:block}
#lb .info{padding:14px;font-size:13.5px}
#lb pre{white-space:pre-wrap;word-break:break-word;background:#8881;padding:8px;border-radius:6px;font:12px/1.4 ui-monospace,Consolas,monospace}
@media(max-width:760px){#lb .box{grid-template-columns:1fr}.tile{width:calc(50% - 4px)}}
</style>
</head>
<body>
<main>
<h1 id="title">Contact sheet</h1>
<div class="mut" id="sub"></div>
<div id="banner"></div>
<div class="stats" id="stats"></div>
<div class="facts" id="facts"></div>
<div class="controls">
  <label>Rows <select id="rowsDim"></select></label>
  <label>Columns <select id="colsDim"></select></label>
  <label><input type="checkbox" id="showBad" checked> show failed / blocked / unknown</label>
</div>
<div id="out"></div>
</main>
<div id="lb"><div class="box"><div><img id="lbimg" alt=""></div><div class="info" id="lbinfo"></div></div></div>
<script>
const DATA = /*__DATA__*/null;
const $ = id => document.getElementById(id);
function el(tag, props, ...kids){
  const e = document.createElement(tag);
  for (const [k, v] of Object.entries(props || {})) { if (k === 'class') e.className = v; else if (k === 'text') e.textContent = v; else e.setAttribute(k, v); }
  for (const kid of kids) if (kid != null) e.append(kid.nodeType ? kid : document.createTextNode(String(kid)));
  return e;
}
const short = m => (m || '').split('/').pop();
const money = v => v == null ? '' : '$' + Number(v).toFixed(4);
const axes = DATA.dims.concat(['model']);
const paramsOf = (row, sample) => (sample && sample.params) ? sample.params : row.params;   // what it was made with
const valOf = (row, sample, axis) => axis === 'model' ? (sample ? sample.model : row.model) : paramsOf(row, sample)[axis];

function fillSelect(sel, withNone, current){
  sel.replaceChildren();
  if (withNone) sel.append(el('option', {value: 'none', text: 'none'}));
  for (const a of axes) sel.append(el('option', {value: a, text: a}));
  sel.value = current;
}
function header(){
  $('title').textContent = DATA.brief || 'Contact sheet';
  $('sub').textContent = [DATA.intent, 'generated ' + DATA.generated_at].filter(Boolean).join(' · ');
  if (DATA.person) $('banner').append(el('div', {class: 'banner', text: 'This experiment uses a photo of a person. Results stay local: do not publish or share this page or its images.'}));
  const s = DATA.stats;
  for (const [n, l] of [[s.images, 'images'], [money(s.cost), 'spent'], [s.batches, 'batches'], [s.models.length, 'models'], [s.problems, 'failed / blocked / unknown']])
    $('stats').append(el('div', {}, el('b', {text: String(n)}), el('span', {class: 'mut', text: l})));
  const facts = $('facts');
  const fx = Object.entries(DATA.fixed);
  if (fx.length) facts.append(el('div', {class: 'card'}, el('h2', {text: 'Fixed in every image'}), el('ul', {}, ...fx.map(([k, v]) => el('li', {text: k + ': ' + v})))));
  if (DATA.visual_checks.length) facts.append(el('div', {class: 'card'}, el('h2', {text: 'Check by eye'}), el('ul', {}, ...DATA.visual_checks.map(v => el('li', {text: v})))));
  const dv = Object.entries(DATA.dim_values).filter(([d]) => d !== 'model' && DATA.dims.includes(d));
  if (dv.length) facts.append(el('div', {class: 'card'}, el('h2', {text: 'Varied'}), el('ul', {}, ...dv.map(([d, vs]) => el('li', {text: d + ': ' + vs.join(', ')})))));
}
function tile(row, smp, otherKeys){
  const cap = el('div', {class: 'cap'});
  cap.append(el('b', {text: smp.id}), ' ', el('span', {class: 'mut', text: short(smp.model)}));
  if (smp.cost != null) cap.append(el('div', {class: 'mut', text: money(smp.cost) + (smp.seconds ? ' · ' + smp.seconds + 's' : '')}));
  if (otherKeys.length) cap.append(el('div', {class: 'rowkey', text: otherKeys.map(k => valOf(row, smp, k)).join(' · ')}));
  let top;
  if (smp.file && smp.status === 'Completed') top = el('img', {src: smp.file, alt: smp.id, loading: 'lazy'});
  else top = el('div', {class: 'ph', text: smp.status + (smp.error ? ': ' + smp.error : '')});
  const t = el('div', {class: 'tile' + (DATA.selected.includes(smp.id) ? ' sel' : '')}, top, cap);
  t.addEventListener('click', () => open(row, smp));
  return t;
}
function open(row, smp){
  const info = $('lbinfo'); info.replaceChildren();
  info.append(el('h3', {text: smp.id + ' · ' + short(smp.model)}));
  info.append(el('div', {class: 'mut', text: [smp.status, money(smp.cost), smp.size, smp.seconds ? smp.seconds + 's' : ''].filter(Boolean).join(' · ')}));
  for (const d of DATA.dims) info.append(el('div', {}, el('span', {class: 'tag', text: d}), paramsOf(row, smp)[d] || ''));
  if (row.notes) info.append(el('p', {text: 'Notes: ' + row.notes}));
  if (smp.error) info.append(el('p', {class: 'mut', text: smp.error}));
  if (smp.prompt) info.append(el('h4', {text: 'Prompt'}), el('pre', {text: smp.prompt}));
  if (smp.file && smp.status === 'Completed') { $('lbimg').src = smp.file; $('lbimg').style.display = ''; } else { $('lbimg').removeAttribute('src'); $('lbimg').style.display = 'none'; }
  $('lb').classList.add('on');
}
function render(){
  const rd = $('rowsDim').value, cd = $('colsDim').value, showBad = $('showBad').checked;
  const used = [rd, cd].filter(a => a && a !== 'none');
  const cells = new Map();   // "r|c" -> [{row, smp}]
  for (const row of DATA.rows) {
    const list = row.samples.filter(s => showBad || s.status === 'Completed');
    for (const smp of list) {
      const key = (rd === 'none' ? '' : valOf(row, smp, rd)) + '\u0001' + (cd === 'none' ? '' : valOf(row, smp, cd));
      if (!cells.has(key)) cells.set(key, []);
      cells.get(key).push({row, smp});
    }
  }
  const rvals = rd === 'none' ? [''] : (DATA.dim_values[rd] || []).slice();
  const cvals = cd === 'none' ? [''] : (DATA.dim_values[cd] || []).slice();
  for (const [k] of cells) { const [r, c] = k.split('\u0001'); if (!rvals.includes(r)) rvals.push(r); if (!cvals.includes(c)) cvals.push(c); }
  const table = el('table');
  const hr = el('tr'); hr.append(el('th', {text: rd === 'none' ? '' : rd + (cd === 'none' ? '' : ' \\ ' + cd)}));
  for (const c of cvals) hr.append(el('th', {text: cd === 'none' ? 'images' : (c || '(none)')}));
  table.append(hr);
  const otherKeys = DATA.dims.filter(a => !used.includes(a)).concat((!used.includes('model') && DATA.dim_values.model.length > 1) ? ['model'] : []);
  for (const r of rvals) {
    const tr = el('tr'); tr.append(el('th', {text: rd === 'none' ? 'All' : (r || '(none)')}));
    for (const c of cvals) {
      const items = cells.get(r + '\u0001' + c) || [];
      if (!items.length) { tr.append(el('td', {class: 'empty', text: 'not generated'})); continue; }
      const box = el('div', {class: 'tiles'});
      for (const {row, smp} of items) box.append(tile(row, smp, otherKeys));
      tr.append(el('td', {}, box));
    }
    table.append(tr);
  }
  $('out').replaceChildren(table);
}
header();
fillSelect($('rowsDim'), true, DATA.default_rows);
fillSelect($('colsDim'), true, DATA.default_cols);
$('rowsDim').addEventListener('change', render);
$('colsDim').addEventListener('change', render);
$('showBad').addEventListener('change', render);
$('lb').addEventListener('click', e => { if (e.target.id === 'lb') $('lb').classList.remove('on'); });
document.addEventListener('keydown', e => { if (e.key === 'Escape') $('lb').classList.remove('on'); });
render();
</script>
</body>
</html>
"""


def main(argv=None) -> int:
    ap = argparse.ArgumentParser(prog="sheet.py", description=__doc__,
                                 formatter_class=argparse.RawDescriptionHelpFormatter)
    add_args(ap)
    return run(ap.parse_args(argv))


def add_args(ap) -> None:
    ap.add_argument("--dir", required=True, help="the experiment folder")
    ap.add_argument("--rows", default=None, help="dimension down the side (default: the first dimension)")
    ap.add_argument("--cols", default=None, help="dimension across (default: the second; `none` for a flat grid)")
    ap.add_argument("--inline", action="store_true", help="embed the images (one shareable file); refused for person experiments")
    ap.add_argument("--out", default=None, help="output file (default <dir>/contact-sheet.html)")
    ap.add_argument("--selected", default="", help="comma-separated sample ids to highlight (your picks)")


def run(a) -> int:
    if hasattr(sys.stdout, "reconfigure"):
        sys.stdout.reconfigure(encoding="utf-8", errors="replace")
    try:
        p = build_sheet(a.dir, a.rows, a.cols, a.inline, a.out, [x.strip() for x in a.selected.split(",") if x.strip()])
    except SheetError as e:
        print(f"error: {e}", file=sys.stderr)
        return 2
    print(f"contact sheet: {p}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
