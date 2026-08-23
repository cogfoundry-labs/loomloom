#!/usr/bin/env python3
"""
capture-assets.py — local, free. Finds and downloads real visual assets from
a live URL or local dev server: the logo and the hero's own visual, always,
plus real content photography (product/feature images) matched against the
real copy Direction Slices already reuses from analysis.md. So those stages
embed real assets instead of empty placeholder boxes.

Root cause this fixes, and keeps fixing: no stage in this pipeline used to
capture any real image asset at all. discover.py only records an assets
*directory path* for a local codebase (irrelevant for a live URL target,
which is what every real run against an external site actually is);
generate-directions.md only ever said to reuse real *copy*;
implement-design.md's only asset instruction was image-generate, for
fabricating NEW photography, never for reusing a site's real images.
Confirmed across every real run this session (cogfoundry.ai, scu.edu.au,
bunnings.com.au): zero <img> tags anywhere, ever, including the logo — the
fallback was never a deliberate design decision, just the absence of a
capture step. Two further gaps surfaced after the first fix, both closed
here: (a) the hero's own visual was still missed on a real site because
capturing it depended on a human remembering to pass the hero heading as a
--match label — it now runs unconditionally, like the logo; (b) a genuine,
unmodified photo straight off a real CDN was large enough (197KB) to fail to
open in a different, size-limited preview tool while smaller images worked
fine — every downloaded raster image is now automatically resized/
recompressed under a real byte cap, not left for someone to notice by eye.

Three capture passes, all unconditional except the third:

  1. Logo. Priority order:
       a. An <img>/<svg> inside an <a href="/"> — a site's logo almost
          always links to the home page; the single most reliable signal.
       b. Fall back to <header img>/<nav img>, filtered to plausible logo
          dimensions (rules out 16-28px UI icons).
     A captured inline <svg> with no width/height attribute is stamped with
     its real captured on-page dimensions — without this it can render at
     the browser's default intrinsic SVG size, or collapse to invisible,
     confirmed on a real site's header.

  2. Hero visual. Anchored to the real <h1>, walking up its ancestors
     checking, at every level: a real <img> descendant, a <video>
     descendant (poster attribute downloaded if present, otherwise a real
     screenshot of the currently-playing frame — the video file itself is
     never downloaded; inlining an actual video as base64 in a standalone
     HTML file is impractical), or the container's own CSS
     background-image. Whichever is found at the shallowest ancestor level
     wins.

  3. Content images (runs when --match labels are given). Real product/
     feature photography is frequently lazy-loaded: its real `src` only
     populates once the image scrolls into view, confirmed directly against
     bunnings.com.au — a page-load-only scan finds zero of the four real
     product photos this pipeline's own copy already reuses, because their
     <img> tags sit with an empty/placeholder src until scrolled to. This
     script scrolls the full page first specifically to trigger that lazy
     load before scanning. Each label is matched primarily by structure —
     the image actually inside the same card/container as matching text on
     the page, confirmed necessary against a real site where a card's own
     photo shared no distinctive words with its heading at all — falling
     back to alt-text word overlap (common/brand words excluded, since a
     site's own recurring name in most alt text otherwise causes false
     cross-matches between labels) only for whatever a label's own text
     can't be found for.

Every downloaded raster image (all three passes) is automatically resized
and recompressed if it exceeds MAX_IMAGE_BYTES — this cannot be skipped by
forgetting a step; it happens inside download_image() itself.

Usage:
    python capture-assets.py <url> --out-dir .output/assets
    python capture-assets.py <url> --out-dir .output/assets \\
        --match "Ekodeck 214 x 26mm,Craftright 5 Tier Shelving Unit"
"""

import argparse
import json
import re
import sys
from pathlib import Path
from urllib.parse import urlparse

from playwright.sync_api import sync_playwright

try:
    from PIL import Image
except ImportError:
    Image = None

