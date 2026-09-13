#!/usr/bin/env python3
"""build-exploration-page.py — turn an Image Lab session into one shareable
case-study page that grows as the session grows.

`image.py run` writes `<out>/round-N/run.json` per round and appends a ledger
entry to `<out>/session.json`. This script reads both and builds ONE
self-contained static folder per session, at a FIXED address that is set once
(from the first round's --title) and never moves on later rounds:

    <out>/<slug>/
        index.html            # the page (styles + a small gallery script inline)
        case-study-data.json  # the data model — a future PDF / social-card renderer reads THIS
        assets/r<N>-alternative-01.png ...   # every published round's images

**No gate. Nothing is spent** — the images already exist. This is a render step.

The page is an **image-selection gallery**, not a benchmark report, in
redesign-lab's house style (Source Serif 4 body, Arial-Black uppercase headings,
IBM Plex Mono labels, hard edges, the loomloom green accent, 3-state dark mode).
It defaults to showing the LATEST round; once a second round has ever been
published to this page, a nav strip directly above the Gallery section
("<- VIEW ROUND N / ROUND N of M / VIEW ROUND N ->") switches to any adjacent
round in place (same URL, no reload), wrapping at both ends — see
render_session()/PAGE's `#round-head`/`#round-body` containers, `_round_nav_html()`,
and the embedded `ROUNDS` JS array. It lives at the point of use, not in the
topbar with the Share buttons, and each side names the specific round it goes
to rather than a generic icon — an earlier icon-only topbar button tested as
easy to miss and not obviously interactive. Every section has the same shape:
a mono eyebrow, an Arial-Black headline, one lead line, then the body.

  topbar (Share: Copy link · X · LinkedIn — session-level, fixed)
  ->  hero (badge · tagline · model legend · stats)  — swappable per round
  ->  round nav (once round 2+ exists) — swappable per round
  ->  Gallery / Pick your image (framed hero + letter-badged thumbnails + lightbox)
  ->  Details / The run (a hairline facts grid)
  ->  Reuse / The prompt (the brief + a 3-step "how to use this")
  ->  Provenance / How this was made (every model + tool, linked, with the cost)
  ->  Roadmap / From image exploration to AI work            — session-level, fixed
  ->  Your turn / Bring your own prompt (CTA)  ->  footer     — session-level, fixed

Share affordances: the topbar Share cluster (Copy link + one-click X / LinkedIn,
hrefs filled from the canonical URL + OG tags, describing whichever round was
latest at build time); a per-alternative deep link (`…/index.html#E` selects
alternative E within the currently-shown round); Open Graph / Twitter-card
meta so a pasted link unfurls with the image.

Usage
    python scripts/build-exploration-page.py --session ./out \
        --title "Murree Meets Toronto" \
        --subject "Murree x Toronto" \
        --invocation "<the exact message the user sent to trigger Image Lab>" \
        [--round N] [--summary "<override the auto tagline>"] \
        [--selected C] [--slug custom-slug — only takes on the first round] \
        [--out ./out/<slug>] \
        [--canonical-url https://you.github.io/case-study/] [--inline]

`--session` is the session root (image.py run's `--out`), not any one round's
own folder; the script finds the latest round in `<session>/session.json` on
its own. --canonical-url is where the folder will actually live; it makes
og:image an absolute URL (so link unfurls work) and the topbar "Copy link"
copy that URL. --inline also writes index.inline.html — the same page with
every round's PNGs embedded as base64 data URIs, i.e. one self-contained file
to publish as an artifact. No recompression (stdlib only); if the images push
it past the 16 MB artifact limit, recompress them to JPEG before publishing.

Standard library only.
"""
from __future__ import annotations

import argparse
import html
import json
import re
import shutil
import sys
from datetime import date
from pathlib import Path

ATTRIBUTION_TEXT = "Generated with CogFoundry's model gateway"
ATTRIBUTION_URL = "https://cogfoundry.ai"
SKILL_URL = "https://github.com/cogfoundry-labs/loomloom/tree/main/examples/community/image-lab"
INSTALL_CMD = "npx skills add cogfoundry-labs/loomloom --skill image-lab -a claude-code -g -y"
_WORDS = ["zero", "one", "two", "three", "four", "five", "six", "seven", "eight"]
X = "×"  # multiplication sign for WxH display


def die(msg: str) -> "NoReturn":  # type: ignore[valid-type]
    print(f"error: {msg}", file=sys.stderr)
    sys.exit(1)


def slugify(s: str) -> str:
    s = re.sub(r"[^a-z0-9]+", "-", s.strip().lower()).strip("-")
    return s[:64].rstrip("-") or "exploration"


def _num(n: int) -> str:
    return _WORDS[n] if 0 <= n < len(_WORDS) else str(n)


def _tagline(n_models: int, n_made: int, subject: str) -> str:
    m = f"{_num(n_models).capitalize()} model" + ("" if n_models == 1 else "s")
    w = f"{_num(n_made).capitalize()} way" + ("" if n_made == 1 else "s") + " to see"
    return f"One brief. {m}. {w} {subject}."


def _prompt_parts(prompt: str, invocation: str) -> tuple[str, str, str]:
    """(core, trigger, copy_text).

    core    = the exact prompt the run used (run.json), always shown in full.
    trigger = how the user wrapped it to reach Image Lab ("use Image Lab", …),
              cleaned for display, or the default.
    copy_text = what the Copy button puts on the clipboard — the user's verbatim
                message if it contained the prompt, else prompt + a default line.
    """
    core = (prompt or "").strip()
    inv = (invocation or "").strip()
    if core and core in inv:
        pre, _, post = inv.partition(core)
        trigger = re.sub(r"\s+", " ", f"{pre} {post}").strip(" :—-–")
        copy_text = inv
    else:
        trigger = inv if (inv and not core) else ""
        copy_text = core + "\n\nuse Image Lab"
    return core, (trigger or "use Image Lab"), copy_text


# --------------------------------------------------------------------------- #
# data model  (the seam — render() is the only thing that reads this)
# --------------------------------------------------------------------------- #
def assemble(run: dict, title: str, subject: str, summary: str | None,
             invocation: str | None, selected_label: str | None, asset_prefix: str = "") -> dict:
    alts_in = run.get("alternatives") or []
    if not alts_in:
        die("run.json has no `alternatives` — is this from a current image.py run?")
    need = {"index", "of", "label", "model_label", "cost_usd", "requested_size", "status"}
    for a in alts_in:
        missing = need - a.keys()
        if missing:
            die(f"run.json alternative {a.get('label', '?')} is missing {sorted(missing)} "
                f"— regenerate it with the current image.py")

    models = []
    for a in alts_in:
        if a["model_label"] not in models:
            models.append(a["model_label"])

    sel = (selected_label or "").strip().upper() or None
    if sel:
        match = next((a for a in alts_in if a["label"] == sel), None)
        if match is None:
            die(f"--selected {sel} is not a branch label in this run "
                f"({', '.join(a['label'] for a in alts_in)})")
        if not (match["status"] == "COMPLETED" and match.get("file")):
            print(f"warning: --selected {sel} did not generate an image; "
                  f"treating the run as having no selection", file=sys.stderr)
            sel = None

    alternatives = []
    for a in alts_in:
        ok = a["status"] == "COMPLETED" and a.get("file")
        alternatives.append({
            "index": a["index"], "of": a["of"], "label": a["label"],
            "asset": f"assets/{asset_prefix}alternative-{a['index']:02d}.png" if ok else None,
            "source_file": a.get("file"),
            "model_label": a["model_label"],
            "model_url": a.get("model_url") or None,
            "cost_usd": a["cost_usd"],
            "seconds": a.get("seconds"),
            "requested_size": a["requested_size"],
            "actual_size": a.get("actual_size"),
            "status": a["status"],
            "note": a.get("note"),
            "selected": a["label"] == sel,
        })

    made = sum(1 for a in alternatives if a["asset"])
    selected = next((a for a in alternatives if a["selected"]), None)
    prompt = run.get("prompt") or ""
    custom_summary = (summary or "").strip()
    core, trigger, copy_text = _prompt_parts(prompt, invocation or "")

    # per-model detail for the hero legend + credits (generated images only)
    detail = []
    for a in alternatives:
        row = next((x for x in detail if x["label"] == a["model_label"]), None)
        if row is None:
            row = {"label": a["model_label"], "url": a["model_url"],
                   "count": 0, "labels": []}
            detail.append(row)
        if a["asset"]:
            row["count"] += 1
            row["labels"].append(a["label"])
    detail = [d for d in detail if d["count"]]

    req_sizes = []
    for a in alternatives:
        if a["requested_size"] not in req_sizes:
            req_sizes.append(a["requested_size"])

    return {
        "kicker": (run.get("intent") or "exploration"),
        "title": title.strip(),
        "slug": slugify(title),
        "subject": subject.strip(),
        "summary": custom_summary or _tagline(len(models), made, subject.strip()),
        "summary_is_auto": not custom_summary,
        "prompt": prompt,
        "prompt_core": core,
        "prompt_trigger": trigger,
        "prompt_copy": copy_text,
        "intent": run.get("intent"),
        "stats": {
            "requested": len(alternatives),
            "generated": made,
            "models": len(models),
            "model_labels": models,
            "models_detail": detail,
            "requested_sizes": req_sizes,
            "actual_usd": run.get("actual_usd", 0.0),
            "estimated_usd": run.get("estimated_usd"),
        },
        "alternatives": alternatives,
        "selected": None if not selected else {
            "index": selected["index"], "label": selected["label"],
            "model_label": selected["model_label"], "model_url": selected["model_url"],
            "cost_usd": selected["cost_usd"], "requested_size": selected["requested_size"],
            "actual_size": selected["actual_size"], "asset": selected["asset"],
        },
        "generated_on": date.today().isoformat(),
        "attribution": {"text": ATTRIBUTION_TEXT, "url": ATTRIBUTION_URL},
    }


