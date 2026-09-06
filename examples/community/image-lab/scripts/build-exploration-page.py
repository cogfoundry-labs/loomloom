#!/usr/bin/env python3
"""build-exploration-page.py — turn one Image Lab run into a shareable page.

Every `image.py run` writes `<out>/run.json`. This script reads that record and
builds a self-contained static folder:

    <out>/<slug>/
        index.html            # the page (styles + a small gallery script inline)
        exploration.json      # the data model — a future PDF / social-card renderer reads THIS
        assets/alternative-01.png ...

**No gate. Nothing is spent** — the images already exist. This is a render step.

The page is an **image-selection gallery**, not a case-study report: a hero image
with a thumbnail strip, so the story is "here are the possibilities, pick one".
The design follows redesign-lab's case-study house style — serif body, heavy
uppercase headings, IBM Plex Mono labels, hard edges, the loomloom green accent.

Usage
    python scripts/build-exploration-page.py --from ./out \
        --title "Murree Meets Toronto" \
        --subject "Murree x Toronto" \
        --invocation "<the exact message the user sent to trigger Image Lab>" \
        [--summary "<override the auto tagline>"] \
        [--selected C] [--slug custom-slug] [--out ./out/<slug>]

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

ATTRIBUTION_TEXT = "Generated with CogFoundry's model router"
ATTRIBUTION_URL = "https://cogfoundry.ai"
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
             invocation: str | None, selected_label: str | None) -> dict:
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
            "asset": f"assets/alternative-{a['index']:02d}.png" if ok else None,
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

    return {
        "kicker": "IMAGE LAB",
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
# renderer
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


def _model_link(label: str, url: str | None) -> str:
    esc = html.escape(label)
    if url:
        return (f'<a class="model" href="{html.escape(url)}" '
                f'target="_blank" rel="noopener">{esc}</a>')
    return f'<span class="model">{esc}</span>'


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
    return (f'<button type="button" class="thumb{" active" if active else ""}"{cur} '
            f'aria-label="Show alternative {a["index"]}" {attrs}>'
            f'<img src="{a["asset"]}" alt="Alternative {a["index"]}" loading="lazy">'
            f'{badge}<span class="thumb-n">{a["index"]:02d}</span></button>')


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


def _gallery(data: dict) -> str:
    shown = [a for a in data["alternatives"] if a["asset"]]
    note = _failed_note(data)
    if not shown:
        return (f'  <section class="gallery">\n'
                f'    <p class="gallery-note">No images were generated in this run.</p>\n'
                f'    {note}\n  </section>')
    initial = next((i for i, a in enumerate(shown) if a["selected"]), 0)
    first = shown[initial]
    label = "Select an image" if len(shown) > 1 else "The image"
    strip = ""
    if len(shown) > 1:
        thumbs = "\n      ".join(_thumb(a, i == initial) for i, a in enumerate(shown))
        strip = (f'    <div class="gallery-thumbs" aria-label="alternatives">\n'
                 f'      {thumbs}\n    </div>\n')
    return (
        '  <section class="gallery">\n'
        f'    <span class="section-label">{label}</span>\n'
        '    <div class="gallery-main">\n'
        f'      <a id="hero-link" href="{first["asset"]}" target="_blank" rel="noopener">'
        f'<img id="hero-img" src="{first["asset"]}" alt="Alternative {first["index"]}"></a>\n'
        f'      <p class="gallery-meta" id="hero-meta">{_meta_html(first)}</p>\n'
        '    </div>\n'
        f'{strip}'
        f'    {note}\n'
        '  </section>'
    )


def _prompt_section(data: dict) -> str:
    core = html.escape(data["prompt_core"] or data["prompt"])
    trig = html.escape(data["prompt_trigger"])
    return (
        '  <section class="prompt">\n'
        '    <span class="section-label">Prompt</span>\n'
        f'    <blockquote class="prompt-core">{core}</blockquote>\n'
        f'    <p class="prompt-trigger">&rarr; {trig}</p>\n'
        '    <button class="copy" data-target="copy-src">Copy prompt</button>\n'
        '    <p class="prompt-how">Paste it into any agent with the Image Lab skill installed.</p>\n'
        f'    <span id="copy-src" hidden>{html.escape(data["prompt_copy"])}</span>\n'
        '  </section>'
    )


def _locate(source_file: str, from_dir: Path) -> Path | None:
    src = Path(source_file)
    for cand in ([src] if src.is_absolute() else [from_dir / src]) + [from_dir / src.name]:
        if cand.exists():
            return cand
    return None


def render(data: dict, from_dir: Path, out_dir: Path) -> None:
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

    attr = data["attribution"]
    page = PAGE.format(
        title=html.escape(data["title"]),
        kicker=html.escape(data["kicker"]),
        summary=html.escape(data["summary"]),
        stat_line=_stat_line(data["stats"]),
        gallery=_gallery(data),
        prompt_section=_prompt_section(data),
        attr_text=html.escape(attr["text"]),
        attr_url=html.escape(attr["url"]),
        generated_on=data["generated_on"],
    )
    (out_dir / "index.html").write_text(page, encoding="utf-8")
    slim = {k: v for k, v in data.items() if k not in ("alternatives", "summary_is_auto")}
    slim["alternatives"] = [{k: v for k, v in a.items() if k != "source_file"}
                            for a in data["alternatives"]]
    (out_dir / "exploration.json").write_text(json.dumps(slim, indent=2), encoding="utf-8")


# --------------------------------------------------------------------------- #
# A single-file page: styles + the gallery script inline, images as sibling
# files (a multi-MB base64 blob breaks browser rendering — redesign-lab's
# render-case-study-web.py learned this the hard way, twice). GitHub-Pages ready.
PAGE = """<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>{title} &mdash; Image Lab</title>
<link rel="icon" href="data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 32 32'%3E%3Crect width='32' height='32' fill='%23007a5c'/%3E%3C/svg%3E">
<link rel="preconnect" href="https://fonts.googleapis.com">
<link rel="preconnect" href="https://fonts.gstatic.com" crossorigin>
<link rel="stylesheet" href="https://fonts.googleapis.com/css2?family=IBM+Plex+Mono:wght@400;500;700&family=Source+Serif+4:ital,wght@0,400;0,600;1,400&display=swap">
<style>
  :root {{
    --bg:#f4f4f0; --surface:#ffffff; --ink:#0b0b0b; --muted:#4a4a46; --faint:#68675e;
    --accent:#007a5c; --accent-fill:#00fdbc; --on-accent-fill:#062017;
    --rule:#d8d6ce; --rule-strong:#0b0b0b;
    --serif:"Source Serif 4", Georgia, "Times New Roman", serif;
    --mono:"IBM Plex Mono", ui-monospace, "SFMono-Regular", Menlo, Consolas, monospace;
    --sans:Arial, "Helvetica Neue", Helvetica, sans-serif;
  }}
  @media (prefers-color-scheme: dark) {{
    :root:not([data-theme="light"]) {{
      --bg:#15150f; --surface:#1d1d16; --ink:#f2f1ea; --muted:#b3b0a5; --faint:#726f63;
      --accent:#00fdbc; --accent-fill:#00fdbc; --on-accent-fill:#062017;
      --rule:#33322a; --rule-strong:#f2f1ea;
    }}
  }}
  :root[data-theme="dark"] {{
    --bg:#15150f; --surface:#1d1d16; --ink:#f2f1ea; --muted:#b3b0a5; --faint:#726f63;
    --accent:#00fdbc; --accent-fill:#00fdbc; --on-accent-fill:#062017;
    --rule:#33322a; --rule-strong:#f2f1ea;
  }}
  * {{ box-sizing: border-box; }}
  html {{ -webkit-text-size-adjust: 100%; }}
  body {{
    margin: 0; background: var(--bg); color: var(--ink);
    font-family: var(--serif); font-size: 18px; line-height: 1.6; overflow-x: hidden;
  }}
  a {{ color: var(--accent); }}
  .wrap {{ max-width: 1080px; margin: 0 auto; padding: 0 24px; }}

  header {{ border-bottom: 2px solid var(--rule-strong); padding: 64px 0 44px; }}
  .kicker {{
    font-family: var(--mono); font-size: 12px; letter-spacing: .2em; text-transform: uppercase;
    color: var(--accent); font-weight: 700; margin: 0 0 20px;
  }}
  h1 {{
    font-family: var(--sans); font-weight: 900; text-transform: uppercase;
    font-size: clamp(2rem, 6vw, 3.1rem); line-height: 1.03; letter-spacing: -.02em;
    margin: 0 0 22px; text-wrap: balance;
  }}
  .summary {{ font-size: 1.22rem; line-height: 1.5; max-width: 34ch; margin: 0 0 22px; color: var(--ink); }}
  .stats {{
    font-family: var(--mono); font-size: 11px; letter-spacing: .06em; text-transform: uppercase;
    color: var(--faint); margin: 0;
  }}

  section {{ padding: 56px 0; border-bottom: 2px solid var(--rule); }}
  section:last-of-type {{ border-bottom: none; }}
  .section-label {{
    display: block; font-family: var(--mono); font-size: 11px; font-weight: 700;
    letter-spacing: .14em; text-transform: uppercase; color: var(--accent); margin: 0 0 20px;
  }}

  /* ---- gallery ---- */
  .gallery-main img {{
    width: 100%; height: auto; max-height: 78vh; object-fit: contain;
    display: block; border: 2px solid var(--rule-strong); background: var(--surface);
  }}
  .gallery-meta {{
    font-family: var(--mono); font-size: 12px; letter-spacing: .02em; text-transform: uppercase;
    color: var(--faint); margin: 14px 0 0;
  }}
  .gallery-meta .model {{ color: var(--ink); font-weight: 700; }}
  .gallery-meta a.model {{ text-decoration: none; border-bottom: 1px solid var(--accent); }}
  .gallery-meta .chose {{ color: var(--accent); font-weight: 700; margin-right: .5rem; }}
  .gallery-thumbs {{ display: flex; flex-wrap: wrap; gap: 12px; margin-top: 22px; }}
  .thumb {{
    position: relative; flex: 1 1 150px; max-width: 220px; aspect-ratio: 3 / 2;
    padding: 0; border: 1px solid var(--rule); background: var(--surface); cursor: pointer;
    transition: box-shadow .1s, border-color .1s;
  }}
  .thumb img {{ width: 100%; height: 100%; object-fit: cover; display: block; }}
  .thumb:hover {{ border-color: var(--muted); }}
  .thumb.active {{ border-color: var(--rule-strong); box-shadow: 0 0 0 2px var(--accent-fill); }}
  .thumb-n {{
    position: absolute; left: 0; bottom: 0; font-family: var(--mono); font-size: 10px; font-weight: 700;
    color: var(--on-accent-fill); background: var(--accent-fill); padding: 2px 6px; letter-spacing: .05em;
  }}
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

  /* ---- prompt ---- */
  .prompt-core {{
    margin: 0; background: var(--surface); border: 1px solid var(--rule);
    border-left: 3px solid var(--accent); padding: 20px 24px;
    font-family: var(--serif); font-style: italic; font-size: 1.05rem; line-height: 1.6;
    color: var(--ink); max-width: 760px; white-space: pre-wrap; word-break: break-word;
  }}
  .prompt-trigger {{
    font-family: var(--mono); font-size: 12px; text-transform: uppercase; letter-spacing: .06em;
    color: var(--faint); margin: 14px 0 0;
  }}
  .prompt-how {{ font-size: 14px; color: var(--muted); margin: 10px 0 0; }}
  button.copy {{
    margin-top: 18px; font-family: var(--mono); font-size: 11px; font-weight: 700;
    text-transform: uppercase; letter-spacing: .06em; cursor: pointer;
    background: none; border: 1px solid var(--rule-strong); color: var(--ink); padding: 9px 16px;
  }}
  button.copy:hover {{ background: var(--ink); color: var(--bg); }}
  button.copy:active {{ transform: translateY(1px); }}

  footer {{
    padding: 32px 0 48px; font-family: var(--mono); font-size: 11px; text-transform: uppercase;
    letter-spacing: .05em; color: var(--faint); display: flex; justify-content: space-between;
    flex-wrap: wrap; gap: .5rem;
  }}
  footer a {{ color: var(--faint); }}
  @media (prefers-reduced-motion: reduce) {{ * {{ transition: none !important; }} }}
