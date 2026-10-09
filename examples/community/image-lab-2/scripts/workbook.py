"""Experiment workbook for Image Lab 2 (design-v2.md section 6.3).

XlsxWriter writes (it has native checkboxes); openpyxl reads, read-only. The workbook is
always REGENERATED from the ledger and never edited in place. Image Lab never opens the
user's file for modification.

Columns: ID (read-only) | Selected (checkbox) | one per dimension | Model | Take | Reference
         | Prompt | Status | Image (thumbnail) | File | Cost | Notes
System-owned (ignored on read): ID, Prompt, Status, Image, File, Cost.

A row is exactly one image. A second image of the same values is a second row with the next Take number.

Thumbnails (needs Pillow; without it the sheet has only the File link): the row's latest completed
image is shown as an in-cell picture (Excel 365/2021, the same requirement as the checkboxes), so
it sorts and filters with its row. Clicking one opens the raw file. Hovering shows a larger preview,
which XlsxWriter cannot write, so it is injected afterwards as a note with a picture fill; if that step
fails the workbook is still valid, just without the hover picture.
"""
from __future__ import annotations

import datetime as _dt
import io
import os
import re
import shutil
import time
import warnings
import zipfile
from pathlib import Path

import ledger as lg

# Excel re-saves dropdowns as an extension that openpyxl cannot read; we only read values, so it is harmless.
warnings.filterwarnings("ignore", message="Data Validation extension is not supported")

SYSTEM = ("ID", "Prompt", "Status", "Image", "File", "Cost")
USER = ("Selected", "Model", "Take", "Reference", "Notes")
LEGACY = ("Images", "Qty")                          # columns of older workbooks: ignored on read
TRUE_WORDS = {"true", "x", "yes", "y", "1", "✓", "✔"}
IMAGE_COL = re.compile(r"^Image( \d+)?$")           # "Image" now; "Image 1".."Image 3" in older workbooks
THUMB_BOX, HOVER_BOX = (192, 240), (768, 960)       # thumbnail in the cell; larger picture shown on hover
NOTE_PX = (640, 800)                                # hover note size on screen
ROW_PT, IMAGE_COL_WIDTH = 180, 27.5                 # 240 px tall, about 192 px wide


def _need(mod: str):
    try:
        return __import__(mod)
    except ImportError as e:
        raise SystemExit(f"{mod} is required for the workbook (pip install XlsxWriter openpyxl): {e}")


def headers(names: list[str], thumbnails: bool = False) -> list[str]:
    return ["ID", "Selected", *names, "Model", "Take", "Reference", "Prompt", "Status",
            *(["Image"] if thumbnails else []), "File", "Cost", "Notes"]


def _have_pillow() -> bool:
    try:
        import PIL.Image  # noqa: F401
        return True
    except ImportError:
        return False


_CONTROL = re.compile("[\x00-\x08\x0b\x0c\x0e-\x1f]")


def _t(v):
    """Text for a cell: control characters would be written as _x0000_ escapes and come back as that text."""
    return _CONTROL.sub("", v) if isinstance(v, str) else v


def _json_safe(v):
    """A user's extra-column value in a form the ledger can store (dates and times become ISO text)."""
    if v is None or isinstance(v, (bool, int, float, str)):
        return v
    if isinstance(v, (_dt.datetime, _dt.date, _dt.time)):
        return v.isoformat()
    return str(v)


def _read_cached(key: Path) -> bytes | None:
    """A cached JPEG, only if it still decodes: a truncated file from an interrupted write must not break every refresh."""
    try:
        if not key.exists():
            return None
        data = key.read_bytes()
        from PIL import Image
        with Image.open(io.BytesIO(data)) as im:
            im.load()
        return data
    except Exception:
        try:
            key.unlink()
        except OSError:
            pass
        return None


def _write_cache(key: Path, data: bytes) -> None:
    tmp = key.with_name(f"{key.name}.{os.getpid()}.tmp")
    try:
        key.parent.mkdir(exist_ok=True)
        tmp.write_bytes(data)
        os.replace(tmp, key)
    except OSError:
        try:
            tmp.unlink()
        except OSError:
            pass


def _flatten(im):
    """RGB for JPEG: transparency is composited onto white (convert alone turns a transparent pixel into its hidden colour)."""
    from PIL import Image
    if im.mode in ("RGBA", "LA") or (im.mode == "P" and "transparency" in im.info):
        rgba = im.convert("RGBA")
        bg = Image.new("RGB", rgba.size, "white")
        bg.paste(rgba, mask=rgba.split()[-1])
        return bg
    return im.convert("RGB")


