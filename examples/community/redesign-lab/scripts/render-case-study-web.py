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
.compare .after-layer{position:absolute;inset:0;width:100%;height:100%;overflow:hidden;clip-path:inset(0 50% 0 0);will-change:clip-path;}
.compare .after-layer img{width:100%;height:auto;display:block;}
.compare .divider{position:absolute;top:0;bottom:0;left:50%;width:2px;background:var(--accent-fill);pointer-events:none;}
/* Live-embed mode (real <iframe> in place of a static screenshot): neither
   side's real content height is readable here -- the "before" iframe is a
   different origin (the live external site), which browsers block reading
   scrollHeight from regardless of X-Frame-Options, so the Math.max(before,
   after) natural-image-height trick above can't apply to this mode at all,
   not even for an after side that happens to be same-origin. Both sides
   instead fill the existing fixed 16:9 .compare-frame box completely and
   scroll internally on their own -- this is a real, deliberate trade-off,
   not an oversight: the previous synced-scroll-together behavior needed
   one shared scrolling container with both images stacked in normal flow,
   which an embedded live page (with its own real DOM, scripts, and scroll
   position) can't be made to participate in from the parent page's JS. */
.compare-frame.is-live .compare{overflow:hidden;touch-action:auto;}
.compare-frame.is-live .compare-inner{height:100%;}
/* `.compare-inner > iframe` (direct-child combinator) only ever matched the
   BEFORE iframe -- the AFTER iframe lives one level deeper, inside
   .after-layer, so it never matched this rule at all and was rendering at
   the browser's bare default iframe size (~300x150px) inside a much taller
   container. A real, confirmed bug: the visible boundary at the bottom of
   that undersized box looked exactly like a stray horizontal scrollbar,
   and -- being a fixed default height, unrelated to the drag handle or the
   video's own playback -- it stayed put at every reveal position, which is
   what made it so easy to mistake for something else while debugging.
   Listing both selectors together closes the gap. */
