---
name: generate-directions
description: Produce 3 real-code slices by default for the human to choose between at Gate 1 (a current-fixed baseline plus 2 structurally distinct exploratory directions, one carefully-chosen colorway each, hero plus enough real sections to judge each, always rendered full-page). A 4th direction and additional colorways are available on request, before building or after seeing the first 3. Uses the active design authority's direction_variants: never generated images, never one design read improvising twice.
---

# generate-directions

The stage the whole product's trust depends on — and, confirmed across
several real runs, the single biggest time cost in the whole pipeline. Rev 7
cuts the default pool from 4 slices (each with 2-3 colorways) to 3 slices
(1 colorway each), specifically to bring that cost down, while keeping every
slice just as real. Six rules, none optional:

1. **Real code, not a generated image.** Each direction is real, rendered
   code, screenshotted locally. Image models cannot reliably render legible,
   aligned UI text, and a gap between a pretty mockup and the real
   implementation breaks trust at the first decision point: this is not a
   cost optimization, it's a trust requirement.
2. **More than a hero, and always screenshotted full-page.** A hero section
   alone is not enough for a human to actually judge a direction: typography
   and color read differently in a dense content section than in a hero, and
   component patterns (cards, grids, accordions) don't exist in a hero at
   all. Build the hero plus at least 2 more real content sections (reuse the
   project's actual copy from `.output/analysis.md`, never placeholder text)
   so the variant's rules get exercised against more than one kind of
   content. Then render and screenshot the **whole page, top to bottom**
   (`render-and-screenshot.py` already does this via `full_page=True`; the
   fix here is never truncating to what one script call happens to name, not
   the flag itself). A screenshot that stops at the fold is functionally the
   same failure as a hero-only slice: the human is choosing blind on
   everything below it. This was learned the hard way: the first pass at this
   stage built hero-only slices and it genuinely wasn't enough to choose from.
3. **Structurally distinct, not improvised.** Pick 2 from the active
   authority's `direction_variants` (`../references/design-authority.md`):
   for the default authority, that's 8 options today (`minimalist-ui`,
   `industrial-brutalist-ui`, `high-end-visual-design`,
   `design-taste-frontend` base, `ui-craft-editorial`,
   `ui-craft-dense-dashboard`, `product-proof-saas`,
   `operational-enterprise-ai`). These are genuinely different,
   mutually-exclusive constraint sets: verified directly by reading each
   one's SKILL.md: not the same generic skill asked to riff on itself twice.
4. **Each direction ships one carefully-chosen colorway, not an arbitrary
   pick.** Rev 7: this used to mean building 2-3 colorways per direction
   upfront. That guaranteed no direction got eliminated over an unlucky
   palette, but it was also a large share of the real time cost of this
   stage, for value the human often didn't ask for on a first pass. The
   colorway shown now is singular but deliberate: read `.output/analysis.md`
   and pick whichever real archetype/palette option the direction offers
   (see "Choosing the one colorway" below) actually fits this site's content
   and register, not whichever is listed first. Additional colorways for a
   direction the human likes are built on request, after Gate 1 — see
   "Colorways on request" below.
5. **Actually responsive, not just checked at three widths.** Every slice
   needs a real `<meta name="viewport" content="width=device-width,
   initial-scale=1">` tag in `<head>`, plus real `@media` breakpoints that
   change the layout (collapse multi-column grids to one column, stack a
   side-by-side hero, let the nav wrap) at mobile and tablet widths — not
   just fluid units (`clamp()`, `%`, `flex-wrap`) that merely avoid breaking.
   Confirmed missing the hard way on a real run: every template built this
   way used CSS Grid with fixed column ratios and zero `@media` rules, and
   `mechanical-check.py --viewport 375x812` still reported 9/9 on every one
   of them, because none of its checks test whether the layout actually
   *changes* between viewports — they just re-run the same content/contrast
   checks at a narrower Playwright-forced width. The deeper reason the bug
   stayed invisible: without the viewport meta tag, a *real* mobile browser
   (as opposed to Playwright's direct `viewport=` override, which sets the
   layout viewport directly regardless of the tag) renders at a fake wide
   desktop-width layout and scales it down, so the `@media` rules never even
   evaluate — this is why the tag is listed here as its own requirement, not
   assumed to follow automatically from writing breakpoints. `mechanical-
   check.py` now has two dedicated checks for this (`viewport-meta-present`,
   `responsive-layout-collapses`); a real Fail on either blocks the slice the
   same as any other check, it doesn't just get a note.