def _jpegs(src: Path, cache: Path):
    """(thumbnail, hover preview) JPEG bytes of `src`, from one decode, cached next to the workbook; None if unreadable."""
    try:
        st = src.stat()
        stem = f"{src.stem}-{int(st.st_mtime)}-{st.st_size}"
        kt, kh = cache / f"{stem}-{THUMB_BOX[0]}.jpg", cache / f"{stem}-{HOVER_BOX[0]}.jpg"
        thumb, big = _read_cached(kt), _read_cached(kh)
        if thumb and big:
            return thumb, big
        from PIL import Image, ImageOps
        with Image.open(src) as im:
            im = _flatten(ImageOps.exif_transpose(im))
        made = []
        for box, quality, key in ((HOVER_BOX, 85, kh), (THUMB_BOX, 82, kt)):       # the preview first, then the smaller one from it
            im.thumbnail(box)
            buf = io.BytesIO()
            im.save(buf, "JPEG", quality=quality, optimize=True)
            made.append(buf.getvalue())
            _write_cache(key, made[-1])
        return made[1], made[0]
    except Exception:                       # unreadable or hostile image: that row just has no thumbnail
        return None


def _image_file(base: Path, link: str) -> Path | None:
    """The local file behind an 'external:<relative path>' link, only if it stays inside the experiment folder."""
    rel = link[len("external:"):] if link.startswith("external:") else link
    try:
        f = (base / rel).resolve()
        f.relative_to(base.resolve())
    except (ValueError, OSError):
        return None
    return f if f.is_file() else None


def plan_summary(plan: dict, names: list[str]) -> list[str]:
    lines = [f"Brief: {plan['brief']}", f"Intent: {plan['intent']}"]
    if any(r.get("contains_person") or r.get("role") == "person" for r in plan.get("references", [])):
        lines.append("This experiment uses a photo of a person. Results stay local: do not publish them.")
    if plan.get("visual_checks"):
        lines.append("Check by eye on the first batch: " + "; ".join(plan["visual_checks"]))
    fixed = plan.get("fixed") or {}
    if fixed:
        lines.append("Fixed: " + "; ".join(f"{k}: {v}" for k, v in fixed.items()))
    for d in names:
        vals = plan["dimensions"].get(d)
        if vals is not None:
            lines.append(f"Variable - {d}: " + ", ".join(v["value"] if isinstance(v, dict) else v for v in vals))
    return lines


READ_ME = [
    "How to use this workbook",
    "Tick Selected for the rows you want in the next batch. Ticking starts nothing: your assistant prices the batch and generation starts only after you approve that quote. Rows that already have an image are skipped, and anything you save after approving a batch is not part of that run; it is in the next quote.",
    "You can change the dimension values, Model, Take, Reference and Notes. Save the file before you ask Image Lab to generate.",
    "ID, Prompt, Status, Image, File and Cost are written by Image Lab and ignored when it reads your edits. IDs are read-only: do not edit them.",
    "A row is exactly one image. Take numbers the images of the same values: r001 Take 1 and r002 Take 2 with identical values produce two images. To add one, run `image.py add-takes --dir <experiment> --rows r001`, or copy the row, change Take and save (the copy gets a new ID).",
    "Image shows the row's generated picture. Click it to open the full-size file; move the mouse over it for a larger preview. File is the file name, also a link. The files must stay in this folder.",
    "Use the filter arrows on the top row to narrow the rows. A ticked row that is hidden by a filter is still ticked and still counts as selected.",
    "Prompt is rebuilt from your values at preflight. Edit the values, not the prompt.",
    "Model: after a row is generated, this shows the model that made its images, and further samples of that row use it. Clear the cell to let Image Lab choose again (it is filled in again the next time the row is generated).",
    "Reference: leave blank to use the plan's reference, type a reference id (for example ref1) to choose one, or none for a text-only row. One reference per row.",
    "Extra columns you add are kept as values and ignored. A new dimension column is not added to the plan.",
    "A value that is not in the plan is accepted and flagged as a custom value at preflight.",
    "Image Lab regenerates this file from its ledger; the previous version is kept as experiment.prev.xlsx.",
]


def write_workbook(path, ledger: dict, plan: dict, prompts: dict | None = None,
                   models: list[str] | None = None, image_links: dict | None = None) -> Path:
    """Write a fresh workbook. Returns the path actually written: if `path` cannot be written
    (for example Excel has it open) a timestamped sibling is used instead."""
    xw = _need("xlsxwriter")
    names = ledger["experiment"]["dimensions"]
    path = Path(path)
    try:
        return _write(xw, path, ledger, plan, names, prompts or {}, models, image_links or {})
    except (OSError, xw.exceptions.FileCreateError):    # XlsxWriter wraps a locked file in its own error type
        alt = path.with_name(f"{path.stem}.refresh-{time.strftime('%Y%m%d-%H%M%S')}{path.suffix}")
        return _write(xw, alt, ledger, plan, names, prompts or {}, models, image_links or {})