# Direction Slices/Explore Variants inline every captured image as base64
# directly in the HTML (generate-directions.md) — there's no server, so
# there's no separate network request to make a large original "free".
# Confirmed for real: a genuine, unmodified photo straight off a real site's
# CDN (197KB raw / 262KB as base64) rendered fine via Playwright but failed
# to even open in a different, size-limited preview tool, while otherwise
# identical smaller images (70-100KB raw) worked — a human caught this only
# by comparing file sizes after the fact. Capping every downloaded raster
# image at capture time removes the need to catch this again by hand.
MAX_IMAGE_BYTES = 150_000
MAX_IMAGE_WIDTH = 1000

MIN_LOGO_WIDTH = 32  # confirmed real square icon-mark logo (36x36, shengsuanyun.com)
                      # fell just short of the old 40px floor and was silently
                      # dropped; 32 still clears MAX_ICON_HEIGHT's 28px icons.
MIN_LOGO_HEIGHT = 16
MAX_ICON_HEIGHT = 28  # UI icons (cart, account, search) cluster at <=28px tall
MIN_CONTENT_SIZE = 80  # excludes icons/thumbnails; real content photos are bigger


def find_logo(page):
    origin_home_hrefs = ("/", "")
    candidates = page.evaluate(
        """(homeHrefs) => {
            function describe(el, reason) {
                const rect = el.getBoundingClientRect();
                return {
                    reason,
                    tag: el.tagName,
                    src: el.tagName === 'IMG' ? el.currentSrc || el.src : null,
                    alt: el.alt || null,
                    width: rect.width,
                    height: rect.height,
                    outerHTML: el.tagName === 'svg' ? el.outerHTML : null,
                };
            }
            const out = [];
            for (const a of document.querySelectorAll('a[href]')) {
                let path;
                try { path = new URL(a.href, document.baseURI).pathname; } catch { continue; }
                if (!homeHrefs.includes(path)) continue;
                for (const img of a.querySelectorAll('img')) out.push(describe(img, 'home-link'));
                for (const svg of a.querySelectorAll('svg')) out.push(describe(svg, 'home-link'));
            }
            for (const img of document.querySelectorAll('header img, nav img')) {
                out.push(describe(img, 'header-or-nav'));
            }
            return out;
        }""",
        list(origin_home_hrefs),
    )

    def plausible(c):
        if c["height"] > 0 and c["height"] <= MAX_ICON_HEIGHT and c["reason"] == "header-or-nav":
            return False  # small header/nav icon (cart, account, search), not a logo
        if c["width"] < MIN_LOGO_WIDTH or c["height"] < MIN_LOGO_HEIGHT:
            return False
        return True

    home_link_hits = [c for c in candidates if c["reason"] == "home-link" and plausible(c)]
    if home_link_hits:
        return home_link_hits[0]
    header_hits = [c for c in candidates if c["reason"] == "header-or-nav" and plausible(c)]
    if header_hits:
        return header_hits[0]
    return None


