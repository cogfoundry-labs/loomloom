"""Case study: a shareable static page from a finished experiment (free, needs Pillow).

    python scripts/image.py case-study --dir <experiment> --out <folder> --meta <case-study meta .json>

The page is the Image Lab case-study design (one self-contained index.html, case-study-data.json, assets/ with JPEG q95 images,
relative paths, so any static host serves it). Everything that can be derived comes from the experiment: the images, their
costs and times, the batches, the model, the size, the plan's brief and the verdicts recorded by `image.py review` (they become
the one-line caption under each image). What a person has to write is in the meta file: the title and summary, a short
description of each group, the caveats, and a tag or note for a batch or image.

The switcher steps through the groups of one dimension (`group_by`, usually `direction` for a Creative Direction run), the way the
Image Lab 1 page steps through rounds. An experiment with one group shows a plain gallery. The page's CSS and script live in
references/case-study/template.html; nothing here changes an experiment or spends money."""
from __future__ import annotations

import html
import json
import re
import shutil
import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))
import ledger as lg  # noqa: E402
import matrix as mx  # noqa: E402
import review as rv  # noqa: E402
import sheet as sh  # noqa: E402

ROOT = Path(__file__).resolve().parent.parent
TEMPLATE = ROOT / "references" / "case-study" / "template.html"
LETTERS = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
HOW = ("Copy the prompt above.",
       "Paste it into an agent that has the Image Lab 2 skill (a checkout of <code>examples/community/image-lab-2</code>; it is not published yet) and ask it to explore creative directions.",
       "Approve the plan, then each paid batch, one at a time. Or swap the subject, style or details to make it yours.")


class CaseStudyError(Exception):
    pass


def esc(s) -> str:
    return html.escape(str(s), quote=True)


# --------------------------------------------------------------------------- #
# reading the experiment
# --------------------------------------------------------------------------- #
def load_meta(path) -> dict:
    p = Path(path)
    try:
        meta = json.loads(p.read_text(encoding="utf-8"))
    except (OSError, ValueError) as e:
        raise CaseStudyError(f"cannot read {p}: {e}")
    for k in ("slug", "title", "summary"):
        if not meta.get(k):
            raise CaseStudyError(f"the meta file needs `{k}`")
    if meta.get("prompt_file"):
        pf = (p.parent / meta["prompt_file"]).resolve() if not Path(meta["prompt_file"]).is_absolute() else Path(meta["prompt_file"])
        try:
            meta["prompt"] = pf.read_text(encoding="utf-8").strip()
        except OSError as e:
            raise CaseStudyError(f"cannot read the prompt file {pf}: {e}")
    return meta


