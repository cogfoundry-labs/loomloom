#!/usr/bin/env python3
"""
render-case-study-web.py — the web renderer for a completed case-study-data.json
(produced by build-case-study.py's `plan` + `generate` phases).

This is deliberately the ONLY function that turns the data model into a
GitHub-Pages-ready folder. A future PDF renderer or social-card renderer
reads the same case-study-data.json and writes its own output format;
neither needs to touch this file or re-derive any fact, since every fact
here already came from a real diff, a real evidence gate, or the one real
loomloom call (narrative) this pipeline still makes.

Rev: no more image-generation. The hero used to be a generated cover
illustration -- real per the actual redesign tokens, but still a decorative
abstraction, not a real artifact, and it carried the ~7x precheck-undershoot
risk that's this pipeline's single biggest cost/reliability problem. It's
replaced with a typographic hero built from real data already on hand
(subject, before/after labels, root color tokens as real swatches) -- $0,
no model call, and it's more "real artifact" than the generated grid ever
was, per this project's own "real artifacts over decoration" principle.

Also rev: real separate files, not one base64-inlined blob. A GitHub Pages
repo is a real deploy target (implement-design.md's own distinction between
"a real project/server" and "a standalone preview with neither" applies
here, not the base64-inlining rule that governs a Direction Slice or a
preview build against a site this pipeline doesn't own). Confirmed the
inlining approach's real cost twice this session: a 2.48MB and a 3.45MB
single-file case study both exceeded this environment's Browser-pane
rendering ceiling and had to be sent as files instead of shown.

Usage:
    python render-case-study-web.py --data .output/<run>/case-study-data.json \\
        --before-image <path> --after-image <path> --logo <path> \\
        --title "<title>" --before-label "<short before label>" \\
        --after-label "<short after label>" --out-dir .output/<run>/case-study/site
"""

import argparse
import html
import json
import re
import shutil
from pathlib import Path
from urllib.parse import urljoin

STATUS_COPY = {
    "implemented": {
        "compare_after_label": "After: live redesign",
        "hero_status": "built, validated, and shipped.",
        "badge": None,
    },
    "preview": {
        "compare_after_label": "After: validated redesign preview",
        "hero_status": "built and mechanically validated as a real redesign preview, not yet deployed to the live site.",
        "badge": "PREVIEW (not deployed)",
    },
}

# Fixed, small enumerated set (diff-transformations.py's own category list) —
# a static short label per category is a rename, not a fabrication, since
# the category itself is already a real, mechanically-detected finding.
CATEGORY_QUICK_LABELS = {
    "color-system": "A real, semantic color system",
    "headline-typography": "A more disciplined type scale",
    "content-hierarchy": "Clearer visual hierarchy",
    "structural-framing": "Sharper structural framing",
    "corner-treatment": "A deliberate corner language",
    "layout-density": "A more focused grid",
}

PIPELINE_STAGES = [
    ("Discover", "Understand the existing site: real structure, real assets, real constraints, no model call."),
    ("Analyze", "Measure the real design system and audit it against a declared design authority's own rules."),
    ("Explore", "Build real, working direction slices and structural variants: real code, never a mockup."),
    ("Select", "A human picks the direction and variant that actually fits, from real rendered pages."),
    ("Implement", "Turn the chosen direction into the real page: real copy, real assets, real responsive layout."),
    ("Validate", "Real mechanical checks and a real axe-core accessibility scan; any failure blocks completion."),
]

MIME_FOR = {".jpg": "image/jpeg", ".jpeg": "image/jpeg", ".png": "image/png", ".webp": "image/webp", ".svg": "image/svg+xml"}


def mime_for(path):
    return MIME_FOR.get(Path(path).suffix.lower(), "image/png")


