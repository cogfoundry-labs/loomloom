---
name: validate-design
description: Confirm the rebuilt site is both good design and hasn't broken anything. Composes three reused pieces (webapp-testing, a11y-audit, the active authority's preflight_check) plus the preservation contract, plus a real content-coverage check (confirms Implement didn't silently drop real sections). Any Fail blocks completion: this is the last stage before Gate 3.
---

# validate-design

Validate answers two separate questions, and treats them as separate
questions on purpose: **"is this design good"** and **"did we redesign the
site without breaking it."** A design that passes the first and fails the
second is not done.

## The five pieces (three reused, two new)

1. **`webapp-testing`**: render the rebuilt site against the real dev
   command, click through every route from `discover.json`, confirm: pages
   load, internal/external links resolve, no console errors, responsive
   behavior holds at mobile/tablet/desktop widths (actually resize and
   re-render: don't assume from the desktop screenshot alone).

   **Do this resizing on a dedicated throwaway tab, never on a tab the
   human is looking at (or will be shown at a later gate).**
   `resize_window` applies per-tab and persists, so resizing a gate tab to
   375px for a mobile check strands it there — and a tab whose viewport
   doesn't match the pane renders visibly broken for the human while still
   looking perfect to every agent-side check. See
   `../references/approval-policy.md`'s "Every tab shown to the human MUST
   have an explicit viewport set" for the full confirmed detail; this is
   the stage where that drift actually gets introduced.
2. **`a11y-audit`**: run its deterministic axe-core-based scan against the
   rebuilt routes. This is real, industry-standard accessibility tooling,
   not an LLM eyeballing a screenshot for contrast. Compare its findings
   against the pre-redesign baseline from Analyze: a *new* violation that
   wasn't there before is a Fail, not just "any violation is a Fail."

   **`a11y-audit`'s own `scripts/scan.js` auto-installs its axe-core/Puppeteer
   dependencies on first use, and that auto-installer can fail silently**
   (confirmed on Windows: `Failed to install axe-core: npm install exited
   with null`, no further detail, no crash — the command just doesn't
   produce a report). If this happens, don't treat it as a hard blocker or
   a reason to skip this piece: `cd` into that skill's own `deps/` directory
   (e.g. `~/.claude/skills/a11y-audit/deps`) and run `npm install axe-core
   puppeteer` directly, then re-run `scan.js` — this installs the exact same
   packages the skill's own installer was trying and failing to fetch, just
   via a plain `npm install` instead of whatever spawn options its installer
   uses internally. This is a gap in `a11y-audit` itself, not something
   redesign-lab's own files can fix (it's a separate installed skill), so
   this note exists here instead: don't let a silent dependency-install
   failure in someone else's tooling read as "Validate can't run."
3. **The active authority's `preflight_check`** (`design-authority.md`):
   for the default authority, `design-taste-frontend` §14's mechanical
   Pre-Flight Check: zero em-dashes anywhere visible, no CTA text wrap, one
   accent color used consistently, one corner-radius system, eyebrow count
   within budget, hero fits in 2 lines, duplicate-CTA-intent check, and the
   rest of the ~50-item list. Every item here is checkable from the render
   without a model call: see `../scripts/mechanical-check.py`.
4. **The preservation contract** (`../references/preservation-contract.md`):
   re-check every baseline item captured during Analyze against the rebuilt
   site, scaled to the run's `brand_fidelity` (`design-brief.md`): routes/
   anchors, nav labels, form field names/order, brand logo, legal copy,
   analytics event coverage, accessibility non-regression (covered again
   here specifically because it's *both* a design-quality concern and a
   preservation concern), links resolving, responsive behavior. What counts
   as a Fail here depends on fidelity level: a reworded nav label is a Fail
   at conservative and expected at moderate; legal copy, analytics coverage,
   and brand identity are never negotiable at any level.

   **Brand logo, specifically**: `mechanical-check.py`'s `logo-present`
   finding reports whether a home-linked `<img>`/`<svg>` exists, and does
   have its own hard Fail branch when one exists but never decoded (a
   broken image, not a design choice anyone made). A missing logo image
   alone isn't automatically a Fail on its own, though — a genuine
   text-only wordmark is a valid outcome. The additional check beyond what
   the script itself catches is cross-referencing against
   `assets/manifest.json`'s (written by `capture-assets.py` at Discover,
   not `discover.json` — that file has no `assets` object) `logo` entry:
   if a real logo was captured but `logo-present` now reports false,
   that's a Fail at every fidelity level — the asset existed and Implement
   dropped it, not a deliberate design decision anyone approved.

   **Hero visual, the same way**: `mechanical-check.py`'s
   `hero-visual-present` finding follows the same pattern — a missing hero
   visual alone isn't automatically a Fail (a solid-color/gradient hero can
   be a genuine real design); the cross-check is against
   `assets/manifest.json`'s `hero_visual` entry (again, not `discover.json`)
   — a real photo, background-image, or video captured at Discover that
   Implement silently dropped. If `hero_visual.type` was `video`, also
   confirm a real `<video>` made it into the rebuilt page
   (`implement-design.md`) rather than only the static poster frame
   Direction Slices used — a video that quietly stayed a static image is
   the same class of regression as a logo
   that quietly stayed text.