def collect(exp: Path, meta: dict) -> dict:
    """The experiment reduced to what the page shows."""
    import image as il
    ledger = lg.load(exp / "ledger.json")
    plan = mx.load_plan(exp / "plan.json") if (exp / "plan.json").exists() else None
    if plan and mx.consent_problem(plan):
        raise CaseStudyError("a person is pictured in this plan: a case study is never published for an experiment with a pictured person")
    if plan and any(r.get("contains_person") for r in plan.get("references", [])):
        raise CaseStudyError("this experiment uses a reference photo of a person: a case study is never published for it")
    rows = {r["id"]: r for r in ledger["rows"]}
    catalog = il.load_model_catalog()
    group_by = meta.get("group_by", "direction" if "direction" in ledger["experiment"]["dimensions"] else None)
    if group_by and group_by not in ledger["experiment"]["dimensions"]:
        raise CaseStudyError(f"group_by {group_by!r} is not a dimension of this experiment ({', '.join(ledger['experiment']['dimensions'])})")
    latest: dict = {}
    for a in ledger["attempts"]:
        if a.get("status") == "Completed" and a.get("file") and sh._safe_image(exp, a["file"]) and sh._safe_image(exp, a["file"]).exists():
            latest[a["row_id"]] = a                      # a row's latest finished image
    if not latest:
        raise CaseStudyError("no finished images yet")
    order = []
    if group_by and plan:
        order = [mx.value_name(v) for v in plan["dimensions"].get(group_by, [])]
    groups: dict = {}
    for rid, a in latest.items():
        row = rows[rid]
        key = row["params"].get(group_by) if group_by else "all"
        groups.setdefault(key, []).append((int(rid[1:]), rid, a, row))
    names = [k for k in order if k in groups] + sorted(k for k in groups if k not in order)
    out_groups = []
    for gi, key in enumerate(names):
        items = []
        for k, (_, rid, a, row) in enumerate(sorted(groups[key], key=lambda t: t[0])):
            entry = catalog.get(a["model"], {})
            items.append({"row": rid, "label": LETTERS[k], "index": k + 1, "file": a["file"], "model": a["model"], "model_label": entry.get("label") or a["model"],
                          "model_url": entry.get("url") or "", "size": a.get("size_actual") or a.get("size") or "", "cost": float(a.get("cost") or 0),
                          "seconds": a.get("seconds") or 0, "note": rv.caption(row.get("review")), "tag": (meta.get("row_tags") or {}).get(rid, ""),
                          "params": row["params"], "take": row.get("take", 1), "batch": a.get("batch"), "review": row.get("review"),
                          "asset": (f"assets/d{gi + 1}-alternative-{k + 1:02d}.jpg" if group_by else f"assets/alternative-{k + 1:02d}.jpg")})
        out_groups.append({"key": key, "index": gi, "items": items})
    all_items = [i for g in out_groups for i in g["items"]]
    done_by_batch: dict = {}
    for a in ledger["attempts"]:
        if a.get("status") == "Completed":
            done_by_batch[a.get("batch")] = done_by_batch.get(a.get("batch"), 0) + 1
    rejected = [a for a in ledger["attempts"] if a.get("status") == "Failed" and not a.get("cost")]
    batches = [{"no": b["no"], "estimated_usd": float(b.get("estimated_usd") or 0), "actual_usd": float(b.get("actual_usd") or 0), "images": done_by_batch.get(b["no"], 0),
                "ended_at": b.get("ended_at") or ""} for b in ledger.get("batches", [])]
    sizes = sorted({i["size"] for i in all_items if i["size"]})
    models = sorted({i["model"] for i in all_items})
    wildcards = 0
    for vals in ((plan or {}).get("dimensions") or {}).values():
        wildcards += sum(1 for v in vals if isinstance(v, dict) and v.get("wildcard"))
    return {"ledger": ledger, "plan": plan, "group_by": group_by, "groups": out_groups, "items": all_items, "batches": batches, "rejected": len(rejected),
            "models": models, "sizes": sizes, "wildcards": wildcards, "quality": (plan or {}).get("quality"),
            "cost": round(sum(i["cost"] for i in all_items), 4), "est": round(sum(b["estimated_usd"] for b in batches), 4)}


# --------------------------------------------------------------------------- #
# the page
# --------------------------------------------------------------------------- #
def _model_link(label, url, cls: str = "model") -> str:
    return f'<a class="{cls}" href="{esc(url)}" target="_blank" rel="noopener">{esc(label)}</a>' if url else f'<span class="{cls}">{esc(label)}</span>'


def thumb_html(it: dict, active: bool) -> str:
    cap = it["row"] + (f" · {it['tag']}" if it["tag"] else "")
    size = it["size"]
    aria = f' aria-current="true"' if active else ""
    return (f'<button type="button" class="thumb{" active" if active else ""}"{aria} '
            f'aria-label="Show alternative {it["label"]} — {esc(it["row"])}" data-asset="{esc(it["asset"])}" data-label="{it["label"]}" '
            f'data-index="{it["index"]}" data-row="{esc(it["row"])}" data-model="{esc(it["model_label"])}" data-model-url="{esc(it["model_url"])}" data-size="{esc(size)}" '
            f'data-asked="{esc(size)}" data-cost="${it["cost"]:.4f}" data-seconds="{it["seconds"]}" data-note="{esc(it["note"])}" data-selected="">'
            f'<span class="shot"><img src="{esc(it["asset"])}" alt="Alternative {it["label"]}" loading="lazy"></span>'
            f'<span class="thumb-cap"><span class="ltr">{it["label"]}</span><span class="m">{esc(cap)}</span></span></button>')