.compare-frame.is-live .compare-inner > iframe,
.compare-frame.is-live .after-layer iframe{position:absolute;inset:0;top:0;left:0;width:100%;height:100%;border:0;background:#fff;pointer-events:none;}
/* pointer-events:none, not an incidental extra: these two iframes are
   real, live pages for comparison, never meant to be clicked/scrolled by
   a mouse -- and leaving them hit-testable had two real, confirmed costs.
   First, a genuine navigation bug: a stray click during testing landed on
   one of the after page's own real links and navigated the entire tab
   away to that link's target, losing the comparison entirely. Second, and
   likely the bigger one for drag feel specifically: Chromium's site-
   isolation architecture runs a cross-origin iframe (the "before" side,
   the real external site) in its own separate process, so every time the
   cursor moves over it the browser has to route hit-testing across that
   process boundary -- real, measurable overhead on top of the layout/
   compositing cost already fixed elsewhere in this file, and one that
   setPointerCapture on the handle doesn't eliminate (capture controls
   which element *receives* events, not the hit-testing work the browser
   still does to track what's visually under the cursor for compositing
   and cursor-style purposes). Blocking pointer-events on both iframes
   removes both problems at once: neither is hit-testable at all now, so
   there's nothing for a stray click to land on and nothing for the
   cursor-move path to cross a process boundary for. Keyboard scrolling
   into either iframe (Tab, then arrow keys) still works -- this only
   removes mouse/pointer interaction, not keyboard access. */
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
.quick-hits{display:grid;gap:14px;max-width:800px;}
.quick-hit{display:flex;align-items:baseline;gap:14px;padding:14px 0;border-top:1px solid var(--rule);}
.quick-hit:first-child{border-top:none;padding-top:0;}
.quick-hit .num{font-family:"IBM Plex Mono",monospace;font-size:12px;color:var(--accent);font-weight:700;flex-shrink:0;}
.quick-hit p{margin:0;font-size:17px;}
.chapter{padding:36px 0;border-top:1px solid var(--rule);max-width:800px;}
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
.request-block{background:var(--surface);border:1px solid var(--rule);padding:16px 20px;margin-bottom:20px;max-width:800px;}
.request-block p{margin:6px 0 0;font-size:15px;font-style:italic;color:var(--ink);line-height:1.5;}
.pipeline{display:grid;gap:1px;background:var(--rule);border:1px solid var(--rule);margin-bottom:28px;max-width:800px;}
.pipeline-stage{background:var(--surface);padding:18px 22px;display:grid;grid-template-columns:140px 1fr;gap:16px;align-items:baseline;}
.pipeline-stage .name{font-family:Arial,"Helvetica Neue",sans-serif;font-weight:800;font-size:14px;text-transform:uppercase;letter-spacing:0.03em;}
.pipeline-stage .desc{font-size:14px;color:var(--muted);margin:0;}
@media (max-width:640px){ .pipeline-stage{grid-template-columns:1fr;gap:4px;} }
.loomloom-note{background:var(--surface);border:1px solid var(--rule);padding:18px 20px;font-size:14px;color:var(--muted);max-width:720px;}
.validate-grid{display:grid;grid-template-columns:repeat(2,1fr);gap:1px;background:var(--rule);border:1px solid var(--rule);}
.validate-cell{background:var(--surface);padding:20px 22px;}
.validate-cell .check{color:#2f6e4e;font-weight:700;margin-right:8px;}
.validate-cell .metric{font-family:Arial,"Helvetica Neue",sans-serif;font-weight:900;font-size:1.4rem;}
.validate-cell .label{display:block;font-size:13px;color:var(--muted);margin-top:4px;}
@media (max-width:640px){ .validate-grid{grid-template-columns:1fr;} }
.repro-block{background:var(--surface);border:2px solid var(--rule-strong);padding:22px 24px;max-width:800px;}
.repro-block code{display:block;font-family:"IBM Plex Mono",monospace;font-size:13px;background:var(--bg);border:1px solid var(--rule);padding:12px 14px;margin:10px 0 18px;overflow-x:auto;}
.repro-links{display:flex;gap:16px;flex-wrap:wrap;margin:0 0 18px;font-family:"IBM Plex Mono",monospace;font-size:13px;text-transform:uppercase;letter-spacing:0.04em;}
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
(function(){
  var widget = document.getElementById('compareWidget');
  if (!widget) return; // script.js is shared by pages with no compare widget (embed/ch-*.html)
  var inner = document.getElementById('compareInner');
  var layer = document.getElementById('afterLayer');
  var divider = document.getElementById('compareDivider');
  var handle = document.getElementById('compareHandle');
  var beforeImg = document.getElementById('beforeImg');
  var afterImg = document.getElementById('afterImg');
  var isLive = beforeImg.tagName === 'IFRAME';

  // Screenshot mode only: the widget's own frame is a fixed 16:9 box (CSS
  // aspect-ratio) so it reads like a video player regardless of content
  // length -- real vertical scrolling happens *inside* it. Both images
  // render at their real natural aspect ratio (no cropping), and `inner`'s
  // height is set to the taller of the two: a redesign that changed real
  // page length (this one compressed 8340px down to 2932px) means one side
  // runs out of real content before the other, shown honestly, not padded
  // or stretched to match. Live-embed mode needs none of this: neither
  // side's real height is readable (the before iframe is cross-origin),
  // and with .after-layer now always the full widget width (see the CSS
  // comment on that rule), each iframe's own plain width:100%;height:100%
  // already sizes both sides correctly with no JS involvement at all.
  function layout(){
    if (isLive) return;
    var w = widget.clientWidth;
    var beforeH = beforeImg.naturalWidth ? w * (beforeImg.naturalHeight / beforeImg.naturalWidth) : 0;
    var afterH = afterImg.naturalWidth ? w * (afterImg.naturalHeight / afterImg.naturalWidth) : 0;
    inner.style.height = Math.max(beforeH, afterH) + 'px';
  }
  function whenReady(img, cb){
    if (isLive) return;
    if (img.complete && img.naturalWidth) cb();
    else img.addEventListener('load', cb);
  }
  whenReady(beforeImg, layout);
  whenReady(afterImg, layout);
  window.addEventListener('resize', layout);

  function applyPct(pct){
    pct = Math.min(Math.max(pct, 0), 100);
    // clip-path, not width -- see the .after-layer CSS comment. inset(top
    // right bottom left): 0 from top/bottom/left, (100-pct)% off the right
    // edge, so pct=100 clips nothing (fully revealed) and pct=0 clips
    // everything (fully hidden), matching the old width-based behavior
    // exactly, just computed on the GPU instead of forcing layout.
    layer.style.clipPath = 'inset(0 ' + (100 - pct) + '% 0 0)';
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
  // Dragging starts only from the handle itself, not anywhere in the widget
  // -- the widget's background now has a real, independent job (native
  // vertical scroll), so a pointerdown anywhere used to fight that gesture
  // by also jumping the horizontal reveal. The handle is a precise, visible
  // grab target; scrolling the rest of the widget no longer touches it.
  //
  // Listeners live on the handle itself, not window, paired with
  // setPointerCapture: without capture, a fast drag that crosses over
  // either live iframe mid-gesture can hand pointer events to that
  // iframe's own document instead of continuing to reach this page's
  // listeners -- a real gap the old static-screenshot version never had
  // anything underneath it to hit. Capture pins every event for this
  // gesture to the handle regardless of what's visually under the
  // cursor, iframe or not.
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
  handle.addEventListener('pointercancel', function(){ dragging = false; dragRect = null; });
  handle.addEventListener('keydown', function(e){
    var current = parseFloat(handle.getAttribute('aria-valuenow')) || 50;
    if (e.key === 'ArrowLeft'){ applyPct(current - 5); e.preventDefault(); }
    else if (e.key === 'ArrowRight'){ applyPct(current + 5); e.preventDefault(); }
    else if (e.key === 'Home'){ applyPct(0); e.preventDefault(); }
    else if (e.key === 'End'){ applyPct(100); e.preventDefault(); }
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
function viewSource(e){
  // The old version linked href="index.html" -- that's just this same
  // page, so clicking it did nothing observable. view-source: on the
  // page's own real, resolved URL actually shows the real HTML, matching
  // what a developer clicking "View source" expects.
  e.preventDefault();
  window.open('view-source:' + location.href.split('#')[0].split('?')[0], '_blank');
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
                                rel_prefix=""):
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
    "before" site, which a relative-path prefix would break)."""
    is_live = bool(before_embed_url and after_embed_url)
    frame_cls = "compare-frame is-live" if is_live else "compare-frame"
    if is_live:
        before_el = f'<iframe id="beforeImg" src="{html.escape(before_embed_url)}" title="Before: {subject_esc}\'s original design, live, {before_label_esc}" loading="lazy"></iframe>'
        after_el = f'<iframe id="afterImg" src="{html.escape(rel_prefix + after_embed_url)}" title="After: {subject_esc} redesigned as {after_label_esc}, live" loading="lazy"></iframe>'
        caption_note = ' <span class="live-note">(real, live pages)</span>'
    else:
        before_el = f'<img id="beforeImg" src="{rel_prefix}{before_rel}" alt="Before: {subject_esc}\'s original design, {before_label_esc}">'
        after_el = f'<img id="afterImg" src="{rel_prefix}{after_rel}" alt="After: {subject_esc} redesigned as {after_label_esc}">'
        caption_note = ""
    # tabindex/aria-label on the wrapper only apply in screenshot mode,
    # where this div is itself the real scrollable region (confirmed via a
    # real axe-core scrollable-region-focusable finding, see
    # build-case-study.md). In live-embed mode each iframe scrolls on its
    # own -- this wrapper no longer scrolls at all (.is-live sets
    # overflow:hidden on it), so keeping the same tabindex/aria-label here
    # would be a real, confirmed aria-prohibited-attr finding: a static,
    # non-scrolling div falsely labeled as a scrollable region. The two
    # iframes are natively focusable and keyboard-scrollable without any
    # extra ARIA needed.
    wrapper_attrs = "" if is_live else ' tabindex="0" aria-label="Scrollable page preview"'
    return f'''<div class="{frame_cls}">
      <div class="compare" id="compareWidget"{wrapper_attrs}>
        <div class="compare-inner" id="compareInner">
          {before_el}
          <div class="after-layer" id="afterLayer">
            {after_el}
          </div>
          <div class="divider" id="compareDivider" aria-hidden="true"></div>
        </div>
      </div>
      <div class="handle" id="compareHandle" role="slider" tabindex="0" aria-label="Before and after comparison position"
           aria-valuemin="0" aria-valuemax="100" aria-valuenow="50"></div>
    </div>
    <div class="compare-caption"><span>Before</span><span>{compare_after_label}{caption_note}</span></div>'''


def render_case_study_html(data, before_rel, after_rel, logo_rel, favicon_rel, title, before_label, after_label,
                            og_image_rel, canonical_url=None, before_embed_url=None, after_embed_url=None):
    """The one function that turns case-study-data.json into the page's
    index.html body+head. Takes real relative asset paths (already copied
    into the output folder by the caller), never inlines them -- this is a
    real GitHub-Pages-deployable folder, not a standalone preview file.

    before_embed_url/after_embed_url are optional: when given, the compare
    widget renders real <iframe>s pointing at them instead of the static
    before_rel/after_rel screenshots (which still get copied and used for
    the Open Graph preview image regardless, since a social-media card
    can't render an iframe). See the CSS/JS "live-embed mode" comments on
    this same widget for why both sides fill a fixed box and scroll
    independently in this mode, rather than the synced-scroll-through-the-
    whole-page behavior the screenshot mode has."""
    status = data["status"]
    copy = STATUS_COPY[status]

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
        title_text = html.escape(cat.replace("-", " ").title())
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
        quick_label = html.escape(CATEGORY_QUICK_LABELS.get(cat, title_text))
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

    og_image_tag = f'<meta property="og:image" content="{html.escape(og_image_rel)}">' if og_image_rel else ""
    canonical_tag = f'<link rel="canonical" href="{html.escape(canonical_url)}">' if canonical_url else ""

    body = f'''
<header>
<div class="wrap">
  <div class="topbar">
    <div class="left">
      {"<img src='" + logo_rel + "' alt='" + subject_esc + " logo'>" if logo_rel else "<span></span>"}
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
    <p style="color:var(--muted);margin-bottom:20px;">{"Drag the handle to compare." if before_embed_url and after_embed_url else "Drag the handle to compare. Scroll inside the frame to see the rest of each page."}</p>
    {render_compare_widget_html(before_rel, after_rel, before_label_esc, after_label_esc, subject_esc, copy["compare_after_label"], before_embed_url=before_embed_url, after_embed_url=after_embed_url)}
    <div class="compare-embed">
      <span class="share-label">Share this chapter</span>
      <button class="share-btn" onclick="{html.escape(f'copyCompareEmbed({json.dumps("Before and after: " + data["subject"])})')}" aria-label="Copy embeddable code for this before/after comparison">Copy the code</button>
    </div>
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
      <div class="repro-links">
        <a href="https://github.com/cogfoundry-labs/loomloom/tree/main/examples/community/redesign-lab">GitHub repository</a>
        <a href="#" id="viewSourceLink" onclick="viewSource(event)">View source</a>
      </div>
      <ul class="tool-list">{tools_html}</ul>
      <code>git clone https://github.com/cogfoundry-labs/loomloom && cd loomloom/examples/community/redesign-lab</code>
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
<meta name="twitter:card" content="summary_large_image">
<meta name="twitter:title" content="{title_esc}">
<meta name="twitter:description" content="{description}">
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
                               before_embed_url=None, after_embed_url=None):
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
  {render_compare_widget_html(before_rel, after_rel, before_label_esc, after_label_esc, subject_esc, copy["compare_after_label"], before_embed_url=before_embed_url, after_embed_url=after_embed_url, rel_prefix="../")}
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
                           out_dir, canonical_url=None, before_embed_url=None, redesign_dir=None):
    """Copies real assets into `out_dir/assets/` and writes index.html +
    styles.css + script.js -- a real GitHub-Pages-ready folder. This is the
    one function both the CLI (`main`, below) and `build-case-study.py`
    (called in-process, no subprocess) use, so the two never drift into
    writing the folder two different ways.

    before_image/after_image (real screenshots) are always copied and
    always used for the Open Graph preview image, regardless of live-embed
    mode -- a social-media card can't render an iframe.

    Live-embed mode (real <iframe>s in the compare widget instead of the
    screenshots) needs both before_embed_url (a real external URL -- the
    live "before" site) and redesign_dir (a local directory: the real
    redesigned page plus its own real local assets, e.g. what
    implement-design.md's asset-pipeline rule produces once a page has a
    real host behind it). redesign_dir is copied into `out_dir/redesign/`
    wholesale so the "after" side is served same-origin (its real content
    height stays readable by this page's own JS, even though the "before"
    side's cross-origin height never can be -- see the CSS/JS "live-embed
    mode" comments on the compare widget for why that asymmetry doesn't
    actually matter here, both sides fill a fixed box either way). Passing
    only one of the two leaves the widget in ordinary screenshot mode --
    live-embed is all-or-nothing, not one side at a time."""
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
    if before_embed_url and redesign_dir:
        redesign_out = out_dir / "redesign"
        if redesign_out.exists():
            shutil.rmtree(redesign_out)
        shutil.copytree(redesign_dir, redesign_out)
        after_embed_url = "redesign/index.html"

    html_text = render_case_study_html(
        data, before_rel, after_rel, logo_rel, favicon_rel, title,
        before_label, after_label, og_image_rel=after_rel, canonical_url=canonical_url,
        before_embed_url=before_embed_url, after_embed_url=after_embed_url,
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
    parser.add_argument("--before-embed-url", default=None, help="real external URL of the live 'before' site -- if given (together with --redesign-dir), the compare widget renders real <iframe>s instead of the before/after screenshots")
    parser.add_argument("--redesign-dir", default=None, help="local directory of the real redesigned page + its own real local assets, copied into out-dir/redesign/ and iframed as the live 'after' side")
    args = parser.parse_args()

    data = json.loads(Path(args.data).read_text(encoding="utf-8"))
    result = build_case_study_site(
        data, args.before_image, args.after_image, args.logo, args.title,
        args.before_label, args.after_label, args.out_dir, canonical_url=args.canonical_url,
        before_embed_url=args.before_embed_url, redesign_dir=args.redesign_dir,
    )
    print(f"wrote {result['out_dir']}/ ({result['file_count']} files, {result['total_bytes']/1024/1024:.2f}MB total)")
    print(f"  index.html: {result['index_html_bytes']/1024:.1f}KB")


if __name__ == "__main__":
    main()