6. **Offer a 4th direction before building, don't just build it.** Ask once,
   before Direction Slices runs, in the same message that narrates the 2
   picks: "want a 3rd exploratory direction? It's a real, honest trade —
   roughly 50% more build/screenshot/check time before you see anything."
   Default (no answer, or "no"/"these are fine") is 2. If the human opts in,
   pick the 3rd the same way as the first 2 (genome distance, ivory-serif
   exclusion), just against a 3-candidate pool instead of 2 — see
   `../references/style-genome.md`. This is never the only chance to ask for
   more: after Gate 1 shows the real slices, the human can always ask for
   additional directions the same way (see "Handoff" below); the upfront
   offer just avoids a human who already knows they want more waiting through
   a first pass to ask for it.

**`<main>` is a real landmark, not a "centered container" utility class.**
Confirmed the hard way against a real `a11y-audit` scan (validate-design.md
piece 2), the first time that piece was actually run end-to-end this
session: three separate slices independently reused `<main>` as a generic
`max-width` wrapper for the hero and for every `<section>`'s inner content,
producing a real, deterministic axe-core violation (`landmark-no-duplicate-
main`, `landmark-unique`) — a page must have *at most one* `<main>`,
wrapping all of its actual primary content, not one per section. Use a
plain `.wrap{max-width:...;margin:0 auto;}` class (or whatever the site's
own real convention already is) for the centered-container styling, and
reserve the one real `<main>` for the genuine landmark, opened right after
`<header>` and closed right before `<footer>`.

## The default: 3 slices, not 4

Gate 1 shows **3 slices by default**: the 2 exploratory directions above (a
4th only if the human opted in per rule 6), plus one baseline slice that
keeps the project's own existing design (real colors, typography, and
layout from `discover.json`/the live site, not a skill's rules) and applies
only the concrete fixes already found: the mechanical-check findings from
scanning the live/current site (real em-dashes, a hero that doesn't fit 2
lines, a nav that clips or wraps badly, low-contrast text) and the
Fix-phase items from `redesign-existing-projects`' Scan → Diagnose pass
captured in `analysis.md`. Nothing else changes: no new palette, no new
component patterns, no new layout logic.

This is a **should**, not conditional on the brief: build it every time,
because it answers a question the exploratory directions can't:
"how much of what's wrong here is actually a full redesign, versus a punch
list of real defects that could be fixed without changing the design at
all?" A human comparing bold new directions against nothing has no anchor
for how much visual risk they're actually choosing to take on; comparing
them against "the same site, but the real problems fixed" makes that
concrete and mechanically verifiable, not a guess.

Build and score it exactly like the exploratory directions (hero plus 2 more
real sections, full-page screenshot, `mechanical-check.py` run against it
same as any other slice) so its score is directly comparable, not asserted.
Label it clearly at Gate 1 (e.g. `current-fixed`) so it doesn't get mistaken
for an aesthetic option: it's a baseline, not a direction.

## Selecting which 2 (or 3, if the human opted in per rule 6)

Don't default to the same picks every time. Read `.output/discover.json` and
`.output/analysis.md` (or `taste-profile.yaml` in reference-mode) and use the
active authority's dial-inference guidance (`design-taste-frontend` §1.A),
together with `../references/style-genome.md`'s scored distance between
candidates, to pick whichever of the available variants diverge most from:
- the site's current state (don't propose a direction that looks like what's
  already there), and
- each other (don't propose two variants that would plausibly converge on
  similar output for this specific brief).

The genome makes "diverge most" a real calculation, not just a feeling: at
the default pool of 2, that's the pair whose 3 real distances (each vs.
current, and against each other) are all genuinely large; at 3 (opted in),
maximize minimum pairwise distance across all 3 (see `style-genome.md`'s
"Using it in Selecting which exploratory directions" for the exact method
and its threshold). Treat it as a check on the selection, not a substitute
for reading the brief: a brief that specifically calls for something the
genome would rank as "close" to another pick can still override the
distance calculation, but that should be a deliberate call, not an accident
from skipping the check.