def head_html(meta: dict, c: dict) -> str:
    legend = "".join(f'<li>{_model_link(next((i["model_label"] for i in c["items"] if i["model"] == m), m), next((i["model_url"] for i in c["items"] if i["model"] == m), ""), "lm")}'
                     f'<span class="x">&times;{sum(1 for i in c["items"] if i["model"] == m)}</span></li>' for m in c["models"])
    n_groups = len(c["groups"])
    word = c["group_by"] or "group"
    eyebrow = meta.get("eyebrow") or f"Image Lab 2 &middot; {esc(c['plan']['intent']) if c['plan'] else 'quick'}"
    parts = [f"{len(c['items'])} images"] + ([f"{n_groups} {word}s"] if n_groups > 1 else []) + [f"{len(c['models'])} model" + ("" if len(c["models"]) == 1 else "s"), f"${c['cost']:.4f} total"]
    return (f'<p class="hero-eyebrow">{eyebrow}</p>\n'
            f'<p class="hero-badge">AI-generated &middot; not a benchmark</p>\n'
            f'    <h1>{esc(meta["title"])}</h1>\n'
            f'    <p class="summary">{esc(meta["summary"])}</p>\n'
            f'    <ul class="legend" aria-label="{"model" if len(c["models"]) == 1 else "models"} in this run">{legend}</ul>\n'
            f'    <p class="stats">{" &middot; ".join(parts)}</p>')


WORDS = ("zero", "one", "two", "three", "four", "five", "six", "seven", "eight", "nine", "ten")


def _word(n: int) -> str:
    return WORDS[n] if 0 <= n < len(WORDS) else str(n)


def _mode(c: dict) -> str:
    plan = c["plan"]
    if not plan:
        return "Quick: one prompt, spread across models"
    if plan.get("batch_by") or c["group_by"] == "direction":
        n = len(c["groups"])
        return f"Creative Direction: {_word(n)} directions" + (f", {_word(c['wildcards'])} marked wildcard" if c["wildcards"] else "")
    return f"Variation: {len([d for d, v in plan['dimensions'].items() if not (d == 'aspect' and len(v) == 1)])} dimensions"


def facts_html(meta: dict, c: dict) -> str:
    plan = c["plan"]
    model_text = ", ".join(next((i["model_label"] for i in c["items"] if i["model"] == m), m) for m in c["models"])
    if c["quality"] and c["quality"] != "auto":
        model_text += f", {c['quality']} quality"
    ask = (plan or {}).get("dimensions", {}).get("aspect")
    ask_ratio = (mx.value_name(ask[0]) if ask and len(ask) == 1 else None)
    size_text = (f"{ask_ratio} &rarr; " if ask_ratio else "") + " / ".join(esc(s).replace("x", "&times;") for s in c["sizes"])
    reject = f" &middot; {c['rejected']} submit{'s' if c['rejected'] != 1 else ''} rejected, not charged" if c["rejected"] else ""
    cards = [("Mode", esc(_mode(c))), ("Intent", esc(plan["intent"]) if plan else esc(c["ledger"]["experiment"]["intent"])), ("Model", esc(model_text)),
             ("Images", f'{len(c["items"])} generated{reject}'), ("Size", size_text),
             ("Actual cost", f'${c["cost"]:.4f} <span class="fact-aside">(est. ~${c["est"]:.4f})</span>')]
    for b in c["batches"]:
        info = (meta.get("batches") or {}).get(str(b["no"]), {})
        title = info.get("title") or "Run"
        note = info.get("note") or f"{b['images']} images"
        cards.append((f"Batch {b['no']} · {title}", f"${b['actual_usd']:.4f} &mdash; {esc(note)}"))
    cards += [("Approvals", f"{len(c['batches'])} &mdash; each paid batch quoted and approved on its own"),
              ("Generated", esc(meta.get("generated_on") or (c["batches"][-1]["ended_at"][:10] if c["batches"] else "")))]
    return "".join(f'<div class="fact"><span class="fact-label">{esc(l)}</span><p>{v}</p></div>' for l, v in cards)


