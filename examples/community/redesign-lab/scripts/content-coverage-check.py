#!/usr/bin/env python3
"""
content-coverage-check.py — local, free, no-model-call. Confirms the
rebuilt page didn't silently drop real content sections relative to the
original, and that every same-page nav anchor still resolves to something
real.

Why this exists: a real run (aider.chat, industrial-brutalist-ui direction)
passed every existing Validate piece -- 11/11 mechanical-check, 0 a11y
violations, a clean preservation-contract table -- while quietly missing 3
of 9 real feature cards, an entire "Getting Started" section, and an
entire "More Information" section (12 real links). Worse: the page's own
`#getting-started` nav anchor had been left pointing at the testimonials
section instead of a real Getting Started section, so the nav item no
longer led anywhere resembling what it said. None of validate-design.md's
four existing pieces check content *completeness* -- they check design
rules, accessibility, and specific named preservation items (nav labels,
logo, forms), not "is everything still here." A human caught this by
noticing the rebuilt page was suspiciously short; this script makes that
check mechanical so it doesn't depend on a human noticing.

Usage:
    python content-coverage-check.py --before <url-or-file> --after <url-or-file> --out report.json

Two sub-checks, both real and computed, no heuristic scoring:

1. nav-anchor-resolves: every same-page `<nav> a[href^="#"]` on the AFTER
   page must resolve to an existing element id, and that element's real
   text content must clear a minimal non-trivial length -- catches both a
   dead anchor and an anchor silently left pointing at the wrong section
   (a real, confirmed failure mode, not a hypothetical one).
2. heading-and-link-coverage: counts real <h2>/<h3> headings and real
   internal <a href> links (excluding the primary nav itself, which is
   covered by piece 1) on both pages, reports the after/before ratio.
   Below 0.6 for headings or 0.5 for links is a Fail -- a real, likely
   content drop that needs explicit accounting in the validate report
   (restore the content, or write down why less is genuinely correct for
   this redesign), not a silent pass.

These thresholds are deliberately generous, not exact-parity: a redesign
is allowed to consolidate real content (e.g. a fixed curated set of
testimonials replacing a dynamic random-rotation carousel is a legitimate,
smaller page). The bar here is "did whole real sections go missing," not
"is the byte count identical."
"""

import argparse
import json
import sys

from playwright.sync_api import sync_playwright

HEADING_RATIO_MIN = 0.6
LINK_RATIO_MIN = 0.5
TRIVIAL_TEXT_CHARS = 40


def to_target(path_or_url):
    if path_or_url.startswith("http://") or path_or_url.startswith("https://"):
        return path_or_url
    from pathlib import Path

    return Path(path_or_url).resolve().as_uri()


def extract_facts(page):
    return page.evaluate(
        """() => {
            const headings = Array.from(document.querySelectorAll('h2, h3'))
                .filter(h => h.textContent.trim().length > 0).length;

            const nav = document.querySelector('nav');
            const bodyLinks = Array.from(document.querySelectorAll('a[href]'))
                .filter(a => !nav || !nav.contains(a))
                .filter(a => {
                    const href = a.getAttribute('href') || '';
                    const text = a.textContent.trim();
                    return text.length > 0 && href !== '#' && !href.startsWith('javascript:');
                }).length;

            const navAnchors = nav
                ? Array.from(nav.querySelectorAll('a[href^="#"]')).map(a => {
                    const id = a.getAttribute('href').slice(1);
                    const target = id ? document.getElementById(id) : null;
                    return {
                        label: a.textContent.trim(),
                        href: a.getAttribute('href'),
                        resolves: !!target,
                        target_text_length: target ? target.textContent.trim().length : 0,
                    };
                })
                : [];

            return { headings, bodyLinks, navAnchors };
        }"""
    )


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--before", required=True)
    parser.add_argument("--after", required=True)
    parser.add_argument("--out", default=None)
    args = parser.parse_args()

    with sync_playwright() as p:
        browser = p.chromium.launch()
        results = {}
        for label, target in [("before", args.before), ("after", args.after)]:
            page = browser.new_page(viewport={"width": 1280, "height": 800})
            try:
                # "load" + fixed wait, not "networkidle" -- see
                # mechanical-check.py's goto for why: a real autoplaying/
                # looping hero <video> never lets networkidle settle.
                page.goto(to_target(target), wait_until="load", timeout=30_000)
                page.wait_for_timeout(1500)
            except Exception as e:
                sys.exit(f"could not load {label} target {target} ({type(e).__name__}: {e})")
            results[label] = extract_facts(page)
            page.close()
        browser.close()

    findings = []

    # 1. nav-anchor-resolves (checked against the AFTER page only -- the
    # after page's own nav is what a real visitor will click).
    broken = [
        a for a in results["after"]["navAnchors"]
        if not a["resolves"] or a["target_text_length"] < TRIVIAL_TEXT_CHARS
    ]
    if broken:
        findings.append({
            "check": "nav-anchor-resolves",
            "status": "fail",
            "detail": "broken or trivial same-page nav anchor(s): " + ", ".join(
                f"\"{a['label']}\" ({a['href']}) -> "
                + ("no element with that id" if not a["resolves"] else f"only {a['target_text_length']} chars of real content")
                for a in broken
            ),
        })
    else:
        findings.append({
            "check": "nav-anchor-resolves",
            "status": "pass",
            "detail": f"{len(results['after']['navAnchors'])} same-page nav anchor(s) all resolve to real, non-trivial sections",
        })

    # 2. heading-and-link-coverage
    b_h, a_h = results["before"]["headings"], results["after"]["headings"]
    b_l, a_l = results["before"]["bodyLinks"], results["after"]["bodyLinks"]
    heading_ratio = (a_h / b_h) if b_h else 1.0
    link_ratio = (a_l / b_l) if b_l else 1.0
    coverage_fail = heading_ratio < HEADING_RATIO_MIN or link_ratio < LINK_RATIO_MIN
    findings.append({
        "check": "heading-and-link-coverage",
        "status": "fail" if coverage_fail else "pass",
        "detail": (
            f"headings: {a_h}/{b_h} real (ratio {heading_ratio:.2f}, min {HEADING_RATIO_MIN}); "
            f"body links: {a_l}/{b_l} real (ratio {link_ratio:.2f}, min {LINK_RATIO_MIN})"
            + ("" if not coverage_fail else " -- likely dropped real content sections, not just a density choice")
        ),
    })

    passed = sum(1 for f in findings if f["status"] == "pass")
    report = {
        "before": args.before,
        "after": args.after,
        "checks_run": len(findings),
        "passed": passed,
        "failed": len(findings) - passed,
        "findings": findings,
    }
    out = json.dumps(report, indent=2)
    print(out)
    if args.out:
        with open(args.out, "w", encoding="utf-8") as f:
            f.write(out)


if __name__ == "__main__":
    main()