def find_hero_visual(page):
    """Find whatever real visual (image, CSS background-image, or video) is
    actually in the hero, anchored to the real <h1> — never dependent on
    someone remembering to pass the hero's own heading as a --match label.
    That dependency is exactly what caused a real miss: an earlier capture
    only asked for card-section labels, and the hero's own real photo
    (confirmed present via the same DOM-walk technique used here) was
    silently never captured or embedded on a real site until a human
    caught it by eye afterward. This runs unconditionally, every time,
    like the logo pass — never opt-in.

    A hero's visual isn't always an <img>: many real hero sections paint a
    full-bleed CSS background-image on the section itself, or run a
    <video> behind the text. All three are checked at every ancestor level
    while walking up from the h1, not just <img>.

    Ancestor walking alone is not enough to stay "in the hero": confirmed on
    a real page where the h1's 2nd-level ancestor was already <main>, whose
    querySelector('img') matched a product-grid photo hundreds of pixels
    further down the page — a real DOM descendant, but nowhere near the
    hero visually. Every image/video candidate is required to sit within a
    real vertical distance of the h1 itself; a container's own
    background-image is only trusted while the container's own height is
    still hero-sized, not the whole page."""
    return page.evaluate(
        """() => {
            const h1 = document.querySelector('h1');
            if (!h1) return null;
            const h1Rect = h1.getBoundingClientRect();
            const nearHero = (rect) => rect.top < h1Rect.bottom + 700 && rect.bottom > h1Rect.top - 200;

            let container = h1;
            for (let i = 0; i < 6 && container; i++) {
                const containerRect = container.getBoundingClientRect();
                if (containerRect.height > window.innerHeight * 1.5) break;  // no longer hero-sized: a page-spanning wrapper, stop here

                for (const img of container.querySelectorAll('img')) {
                    const rect = img.getBoundingClientRect();
                    if (rect.width >= 80 && rect.height >= 80 && nearHero(rect)) {
                        return { type: 'image', src: img.currentSrc || img.src, alt: img.alt || '', width: rect.width, height: rect.height };
                    }
                }
                const video = container.querySelector('video');
                if (video) {
                    const r = video.getBoundingClientRect();
                    const src = video.currentSrc || (video.querySelector('source') ? video.querySelector('source').src : null);
                    if (src && nearHero(r)) {
                        return { type: 'video', src, poster: video.poster || null, alt: '', rect: { x: r.x, y: r.y, width: r.width, height: r.height } };
                    }
                }
                const bg = getComputedStyle(container).backgroundImage;
                const m = bg && bg.match(/url\\(["']?([^"')]+)["']?\\)/);
                if (m && !m[1].startsWith('data:')) {
                    return { type: 'background-image', src: new URL(m[1], document.baseURI).href, alt: '' };
                }
                container = container.parentElement;
            }
            return null;
        }"""
    )


def trigger_lazy_load(page):
    """Scroll the full page so lazy-loaded <img>s populate a real src.
    Confirmed necessary against bunnings.com.au: at rest after networkidle,
    every real product photo below the fold still has an empty/placeholder
    src, and only resolves to the real CDN URL once scrolled into view."""
    height = page.evaluate("() => document.body.scrollHeight")
    step = 800
    y = 0
    while y < height:
        page.mouse.wheel(0, step)
        page.wait_for_timeout(250)
        y += step
        new_height = page.evaluate("() => document.body.scrollHeight")
        height = max(height, new_height)
    page.evaluate("() => window.scrollTo(0, 0)")
    page.wait_for_timeout(200)


def find_content_images(page, exclude_src):
    candidates = page.evaluate(
        """() => {
            const inChrome = (el) => el.closest('header, nav, footer') !== null;
            return Array.from(document.querySelectorAll('img'))
                .filter(el => !inChrome(el))
                .map(el => {
                    const rect = el.getBoundingClientRect();
                    return {
                        src: el.currentSrc || el.src,
                        alt: el.alt || '',
                        width: rect.width,
                        height: rect.height,
                    };
                });
        }"""
    )
    seen = set()
    out = []
    for c in candidates:
        if c["width"] < MIN_CONTENT_SIZE or c["height"] < MIN_CONTENT_SIZE:
            continue
        if not c["src"] or c["src"] == exclude_src or c["src"] in seen:
            continue
        seen.add(c["src"])
        out.append(c)
    return out


GENERIC_STOPWORDS = {
    "a", "an", "the", "with", "and", "of", "in", "on", "at", "to", "for",
    "your", "our", "is", "are", "app", "image", "photo", "picture",
}
COMMON_WORD_DOC_FREQ_THRESHOLD = 0.35


def words_of(text):
    return set(re.findall(r"[a-z0-9]+", text.lower()))


def find_common_words(candidates):
    """Words that show up in most candidates' alt text are noise for
    matching, not signal — confirmed necessary against a real site: every
    RACV page image's alt text contains the word "racv" (the brand name),
    so naive word-overlap scoring matched labels to the wrong photos purely
    because both happened to share that one universal word, not because
    they were actually about the same thing. Document-frequency across the
    actual candidate pool catches this generically, for any site's own
    recurring words, not just one hardcoded brand name."""
    if len(candidates) < 3:
        return set(GENERIC_STOPWORDS)
    doc_freq = {}
    for c in candidates:
        for w in words_of(c["alt"]):
            doc_freq[w] = doc_freq.get(w, 0) + 1
    threshold = COMMON_WORD_DOC_FREQ_THRESHOLD * len(candidates)
    common = {w for w, n in doc_freq.items() if n > threshold}
    return common | GENERIC_STOPWORDS


