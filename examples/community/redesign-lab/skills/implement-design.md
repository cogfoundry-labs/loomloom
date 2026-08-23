---
name: implement-design
description: Build the human-chosen winning variant into the real project. Mostly a pointer into the active design authority's build_rules: the new work is wiring the result into the actual project structure Discover found.
---

# implement-design

Runs after Gate 2 (variant choice), and again after Gate 3 if the human
picks a wider scope there (see "Scope" below). Not a place to re-litigate
design decisions already made at Gate 1 or Gate 2: this stage builds, it
doesn't redesign.

## Scope

The first run targets **the specific page the request was actually about**,
not every route. For a whole-site redesign request ("redesign my website"),
that's commonly the home page. For a request that named a specific page
("redesign my pricing page"), it's that page — not an unconditional home-page
default regardless of what was asked. Building just that one page first is
deliberate, not a limitation: it's the cheapest way to get a real, validated
result in front of the human before committing to a full-site rewrite, and
it's what Validate and Gate 3 are actually checking the first time through.
Gate 3's prompt (`../references/approval-policy.md`) asks the human to pick
the scope to build further:

- **This page only**: already done by the time Gate 3 fires. Hand off to
  `validate-design.md`'s already-passed result, then Share.
- **Other key pages**: a human-curated subset beyond this page (e.g.
  pricing, docs landing, about), not an algorithmic guess. Suggest a default
  list from `discover.json`'s routes (pages linked from primary nav are a
  reasonable starting suggestion) and let the human confirm or edit it
  before this stage runs again against just that list.
- **All pages**: every route `discover.json` listed.

For "other key pages" or "all pages", this stage runs again against the
newly-added routes only (the first page is already done), reusing the same
winning direction/variant/colorway with no new design decisions to make:
apply the already-confirmed pattern, don't re-litigate it. Hand off to
`validate-design.md` again for the newly-added routes specifically (a page
with different content can surface a real issue the first page didn't have:
see `../scripts/mechanical-check.py`'s no-em-dash check, which is
content-dependent — also confirmed in practice: a real wider-scope pass
found a genuine `heading-order` violation on a page whose content structure
differed from the first page's). This does not need a new gate: Gate 3's
approval already covered the scope decision, the same way Gate 2's variant
choice covers Implement without a separate approval for the build itself
(`../references/approval-policy.md`, "What never gets a gate"). If the wider
validate fails, the normal `on_fail: return_to(implement)` applies, same as
any other Validate failure.

## What's reused (no new instructional content)

The active authority's `build_rules` govern everything about *how* the code
gets written: for the default authority, all of `design-taste-frontend`
§2-13: typography, color calibration, layout diversification, materiality,
interactive states, layout discipline, image strategy, content density, quote
handling, theme lock. Don't re-explain these rules here; read them from the
installed skill.

If the winning variant came from a sibling aesthetic skill (`minimalist-ui`,
`industrial-brutalist-ui`, `high-end-visual-design`, `ui-craft-editorial`,
`ui-craft-dense-dashboard`, `product-proof-saas`, `operational-enterprise-ai`),
that skill's specific constraints stay in force for the full build, layered
on top of the base rules: same as during Direction Slices.

`redesign-existing-projects`' Fix-phase priority list (captured back in
`extract-design-signal.md`'s Analyze stage) becomes the literal edit order
here: the same Scan → Diagnose output, now actioned instead of just reported.

## What's new here

- **Wiring the winning variant into the real project.** The Direction Slice
  was a handful of sections in isolation (hero plus 2+ more, per
  `generate-directions.md`); this stage extends that same direction into the
  project's actual file structure (components dir, styling system, routing
  convention): not a fresh scaffold. First run: the one page the request was
  about, see "Scope" above. Later runs (if Gate 3 picks a wider scope): the
  additional routes.