def _write(xw, path: Path, ledger: dict, plan: dict, names: list[str], prompts: dict,
           models: list[str] | None, image_links: dict) -> Path:
    tmp = path.with_name(f"{path.name}.{os.getpid()}.tmp")           # written beside the target and renamed: a failure never
    try:                                                             # leaves a half-written experiment.xlsx
        _write_to(xw, tmp, ledger, plan, names, prompts, models, image_links)
        os.replace(tmp, path)                                        # a locked file (Excel) fails here, before anything is lost
        return path
    except BaseException:
        try:
            tmp.unlink()
        except OSError:
            pass
        raise


def _write_to(xw, path: Path, ledger: dict, plan: dict, names: list[str], prompts: dict,
              models: list[str] | None, image_links: dict) -> None:
    base = Path(path).parent
    # User and plan text is data: never let a cell starting with "=" become a formula or "http" a link.
    wb = xw.Workbook(str(path), {"strings_to_formulas": False, "strings_to_urls": False})
    ws = wb.add_worksheet("Experiment")
    info = wb.add_worksheet("Read me")
    head = wb.add_format({"bold": True, "bg_color": "#E8F0EC", "border": 1, "text_wrap": True, "valign": "top"})
    sysf = wb.add_format({"font_color": "#666666", "valign": "top", "text_wrap": True})
    usrf = wb.add_format({"valign": "top", "num_format": "@"})      # text: Excel must not turn 3/4 into a date
    numf = wb.add_format({"valign": "top"})
    link = wb.add_format({"font_color": "blue", "underline": 1, "valign": "top"})
    thumbnails = bool(image_links) and _have_pillow()                # no picture column before the first image exists
    cols = headers(names, thumbnails)
    hover: list[tuple[int, int, bytes]] = []
    for c, h in enumerate(cols):
        ws.write(0, c, h, head)
    rows = [r for r in ledger["rows"] if r["status"] != "Removed"]
    for i, row in enumerate(rows, start=1):
        for c, h in enumerate(cols):
            if h == "ID":
                ws.write(i, c, _t(row["id"]), sysf)
            elif h == "Selected":
                ws.insert_checkbox(i, c, bool(row["selected"]))
            elif h in names:
                ws.write(i, c, _t(row["params"].get(h, "")), usrf)
            elif h == "Model":
                ws.write(i, c, _t(row.get("model") or ""), usrf)
            elif h == "Take":
                ws.write(i, c, row.get("take", 1), numf)
            elif h == "Reference":
                ws.write(i, c, _t(row.get("reference") or ""), usrf)
            elif h == "Prompt":
                ws.write(i, c, _t(prompts.get(row["id"], "")), sysf)
            elif h == "Status":
                ws.write(i, c, _t(row["status"]), sysf)
            elif h == "Image":
                links = image_links.get(row["id"], [])
                if links:
                    _thumbnail(ws, base, i, c, links, hover)
            elif h == "File":
                shown = next((l for l in reversed(image_links.get(row["id"], [])) if _image_file(base, l)), None)
                if shown:                                              # the row's latest image that exists, by name
                    ws.write_url(i, c, shown, link, string=Path(shown.replace("\\", "/")).name)
                else:
                    ws.write(i, c, "", sysf)
            elif h == "Cost":
                cost = lg.row_cost(ledger, row["id"])
                ws.write(i, c, cost if cost else "", sysf)
            elif h == "Notes":
                ws.write(i, c, _t(row.get("notes") or ""), usrf)
    extra_heads = []
    for row in rows:
        for k in (row.get("extras") or {}):
            if k not in cols and k not in extra_heads:
                extra_heads.append(k)
    for i, row in enumerate(rows, start=1):                  # carried user columns, each under its own header
        for k, v in (row.get("extras") or {}).items():
            if k in extra_heads:
                ws.write(i, len(cols) + extra_heads.index(k), _t(v), usrf)
    for e, k in enumerate(extra_heads):
        ws.write(0, len(cols) + e, k, head)
    last = max(len(rows), 1)
    take_col, model_col = cols.index("Take"), cols.index("Model")
    ws.data_validation(1, take_col, last, take_col, {"validate": "integer", "criteria": ">=", "value": 1,
                                                     "error_message": "Take is a whole number from 1."})
    if models:                                    # a range, not an inline list (Excel limits inline lists to 255 characters)
        lists = wb.add_worksheet("Lists")
        for i, m in enumerate(models):
            lists.write(i, 0, m)
        lists.hide()
        ws.data_validation(1, model_col, last, model_col, {"validate": "list", "source": f"=Lists!$A$1:$A${len(models)}", "ignore_blank": True})
    ws.set_column(0, 0, 7)
    ws.set_column(1, 1, 9)
    ws.set_column(2, 1 + len(names), 22, usrf)               # text format for rows the user adds below
    ws.set_column(model_col, model_col, 28, usrf)
    ws.set_column(cols.index("Reference"), cols.index("Reference"), 12, usrf)
    ws.set_column(cols.index("Prompt"), cols.index("Prompt"), 60)
    ws.set_column(cols.index("Status"), cols.index("Cost"), 12)
    ws.set_column(cols.index("Notes"), cols.index("Notes"), 30, usrf)
    for h in cols:
        if IMAGE_COL.match(h):
            ws.set_column(cols.index(h), cols.index(h), IMAGE_COL_WIDTH)
    ws.autofilter(0, 0, last, len(cols) + len(extra_heads) - 1)
    ws.freeze_panes(1, 2)
    info.set_column(0, 0, 110)
    wrap = wb.add_format({"text_wrap": True, "valign": "top"})
    bold = wb.add_format({"bold": True})
    r = 0
    for n, line in enumerate(READ_ME):
        info.write(r, 0, line, bold if n == 0 else wrap)
        r += 1
    r += 1
    info.write(r, 0, "Plan summary", bold)
    for line in plan_summary(plan, names):
        r += 1
        info.write(r, 0, line, wrap)
    wb.close()
    if hover:
        _inject_hover(path, hover)