CSS = '''
:root{
  /* --accent is the loomloom brand green (#00fdbc) used AS TEXT directly on
     --bg -- tuned per theme for AA contrast, the same way --red used to be
     (light gets a darkened #007A5C shade: pure #00fdbc as text on the near-
     white light background is ~1.2:1, nowhere near readable; dark keeps the
     literal brand color, since it's ~11:1 against the near-black dark bg).
     --accent-fill/--on-accent-fill are a SEPARATE pair for solid swatches
     (badge, compare-slider handle): always the literal, undimmed brand
     green with dark ink on top, constant across both themes -- confirmed
     white text on the pure green is ~1.3:1 (fails badly), so those spots
     never use plain white regardless of which theme is active. */
  --bg:#F4F4F0; --surface:#FFFFFF; --ink:#0B0B0B; --muted:#4A4A46;
  --faint:#68675E; --accent:#007A5C; --accent-fill:#00FDBC; --on-accent-fill:#062017;
  --rule:#D8D6CE; --rule-strong:#0B0B0B;
  --shadow: 0 1px 3px rgba(11,11,11,0.08);
}
@media (prefers-color-scheme: dark){
  :root:not([data-theme="light"]){
    --bg:#15150F; --surface:#1D1D16; --ink:#F2F1EA; --muted:#B3B0A5;
    --faint:#726F63; --accent:#00FDBC; --accent-fill:#00FDBC; --on-accent-fill:#062017;
    --rule:#33322A; --rule-strong:#F2F1EA;
    --shadow: 0 1px 3px rgba(0,0,0,0.4);
  }
}
:root[data-theme="dark"]{
  --bg:#15150F; --surface:#1D1D16; --ink:#F2F1EA; --muted:#B3B0A5;
  --faint:#726F63; --accent:#00FDBC; --accent-fill:#00FDBC; --on-accent-fill:#062017;
  --rule:#33322A; --rule-strong:#F2F1EA;
  --shadow: 0 1px 3px rgba(0,0,0,0.4);
}
*{box-sizing:border-box;}
html{-webkit-text-size-adjust:100%;}
body{margin:0;background:var(--bg);color:var(--ink);font-family:"Source Serif 4",Georgia,serif;font-size:18px;line-height:1.6;overflow-x:hidden;}
.wrap{max-width:1280px;margin:0 auto;padding:0 24px;}
/* 1280px (revised down from an earlier 1920px try): 1920 across the whole
   page read as too extreme once actually seen rendered -- narrow ~800px
   prose blocks sitting inside a 1920px page left huge, unbalanced empty
   margins next to them. 1280 keeps the same "whole page scales together,
   not just the compare widget" idea (a human specifically wanted the
   Before/After section to keep its aspect-ratio:16/9 box at a size usable
   for real screen recordings, and the rest of the page to match rather
   than sit narrow beside one wide section), while landing at a page/prose
   ratio (1280 vs the ~800px reading-width blocks below) that actually
   reads as a deliberate, balanced layout instead of a wide page with an
   accidentally-narrow column floating in it. Flowing prose
   (.chapter-narrative, .loomloom-note, .request-block, and everything
   under the shared 800px cap below) still gets its own narrower max-width
   for real readability; non-text elements (headings, the compare widget,
   stat/fact grids, swatches) use the full column. */
a{color:var(--accent);}
.topbar{display:flex;align-items:center;justify-content:space-between;padding:20px 0;border-bottom:2px solid var(--rule-strong);gap:16px;}
.topbar .left{display:flex;align-items:center;gap:12px;min-width:0;}
.topbar img{height:18px;flex-shrink:0;}
:root:not([data-theme="light"]) .topbar img{filter:invert(1);}
:root[data-theme="dark"] .topbar img{filter:invert(1);}
.topbar .tag{font-family:"IBM Plex Mono",monospace;font-size:11px;letter-spacing:0.1em;text-transform:uppercase;color:var(--faint);white-space:nowrap;overflow:hidden;text-overflow:ellipsis;}
.copy-page-btn{font-family:"IBM Plex Mono",monospace;font-size:11px;text-transform:uppercase;letter-spacing:0.05em;background:none;border:1px solid var(--rule-strong);color:var(--ink);padding:7px 14px;cursor:pointer;flex-shrink:0;}
.copy-page-btn:hover{background:var(--ink);color:var(--bg);}
.hero{padding:72px 0 56px;border-bottom:2px solid var(--rule-strong);text-align:center;}
.hero-eyebrow{font-family:"IBM Plex Mono",monospace;font-size:12px;letter-spacing:0.18em;text-transform:uppercase;color:var(--accent);margin-bottom:22px;}
.badge{display:inline-block;font-family:"IBM Plex Mono",monospace;font-size:11px;letter-spacing:0.08em;text-transform:uppercase;background:var(--accent-fill);color:var(--on-accent-fill);padding:5px 12px;margin-bottom:18px;}
.hero .subject{font-family:Arial,"Helvetica Neue",sans-serif;font-weight:900;font-size:clamp(2rem,6vw,3.2rem);letter-spacing:-0.02em;line-height:1.02;text-transform:uppercase;margin:0 0 32px;text-wrap:balance;}
.hero-shift{display:flex;flex-direction:column;align-items:center;gap:6px;margin-bottom:28px;}
.hero-shift .label{font-family:"IBM Plex Mono",monospace;font-size:14px;letter-spacing:0.02em;color:var(--muted);padding:8px 18px;border:1px solid var(--rule);}
.hero-shift .label.after{color:var(--ink);border-color:var(--rule-strong);font-weight:700;}
.hero-shift .arrow{font-size:20px;color:var(--accent);line-height:1;}
.hero .tagline{font-family:"IBM Plex Mono",monospace;font-size:12px;letter-spacing:0.1em;text-transform:uppercase;color:var(--faint);margin:0 0 24px;}
.swatches{display:flex;justify-content:center;gap:8px;flex-wrap:wrap;}
.swatch{width:28px;height:28px;border:1px solid var(--rule-strong);}
section{padding:64px 0;border-bottom:2px solid var(--rule);}
section:last-of-type{border-bottom:none;}
.section-label{font-family:"IBM Plex Mono",monospace;font-size:11px;letter-spacing:0.12em;text-transform:uppercase;color:var(--accent);display:block;margin-bottom:10px;font-weight:700;}
h2{font-family:Arial,"Helvetica Neue",sans-serif;font-weight:900;font-size:1.9rem;letter-spacing:-0.01em;text-transform:uppercase;margin:0 0 28px;}
.compare-frame{position:relative;width:100%;aspect-ratio:16/9;border:2px solid var(--rule-strong);overflow:hidden;user-select:none;}
.compare{position:absolute;inset:0;overflow-y:auto;overflow-x:hidden;-webkit-overflow-scrolling:touch;touch-action:pan-y;}
.compare-inner{position:relative;width:100%;}
.compare img{width:100%;height:auto;display:block;}
/* Reveal is done with clip-path, not width. width is a layout-triggering
   CSS property -- animating it forces the browser to recompute layout for
   this box on every single drag frame, not just repaint it, and this box
   contains a whole live iframe (one of them with an actively-decoding
   <video> inside). Confirmed the real cause of visible drag stutter on a
   real deployed page: clip-path is compositor-only (GPU, no layout/
   reflow), the standard technique for exactly this before/after slider
   pattern. The layer itself is now always the full widget width/height --
   both images and both iframes can just be plain width:100% (matching
   each other automatically), no more JS-computed pixel width needed for
   either mode; see layout() below, much shorter now. */
.compare .after-layer{position:absolute;inset:0;width:100%;height:100%;overflow:hidden;clip-path:inset(0 0 0 50%);will-change:clip-path;}
.compare .after-layer img{width:100%;height:auto;display:block;}
.compare .divider{position:absolute;top:0;bottom:0;left:50%;width:2px;background:var(--accent-fill);pointer-events:none;}
/* Enhanced mode (a real <iframe> or a real <video> in place of a static
   screenshot on at least one side -- see render_compare_widget_html's
   before_type/after_type, decided independently per side): none of these
   content types expose a real content height here the way a plain <img>
   does -- an iframe's "before" side is typically a different origin (the
   live external site), which browsers block reading scrollHeight from
   regardless of X-Frame-Options, and a <video> has no comparable "how
   tall would this be as a page" concept at all, so the Math.max(before,
   after) natural-image-height trick above can't apply to this mode,
   even for an after side that happens to be same-origin. Every side
   instead fills the existing fixed 16:9 .compare-frame box completely and
   plays/scrolls internally on its own -- this is a real, deliberate
   trade-off, not an oversight: the previous synced-scroll-together
   behavior needed one shared scrolling container with both images
   stacked in normal flow, which an embedded live page (with its own real
   DOM, scripts, and scroll position) can't be made to participate in
   from the parent page's JS, and a <video> was never a scrolling
   participant to begin with. */
.compare-frame.is-enhanced .compare{overflow:hidden;touch-action:auto;}
.compare-frame.is-enhanced .compare-inner{height:100%;}
/* `.compare-inner > iframe` (direct-child combinator) only ever matched the
   BEFORE iframe -- the AFTER iframe lives one level deeper, inside
   .after-layer, so it never matched this rule at all and was rendering at
   the browser's bare default iframe size (~300x150px) inside a much taller
   container. A real, confirmed bug: the visible boundary at the bottom of
   that undersized box looked exactly like a stray horizontal scrollbar,
   and -- being a fixed default height, unrelated to the drag handle or the
   video's own playback -- it stayed put at every reveal position, which is
   what made it so easy to mistake for something else while debugging.
   Listing both selectors together closes the gap.
   pointer-events:none is permanent here now, not toggled -- this widget's
   two live iframes are frozen to the top of each real page on purpose (no
   scrolling, no clicking into either real site): every attempt at making
   them genuinely scrollable ran into the same real, confirmed wall --
   wheel-scroll over the cross-origin "before" iframe gets silently
   swallowed by the parent page's own scroll instead of scrolling the
   iframe, reproducible any time a cross-origin iframe sits inside a
   position:absolute ancestor on a scrollable page (isolated and confirmed
   with controlled tests, independent of clip-path/siblings/nesting depth
   -- a genuine Chromium cross-origin-iframe limitation, not a CSS mistake
   here). A custom-built scrollbar for the same-origin "after" side alone
   fixed only half the problem and added its own real interaction bugs.
   Freezing both to a static top-of-page comparison sidesteps all of it --
   the full, real, scrollable comparison lives in the second (screenshot-
   based, no video, no cross-origin routing problem) widget below, behind
   the "see the full page" toggle; see render_compare_widget_html and the
   .compare-full rule further down. */
.compare-frame.is-enhanced .compare-inner > iframe,
.compare-frame.is-enhanced .compare-inner > video,
.compare-frame.is-enhanced .after-layer iframe{position:absolute;inset:0;top:0;left:0;width:100%;height:100%;border:0;background:#fff;pointer-events:none;overflow:hidden;}
/* object-fit/object-position are no-ops on iframe (not a replaced element
   with an intrinsic ratio the way <video> is) but required for a real
   <video> before-side: without them the video distorts to the box's own
   aspect ratio instead of cropping like every other real screenshot/frame
   in this widget, and without object-position:top specifically the crop
   would center vertically instead -- a real source taller/wider than this
   fixed 16:9 box, cropped to its vertical middle, would contradict this
   widget's own "top of page only" promise (the caption text right below
   it, and the plain-screenshot mode's own top-anchored real content). */
.compare-frame.is-enhanced .compare-inner > video{object-fit:cover;object-position:top;}
/* width:100% is required here, not optional -- confirmed the hard way on
   the real deployed site: an <iframe> is a *replaced element* (same CSS
   category as <img>/<video>/<object>), and for an absolutely positioned
   replaced element, inset:0 alone does NOT stretch it to fill its
   container the way it would a plain <div> -- width:auto instead falls
   back to the browser's intrinsic default iframe size (300px). Without
   this rule the BEFORE iframe (which has no JS-set inline width to
   override that default, unlike the after iframe below) was stuck at
   exactly 300px, positioned at the widget's left edge -- entirely
   underneath whatever the after-layer's own reveal width was covering,
   so the actual visible "before" region showed nothing at all. The after
   iframe's JS-set inline style.width (below) still wins over this
   width:100% for that element specifically, since an inline style always
   beats a stylesheet rule regardless of specificity -- this line is safe
   for both, not just the one it happens to fix. */
/* No .after-layer width/height override needed for live mode anymore --
   .after-layer is unconditionally position:absolute;inset:0;width:100%;
   height:100% now (the base rule above), same in both screenshot and
   live-embed mode, with clip-path doing the reveal instead of a narrower
   box. That also means the after iframe's width:100% (the shared rule
   above) is now its real effective width, not just a fallback needing a
   JS override to matter -- previously JS had to force it to the full
   widget width specifically because .after-layer itself was narrower than
   that (whatever % was revealed); now .after-layer is always full width,
   so the iframe's own plain width:100% already matches the before
   iframe's, with nothing left for JS to override. */
.compare-frame .handle{position:absolute;top:50%;left:50%;transform:translate(-50%,-50%);width:44px;height:44px;background:var(--accent-fill);display:flex;align-items:center;justify-content:center;cursor:ew-resize;box-shadow:var(--shadow);z-index:2;touch-action:none;}
.compare-frame .handle::before{content:"\\2194";color:var(--on-accent-fill);font-size:18px;font-weight:900;}
.compare-frame .handle:focus-visible{outline:3px solid var(--ink);outline-offset:3px;}
.compare:focus-visible{outline:3px solid var(--ink);outline-offset:-3px;}
.compare-caption{display:flex;justify-content:space-between;font-family:"IBM Plex Mono",monospace;font-size:11px;text-transform:uppercase;letter-spacing:0.08em;color:var(--faint);margin-top:10px;}
.compare-full-toggle{margin:20px 0 0;text-align:center;}
/* The full-page comparison: real, static screenshots (no video, no
   cross-origin iframe), sized to the same 800px column as the rest of the
   article's prose instead of the 1280px full-bleed width the frozen live
   widget above uses -- it's a secondary, opt-in view, not the page's main
   visual anchor. Hidden by default ([hidden], a real attribute not just a
   class, so it's inert to layout/AT until toggled) and revealed by the
   button's click handler in script.js. */
.compare-full{max-width:800px;margin:20px auto 0;}
.compare-full[hidden]{display:none;}
.quick-hits{display:grid;gap:14px;max-width:800px;margin-left:auto;margin-right:auto;}
.quick-hit{display:flex;align-items:baseline;gap:14px;padding:14px 0;border-top:1px solid var(--rule);}
.quick-hit:first-child{border-top:none;padding-top:0;}
.quick-hit .num{font-family:"IBM Plex Mono",monospace;font-size:12px;color:var(--accent);font-weight:700;flex-shrink:0;}
.quick-hit p{margin:0;font-size:17px;}
.chapter{padding:36px 0;border-top:1px solid var(--rule);max-width:800px;margin-left:auto;margin-right:auto;}
.chapter:first-of-type{border-top:none;padding-top:0;}
.chapter-head{display:flex;align-items:baseline;gap:14px;margin-bottom:16px;}
.chapter-num{font-family:"IBM Plex Mono",monospace;font-size:12px;color:var(--accent);font-weight:700;}
.chapter h3{font-family:Arial,"Helvetica Neue",sans-serif;font-weight:800;font-size:1.25rem;margin:0;letter-spacing:-0.005em;}
.chapter-facts{display:grid;grid-template-columns:1fr 1fr;gap:20px;margin-bottom:18px;}
.fact{background:var(--surface);border:1px solid var(--rule);padding:14px 16px;}
.fact-label{display:block;font-family:"IBM Plex Mono",monospace;font-size:10px;letter-spacing:0.1em;text-transform:uppercase;color:var(--faint);margin-bottom:8px;}
.fact p{margin:0;font-size:13px;color:var(--muted);line-height:1.55;font-family:"IBM Plex Mono",monospace;word-break:break-word;}
.chapter-narrative{margin:0;font-size:17px;color:var(--ink);}
.chapter-share{display:flex;align-items:center;gap:10px;margin-top:16px;}
.share-label{font-family:"IBM Plex Mono",monospace;font-size:10px;letter-spacing:0.08em;text-transform:uppercase;color:var(--faint);}
.share-btn{font-family:"IBM Plex Mono",monospace;font-size:11px;text-transform:uppercase;letter-spacing:0.05em;background:none;border:1px solid var(--rule-strong);color:var(--ink);padding:6px 12px;cursor:pointer;}
.share-btn:hover{background:var(--ink);color:var(--bg);}
@media (max-width:640px){ .chapter-facts{grid-template-columns:1fr;} }
.request-block{background:var(--surface);border:1px solid var(--rule);padding:16px 20px;margin:0 auto 20px;max-width:800px;}
.request-block p{margin:6px 0 0;font-size:15px;font-style:italic;color:var(--ink);line-height:1.5;}
.pipeline{display:grid;gap:1px;background:var(--rule);border:1px solid var(--rule);margin:0 auto 28px;max-width:800px;}
.pipeline-stage{background:var(--surface);padding:18px 22px;display:grid;grid-template-columns:140px 1fr;gap:16px;align-items:baseline;}
.pipeline-stage .name{font-family:Arial,"Helvetica Neue",sans-serif;font-weight:800;font-size:14px;text-transform:uppercase;letter-spacing:0.03em;}
.pipeline-stage .desc{font-size:14px;color:var(--muted);margin:0;}
@media (max-width:640px){ .pipeline-stage{grid-template-columns:1fr;gap:4px;} }
.loomloom-note{background:var(--surface);border:1px solid var(--rule);padding:18px 20px;font-size:14px;color:var(--muted);max-width:720px;margin-left:auto;margin-right:auto;}
.validate-grid{display:grid;grid-template-columns:repeat(2,1fr);gap:1px;background:var(--rule);border:1px solid var(--rule);}
.validate-cell{background:var(--surface);padding:20px 22px;}
.validate-cell .check{color:#2f6e4e;font-weight:700;margin-right:8px;}
.validate-cell .metric{font-family:Arial,"Helvetica Neue",sans-serif;font-weight:900;font-size:1.4rem;}
.validate-cell .label{display:block;font-size:13px;color:var(--muted);margin-top:4px;}
@media (max-width:640px){ .validate-grid{grid-template-columns:1fr;} }
.repro-block{background:var(--surface);border:2px solid var(--rule-strong);padding:22px 24px;max-width:800px;margin-left:auto;margin-right:auto;}
.tool-list{list-style:none;margin:0;padding:0;max-width:800px;}
.tool-list li{padding:12px 0;border-bottom:1px solid var(--rule);}
.tool-list li:last-child{border-bottom:none;}
.tool-list .name{font-family:"IBM Plex Mono",monospace;font-size:13px;font-weight:700;}
.tool-list .role{display:block;font-size:14px;color:var(--muted);margin-top:4px;line-height:1.5;}
.compare-embed{display:flex;align-items:center;gap:10px;margin-top:14px;}
.live-note{font-weight:400;text-transform:none;letter-spacing:0;}
.note{font-size:13px;color:var(--faint);margin-top:18px;}
.final-cta{text-align:center;padding:56px 0;}
.final-cta h2{margin-bottom:24px;}
.final-cta .btn{display:inline-block;font-family:"IBM Plex Mono",monospace;font-size:13px;text-transform:uppercase;letter-spacing:0.06em;background:var(--ink);color:var(--bg);padding:14px 28px;text-decoration:none;font-weight:700;}
footer{padding:32px 0 48px;font-family:"IBM Plex Mono",monospace;font-size:11px;color:var(--faint);text-align:center;}
@media (prefers-reduced-motion: reduce){ *{transition:none !important;} }
'''