def find_image_near_label(page, label, exclude_src):
    """Find the image that's actually in the same card/container as text
    matching this label, by walking up the DOM from the matching text
    element until an <img> descendant turns up. This is the reliable
    signal, not alt-text similarity: confirmed necessary against a real
    site where a card headed "RACV Trades" contains a photo whose alt text
    is "RACV tradesman measuring an outdoor step" — a human reads that as
    an obvious match, but it shares no distinctive words with the label at
    all. The heading and its photo are only provably related by both
    living in the same card, not by what either one says."""
    result = page.evaluate(
        """(label) => {
            const norm = s => s.trim().toLowerCase().replace(/\\s+/g, ' ');
            const target = norm(label);
            const walker = document.createTreeWalker(document.body, NodeFilter.SHOW_ELEMENT);
            const exactMatches = [];
            const substringMatches = [];
            let node;
            while ((node = walker.nextNode())) {
                if (node.children.length > 0) continue;  // leaf elements only: the heading/label itself
                const text = norm(node.textContent || '');
                if (!text) continue;
                if (text === target) exactMatches.push(node);
                else if (text.includes(target) || target.includes(text)) substringMatches.push(node);
            }
            // Stop widening once the container itself spans too much of the
            // page (same cap find_hero_visual uses), and never treat
            // <body>/<html> as "the same card" regardless of their own
            // height on a short page — 6 levels up from a leaf routinely
            // *is* <body> already, whose own rect can be well under the
            // 1.5x-viewport cap even though it now contains every image on
            // the page. A container this large isn't a real proximity
            // match; better to report none (and leave the placeholder, per
            // this script's own "don't invent one" rule) than confidently
            // return an unrelated photo.
            const imageNear = (match) => {
                let container = match;
                for (let i = 0; i < 6 && container; i++) {
                    if (container === document.body || container === document.documentElement) break;
                    const containerRect = container.getBoundingClientRect();
                    if (containerRect.height > window.innerHeight * 1.5) break;
                    const img = container.querySelector('img');
                    if (img) {
                        const rect = img.getBoundingClientRect();
                        return { src: img.currentSrc || img.src, alt: img.alt || '', width: rect.width, height: rect.height };
                    }
                    container = container.parentElement;
                }
                return null;
            };
            // The same label text can appear more than once (a nav/menu
            // duplicate of a card heading, confirmed on a real site) — don't
            // stop at the first textual hit if it has no image nearby, keep
            // checking every match of the *same specificity* in document
            // order until one does.
            //
            // Substring matches are only tried when there is no exact match
            // at all — confirmed necessary for real against a fixture site
            // where the label "Widgets" (an exact match on a card heading
            // with no photo of its own) also reads as a substring of the
            // page's own unrelated hero headline ("Hello World Widgets
            // Makes Simple Tools..."), which sits earlier in document
            // order and does have an image nearby (the hero photo). Falling
            // through to that substring hit just because the exact match's
            // own card genuinely has no photo would silently return the
            // wrong image; the correct behavior when a label's real heading
            // has no nearby photo is no match, not a looser guess.
            if (exactMatches.length > 0) {
                for (const match of exactMatches) {
                    const hit = imageNear(match);
                    if (hit) return hit;
                }
                return null;
            }
            for (const match of substringMatches) {
                const hit = imageNear(match);
                if (hit) return hit;
            }
            return null;
        }""",
        label,
    )
    if result and result["src"] and result["src"] != exclude_src:
        return result
    return None


def score_match(label, alt, common_words):
    label_words = words_of(label) - common_words
    alt_words = words_of(alt) - common_words
    if not label_words or not alt_words:
        return 0
    return len(label_words & alt_words)


def slugify(text):
    slug = re.sub(r"[^a-z0-9]+", "-", text.lower()).strip("-")
    return slug[:60] or "image"