</style>
</head>
<body>
<header>
  <div class="wrap">
    <p class="kicker">{kicker}</p>
    <h1>{title}</h1>
    <p class="summary">{summary}</p>
    <p class="stats">{stat_line}</p>
  </div>
</header>

<main class="wrap">
{gallery}

{prompt_section}

  <footer>
    <span>{attr_text} &middot; <a href="{attr_url}" target="_blank" rel="noopener">cogfoundry.ai</a></span>
    <span>{generated_on}</span>
  </footer>
</main>
<script>
  (function () {{
    var thumbs = Array.prototype.slice.call(document.querySelectorAll(".thumb"));
    var hero = document.getElementById("hero-img");
    var link = document.getElementById("hero-link");
    var meta = document.getElementById("hero-meta");
    if (!thumbs.length || !hero) return;
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
    function show(t) {{
      thumbs.forEach(function (x) {{
        var on = x === t;
        x.classList.toggle("active", on);
        if (on) x.setAttribute("aria-current", "true"); else x.removeAttribute("aria-current");
      }});
      var d = t.dataset;
      hero.src = d.asset;
      hero.alt = "Alternative " + d.index;
      if (link) link.href = d.asset;
      meta.innerHTML = rebuildMeta(d);
    }}
    thumbs.forEach(function (t) {{ t.addEventListener("click", function () {{ show(t); }}); }});
    document.addEventListener("keydown", function (e) {{
      if (e.key !== "ArrowRight" && e.key !== "ArrowLeft") return;
      var i = thumbs.findIndex(function (x) {{ return x.classList.contains("active"); }});
      if (e.key === "ArrowRight" && i < thumbs.length - 1) show(thumbs[i + 1]);
      if (e.key === "ArrowLeft" && i > 0) show(thumbs[i - 1]);
    }});
  }})();
  document.querySelectorAll("button.copy").forEach(function (b) {{
    b.addEventListener("click", function () {{
      var el = document.getElementById(b.dataset.target);
      navigator.clipboard.writeText(el ? el.textContent : "").then(function () {{
        var old = b.textContent; b.textContent = "Copied";
        setTimeout(function () {{ b.textContent = old; }}, 1400);
      }});
    }});
  }});