# --------------------------------------------------------------------------- #
# renderer helpers
# --------------------------------------------------------------------------- #
def _money(x: float) -> str:
    return f"${x:.4f}" if x < 1 else f"${x:.2f}"


def _plural(n: int, word: str) -> str:
    return f"{n} {word}" + ("" if n == 1 else "s")


def _stat_line(s: dict) -> str:
    n, req = s["generated"], s["requested"]
    count = _plural(n, "candidate") if n == req else f"{n} of {req} generated"
    return " &middot; ".join([count, _plural(s["models"], "model"),
                             f"{_money(s['actual_usd'])} total"])


def _size_disp(a: dict) -> str:
    got = (a["actual_size"] or a["requested_size"]).replace("x", X)
    if a["actual_size"] and a["actual_size"] != a["requested_size"]:
        got += f' (asked {a["requested_size"].replace("x", X)})'
    return got


def _model_link(label: str, url: str | None, cls: str = "model") -> str:
    esc = html.escape(label)
    if url:
        return (f'<a class="{cls}" href="{html.escape(url)}" '
                f'target="_blank" rel="noopener">{esc}</a>')
    return f'<span class="{cls}">{esc}</span>'


def _secs(a: dict) -> str:
    s = a.get("seconds")
    return f" &middot; {s:g}s" if isinstance(s, (int, float)) and s > 0 else ""


def _meta_html(a: dict) -> str:
    # keep the shape in sync with rebuildMeta() in the inline <script>
    chose = ('<span class="chose">the creator&rsquo;s pick</span> '
             if a["selected"] else "")
    return (f'{chose}Alternative {html.escape(a["label"])} &middot; '
            f'{_model_link(a["model_label"], a["model_url"])} &middot; '
            f'{html.escape(_size_disp(a))} &middot; {_money(a["cost_usd"])}{_secs(a)}')


def _legend(data: dict) -> str:
    detail = data["stats"]["models_detail"]
    if not detail:
        return ""
    items = "".join(
        f'<li>{_model_link(d["label"], d["url"], "lm")}'
        f'<span class="x">{X}{d["count"]}</span></li>'
        for d in detail
    )
    return f'    <ul class="legend" aria-label="models in this run">{items}</ul>\n'


def _thumb(a: dict, active: bool) -> str:
    d = {
        "data-asset": a["asset"],
        "data-label": a["label"],
        "data-index": str(a["index"]),
        "data-model": a["model_label"],
        "data-model-url": a["model_url"] or "",
        "data-size": a["actual_size"] or a["requested_size"],
        "data-asked": a["requested_size"],
        "data-cost": _money(a["cost_usd"]),
        "data-seconds": (f'{a["seconds"]:g}'
                         if isinstance(a.get("seconds"), (int, float)) and a["seconds"] > 0 else ""),
        "data-selected": "1" if a["selected"] else "",
    }
    attrs = " ".join(f'{k}="{html.escape(str(v), quote=True)}"' for k, v in d.items())
    badge = ('<span class="thumb-badge" title="the creator&rsquo;s pick">&#10003;</span>'
             if a["selected"] else "")
    cur = ' aria-current="true"' if active else ""
    lab, model = html.escape(a["label"]), html.escape(a["model_label"])
    return (f'<button type="button" class="thumb{" active" if active else ""}"{cur} '
            f'aria-label="Show alternative {lab} — {model}" {attrs}>'
            f'<span class="shot"><img src="{a["asset"]}" alt="Alternative {lab}" loading="lazy">'
            f'{badge}</span>'
            f'<span class="thumb-cap"><span class="ltr">{lab}</span>'
            f'<span class="m">{model}</span></span></button>')


def _failed_note(data: dict) -> str:
    bad = [a for a in data["alternatives"] if not a["asset"]]
    if not bad:
        return ""
    labels = ", ".join(a["label"] for a in bad)
    reasons = "; ".join(sorted({a["note"] for a in bad if a["note"]}))
    tail = f" ({reasons})" if reasons else ""
    plural = "alternatives" if len(bad) > 1 else "alternative"
    return (f'<p class="gallery-note">{data["stats"]["generated"]} of '
            f'{data["stats"]["requested"]} generated &mdash; {plural} {labels} '
            f'did not{tail}.</p>')


def _lightbox(first_asset: str) -> str:
    fa = html.escape(first_asset, quote=True)
    return (
        '    <div class="lightbox" id="lightbox" hidden role="dialog" aria-modal="true" '
        'aria-label="Full size image">\n'
        '      <button type="button" class="lb-close" id="lb-close" aria-label="Close (Esc)">'
        '&times;</button>\n'
        '      <button type="button" class="lb-nav prev" id="lb-prev" aria-label="Previous alternative">'
        '&#8592;</button>\n'
        '      <button type="button" class="lb-nav next" id="lb-next" aria-label="Next alternative">'
        '&#8594;</button>\n'
        '      <figure class="lb-figure">\n'
        f'        <img id="lb-img" src="{fa}" alt="">\n'
        '        <figcaption class="lb-bar">\n'
        '          <span id="lb-meta"></span>\n'
        '          <span class="lb-right"><span id="lb-pos"></span>'
        '<button type="button" class="lb-linkbtn" id="lb-copy-link">Copy link</button>'
        f'<a id="lb-raw" href="{fa}" target="_blank" rel="noopener">Open raw &#8599;</a></span>\n'
        '        </figcaption>\n'
        '      </figure>\n'
        '    </div>\n'
    )


def _head(eyebrow: str, headline: str, lead: str = "") -> str:
    """The one section-header shape every section uses: mono eyebrow, Arial-Black
    headline, optional one lead paragraph."""
    out = (f'    <span class="section-label">{html.escape(eyebrow)}</span>\n'
           f'    <h2>{html.escape(headline)}</h2>\n')
    if lead:
        out += f'    <p class="lead">{lead}</p>\n'
    return out


def _gallery(data: dict) -> str:
    shown = [a for a in data["alternatives"] if a["asset"]]
    note = _failed_note(data)
    if not shown:
        return ('  <section class="gallery">\n'
                + _head("Gallery", "Pick your image")
                + '    <p class="gallery-note">No images were generated in this run.</p>\n'
                + f'    {note}\n  </section>')
    n = len(shown)
    multi = n > 1
    initial = next((i for i, a in enumerate(shown) if a["selected"]), 0)
    first = shown[initial]
    headline = "Pick your image" if multi else "The image"
    lead_txt = (f'Compare the {n} alternatives below &mdash; each a candidate, not a step. '
                f'Select one to inspect it full size.' if multi else
                'The single image this run produced.')
    head = _head("Gallery", headline, lead_txt)
    navs = (
        '        <button type="button" class="hero-nav prev js-only" id="hero-prev" '
        'aria-label="Previous alternative">&#8592;</button>\n'
        '        <button type="button" class="hero-nav next js-only" id="hero-next" '
        'aria-label="Next alternative">&#8594;</button>\n'
    ) if multi else ""
    browse = ('<span class="js-only"> &middot; &#8592; &#8594; to browse</span>'
              if multi else "")
    pos = (f'      <span class="pos" id="hero-pos">{initial + 1} / {n}</span>\n'
           if multi else "")
    link_btn = (f'      <button type="button" class="link-btn btn-ghost js-only" id="copy-alt-link">'
                f'Copy link to alternative {html.escape(first["label"])}</button>\n'
                if multi else "")
    strip = ""
    if multi:
        thumbs = "\n      ".join(_thumb(a, i == initial) for i, a in enumerate(shown))
        strip = (f'    <div class="gallery-thumbs" aria-label="alternatives">\n'
                 f'      {thumbs}\n    </div>\n')
    lb = _lightbox(first["asset"]) if multi else ""
    return (
        '  <section class="gallery">\n'
        f'{head}'
        '    <div class="gallery-main">\n'
        '      <div class="hero-frame">\n'
        f'{navs}'
        '        <button type="button" class="hero-open btn-solid js-only" id="hero-open" '
        'aria-label="Open full size">&#8599; Open full size</button>\n'
        f'        <a id="hero-link" href="{first["asset"]}" target="_blank" rel="noopener">'
        f'<img id="hero-img" src="{first["asset"]}" alt="Alternative {html.escape(first["label"])}"></a>\n'
        '      </div>\n'
        f'      <p class="gallery-meta" id="hero-meta">{_meta_html(first)}</p>\n'
        '      <p class="gallery-hint">\n'
        f'        <span>Click image to open full size{browse}</span>\n'
        f'{pos}'
        '      </p>\n'
        f'{link_btn}'
        '    </div>\n'
        f'{strip}'
        f'    {note}\n'
        f'{lb}'
        '  </section>'
    )


