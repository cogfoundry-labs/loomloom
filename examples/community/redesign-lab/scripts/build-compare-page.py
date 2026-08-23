#!/usr/bin/env python3
"""
build-compare-page.py — local, free, no-model-call. Builds the one-page,
button-switched comparison page used at Gate 1 (Direction Slices) and
Gate 2 (Explore Variants), replacing the old "one browser tab per option"
mechanism (see ../references/approval-policy.md's "How gates are actually
shown" for why that mechanism doesn't hold up in this environment).

Confirmed directly, in a real run: the Browser pane only ever composites
one tab live. Switching tabs in the pane's own tab bar does not reliably
hand off compositing to the newly-selected tab, so every tab but the most
recently active one renders a stale frame -- which reads to a human as
"all the options look the same" or "this one didn't render," depending on
which stale frame happened to be showing. A single page with a button per
option sidesteps this by construction: there is only ever one tab, so
there is nothing for the pane to get wrong.

This revives the combined-comparison-page mechanism rev 7 of
approval-policy.md removed -- but the reason rev 7 removed it (base64
`<iframe srcdoc>` payloads exceeding this environment's rendering limit,
and cross-page links being inert inside a `data:`-rendered file) does not
apply here: every iframe below points at a real `src="/..."` URL served
by a real local HTTP server, not an inlined srcdoc. The payload is a URL
string, not the page's own bytes.

Usage:
    python build-compare-page.py --title "Gate 1: pypi.org" \\
        --option "current-fixed=/.output/directions/current-fixed/index.html=11/11" \\
        --option "high-end-visual-design=/.output/directions/high-end-visual-design/colorway-1/index.html=11/11" \\
        --out .output/compare.html

Each --option is "label=src=score" (score is free text, shown small next
to the label -- "11/11", "10/11 (nav height, see chat)", etc.). The
generated page must be opened through the same local HTTP server that
serves the option pages themselves (file:// breaks the iframe src
resolution and reintroduces the exact rendering problem this script
exists to avoid) -- see approval-policy.md for starting one.
"""

import argparse
import html
import json
import sys


PAGE_TEMPLATE = """<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>{title}</title>
<style>
  /* Tool-chrome palette: deliberately a color no real redesign is likely to
     use for its own background -- saturated amber, high-contrast black
     text -- so this bar reads as "comparison tool," never as part of the
     design being judged, regardless of whether the option shown is a dark
     or a light page. Confirmed necessary: a neutral dark chrome (#141416)
     was visually indistinguishable from a redesign that also used a
     near-black background, and a human reviewing it couldn't tell where
     the tool's own UI ended and the real page began. */
  :root{{--chrome:#F5B400;--chrome-ink:#1A1200;--chrome-line:#B98400;--dim:#6B5200;}}
  *{{box-sizing:border-box;}}
  html,body{{height:100%;}}
  body{{margin:0;background:#0A0A0A;color:var(--chrome-ink);display:flex;flex-direction:column;
       font-family:ui-sans-serif,system-ui,-apple-system,"Segoe UI",sans-serif;}}

  header{{background:var(--chrome);border-bottom:3px solid var(--chrome-line);
         padding:10px 16px;display:flex;align-items:center;gap:14px;flex-wrap:wrap;flex:0 0 auto;
         box-shadow:0 2px 10px rgba(0,0,0,.4);}}
  .badge{{font-size:10px;font-weight:800;letter-spacing:.08em;text-transform:uppercase;
         background:var(--chrome-ink);color:var(--chrome);padding:3px 8px;border-radius:3px;}}
  h1{{font-size:13px;font-weight:700;margin:0;letter-spacing:.01em;}}
  .tabs{{display:flex;gap:6px;margin-left:auto;flex-wrap:wrap;}}
  .tabs button{{font:inherit;font-size:13px;padding:8px 16px;background:#fff;
    color:var(--chrome-ink);border:1px solid var(--chrome-line);border-radius:6px;cursor:pointer;}}
  .tabs button[aria-selected="true"]{{background:var(--chrome-ink);border-color:var(--chrome-ink);
    color:var(--chrome);font-weight:700;}}
  .tabs button:hover:not([aria-selected="true"]){{border-color:var(--chrome-ink);}}
  .tabs button:focus-visible{{outline:2px solid var(--chrome-ink);outline-offset:2px;}}
  .tabs .score{{font-size:11px;opacity:.8;margin-left:6px;font-weight:400;}}

  main{{flex:1 1 auto;min-height:0;position:relative;}}
  iframe{{position:absolute;inset:0;width:100%;height:100%;border:0;display:none;background:#000;}}
  iframe.is-active{{display:block;}}
</style>
</head>
<body>
<header>
  <span class="badge">Comparison tool &mdash; not part of the design</span>
  <h1>{title}</h1>
  <div class="tabs" role="tablist" aria-label="Choose an option">
{tab_buttons}
  </div>
</header>

<main>
{iframes}
</main>

<script>
  var buttons = Array.prototype.slice.call(document.querySelectorAll('.tabs button'));
  var frames = Array.prototype.slice.call(document.querySelectorAll('iframe'));

  function select(key){{
    buttons.forEach(function(b){{ b.setAttribute('aria-selected', String(b.dataset.key === key)); }});
    frames.forEach(function(f){{ f.classList.toggle('is-active', f.dataset.key === key); }});
  }}

  buttons.forEach(function(b){{
    b.addEventListener('click', function(){{ select(b.dataset.key); }});
  }});
</script>
</body>
</html>
"""


def parse_option(raw):
    parts = raw.split("=", 2)
    if len(parts) != 3:
        sys.exit(f'--option must be "label=src=score", got: {raw!r}')
    label, src, score = parts
    return {"label": label, "src": src, "score": score}


def build(title, options):
    tab_buttons = []
    iframes = []
    for i, opt in enumerate(options):
        key = f"opt{i}"
        selected = "true" if i == 0 else "false"
        active_cls = " is-active" if i == 0 else ""
        tab_buttons.append(
            f'    <button type="button" role="tab" data-key="{key}" aria-selected="{selected}">'
            f'{html.escape(opt["label"])}<span class="score">{html.escape(opt["score"])}</span></button>'
        )
        iframes.append(
            f'  <iframe class="iframe{active_cls}" data-key="{key}" '
            f'title="{html.escape(opt["label"])}" src="{html.escape(opt["src"])}"></iframe>'
        )
    return PAGE_TEMPLATE.format(
        title=html.escape(title),
        tab_buttons="\n".join(tab_buttons),
        iframes="\n".join(iframes),
    )


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--title", required=True)
    parser.add_argument("--option", action="append", required=True, dest="options",
                         help='One per option: "label=src=score"')
    parser.add_argument("--out", required=True)
    args = parser.parse_args()

    options = [parse_option(o) for o in args.options]
    if len(options) < 2:
        sys.exit("need at least 2 --option flags to build a comparison page")

    page = build(args.title, options)
    with open(args.out, "w", encoding="utf-8") as f:
        f.write(page)
    print(json.dumps({"wrote": args.out, "options": [o["label"] for o in options]}))


if __name__ == "__main__":
    main()