</script>
</body>
</html>
"""


def main() -> None:
    ap = argparse.ArgumentParser(prog="build-exploration-page.py")
    ap.add_argument("--from", dest="frm", required=True,
                    help="the run's --out directory (must contain run.json)")
    ap.add_argument("--title", required=True, help="3-5 word title for the exploration")
    ap.add_argument("--subject", required=True,
                    help='short noun phrase for the tagline, e.g. "Murree x Toronto"')
    ap.add_argument("--invocation", default=None,
                    help="the exact message the user sent to trigger Image Lab (shown verbatim)")
    ap.add_argument("--summary", default=None,
                    help="override the auto tagline (One brief. N models. M ways to see ...)")
    ap.add_argument("--selected", default=None, help="branch label of the creator's pick (A, B, ...)")
    ap.add_argument("--slug", default=None, help="override the URL slug (default: from --title)")
    ap.add_argument("--out", default=None, help="output folder (default: <from>/<slug>)")
    a = ap.parse_args()

    from_dir = Path(a.frm).resolve()
    run_path = from_dir / "run.json"
    if not run_path.exists():
        die(f"{run_path} not found — run `image.py run --out {a.frm} ...` first")
    try:
        run = json.loads(run_path.read_text(encoding="utf-8"))
    except (json.JSONDecodeError, OSError) as e:
        die(f"could not read {run_path}: {e}")

    data = assemble(run, a.title, a.subject, a.summary, a.invocation, a.selected)
    if a.slug:
        data["slug"] = slugify(a.slug)
    out_dir = Path(a.out).resolve() if a.out else from_dir / data["slug"]
    render(data, from_dir, out_dir)

    made = data["stats"]["generated"]
    print(f"exploration page: {out_dir / 'index.html'}", file=sys.stderr)
    print(f"  {made} image(s), {data['stats']['models']} model(s)"
          + (f", selected {data['selected']['label']}" if data["selected"] else ""),
          file=sys.stderr)
    print(json.dumps({
        "out_dir": str(out_dir),
        "index_html": str(out_dir / "index.html"),
        "slug": data["slug"],
        "images": made,
        "selected": data["selected"]["label"] if data["selected"] else None,
    }, indent=2))


if __name__ == "__main__":
    main()
