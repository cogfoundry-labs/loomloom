---
name: explore-variants
description: Produce 3 real structural variants by default of the Gate-1-chosen direction+colorway (the Gate 1 slice itself, reused for free, plus 2 new compositions), mechanically checked and agent-scored, no loomloom. A 4th variant and further variants are available on request, before building or after seeing the first 3. Rev 7: promoted from an inline orchestration note to its own skill file, since the process now has real judgment calls worth writing down.
---

# explore-variants

Runs after Gate 1 (direction + colorway choice), before Gate 2. Not a place
to re-litigate the direction or colorway already chosen — this stage varies
**structure**, not aesthetic: layout, composition, section order, emphasis.
A variant may end up using a different colorway than the Gate-1 pick if a
particular structural idea genuinely reads better in one (e.g. a full-bleed
photo hero wants the direction's darker colorway more than its lightest
one) — that's a legitimate per-variant choice, not a second palette
exploration. The axis being tested here is structure; color is incidental to
it, not the subject of it.

## The default: 3 variants, not 6-10

Rev 7 cuts this stage's default pool from 6-10 structural variants down to
3, for the same reason Direction Slices was cut (rev 7, see
`generate-directions.md`): building, screenshotting, and mechanically
checking each variant is real time, and a human choosing at Gate 2 was
rarely meaningfully served by the 4th through 10th option.

1. **Variant 1 is the Gate-1 slice itself, reused, not rebuilt.** It already
   exists, already passed its mechanical check, already has a real
   screenshot. Present it as-is at Gate 2 as the baseline composition — free.
2. **Variants 2 and 3 are new, genuinely different structural compositions**
   of the same direction+colorway. Different axes to draw from (pick 2 that
   contrast, not 2 small tweaks of the same idea): hero treatment (split vs.
   full-bleed vs. asymmetric), content hierarchy (flat grid vs. one featured
   item promoted), section order (does the most time-sensitive or most
   compelling real content lead, or does it follow a warm-up section), nav
   placement (top bar vs. persistent sidebar rail), density (a stacked
   single-column read vs. a multi-column grid). Ground the choice in
   `analysis.md`'s real findings — which axis actually matters for *this*
   site's real content — not a generic list applied unchanged every run.
3. **Offer a 4th before building, same pattern as Direction Slices.** Ask
   once, in the same message that narrates variants 2 and 3: "want a 4th
   structural variant? Real trade: more build/screenshot/check time before
   Gate 2." Default (no answer) is 3. The human can always ask for more
   after seeing Gate 2's real variants too — this isn't the only chance.

## Building each new variant

Same rules `generate-directions.md` already established, applied to a new
composition rather than a new direction:

- Real code, real copy from `analysis.md`, real assets from
  `.output/assets/manifest.json` (base64-inlined, same reasoning as
  Direction Slices — a variant is a standalone file with no server behind
  it yet).
- Hero plus at least 2 more real content sections, full-page screenshot,
  never a viewport crop.
- One real `<main>` landmark, real `<meta name="viewport">` plus real
  `@media` breakpoints that actually change the layout, no em-dashes.
- Run `mechanical-check.py` against every variant. A real, honest
  mechanical fail on one check is not disqualifying by itself — a variant
  that deliberately trades a persistent sidebar nav for a nav-height fail
  (`nav-single-line` assumes a horizontal bar) is a real, explainable
  choice, not a defect to hide or a reason to discard the variant. Say so
  plainly in that variant's own notes rather than silently dropping it or
  reshaping it just to chase a clean pass.
- Write an aesthetic note per variant yourself (no loomloom — the
  `asset_ref`/`reference`-port path this would have used is confirmed
  broken, see `../references/model-policy.md`; this stage has been 100%
  agent-scored since rev 6). One or two sentences on what the composition
  actually trades off, grounded in what's real about it, not generic praise.

## Handoff

**Progressive build, single comparison page — same mechanism as Direction
Slices** — see `generate-directions.md`'s "Handoff" for the full reasoning
(the only-one-tab-composites finding and the phantom-`file://`-tab sweep
both apply here unchanged; see `../references/approval-policy.md`'s "How
gates are actually shown," Rev 8). Build variant 2, regenerate the
comparison page with `../scripts/build-compare-page.py` adding it as a new
`--option`, force-reload the one comparison-page tab, tell the human it's
ready with its mechanical score and aesthetic note, then build variant 3
while they look. Variant 1 (the reused Gate-1 slice) is just another
`--option` in the same page from the start — no separate tab to reuse or
reopen.

**Mechanical score and the aesthetic note are reference information for the
human's decision, not a pre-filter the agent uses to decide what gets
shown.** Rev 7 fixes a real mistake from an earlier run: a variant was
ranked last in a curated list specifically because of an honest,
already-explained mechanical trade-off (a sidebar nav's real nav-height
fail), despite being independently described as the most structurally
interesting option in the same breath. With only 3-4 variants built by
default, there is no curation step to begin with — every variant that gets
built gets shown, in the order built, with its real score and note attached
so the human can weigh it themselves.

**The decision is an `AskUserQuestion` call**, same shape as Gate 1: each
variant by name (mechanical score + aesthetic note as the option's
description), plus "ask for 1-2 more variants" and "let me describe a
structural idea myself" as explicit options, plus "stop here." See
`../references/approval-policy.md`'s Gate 2 section.
