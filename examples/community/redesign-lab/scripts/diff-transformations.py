#!/usr/bin/env python3
"""
diff-transformations.py — real, deterministic before/after comparison for the
Share stage's case study. Compares two rendered pages (the real current site
and the real Gate-2-chosen variant) on a fixed set of computed-style axes,
scores each real difference found by how significant it is, and outputs a
ranked list. Deciding which ranked differences become case-study *chapters*
(3-6, "meaningful transformations, not transformation count") stays a human/
agent judgment call — this script's job stops at producing objective,
verified facts to judge from, the same division of labor validated by the
larstornoe.com Golden Case Study, where this same comparison was done by hand.

No model call, no loomloom, $0 always — the same guarantee mechanical-check.py
makes, for the same reason: this is comparing rendered facts, not forming an
opinion.

Usage:
    python diff-transformations.py <before-url-or-file> <after-url-or-file> [--out report.json]
"""

import argparse
import json
import re
import sys
from pathlib import Path

from playwright.sync_api import sync_playwright


def to_target(path_or_url):
    if path_or_url.startswith("http://") or path_or_url.startswith("https://"):
        return path_or_url
    p = Path(path_or_url).resolve()
    return p.as_uri()


def extract_facts(page):
    """One real, computed fact per axis. Never guesses: every value here is
    read directly from the rendered page, the same way mechanical-check.py
    reads computed styles rather than trusting source-order assumptions."""
    facts = {}

    # ---- Color tokens: read :root custom properties directly, not guessed
    # from source text (a custom property can be set anywhere, including via
    # JS). Filtered to values that actually look like colors -- a real
    # :root commonly mixes non-color tokens in with color tokens (--shadow,
    # --radius, --gap), and without this filter those get reported as "root
    # color tokens" too. Confirmed against this project's own design-spec
    # artifact, whose :root defines --shadow: 0 1px 2px rgba(...) alongside
    # real colors. ----
    facts["root_colors"] = page.evaluate(
        """() => {
            const cs = getComputedStyle(document.documentElement);
            const out = {};
            const colorLike = /^(#[0-9a-f]{3,8}\\b|rgba?\\(|hsla?\\(|(transparent|white|black|red|blue|green|yellow|orange|purple|pink|gray|grey|brown|cyan|magenta|navy|teal|maroon|olive|silver|gold|beige|ivory|coral|salmon|crimson|indigo|violet|lavender|khaki|tan)$)/i;
            for (const prop of cs) {
                if (prop.startsWith('--')) {
                    const val = cs.getPropertyValue(prop).trim();
                    if (val && colorLike.test(val)) out[prop] = val;
                }
            }
            return out;
        }"""
    )

    # ---- Headline typography: the same "find the real hero heading" logic
    # mechanical-check.py already uses, so this stays consistent with what
    # Validate itself considers "the heading" ----
    heading_style = page.evaluate(
        """() => {
            const h = document.querySelector('h1') || document.querySelector('h1,h2,h3');
            if (!h || h.offsetHeight <= 2) return null;
            const cs = getComputedStyle(h);
            return {
                fontFamily: cs.fontFamily,
                fontWeight: cs.fontWeight,
                fontSize: cs.fontSize,
                letterSpacing: cs.letterSpacing,
                textTransform: cs.textTransform,
                lineHeight: cs.lineHeight,
            };
        }"""
    )
    facts["heading_style"] = heading_style

    # ---- Nav typography ----
    nav_style = page.evaluate(
        """() => {
            const a = document.querySelector('nav a, header nav a');
            if (!a) return null;
            const cs = getComputedStyle(a);
            return {
                fontFamily: cs.fontFamily,
                fontWeight: cs.fontWeight,
                fontSize: cs.fontSize,
                letterSpacing: cs.letterSpacing,
                textTransform: cs.textTransform,
            };
        }"""
    )
    facts["nav_style"] = nav_style

    # ---- Structural framing: real border presence, not assumed from a class
    # name ----
    framing = page.evaluate(
        """() => {
            const header = document.querySelector('header');
            const headerBorder = header ? getComputedStyle(header).borderBottomWidth : '0px';
            // Section-label heuristic: short, uppercase-tracked, monospace-ish
            // text nodes immediately preceding a heading -- the real signature
            // of an added "[ STUDIO / NO-01 ]" / "/// 01" style label, not a
            // guess based on class names (which vary per direction).
            //
            // `text === text.toUpperCase()` is meant to catch deliberately
            // all-caps English labels, but it's vacuously true for any string
            // with no case distinction at all -- confirmed on a real Chinese-
            // language site: a label reading "自然语言" trivially equals its
            // own .toUpperCase() (CJK script has no case), so a check meant
            // to require "this text was deliberately capitalized" silently
            // passed for text where capitalization isn't even a concept.
            // Require at least one actual Latin letter so the uppercase
            // comparison is testing something real, not a vacuous truth.
            const headings = Array.from(document.querySelectorAll('h1,h2,h3'));
            let labelCount = 0;
            for (const h of headings) {
                const prev = h.previousElementSibling;
                if (!prev) continue;
                const text = prev.textContent.trim();
                const cs = getComputedStyle(prev);
                const hasLatinLetter = /[a-zA-Z]/.test(text);
                const looksLikeLabel = text.length > 0 && text.length < 40
                    && hasLatinLetter && text === text.toUpperCase()
                    && /monospace|courier/i.test(cs.fontFamily);
                if (looksLikeLabel) labelCount++;
            }
            return {
                headerBorderPresent: parseFloat(headerBorder) > 0,
                headerBorderWidth: headerBorder,
                sectionLabelCount: labelCount,
            };
        }"""
    )
    facts["framing"] = framing

    # ---- Border radius: real presence/absence anywhere meaningful (cards,
    # buttons, images), not just the body ----
    radius = page.evaluate(
        """() => {
            const els = Array.from(document.querySelectorAll('img, a, button, .card, [class*="card"]')).slice(0, 40);
            let anyRounded = false;
            for (const el of els) {
                const r = parseFloat(getComputedStyle(el).borderRadius);
                if (r > 2) { anyRounded = true; break; }
            }
            return { anyRounded };
        }"""
    )
    facts["radius"] = radius

    # ---- Layout mechanism + content hierarchy: real grid/column structure,
    # and whether one item is structurally emphasized (larger, alone in its
    # own row/section) above the rest ----
    layout = page.evaluate(
        """() => {
            const candidates = Array.from(document.querySelectorAll('[style*=\"column-count\"], div, section'))
                .map(el => getComputedStyle(el))
                .filter(cs => cs.columnCount && cs.columnCount !== 'auto');
            const columnCounts = [...new Set(candidates.map(cs => cs.columnCount))];
            const grids = Array.from(document.querySelectorAll('*')).map(el => getComputedStyle(el))
                .filter(cs => cs.display === 'grid' || cs.display === 'inline-grid')
                .map(cs => cs.gridTemplateColumns.split(' ').filter(Boolean).length)
                .filter(n => n > 1);
            // Featured/hero heuristic: a real <img> whose rendered width is
            // meaningfully larger (>1.4x) than the median image width on the
            // page, appearing before the main repeating grid.
            const imgs = Array.from(document.querySelectorAll('img'))
                .map(img => img.getBoundingClientRect().width)
                .filter(w => w > 20)
                .sort((a, b) => a - b);
            let hasFeatured = false;
            if (imgs.length > 2) {
                const median = imgs[Math.floor(imgs.length / 2)];
                const largest = imgs[imgs.length - 1];
                hasFeatured = largest > median * 1.4;
            }
            return {
                columnCounts,
                gridColumnCounts: [...new Set(grids)],
                imageCount: imgs.length,
                hasFeaturedItem: hasFeatured,
            };
        }"""
    )
    facts["layout"] = layout

    return facts