JS = '''
// Initializes every .compare-frame on the page independently (the case
// study now renders this widget twice -- the frozen live top-of-page one,
// and the full-page screenshot one behind the "see the full page" toggle
// -- see the render_compare_widget_html comment on why none of its inner
// pieces carry ids any more). Each instance's pieces are found relative to
// its own .compare-frame, not via a single fixed set of page-wide ids.
Array.prototype.forEach.call(document.querySelectorAll('.compare-frame'), function(frameEl){
 try { // Two real instances of this widget can be on one page now (the
  // frozen live one, the full-page screenshot one) -- forEach doesn't
  // catch per-iteration errors on its own, so one instance rendering
  // incompletely (a future template edit, a data gap) would otherwise
  // throw here and silently abort init for every instance after it in
  // document order. This try/catch keeps each instance's init isolated.
  var widget = frameEl.querySelector('.compare');
  if (!widget) return; // script.js is shared by pages with no compare widget (embed/ch-*.html)
  var inner = frameEl.querySelector('.compare-inner');
  var layer = frameEl.querySelector('.after-layer');
  var divider = frameEl.querySelector('.divider');
  var handle = frameEl.querySelector('.handle');
  // .before-media/.after-media, not a tag-name search -- this diff added
  // those exact classes to both elements for this lookup; a plain
  // `querySelector('iframe, img')` would silently grab the wrong node if
  // a future edit ever adds another image (a spinner, a placeholder)
  // inside .after-layer ahead of the real one.
  var beforeImg = frameEl.querySelector('.before-media');
  var afterImg = frameEl.querySelector('.after-media');
  // The class Python already stamps on frameEl itself, not a re-derived
  // tag-name check -- frame_cls in render_compare_widget_html sets
  // "is-enhanced" for exactly this (a real iframe or real video on at
  // least one side, decided independently per side -- see that function's
  // before_type/after_type), and the CSS already keys off it too; this
  // was an independent (and easy to drift out of sync) re-check of the
  // same fact.
  var isEnhanced = frameEl.classList.contains('is-enhanced');

  // Plain screenshot mode only: the widget's own frame is a fixed 16:9 box
  // (CSS aspect-ratio) so it reads like a video player regardless of
  // content length -- real vertical scrolling happens *inside* it. Both images
  // render at their real natural aspect ratio (no cropping), and `inner`'s
  // height is set to the taller of the two: a redesign that changed real
  // page length (this one compressed 8340px down to 2932px) means one side
  // runs out of real content before the other, shown honestly, not padded
  // or stretched to match. Live-embed mode needs none of this: both
  // iframes are permanently non-interactive and frozen to the top of each
  // real page (see the CSS comment on that rule) -- there's no scroll
  // height to measure or match here at all.
  function layout(){
    if (isEnhanced) return;
    var w = widget.clientWidth;
    var beforeH = beforeImg.naturalWidth ? w * (beforeImg.naturalHeight / beforeImg.naturalWidth) : 0;
    var afterH = afterImg.naturalWidth ? w * (afterImg.naturalHeight / afterImg.naturalWidth) : 0;
    inner.style.height = Math.max(beforeH, afterH) + 'px';
  }
  function whenReady(img, cb){
    if (isEnhanced) return;
    if (img.complete && img.naturalWidth) cb();
    else img.addEventListener('load', cb);
  }
  whenReady(beforeImg, layout);
  whenReady(afterImg, layout);
  window.addEventListener('resize', layout);
  // Exposed so the "see the full page" toggle below can force a re-layout
  // for just the one instance it's revealing, instead of broadcasting a
  // page-wide synthetic resize event that every .compare-frame's own
  // resize listener would also receive.
  frameEl._compareLayout = layout;

  function applyPct(pct){
    pct = Math.min(Math.max(pct, 0), 100);
    // clip-path, not width -- see the .after-layer CSS comment. inset(top
    // right bottom left): 0 from top/right/bottom, pct% off the LEFT edge,
    // so the after-layer (the redesign) is only ever revealed to the right
    // of the handle, matching the page's own "BEFORE -> AFTER" labels
    // (before on the left, after on the right).
    layer.style.clipPath = 'inset(0 0 0 ' + pct + '%)';
    divider.style.left = pct + '%';
    handle.style.left = pct + '%';
    handle.setAttribute('aria-valuenow', Math.round(pct));
  }
  // Drag smoothness, confirmed the hard way on a real deployed live-embed
  // page (two composited iframes, one with an actively-decoding <video> --
  // real per-frame compositing cost neither the old static-screenshot
  // widget nor a simple drag demo has to pay):
  // 1. widget.getBoundingClientRect() forces a synchronous layout
  //    recalculation -- calling it on every single raw pointermove event
  //    (which can fire well over 60 times/sec on a fast mouse) was real,
  //    avoidable work on top of the compositing cost above. The widget's
  //    own box doesn't move mid-drag, so it's captured once at
  //    pointerdown instead.
  // 2. Every pointermove wrote directly to style.width/style.left
  //    synchronously, with no batching -- on a page already busy
  //    compositing a live video, this can produce more style writes than
  //    the browser can actually paint, which reads as stutter, not
  //    smooth motion. Coalesced into one applyPct() per animation frame.
  var dragging = false;
  var dragRect = null;
  var pendingPct = null;
  var rafScheduled = false;
  function scheduleApply(){
    if (rafScheduled) return;
    rafScheduled = true;
    requestAnimationFrame(function(){
      rafScheduled = false;
      if (pendingPct !== null){ applyPct(pendingPct); pendingPct = null; }
    });
  }
  function setPos(clientX){
    var x = Math.min(Math.max(clientX - dragRect.left, 0), dragRect.width);
    pendingPct = (x / dragRect.width) * 100;
    scheduleApply();
  }
  // Dragging starts only from the handle itself, not anywhere in the
  // widget -- listeners live on the handle, paired with setPointerCapture
  // so a fast drag stays glued to the handle regardless of what's under
  // the cursor mid-gesture.
  handle.addEventListener('pointerdown', function(e){
    dragging = true;
    dragRect = widget.getBoundingClientRect();
    handle.setPointerCapture(e.pointerId);
    e.preventDefault();
  });
  handle.addEventListener('pointermove', function(e){ if (dragging) setPos(e.clientX); });
  handle.addEventListener('pointerup', function(e){
    dragging = false;
    dragRect = null;
    handle.releasePointerCapture(e.pointerId);
  });
  handle.addEventListener('pointercancel', function(){
    dragging = false;
    dragRect = null;
  });
  handle.addEventListener('keydown', function(e){
    // Not `parseFloat(...) || 50` -- a real, confirmed bug: when the
    // handle sits at 0 (after Home, or a drag to the far left),
    // parseFloat returns the number 0, and `0 || 50` evaluates to 50
    // because 0 is falsy in JS. That turned a real ArrowRight nudge (0 ->
    // 5) into a jarring jump (0 -> 55). isNaN is the actual "is this
    // missing" check; 0 is a perfectly valid position, not a missing one.
    var raw = parseFloat(handle.getAttribute('aria-valuenow'));
    var current = isNaN(raw) ? 50 : raw;
    if (e.key === 'ArrowLeft'){ applyPct(current - 5); e.preventDefault(); }
    else if (e.key === 'ArrowRight'){ applyPct(current + 5); e.preventDefault(); }
    else if (e.key === 'Home'){ applyPct(0); e.preventDefault(); }
    else if (e.key === 'End'){ applyPct(100); e.preventDefault(); }
  });
 } catch (err) { /* isolated per-instance -- see the try comment above */ }
});

// The "see the full page" toggle: the second .compare-frame (screenshot
// mode, hidden by default) only exists so the top-of-page enhanced widget
// above doesn't need scrolling at all -- see the CSS comment on
// .compare-frame.is-enhanced iframe/video for why that widget is frozen.
// Plain show/hide, no data to fetch: both screenshots are already real
// files in this build, same as the frozen widget's own poster-frame images.
(function(){
  var btn = document.getElementById('compareFullToggleBtn');
  var panel = document.getElementById('compareFull');
  if (!btn || !panel) return;
  btn.addEventListener('click', function(){
    panel.hidden = !panel.hidden;
    var isOpen = !panel.hidden;
    btn.setAttribute('aria-expanded', String(isOpen));
    btn.textContent = isOpen ? btn.getAttribute('data-hide-label') : btn.getAttribute('data-show-label');
    if (isOpen){
      // A real, confirmed bug: this panel starts `hidden` (display:none),
      // so its .compare-inner's height -- computed once from each image's
      // natural size times the *then-zero* widget.clientWidth (see the
      // layout() comment above) -- got permanently set to 0px before this
      // click ever happened. With the after-layer's height:100% resolving
      // against that real (if zero) explicit height, it collapsed to
      // nothing and clipped away 100% of its own content -- the before
      // image, a normal in-flow block, wasn't affected and rendered fine,
      // which is exactly why only "before" was ever visible no matter
      // where the handle was dragged. layout() never re-runs on its own
      // once the panel becomes visible -- only a resize event or a fresh
      // image load re-triggers it -- so it has to be forced here, now that
      // widget.clientWidth is finally real. Calls the revealed instance's
      // own layout() directly (stashed as frameEl._compareLayout by the
      // init loop above) instead of broadcasting a page-wide synthetic
      // resize event, so this doesn't also re-run layout for the other,
      // already-correctly-sized widget still visible above.
      var revealedFrame = panel.querySelector('.compare-frame');
      if (revealedFrame){
        // The two images in this instance render with data-src, not src
        // (see render_compare_widget_html's defer_load), so nothing has
        // fetched yet -- this is the one, deterministic trigger. Each
        // gets its own fresh 'load' listener (the one whenReady attached
        // at page-init time already fired trivially back then, since a
        // src-less <img> counts as "complete" with nothing to report) so
        // layout() gets a real recompute once each image actually
        // finishes loading, not just once at open time before either has
        // any pixels to measure.
        var deferred = revealedFrame.querySelectorAll('img[data-src]');
        Array.prototype.forEach.call(deferred, function(img){
          img.addEventListener('load', function(){
            if (revealedFrame._compareLayout) revealedFrame._compareLayout();
          });
          img.src = img.getAttribute('data-src');
          img.removeAttribute('data-src');
        });
        if (revealedFrame._compareLayout) revealedFrame._compareLayout();
      }
      panel.scrollIntoView({behavior: 'smooth', block: 'nearest'});
    }
  });
})();
function getBaseDir(){
  // The directory the deployed page actually lives in, resolved at click
  // time -- correct whether this is being previewed on a local server or
  // already live on GitHub Pages, without needing a build-time URL baked
  // in. A `<link rel="canonical">` tag (set if --canonical-url was given at
  // build time) wins when present, since it's a real, deliberately-declared
  // deploy target; otherwise falls back to wherever the page actually is
  // right now.
  var canonical = document.querySelector('link[rel="canonical"]');
  var url = (canonical ? canonical.href : location.href).split('#')[0].split('?')[0];
  return url.substring(0, url.lastIndexOf('/') + 1);
}
function copyPageLink(){
  copyText(getBaseDir() + 'index.html', event.target);
}
function escAttr(s){
  // The generated <iframe> is copy-pasted as raw HTML text onto someone
  // else's page -- `title` is real content (a subject name, a category
  // label) that can legitimately contain a literal quote character, which
  // would otherwise terminate the title="..." attribute early and produce
  // broken markup for the person pasting it.
  return String(s).replace(/&/g, '&amp;').replace(/"/g, '&quot;').replace(/</g, '&lt;').replace(/>/g, '&gt;');
}
function copyCompareEmbed(title){
  // The widget itself is now a fixed 16:9 frame (not content-length-driven),
  // so the embed fragment's real height is predictable from its rendered
  // width: measured for real at a representative 700px-wide embed, the
  // frame plus its surrounding chrome (wrap padding, the "via" attribution
  // line) renders at ~486px -- 520 leaves real margin without being
  // needlessly oversized the way a flat 700px default was before this rev.
  var code = '<iframe src="' + getBaseDir() + 'embed/compare.html" width="100%" height="520" '
    + 'style="border:1px solid #ddd;max-width:900px;" loading="lazy" title="' + escAttr(title) + '"></iframe>';
  copyText(code, event.target);
}
function copyChapterEmbed(num, title){
  // A chapter's real height depends on its own before/after fact text
  // length (often the longer driver than the narrative prose) and varies
  // per chapter -- 480 comfortably fit every chapter in the run this was
  // measured against (worst case ~421px), but a future run with longer
  // real facts could still need more; there's no generic height that's
  // exactly right for arbitrary future content.
  var code = '<iframe src="' + getBaseDir() + 'embed/ch-' + num + '.html" width="100%" height="480" '
    + 'style="border:1px solid #ddd;max-width:700px;" loading="lazy" title="' + escAttr(title) + '"></iframe>';
  copyText(code, event.target);
}
function copyText(text, btn){
  if (navigator.clipboard) { navigator.clipboard.writeText(text); }
  var original = btn.textContent;
  btn.textContent = 'Copied';
  setTimeout(function(){ btn.textContent = original; }, 1500);
}
'''


