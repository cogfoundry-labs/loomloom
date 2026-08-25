---
name: redesign-lab
description: Turn an existing website into several real, working design directions, let a human choose, then build the winner. Triggers on "redesign my website", "redesign this site", "I like [reference sites], redesign my site using that taste", or "explore variants of this direction". Built on loomloom and a pluggable design-authority contract (default Leonxlnx/taste-skill).
---

# Redesign Lab

"Give me a website. I'll show you what it could become, let you choose, then
build the winner."

## Entry point

1. Read `references/design-brief.md`'s shape and capture the user's request
   into that shape: mode (`existing-site` vs `from-references`), target
   project, any named reference sites, the one-line message.
2. Read `pipelines/redesign-existing-site.yaml` (always, even in
   `from-references` mode: see step 3) for the stage sequence and gate
   copy. This is not executable YAML; it's a manifest you follow, the same
   way you'd read any pipeline stage-by-stage.
3. If `mode: from-references`, first read `skills/extract-design-signal.md`'s
   reference-mode section: it prepends one stage, then rejoins the same
   pipeline. Otherwise skip straight to Discover.
4. Work through the seven Redesign Lab skills in `skills/`, in the order the
   pipeline manifest lists them: `discover-site.md` → `extract-design-signal.md`
   → `generate-directions.md` → (Gate 1) → `explore-variants.md` → (Gate 2) →
   `implement-design.md` → `validate-design.md` → (Gate 3) → Share
   (Gate 4, `build-case-study.md`).

## The one rule that governs everything else

**Real code, never a mockup, at every decision point.** Direction Slices are
real, rendered multi-section pages (hero plus at least 2 more real content
sections: see `skills/generate-directions.md`), always screenshotted
full-page, never cropped to a viewport. Variants are real rendered
implementations, same rule. Nothing the human chooses between was ever a
generated image, or a hero-only render hiding everything below the fold,
standing in for what the code will actually look like.

## Every gate: real choices, never a dead end

Read `references/approval-policy.md` in full before running any gate, not
just for its copy. Every gate — Gate 1, Gate 2, Gate 3, Gate 4, and every
re-presentation after a fix — shows real work, then asks for a decision.
For a design choice (Gate 1, Gate 2): build each option's real HTML, serve
the run's output over a real local HTTP server, generate a single real
comparison page with `scripts/build-compare-page.py` (each option a real
`<iframe src="http://...">`, never inlined bytes), and re-run it — one more
`--option` each time — as each option finishes building, force-reloading
the same one tab so the reveal stays progressive without ever needing a
second tab. Once every option in that pass is built, ask which one via
`AskUserQuestion` — not a numbered list typed in chat. Six standing rules
apply, no exceptions:

1. **"Stop here" is always one of the options in the same `AskUserQuestion`
   call** — not a side channel the human has to know to ask for unprompted,
   and never squeezed out by however many real options exist that run.
2. **Design choices (Gate 1, Gate 2) are shown as real rendered pages,
   never described or only screenshotted**: every option is the actual
   working code, embedded live via a real HTTP-served `<iframe src>` with
   no wrapper or meta-header added to the option's own page — a few lines
   on what's actually notable about it (including a one-line
   mechanical-check summary) go in the chat message announcing it's ready,
   not printed into the page itself. See approval-policy.md's "How gates
   are actually shown: real pages, not descriptions" for the full
   contract, including why this replaced an earlier one-tab-per-option
   mechanism (this environment's Browser pane only ever composites one tab
   live, so every tab but the most recently active one silently showed a
   stale frame — confirmed directly, twice), and Gate 3's plain-text-only
   variant with no comparison page at all (it isn't a design choice — the
   design was already chosen at Gate 2).
3. **Every choice includes the reminder that a problem gets fixed and
   re-presented**, not forced into picking the least-bad of several broken
   options.