# Significance scoring, principled rather than per-category magic numbers.
# Three tiers, applied consistently below:
#   PRESENCE  -- something structural appeared or disappeared wholesale
#                (an accent color joined the palette, a featured item showed
#                up). The biggest real signal a page can carry.
#   SCALED    -- a multi-property fact (typography, structural framing)
#                scores by how many of its sub-properties actually changed,
#                capped so no single fact can out-rank a presence change.
#   FIXED     -- a real but inherently single-valued fact (a boolean flips,
#                and there's nothing further to scale by). Categories still
#                get their own fixed weight where the underlying change
#                genuinely differs in visual importance (a featured item
#                reorganizing the page reads as a bigger deal than corners
#                going square) -- that's a deliberate weighting, not an
#                unexplained inconsistency.
SIG_PRESENCE = 6
SIG_SCALED_BASE = 2
SIG_SCALED_CAP = 6
SIG_NAV_BASE = 1       # nav is real signal but less visually prominent than
SIG_NAV_CAP = 5        # the headline, so its scale sits a notch lower
SIG_CORNER_TREATMENT = 3   # single boolean flip, no sub-properties to scale by
SIG_CONTENT_HIERARCHY = 6  # single boolean flip, but a featured item appearing
                           # is a real content-hierarchy change, not decoration
SIG_LAYOUT_DENSITY = 3     # secondary structural signal, usually accompanies
                           # a content-hierarchy or framing change rather than
                           # standing alone