def _run_facts(data: dict) -> str:
    s = data["stats"]
    md = s["models_detail"]
    models_txt = ", ".join(d["label"] for d in md) if md else "—"
    cand = (f'{s["generated"]} generated' if s["generated"] == s["requested"]
            else f'{s["generated"]} of {s["requested"]} generated')
    sizes = " / ".join(x.replace("x", X) for x in s["requested_sizes"]) or "—"
    est = ""
    if isinstance(s.get("estimated_usd"), (int, float)):
        est = f' <span class="fact-aside">(est. ~{_money(s["estimated_usd"])})</span>'
    rows = [
        ("Intent", html.escape(data["intent"] or "unclassified")),
        ("Models", f'{s["models"]} &mdash; {html.escape(models_txt)}'),
        ("Candidates", cand),
        ("Requested size", html.escape(sizes)),
        ("Actual cost", f'{_money(s["actual_usd"])}{est}'),
        ("Generated", html.escape(data["generated_on"])),
    ]
    cells = "".join(
        f'<div class="fact"><span class="fact-label">{lab}</span><p>{val}</p></div>'
        for lab, val in rows
    )
    return (
        '  <section class="run-facts">\n'
        '    <div class="col">\n'
        + _head("Details", "The run",
                "What Image Lab planned, and what it actually cost.")
        + f'      <div class="facts">{cells}</div>\n'
        + '    </div>\n'
        + '  </section>'
    )


def _prompt_section(data: dict) -> str:
    core = html.escape(data["prompt_core"] or data["prompt"])
    trig = html.escape(data["prompt_trigger"])
    return (
        '  <section class="prompt">\n'
        '    <div class="col">\n'
        + _head("Reuse", "The prompt",
                "The exact brief this run used. Copy it, or adapt it.")
        + f'      <blockquote class="prompt-core">{core}</blockquote>\n'
        + f'      <p class="prompt-trigger">&rarr; {trig}</p>\n'
        + '      <button class="copy btn-ghost" data-target="copy-src">Copy prompt</button>\n'
        + '      <ol class="how">\n'
        + '        <li>Copy the prompt above.</li>\n'
        + '        <li>Paste it into any agent that has the Image Lab skill &mdash; or install it below.</li>\n'
        + '        <li>Run it as-is, or swap the subject, style, or details to make it yours.</li>\n'
        + '      </ol>\n'
        + f'      <span id="copy-src" hidden>{html.escape(data["prompt_copy"])}</span>\n'
        + '    </div>\n'
        + '  </section>'
    )


def _credits(data: dict) -> str:
    s = data["stats"]
    tools = [
        (_model_link("CogFoundry model gateway", ATTRIBUTION_URL, "tn"),
         f'Ran every generation in this run through <code>POST /api/v1/tasks/generations</code> '
         f'and reported the real per-image cost. Run total: {_money(s["actual_usd"])}.'),
    ]
    for d in s["models_detail"]:
        labs = ", ".join(d["labels"])
        tools.append((_model_link(d["label"], d["url"], "tn"),
                      f'{_plural(d["count"], "alternative")} ({labs}).'))
    tools.append((_model_link("Image Lab", SKILL_URL, "tn"),
                  'A loomloom community example &mdash; the skill that classified the brief, '
                  'chose the model allocation, gated the cost, and rendered this page.'))
    lis = "".join(
        f'<li><span class="tname">{name}</span>'
        f'<span class="trole">{role}</span></li>'
        for name, role in tools
    )
    lead = ('Image Lab is not a benchmark &mdash; different models and seeds make these '
            'an unfair comparison. It turns one brief into strong choices. Everything '
            'below did real, billed work in this run.')
    return (
        '  <section class="credits">\n'
        '    <div class="col">\n'
        + _head("Provenance", "How this was made", lead)
        + f'      <ul class="tool-list">{lis}</ul>\n'
        + '    </div>\n'
        + '  </section>'
    )


def _coming_soon() -> str:
    return (
        '  <section class="coming-soon">\n'
        '    <div class="col">\n'
        + _head("Roadmap", "From image exploration to AI work.",
                "loomloom will bring agentic workflows to Image Lab &mdash; turning a single "
                "prompt into an iterative process of generation, evaluation, selection, and "
                "refinement.")
        + '    </div>\n'
        + '  </section>'
    )


def _cta() -> str:
    return (
        '  <section class="final-cta">\n'
        '    <div class="col">\n'
        + _head("Your turn", "Bring your own prompt",
                "Install Image Lab, give it any image prompt, and run cost-gated exploration "
                "across the best-fit models.")
        + f'      <div class="cta-cmd"><code>{html.escape(INSTALL_CMD)}</code></div>\n'
        + f'      <a class="btn btn-solid" href="{SKILL_URL}" target="_blank" rel="noopener">'
        + 'See Image Lab on GitHub</a>\n'
        + '    </div>\n'
        + '  </section>'
    )


def _og_tags(data: dict, canonical: str | None, og_asset: str | None) -> str:
    t = html.escape(data["title"])
    d = html.escape(data["summary"])
    img_rel = og_asset or "assets/alternative-01.png"
    lines = []
    if canonical:
        base = canonical if canonical.endswith("/") else canonical + "/"
        lines.append(f'<link rel="canonical" href="{html.escape(base)}index.html">')
        img = html.escape(base + img_rel)
    else:
        img = html.escape(img_rel)
    lines += [
        f'<meta name="description" content="{d}">',
        '<meta property="og:type" content="website">',
        f'<meta property="og:title" content="{t}">',
        f'<meta property="og:description" content="{d}">',
        f'<meta property="og:image" content="{img}">',
        '<meta name="twitter:card" content="summary_large_image">',
        f'<meta name="twitter:title" content="{t}">',
        f'<meta name="twitter:description" content="{d}">',
        f'<meta name="twitter:image" content="{img}">',
    ]
    return "\n".join(lines)


def _locate(source_file: str, from_dir: Path) -> Path | None:
    src = Path(source_file)
    for cand in ([src] if src.is_absolute() else [from_dir / src]) + [from_dir / src.name]:
        if cand.exists():
            return cand
    return None


def _finalize_round(data: dict, from_dir: Path, out_dir: Path) -> str | None:
    """Copies this round's images into out_dir/assets (already asset_prefix-scoped
    by assemble()), recomputes the stats/selection fields that depend on which
    images actually made it to disk, and returns the OG-image asset path (or
    None). Mutates `data` in place — one round's worth of what `render()` used
    to do inline before this page had to support more than one round."""
    assets = out_dir / "assets"
    assets.mkdir(parents=True, exist_ok=True)
    for a in data["alternatives"]:
        if a["asset"] and a["source_file"]:
            src = _locate(a["source_file"], from_dir)
            if src:
                shutil.copyfile(src, out_dir / a["asset"])
            else:
                print(f"warning: image for alternative {a['index']:02d} not found "
                      f"({a['source_file']}) — rendering it as not generated", file=sys.stderr)
                a["asset"] = None

    data["stats"]["generated"] = sum(1 for a in data["alternatives"] if a["asset"])
    data["stats"]["models_detail"] = [
        d for d in data["stats"]["models_detail"]
        if any(a["asset"] and a["model_label"] == d["label"] for a in data["alternatives"])
    ]
    if data.get("summary_is_auto"):
        data["summary"] = _tagline(data["stats"]["models"],
                                   data["stats"]["generated"], data["subject"])
    if data["selected"] and not any(
            a["label"] == data["selected"]["label"] and a["asset"]
            for a in data["alternatives"]):
        print(f"warning: selected alternative {data['selected']['label']} has no image "
              f"— dropping the selection", file=sys.stderr)
        data["selected"] = None
        for a in data["alternatives"]:
            a["selected"] = False

    shown = [a for a in data["alternatives"] if a["asset"]]
    og_asset = None
    if data["selected"] and data["selected"].get("asset"):
        og_asset = data["selected"]["asset"]
    elif shown:
        og_asset = shown[0]["asset"]
    return og_asset