def _thumbnail(ws, base: Path, row: int, col: int, links: list, hover: list) -> bool:
    """Embed the row's newest image that can be read (a bad latest file falls back to the one before it). Never raises:
    a row whose images cannot be read simply has no picture."""
    for link in reversed(links):
        src = _image_file(base, link)
        if src is None:
            continue
        pair = _jpegs(src, base / ".thumbs")
        if pair is None:
            continue
        thumb, big = pair
        try:
            from PIL import Image
            with Image.open(io.BytesIO(big)) as im:             # a note is stretched to its box, so the box must have the picture's shape
                w, h = im.size
            fit = min(NOTE_PX[0] / w, NOTE_PX[1] / h)
            rel = (link[len("external:"):] if link.startswith("external:") else link).replace("\\", "/")
            tip = f"Click to open the full-size image ({src.stat().st_size / 1e6:.1f} MB)"
            ws.embed_image(row, col, src.stem + ".jpg", {"image_data": io.BytesIO(thumb), "description": src.stem,
                                                          "url": "external:" + rel, "tip": tip})
            ws.write_comment(row, col, " ", {"width": round(w * fit), "height": round(h * fit), "author": "Image Lab"})
        except Exception:
            continue
        ws.set_row(row, ROW_PT)
        hover.append((row, col, big))
        return True
    return False


def _inject_hover(path: Path, hover: list[tuple[int, int, bytes]]) -> None:
    """Give each thumbnail note a picture fill (the larger preview). XlsxWriter cannot do this, so the xlsx
    zip is patched: VML fill -> picture, a relationships part, the media files and a jpeg content type.
    Any failure leaves the original workbook untouched (it still opens; the notes are just blank)."""
    tmp = path.with_name(path.name + ".tmp")
    try:
        with zipfile.ZipFile(path) as z:
            parts = {n: z.read(n) for n in z.namelist()}
        vmls = [n for n in parts if n.startswith("xl/drawings/vmlDrawing") and n.endswith(".vml")]
        if len(vmls) != 1:
            return
        shapes = parts[vmls[0]].decode("utf-8").split("<v:shape ")
        wanted = {(r, c): data for r, c, data in hover}
        rels, out = [], [shapes[0]]
        for s in shapes[1:]:
            r = int(re.search(r"<x:Row>(\d+)</x:Row>", s).group(1))
            c = int(re.search(r"<x:Column>(\d+)</x:Column>", s).group(1))
            data = wanted.get((r, c))
            if data is not None and '<v:fill color2="#ffffe1"/>' in s:
                n = len(rels) + 1
                parts[f"xl/media/hover{n}.jpeg"] = data
                rels.append((f"rId{n}", f"../media/hover{n}.jpeg"))
                s = re.sub(r"\s*<x:Anchor>.*?</x:Anchor>", "", s, flags=re.S)     # Excel sizes the note from the cells it spans (a tall
                                                                                   # wrapped row would stretch the picture); without it, from style
                s = s.replace('<v:fill color2="#ffffe1"/>',
                              f'<v:fill o:relid="rId{n}" o:title="preview" recolor="t" rotate="t" type="frame"/>')
            out.append(s)
        if not rels:
            return
        parts[vmls[0]] = "<v:shape ".join(out).encode("utf-8")
        parts[f"xl/drawings/_rels/{Path(vmls[0]).name}.rels"] = (
            '<?xml version="1.0" encoding="UTF-8" standalone="yes"?>\n'
            '<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">'
            + "".join(f'<Relationship Id="{rid}" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/image" Target="{t}"/>'
                      for rid, t in rels) + "</Relationships>").encode("utf-8")
        ct = parts["[Content_Types].xml"].decode("utf-8")
        if 'Extension="jpeg"' not in ct:
            ct = ct.replace('<Default Extension="xml"', '<Default Extension="jpeg" ContentType="image/jpeg"/><Default Extension="xml"', 1)
        parts["[Content_Types].xml"] = ct.encode("utf-8")
        with zipfile.ZipFile(tmp, "w", zipfile.ZIP_DEFLATED) as z:
            z.writestr("[Content_Types].xml", parts.pop("[Content_Types].xml"))
            for name, blob in parts.items():
                z.writestr(name, blob)
        os.replace(tmp, path)
    except Exception:
        try:
            tmp.unlink()
        except OSError:
            pass