**Divergence isn't only color.** Five of the eight current variants
(`minimalist-ui`, `industrial-brutalist-ui`, `high-end-visual-design`,
`design-taste-frontend` base, `ui-craft-editorial`) are **aesthetic/vibe**
families: they govern typography, color, shadows, borders, but not
information architecture: a hero-plus-card-grid skeleton painted different
colors still reads as "the same site in different colors" (this happened in
practice: see the redesign of cogfoundry.ai, where 3 aesthetic-only slices
all converged on the same structure). The other three (`ui-craft-dense-dashboard`,
`product-proof-saas`, `operational-enterprise-ai`) are **structural/IA**
families: each changes what the page is organized around (density and
operator workflow; a real product-demo mechanism as evidence; permission/audit/
rollback state), not just paint. When the brief's content genuinely supports
one of these reads (not every brief does: a dense-dashboard read needs real
dense data, a product-proof read needs a real workflow to demonstrate, an
operational-enterprise read needs real permissions/audit/rollback concerns),
prefer including one structural/IA variant among the picks, specifically so
the divergence isn't purely chromatic. This is a should, not a hard rule:
don't force a structural read onto a brief with nothing to justify it, and
don't include more than one structural variant just because more are
available now: two structural reads competing with each other wastes a slot
that could have covered a genuinely different axis. At the default pool of
2, this means at most one of the two picks is structural/IA — the other
should be aesthetic/vibe, so the human still sees a real palette/typography
contrast, not two structural reads with no aesthetic-only anchor at all.

**A confirmed convergence cluster overrides the distance math.** A real test
(hero-only thumbnails of all 8 variants, no colorways, built against a live
redesign target) found that `minimalist-ui`, `design-taste-frontend` (base),
and `ui-craft-editorial` read as the same direction shown three times when
reduced to hero-only, despite each being a genuinely different skill and
despite nonzero genome distance between their rows. See
`../references/style-genome.md`'s "Known convergence: the ivory-serif
cluster" for the details. The rule from that finding: never pick more than
one of these three among the exploratory directions chosen, full stop,
whether or not the distance calculation above would otherwise have allowed
it.

This same test is also why Direction Slices builds hero-plus-2-sections
rather than hero-only (rule 2, above): 2 of the 3 structural/IA variants
(`ui-craft-dense-dashboard`, `operational-enterprise-ai`) only read as
distinct once a real content section is visible — their hero alone is close
to indistinguishable from a generic dark SaaS hero. Building hero-only for
speed would have hidden the exact signal those two variants exist to show.

If the active authority has no `direction_variants` filler (documented in
`design-authority.md` for `baoyu-design` today), say so explicitly to the
user before proceeding: one design read improvising twice is a real but
weaker fallback, not a silent substitute for the guarantee.

## Choosing the one colorway

Picking a structurally-distinct skill doesn't guarantee an eye-catching
result: several variant skills (notably `high-end-visual-design`, whose own
§3.A lists 3 internal vibe archetypes: Ethereal Glass / Editorial Luxury /
Soft Structuralism) leave real room to choose within the skill, and
defaulting to the same safe pick every run (or independently landing on
similar tones by not thinking about it) wastes the one colorway slot each
direction now gets. Before writing any code, for each of the 2 (or 3) chosen
skills:

1. Note whether it has internal archetype/vibe choices (read its own
   SKILL.md section on this) or a single fixed palette
   (`ui-craft-editorial`, `ui-craft-dense-dashboard`, `product-proof-saas`,
   and `operational-enterprise-ai` are each fixed; `high-end-visual-design`
   is not).
2. If it has real internal choices, pick the one archetype that best fits
   *this* site's real content and register per `analysis.md` — not
   whichever is listed first in the skill's own SKILL.md. A selective
   academic school, say, reads differently against "Ethereal Glass" (dark,
   SaaS-flavored) than "Editorial Luxury" (warm, gravitas-flavored); pick
   the one the real brief actually supports.
3. Write down each direction's resulting base canvas tone in one line (e.g.
   dark / warm-light / cool-light) before building anything. If 2 (or all 3)
   land in the same bucket, go back and pick a different archetype for
   whichever skill allows it, or reconsider which skills were chosen — don't
   discover the convergence after screenshotting.

## Colorways on request, not built upfront