def caveats_html(meta: dict) -> str:
    items = meta.get("caveats") or []
    if not items:
        return ""
    lis = "".join(f'<li><span class="tname">{esc(x["title"])}</span><span class="trole">{esc(x["text"])}</span></li>' for x in items)
    return (f'<section class="caveats">\n    <div class="col">\n    <span class="section-label">Caveats</span>\n    <h2>What to know</h2>\n'
            f'    <p class="lead">Read these before using the images. They come from checking every image at full size against the plan\'s visual checks.</p>\n'
            f'      <ul class="tool-list">{lis}</ul>\n    </div>\n  </section>')


def prompt_html(meta: dict, c: dict) -> str:
    p = esc(meta.get("prompt") or c["ledger"]["experiment"]["brief"])
    how = meta.get("how") or list(HOW)
    steps = ("\n" + " " * 8).join(f"<li>{x}</li>" for x in how)
    trigger = meta.get("trigger") or "use Image Lab 2"
    return (f'<section class="prompt">\n    <div class="col">\n    <span class="section-label">Reuse</span>\n    <h2>The prompt</h2>\n'
            f'    <p class="lead">The exact brief this run used. Copy it, or adapt it.</p>\n'
            f'      <blockquote class="prompt-core">{p}</blockquote>\n'
            f'      <p class="prompt-trigger">&rarr; {esc(trigger)}</p>\n'
            f'      <button class="copy btn-ghost" data-target="copy-src">Copy prompt</button>\n'
            f'      <ol class="how">\n        {steps}\n      </ol>\n'
            f'      <span id="copy-src" hidden>{p}</span>\n    </div>\n  </section>')


def credits_html(meta: dict, c: dict) -> str:
    li = (f'<li><span class="tname"><a class="tn" href="https://cogfoundry.ai" target="_blank" rel="noopener">CogFoundry model gateway</a></span>'
          f'<span class="trole">Ran every generation in this run through <code>POST /api/v1/tasks/generations</code> and reported the real per-image cost. Run total: ${c["cost"]:.4f}.</span></li>')
    sizes_text = " / ".join(esc(sz).replace("x", "&times;") for sz in c["sizes"])
    quality_text = f", at {esc(c['quality'])} quality" if c["quality"] and c["quality"] != "auto" else ""
    for m in c["models"]:
        its = [i for i in c["items"] if i["model"] == m]
        link = _model_link(its[0]["model_label"], its[0]["model_url"], "tn")
        count = f"All {len(its)}" if len(its) == len(c["items"]) else str(len(its))
        li += (f'<li><span class="tname">{link}</span><span class="trole">{count} images{quality_text}'
               + (f", {sizes_text}" if sizes_text else "") + '.</span></li>')
    li += (f'<li><span class="tname"><a class="tn" href="{esc(meta.get("project_url") or "https://github.com/cogfoundry-labs/loomloom/tree/main/examples/community/image-lab-2")}" target="_blank" rel="noopener">Image Lab 2</a></span>'
           f'<span class="trole">A loomloom community example &mdash; the skill that read the brief, proposed the {_word(len(c["groups"])) + " directions" if c["group_by"] == "direction" else "plan"}, priced each batch, ran only the approved ones and kept a record of every attempt.</span></li>')
    return (f'<section class="credits">\n    <div class="col">\n    <span class="section-label">Provenance</span>\n    <h2>How this was made</h2>\n'
            f'    <p class="lead">Image Lab is not a benchmark &mdash; one model, one random draw per row, and the directions differ on purpose. It helps you explore a brief. '
            f'Everything below did real, billed work in this run.</p>\n      <ul class="tool-list">{li}</ul>\n    </div>\n  </section>')