def _round_nav_html(order: list[int], idx: int) -> str:
    """A prev/current/next strip — sits just above the Gallery section, inside
    the swappable round content (so its own numbers/targets always match
    whichever round is currently shown). Empty when there's only one round —
    nothing to navigate between yet. Wraps at both ends (round 1's "prev" is
    the last round, and vice versa) rather than disabling a button; each side
    names the SPECIFIC round it goes to, not a generic "switch", and carries
    that round's index directly in data-round-index so the click handler
    doesn't need to recompute wraparound arithmetic itself."""
    n = len(order)
    if n <= 1:
        return ""
    prev_i, next_i = (idx - 1) % n, (idx + 1) % n
    return (
        '  <nav class="round-nav" aria-label="Switch which round is shown">\n'
        f'    <button type="button" class="round-nav-btn prev" id="round-prev" '
        f'data-round-index="{prev_i}">&#8592; VIEW ROUND {order[prev_i]}</button>\n'
        f'    <span class="round-nav-label">ROUND {order[idx]} / {n}</span>\n'
        f'    <button type="button" class="round-nav-btn next" id="round-next" '
        f'data-round-index="{next_i}">VIEW ROUND {order[next_i]} &#8594;</button>\n'
        '  </nav>'
    )


def _round_head_body(data: dict) -> tuple[str, str]:
    """The part of the page that's specific to ONE round: everything currently
    inside <header> below the topbar, plus everything in <main> above the
    session-level coming_soon/cta/footer. Built from the exact same section
    functions a single-round page always used — nothing about their output
    changes; only how many times they're called does. The round-nav strip (if
    any) is prepended by the caller, which has the cross-round context
    (ordering, current index) this function doesn't."""
    shown = [a for a in data["alternatives"] if a["asset"]]
    badge = ('<p class="hero-badge">AI-generated &middot; not a benchmark</p>\n'
             if shown else "")
    head_html = (
        f'<p class="hero-eyebrow">{html.escape(data["kicker"])}</p>\n'
        f'{badge}    <h1>{html.escape(data["title"])}</h1>\n'
        f'    <p class="summary">{html.escape(data["summary"])}</p>\n'
        f'{_legend(data)}    <p class="stats">{_stat_line(data["stats"])}</p>'
    )
    body_html = (
        f'{_gallery(data)}\n\n{_run_facts(data)}\n\n'
        f'{_prompt_section(data)}\n\n{_credits(data)}'
    )
    return head_html, body_html


def render_session(rounds_data: list[tuple[int, dict]], session_root: Path, out_dir: Path,
                    canonical: str | None = None) -> None:
    """rounds_data: [(round_number, data), ...] in ascending round order — every
    round that has ever been published to this page, each already run through
    assemble() with THAT round's own title/subject/etc. Builds one page whose
    initial (server-rendered, no-JS-safe) state is the LATEST round, with every
    other round embedded as data for the round-nav strip to swap in
    client-side — no reload, no rebuild needed just to look at an earlier round.
    """
    assets_dir = out_dir / "assets"
    order = [n for n, _ in rounds_data]
    rounds_json = []
    latest_data, latest_og_asset = None, None
    for i, (round_no, data) in enumerate(rounds_data):
        from_dir = session_root / f"round-{round_no}"
        og_asset = _finalize_round(data, from_dir, out_dir)
        head_html, body_html = _round_head_body(data)
        body_html = f'{_round_nav_html(order, i)}\n\n{body_html}'.strip()
        rounds_json.append({"round": round_no, "title": data["title"],
                             "head": head_html, "body": body_html})
        latest_data, latest_og_asset = data, og_asset

    attr = latest_data["attribution"]
    rounds_js = json.dumps(rounds_json).replace("</script>", "<\\/script>")
    page = PAGE.format(
        title=html.escape(latest_data["title"]),
        og_tags=_og_tags(latest_data, canonical, latest_og_asset),
        round_head=rounds_json[-1]["head"],
        round_body=rounds_json[-1]["body"],
        rounds_json=rounds_js,
        coming_soon=_coming_soon(),
        cta=_cta(),
        attr_text=html.escape(attr["text"]),
        attr_url=html.escape(attr["url"]),
        generated_on=latest_data["generated_on"],
    )
    (out_dir / "index.html").write_text(page, encoding="utf-8")
    slim = [
        {k: v for k, v in d.items() if k not in ("alternatives", "summary_is_auto")}
        | {"round": n, "alternatives": [{k: v for k, v in a.items() if k != "source_file"}
                                         for a in d["alternatives"]]}
        for n, d in rounds_data
    ]
    (out_dir / "case-study-data.json").write_text(json.dumps(slim, indent=2), encoding="utf-8")


