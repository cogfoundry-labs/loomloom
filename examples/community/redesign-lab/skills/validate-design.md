---
name: validate-design
description: Confirm the rebuilt site is both good design and hasn't broken anything. Composes three reused pieces (webapp-testing, a11y-audit, the active authority's preflight_check) plus the preservation contract. Any Fail blocks completion: this is the last stage before Gate 3.
---

# validate-design

Validate answers two separate questions, and treats them as separate
questions on purpose: **"is this design good"** and **"did we redesign the
site without breaking it."** A design that passes the first and fails the
second is not done.

## The four pieces (three reused, one new)

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
   finding reports whether a home-linked `<img>`/`<svg>` exists, but never
   fails on its own (a genuine text-only wordmark is a valid outcome). The
   real check is cross-referencing it against `discover.json`'s
   `assets.logo` entry (written by `capture-assets.py` at Discover): if a
   real logo was captured but `logo-present` now reports false, that's a
   Fail at every fidelity level — the asset existed and Implement dropped
   it, not a deliberate design decision anyone approved.

   **Hero visual, the same way**: `mechanical-check.py`'s
   `hero-visual-present` finding is the same pattern — never a Fail on its
   own (a solid-color/gradient hero can be a genuine real design), only a
   Fail when it contradicts `discover.json`'s `assets.hero_visual` (a real
   photo, background-image, or video captured at Discover that Implement
   silently dropped). If `hero_visual.type` was `video`, also confirm a real
   `<video>` made it into the rebuilt page (`implement-design.md`) rather
   than only the static poster frame Direction Slices used — a video that
   quietly stayed a static image is the same class of regression as a logo
   that quietly stayed text.

## Running it

`../scripts/mechanical-check.py` runs pieces 1, 3, and the checkable half of
4 locally: no model call, no loomloom spend, genuinely free. Piece 2
(`a11y-audit`) is also local and free; it's listed separately because it's a
distinct installed skill, not part of the mechanical-check script itself.
Nothing in Validate spends anything: Gates 1-3 are entirely free (rev 6
removed loomloom from Explore Variants' aesthetic-advisory scoring
entirely, see `../SKILL.md`'s "Gates 1-3 are entirely free").

## Pass/fail

Combine all four into `.output/validate-report.md`. **Any single Fail from
any of the four blocks completion**: don't average them into one score and
call it "mostly passing." Fix, re-run this stage, repeat until every piece
passes, then hand off to Gate 3.