def looks_like_color(value):
    return bool(re.match(r"^#[0-9a-fA-F]{3,8}$", value.strip()) or re.match(r"^(rgb|hsl)a?\(", value.strip()))


def render_compare_widget_html(before_rel, after_rel, before_label_esc, after_label_esc, subject_esc,
                                compare_after_label, before_embed_url=None, after_embed_url=None,
                                before_video_url=None, rel_prefix="", has_full_page_toggle=False,
                                handle_label_suffix="", defer_load=False):
    """The compare-frame markup shared by the main page and the standalone
    embed fragment (render_embed_compare_html) -- one place that decides
    <img> (real screenshots) vs <iframe> (real live pages) so the two
    callers can't drift into rendering this widget two different ways.

    rel_prefix is "" for the main page, "../" for the embed fragment (one
    directory deeper). Applied to before_rel/after_rel (the screenshot
    paths) same as before. For the embed URLs, applied only to
    after_embed_url (a same-origin relative path into this same deployed
    site -- needs the same "one directory deeper" adjustment) and never to
    before_embed_url (always a real absolute external URL, the live
    "before" site, which a relative-path prefix would break).

    has_full_page_toggle: only True for the main case-study page's top
    (frozen, live) widget, which has a real "see the full page" toggle +
    second widget right below it -- render_embed_compare_html never passes
    this, since the standalone embed fragment has no such counterpart.
    Controls whether the live-mode caption promises "see the full page
    below": confirmed a real bug when this was unconditional -- the embed
    fragment rendered the exact same caption pointing at content that was
    never on that page.

    handle_label_suffix: distinguishes this instance's slider from another
    one that might be visible on the same page at once (only the main page
    can render this function twice -- the embed fragment always renders it
    once). Without this, two identical role="slider" aria-labels exist in
    the accessibility tree together once "see the full page" is opened,
    with no way for a screen-reader user to tell them apart."""
    # Each side's content type is decided independently -- not a single
    # binary "live or not" flag. A real, confirmed case this decoupling
    # exists for: tabbyml.com's own CSP blocks framing the whole page (no
    # `before_embed_url` possible), but its real hero video (a plain media
    # resource, not subject to that restriction -- confirmed directly, a
    # video load isn't "framing" anything) is still a real, genuinely
    # animated stand-in for the "before" side, while the "after" side (our
    # own same-origin redesign) can still be a real live iframe regardless
    # of what happened on the before side. The old all-or-nothing
    # `is_live = bool(before_embed_url and after_embed_url)` couldn't
    # represent this mixed case at all -- it either required both sides to
    # be page iframes or fell all the way back to plain screenshots on
    # both, losing the wide "frozen, top-of-page" widget entirely the
    # moment framing was blocked on just one side.
    before_type = "video" if before_video_url else ("iframe" if before_embed_url else "image")
    after_type = "iframe" if after_embed_url else "image"
    is_enhanced = before_type != "image" or after_type != "image"
    frame_cls = "compare-frame is-enhanced" if is_enhanced else "compare-frame"
    # defer_load (data-src, not loading="lazy"/autoplay): only the toggle-
    # revealed full-page widget passes this -- it starts inside a [hidden]
    # panel most visitors never open, so its assets (the same class of
    # full-page screenshot that covers an 8340px page in this file's own
    # comments elsewhere) shouldn't fetch until someone actually opens it.
    # loading="lazy" looked like the obvious fix and was tried first, but
    # confirmed unreliable for this exact shape of problem: an element
    # inside an ancestor that starts display:none and later gets shown via
    # JS is a known, documented edge case browsers' native lazy-loading
    # doesn't handle consistently (there's no spec-guaranteed re-check when
    # a hidden ancestor becomes visible -- it's built for scrolling down a
    # long page, not a JS-toggled reveal). data-src is deterministic
    # instead: no src attribute at all until the toggle's own click handler
    # in script.js copies data-src into src, which is a real, unambiguous
    # fetch trigger regardless of browser/engine lazy-load heuristics. The
    # always-visible primary widget (defer_load False, the only case that
    # ever reaches this function with an enhanced side) keeps loading
    # immediately -- deferring it would only delay real, immediately-needed
    # content for no benefit. Never actually exercised for video/iframe
    # sides in practice (the toggle-revealed widget is always plain
    # screenshot mode, see has_full_page_toggle's caller), kept here so an
    # image side stays correct regardless of which widget instance called
    # this function.
    src_attr = "data-src" if defer_load else "src"

    if before_type == "video":
        # autoplay+muted+loop+playsinline: the same real, minimum set
        # browsers require for autoplay to actually start without a user
        # gesture (confirmed: autoplay alone, without muted, is silently
        # blocked by every major browser's media-engagement policy). No
        # `loading="lazy"` here -- that attribute only applies to
        # <img>/<iframe>, not <video>; not needed anyway since this branch
        # is never used for the deferred (defer_load) widget instance.
        before_el = f'<video class="before-media" src="{html.escape(before_video_url)}" title="Before: {subject_esc}\'s original design, real captured video, {before_label_esc}" autoplay muted loop playsinline tabindex="-1"></video>'
    elif before_type == "iframe":
        # scrolling="no": iframes in enhanced mode are permanently
        # non-interactive (see the CSS comment on that rule) and frozen to
        # the top of each real page, so a visible native scrollbar is pure
        # leftover chrome from before that freeze -- confirmed real:
        # pointer-events:none stops the scrollbar from being *usable*, but
        # doesn't stop the browser from *drawing* it, since the iframe's
        # own content is still genuinely scrollable underneath. This
        # attribute (still respected by Chromium despite being long
        # deprecated in the HTML spec) suppresses the scrollbar itself,
        # backed by the matching overflow:hidden on the CSS rule below for
        # engines that ignore the attribute. Known, accepted cost: unlike
        # plain overflow:hidden, scrolling="no" also blocks the iframe's
        # keyboard-driven scroll (Tab in, then arrow keys) -- there's no
        # CSS-only way to suppress a cross-origin iframe's native scrollbar
        # from the embedding page (we don't control the real external
        # site's own stylesheet), so this is the only mechanism available
        # for the "before" side specifically. tabindex="-1" below matches
        # that reality instead of leaving a focusable control that can no
        # longer do anything when focused.
        before_el = f'<iframe class="before-media" src="{html.escape(before_embed_url)}" title="Before: {subject_esc}\'s original design, live, {before_label_esc}" loading="lazy" tabindex="-1" scrolling="no"></iframe>'
    else:
        before_el = f'<img class="before-media" {src_attr}="{rel_prefix}{before_rel}" alt="Before: {subject_esc}\'s original design, {before_label_esc}">'

    if after_type == "iframe":
        after_el = f'<iframe class="after-media" src="{html.escape(rel_prefix + after_embed_url)}" title="After: {subject_esc} redesigned as {after_label_esc}, live" loading="lazy" tabindex="-1" scrolling="no"></iframe>'
    else:
        after_el = f'<img class="after-media" {src_attr}="{rel_prefix}{after_rel}" alt="After: {subject_esc} redesigned as {after_label_esc}">'

    # "top of page only" without claiming every enhanced side is "live" --
    # a real, confirmed-accurate wording regardless of which mix of
    # video/iframe/image is actually in play (the old wording, "live
    # pages," was only ever true when both sides were literally live page
    # iframes). The "see the full page below" pointer is only accurate when
    # a caller actually renders that toggle+widget -- confirmed a real bug
    # when this was unconditional: the standalone embed fragment
    # (render_embed_compare_html) shares this same enhanced-mode branch but
    # never gets a full-page counterpart, so it rendered an identical
    # caption pointing at content that didn't exist on that page.
    if is_enhanced:
        caption_note = (
            f' <span class="live-note">(top of page only'
            f'{" -- see the full page below for the rest" if has_full_page_toggle else ""})</span>'
        )
    else:
        caption_note = ""
    # tabindex/aria-label on the wrapper only apply in plain screenshot
    # mode, where this div is itself the real scrollable region (confirmed
    # via a real axe-core scrollable-region-focusable finding, see
    # build-case-study.md). In enhanced mode each video/iframe scrolls (or
    # plays) on its own -- this wrapper no longer scrolls at all
    # (.is-enhanced sets overflow:hidden on it), so keeping the same
    # tabindex/aria-label here would be a real, confirmed
    # aria-prohibited-attr finding: a static, non-scrolling div falsely
    # labeled as a scrollable region. Iframes are permanently
    # non-interactive in enhanced mode (see the CSS comment on that rule),
    # so they don't need to be in the tab order either -- tabindex="-1"
    # above, alongside dropping this wrapper's own tabindex/aria-label. A
    # video side gets the same tabindex="-1" treatment for the same
    # reason: this widget is frozen/non-interactive regardless of which
    # enhanced content type either side actually uses.
    wrapper_attrs = "" if is_enhanced else ' tabindex="0" aria-label="Scrollable page preview"'
    # No ids on any of the repeated inner pieces (widget/inner/layer/
    # divider/handle/before-media/after-media) -- this function can now
    # render onto the same page twice (the frozen live widget up top, the
    # full-page screenshot widget behind the "see the full page" toggle
    # further down), and duplicate ids would be invalid HTML plus silently
    # break every getElementById lookup the old single-instance JS used to
    # rely on. The shared init script below now selects each instance's
    # pieces relative to its own .compare-frame instead.
    return f'''<div class="{frame_cls}">
      <div class="compare"{wrapper_attrs}>
        <div class="compare-inner">
          {before_el}
          <div class="after-layer">
            {after_el}
          </div>
          <div class="divider" aria-hidden="true"></div>
        </div>
      </div>
      <div class="handle" role="slider" tabindex="0" aria-label="Before and after comparison position{handle_label_suffix}"
           aria-valuemin="0" aria-valuemax="100" aria-valuenow="50"></div>
    </div>
    <div class="compare-caption"><span>Before</span><span>{compare_after_label}{caption_note}</span></div>'''