def ensure_svg_dimensions(svg_markup, width, height):
    """A captured inline <svg> with no width/height attribute (common: many
    sites size their logo purely via a CSS class) silently renders at the
    browser's default intrinsic SVG size (300x150) or collapses, depending
    on context — confirmed against a real site: it rendered as effectively
    invisible in a compact header with no explicit sizing anywhere. Stamp
    the real, captured on-page dimensions onto the root <svg> tag so every
    consumer gets a correctly-sized logo without having to remember to add
    this themselves."""
    if re.search(r"<svg[^>]*\bwidth=", svg_markup):
        return svg_markup
    return re.sub(
        r"<svg\b",
        f'<svg width="{round(width)}" height="{round(height)}" style="display:block"',
        svg_markup,
        count=1,
    )


def guess_extension(url, content_type):
    if content_type:
        if "svg" in content_type:
            return ".svg"
        if "png" in content_type:
            return ".png"
        if "jpeg" in content_type or "jpg" in content_type:
            return ".jpg"
        if "webp" in content_type:
            return ".webp"
    path = urlparse(url).path
    m = re.search(r"\.(svg|png|jpe?g|webp)$", path, re.IGNORECASE)
    return f".{m.group(1).lower()}" if m else ".png"


def shrink_if_needed(path):
    """Resize/recompress a downloaded raster image if it's larger than a
    real deliverable HTML file should carry inline. Skipped for .svg
    (vector, no benefit) and anything already under the cap. Real photos
    straight off a CDN commonly exceed this; a hero photo at 1600x900 came
    back at 197KB raw before this existed, and nothing caught it until a
    human noticed a broken image in one specific viewer afterward."""
    if Image is None or path.suffix.lower() == ".svg":
        return
    if path.stat().st_size <= MAX_IMAGE_BYTES:
        return
    try:
        img = Image.open(path)
        img.load()
    except Exception:
        return  # not a real raster image (or unreadable) -- leave it alone, don't guess

    has_alpha = img.mode in ("RGBA", "LA") and img.getchannel("A").getextrema()[0] < 255
    if img.width > MAX_IMAGE_WIDTH:
        ratio = MAX_IMAGE_WIDTH / img.width
        img = img.resize((MAX_IMAGE_WIDTH, round(img.height * ratio)), Image.LANCZOS)

    if has_alpha:
        img.save(path, format="PNG", optimize=True)
        return  # PNG quality isn't a dial the way JPEG's is; resizing is the only lever here

    rgb = img.convert("RGB")
    for quality in (82, 72, 62, 50):
        rgb.save(path, format="JPEG", quality=quality, optimize=True)
        if path.stat().st_size <= MAX_IMAGE_BYTES:
            return
    # Still over the cap at the lowest quality tried: keep the last save
    # rather than looping forever chasing an arbitrary byte count.