A direction's *other* real colorways (its remaining internal archetypes, or
whatever its own rules leave open if it has a fixed palette) are real and
cheap to build — swapping CSS custom-property token values and re-rendering
the identical markup, no new layout or copy — but rev 7 stopped building
them by default specifically to cut this stage's time cost. Build one only
when asked, either as part of the upfront narration ("want to see
`industrial-brutalist-ui` in a second palette before choosing? Say so") or
after Gate 1 ("show me `high-end-visual-design` in its other 2 colorways").
When one is requested:

1. Pick the next real option per "Choosing the one colorway" above's method
   (an unused internal archetype, or the skill's own stated open slot for a
   fixed-palette skill) — never invent a palette a color-locked skill's
   SKILL.md doesn't sanction.
2. Render and screenshot the same structural build with the new colorway
   (`render-and-screenshot.py`, one token file swapped in). Full-page, same
   as every other slice.
3. Save it as `.output/directions/<variant-name>/colorway-<n>/`, and open it
   the same progressive, one-tab way as every other slice (see "Handoff"
   below) — a requested colorway is a real slice, not a lesser follow-up.

## Building each slice

For each of the 2 (or 3) chosen variants:

1. Load the variant skill (e.g. `industrial-brutalist-ui`) alongside the base
   authority skill: the variant is additive to the base Pre-Flight Check and
   em-dash ban, not a replacement for them. If it offers internal
   archetype/vibe choices, the one committed to in the palette check above.
2. Write the hero plus at least 2 more real content sections (a mix of
   section types where possible: e.g. one text-heavy section, one
   card/grid section) using the project's actual framework/styling system
   (from `discover.json`) and its real copy (from `analysis.md`), following
   the variant's specific rules throughout, not just in the hero.
3. **Embed the real logo and real content photos, not text/placeholder
   stand-ins.** Read `discover.json`'s `assets` field
   (`.output/assets/manifest.json`, written by `capture-assets.py`). If it
   has a `logo` entry, use it as a real `<img src="...">` (or inline the
   captured `.svg`) in the header, sized to fit the variant's own nav/header
   rules — never re-colored or redrawn to "fit" a direction's palette, since
   a logo is a fixed-identity element (`../references/preservation-contract.md`),
   not a design choice. A real logo whose only real-world context is a
   different background (e.g. a white wordmark meant for a dark header,
   dropped into a light-header direction) gets a small backing chip in the
   site's own real color behind it, not a recolor of the asset itself.
   Separately, for every named real item this slice's copy reuses (a
   product, a feature, anything analysis.md names concretely), check
   `manifest.content_images` for a matching `label` and use that real photo
   instead of an empty placeholder box. Every direction slice and variant
   built before 2026-08 skipped all of this because nothing upstream ever
   captured a real asset; text/placeholder stand-ins were never a deliberate
   design decision, they were the absence of one. If `assets` has no entry
   for something (capture genuinely found nothing), the placeholder/text
   fallback is correct — say so, don't invent a photo or a logo.
   **The hero gets its own real visual too, unconditionally.** Check
   `manifest.hero_visual` — `capture-assets.py` captures whatever real thing
   sits behind the hero heading (a photo, a CSS background-image, or a
   `<video>`'s poster/first frame) every run, without needing to be told to.
   An earlier version of this pipeline only captured images a human
   remembered to ask for by name, and a real hero photo went missing from
   every direction slice on a real site until someone noticed it missing by
   eye afterward — that gap is why this is now unconditional, the same way
   the logo capture always was. Use `hero_visual.type` to decide how: `image`
   → real `<img>`; `background-image` → `background-image: url(...)` (or
   inline it as a data URI, same rule as below) on the hero container;
   `video` → this pipeline never inlines an actual video file (impractical
   as base64 in a standalone slice), so use the captured poster/frame as a
   static image in Direction Slices/Explore Variants, and only wire up the
   real `<video src>` once this variant reaches `implement-design.md`'s real
   project, where it's a normal asset reference, not an inline blob. If
   `hero_visual` is null, the hero genuinely has no real visual to reuse
   (a solid-color or gradient-only hero is a legitimate real design) — don't
   invent one.
   **Inline every captured image as a base64 `data:` URI directly in the
   `<img src="...">`, never a relative file path to `.output/assets/`.**
   Confirmed necessary, not a style preference: a relative path only
   resolves when something does a genuine `file://` navigation (which is
   what `render-and-screenshot.py`'s own screenshots do, so this bug stayed
   invisible there); several ordinary ways of actually looking at the file
   directly instead take a static snapshot of the HTML that breaks relative
   references to sibling files entirely, silently, with no error — a real
   deliverable HTML file has to survive being opened, copied, or shared
   through more than one of those. A slice/variant that only ever "works in
   the screenshot" isn't real code in the sense this whole product promises.
   `capture-assets.py` already resizes/recompresses every downloaded image
   under a real byte cap for exactly this reason (confirmed: a genuine,
   unmodified photo straight off a real CDN was large enough that a
   different, size-limited viewer failed to even open the file, while
   smaller images on the same page worked) — there's no separate manual
   resize step to remember here, it already happened at capture time.
4. Render it via `webapp-testing` against the dev server, screenshot the
   **whole rendered page, full-page, top to bottom** (never a viewport-height
   crop), save to `.output/directions/<variant-name>/`.

## Handoff

**Before building anything, narrate which 2 (or 3) exploratory directions
"Selecting which 2" picked and why** — plain language, not just internal
reasoning: name them, and say what made them win (genome distance from each
other, the ivory-serif exclusion, a structural/IA fit the brief specifically
calls for), and ask the rule-6 question about a 4th slice in the same
message. State the total explicitly as **"3 total: your current design with
its real defects fixed, plus the 2 I picked"** (or "4 total" if the human
opted into a 3rd) so an earlier "2 I'm picking" and a later slice count
never read as two different, unreconciled numbers. The first time "direction
slice" comes up in a session, say what it means in plain language (e.g. "a
full-page rendered mockup of one design direction") — don't assume the term
is self-explanatory.