def render_case_study_html(data, before_rel, after_rel, logo_rel, favicon_rel, title, before_label, after_label,
                            og_image_rel, canonical_url=None, before_embed_url=None, after_embed_url=None,
                            before_video_url=None):
    """The one function that turns case-study-data.json into the page's
    index.html body+head. Takes real relative asset paths (already copied
    into the output folder by the caller), never inlines them -- this is a
    real GitHub-Pages-deployable folder, not a standalone preview file.

    before_embed_url/after_embed_url/before_video_url are all optional, and
    before_video_url is a real alternative to before_embed_url, not an
    addition to it: when a whole-page live embed is given, the compare
    widget renders real <iframe>s; when the before side instead has a real
    captured video (the common case when the original site's own CSP
    blocks framing but its hero video is still a plain, embeddable media
    resource -- confirmed real on tabbyml.com), that side renders a real
    <video> instead. Either real thing beats the static before_rel/after_rel
    screenshots (which still get copied and used for the Open Graph preview
    image regardless, since a social-media card can't render an iframe or a
    playing video). See the CSS/JS "enhanced mode" comments on this same
    widget for why every side fills a fixed box and plays/scrolls
    independently in this mode, rather than the synced-scroll-through-the-
    whole-page behavior the plain screenshot mode has."""
    status = data["status"]
    copy = STATUS_COPY[status]
    # Computed once, reused for both the intro paragraph's conditional text
    # and the "see the full page" toggle block's own gate below -- these
    # two used to be checked separately with slightly different syntax
    # (bool(...) here vs. a bare truthy check there), a real risk of the
    # two silently drifting out of agreement if only one got updated.
    # Decoupled from whether *both* sides specifically got a live iframe
    # (the old has_live_embed = bool(before_embed_url and after_embed_url)):
    # a real captured before-video with a plain after-screenshot, or a live
    # after-iframe with a before-video, are both real, legitimately
    # "enhanced" combinations that deserve the same wide top-of-page widget
    # plus toggle-revealed full-page widget -- see
    # render_compare_widget_html's own before_type/after_type for where the
    # actual per-side content-type decision lives.
    has_live_embed = bool(before_embed_url or after_embed_url or before_video_url)

    title_esc = html.escape(title)
    subject_esc = html.escape(data["subject"])
    before_label_esc = html.escape(before_label)
    after_label_esc = html.escape(after_label)
    description = html.escape(f"{data['subject']}: {before_label} to {after_label}. A real, evidence-based redesign case study.")

    chapters_html = ""
    quick_hits_html = ""
    for i, ch in enumerate(data["chapters"], start=1):
        num = f"{i:02d}"
        cat = ch["category"]
        title_raw = cat.replace("-", " ").title()
        title_text = html.escape(title_raw)
        before_fact = html.escape(ch["before_fact"])
        after_fact = html.escape(ch["after_fact"])
        narrative = html.escape(ch.get("narrative", "").strip() or f"{ch['before_fact']} {ch['after_fact']}")
        chapters_html += f'''
    <article class="chapter" id="ch-{num}">
      <div class="chapter-head">
        <span class="chapter-num">/// {num}</span>
        <h3>{title_text}</h3>
      </div>
      <div class="chapter-facts">
        <div class="fact"><span class="fact-label">Before</span><p>{before_fact}</p></div>
        <div class="fact"><span class="fact-label">After</span><p>{after_fact}</p></div>
      </div>
      <p class="chapter-narrative">{narrative}</p>
      <div class="chapter-share">
        <span class="share-label">Share this chapter</span>
        <button class="share-btn" onclick="{html.escape(f'copyChapterEmbed({json.dumps(num)}, {json.dumps(data["subject"] + ": " + cat.replace("-", " ").title())})')}" aria-label="Copy embeddable code for this chapter">Copy the code</button>
      </div>
    </article>'''
        # title_raw (unescaped), not title_text -- title_text is already
        # html.escape()'d for its own use above, so using it as the .get()
        # fallback here double-escaped it once html.escape() below ran
        # again. Currently inert (category slugs are hyphenated
        # identifiers unlikely to contain HTML metacharacters) but a real
        # logic defect: a category ever added to diff-transformations.py
        # without a matching CATEGORY_QUICK_LABELS entry would render
        # literal double-escaped entities instead of the real character.
        quick_label = html.escape(CATEGORY_QUICK_LABELS.get(cat, title_raw))
        quick_hits_html += f'''
      <div class="quick-hit"><span class="num">{num}</span><p>{quick_label}</p></div>'''

    pipeline_html = "".join(
        f'''
      <div class="pipeline-stage"><span class="name">{name}</span><p class="desc">{html.escape(desc)}</p></div>'''
        for name, desc in PIPELINE_STAGES
    )

    swatches_html = "".join(
        f'<span class="swatch" style="background:{html.escape(v)}" title="{html.escape(k.lstrip(chr(45)))}: {html.escape(v)}"></span>'
        for k, v in data["root_colors"].items() if looks_like_color(v)
    )

    validation = data.get("validation") or {}
    validate_cells = []
    if validation.get("mechanical_passed") is not None:
        validate_cells.append((
            f'{validation["mechanical_passed"]}/{validation["mechanical_total"]}',
            "Mechanical checks passed",
        ))
    if validation.get("a11y_violations") is not None:
        n = validation["a11y_violations"]
        validate_cells.append((str(n), "Accessibility violations" if n else "Accessibility violations (real axe-core scan)"))
    # These two claims aren't backed by their own report file the way the
    # two above are (there's no --responsive-report/--implementation-report
    # flag, nor should there be one just for this) -- a real, confirmed bug
    # when they were unconditional: a run built with neither
    # --mechanical-report nor --a11y-report still published two green
    # checkmarks claiming both were verified, contradicting this
    # codebase's own evidence-gating philosophy elsewhere (see
    # package-share.py's gather_evidence()). Gated on the same real
    # evidence the two cells above require: if at least one actual
    # validation report was provided, Validate genuinely ran against this
    # page, and both properties (real rendered code, checked responsive
    # per validate-design.md's webapp-testing piece) are true of anything
    # that made it through Validate at all -- but an empty validation dict
    # means Validate's evidence was never passed to this build at all, and
    # neither claim should be asserted.
    if validation:
        validate_cells.append(("✓", "Responsive layout (mobile / tablet / desktop)"))
        validate_cells.append(("✓", "Real rendered implementation, not a mockup"))
    validate_html = "".join(
        f'<div class="validate-cell"><span class="metric">{html.escape(v)}</span><span class="label">{html.escape(lbl)}</span></div>'
        for v, lbl in validate_cells
    )

    def tool_li(t):
        name = html.escape(t["name"])
        name_html = f'<a href="{html.escape(t["repo"])}" class="name">{name}</a>' if t.get("repo") else f'<span class="name">{name}</span>'
        return f'<li>{name_html}<span class="role">{html.escape(t["role"])}</span></li>'

    tools_html = "".join(tool_li(t) for t in data["evidence"]["tools_used"]) or "<li>(no evidence files found in this run)</li>"
    badge_html = f'<span class="badge">{html.escape(copy["badge"])}</span><br>' if copy["badge"] else ""
    # Real, optional: the actual request that started this run, human-facing
    # part only. Shown before the pipeline grid in "The Workflow" so a
    # reader sees what was actually asked for, not just what came out --
    # omitted entirely (not a placeholder) for a run that didn't capture one.
    request_block_html = (
        f'<div class="request-block"><span class="fact-label">The Request</span>'
        f'<p>&ldquo;{html.escape(data["request_prompt"])}&rdquo;</p></div>'
        if data.get("request_prompt") else ""
    )

    # A bare relative og:image path plus no og:url is a real, confirmed
    # problem: per the Open Graph protocol og:image should be an absolute
    # URL, and without og:url many crawlers (Facebook's included) have
    # nothing to resolve a relative path against -- publish without
    # --canonical-url, share the link, and the preview image is likely to
    # fail to render. When canonical_url is available, build a real
    # absolute image URL from it and emit a matching og:url; when it
    # isn't, at least keep twitter:image (previously missing entirely
    # despite declaring twitter:card=summary_large_image, which doesn't
    # reliably fall back to og:image on its own) pointing at the same
    # value og:image gets, rather than adding no fallback at all.
    og_image_abs = urljoin(canonical_url, og_image_rel) if (canonical_url and og_image_rel) else og_image_rel
    og_image_tag = f'<meta property="og:image" content="{html.escape(og_image_abs)}">' if og_image_abs else ""
    twitter_image_tag = f'<meta name="twitter:image" content="{html.escape(og_image_abs)}">' if og_image_abs else ""
    canonical_tag = f'<link rel="canonical" href="{html.escape(canonical_url)}">' if canonical_url else ""
    og_url_tag = f'<meta property="og:url" content="{html.escape(canonical_url)}">' if canonical_url else ""

    body = f'''
<header>
<div class="wrap">
  <div class="topbar">
    <div class="left">
      {f'<img src="{html.escape(logo_rel)}" alt="{subject_esc} logo">' if logo_rel else "<span></span>"}
      <span class="tag">Case Study &middot; Redesign Lab</span>
    </div>
    <button class="copy-page-btn" onclick="copyPageLink()" aria-label="Copy link to this case study">Copy link</button>
  </div>
</div>
</header>

<main>
<div class="hero">
  <div class="wrap">
    <span class="hero-eyebrow">Website Redesign Case Study</span>
    {badge_html}
    <h1 class="subject">{subject_esc}</h1>
    <div class="hero-shift">
      <span class="label before">{before_label_esc}</span>
      <span class="arrow">&darr;</span>
      <span class="label after">{after_label_esc}</span>
    </div>
    <p class="tagline">Real site &middot; Real assets &middot; Real code</p>
    <div class="swatches">{swatches_html}</div>
  </div>
</div>

<div class="wrap">

  <section id="compare">
    <span class="section-label">The Redesign</span>
    <h2>Before &rarr; After</h2>
    <p style="color:var(--muted);margin-bottom:20px;">{"Drag the handle to compare the top of each real page." if has_live_embed else "Drag the handle to compare. Scroll inside the frame to see the rest of each page."}</p>
    {render_compare_widget_html(before_rel, after_rel, before_label_esc, after_label_esc, subject_esc, copy["compare_after_label"], before_embed_url=before_embed_url, after_embed_url=after_embed_url, before_video_url=before_video_url, has_full_page_toggle=has_live_embed, handle_label_suffix=" (top of page)" if has_live_embed else "")}
    <div class="compare-embed">
      <span class="share-label">Share this chapter</span>
      <button class="share-btn" onclick="{html.escape(f'copyCompareEmbed({json.dumps("Before and after: " + data["subject"])})')}" aria-label="Copy embeddable code for this before/after comparison">Copy the code</button>
    </div>
    {f'''<div class="compare-full-toggle">
      <button class="share-btn" id="compareFullToggleBtn" type="button" aria-expanded="false" aria-controls="compareFull" data-show-label="See the full page, top to bottom" data-hide-label="Hide the full page">See the full page, top to bottom</button>
    </div>
    <div class="compare-full" id="compareFull" hidden>
      <p style="color:var(--muted);margin-bottom:20px;">A full-page comparison (static screenshots, not the live sites above) -- drag the handle, then scroll down inside it to see the rest of both pages together.</p>
      {render_compare_widget_html(before_rel, after_rel, before_label_esc, after_label_esc, subject_esc, copy["compare_after_label"], handle_label_suffix=" (full page)", defer_load=True)}
    </div>''' if has_live_embed else ""}
  </section>

  <section id="what-changed">
    <span class="section-label">What Changed</span>
    <h2>At A Glance</h2>
    <div class="quick-hits">{quick_hits_html}</div>
  </section>

  <section id="chapters">
    <span class="section-label">Design Transformations</span>
    <h2>Real, Measured Changes</h2>
    {chapters_html}
  </section>

  <section id="how">
    <span class="section-label">How loomloom Did It</span>
    <h2>The Workflow</h2>
    {request_block_html}
    <div class="pipeline">{pipeline_html}
    </div>
    <div class="loomloom-note">loomloom's role in this case study: writing the {len(data["chapters"])} narrative paragraphs above from real, verified facts, the only real model call in this run's Share stage. No image was generated: the hero above uses the real color tokens from the actual redesign directly, not an AI illustration of them.</div>
  </section>

  <section id="validate">
    <span class="section-label">Validation</span>
    <h2>It Works, Not Just Looks Right</h2>
    <div class="validate-grid">{validate_html}</div>
  </section>

  <section id="reproduce">
    <span class="section-label">Reproduce This</span>
    <h2>How This Was Actually Built</h2>
    <div class="repro-block">
      <p style="margin-top:0;color:var(--muted);font-size:14px;">This redesign is built on real open-source work. Thanks to every project below: each is listed because real evidence of its use exists in this run.</p>
      <ul class="tool-list">{tools_html}</ul>
      {f'<p class="note">{html.escape(data["diff_capture_note"])}</p>' if data.get("diff_capture_note") else ""}
    </div>
  </section>

  <div class="final-cta">
    <h2>Want To Redesign Another Site?</h2>
    <a class="btn" href="https://github.com/cogfoundry-labs/loomloom/tree/main/examples/community/redesign-lab">Try Redesign Lab</a>
  </div>

</div>
</main>

<footer>Made with Redesign Lab &middot; a loomloom-native design pipeline</footer>

<script src="script.js" defer></script>
'''

    head = f'''<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>{title_esc}</title>
<meta name="description" content="{description}">
<meta property="og:type" content="website">
<meta property="og:title" content="{title_esc}">
<meta property="og:description" content="{description}">
{og_image_tag}
{og_url_tag}
<meta name="twitter:card" content="summary_large_image">
<meta name="twitter:title" content="{title_esc}">
<meta name="twitter:description" content="{description}">
{twitter_image_tag}
{canonical_tag}
{'<link rel="icon" href="' + favicon_rel + '">' if favicon_rel else ""}
<link rel="stylesheet" href="styles.css">
</head>
<body>
{body}
</body>
</html>
'''
    return head


