#!/usr/bin/env python3
"""
mechanical-check.py — local, free, no-model-call checks against a rendered page.

Every check here is computable from the DOM/CSS after render. None of them
need a vision-language model, and none of them need loomloom. This is what
makes the "explore free" path in Gate 1 a real, complete option rather than a
crippled demo of the paid path.

Usage:
    python mechanical-check.py <url-or-file-path> [--out report.json]

Exit code 0 = all checks passed. Exit code 1 = at least one Fail.
"""

import argparse
import io
import json
import re
import sys
from pathlib import Path

from PIL import Image
from playwright.sync_api import sync_playwright

EM_DASH_CHARS = ("—", "–")  # em dash, en dash

# CTA-intent buckets: literal strings that count as "the same ask" even with
# different wording. Kept small and explicit rather than fuzzy-matched.
CTA_INTENT_BUCKETS = {
    "contact": {"contact us", "get in touch", "let's talk", "reach out", "start a project", "talk to us"},
    "signup": {"try free", "get started", "sign up free", "sign up", "start free trial", "try it free"},
    "portfolio": {"view work", "see selected work", "browse projects", "our work", "view portfolio"},
}


def relative_luminance(rgb):
    def channel(c):
        c = c / 255
        return c / 12.92 if c <= 0.03928 else ((c + 0.055) / 1.055) ** 2.4

    r, g, b = rgb
    return 0.2126 * channel(r) + 0.7152 * channel(g) + 0.0722 * channel(b)


def contrast_ratio(rgb1, rgb2):
    l1, l2 = relative_luminance(rgb1), relative_luminance(rgb2)
    lighter, darker = max(l1, l2), min(l1, l2)
    return (lighter + 0.05) / (darker + 0.05)


def parse_rgb(css_color, treat_transparent_as_none=True):
    m = re.match(r"rgba?\((\d+),\s*(\d+),\s*(\d+)(?:,\s*([\d.]+))?\)", css_color or "")
    if not m:
        return None
    alpha = float(m.group(4)) if m.group(4) is not None else 1.0
    if treat_transparent_as_none and alpha < 0.05:
        return None  # effectively transparent — not a real background to compare against
    return tuple(int(m.group(i)) for i in (1, 2, 3))


def cta_bucket(text):
    t = text.strip().lower()
    for bucket, phrases in CTA_INTENT_BUCKETS.items():
        if t in phrases:
            return bucket
    return None