def download_image(page, url, out_path):
    resp = page.request.get(url, timeout=15_000)
    content_type = resp.headers.get("content-type", "")
    ext = guess_extension(url, content_type)
    final_path = out_path.with_suffix(ext)
    final_path.write_bytes(resp.body())
    shrink_if_needed(final_path)
    return final_path


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("target", help="URL to inspect (http://... or https://...)")
    parser.add_argument("--out-dir", default=".output/assets")
    parser.add_argument(
        "--match",
        action="append",
        default=None,
        help="A real label (product/feature name, from analysis.md's real copy) to find a real "
        "content photo for. Repeat --match once per label (--match \"a\" --match \"b\"); a single "
        "--match value may also hold several comma-separated labels. Omit to skip content-image "
        "capture. Confirmed the hard way: passing --match repeatedly used to silently keep only "
        "the last occurrence (argparse's default single-value behavior for a repeated flag), which "
        "looked exactly like a matching-algorithm failure — only the last label ever got captured. "
        "action=\"append\" is what actually fixes that, not a smarter matcher.",
    )
    args = parser.parse_args()

    if not args.target.startswith("http"):
        sys.exit(
            "capture-assets.py takes a URL (a live site or a local dev server), "
            "not a file path — point it at the same target render-and-screenshot.py uses."
        )

    out_dir = Path(args.out_dir)
    out_dir.mkdir(parents=True, exist_ok=True)
    manifest = {"logo": None, "hero_visual": None, "content_images": []}

    # Merge with whatever a prior run already captured, never overwrite it
    # wholesale. Confirmed necessary the hard way: a first run against a real
    # site captured most content images but missed a couple of labels
    # (`--match`'s own greedy assignment doesn't always place every label in
    # one pass); a second run with just the missed labels silently discarded
    # the first run's content_images entirely, because this used to always
    # write a brand-new manifest. logo/hero_visual are overwritten by a fresh
    # non-null find (they're unconditional captures, so a real run always
    # either reconfirms or corrects them); content_images are merged by
    # label, so an earlier label's real photo survives a later, narrower run.
    existing_manifest_path = out_dir / "manifest.json"
    if existing_manifest_path.exists():
        try:
            existing = json.loads(existing_manifest_path.read_text(encoding="utf-8"))
            manifest["logo"] = existing.get("logo")
            manifest["hero_visual"] = existing.get("hero_visual")
            manifest["content_images"] = existing.get("content_images", [])
        except (json.JSONDecodeError, OSError):
            pass  # a corrupt/unreadable prior manifest is not worth failing the run over

    with sync_playwright() as p:
        browser = p.chromium.launch()
        try:
            page = browser.new_page(viewport={"width": 1280, "height": 800})
            try:
                page.goto(args.target, wait_until="networkidle", timeout=30_000)
            except Exception as e:
                sys.exit(f"could not load {args.target} within 30s ({type(e).__name__}: {e})")

            logo = find_logo(page)
            logo_src = None
            if logo is None:
                print(f"no plausible logo found on {args.target} — leaving manifest.logo empty")
            elif logo["tag"] == "svg" and logo["outerHTML"]:
                logo_path = out_dir / "logo.svg"
                svg_markup = ensure_svg_dimensions(logo["outerHTML"], logo["width"], logo["height"])
                logo_path.write_text(svg_markup, encoding="utf-8")
                manifest["logo"] = {
                    "path": str(logo_path),
                    "source": "inline-svg",
                    "alt": logo["alt"],
                    "found_via": logo["reason"],
                }
                print(f"captured inline SVG logo -> {logo_path}")
            elif logo["src"]:
                logo_src = logo["src"]
                try:
                    logo_path = download_image(page, logo["src"], out_dir / "logo")
                    manifest["logo"] = {
                        "path": str(logo_path),
                        "source_url": logo["src"],
                        "alt": logo["alt"],
                        "found_via": logo["reason"],
                        "width": logo["width"],
                        "height": logo["height"],
                    }
                    print(f"captured logo ({logo['width']:.0f}x{logo['height']:.0f}) -> {logo_path}")
                except Exception as e:
                    print(f"found a logo candidate but could not download it ({type(e).__name__}: {e})")

            # Hero visual: unconditional, like the logo — never gated behind
            # remembering to pass the hero's own heading as a --match label.
            # That dependency is exactly what let a real hero photo go
            # uncaptured on a real site until a human noticed it missing
            # after the fact.
            hero = find_hero_visual(page)
            if hero is None:
                print("no hero visual found near the h1 (no <img>, <video>, or CSS background-image) -- leaving manifest.hero_visual empty")
            elif hero["type"] == "video":
                if hero.get("poster"):
                    try:
                        poster_path = download_image(page, hero["poster"], out_dir / "hero-video-poster")
                        manifest["hero_visual"] = {
                            "type": "video", "video_src_url": hero["src"], "poster_path": str(poster_path),
                            "note": "video file itself not downloaded (impractical to inline); poster frame captured instead",
                        }
                        print(f"hero is a <video> -- captured its poster frame -> {poster_path}")
                    except Exception as e:
                        print(f"hero <video> found but its poster could not be downloaded ({type(e).__name__}: {e})")
                else:
                    try:
                        frame_path = out_dir / "hero-video-frame.jpg"
                        r = hero["rect"]
                        png_bytes = page.screenshot(clip={"x": r["x"], "y": r["y"], "width": max(r["width"], 1), "height": max(r["height"], 1)})
                        frame_path.write_bytes(png_bytes)
                        shrink_if_needed(frame_path)
                        manifest["hero_visual"] = {
                            "type": "video", "video_src_url": hero["src"], "poster_path": str(frame_path),
                            "note": "video file itself not downloaded (impractical to inline); no <video poster> attribute existed, so this is a real screenshot of the currently-playing frame, not an official poster",
                        }
                        print(f"hero is a <video> with no poster -- captured a real playing frame instead -> {frame_path}")
                    except Exception as e:
                        print(f"hero <video> found but a frame could not be captured ({type(e).__name__}: {e})")
            else:
                try:
                    hero_path = download_image(page, hero["src"], out_dir / "hero-visual")
                    manifest["hero_visual"] = {
                        "type": hero["type"], "path": str(hero_path), "source_url": hero["src"], "alt": hero.get("alt", ""),
                    }
                    print(f"captured hero visual ({hero['type']}) -> {hero_path}")
                except Exception as e:
                    print(f"found a hero visual candidate but could not download it ({type(e).__name__}: {e})")

            if args.match:
                labels = []
                for value in args.match:
                    labels.extend(label.strip() for label in value.split(",") if label.strip())
                trigger_lazy_load(page)

                # Pass 1, primary: structural. Find the label's own text on
                # the page and take the image inside that same card/container
                # — reliable regardless of what the image's alt text says.
                assigned = {}
                used_src = set()
                remaining_labels = []
                for label in labels:
                    hit = find_image_near_label(page, label, exclude_src=logo_src)
                    if hit:
                        assigned[label] = hit
                        used_src.add(hit["src"])
                    else:
                        remaining_labels.append(label)

                # Pass 2, fallback: alt-text word overlap, only for labels
                # that don't literally appear as text anywhere on the page
                # (e.g. a product name transcribed slightly differently from
                # the page's own copy). Global greedy assignment across all
                # remaining (label, candidate) pairs at once, not
                # per-label-in-order — an earlier label grabbing a mediocre
                # match would otherwise starve a later label's actual best
                # photo. Confirmed both problems for real against RACV: its
                # own brand name recurring in most alt text left enough
                # residual word-overlap that per-label-in-order greedy
                # assigned the real "RACV Trades" photo to the wrong label.
                if remaining_labels:
                    candidates = find_content_images(page, exclude_src=logo_src)
                    candidates = [c for c in candidates if c["src"] not in used_src]
                    common = find_common_words(candidates)
                    pairs = []
                    for label in remaining_labels:
                        for c in candidates:
                            score = score_match(label, c["alt"], common)
                            if score > 0:
                                pairs.append((score, label, c))
                    pairs.sort(key=lambda p: p[0], reverse=True)
                    used_labels = set()
                    for score, label, c in pairs:
                        if label in used_labels or c["src"] in used_src:
                            continue
                        assigned[label] = c
                        used_labels.add(label)
                        used_src.add(c["src"])

                for label in labels:
                    best = assigned.get(label)
                    if best is None:
                        print(f"no content image matched '{label}' (by structure or alt text) -- leave as placeholder, don't invent one")
                        continue
                    try:
                        img_path = download_image(page, best["src"], out_dir / f"content-{slugify(label)}")
                        new_entry = {
                            "label": label,
                            "path": str(img_path),
                            "source_url": best["src"],
                            "alt": best["alt"],
                            "width": best["width"],
                            "height": best["height"],
                        }
                        # Merge by label: this run's find replaces a same-label
                        # entry from a prior merged-in manifest, rather than
                        # piling up a duplicate.
                        manifest["content_images"] = [
                            c for c in manifest["content_images"] if c.get("label") != label
                        ]
                        manifest["content_images"].append(new_entry)
                        print(f"captured content image for '{label}' -> {img_path}")
                    except Exception as e:
                        print(f"matched a content image for '{label}' but could not download it ({type(e).__name__}: {e})")
        finally:
            browser.close()

    manifest_path = out_dir / "manifest.json"
    manifest_path.write_text(json.dumps(manifest, indent=2), encoding="utf-8")
    print(f"wrote {manifest_path}")
    sys.exit(0 if (manifest["logo"] or manifest["hero_visual"] or manifest["content_images"]) else 1)


if __name__ == "__main__":
    main()