def render_embed_compare_html(data, before_rel, after_rel, before_label, after_label,
                               before_embed_url=None, after_embed_url=None, before_video_url=None):
    """A standalone fragment for the 'Copy the code' embed on the Before/
    After section -- meant to be dropped into an <iframe> on someone else's
    blog. Reuses the main page's real styles.css/script.js via a relative
    `../` path rather than duplicating the compare-widget CSS/JS a second
    time; the two are one directory apart by construction (`embed/*.html`
    sits alongside `index.html`)."""
    subject_esc = html.escape(data["subject"])
    copy = STATUS_COPY[data["status"]]
    before_label_esc = html.escape(before_label)
    after_label_esc = html.escape(after_label)
    return f'''<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<meta name="robots" content="noindex">
<title>{subject_esc}: Before / After</title>
<link rel="stylesheet" href="../styles.css">
<style>.embed-h1{{position:absolute;width:1px;height:1px;overflow:hidden;clip:rect(0,0,0,0);white-space:nowrap;}}</style>
</head>
<body>
<main>
<div class="wrap" style="padding:20px 16px;">
  <h1 class="embed-h1">Before and after: {subject_esc}</h1>
  {render_compare_widget_html(before_rel, after_rel, before_label_esc, after_label_esc, subject_esc, copy["compare_after_label"], before_embed_url=before_embed_url, after_embed_url=after_embed_url, before_video_url=before_video_url, rel_prefix="../")}
  <p style="font-family:&quot;IBM Plex Mono&quot;,monospace;font-size:11px;color:var(--faint);margin-top:14px;">From a <a href="../index.html" target="_top">Redesign Lab case study</a></p>
</div>
</main>
<script src="../script.js" defer></script>
</body>
</html>
'''