- **The real logo and real content photos carry over, but out of inline
  base64 and into the project's actual asset pipeline.** Direction
  Slices/Explore Variants inline every captured image as a base64
  `data:` URI (`generate-directions.md`) because those are standalone files
  with no server behind them. A real project has one: copy the actual files
  from `.output/assets/` into the project's real assets location
  (`discover.json`'s `existing_assets_dir`, e.g. `public/`) and reference
  them with a normal `<img src="/assets/logo.svg">`-style path, the same way
  every other asset in that codebase is referenced. Leaving them as inline
  base64 here would bloat every commit with re-encoded binary data for no
  reason — that trade-off only makes sense for a file with no project and no
  server around it. If the chosen variant somehow still has a text-only
  brand name or a placeholder box and a real asset was captured, that's a
  defect to fix here, not carry forward: check `.output/assets/manifest.json`
  before calling this stage done.

  **This rule assumes a real project with a real server. When Implement's
  target is a live external site this pipeline doesn't own** (the common
  case for a redesign request — there is no real project directory to copy
  assets into, no server to resolve a relative path against), Implement is
  still producing a standalone preview file, same as Direction Slices and
  Explore Variants — and that means the *base64-inline* rule governs, not
  this one. Confirmed the hard way on the shengsuanyun.com run: a relative
  `<img src="assets/logo.svg">` rendered with `naturalWidth`/`naturalHeight`
  0 (a silently broken image, `complete: true` but nothing decoded) the
  moment the file was opened in this environment's Browser pane, because a
  local file outside the project's own directory renders as a static
  snapshot that does not fetch sibling files by relative path — the same
  environment constraint Direction Slices' base64 rule already exists to
  route around, just hit one stage later because this rule said to switch
  away from it. `mechanical-check.py`'s own `logo-present` check does not
  catch this: it only confirms an `<img>`/`<svg>` element exists in a
  home-link at a plausible size, not that the image actually decoded
  (`naturalWidth > 0`) — a real detection gap, not just a one-off miss.
  **Cross-check `naturalWidth`/`naturalHeight` on every carried-over image
  after Implement, whenever the target has no real project/server behind
  it** (a real `<img>` element existing is not the same claim as the image
  actually rendering); don't trust the mechanical check's existence-only
  signal alone for this case.
- **A real hero video becomes a real `<video>` here, not a static frame.**
  If `manifest.hero_visual.type` is `video`, Direction Slices only ever
  showed a captured poster/frame (a real video file is impractical to
  inline as base64). Here, with a real server behind the page, wire up an
  actual `<video>` pointing at `hero_visual.video_src_url` (downloading and
  hosting that file locally if the project's own asset pipeline expects
  local assets rather than a remote URL), with the captured frame as its
  real `poster` attribute so there's still a correct first paint before the
  video loads. Carrying forward only the static frame here, when a real
  video exists, is the same class of defect as a logo that quietly stayed
  text-only.
- **Newly-generated assets, if any.** `image-generate` (via loomloom) is used
  here, and only here, for real assets that ship inside the page beyond the
  logo: hero photography, textures. Never as a stand-in for layout; the
  layout is already real code from Direction Slices / Explore Variants.
- **Respecting the preservation contract, at the run's fidelity level.**
  Every item in `../references/preservation-contract.md`'s baseline
  (captured during Analyze) must still hold after this stage, scaled to
  whichever `brand_fidelity` (`design-brief.md`) the run was set to:
  conservative fixes nearly everything, radical opens up nav labels, IA, and
  logo treatment while still never touching legal copy, analytics coverage,
  or brand identity. This isn't Validate's job alone: Implement is where
  violations actually get introduced, so check as you go rather than relying
  on Validate to catch everything after the fact.

## Handoff

Once this pass's scope is built (the request's own page first, wider scope
on a later pass per "Scope" above), hand off to `validate-design.md`. Don't call this
stage "done" until Validate passes: Implement produces a candidate for
Gate 3 (or, on a wider-scope re-entry, for Share directly), not the final
approved build.