def sample_bg_color(page, el, box):
    """Read the actually-rendered background pixels instead of trusting
    getComputedStyle(el).backgroundColor: a gradient/image fill painted by a
    ::before overlay (a common CTA button pattern) is invisible to the CSS
    walk-up but is right there in the pixels.

    Two-phase, because neither phase alone is safe on real markup:

    Phase 1 — sample inside the element's own box, at points that measurably
    avoid the actual glyph rects (via the same TreeWalker/getClientRects
    technique the wrap-check uses), not a fixed top/bottom fraction. This is
    correct for the common case: a button with real internal padding, sized
    and positioned next to other buttons or busy content. Confirmed necessary
    on a real site during testing — a filled yellow CTA button sitting on a
    photographic hero, 10px from its neighbor: sampling outside the box (as
    phase 2 does) landed on the photo behind it or the adjacent button, both
    times misreporting solid-black-on-yellow as near-invisible.

    Phase 2 — only when phase 1 finds no safe interior point (glyphs fill
    the box with no real padding, so there's nowhere inside it to sample):
    expand outward into the surrounding margin instead. Confirmed necessary
    on a different real site: bold nav caps with near-zero padding, where
    every interior point is glyph ink — the true background only exists
    just outside the box.

    Per-channel median over a grid of points, not a handful of single corner
    pixels, in both phases: a decorative line-art/grid background can put an
    arbitrary accent color under any one exact pixel."""
    try:
        safe_points = el.evaluate(
            """el => {
                const rect = el.getBoundingClientRect();
                const walker = document.createTreeWalker(
                    el, NodeFilter.SHOW_TEXT,
                    { acceptNode: n => n.nodeValue.trim().length > 0
                        ? NodeFilter.FILTER_ACCEPT : NodeFilter.FILTER_SKIP }
                );
                const glyphRects = [];
                let node;
                while ((node = walker.nextNode())) {
                    const range = document.createRange();
                    range.selectNodeContents(node);
                    for (const r of range.getClientRects()) {
                        if (r.width > 0 && r.height > 0) glyphRects.push(r);
                    }
                }
                const buffer = 2;
                const insideGlyph = (x, y) => glyphRects.some(r =>
                    x >= r.left - buffer && x <= r.right + buffer &&
                    y >= r.top - buffer && y <= r.bottom + buffer);
                const cols = 5, rows = 3;
                const points = [];
                for (let ri = 0; ri < rows; ri++) {
                    const y = rect.top + (rect.height * (ri + 0.5)) / rows;
                    for (let ci = 0; ci < cols; ci++) {
                        const x = rect.left + (rect.width * (ci + 0.5)) / cols;
                        if (!insideGlyph(x, y)) points.push([x, y]);
                    }
                }
                return points;
            }"""
        )
        if len(safe_points) >= 4:
            clip = {"x": box["x"], "y": box["y"], "width": max(box["width"], 1), "height": max(box["height"], 1)}
            png_bytes = page.screenshot(clip=clip)
            img = Image.open(io.BytesIO(png_bytes)).convert("RGB")
            w, h = img.size
            samples = []
            for px, py in safe_points:
                lx = min(max(int(px - box["x"]), 0), w - 1)
                ly = min(max(int(py - box["y"]), 0), h - 1)
                samples.append(img.getpixel((lx, ly)))
        else:
            # No safe interior point: the glyphs fill the box, so look just
            # outside it instead.
            margin = max(6, round(box["height"] * 0.3))
            x = max(box["x"] - margin, 0)
            y = max(box["y"] - margin, 0)
            clip = {
                "x": x,
                "y": y,
                "width": max(box["width"] + (box["x"] - x) + margin, 1),
                "height": max(box["height"] + (box["y"] - y) + margin, 1),
            }
            png_bytes = page.screenshot(clip=clip)
            img = Image.open(io.BytesIO(png_bytes)).convert("RGB")
            w, h = img.size
            if w < 4 or h < 4:
                return None
            cols = 6
            samples = []
            for row_frac in (0.03, 0.97):
                row_y = min(max(int(h * row_frac), 0), h - 1)
                for i in range(cols):
                    col_x = min(max(int(w * (i + 0.5) / cols), 0), w - 1)
                    samples.append(img.getpixel((col_x, row_y)))
        if not samples:
            return None
        r = sorted(s[0] for s in samples)[len(samples) // 2]
        g = sorted(s[1] for s in samples)[len(samples) // 2]
        b = sorted(s[2] for s in samples)[len(samples) // 2]
        return (r, g, b)
    except Exception:
        return None


def run_checks(page):
    findings = []

    def fail(check, detail):
        findings.append({"check": check, "status": "fail", "detail": detail})

    def ok(check, detail=""):
        findings.append({"check": check, "status": "pass", "detail": detail})

    # ---- 1. Em-dash / en-dash anywhere in text — including collapsed/hidden
    # content (accordions, tabs) a user can reach by interacting. innerText
    # would miss these since it respects current CSS display; textContent
    # does not, and "anywhere visible to the user" includes content one click
    # away, not just what's on screen right now.
    body_text = page.eval_on_selector("body", "el => el.textContent")
    em_hits = sum(body_text.count(c) for c in EM_DASH_CHARS)
    if em_hits > 0:
        fail("no-em-dash", f"found {em_hits} em/en-dash character(s) in page text (including collapsed content)")
    else:
        ok("no-em-dash")

    # ---- 2. Hero heading fits in 2 lines ----
    # offsetHeight, not bounding_box()/getBoundingClientRect(): a rotated
    # heading (the Z-Axis Cascade layout pattern applies transform: rotate()
    # to hero cards) inflates the axis-aligned bbox height well beyond the
    # element's real, rendered line count. offsetHeight is the layout box
    # height and is unaffected by a CSS transform on the element itself.
    #
    # A real <h1> isn't always the visible hero headline: a common pattern
    # (seen on a real site during testing) is a visually-hidden `sr-only` h1
    # carrying the site/brand name for SEO, with the actual prominent
    # headline marked up as h2 or lower. Checking a near-zero-height h1
    # would silently false-pass regardless of what the visible headline
    # actually does, so fall back to the largest-font heading-level element
    # near the top of the page when the h1 itself isn't rendered.
    h1 = page.query_selector("h1")
    h1_visible_height = h1.evaluate("el => el.offsetHeight") if h1 else 0
    if h1 and h1_visible_height > 2:
        heading = h1
    else:
        # Font-size is not a reliable ranking signal here: a real site can
        # style a later section header (e.g. "I'm looking for...") bigger
        # than its own actual hero tagline, which sits in a modest-sized
        # masthead widget near the very top. Position first (tight window,
        # not "anywhere in the first viewport") then longest text content
        # among what's left: a real tagline reads as a full sentence, a
        # widget label ("Find a course") doesn't.
        # "Near the top" is relative to the actual configured viewport (this
        # script takes --viewport), not a fixed pixel count: a fixed 500px
        # cutoff would silently stop meaning "near the top" on a short mobile
        # viewport or a tall custom one. 0.6 of viewport height is a tight
        # enough window to exclude a lower section's own header while still
        # covering a hero that runs a bit taller than the fold.
        heading = page.evaluate_handle(
            """(maxTop) => {
                const candidates = Array.from(document.querySelectorAll('h1,h2,h3'));
                let best = null, bestLen = 0;
                for (const el of candidates) {
                    const rect = el.getBoundingClientRect();
                    if (rect.top > maxTop || rect.width === 0 || el.offsetHeight <= 2) continue;
                    const len = el.textContent.trim().length;
                    if (len > bestLen) { bestLen = len; best = el; }
                }
                return best;
            }""",
            page.viewport_size["height"] * 0.6,
        ).as_element()
    if heading:
        offset_height = heading.evaluate("el => el.offsetHeight")
        line_height = heading.evaluate("el => parseFloat(getComputedStyle(el).lineHeight)")
        # `line_height` is NaN (not falsy in Python!) when `line-height`
        # computes to a non-numeric value ("normal" is the CSS default, e.g.
        # a heading whose intended selector didn't actually match its
        # element). `line_height > 0` is False for NaN, unlike a bare
        # truthiness check, so this can't reach `round(x / NaN)` and crash.
        if offset_height and line_height > 0:
            lines = round(offset_height / line_height)
            if lines > 2:
                fail("hero-line-count", f"heading renders as ~{lines} lines, budget is 2")
            else:
                ok("hero-line-count", f"~{lines} lines")
        else:
            ok("hero-line-count", "could not measure, skipped")
    else:
        fail("hero-line-count", "no visible h1/h2/h3 heading found near the top of the page")

    # ---- 3. CTA buttons: single line, no wrap ----
    # Scoped to actual buttons/CTAs, not every <a>/<button> on the page — a
    # model card is an <a> too, and its multi-line content is by design, not
    # a wrap bug. Detection is structural (proximity to the hero heading
    # found in check 2, plus anything inside <nav>), not tied to any assumed
    # class name: an earlier version hardcoded `.hero-ctas`/`.btn`, which
    # silently checked zero hero elements on any direction/variant that used
    # a different (equally valid) class name — confirmed by running it
    # against real generated markup that used `.ctas` instead. A class name
    # is a convention one skill happens to follow; "sits next to the hero
    # heading" is true regardless of what any given skill calls its wrapper.
    # Wrap is measured on the TEXT's own line boxes (Range.getClientRects),
    # not the container's box height — a container's vertical padding
    # otherwise reads as false-positive "wrapping."
    # Visible-only: a responsive duplicate sitting in a collapsed mobile
    # drawer at this viewport isn't something a user sees without first
    # opening the drawer — that's a different breakpoint/interaction state
    # (re-run with --viewport for it), not a defect in this render.
    # is_visible() alone isn't enough: it misses an element that has real
    # size and isn't display:none but is clipped by an ancestor's
    # overflow:hidden (a closed dropdown/drawer still occupying box space).
    # elementFromPoint at the element's own center is the actual "is this the
    # thing a user would see if they looked here right now" test.
    def truly_visible(el):
        # Scrolled into view first: below-the-fold content (a footer nav, say)
        # is normal and reachable, not "not visible" — it just isn't in the
        # viewport at the current scroll position. A closed drawer item stays
        # occluded even after scrolling, since its own ancestor clips it
        # regardless of page scroll; that's the real signal being isolated.
        try:
            el.scroll_into_view_if_needed(timeout=2000)
        except Exception:
            return False
        return el.evaluate(
            """el => {
                const rect = el.getBoundingClientRect();
                if (rect.width === 0 || rect.height === 0) return false;
                const cx = rect.left + rect.width / 2;
                const cy = rect.top + rect.height / 2;
                if (cx < 0 || cy < 0 || cx > window.innerWidth || cy > window.innerHeight) return false;
                const topEl = document.elementFromPoint(cx, cy);
                return !!topEl && (topEl === el || el.contains(topEl) || topEl.contains(el));
            }"""
        )

    # Mark hero CTAs by structure: walk up from the hero heading (found in
    # check 2, whatever tag/class it actually uses) to the nearest ancestor
    # that contains an <a>/<button> outside of <nav>, and tag those. Capped
    # at 5 levels AND stopped at <main>/<body>/<html> — the level cap alone
    # doesn't help on a page where <main> sits just 1-2 levels above the
    # hero and already contains every other section: confirmed on a real
    # page whose hero had no real CTA at all, where the walk reached <main>
    # in a single step and mislabeled an unrelated link buried in a later
    # section (a page-footer "more about the studio" link) as the hero's
    # own CTA — same class of bug as hero-visual-present's ancestor walk
    # needing this exact stop condition, just in a different check.
    if heading:
        page.evaluate(
            """(headingEl) => {
                document.querySelectorAll('[data-rf-cta]').forEach(el => el.removeAttribute('data-rf-cta'));
                if (!headingEl) return;
                let container = headingEl.parentElement;
                for (let i = 0; i < 5 && container; i++) {
                    if (container === document.body || container === document.documentElement
                        || container.tagName === 'MAIN') break;
                    const interactive = Array.from(container.querySelectorAll('a, button'))
                        .filter(el => !el.closest('nav'));
                    if (interactive.length > 0) {
                        interactive.forEach(el => el.setAttribute('data-rf-cta', 'hero'));
                        break;
                    }
                    container = container.parentElement;
                }
            }""",
            heading,
        )
    cta_selector = "nav a, nav button, [data-rf-cta='hero']"
    ctas = [el for el in page.query_selector_all(cta_selector) if el.is_visible() and truly_visible(el)]
    wrapped = []
    seen_buckets = {}
    for el in ctas:
        text = (el.inner_text() or "").strip()
        if not text or len(text) > 40 or "\n" in text:
            continue  # multi-line-by-design content, not a single CTA label
        # Measured on text nodes only (not selectNodeContents(el)): an inline
        # icon/svg sibling next to the label otherwise contributes its own
        # rect and reads as a false-positive wrap on any icon+text CTA.
        line_count = el.evaluate(
            """el => {
                const walker = document.createTreeWalker(
                    el, NodeFilter.SHOW_TEXT,
                    { acceptNode: n => n.nodeValue.trim().length > 0
                        ? NodeFilter.FILTER_ACCEPT : NodeFilter.FILTER_SKIP }
                );
                const rects = [];
                let node;
                while ((node = walker.nextNode())) {
                    const range = document.createRange();
                    range.selectNodeContents(node);
                    for (const r of range.getClientRects()) {
                        if (r.width > 0 && r.height > 0) rects.push(r);
                    }
                }
                const lines = [];
                for (const r of rects) {
                    const cy = r.top + r.height / 2;
                    const match = lines.find(l => Math.abs(l.cy - cy) < l.height / 2);
                    if (match) { match.cy = (match.cy + cy) / 2; }
                    else { lines.push({ cy, height: r.height }); }
                }
                return lines.length;
            }"""
        )
        if line_count and line_count > 1:
            wrapped.append(text)
        bucket = cta_bucket(text)
        if bucket:
            seen_buckets.setdefault(bucket, []).append(text)
    if wrapped:
        fail("cta-no-wrap", f"CTA text wraps to 2+ lines: {wrapped}")
    else:
        ok("cta-no-wrap")

    dup_intent = {b: labels for b, labels in seen_buckets.items() if len(set(labels)) > 1}
    if dup_intent:
        fail("no-duplicate-cta-intent", f"same intent, different labels on page: {dup_intent}")
    else:
        ok("no-duplicate-cta-intent")

    # ---- 4. Contrast: every CTA's text vs its EFFECTIVE background ----
    # Background is read from actual rendered pixels (sample_bg_color), not
    # just getComputedStyle: a gradient/image fill painted by a ::before
    # overlay is invisible to a CSS-property walk-up but shows up in the
    # pixels. The DOM walk-up is kept only as a fallback for when pixel
    # sampling can't run (zero-size or off-viewport box).
    #
    # Re-scroll each element into view right before measuring it: `ctas` was
    # built by filtering through truly_visible(), which scrolls the page to
    # each candidate in turn and leaves it wherever the LAST one landed. On a
    # real site with a large <nav> (confirmed on one with 100+ nav-tagged
    # links — a full sitemap, not just a top bar), bounding_box() for an
    # earlier element in the list can come back hundreds of pixels off-screen
    # (e.g. y around -5000) purely because of that leftover scroll position,
    # not because the element actually moved. sample_bg_color silently
    # returns None for an off-viewport box, which used to fall through to the
    # DOM walk-up and could match the wrong ancestor (e.g. a white <body>
    # instead of the real dark fixed-nav background), reporting a false
    # contrast failure against colors nobody actually sees stacked together.
    contrast_fails = []
    contrast_checked = 0
    for el in ctas:
        text = (el.inner_text() or "").strip()
        if not text:
            continue
        try:
            el.scroll_into_view_if_needed(timeout=2000)
        except Exception:
            pass
        fg_css = el.evaluate("el => getComputedStyle(el).color")
        fg = parse_rgb(fg_css, treat_transparent_as_none=False)
        box = el.bounding_box()
        bg = sample_bg_color(page, el, box) if box else None
        if bg is None:
            bg_css = el.evaluate(
                """el => {
                    let node = el.parentElement;
                    let bg = 'rgba(0,0,0,0)';
                    while (node) {
                        const c = getComputedStyle(node).backgroundColor;
                        if (c && c !== 'rgba(0, 0, 0, 0)' && c !== 'transparent') { bg = c; break; }
                        node = node.parentElement;
                    }
                    return bg;
                }"""
            )
            bg = parse_rgb(bg_css)
        if fg is None or bg is None:
            continue  # no resolvable background anywhere — genuinely can't check, skip rather than guess
        contrast_checked += 1
        ratio = contrast_ratio(fg, bg)
        if ratio < 4.5:
            contrast_fails.append({"text": text, "ratio": round(ratio, 2)})
    if contrast_fails:
        fail("contrast-aa", f"below 4.5:1: {contrast_fails}")
    else:
        ok("contrast-aa", f"checked {contrast_checked} interactive elements")

    # ---- 5. Eyebrow budget: uppercase tracked micro-labels ABOVE a headline ----
    # The rule (design-taste-frontend §4.7) is specifically about a small
    # label sitting above a section headline — not any small-caps text
    # anywhere on the page. Without the "immediately precedes a heading"
    # constraint this over-matches table headers, card metadata badges, and
    # footer <h4>s, none of which are eyebrows in the rule's sense.
    section_count = len(page.query_selector_all("section")) or 1
    eyebrow_candidates = page.evaluate(
        """() => {
            const els = Array.from(document.querySelectorAll('*'));
            const matches = els.filter(el => {
                const s = getComputedStyle(el);
                const text = el.textContent.trim();
                const styled = text.length > 0 && text.length < 40
                    && s.textTransform === 'uppercase'
                    && parseFloat(s.letterSpacing) > 0.5
                    && el.children.length === 0;
                if (!styled) return false;
                let sib = el.nextElementSibling;
                for (let i = 0; i < 2 && sib; i++) {
                    if (/^H[1-3]$/.test(sib.tagName)) return true;
                    sib = sib.nextElementSibling;
                }
                return false;
            });

            // Collapse repeated sibling instances of one component (e.g. 3
            // "step-kicker" labels inside 3 near-identical .step cards) into
            // ONE budget unit — that's one cohesive multi-part element using
            // consistent internal labeling, not three separate page-level
            // eyebrow tells. Grouped by: same class on the candidate AND its
            // immediate parent is one of >=2 siblings sharing the parent's
            // own tag+class (i.e. genuinely a repeated list/grid item).
            const seenGroupKeys = new Set();
            let budgetUnits = 0;
            for (const el of matches) {
                const parent = el.parentElement;
                const parentSiblingsSameShape = parent && parent.parentElement
                    ? Array.from(parent.parentElement.children).filter(
                        c => c.tagName === parent.tagName && c.className === parent.className
                      ).length
                    : 1;
                const isRepeatedComponent = parentSiblingsSameShape >= 2 && el.className;
                const groupKey = isRepeatedComponent
                    ? 'group:' + el.className
                    : 'solo:' + el.className + ':' + el.textContent.trim();
                if (!seenGroupKeys.has(groupKey)) {
                    seenGroupKeys.add(groupKey);
                    budgetUnits += 1;
                }
            }
            return budgetUnits;
        }"""
    )
    budget = -(-section_count // 3)  # ceil(sectionCount / 3)
    if eyebrow_candidates > budget:
        fail("eyebrow-budget", f"{eyebrow_candidates} eyebrow-like labels found, budget is {budget} (ceil({section_count}/3))")
    else:
        ok("eyebrow-budget", f"{eyebrow_candidates}/{budget}")

    # ---- 6. Navigation on one line ----
    # A raw getBoundingClientRect() on <nav> includes collapsed mega-menu
    # panels: visibility:hidden (unlike display:none) still occupies its
    # full layout height, so a closed dropdown can make an ordinary
    # one-line nav bar measure as 500px+. Walk descendants and use only the
    # max bottom edge among genuinely visible ones (visibility != hidden,
    # display != none, opacity != 0) to get the height a user actually sees.
    nav = page.query_selector("nav")
    if nav:
        nav_height = nav.evaluate(
            """el => {
                const navTop = el.getBoundingClientRect().top;
                let maxBottom = navTop;
                for (const child of [el, ...el.querySelectorAll('*')]) {
                    const cs = getComputedStyle(child);
                    if (cs.visibility === 'hidden' || cs.display === 'none' || parseFloat(cs.opacity) === 0) continue;
                    const r = child.getBoundingClientRect();
                    if (r.width === 0 || r.height === 0) continue;
                    if (r.bottom > maxBottom) maxBottom = r.bottom;
                }
                return maxBottom - navTop;
            }"""
        )
        # Target is ~80px for a one-line nav at typical padding/font sizes;
        # the fail threshold sits 10px above that as slack for line-height
        # and padding variance across real sites, not a second, unrelated
        # number.
        NAV_SINGLE_LINE_TARGET_PX = 80
        NAV_SINGLE_LINE_FAIL_PX = NAV_SINGLE_LINE_TARGET_PX + 10
        if nav_height and nav_height > NAV_SINGLE_LINE_FAIL_PX:
            fail("nav-single-line", f"nav height {round(nav_height)}px, expected <= ~{NAV_SINGLE_LINE_TARGET_PX}px for a one-line nav")
        else:
            ok("nav-single-line", f"{round(nav_height)}px" if nav_height else "no visible content inside <nav>, skipped (the visible top bar may not be the <nav>-tagged element on this site)")
    else:
        ok("nav-single-line", "no <nav> element found, skipped")

    # ---- 7. Real logo present, not silently dropped to a text-only stand-in ----
    # Same home-link heuristic capture-assets.py uses to find a real logo: a
    # site's logo almost always sits inside an <a> pointing at the home page.
    # This never fails on its own — a plain text wordmark can be the genuine,
    # correct brand identity for a site with no graphical mark. It's
    # informational, meant to be cross-checked against discover.json's
    # assets.logo entry (validate-design.md's job): if Discover captured a
    # real logo but this reports false, that's a real regression introduced
    # during Implement, not an acceptable design choice.
    # Existence alone isn't enough: an <img> can sit in the DOM at the right
    # size and still never actually decode (naturalWidth 0), which reads to
    # a human as "no logo" every bit as much as a missing element does.
    # Confirmed for real: a relative `<img src="assets/logo.svg">` reported
    # `has_logo_image = true` under the old element-only check while
    # visibly showing nothing, because this environment's Browser pane
    # renders a local file outside the project directory as a static
    # snapshot that doesn't fetch sibling files by relative path (see
    # `implement-design.md`'s asset-pipeline note). An inline <svg> has no
    # naturalWidth property at all (only <img> does) and is never fetched,
    # so it can't have this failure mode -- only check <img> elements for it.
    logo_status = page.evaluate(
        """() => {
            const homeLinks = Array.from(document.querySelectorAll('a[href]')).filter(a => {
                try {
                    const path = new URL(a.href, document.baseURI).pathname;
                    return path === '/' || a.getAttribute('href') === '/';
                } catch { return false; }
            });
            for (const a of homeLinks) {
                const el = a.querySelector('img, svg');
                if (!el) continue;
                const rect = el.getBoundingClientRect();
                // 32/16, not 24/12: must match capture-assets.py's own
                // MIN_LOGO_WIDTH/HEIGHT exactly. A real gap existed here
                // before -- capture-assets.py raised its own floor to 32 to
                // clear a real 36px square icon-mark logo while staying
                // above MAX_ICON_HEIGHT's 28px UI-icon boundary, but this
                // check still accepted anything down to 24px: a real logo in
                // the 24-31px range would fail to be captured as "the logo"
                // by capture-assets.py yet still report `present` here,
                // silently disagreeing about the exact same element.
                if (rect.width < 32 || rect.height < 16) continue;
                if (el.tagName === 'IMG' && el.complete && el.naturalWidth === 0) {
                    return 'broken';
                }
                return 'present';
            }
            return 'absent';
        }"""
    )
    if logo_status == "broken":
        fail(
            "logo-present",
            "an <img> logo exists in a home-link at a plausible size but never decoded "
            "(naturalWidth 0 despite complete) -- a real broken image, not a text-only "
            "brand choice; check the src resolves in this render context",
        )
    else:
        ok(
            "logo-present",
            "image/svg logo found in a home-link" if logo_status == "present"
            else "no graphical logo found in a home-link (text-only brand name, or no home-link) -- cross-check discover.json's assets.logo before treating this as fine",
        )

    # ---- 8. Real hero visual present, not silently dropped to bare text ----
    # Same anchor-and-walk technique capture-assets.py's find_hero_visual
    # uses: real <img>, <video>, or CSS background-image within a few
    # ancestor levels of the real <h1>. Also never fails on its own — some
    # genuine designs run a solid-color or gradient hero with no photo/video
    # at all. Informational, meant to be cross-checked against
    # discover.json's assets.hero_visual: if Discover captured a real hero
    # visual but this reports none, that's a real regression introduced
    # during Implement, not an acceptable design choice. This exists
    # because the capture step itself used to be skipped by accident: an
    # earlier version only captured whatever a human remembered to name,
    # and a real hero photo went missing on a real site until someone
    # noticed it by eye afterward.
    # Ancestor walking alone is not a tight enough boundary: confirmed on a
    # real page where the h1's 2nd-level ancestor was already <main>, whose
    # querySelector('img') matched a product-grid photo hundreds of pixels
    # further down — a real descendant, but nowhere near the hero visually,
    # which made this check trivially true on almost any page (some image
    # exists *somewhere* under <main>/<body>) and unable to ever catch a
    # real regression. Every candidate must sit within a real vertical
    # distance of the h1 itself; a container's background-image is only
    # trusted while that container is still hero-sized, not page-spanning.
    has_hero_visual = page.evaluate(
        """() => {
            const h1 = document.querySelector('h1');
            if (!h1) return false;
            const h1Rect = h1.getBoundingClientRect();
            const nearHero = (rect) => rect.top < h1Rect.bottom + 700 && rect.bottom > h1Rect.top - 200;

            let container = h1;
            for (let i = 0; i < 6 && container; i++) {
                const containerRect = container.getBoundingClientRect();
                if (containerRect.height > window.innerHeight * 1.5) break;

                for (const img of container.querySelectorAll('img')) {
                    const rect = img.getBoundingClientRect();
                    if (rect.width >= 80 && rect.height >= 80 && nearHero(rect)) return true;
                }
                const video = container.querySelector('video');
                if (video && nearHero(video.getBoundingClientRect())) return true;
                const bg = getComputedStyle(container).backgroundImage;
                if (bg && bg.includes('url(')) return true;
                container = container.parentElement;
            }
            return false;
        }"""
    )
    ok(
        "hero-visual-present",
        "image, video, or background-image found near the hero heading" if has_hero_visual
        else "no hero visual found near the h1 -- cross-check discover.json's assets.hero_visual before treating this as fine",
    )

    # ---- 9. Viewport meta tag present ----
    # A real Fail, not informational: without this tag, a real mobile browser
    # renders at a fake wide desktop-width layout and scales it down, so
    # every @media rule on the page silently never evaluates against the
    # device's actual width. Confirmed missing on a real run this session —
    # every @media breakpoint had been written correctly and still had zero
    # effect on a real mobile render, because Playwright's own viewport=
    # override (what every other check in this file runs under) sets the
    # layout viewport directly regardless of this tag, which is exactly why
    # this bug stayed invisible to every other check here. This one check
    # is what actually catches it.
    has_viewport_meta = page.evaluate(
        """() => {
            const m = document.querySelector('meta[name="viewport"]');
            return !!(m && /width\\s*=\\s*device-width/i.test(m.getAttribute('content') || ''));
        }"""
    )
    if has_viewport_meta:
        ok("viewport-meta-present")
    else:
        fail(
            "viewport-meta-present",
            'missing <meta name="viewport" content="width=device-width, initial-scale=1"> -- '
            "real mobile browsers will render this page at a fake wide desktop layout and scale "
            "it down, silently disabling every @media breakpoint on the page",
        )

    return findings


def count_multicolumn_grids(page):
    """How many elements render as a CSS grid with 2+ real column tracks,
    at whatever viewport `page` currently has. A generic signal that works
    across any real template (card grids, hero-grids, news grids all use
    this), without needing to match specific elements by class name."""
    return page.evaluate(
        """() => {
            let count = 0;
            for (const el of document.querySelectorAll('*')) {
                const style = getComputedStyle(el);
                if (style.display !== 'grid' && style.display !== 'inline-grid') continue;
                const tracks = style.gridTemplateColumns.split(' ').filter(Boolean);
                if (tracks.length >= 2) count++;
            }
            return count;
        }"""
    )


def check_responsive_collapse(browser, target, primary_w, primary_h):
    """Check 10: does any multi-column grid actually collapse at a narrower
    width? Confirmed necessary the hard way: every @media rule in a real
    template can be written correctly and this can still silently do
    nothing (see viewport-meta-present above) — that check catches the tag
    being missing, this one catches the breakpoint not actually firing for
    any other reason (a wrong max-width, a selector typo, the rule living in
    the wrong file). Compares a *count* of multi-column grids between two
    real viewport widths, not specific elements by identity: robust to any
    real template's actual class names.

    The primary viewport picks which width is "wide" for this comparison: if
    it's already mobile-narrow (<=480px), compare against a wider 1280px
    render instead, so the check is always comparing two genuinely different
    widths regardless of what --viewport the rest of the checks used.
    """
    if primary_w <= 480:
        wide_w, narrow_w = 1280, primary_w
    else:
        wide_w, narrow_w = primary_w, 375

    counts = {}
    for label, w in (("wide", wide_w), ("narrow", narrow_w)):
        page = browser.new_page(viewport={"width": w, "height": primary_h})
        try:
            page.goto(target, wait_until="networkidle", timeout=30_000)
            counts[label] = count_multicolumn_grids(page)
        finally:
            page.close()

    if counts["wide"] == 0:
        return {
            "check": "responsive-layout-collapses",
            "status": "pass",
            "detail": f"no multi-column grid found at {wide_w}px to test collapsing -- nothing to check",
        }
    if counts["narrow"] < counts["wide"]:
        return {
            "check": "responsive-layout-collapses",
            "status": "pass",
            "detail": f"{counts['wide']} multi-column grid(s) at {wide_w}px, {counts['narrow']} at {narrow_w}px",
        }
    return {
        "check": "responsive-layout-collapses",
        "status": "fail",
        "detail": (
            f"{counts['wide']} multi-column grid(s) at {wide_w}px, still {counts['narrow']} at "
            f"{narrow_w}px -- none of them actually collapsed to fewer columns at the narrower width"
        ),
    }


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("target", help="URL (http://localhost:...) or local file path")
    parser.add_argument("--out", default=None, help="write JSON report here")
    parser.add_argument("--viewport", default="1280x800")
    args = parser.parse_args()

    target = args.target
    if not target.startswith("http") and not target.startswith("file://"):
        target = Path(target).resolve().as_uri()

    w, h = (int(x) for x in args.viewport.split("x"))

    with sync_playwright() as p:
        browser = p.chromium.launch()
        try:
            page = browser.new_page(viewport={"width": w, "height": h})
            try:
                page.goto(target, wait_until="networkidle", timeout=30_000)
            except Exception as e:
                # A real, unfamiliar site can carry a chat widget or analytics
                # beacon that keeps polling forever, so "networkidle" never
                # settles — that's a live-site condition to report clearly,
                # not a crash to hand Claude as a raw traceback.
                sys.exit(f"could not load {target} within 30s ({type(e).__name__}: {e})")
            findings = run_checks(page)
            findings.append(check_responsive_collapse(browser, target, w, h))
        finally:
            browser.close()

    passed = sum(1 for f in findings if f["status"] == "pass")
    failed = [f for f in findings if f["status"] == "fail"]

    report = {
        "target": target,
        "checks_run": len(findings),
        "passed": passed,
        "failed": len(failed),
        "findings": findings,
    }

    out_json = json.dumps(report, indent=2)
    if args.out:
        Path(args.out).write_text(out_json, encoding="utf-8")
    print(out_json)

    sys.exit(1 if failed else 0)


if __name__ == "__main__":
    main()