def score_and_diff(before, after):
    """Real, deterministic comparisons -> ranked findings. See the SIG_*
    constants above for the scoring rationale."""
    findings = []

    # Color tokens
    b_colors, a_colors = before["root_colors"], after["root_colors"]
    if b_colors != a_colors:
        added_accent = len(a_colors) > len(b_colors) and len(b_colors) <= 3
        findings.append({
            "category": "color-system",
            "significance": SIG_PRESENCE if added_accent else SIG_SCALED_BASE + 1,
            "before_fact": f"Root color tokens: {b_colors}",
            "after_fact": f"Root color tokens: {a_colors}",
        })

    # Headline typography
    bh, ah = before["heading_style"], after["heading_style"]
    if bh and ah:
        changed_props = [k for k in bh if bh.get(k) != ah.get(k)]
        if changed_props:
            findings.append({
                "category": "headline-typography",
                "significance": min(SIG_SCALED_CAP, SIG_SCALED_BASE + len(changed_props)),
                "before_fact": f"Heading style: {bh}",
                "after_fact": f"Heading style: {ah}",
            })

    # Nav typography
    bn, an = before["nav_style"], after["nav_style"]
    if bn and an:
        changed_props = [k for k in bn if bn.get(k) != an.get(k)]
        if changed_props:
            findings.append({
                "category": "nav-typography",
                "significance": min(SIG_NAV_CAP, SIG_NAV_BASE + len(changed_props)),
                "before_fact": f"Nav link style: {bn}",
                "after_fact": f"Nav link style: {an}",
            })

    # Structural framing: two independent sub-signals (header border
    # presence, section-label count) -- score by how many actually changed,
    # symmetric in either direction, not just "did the label count increase."
    bf, af = before["framing"], after["framing"]
    framing_changed = (
        int(bf["headerBorderPresent"] != af["headerBorderPresent"])
        + int(bf["sectionLabelCount"] != af["sectionLabelCount"])
    )
    if framing_changed:
        findings.append({
            "category": "structural-framing",
            "significance": SIG_SCALED_BASE + 1 + 2 * (framing_changed - 1),  # 3 if one changed, 5 if both did
            "before_fact": f"Header border: {bf['headerBorderPresent']}, section labels: {bf['sectionLabelCount']}",
            "after_fact": f"Header border: {af['headerBorderPresent']}, section labels: {af['sectionLabelCount']}",
        })

    # Border radius
    if before["radius"]["anyRounded"] != after["radius"]["anyRounded"]:
        findings.append({
            "category": "corner-treatment",
            "significance": SIG_CORNER_TREATMENT,
            "before_fact": f"Rounded corners present: {before['radius']['anyRounded']}",
            "after_fact": f"Rounded corners present: {after['radius']['anyRounded']}",
        })

    # Content hierarchy (featured item)
    bl, al = before["layout"], after["layout"]
    if bl["hasFeaturedItem"] != al["hasFeaturedItem"]:
        findings.append({
            "category": "content-hierarchy",
            "significance": SIG_CONTENT_HIERARCHY,
            "before_fact": f"Featured/emphasized item present: {bl['hasFeaturedItem']} (flat grid of {bl['imageCount']} images)"
                            if not bl["hasFeaturedItem"] else f"Featured item present: True",
            "after_fact": f"Featured/emphasized item present: {al['hasFeaturedItem']}",
        })

    # Layout mechanism (column/grid count changed, independent of hierarchy)
    if bl["columnCounts"] != al["columnCounts"] or bl["gridColumnCounts"] != al["gridColumnCounts"]:
        findings.append({
            "category": "layout-density",
            "significance": SIG_LAYOUT_DENSITY,
            "before_fact": f"Column layout: multi-column counts {bl['columnCounts']}, grid track counts {bl['gridColumnCounts']}",
            "after_fact": f"Column layout: multi-column counts {al['columnCounts']}, grid track counts {al['gridColumnCounts']}",
        })

    findings.sort(key=lambda f: -f["significance"])
    return findings


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("before")
    parser.add_argument("after")
    parser.add_argument("--out", default=None)
    args = parser.parse_args()

    with sync_playwright() as p:
        browser = p.chromium.launch()
        results = {}
        for label, target in [("before", args.before), ("after", args.after)]:
            page = browser.new_page(viewport={"width": 1280, "height": 800})
            page.goto(to_target(target), wait_until="networkidle")
            results[label] = extract_facts(page)
            page.close()
        browser.close()

    findings = score_and_diff(results["before"], results["after"])
    report = {
        "before": args.before,
        "after": args.after,
        "real_differences_found": len(findings),
        "findings": findings,
    }
    out_json = json.dumps(report, indent=2)
    if args.out:
        Path(args.out).write_text(out_json, encoding="utf-8")
    print(out_json)
    print(
        f"\n{len(findings)} real difference(s) found. Select 3-6 of the most "
        "significant as case-study chapters -- if fewer than 3 real "
        "differences exist, use fewer than 3; never manufacture chapters to "
        "hit a target count.",
        file=sys.stderr,
    )


if __name__ == "__main__":
    main()