# --------------------------------------------------------------------------- #
# A single-file page: styles + the gallery/lightbox script inline, images as
# sibling files (the static folder stays lean; `--inline` makes the one-file
# copy for publishing). `rebuildMeta()` in the script mirrors `_meta_html()`.
# GitHub-Pages ready.
PAGE = """<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>{title}</title>
{og_tags}
<link rel="icon" href="data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 32 32'%3E%3Crect width='32' height='32' fill='%23007a5c'/%3E%3C/svg%3E">
<link rel="preconnect" href="https://fonts.googleapis.com">
<link rel="preconnect" href="https://fonts.gstatic.com" crossorigin>
<link rel="stylesheet" href="https://fonts.googleapis.com/css2?family=IBM+Plex+Mono:wght@400;500;700&family=Source+Serif+4:ital,wght@0,400;0,600;1,400&display=swap">
<style>
  :root {{
    --bg:#f4f4f0; --surface:#ffffff; --ink:#0b0b0b; --muted:#4a4a46; --faint:#68675e;
    --accent:#007a5c; --accent-fill:#00fdbc; --on-accent-fill:#062017;
    --rule:#d8d6ce; --rule-strong:#0b0b0b; --shadow:0 1px 3px rgba(11,11,11,.09);
    --prose:820px;
    --serif:"Source Serif 4", Georgia, "Times New Roman", serif;
    --mono:"IBM Plex Mono", ui-monospace, "SFMono-Regular", Menlo, Consolas, monospace;
    --sans:Arial, "Helvetica Neue", Helvetica, sans-serif;
  }}
  @media (prefers-color-scheme: dark) {{
    :root:not([data-theme="light"]) {{
      --bg:#15150f; --surface:#1d1d16; --ink:#f2f1ea; --muted:#b3b0a5; --faint:#726f63;
      --accent:#00fdbc; --accent-fill:#00fdbc; --on-accent-fill:#062017;
      --rule:#33322a; --rule-strong:#f2f1ea; --shadow:0 1px 3px rgba(0,0,0,.45);
    }}
  }}
  :root[data-theme="dark"] {{
    --bg:#15150f; --surface:#1d1d16; --ink:#f2f1ea; --muted:#b3b0a5; --faint:#726f63;
    --accent:#00fdbc; --accent-fill:#00fdbc; --on-accent-fill:#062017;
    --rule:#33322a; --rule-strong:#f2f1ea; --shadow:0 1px 3px rgba(0,0,0,.45);
  }}
  * {{ box-sizing: border-box; }}
  html {{ -webkit-text-size-adjust: 100%; }}
  body {{
    margin: 0; background: var(--bg); color: var(--ink);
    font-family: var(--serif); font-size: 18px; line-height: 1.6; overflow-x: hidden;
  }}
  a {{ color: var(--accent); }}
  .wrap {{ max-width: 1200px; margin: 0 auto; padding: 0 24px; }}
  .col {{ max-width: var(--prose); margin-left: auto; margin-right: auto; }}
  html:not(.js) .js-only,
  html:not(.js) .hero-open,
  html:not(.js) .hero-nav {{ display: none !important; }}

  /* ---- shared buttons ---- */
  .btn-ghost {{
    font-family: var(--mono); font-size: 10px; font-weight: 700; text-transform: uppercase;
    letter-spacing: .07em; cursor: pointer; background: none; text-decoration: none;
    border: 1px solid var(--rule-strong); color: var(--ink); padding: 7px 13px;
  }}
  .btn-ghost:hover {{ background: var(--ink); color: var(--bg); }}
  .btn-ghost:active {{ transform: translateY(1px); }}
  .btn-ghost:focus-visible {{ outline: 2px solid var(--accent); outline-offset: 2px; }}
  .btn-solid {{
    font-family: var(--mono); font-weight: 700; text-transform: uppercase; letter-spacing: .07em;
    background: var(--ink); color: var(--bg); border: 0; cursor: pointer; text-decoration: none;
    box-shadow: var(--shadow);
  }}
  .btn-solid:hover {{ background: var(--accent); color: var(--on-accent-fill); }}
  .btn-solid:focus-visible {{ outline: 2px solid var(--accent); outline-offset: 2px; }}

  /* ---- topbar ---- */
  .topbar {{ border-bottom: 2px solid var(--rule-strong); }}
  .topbar-in {{
    display: flex; align-items: center; justify-content: space-between; gap: 14px;
    padding-top: 14px; padding-bottom: 14px;
  }}
  .wordmark {{
    font-family: var(--mono); font-weight: 700; font-size: 12px; letter-spacing: .16em;
    text-transform: uppercase; color: var(--accent); flex-shrink: 0;
  }}
  .share {{ display: flex; align-items: center; gap: 6px; flex-shrink: 0; }}
  .share-lbl {{
    font-family: var(--mono); font-size: 10px; font-weight: 700; letter-spacing: .12em;
    text-transform: uppercase; color: var(--faint); margin-right: 2px;
  }}
  @media (max-width: 560px) {{ .share-lbl {{ display: none; }} }}

  /* ---- header ---- */
  header {{ border-bottom: 2px solid var(--rule-strong); padding: 56px 0 44px; }}
  .hero-eyebrow {{
    font-family: var(--mono); font-size: 12px; letter-spacing: .18em; text-transform: uppercase;
    color: var(--accent); font-weight: 700; margin: 0 0 18px;
  }}
  .hero-badge {{
    display: inline-block; font-family: var(--mono); font-size: 10px; letter-spacing: .1em;
    text-transform: uppercase; background: var(--accent-fill); color: var(--on-accent-fill);
    padding: 4px 10px; margin: 0 0 18px;
  }}
  h1 {{
    font-family: var(--sans); font-weight: 900; text-transform: uppercase;
    font-size: clamp(2rem, 6vw, 3.1rem); line-height: 1.03; letter-spacing: -.02em;
    margin: 0 0 20px; text-wrap: balance;
  }}
  .summary {{ font-size: 1.22rem; line-height: 1.5; max-width: 34ch; margin: 0 0 4px; color: var(--ink); }}
  .legend {{ list-style: none; margin: 18px 0 0; padding: 0; display: flex; flex-wrap: wrap; gap: 8px; }}
  .legend li {{
    font-family: var(--mono); font-size: 11px; text-transform: uppercase; letter-spacing: .04em;
    border: 1px solid var(--rule); padding: 5px 10px; display: flex; gap: 7px; align-items: baseline;
  }}
  .legend .lm {{ color: var(--ink); text-decoration: none; border-bottom: 1px solid var(--accent); }}
  .legend .x {{ color: var(--accent); font-weight: 700; }}
  .stats {{
    font-family: var(--mono); font-size: 11px; letter-spacing: .06em; text-transform: uppercase;
    color: var(--faint); margin: 18px 0 0;
  }}

  section {{ padding: 52px 0; border-bottom: 2px solid var(--rule); }}
  section:last-of-type {{ border-bottom: none; }}
  .section-label {{
    display: block; font-family: var(--mono); font-size: 11px; font-weight: 700;
    letter-spacing: .14em; text-transform: uppercase; color: var(--accent); margin: 0 0 14px;
  }}
  h2 {{
    font-family: var(--sans); font-weight: 900; text-transform: uppercase;
    font-size: 1.7rem; line-height: 1.08; letter-spacing: -.01em; margin: 0 0 20px;
  }}
  .lead {{
    font-size: 1.05rem; line-height: 1.55; color: var(--muted);
    margin: 0 0 26px; max-width: 60ch;
  }}

  /* ---- round-nav: sits just above Gallery, inside the swappable round
     content, so its own round numbers always match what's shown ---- */
  .round-nav {{
    display: flex; align-items: center; justify-content: space-between; gap: 12px;
    padding: 22px 0; margin-bottom: 30px; border-bottom: 2px solid var(--rule-strong);
  }}
  .round-nav-btn {{
    font-family: var(--mono); font-size: 13px; font-weight: 700; text-transform: uppercase;
    letter-spacing: .05em; background: none; border: 0; cursor: pointer; color: var(--ink);
    padding: 4px 0;
  }}
  .round-nav-btn:hover {{ color: var(--accent); }}
  .round-nav-btn:focus-visible {{ outline: 2px solid var(--accent); outline-offset: 2px; }}
  .round-nav-label {{
    font-family: var(--mono); font-size: 13px; font-weight: 700; text-transform: uppercase;
    letter-spacing: .1em; color: var(--accent); white-space: nowrap;
  }}
  @media (max-width: 560px) {{
    .round-nav {{ flex-wrap: wrap; justify-content: center; text-align: center; }}
    .round-nav-label {{ order: -1; flex-basis: 100%; margin-bottom: 6px; }}
  }}

  /* ---- gallery ---- */
  .hero-frame {{
    /* fixed medium-dark grey, not the --surface variable, so it stays dark
       in both site themes */
    position: relative; border: 2px solid var(--rule-strong); background: #58584f;
  }}
  .hero-frame a {{ display: block; cursor: zoom-in; }}
  .hero-frame img {{
    width: 100%; height: auto; max-height: 78vh; object-fit: contain; display: block;
  }}
  .hero-open {{
    position: absolute; top: 0; right: 0; z-index: 2; display: block;
    font-size: 10px; padding: 7px 11px;
  }}
  .hero-nav {{
    position: absolute; top: 50%; transform: translateY(-50%); z-index: 2;
    width: 40px; height: 60px; border: 0; cursor: pointer; padding: 0;
    background: var(--ink); color: var(--accent-fill); font-size: 20px; line-height: 1;
    opacity: 0; transition: opacity .12s;
  }}
  .hero-frame:hover .hero-nav {{ opacity: .9; }}
  .hero-nav:focus-visible {{ opacity: 1; outline: 2px solid var(--accent-fill); }}
  .hero-nav.prev {{ left: 0; }}
  .hero-nav.next {{ right: 0; }}
  .hero-nav[disabled] {{ display: none; }}
  .gallery-meta {{
    font-family: var(--mono); font-size: 12px; letter-spacing: .02em; text-transform: uppercase;
    color: var(--faint); margin: 14px 0 0;
  }}
  .gallery-meta .model {{ color: var(--ink); font-weight: 700; }}
  .gallery-meta a.model {{ text-decoration: none; border-bottom: 1px solid var(--accent); }}
  .gallery-meta .chose {{ color: var(--accent); font-weight: 700; margin-right: .5rem; }}
  .gallery-hint {{
    font-family: var(--mono); font-size: 10px; letter-spacing: .07em; text-transform: uppercase;
    color: var(--faint); margin: 8px 0 0; display: flex; justify-content: space-between; gap: 14px;
  }}
  .gallery-hint .pos {{ color: var(--ink); font-weight: 700; font-variant-numeric: tabular-nums; }}
  .link-btn {{ margin: 14px 0 0; }}

  .gallery-thumbs {{ display: flex; flex-wrap: wrap; gap: 12px; margin-top: 24px; }}
  .thumb {{
    position: relative; flex: 1 1 150px; max-width: 220px;
    padding: 0; border: 1px solid var(--rule); background: var(--surface); cursor: pointer;
    display: flex; flex-direction: column; text-align: left;
    transition: box-shadow .1s, border-color .1s;
  }}
  .thumb .shot {{ position: relative; aspect-ratio: 3 / 2; }}
  .thumb .shot img {{ width: 100%; height: 100%; object-fit: cover; display: block; }}
  .thumb:hover {{ border-color: var(--muted); }}
  .thumb.active {{ border-color: var(--rule-strong); box-shadow: 0 0 0 2px var(--accent-fill); }}
  .thumb-cap {{
    font-family: var(--mono); font-size: 10px; font-weight: 700; letter-spacing: .04em;
    text-transform: uppercase; color: var(--ink); border-top: 1px solid var(--rule);
    padding: 4px 7px; display: flex; gap: 5px; align-items: baseline;
  }}
  .thumb-cap .ltr {{ color: var(--accent); font-size: 11px; }}
  .thumb-cap .ltr::after {{ content: " \\00b7"; color: var(--faint); }}
  .thumb-cap .m {{ font-weight: 400; color: var(--faint); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }}
  .thumb.active .thumb-cap {{ background: var(--accent-fill); border-top-color: var(--accent-fill); }}
  .thumb.active .thumb-cap .ltr, .thumb.active .thumb-cap .m {{ color: var(--on-accent-fill); }}
  .thumb-badge {{
    position: absolute; right: 0; top: 0; width: 18px; height: 18px;
    background: var(--accent-fill); color: var(--on-accent-fill);
    font-size: .72rem; display: grid; place-items: center;
  }}
  .gallery-note {{ font-family: var(--mono); font-size: 11px; text-transform: uppercase; letter-spacing: .05em; color: var(--faint); margin-top: 16px; }}
  @media (max-width: 640px) {{
    .gallery-thumbs {{ flex-wrap: nowrap; overflow-x: auto; -webkit-overflow-scrolling: touch; padding-bottom: 6px; }}
    .thumb {{ flex: 0 0 64%; max-width: none; }}
  }}

  /* ---- lightbox (a deliberate single-look full-screen viewer) ---- */
  .lightbox {{
    position: fixed; inset: 0; z-index: 50; background: rgba(8, 8, 6, .93);
    display: flex; align-items: center; justify-content: center; padding: 5vmin;
  }}
  .lightbox[hidden] {{ display: none; }}
  .lb-figure {{ margin: 0; max-width: 100%; max-height: 100%; display: flex; flex-direction: column; gap: 12px; }}
  .lb-figure img {{
    max-width: 100%; max-height: 82vh; object-fit: contain; display: block; margin: 0 auto;
    border: 1px solid rgba(255, 255, 255, .16);
  }}
  .lb-bar {{
    font-family: var(--mono); font-size: 11px; letter-spacing: .04em; text-transform: uppercase;
    color: #d8d6ce; display: flex; justify-content: space-between; gap: 16px; flex-wrap: wrap;
  }}
  .lb-bar .model {{ color: #fff; font-weight: 700; }}
  .lb-bar a, .lb-bar a.model {{ color: var(--accent-fill); border: 0; text-decoration: none; }}
  .lb-right {{ display: flex; gap: 16px; align-items: baseline; font-variant-numeric: tabular-nums; }}
  .lb-linkbtn {{
    font-family: var(--mono); font-size: 11px; text-transform: uppercase; letter-spacing: .04em;
    background: none; border: 0; color: var(--accent-fill); cursor: pointer; padding: 0;
  }}
  .lb-close, .lb-nav {{
    position: absolute; background: rgba(255, 255, 255, .12); color: #fff; border: 0; cursor: pointer;
    font-family: var(--mono); line-height: 1;
  }}
  .lb-close {{ top: 3vmin; right: 3vmin; width: 40px; height: 40px; font-size: 22px; }}
  .lb-nav {{ top: 50%; transform: translateY(-50%); width: 44px; height: 64px; font-size: 22px; }}
  .lb-nav.prev {{ left: 3vmin; }}
  .lb-nav.next {{ right: 3vmin; }}
  .lb-nav[disabled] {{ opacity: .3; cursor: default; }}
  .lb-close:hover, .lb-nav:not([disabled]):hover {{ background: var(--accent); color: var(--on-accent-fill); }}
  .lb-close:focus-visible, .lb-nav:focus-visible {{ outline: 2px solid var(--accent-fill); }}

  /* ---- run facts (hairline grid) ---- */
  .facts {{
    display: grid; grid-template-columns: repeat(3, 1fr); gap: 1px;
    background: var(--rule); border: 1px solid var(--rule);
  }}
  .fact {{ background: var(--surface); padding: 14px 16px; }}
  .fact-label {{
    display: block; font-family: var(--mono); font-size: 10px; letter-spacing: .1em;
    text-transform: uppercase; color: var(--faint); margin-bottom: 6px;
  }}
  .fact p {{ margin: 0; font-size: 13px; color: var(--muted); font-family: var(--mono); line-height: 1.5; word-break: break-word; }}
  .fact-aside {{ color: var(--faint); }}
  @media (max-width: 640px) {{ .facts {{ grid-template-columns: 1fr; }} }}

  /* ---- prompt ---- */
  .prompt-core {{
    margin: 0; background: var(--surface); border: 1px solid var(--rule);
    border-left: 3px solid var(--accent); padding: 20px 24px;
    font-family: var(--serif); font-style: italic; font-size: 1.05rem; line-height: 1.6;
    color: var(--ink); white-space: pre-wrap; word-break: break-word;
  }}
  .prompt-trigger {{
    font-family: var(--mono); font-size: 12px; text-transform: uppercase; letter-spacing: .06em;
    color: var(--faint); margin: 14px 0 0;
  }}
  button.copy {{ margin-top: 18px; }}
  .how {{ counter-reset: how; list-style: none; margin: 24px 0 0; padding: 0; }}
  .how li {{
    counter-increment: how; display: flex; gap: 14px; padding: 11px 0;
    border-top: 1px solid var(--rule); font-size: 15px; color: var(--ink); line-height: 1.5;
  }}
  .how li:first-child {{ border-top: none; padding-top: 0; }}
  .how li::before {{
    content: counter(how); flex-shrink: 0; font-family: var(--mono); font-weight: 700;
    font-size: 12px; color: var(--accent); padding-top: 2px;
  }}

  /* ---- credits ---- */
  .tool-list {{ list-style: none; margin: 0; padding: 0; }}
  .tool-list li {{ padding: 12px 0; border-top: 1px solid var(--rule); }}
  .tool-list li:first-child {{ border-top: none; padding-top: 0; }}
  .tool-list .tname {{ font-family: var(--mono); font-size: 13px; font-weight: 700; }}
  .tool-list .tn {{ color: var(--ink); text-decoration: none; border-bottom: 1px solid var(--accent); }}
  .tool-list .trole {{ display: block; font-size: 14px; color: var(--muted); margin-top: 4px; line-height: 1.5; }}
  .tool-list code, .cta-cmd code {{
    font-family: var(--mono); font-size: 12px; background: var(--surface);
    border: 1px solid var(--rule); padding: 0 4px;
  }}

  /* ---- coming soon: uses only the shared .lead ---- */

  /* ---- final CTA ---- */
  .final-cta .col {{ text-align: center; }}
  .final-cta .lead {{ margin-left: auto; margin-right: auto; max-width: 46ch; }}
  .cta-cmd {{ max-width: 660px; margin: 0 auto 24px; }}
  .cta-cmd code {{ display: block; padding: 11px 14px; overflow-x: auto; text-align: left; }}
  .final-cta .btn {{ display: inline-block; font-size: 12px; padding: 13px 26px; }}

  footer {{
    padding: 30px 0 46px; font-family: var(--mono); font-size: 11px; text-transform: uppercase;
    letter-spacing: .05em; color: var(--faint); text-align: center;
  }}
  footer a {{ color: var(--faint); }}
  @media (prefers-reduced-motion: reduce) {{ * {{ transition: none !important; }} }}
</style>
<script>document.documentElement.className += " js";</script>
</head>
<body>
<div class="topbar">
  <div class="wrap topbar-in">
    <span class="wordmark">Image Lab</span>
    <div class="share js-only">
      <span class="share-lbl">Share</span>
      <button type="button" class="btn-ghost" id="copy-page" aria-label="Copy link to this page">Copy link</button>
      <a class="btn-ghost" id="share-x" target="_blank" rel="noopener" aria-label="Share on X">X</a>
      <a class="btn-ghost" id="share-li" target="_blank" rel="noopener" aria-label="Share on LinkedIn">LinkedIn</a>
    </div>
  </div>
</div>
<header>
  <div class="wrap" id="round-head">
{round_head}
  </div>
</header>

<main class="wrap">
  <div id="round-body">
{round_body}
  </div>

{coming_soon}

{cta}

  <footer>
    {attr_text} &middot; <a href="{attr_url}" target="_blank" rel="noopener">cogfoundry.ai</a> &middot; {generated_on}
  </footer>
</main>
<script>
  function initGallery() {{
    var thumbs = Array.prototype.slice.call(document.querySelectorAll("#round-body .thumb"));
    var hero = document.getElementById("hero-img");
    var link = document.getElementById("hero-link");
    var meta = document.getElementById("hero-meta");
    var posEl = document.getElementById("hero-pos");
    if (!hero || !thumbs.length) return;
    var MUL = "\\u00d7";
    function esc(s) {{
      return String(s).replace(/[&<>"]/g, function (c) {{
        return {{ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;" }}[c];
      }});
    }}
    // rebuildMeta() — keep the shape in sync with _meta_html() in the builder
    function rebuildMeta(d) {{
      var size = d.size.replace("x", MUL);
      if (d.asked && d.asked !== d.size) size += " (asked " + d.asked.replace("x", MUL) + ")";
      var secs = d.seconds ? " \\u00b7 " + esc(d.seconds) + "s" : "";
      var model = d.modelUrl
        ? '<a class="model" href="' + esc(d.modelUrl) + '" target="_blank" rel="noopener">' + esc(d.model) + "</a>"
        : '<span class="model">' + esc(d.model) + "</span>";
      var chose = d.selected ? '<span class="chose">the creator\\u2019s pick</span> ' : "";
      return chose + "Alternative " + esc(d.label) + " \\u00b7 " + model
        + " \\u00b7 " + esc(size) + " \\u00b7 " + esc(d.cost) + secs;
    }}

    var lb = document.getElementById("lightbox");
    var lbImg = document.getElementById("lb-img");
    var lbMeta = document.getElementById("lb-meta");
    var lbPos = document.getElementById("lb-pos");
    var lbRaw = document.getElementById("lb-raw");
    var heroPrev = document.getElementById("hero-prev");
    var heroNext = document.getElementById("hero-next");
    var lbPrev = document.getElementById("lb-prev");
    var lbNext = document.getElementById("lb-next");
    var altLinkBtn = document.getElementById("copy-alt-link");
    var lbCopyLink = document.getElementById("lb-copy-link");
    var cur = 0, lastFocus = null;

    function baseHref() {{ return location.href.split("#")[0]; }}
    function shareUrl() {{ return baseHref() + "#" + thumbs[cur].dataset.label; }}
    function flash(btn, msg) {{
      try {{ if (navigator.clipboard) navigator.clipboard.writeText(msg); }} catch (e) {{}}
      var o = btn.getAttribute("data-label-text") || btn.textContent;
      btn.textContent = "Copied"; setTimeout(function () {{ btn.textContent = o; }}, 1400);
    }}
    function updateNav() {{
      [heroPrev, lbPrev].forEach(function (b) {{ if (b) b.disabled = cur <= 0; }});
      [heroNext, lbNext].forEach(function (b) {{ if (b) b.disabled = cur >= thumbs.length - 1; }});
    }}
    function fillLB(d) {{
      if (!lb) return;
      lbImg.src = d.asset;
      lbImg.alt = "Alternative " + d.label;
      lbMeta.innerHTML = rebuildMeta(d);
      lbPos.textContent = (cur + 1) + " / " + thumbs.length;
      lbRaw.href = d.asset;
    }}
    function show(t, keepHash) {{
      var i = thumbs.indexOf(t);
      if (i < 0) return;
      cur = i;
      thumbs.forEach(function (x) {{
        var on = x === t;
        x.classList.toggle("active", on);
        if (on) x.setAttribute("aria-current", "true"); else x.removeAttribute("aria-current");
      }});
      var d = t.dataset;
      hero.src = d.asset;
      hero.alt = "Alternative " + d.label;
      if (link) link.href = d.asset;
      meta.innerHTML = rebuildMeta(d);
      if (posEl) posEl.textContent = (i + 1) + " / " + thumbs.length;
      if (altLinkBtn) {{
        altLinkBtn.textContent = "Copy link to alternative " + d.label;
        altLinkBtn.setAttribute("data-label-text", altLinkBtn.textContent);
      }}
      if (lb && !lb.hidden) fillLB(d);
      updateNav();
      if (!keepHash) {{ try {{ history.replaceState(null, "", "#" + d.label); }} catch (e) {{}} }}
    }}
    function step(dir) {{
      var i = cur + dir;
      if (i >= 0 && i < thumbs.length) show(thumbs[i]);
    }}
    function openLB() {{
      if (!lb) return;
      lastFocus = document.activeElement;
      fillLB(thumbs[cur].dataset);
      lb.hidden = false;
      document.body.style.overflow = "hidden";
      updateNav();
      document.getElementById("lb-close").focus();
    }}
    function closeLB() {{
      if (!lb || lb.hidden) return;
      lb.hidden = true;
      document.body.style.overflow = "";
      if (lastFocus && lastFocus.focus) lastFocus.focus();
    }}

    thumbs.forEach(function (t) {{ t.addEventListener("click", function () {{ show(t); }}); }});
    if (heroPrev) heroPrev.addEventListener("click", function () {{ step(-1); }});
    if (heroNext) heroNext.addEventListener("click", function () {{ step(1); }});
    var openBtn = document.getElementById("hero-open");
    if (openBtn) openBtn.addEventListener("click", openLB);
    if (link) link.addEventListener("click", function (e) {{
      if (lb) {{ e.preventDefault(); openLB(); }}
    }});
    if (lbPrev) lbPrev.addEventListener("click", function () {{ step(-1); }});
    if (lbNext) lbNext.addEventListener("click", function () {{ step(1); }});
    var lbClose = document.getElementById("lb-close");
    if (lbClose) lbClose.addEventListener("click", closeLB);
    if (lb) lb.addEventListener("click", function (e) {{ if (e.target === lb) closeLB(); }});
    if (altLinkBtn) altLinkBtn.addEventListener("click", function () {{ flash(altLinkBtn, shareUrl()); }});
    if (lbCopyLink) lbCopyLink.addEventListener("click", function () {{ flash(lbCopyLink, shareUrl()); }});

    handleKey = function (e) {{
      if (e.key === "Escape") {{ closeLB(); return; }}
      if (e.key === "ArrowRight") step(1);
      else if (e.key === "ArrowLeft") step(-1);
    }};

    function selectFromHash(keepHash) {{
      var w = decodeURIComponent(location.hash.replace(/^#/, "")).toUpperCase();
      if (!w) return;
      var t = thumbs.filter(function (x) {{ return x.dataset.label.toUpperCase() === w; }})[0];
      if (t) show(t, keepHash);
    }}
    cur = Math.max(0, thumbs.findIndex(function (x) {{ return x.classList.contains("active"); }}));
    selectFromHash(true);
    handleHashChange = function () {{ selectFromHash(true); }};
    updateNav();
  }}

  function initPromptCopy() {{
    document.querySelectorAll("#round-body button.copy").forEach(function (b) {{
      b.addEventListener("click", function () {{
        var el = document.getElementById(b.dataset.target);
        navigator.clipboard.writeText(el ? el.textContent : "").then(function () {{
          var old = b.textContent; b.textContent = "Copied";
          setTimeout(function () {{ b.textContent = old; }}, 1400);
        }});
      }});
    }});
  }}

  // handleKey/handleHashChange are reassigned by initGallery() on every round
  // switch; registered once here (not inside initGallery) so switching rounds
  // never stacks up duplicate document/window-level listeners.
  var handleKey = function () {{}};
  var handleHashChange = function () {{}};
  document.addEventListener("keydown", function (e) {{ handleKey(e); }});
  window.addEventListener("hashchange", function () {{ handleHashChange(); }});

  (function () {{
    var canon = document.querySelector('link[rel="canonical"]');
    var pageUrl = canon ? canon.href : location.href.split("#")[0];
    function meta(p) {{ var m = document.querySelector('meta[property="' + p + '"], meta[name="' + p + '"]'); return m ? m.content : ""; }}
    var shareText = (meta("og:title") || document.title)
      + (meta("og:description") ? " \\u2014 " + meta("og:description") : "");
    var x = document.getElementById("share-x");
    var li = document.getElementById("share-li");
    if (x) x.href = "https://twitter.com/intent/tweet?text="
      + encodeURIComponent(shareText) + "&url=" + encodeURIComponent(pageUrl);
    if (li) li.href = "https://www.linkedin.com/sharing/share-offsite/?url="
      + encodeURIComponent(pageUrl);
    var cp = document.getElementById("copy-page");
    if (cp) cp.addEventListener("click", function () {{
      try {{ if (navigator.clipboard) navigator.clipboard.writeText(pageUrl); }} catch (e) {{}}
      var o = cp.textContent; cp.textContent = "Copied";
      setTimeout(function () {{ cp.textContent = o; }}, 1400);
    }});
  }})();

  var ROUNDS = {rounds_json};
  function initRoundNav() {{
    var prevBtn = document.getElementById("round-prev");
    var nextBtn = document.getElementById("round-next");
    [prevBtn, nextBtn].forEach(function (b) {{
      if (!b) return;
      b.addEventListener("click", function () {{
        renderRound(parseInt(b.dataset.roundIndex, 10));
      }});
    }});
  }}
  function renderRound(i) {{
    var r = ROUNDS[i];
    document.getElementById("round-head").innerHTML = r.head;
    document.getElementById("round-body").innerHTML = r.body;
    document.title = r.title;
    try {{ history.replaceState(null, "", location.pathname + location.search); }} catch (e) {{}}
    initGallery();
    initPromptCopy();
    initRoundNav();
    document.getElementById("round-body").scrollIntoView({{ block: "start", behavior: "smooth" }});
  }}

  initGallery();
  initPromptCopy();
  initRoundNav();
</script>
</body>
</html>
"""