def body_html(meta: dict, c: dict, gi: int) -> str:
    g = c["groups"][gi]
    items = g["items"]
    n = len(c["groups"])
    info = (meta.get("groups") or {}).get(g["key"], {})
    word = (c["group_by"] or "group").upper()
    nav = ""
    if n > 1:
        prev_i, next_i = (gi - 1) % n, (gi + 1) % n
        nav = (f'<nav class="round-nav" aria-label="Switch which {esc(c["group_by"] or "group")} is shown">\n'
               f'    <button type="button" class="round-nav-btn prev" id="round-prev" data-round-index="{prev_i}">&#8592; VIEW {word} {prev_i + 1}</button>\n'
               f'    <span class="round-nav-label">{word} {gi + 1} / {n}</span>\n'
               f'    <button type="button" class="round-nav-btn next" id="round-next" data-round-index="{next_i}">VIEW {word} {next_i + 1} &#8594;</button>\n  </nav>')
    direction = ""
    if n > 1:
        cost = sum(i["cost"] for i in items)
        wild = ('<span class="hero-badge dir-wild">wildcard &middot; relaxes one rule of the brief (approved by the creator)</span>' if info.get("wildcard") else "")
        cards = ""
        if info.get("gives_up"):
            cards += f'<div class="fact"><span class="fact-label">Gives up</span><p>{esc(info["gives_up"])}</p></div>'
        if info.get("outcome"):
            cards += f'<div class="fact"><span class="fact-label">How it turned out</span><p>{esc(info["outcome"])}</p></div>'
        cards += f'<div class="fact"><span class="fact-label">In this {esc(c["group_by"] or "group")}</span><p>{len(items)} images &middot; ${cost:.4f}</p></div>'
        direction = (f'<section class="direction">\n    <span class="section-label">{esc((c["group_by"] or "group").capitalize())} {gi + 1} of {n}</span>\n    <h2>{esc(info.get("name") or g["key"])}</h2>\n    {wild}\n'
                     + (f'    <p class="lead">{esc(info["idea"])}</p>\n' if info.get("idea") else "")
                     + f'    <div class="facts dir-facts">{cards}</div>\n  </section>')
    first = items[0]
    thumbs = "\n      ".join(thumb_html(it, k == 0) for k, it in enumerate(items))
    size = first["size"].replace("x", "×")
    meta_line = (f'Alternative {first["label"]} &middot; {esc(first["row"])} &middot; {_model_link(first["model_label"], first["model_url"])}'
                 f' &middot; {esc(size)} &middot; ${first["cost"]:.4f} &middot; {first["seconds"]}s')
    gallery = (f'<section class="gallery">\n    <span class="section-label">Gallery</span>\n    <h2>Pick your image</h2>\n'
               f'    <p class="lead">Compare the {len(items)} images below &mdash; each a candidate, not a step. Select one to inspect it full size. The line under the picture is how it checked out.</p>\n'
               f'    <div class="gallery-main">\n      <div class="hero-frame">\n'
               f'        <button type="button" class="hero-nav prev js-only" id="hero-prev" aria-label="Previous alternative">&#8592;</button>\n'
               f'        <button type="button" class="hero-nav next js-only" id="hero-next" aria-label="Next alternative">&#8594;</button>\n'
               f'        <button type="button" class="hero-open btn-solid js-only" id="hero-open" aria-label="Open full size">&#8599; Open full size</button>\n'
               f'        <a id="hero-link" href="{esc(first["asset"])}" target="_blank" rel="noopener"><img id="hero-img" src="{esc(first["asset"])}" alt="Alternative {first["label"]}"></a>\n      </div>\n'
               f'      <p class="gallery-meta" id="hero-meta">{meta_line}</p>\n'
               f'      <p class="gallery-noteline" id="hero-note">{esc(first["note"])}</p>\n'
               f'      <p class="gallery-hint">\n        <span>Click image to open full size<span class="js-only"> &middot; &#8592; &#8594; to browse</span></span>\n'
               f'      <span class="pos" id="hero-pos">1 / {len(items)}</span>\n      </p>\n'
               f'      <button type="button" class="link-btn btn-ghost js-only" id="copy-alt-link">Copy link to alternative {first["label"]}</button>\n    </div>\n'
               f'    <div class="gallery-thumbs" aria-label="alternatives">\n      {thumbs}\n    </div>\n'
               f'    <div class="lightbox" id="lightbox" hidden role="dialog" aria-modal="true" aria-label="Full size image">\n'
               f'      <button type="button" class="lb-close" id="lb-close" aria-label="Close (Esc)">&times;</button>\n'
               f'      <button type="button" class="lb-nav prev" id="lb-prev" aria-label="Previous alternative">&#8592;</button>\n'
               f'      <button type="button" class="lb-nav next" id="lb-next" aria-label="Next alternative">&#8594;</button>\n'
               f'      <figure class="lb-figure">\n        <img id="lb-img" src="{esc(first["asset"])}" alt="">\n'
               f'        <figcaption class="lb-bar">\n          <span id="lb-meta"></span>\n'
               f'          <span class="lb-right"><span id="lb-pos"></span><button type="button" class="lb-linkbtn" id="lb-copy-link">Copy link</button><a id="lb-raw" href="{esc(first["asset"])}" target="_blank" rel="noopener">Open raw &#8599;</a></span>\n'
               f'        </figcaption>\n      </figure>\n    </div>\n  </section>')
    facts = (f'<section class="run-facts">\n    <div class="col">\n    <span class="section-label">Details</span>\n    <h2>The run</h2>\n'
             f'    <p class="lead">What Image Lab planned, and what it actually cost. These facts are for the whole run, not only this {esc(c["group_by"] or "group")}.</p>\n'
             f'      <div class="facts">{facts_html(meta, c)}</div>\n    </div>\n  </section>')
    return "\n\n  ".join(x for x in [nav, direction, gallery, facts, caveats_html(meta), prompt_html(meta, c), credits_html(meta, c)] if x)