def render_embed_chapter_html(data, ch, num):
    """A standalone fragment for one chapter's 'Copy the code' embed --
    same reuse-the-real-stylesheet approach as render_embed_compare_html."""
    subject_esc = html.escape(data["subject"])
    title_text = html.escape(ch["category"].replace("-", " ").title())
    before_fact = html.escape(ch["before_fact"])
    after_fact = html.escape(ch["after_fact"])
    narrative = html.escape(ch.get("narrative", "").strip() or f"{ch['before_fact']} {ch['after_fact']}")
    return f'''<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<meta name="robots" content="noindex">
<title>{subject_esc}: {title_text}</title>
<link rel="stylesheet" href="../styles.css">
<style>.chapter h1{{font-family:Arial,"Helvetica Neue",sans-serif;font-weight:800;font-size:1.25rem;margin:0;letter-spacing:-0.005em;}}</style>
</head>
<body>
<main>
<div class="wrap" style="padding:20px 16px;">
  <article class="chapter" style="padding-top:0;border-top:none;">
    <div class="chapter-head">
      <span class="chapter-num">/// {num}</span>
      <h1>{title_text}</h1>
    </div>
    <div class="chapter-facts">
      <div class="fact"><span class="fact-label">Before</span><p>{before_fact}</p></div>
      <div class="fact"><span class="fact-label">After</span><p>{after_fact}</p></div>
    </div>
    <p class="chapter-narrative">{narrative}</p>
  </article>
  <p style="font-family:&quot;IBM Plex Mono&quot;,monospace;font-size:11px;color:var(--faint);margin-top:14px;">From a <a href="../index.html" target="_top">Redesign Lab case study</a>: {subject_esc}</p>
</div>
</main>
</body>
</html>
'''