5. **`content-coverage-check.py`**: confirms Implement didn't silently drop
   real content sections, and that every same-page nav anchor still
   resolves to something real. **Added after a real run (aider.chat,
   industrial-brutalist-ui) passed every check above — 11/11 mechanical,
   0 a11y violations, a clean preservation-contract table — while the
   rebuilt page was quietly missing 3 of 9 real feature cards, an entire
   "Getting Started" section, and an entire "More Information" section (12
   real links), and its own `#getting-started` nav anchor had been left
   pointing at the testimonials section instead of a real Getting Started
   section.** A human caught it by noticing the page read suspiciously
   short; none of pieces 1-4 above check content *completeness* at all —
   they check design rules, accessibility, and specific named preservation
   items, not "is everything still here." Two sub-checks, both real and
   computed, no heuristic scoring:
   - `nav-anchor-resolves`: every same-page nav anchor on the rebuilt page
     must resolve to an existing element id whose real text content clears
     a minimal non-trivial length — this is what would have caught the
     `#getting-started` mislabeling directly.
   - `heading-and-link-coverage`: real `<h2>`/`<h3>` heading count and real
     internal `<a href>` link count (nav excluded), before vs. after,
     reported as a ratio. Below 0.6 for headings or 0.5 for links is a
     Fail — a likely real content drop that needs explicit accounting
     (restore the content, or write down why less is genuinely correct for
     this redesign), not a silent pass. Thresholds are deliberately
     generous, not exact-parity: a redesign consolidating a dynamic
     random-rotation testimonial carousel into a smaller curated set is a
     legitimate, real density choice, not a bug.

   **A real catch this surfaced beyond the missing sections**: the fix for
   the sections above still failed `heading-and-link-coverage` on its own
   first pass (0.40 heading ratio) — not a false positive, but a second,
   real, distinct bug: the original's 9 feature cards are each a real link
   to their own docs page with a real `<h3>` title; the rebuilt rows had
   neither, just plain non-clickable `<div>`s with no heading semantics.
   Fixed by making each row a real `<a href>` to the same real doc URL,
   wrapping a real `<h3>` — which also caught a real ordering drift
   ("Voice-to-code"/"Images & web pages" had been swapped relative to the
   live site's actual order). Don't tune this check's thresholds to make a
   real finding disappear; fix what it found instead, the same way this
   one was.

   **A real limit of this check, found the same run**: `heading-and-link-
   coverage` is one ratio across the *whole* page. The same run passed it
   (0.76) while one specific section (testimonials) had lost 100% of its
   own real attribution links (the original links every testimonial
   author to their real source post; the rebuilt section kept 3 of the
   original 6 testimonials, none linked at all) — the 9 restored feature
   links and 12 More Information links were enough to carry the page-wide
   ratio well past the floor on their own. A passing aggregate ratio is
   not proof every individual section is intact; a human caught this one
   by checking the testimonials section directly. Treat a pass here as "no
   page-wide red flag," not as "every section verified" — a section-level
   spot check is still worth doing on a page with several distinct content
   blocks, this script doesn't replace that.

## Running it

`../scripts/mechanical-check.py` runs piece 3 and the checkable half of 4
locally: no model call, no loomloom spend, genuinely free. It only ever
loads one already-rendered page and checks its DOM/CSS — it doesn't iterate
routes, inspect console output, or resolve links, so it does not cover
piece 1 despite piece 1 being listed first; piece 1 (`webapp-testing`) is
local and free too, but a distinct installed skill actually invoked
separately, same as piece 2. Piece 2 (`a11y-audit`) is also local and free;
it's listed separately because it's a distinct installed skill, not part of
the mechanical-check script itself. Piece 5 (`content-coverage-check.py`)
is also local and free, run against
the same before/after targets Share's `diff-transformations.py` will later
use. Nothing in Validate spends anything: Gates 1-3 are entirely free (rev 6
removed loomloom from Explore Variants' aesthetic-advisory scoring
entirely, see `../SKILL.md`'s "Gates 1-3 are entirely free").

## Pass/fail

Combine all five into `.output/validate-report.md`. **Any single Fail from
any of the five blocks completion**: don't average them into one score and
call it "mostly passing." Fix, re-run this stage, repeat until every piece
passes, then hand off to Gate 3.