# --------------------------------------------------------------------------- #
# writing the folder
# --------------------------------------------------------------------------- #
def write_assets(exp: Path, out: Path, items: list[dict]) -> None:
    try:
        from PIL import Image
    except ImportError as e:
        raise CaseStudyError("the case study needs Pillow (pip install Pillow)") from e
    (out / "assets").mkdir(parents=True, exist_ok=True)
    for it in items:
        im = Image.open(sh._safe_image(exp, it["file"])).convert("RGB")
        longest = max(im.size)
        if longest > 2048:
            s = 2048 / longest
            im = im.resize((round(im.width * s), round(im.height * s)), Image.LANCZOS)
        im.save(out / it["asset"], format="JPEG", quality=95, optimize=True, progressive=True)


def build(exp_dir, out_dir, meta_path, force: bool = False) -> dict:
    exp, out = Path(exp_dir), Path(out_dir)
    if not (exp / "ledger.json").exists():
        raise CaseStudyError(f"{exp} has no ledger.json")
    meta = load_meta(meta_path)
    c = collect(exp, meta)
    if out.exists() and any(out.iterdir()):
        if not force:
            raise CaseStudyError(f"{out} already has files: pass --force to replace them")
        here, there = out.resolve(), exp.resolve()
        if there == here or here in there.parents:
            raise CaseStudyError(f"{out} is, or contains, the experiment folder: refusing to replace it")
        if not ((out / "index.html").exists() or (out / "case-study-data.json").exists()):
            raise CaseStudyError(f"{out} has files but is not a case-study folder (no index.html or case-study-data.json): refusing to delete it. Choose an empty or new folder")
        shutil.rmtree(out)
    out.mkdir(parents=True, exist_ok=True)
    write_assets(exp, out, c["items"])
    tpl = TEMPLATE.read_text(encoding="utf-8")
    page_title = meta.get("page_title") or meta["title"]
    canon = (meta.get("canonical_url") or "").strip()
    og = (canon.rstrip("/") + "/" if canon else "") + c["groups"][0]["items"][0]["asset"]
    canon_block = (f'<link rel="canonical" href="{esc(canon)}">\n<meta property="og:url" content="{esc(canon)}">\n' if canon else "")
    rounds = [{"round": gi + 1, "title": f"{meta['title']}: {(meta.get('groups') or {}).get(g['key'], {}).get('name') or g['key']}" if len(c["groups"]) > 1 else meta["title"],
               "head": head_html(meta, c), "body": body_html(meta, c, gi)} for gi, g in enumerate(c["groups"])]
    page = (tpl.replace("{{PAGE_TITLE}}", esc(page_title)).replace("{{DESCRIPTION}}", esc(meta["summary"])).replace("{{OG_IMAGE}}", esc(og))
            .replace("{{CANONICAL}}", canon_block).replace("{{WORDMARK}}", esc(meta.get("wordmark") or "Image Lab 2"))
            .replace("{{ROUND_HEAD}}", rounds[0]["head"]).replace("{{ROUND_BODY}}", rounds[0]["body"])
            .replace("{{ROUNDS_JSON}}", re.sub(r"(?i)</(script)", r"<\\/\1", json.dumps(rounds, ensure_ascii=False))))
    (out / "index.html").write_text(page, encoding="utf-8", newline="\n")
    prompt = meta.get("prompt") or c["ledger"]["experiment"]["brief"]
    data = []
    for g in c["groups"]:
        info = (meta.get("groups") or {}).get(g["key"], {})
        its = g["items"]
        data.append({"kicker": c["plan"]["intent"] if c["plan"] else c["ledger"]["experiment"]["intent"], "title": meta["title"], "slug": meta["slug"],
                     "group": {"by": c["group_by"], "number": g["index"] + 1, "key": g["key"], **{k: info[k] for k in ("name", "idea", "gives_up", "outcome", "wildcard") if k in info}},
                     "summary": meta["summary"], "prompt": prompt, "prompt_trigger": meta.get("trigger") or "use Image Lab 2",
                     "stats": {"images_total": len(c["items"]), "groups": len(c["groups"]), "models": len(c["models"]), "model_labels": [i["model_label"] for i in c["items"][:1]],
                               "actual_usd": c["cost"], "estimated_usd": c["est"], "sizes": c["sizes"], "quality": c["quality"],
                               "group_images": len(its), "group_usd": round(sum(i["cost"] for i in its), 4)},
                     "batches": [{"no": b["no"], "actual_usd": b["actual_usd"], "estimated_usd": b["estimated_usd"], "images": b["images"],
                                  **{k: v for k, v in (meta.get("batches") or {}).get(str(b["no"]), {}).items()}} for b in c["batches"]],
                     "generated_on": meta.get("generated_on") or (c["batches"][-1]["ended_at"][:10] if c["batches"] else ""),
                     "attribution": {"text": "Generated with CogFoundry's model gateway", "url": "https://cogfoundry.ai"},
                     "alternatives": [{"index": i["index"], "of": len(its), "label": i["label"], "row": i["row"], "asset": i["asset"], "model_label": i["model_label"],
                                       "model_url": i["model_url"], "cost_usd": i["cost"], "seconds": i["seconds"], "actual_size": i["size"], "status": "COMPLETED",
                                       "note": i["note"], "take": i["take"], "levers": {k: v for k, v in i["params"].items() if k not in (c["group_by"], "aspect")},
                                       **({"review": {"reviewer": i["review"]["reviewer"], "source": i["review"]["source"], "checks": i["review"]["checks"]}} if i["review"] and i["review"].get("checks") else {})}
                                      for i in its],
                     "caveats": meta.get("caveats") or []})
    (out / "case-study-data.json").write_text(json.dumps(data, indent=2, ensure_ascii=False), encoding="utf-8", newline="\n")
    mb = sum(p.stat().st_size for p in (out / "assets").glob("*.jpg")) / 1048576
    return {"out": out, "images": len(c["items"]), "groups": len(c["groups"]), "assets_mb": round(mb, 1), "cost": c["cost"]}


def add_args(ap) -> None:
    ap.add_argument("--dir", required=True, help="the finished experiment folder")
    ap.add_argument("--out", required=True, help="the case-study folder to write (index.html, case-study-data.json, assets/)")
    ap.add_argument("--meta", required=True, help="the case-study meta file: title, summary, group descriptions, caveats (see references/case-study/README.md)")
    ap.add_argument("--force", action="store_true", help="replace a folder that already has files")


def run(a) -> int:
    try:
        r = build(a.dir, a.out, a.meta, a.force)
    except (CaseStudyError, mx.PlanError) as e:
        print(f"error: {e}", file=sys.stderr)
        return 2
    print(f"case study: {r['out']}  ({r['images']} images in {r['groups']} group(s), {r['assets_mb']} MB, total ${r['cost']:.4f})")
    return 0