**Build slices one at a time, progressively — never build all of them
silently and reveal them together.** But show them through a single
regenerated comparison page, not a fresh browser tab per slice.
Confirmed directly: this environment's Browser pane only ever composites
one tab live, so a tab-per-slice mechanism silently shows stale frames on
every tab but the most recently active one — it reads to the human as "the
other slices didn't render" or "they all look the same," not as a tooling
bug. `../scripts/build-compare-page.py` builds one page with a button per
slice, each button showing a real `<iframe src="http://...">` served by a
local HTTP server — see `../references/approval-policy.md`'s "How gates
are actually shown" (Rev 8) for the full finding and why this isn't the
same thing as the base64-`srcdoc` combined page rev 7 removed.

Concretely, for each slice (current-fixed first, then each exploratory
direction in the order narrated above):

1. Build the real HTML file (per "Building each slice" above), with a real,
   distinguishing `<title>` (e.g. "industrial-brutalist-ui — paper
   colorway") — the rendered page is the real deliverable, unwrapped, not a
   slice embedded inside some other meta-page.
2. Run `mechanical-check.py` against it.
3. Regenerate the comparison page with `build-compare-page.py`, adding one
   more `--option "label=/relative/path/to/index.html=score"` for this
   slice, then open (first slice) or force-reload (every slice after) the
   one comparison-page tab. Tell the human it's ready in the *chat*
   message — "`current-fixed` is up: [1-3 real notable facts, including any
   defects fixed] — mechanical-check: 11/11." One short message per slice,
   not saved up for the end. The facts and score live in this message, not
   baked into the option's own page — the comparison page's button labels
   and score badges are the only thing printed near the slice, and that's
   generated by the script, not hand-authored per slice.
4. Immediately start the next slice. The human can look at what's already
   in the comparison page while this one builds — that's the point of doing
   this one at a time instead of batching everything before showing
   anything.

**Before presenting anything, sweep for and close any stray `file://` tab.**
The `Write` tool's own preview hook auto-opens a `file://` tab for every
HTML file the instant it's written — before asset substitution, so it can
show broken placeholders that were never real. Check `tabs_context` for
`file://`-origin tabs and close them; only the one `http://`-served
comparison-page tab should ever be shown to the human.

**The decision itself, once every slice in this pass is in the comparison
page, is an `AskUserQuestion` call** — not a numbered list in plain chat.
Options: each slice by name (with its 1-3 notable facts as the option's
description), plus explicit flexibility options in the same call: "ask for
more directions" (agent picks 1-2 more, same selection method, added to the
same comparison page) and "let me pick one more myself" (name a
`direction_variants` skill not yet shown). Always include a "stop here"
option — nothing already built is lost. See `approval-policy.md`'s "How
gates are actually shown" for why this is the mechanism now (and what it
replaced).