# --------------------------------------------------------------------------- #
def _truthy(v) -> bool:
    if isinstance(v, bool):
        return v
    if v is None:
        return False
    return str(v).strip().lower() in TRUE_WORDS


def _qty(v):
    if v is None or v == "":
        return None
    try:
        f = float(v)
        return int(f) if f == int(f) else f
    except (TypeError, ValueError, OverflowError):
        return v                          # kept raw (also nan and inf); preflight reports it


def read_workbook(path, names: list[str]) -> dict:
    """Read user-owned cells, read-only. Returns {rows, extra_columns, saved_at}.

    `rows` items: {line, id, selected, params, model, take, reference, notes, extras}; an older workbook
    that still has Qty and no Take gives `legacy_qty` instead of `take`.
    System-owned columns are ignored."""
    openpyxl = _need("openpyxl")
    path = Path(path)
    wb = openpyxl.load_workbook(path, read_only=True, data_only=True)
    ws = wb["Experiment"]
    it = ws.iter_rows(values_only=True)
    head = [("" if h is None else str(h).strip()) for h in next(it)]
    idx = {h: i for i, h in enumerate(head) if h}
    fold = {h.casefold(): i for h, i in reversed(list(idx.items()))}
    dim_at = {d: idx.get(d, fold.get(d.casefold())) for d in names}      # a changed capitalisation must not wipe the column
    missing = [d for d in names if dim_at[d] is None]
    known = set(SYSTEM) | set(USER) | set(LEGACY) | set(names)
    dim_fold = {d.casefold() for d in names}
    extra_cols = [h for h in head if h and h not in known and h.casefold() not in dim_fold and not IMAGE_COL.match(h)]
    out = []
    for line, row in enumerate(it, start=2):
        if row is None or all(c in (None, "") for c in row):
            continue

        def cell(name):
            i = idx.get(name)
            return row[i] if i is not None and i < len(row) else None

        def text(v):
            return None if v is None or str(v).strip() == "" else str(v).strip()

        out.append({
            "line": line, "id": text(cell("ID")), "selected": _truthy(cell("Selected")),
            "params": {d: ("" if row[dim_at[d]] is None else str(row[dim_at[d]]).strip()) for d in names
                       if dim_at[d] is not None and dim_at[d] < len(row)},
            "model": text(cell("Model")), "reference": text(cell("Reference")),
            **({"take": _qty(cell("Take"))} if "Take" in idx else {}),
            **({"legacy_qty": _qty(cell("Qty"))} if "Qty" in idx and "Take" not in idx else {}),
            "notes": "" if cell("Notes") is None else str(cell("Notes")),
            "extras": {h: _json_safe(cell(h)) for h in extra_cols if cell(h) is not None},
        })
    wb.close()
    return {"rows": out, "extra_columns": extra_cols, "saved_at": path.stat().st_mtime, "missing_columns": missing}


def backup_previous(path) -> Path | None:
    """Copy the current workbook to <name>.prev.xlsx before it is regenerated."""
    path = Path(path)
    if not path.exists():
        return None
    prev = path.with_name(f"{path.stem}.prev{path.suffix}")
    shutil.copy2(path, prev)
    return prev
