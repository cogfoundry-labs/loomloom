---
name: extract-design-signal
description: Measure a site's design (the existing project, or named reference sites) using the `taste` skill, then feed that measurement into the active design authority's redesign_audit for judgment. Self-mode runs on every project; reference-mode only runs when the user names reference sites. Second stage of Pipeline 1, first stage of Pipeline 2.
---

# extract-design-signal

Thin orchestration around two already-installed skills. No new measurement
logic: `taste` (senlindesign/taste-skill) does the actual DOM/screenshot
capture, `redesign-existing-projects` does the actual judgment.

## Self-mode (every `redesign-existing-site` run)

1. Run the `taste` skill against the running dev server (`discover.json`'s
   `dev_command`, on `localhost`). This produces `{domain}.md` + `{domain}.json`
  : a Design Map (colors, typography, spacing, radii, shadows, grid) plus
   Taste DNA (Trigger → Decision → Reason → Evidence → Trade-off principles).

   **If Playwright MCP isn't installed/connected, don't treat that as a hard
   blocker** — `taste`'s own `SKILL.md` hard-codes `mcp__playwright__browser_*`
   tool names with no fallback, but its actual methodology (Phase 1: resize
   viewport, navigate, screenshot, run `references/extract.js` via
   `browser_evaluate`; Phase 2: the four-step measure → pattern → taste →
   observer analysis in `references/step1-measure.md` through
   `step4-observer.md`) works the same with any browser-automation tool
   already available in this session — a generic Browser-pane tool included.
   Confirmed directly: `extract.js`'s function body runs unmodified via a
   plain JS-eval tool, wrapped as `(() => { ... })()` instead of passed as
   Playwright's `function` parameter, and produces the same structured
   `domData` the four analysis steps expect. If no browser-automation tool
   at all is available and screenshot capture also fails, run the four
   steps DOM-data-only (the accessibility tree from a `read_page`-style tool
   substitutes for structural context) and say so plainly in `analysis.md` —
   a degraded-but-honest measurement, not a blocked stage. Only actually
   stop and ask the user to install Playwright MCP
   (`claude mcp add playwright -s user -- npx -y @playwright/mcp@latest
   --isolated`, then restart) if literally no way to evaluate JS in a real
   browser context exists in this session.
2. Feed that output into the active authority's `redesign_audit` (from
   `design-authority.md`). For the default authority, that's two passes over
   the same measurement, not one:
   - **`redesign-existing-projects`**, Scan → Diagnose phase: six audit
     categories (typography, color/surfaces, layout, interactivity/states,
     content, component patterns/iconography/code quality), producing a
     numbered, prioritized fix list with concrete component-pattern
     replacements (carousels, modals, pricing tables) where relevant.
   - **`design-taste-frontend` §11.B** judgment on the same data: brand
     tokens, IA, patterns to preserve vs. retire, dial reading
     (`DESIGN_VARIANCE` / `MOTION_INTENSITY` / `VISUAL_DENSITY`), SEO
     baseline.
3. Capture the **preservation baseline** from `../references/preservation-contract.md`
   against the *current* live site: this is the contract Implement can't
   violate later.
4. Merge steps 2 and 3 into `.output/analysis.md`: one artifact, not three
   separate reports the user has to reconcile themselves. **State the
   declared authority in a real, checkable line near the top**, exactly
   this shape: `` Authority: `{authority-name}` `` (the authority's own
   identifier, backtick-wrapped, after the literal word "Authority:"). This
   isn't just a style preference: `package-share.py`'s `gather_evidence()`
   regex-matches this exact pattern as its *only* way to credit the design
   authority when `discover.json` doesn't exist (any run against a live
   external site this pipeline doesn't own, which is the common case) —
   confirmed as a real gap this rev: an earlier draft of this line's own
   phrasing was accidentally freeform, worked only because one specific
   analysis.md happened to phrase it this way, and would have silently
   under-credited the authority in this run's own case study had that
   phrasing ever drifted.
5. **Capture real content images for whatever concrete items `analysis.md`
   names** (real products, features, named things Direction Slices will
   reuse the copy of): run `scripts/capture-assets.py <url> --out-dir
   .output/assets --match "<label 1>,<label 2>,..."` (repeating `--match`
   once per label instead works too — confirmed the hard way that a repeated
   flag used to silently keep only its last occurrence, discarding every
   earlier label with no error; both forms are safe now). This is a second
   invocation of the same script Discover already ran for the logo — real
   content photos can't be matched until real copy exists to match them
   against, which is why this doesn't happen at Discover. Confirmed
   necessary against a real site (bunnings.com.au): these images are
   frequently lazy-loaded, so the script scrolls the full page first to
   trigger their real `src` before matching — a page-load-only scan finds
   none of them. If a label finds no plausible match, that item's slice
   keeps a placeholder for it; don't invent a stand-in photo.

Do not skip step 2's second pass because step 1 already produced "design
data": the Taste DNA output is measurement, not a preserve/retire decision.
Both halves of step 2 are needed; neither substitutes for the other.

## Reference-mode (`redesign-from-references` only: never runs otherwise)

Only enters this mode when the user names reference sites ("I like Stripe,
Linear, and Vercel"). A plain "redesign my website" never triggers this path
and never shows taste-extraction language to the user.

1. Run `taste` once per named reference site: sequential, local invocations,
   not a loomloom batch. (`taste` is itself an agent-native skill, not a bare
   API call; at the typical N of 2-4 references, sequential is cheap and its
   full methodology: cross-page validation, screenshot-over-DOM conflict
   resolution: beats a condensed substitute.)
2. Merge the resulting Design Maps into one `.output/taste-profile.yaml`:
   typography, layout, density, whitespace, color, motion, hierarchy, brand
   character: principles, not literal appearance. The goal is "why this
   works," never "copy this pixel-for-pixel."
3. Hand `taste-profile.yaml` to `generate-directions.md` alongside the
   self-mode `analysis.md` from above: references inform which of the
   authority's `direction_variants` get picked, they don't replace Analyze.

## Schema

Validate both outputs against `../references/taste-profile-schema.md` before
handing off. A malformed `taste-profile.yaml` fails this stage, not silently
propagates into Direction Slices with missing fields.