CASE_STUDY_FILE = "case-study.json"


def _load_case_study(session_root: Path) -> dict:
    """{"slug": str|None, "rounds": {"<N>": {title, subject, summary,
    invocation, selected}}} — this page-builder's own persistent record,
    separate from image.py's session.json (which knows nothing about titles
    or page display concerns). Missing/corrupt reads as fresh — the manifest
    is a convenience record, not load-bearing for correctness."""
    p = session_root / CASE_STUDY_FILE
    if not p.exists():
        return {"slug": None, "rounds": {}}
    try:
        m = json.loads(p.read_text(encoding="utf-8"))
    except (json.JSONDecodeError, OSError):
        return {"slug": None, "rounds": {}}
    m.setdefault("slug", None)
    m.setdefault("rounds", {})
    return m


def _save_case_study(session_root: Path, manifest: dict) -> None:
    (session_root / CASE_STUDY_FILE).write_text(json.dumps(manifest, indent=2), encoding="utf-8")


def main() -> None:
    ap = argparse.ArgumentParser(prog="build-exploration-page.py")
    ap.add_argument("--session", default=None, required=True,
                    help="the session root (image.py's --out) — contains session.json and "
                         "round-N/ folders. The case-study page always lives at a fixed "
                         "<session>/<slug>/ address, set once from the first round's --title "
                         "and never moved by a later round, regardless of how many rounds exist.")
    ap.add_argument("--round", type=int, default=None,
                    help="which round this --title/--subject/etc describe (default: the latest "
                         "round recorded in <session>/session.json — the normal case, since this "
                         "is meant to be called right after that round's `image.py run` finishes)")
    ap.add_argument("--title", required=True, help="3-5 word title for this round")
    ap.add_argument("--subject", required=True,
                    help='short noun phrase for the tagline, e.g. "Murree x Toronto"')
    ap.add_argument("--invocation", default=None,
                    help="the exact message the user sent to trigger Image Lab (shown verbatim)")
    ap.add_argument("--summary", default=None,
                    help="override the auto tagline (One brief. N models. M ways to see ...)")
    ap.add_argument("--selected", default=None, help="branch label of the creator's pick (A, B, ...)")
    ap.add_argument("--slug", default=None,
                    help="URL slug — only takes effect on the FIRST round ever published for this "
                         "session (default: derived from that round's --title); ignored with a "
                         "note on later rounds, since the address must never move")
    ap.add_argument("--out", default=None, help="output folder (default: <session>/<slug>)")
    ap.add_argument("--canonical-url", dest="canonical_url", default=None,
                    help="absolute URL the folder will live at — makes og:image absolute "
                         "(link unfurls) and the topbar Copy-link copy that URL")
    ap.add_argument("--inline", action="store_true",
                    help="also write index.inline.html with the PNGs embedded as base64 "
                         "data URIs — one self-contained file to publish as an artifact")
    a = ap.parse_args()

    session_root = Path(a.session).resolve()
    session_path = session_root / "session.json"
    if not session_path.exists():
        die(f"{session_path} not found — run `image.py run --out {a.session} ...` at least once first")
    try:
        session = json.loads(session_path.read_text(encoding="utf-8"))
    except (json.JSONDecodeError, OSError) as e:
        die(f"could not read {session_path}: {e}")
    known_rounds = {r["round"]: r for r in session.get("rounds", [])}
    if not known_rounds:
        die(f"{session_path} has no recorded rounds")

    target_round = a.round if a.round is not None else max(known_rounds)
    if target_round not in known_rounds:
        die(f"round {target_round} is not in {session_path} — known rounds: {sorted(known_rounds)}")

    manifest = _load_case_study(session_root)
    if manifest["slug"] is None:
        manifest["slug"] = slugify(a.slug or a.title)
    elif a.slug and slugify(a.slug) != manifest["slug"]:
        print(f"note: this session's page address is fixed at '{manifest['slug']}' "
              f"(set on its first round) — ignoring --slug {a.slug!r}", file=sys.stderr)
    slug = manifest["slug"]

    manifest["rounds"][str(target_round)] = {
        "title": a.title, "subject": a.subject, "summary": a.summary,
        "invocation": a.invocation, "selected": a.selected,
    }
    _save_case_study(session_root, manifest)

    for rn in sorted(known_rounds):
        if str(rn) not in manifest["rounds"]:
            # A round session.json knows about but this script was never run
            # for — it silently never appears on the page. Loud here beats a
            # user wondering why an earlier round vanished from the case study.
            print(f"warning: round {rn} is recorded in {session_path} but was "
                  f"never published to the case-study page — it will not appear",
                  file=sys.stderr)

    rounds_data = []
    for round_no in sorted(int(k) for k in manifest["rounds"] if int(k) in known_rounds):
        round_meta = manifest["rounds"][str(round_no)]
        run_path = session_root / f"round-{round_no}" / "run.json"
        if not run_path.exists():
            print(f"warning: {run_path} not found — skipping round {round_no}", file=sys.stderr)
            continue
        try:
            run = json.loads(run_path.read_text(encoding="utf-8"))
        except (json.JSONDecodeError, OSError) as e:
            print(f"warning: could not read {run_path}: {e} — skipping round {round_no}",
                  file=sys.stderr)
            continue
        data = assemble(run, round_meta["title"], round_meta["subject"], round_meta["summary"],
                         round_meta["invocation"], round_meta["selected"], asset_prefix=f"r{round_no}-")
        rounds_data.append((round_no, data))
    if not rounds_data:
        die("no round with a readable run.json was found for this case-study page")

    out_dir = Path(a.out).resolve() if a.out else session_root / slug
    render_session(rounds_data, session_root, out_dir, a.canonical_url)

    inline_path = _write_inline(out_dir) if a.inline else None

    latest_round, latest_data = rounds_data[-1]
    made = latest_data["stats"]["generated"]
    print(f"case-study page: {out_dir / 'index.html'}", file=sys.stderr)
    print(f"  round {latest_round} of {len(rounds_data)} shown by default &mdash; "
          f"{made} image(s), {latest_data['stats']['models']} model(s)"
          + (f", selected {latest_data['selected']['label']}" if latest_data["selected"] else ""),
          file=sys.stderr)
    if inline_path:
        mb = inline_path.stat().st_size / 1_000_000
        print(f"  self-contained: {inline_path.name}  ({mb:.1f} MB)"
              + ("  — OVER the 16 MB artifact limit; recompress the PNGs to JPEG"
                 if mb > 15 else ""), file=sys.stderr)
    print(json.dumps({
        "out_dir": str(out_dir),
        "index_html": str(out_dir / "index.html"),
        "inline_html": str(inline_path) if inline_path else None,
        "slug": slug,
        "rounds_shown": [n for n, _ in rounds_data],
        "latest_round": latest_round,
        "images": made,
        "selected": latest_data["selected"]["label"] if latest_data["selected"] else None,
    }, indent=2))


def _write_inline(out_dir: Path) -> Path:
    """index.inline.html — every assets/*.png swapped for a base64 data URI, so
    the page is a single file to publish. No recompression (stdlib only); if the
    run's PNGs are large the caller recompresses to JPEG before publishing."""
    import base64
    page = (out_dir / "index.html").read_text(encoding="utf-8")
    # the OG/Twitter image tags are useless once inlined (scrapers ignore data:
    # URIs) and would bloat the file with a base64 blob — drop them.
    page = re.sub(r'\n<meta (?:property="og:image"|name="twitter:image")[^>]*>', "", page)
    for png in sorted((out_dir / "assets").glob("*.png")):
        uri = "data:image/png;base64," + base64.b64encode(png.read_bytes()).decode("ascii")
        page = page.replace(f"assets/{png.name}", uri)
    dest = out_dir / "index.inline.html"
    dest.write_text(page, encoding="utf-8")
    return dest


if __name__ == "__main__":
    main()