def build_case_study_site(data, before_image, after_image, logo, title, before_label, after_label,
                           out_dir, canonical_url=None, before_embed_url=None, redesign_dir=None,
                           before_video_url=None):
    """Copies real assets into `out_dir/assets/` and writes index.html +
    styles.css + script.js -- a real GitHub-Pages-ready folder. This is the
    one function both the CLI (`main`, below) and `build-case-study.py`
    (called in-process, no subprocess) use, so the two never drift into
    writing the folder two different ways.

    before_image/after_image (real screenshots) are always copied and
    always used for the Open Graph preview image, regardless of enhanced-
    mode -- a social-media card can't render an iframe or a playing video.

    redesign_dir (a local directory: the real redesigned page plus its own
    real local assets, e.g. what implement-design.md's asset-pipeline rule
    produces once a page has a real host behind it), when given alone,
    already gets the "after" side a real live iframe -- it's always
    same-origin (our own copied files), never subject to a foreign site's
    CSP, so it doesn't need before_embed_url's cooperation the way it used
    to. redesign_dir is copied into `out_dir/redesign/` wholesale so that
    iframe is served same-origin.

    before_embed_url (a real external URL -- the live "before" site) is a
    separate, independent real enhancement for the "before" side
    specifically: given alone (no redesign_dir), only the before side goes
    live and after stays a screenshot; given together with redesign_dir,
    both sides go live. Neither requires the other any more -- see
    render_compare_widget_html's before_type/after_type for where each
    side's actual content-type decision lives.

    before_video_url is a real alternative to before_embed_url, not an
    addition to it, for exactly the case a whole-page live embed can't
    reach: a site whose own CSP blocks framing the page at all, but whose
    real hero video (a plain media resource, not subject to that
    restriction) is still a genuine, animated stand-in for the "before"
    side -- confirmed real on tabbyml.com. Passing all three of
    before_embed_url/redesign_dir/before_video_url as none leaves the
    widget in ordinary screenshot mode, same as before."""
    out_dir = Path(out_dir)
    assets_dir = out_dir / "assets"
    assets_dir.mkdir(parents=True, exist_ok=True)

    def copy_asset(src_path, name):
        if not src_path:
            return None
        src = Path(src_path)
        dest = assets_dir / f"{name}{src.suffix.lower()}"
        shutil.copy2(src, dest)
        return f"assets/{dest.name}"

    before_rel = copy_asset(before_image, "before")
    after_rel = copy_asset(after_image, "after")
    logo_rel = copy_asset(logo, "logo") if logo else None
    favicon_rel = copy_asset(logo, "favicon") if logo else None

    after_embed_url = None
    if redesign_dir:
        redesign_out = out_dir / "redesign"
        if redesign_out.exists():
            shutil.rmtree(redesign_out)
        shutil.copytree(redesign_dir, redesign_out)
        after_embed_url = "redesign/index.html"

    html_text = render_case_study_html(
        data, before_rel, after_rel, logo_rel, favicon_rel, title,
        before_label, after_label, og_image_rel=after_rel, canonical_url=canonical_url,
        before_embed_url=before_embed_url, after_embed_url=after_embed_url,
        before_video_url=before_video_url,
    )
    (out_dir / "styles.css").write_text(CSS, encoding="utf-8")
    (out_dir / "script.js").write_text(JS, encoding="utf-8")
    (out_dir / "index.html").write_text(html_text, encoding="utf-8")

    # Standalone embed fragments -- what each "Copy the code" button's
    # <iframe> actually points at. One per chapter plus the compare widget,
    # each a real, self-contained page (reusing ../styles.css/../script.js)
    # so it renders correctly on a third-party site regardless of that
    # site's own CSS.
    embed_dir = out_dir / "embed"
    embed_dir.mkdir(exist_ok=True)
    (embed_dir / "compare.html").write_text(
        render_embed_compare_html(
            data, before_rel, after_rel, before_label, after_label,
            before_embed_url=before_embed_url, after_embed_url=after_embed_url,
            before_video_url=before_video_url,
        ), encoding="utf-8"
    )
    for i, ch in enumerate(data["chapters"], start=1):
        num = f"{i:02d}"
        (embed_dir / f"ch-{num}.html").write_text(render_embed_chapter_html(data, ch, num), encoding="utf-8")

    total_bytes = sum(f.stat().st_size for f in out_dir.rglob("*") if f.is_file())
    return {
        "out_dir": str(out_dir),
        "file_count": len(list(out_dir.rglob("*"))),
        "total_bytes": total_bytes,
        "index_html_bytes": (out_dir / "index.html").stat().st_size,
    }


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--data", required=True)
    parser.add_argument("--before-image", required=True)
    parser.add_argument("--after-image", required=True)
    parser.add_argument("--logo", default=None)
    parser.add_argument("--title", required=True)
    parser.add_argument("--before-label", required=True, help="short, real characterization of the original design (e.g. 'Consumer-SaaS Card Grid')")
    parser.add_argument("--after-label", required=True, help="short, real characterization of the redesign (e.g. 'Enterprise Operations Console')")
    parser.add_argument("--canonical-url", default=None)
    parser.add_argument("--out-dir", required=True, help="GitHub-Pages-ready folder: index.html, styles.css, script.js, assets/")
    parser.add_argument("--before-embed-url", default=None, help="real external URL of the live 'before' site -- if given, the compare widget's before side renders a real <iframe> instead of a screenshot (independent of --redesign-dir now: each side's enhancement stands on its own)")
    parser.add_argument("--redesign-dir", default=None, help="local directory of the real redesigned page + its own real local assets, copied into out-dir/redesign/ and iframed as the live 'after' side -- works whether or not --before-embed-url is also given")
    parser.add_argument("--before-video-url", default=None, help="real URL of a captured video for the original site's hero (a plain media resource, not a page) -- a real alternative to --before-embed-url for a site whose own CSP blocks framing the page at all but not loading its video directly")
    args = parser.parse_args()

    data = json.loads(Path(args.data).read_text(encoding="utf-8"))
    result = build_case_study_site(
        data, args.before_image, args.after_image, args.logo, args.title,
        args.before_label, args.after_label, args.out_dir, canonical_url=args.canonical_url,
        before_embed_url=args.before_embed_url, redesign_dir=args.redesign_dir,
        before_video_url=args.before_video_url,
    )
    print(f"wrote {result['out_dir']}/ ({result['file_count']} files, {result['total_bytes']/1024/1024:.2f}MB total)")
    print(f"  index.html: {result['index_html_bytes']/1024:.1f}KB")


if __name__ == "__main__":
    main()