4. **Narrate the reasoning behind the picks before building anything, and
   go straight from opening each tab to telling the human it's ready** — no
   placeholder/no-op tool call in between. Say which options were picked
   and why (genome distance, structural-axis reasoning), and state totals
   explicitly (e.g. "3: baseline + the 2 I picked") so two true numbers
   never read as an unreconciled discrepancy. See approval-policy.md's Gate
   1/2 sections.
5. **Small pools by default, more always available.** Rev 7 cuts Gate 1's
   default from 4 slices to 3, and Gate 2's from 6-10 variants down to 3
   (the first reused free from Gate 1) — specifically because building and
   checking each option is real time, and most runs didn't need the extra
   options. Every gate's `AskUserQuestion` still offers "show me more" and
   "let me pick/describe one not yet shown" as real options, both before
   building (a one-time offer alongside the initial narration) and after
   seeing the first pass's results — cutting the default never means
   cutting the ceiling.
6. **Share is Gate 4, and it's the one deliberately paid stage** — Explore
   stays free by design, Share costs money because it produces one real,
   publishable asset the pipeline can't write for free: narrative prose
   turning verified facts into readable chapters. (Rev: the Share cover
   used to be a generated visual identity too; that's removed — a
   typographic hero built from real color tokens replaced it, free, no model
   call, after the generated version proved both the pipeline's single
   biggest cost/reliability risk and, on its own terms, still only
   decorative.) Show the real cost estimate before anything is spent, reuse
   every already-produced artifact rather than regenerating it, and open
   the finished case study (and the real site, if this run actually
   produced one) in the Browser pane — the final result gets the same
   "show, don't just describe" treatment as every gate before it. See
   `skills/build-case-study.md` and approval-policy.md's Gate 4 section.

## Design authority

Read `references/design-authority.md` before any design judgment or
build-rule decision. Default is `leonxlnx-taste-skill`: do not modify or
fork it; it's read, applied, never edited.

## Gates 1-3 are entirely free — and loomloom doesn't need to be installed until Gate 4

Rev 6 removed loomloom from Explore Variants entirely: the `asset_ref` ->
`reference` port binding that path depended on is confirmed broken (17
real failing tasks across 4 models and 2 execution units — see
`references/model-policy.md`). There is no paid alternative to offer at
Gate 1 anymore, and no execution-mode choice to make there either —
mechanical checks (`scripts/mechanical-check.py`) plus agent-written
aesthetic notes are the only path through Direction Slices and Explore
Variants, always free. Loomloom's only role anywhere in this pipeline is
Gate 4 / Share, and that's deliberately paid — see `skills/build-case-study.md`.

**This means loomloom's own installation and API-key configuration should
also wait until Gate 4 is actually confirmed, not happen as part of
setting up this pipeline skill.** A human who redesigns a site and stops at
Gate 3 (a common case) never needed loomloom at all; making them install
and configure it upfront is unnecessary friction for a capability most
runs never touch. `build-case-study.md`'s first documented step is
checking loomloom is installed/configured right when a human says "do the
case study now," not before — see that file's "Prerequisite" section.

## What NOT to do

- Don't ask loomloom to write design-authority-governed code. Loomloom is a
  bare model call; it can't load a skill file. Its only real role anywhere
  in this pipeline is Gate 4 / Share (see "Gates 1-3 are entirely free").
- Don't generate an image as a stand-in for a direction or variant. Ever.
- Don't skip the preservation contract (`references/preservation-contract.md`)
  because the new design looks good. "Looks good" and "didn't break the
  site" are two different, both-required questions.
- Don't add an eighth Redesign Lab skill. If a capability can be represented as
  data, a reference file, or an existing installed skill, it isn't a new
  skill: extend one of the seven above instead. (`explore-variants.md`,
  rev 7, cleared this same bar itself: the process it documents — default
  pool size, structural-vs-color axis, progressive one-tab-per-variant
  reveal, mechanical score as reference not filter — was substantial enough
  to warrant its own file rather than living on as an inline note here.)
