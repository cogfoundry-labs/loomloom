#!/usr/bin/env python3
"""
render-and-screenshot.py — local, free. Renders a URL or local file at one or
more viewport widths and saves a screenshot per width. Used by Direction
Slices, Explore Variants, and Validate — the same render step, three stages.

Usage:
    python render-and-screenshot.py <url-or-file> --out-dir .output/shots/x
"""

import argparse
import sys
from pathlib import Path

from playwright.sync_api import sync_playwright

VIEWPORTS = {
    "mobile": (375, 812),
    "tablet": (768, 1024),
    "desktop": (1280, 800),
}


def prepare_for_full_page_capture(page):
    """Scroll the full page so scroll-triggered reveal animations (fade/slide-in
    on enter, e.g. AOS-style libraries) fire before the screenshot, and freeze
    all CSS transitions/animations so nothing is caught mid-fade.

    Confirmed necessary against a real site (ShengSuanYun's LoomLoom page):
    without this, page.screenshot(full_page=True) resizes the viewport to the
    full document height and captures in one shot, which never fires the
    `scroll` events those reveal animations listen for. Every section below
    the first fold was still at its pre-animation opacity:0 state, producing
    a screenshot that was ~80% blank white despite real content being there.
    Same root cause as trigger_lazy_load() in capture-assets.py, applied here
    too since this script builds the before/after case-study screenshots."""
    # page.add_style_tag() injects a real <style> element, which a strict
    # Content-Security-Policy (style-src without 'unsafe-inline') blocks
    # outright -- confirmed on pypi.org, whose CSP rejected it and crashed
    # this function entirely before a single scroll happened. Setting the
    # same properties via the CSSOM (element.style.setProperty) instead is
    # script execution through Playwright's CDP evaluate, not a stylesheet
    # resource, so it isn't subject to style-src at all -- same effect,
    # doesn't require the target site's CSP to cooperate.
    page.evaluate(
        """() => {
            const css = 'transition-duration:0s!important;transition-delay:0s!important;'
                + 'animation-duration:0s!important;animation-delay:0s!important;';
            document.querySelectorAll('*').forEach(el => el.style.cssText += css);
        }"""
    )
    height = page.evaluate("() => document.body.scrollHeight")
    step = 800
    y = 0
    while y < height:
        page.mouse.wheel(0, step)
        page.wait_for_timeout(120)
        y += step
        new_height = page.evaluate("() => document.body.scrollHeight")
        height = max(height, new_height)
    page.evaluate("() => window.scrollTo(0, 0)")
    page.wait_for_timeout(100)


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("target")
    parser.add_argument("--out-dir", required=True)
    parser.add_argument("--widths", default="mobile,tablet,desktop")
    args = parser.parse_args()

    target = args.target
    if not target.startswith("http") and not target.startswith("file://"):
        target = Path(target).resolve().as_uri()

    names = [n.strip() for n in args.widths.split(",")]
    unknown = [n for n in names if n not in VIEWPORTS]
    if unknown:
        sys.exit(f"unknown viewport name(s) {unknown}, expected one of {sorted(VIEWPORTS)}")

    out_dir = Path(args.out_dir)
    out_dir.mkdir(parents=True, exist_ok=True)

    with sync_playwright() as p:
        browser = p.chromium.launch()
        try:
            for name in names:
                w, h = VIEWPORTS[name]
                page = browser.new_page(viewport={"width": w, "height": h})
                try:
                    page.goto(target, wait_until="networkidle", timeout=30_000)
                except Exception as e:
                    # A slow/unfamiliar live site can keep the network busy
                    # (chat widgets, analytics polling) so "networkidle" never
                    # settles. Report which viewport failed and keep going
                    # rather than losing every other width to one bad load.
                    print(f"{name}: skipped, could not load within 30s ({type(e).__name__}: {e})")
                    page.close()
                    continue
                prepare_for_full_page_capture(page)
                shot_path = out_dir / f"{name}.png"
                page.screenshot(path=str(shot_path), full_page=True)
                print(f"{name}: {shot_path}")
                page.close()
        finally:
            browser.close()


if __name__ == "__main__":
    main()
