# Redesign Lab — Design Specification (v1)

*A loomloom-native website-redesign pipeline, built on a pluggable
design-authority contract and a free-by-default execution path.*

> "Give me a website. I'll show you what it could become, let you choose,
> then build the winner."

**Status**: Pipeline 1 (`redesign-existing-site`) implemented and tested
end to end, all four human gates, on real live sites and one local test
fixture. **Default design authority**: Leonxlnx/taste-skill v2, unmodified.
**loomloom's role**: exactly one call, in the final Share stage.

---

## 1. Product concept

Redesign Lab turns an existing website into several real, working design
directions, lets a human choose, then builds the winner. The user's mental
model stays simple regardless of what runs underneath it:

```
Existing website
    │
    ▼
Discover (real routes, real logo, real hero image, design authority declared)
    │
    ▼
Analyze (taste measurement + design-authority audit + preservation baseline)
    │
    ▼
Direction Slices ×3 (current-fixed baseline + 2 exploratory directions,
    one deliberately-chosen colorway each, hero + 2 real sections, full-page,
    opened progressively in real browser tabs — a 4th slice and further
    colorways are available on request)
    │
    ▼
GATE 1 — pick a slice, ask for more, or stop
    │
    ▼
Explore Variants ×3 (the Gate-1 slice reused free, + 2 new structural
    compositions of that same direction — hero treatment, hierarchy,
    section order, nav placement)
    │
    ▼
GATE 2 — pick a variant, ask for more, or stop
    │
    ▼
Implement (the specific page the request was about, first)
    → Validate (functional + accessibility + mechanical checks + preservation)
    │
    ▼
GATE 3 — approve the build, choose scope (this page / key pages / all pages)
    │
    ▼
GATE 4 — Share confirmation (the one deliberately-paid step)
    plan (free): real diff, real evidence, real cost estimate
    generate (paid, after approval): narrative chapters via loomloom
    │
    ▼
Case Study (interactive before/after, real chapters, reproduce-this)
```

The single idea the whole architecture exists to prove: **exploration and
production are separate steps, and every decision a human makes is informed
by real, rendered code — never a mockup.** Section 5 is why that's true in
practice, not just in the diagram.

## 2. Principles

1. **A design authority = design intelligence, loomloom = execution engine,
   Redesign Lab = orchestration and product experience.** These roles never
   blur into each other.
2. **Reuse before build.** A new Redesign Lab skill file only gets written
   when no existing skill covers the need, existing coverage doesn't clear
   the quality bar, or the gap is specifically about orchestrating this
   workflow.
3. **Never modify or fork a design authority.** It's read, not edited, not
   copied into a digest file, not forked into a Redesign-Lab-owned variant.
4. **Separate exploration from production.** No single action goes straight
   from "analyze" to "shipped." Directions get explored before any one of
   them gets built out fully.
5. **Humans decide at four points only** — direction choice, variant choice,
   final build & scope, and Share's spend confirmation — everything else the
   system does on its own.

## 3. Architecture

A loomloom step is a bare model call — prompt, optional reference, one
output. There is no concept in loomloom's execution model of loading an
agent skill file into it, and a design authority only exists as rules an
agent session reads and applies. The agent session, not loomloom, is the
center of this system:

```
                    AGENT SESSION (Claude Code / Codex)
                active design authority loaded throughout
                                │
             reads project ────┼──── writes/edits real code
                                │
             ┌──────────────────┼──────────────────┐
             │                  │                  │
             ▼                  ▼                  ▼
      redesign-lab       loomloom CLI (Share only)   local tools
    (pipeline + skills)          │              (render, screenshot,
                                 │               filesystem, git, axe-core)
                                 ▼
                           text-generate
                     (Share's narrative chapters,
                      batched, text-only — the one
                        real loomloom call here)
```

loomloom is invoked by the agent session, as a CLI subprocess, only inside
the Share stage (Gate 4) — it never runs the pipeline itself, and it never
touches the design authority directly. Every stage before Share (Discover
through Gate 3) is entirely local/agent, with zero loomloom involvement and
zero cost. `image-generate` can also be called, separately, by Implement for
real embedded page assets beyond the logo — rare in practice, and unrelated
to Share.

## 4. The design-authority contract

No Redesign Lab skill hardcodes a design authority's name. Each reads
`references/design-authority.md` — one flat file, not a plugin system — for
four capabilities: `build_rules`, `direction_variants`, `redesign_audit`,
`preflight_check`.

```yaml
name: leonxlnx-taste-skill
version: v2
status: default
capabilities:
  build_rules: design-taste-frontend §2-13
  direction_variants:
    minimalist-ui, industrial-brutalist-ui, high-end-visual-design,
    design-taste-frontend (base), ui-craft-editorial,
    ui-craft-dense-dashboard, product-proof-saas,
    operational-enterprise-ai
  redesign_audit: redesign-existing-projects (Scan→Diagnose→Fix)
  preflight_check: design-taste-frontend §14
```

Which authority governs a project is one line in the Discover output,
chosen once and locked for the run, never mixed mid-project.

### The direction-variant pool

Five of the eight variants are **aesthetic/vibe** families: real, distinct
constraint sets — typography, color, shadows, borders — that don't touch
information architecture. The other three are **structural/IA** families,
each organizing the page around something different (operator density; a
real product mechanism as evidence; permission/audit/rollback state):

| Variant | Register |
|---|---|
| `minimalist-ui`, `industrial-brutalist-ui`, `high-end-visual-design`, `design-taste-frontend` (base) | Aesthetic/vibe |
| `ui-craft-editorial` | Aesthetic/vibe — serif, reading-column, magazine register |
| `ui-craft-dense-dashboard` | Structural/IA — operator-tool density, semantic-color tables |
| `product-proof-saas` | Structural/IA — SaaS page structured around a real workflow demo as evidence |
| `operational-enterprise-ai` | Structural/IA — enterprise/ops page structured around permissions, audit, rollback |

`generate-directions.md` prefers including at least one structural/IA pick
among the directions chosen when the brief's content actually supports it —
a should, not a hard rule.

### The style genome

All eight variants (never `current-fixed` — it isn't exploring a point in
the design space, it's the project's own existing point) are scored 0–100
on eight shared dimensions in `references/style-genome.md`: Density,
Whitespace, Motion, GridRigidity, EditorialFeel, DataDensity, and two more,
read directly from what each skill actually specifies. `generate-directions.md`
picks exploratory directions that maximize real distance from the current
site and from each other on this genome — a calculation, not a vibe check.

One hard exclusion sits on top of the distance calculation:
`minimalist-ui`, `design-taste-frontend` (base), and `ui-craft-editorial`
converge on the same "warm ivory canvas, serif headline" read despite
nonzero genome distance between them — never pick more than one of that
trio among the directions shown.

## 5. Pipeline — `redesign-existing-site`

### Stage detail

- **Discover** (local, no model call): framework, styling system, package
  manager, routes, existing components/assets, dev command, plus which
  design authority governs this project (default `leonxlnx-taste-skill`
  unless overridden). Also runs `scripts/capture-assets.py` unconditionally:
  the real logo, and the hero's own real visual (photo, CSS
  background-image, or video poster/frame), anchored to real page structure
  rather than a label someone remembered to ask for.

- **Analyze** (agent, `redesign_audit`): `extract-design-signal.md`, run in
  self-mode — invoke [senlindesign/taste-skill](https://github.com/senlindesign/taste-skill)
  against the existing site (Playwright screenshot + DOM extraction,
  producing a Design Map + Taste DNA) for objective measurement, then feed
  that into the declared authority's `redesign_audit`. For the default,
  that's Leonxlnx/taste-skill §11.B (brand tokens, IA, preserve/retire, dial
  reading, SEO baseline) plus `redesign-existing-projects`' Scan→Diagnose
  phase for the tactical, numbered-priority fix list. This stage also
  establishes the preservation baseline (below) and the mechanical-check
  findings `current-fixed` exists to resolve.

- **Direction Slices** (agent, real code): three slices by default. One is
  `current-fixed`: the project's own existing design with only its concrete
  defects actually fixed — a real, mechanically-scored anchor for "how much
  of what's wrong here is a full redesign versus a punch list of real
  defects." The other two come from the declared authority's
  `direction_variants`, picked to maximize style-genome distance (Section
  4); a third exploratory direction is offered on request. Every slice is
  hero plus at least two more real content sections, never hero-only, and
  ships one deliberately-chosen colorway — picked from the real site's own
  content and register, not whichever archetype a skill lists first.
  Screenshotted full-page, top to bottom, every time. Image models never
  touch this stage: they can't reliably render legible, aligned UI text.

- **Gate 1 — direction choice**: each slice opens in its own real browser
  tab, progressively, as it's built — the human sees the first one
  immediately, not after the whole batch. Once every slice in the pass is
  open, an `AskUserQuestion` call presents them (mechanical score + real
  notable facts per option) alongside "show me more directions," "let me
  pick one myself," and "stop."

- **Explore Variants**: three by default. Variant 1 is the Gate-1-chosen
  slice itself, reused for free; variants 2 and 3 are genuinely different
  structural compositions of that same direction and colorway — hero
  treatment, content hierarchy, section order, nav placement — not a second
  round of color exploration. Every variant gets a mechanical check and an
  agent-written aesthetic note; no loomloom call anywhere in this stage.

- **Gate 2**: same progressive-tabs-then-`AskUserQuestion` mechanism as Gate
  1. Mechanical score and aesthetic notes are shown as reference information
  for the human's own decision, never used by the agent to pre-filter what
  gets shown.

- **Implement** (agent, `build_rules`): the winner becomes the real site —
  the specific page the request was actually about, first, not a
  full-site rewrite in one shot. `redesign-existing-projects`' Fix-phase
  priority list becomes the actual edit order. `image-generate` is used, if
  at all, only for embedded assets, never as a stand-in for layout. Gate 3
  decides whether to extend further.

- **Validate** (`preflight_check` + preservation, plus two
  authority-agnostic pieces): [anthropics/skills webapp-testing](https://github.com/anthropics/skills/tree/main/skills/webapp-testing)
  for render/functional/link checks, [snapsynapse/skill-a11y-audit](https://github.com/snapsynapse/skill-a11y-audit)
  for real axe-core accessibility scanning, the declared authority's
  mechanical Pre-Flight Check, and the Preservation Contract below, scaled
  to the run's `brand_fidelity`. Any Fail blocks completion.

- **Gate 3**: human approves the build, and chooses scope — this page only,
  other key pages (human-curated, suggested from nav-linked routes), all
  pages, or stop. Picking wider scope re-enters Implement/Validate for just
  the newly-chosen routes, reusing the already-approved direction/variant/
  colorway with no new design decisions and no new gate for that pass.

- **Share**: Section 8.

### The preservation contract

Redesign carries a trust risk greenfield building doesn't: making a site
prettier while quietly breaking it. Established as a baseline in Analyze,
re-checked in Validate: routes and anchors, primary nav labels, form field
names/order, brand logo, legal/consent copy, analytics events, accessibility
not regressed, links resolve, responsive behavior holds. A real logo/hero
check cross-references a mechanical `*-present` finding against what
`capture-assets.py` actually captured at Discover, so a slice that silently
drops a real asset is caught, not assumed fine because "it looks done."

A `brand_fidelity` dial (`conservative` / `moderate` / `radical`, set once
in `design-brief.md`) scales how much of the above is negotiable. Three
items never move at any level: legal/consent copy, analytics event
coverage, and brand identity. Everything else scales — conservative fixes
nearly all of it, radical opens up nav-label wording, IA, route naming, and
logo treatment while still keeping accessibility, link integrity, and
working functionality non-negotiable.

## 6. Where loomloom fits — and where it doesn't

| Task | Engine | Why |
|---|---|---|
| Analyze the existing site | agent, local — senlindesign/taste-skill | Agent-native, not a bare API call |
| Write direction slices & variants | agent, local | Needs the declared authority's real build rules and real rendering to be trustworthy |
| Mechanical scoring (em-dash, contrast, CTA wrap, hero lines…) | local, always free — webapp-testing / axe-core | Every mechanical rubric item is checkable from computed DOM/CSS with a script, no model call needed |
| Aesthetic-advisory scoring (distinctive, premium, composition) | agent, local, always free | Written per option, never used to decide what gets shown |
| Generate embedded assets during Implement | loomloom, `image-generate` | Real assets inside real code, never a stand-in for layout |
| Narrative chapters for the case study | loomloom, `text-generate` | The one real loomloom call in this pipeline |

loomloom is provider-neutral infrastructure with exactly one role in this
pipeline: Share's narrative pass. A human who never reaches Share never
needs loomloom installed at all.

### Known constraint: image references aren't wired through loomloom here

Binding an uploaded reference image to a model's reference input is not
something this pipeline relies on: aesthetic scoring and direction
selection are handled by the agent session directly instead, and confirmed
working that way across every real run. If you're extending this pipeline
to call loomloom with an image reference for scoring or comparison, verify
that path directly against your own loomloom deployment before depending on
it — don't assume it behaves like a plain text-in/text-out call.

### Known constraint: model selection is a fixed pin, not auto-routed

loomloom's execution units expose a flat model list per unit with a single
`isDefault` boolean — no quality/efficiency-tier routing. Share's one
`text-generate` call pins a specific `modelKey` rather than relying on
`isDefault`, so the model used is explicit and stable rather than whatever
a future default change might pick.

## 7. Gates and budget

1. **Gate 1 — direction choice.** Pick one of three real slices by default
   (a `current-fixed` baseline plus two exploratory directions, each shown
   in one deliberately-chosen colorway), presented via `AskUserQuestion`
   once each opens progressively in its own tab.
2. **Gate 2 — variant choice.** Pick from three variants by default (the
   Gate-1 slice reused free, plus two new structural compositions), same
   progressive-tabs-then-`AskUserQuestion` mechanism.
3. **Gate 3 — final implementation & scope.** Approve the build of the
   specific page the request was actually about, then choose how far it
   should reach: this page only, other key pages, or all pages.
4. **Gate 4 — Share confirmation.** The one deliberately-paid gate: shows
   exactly what a real case study will generate and a real `precheck`-
   derived cost estimate for the one loomloom call, before anything is
   spent. If the run built more than one page, a choice of which page to
   feature comes first.

Every cost shown to the user comes from an actual `precheck` call against
the real plan — never a hard-coded figure.

### Every gate: three outcomes

All four gates offer **approve, reject-and-retry, and stop** — never just
the first two (Gate 4's "reject" is simply "skip," since there's no design
decision left to retry). This is a standing rule in
`references/approval-policy.md`: stop is always a real, selectable option
alongside the domain choices, never a side channel the human has to know to
invoke unprompted. Stop means exactly that — nothing already produced is
discarded, and nothing loops back to retry anything on its own.

### How design options are shown

Gate 1 and Gate 2 both ask a human to choose between real, rendered
options:

1. Each option opens in its own real, separate browser tab: the actual
   working file, full quality, no wrapper added to the page beyond a
   distinguishing `<title>`. Opened progressively, one at a time, as each
   option finishes building.
2. The decision is an `AskUserQuestion` call, made only after every option
   in that pass has its own tab open — each option's notable facts and
   mechanical-check summary go in the option's description text, never
   printed into the page itself.
3. "Show me more" and "let me pick/describe one myself" are always among
   the options, alongside "stop."
4. Every choice message ends with the same explicit reminder: if something
   looks wrong, say so and it gets fixed and re-presented — a gate is never
   "pick the least-bad of these broken options."

Browser-tab creation happens one at a time, never batched in parallel, and
overwriting a file on disk does not refresh an already-open tab — a
rebuilt option needs a fresh, forced-reload navigation.

Gate 3 is a scope choice, not a design choice, so there's no per-option
tabs to open there — just the same `AskUserQuestion`-with-a-"stop"-option
pattern every gate carries.

### How Share (Gate 4) works

Not a design choice either — the design was picked at Gate 2, the build
approved at Gate 3. Share is a spend decision, and it's the only one in the
whole pipeline. Two steps happen before its two phases fire:

1. **Confirm loomloom is actually installed and configured**, right at the
   moment a human says "do the case study now" — nothing earlier in the
   pipeline needed it. If it isn't, walk through installing and getting a
   real key there, before `plan` runs at all.
2. **If this run built more than one page, ask which one the case study
   should feature** — one `AskUserQuestion` option per page this run
   actually built. The chosen page decides which real before/after
   screenshots and validation reports feed everything below.

Then the two phases:

1. **`plan` (always free).** `scripts/diff-transformations.py` diffs the
   real before/after pages ($0, no model call), selects 3–6 chapters by
   significance, `scripts/package-share.py`'s `gather_evidence()` collects
   what actually ran, and the one loomloom `TemplateSpec` (narrative) gets
   registered and `precheck`'d for a real cost. Nothing is spent yet.
2. **`generate` (paid, only after approval).** Runs exactly one loomloom
   call — a batched narrative pass over the selected chapters — then builds
   the finished case study as a real, separate-file GitHub-Pages-ready
   folder. The hero uses the real root-color tokens directly; no model call
   is involved in producing it.

The `implemented`/`preview` status is decided once, by one real fact (did
Implement write into a project this pipeline could deploy, or was the
target a live external site it was never going to touch), and a single
shared copy table is the only place that fact becomes user-facing language.

## 8. Scoring: mechanical vs. aesthetic

**Mechanical (auto-scored, can block completion).** Pulled from the
declared authority's `preflight_check` — for the default, Leonxlnx/
taste-skill §14: em-dash, CTA wrap, contrast fails AA, duplicate CTA
intent, eyebrow budget, hero exceeds 2 lines. Checkable without a model
call at all.

**Aesthetic (advisory only, never eliminates).** "Feels distinctive,"
"reads premium," "composition is interesting." Surfaced alongside the
mechanical score, never used to auto-rank or cut a candidate. The human
stays the final judge.

This split matters because vision-language models are reliable at the
first kind of judgment and not the second — observation versus taste.
Direction selection gets the same treatment: the style genome (Section 4)
makes "these picks diverge" a mechanical, checkable claim, and aesthetic
judgment of the results stays with the human at Gate 1.

## 9. The skills

None of the seven names a design authority directly — each reads
`references/design-authority.md` for the capability it needs.

| Skill | Role |
|---|---|
| `discover-site.md` | Project-inspection logic, plus declaring which design authority governs this project. Runs `capture-assets.py` unconditionally for the real logo and hero visual. |
| `extract-design-signal.md` | Thin orchestration: senlindesign/taste-skill measures (authority-agnostic); self-mode feeds the declared authority's `redesign_audit`; reference-mode merges N reference sites into `taste-profile.yaml` when references exist. |
| `generate-directions.md` | Builds the `current-fixed` baseline plus genome-picked exploratory directions, one colorway each, hero-plus-2-sections, full-page. |
| `explore-variants.md` | Builds the default pool of three structural variants (one reused, two new) and hands the decision to the same progressive-tabs-then-`AskUserQuestion` mechanism as Direction Slices. |
| `implement-design.md` | Pointer to the declared authority's `build_rules`, scope-aware: targets the specific requested page first, re-enters for wider scope only after Gate 3. |
| `validate-design.md` | Thin orchestration over webapp-testing, a11y-audit, `preflight_check`, and the Preservation Contract, combining all results into one gate. |
| `build-case-study.md` | Documents Share's judgment calls: chapter selection by significance, the `implemented`/`preview` status determination, the embeddable-fragment mechanism, and the naming conventions for the pipeline's own rendered copy. |

The governing test for whether an eighth skill ever gets added: **if a
capability can be represented as data, a reference, or an existing skill,
it doesn't get a new Redesign Lab skill file.**

## 10. The case study output

Share's `generate` phase produces a real, separate-file GitHub-Pages-ready
folder (`index.html` + `styles.css` + `script.js` + `assets/*`), not one
inlined blob:

```
Hero (typographic, real color tokens, $0) → Interactive before/after comparison
   → What Changed (quick-hit summary) → 3–6 transformation chapters (real diff, real narrative)
   → How Redesign Lab Did It (the real pipeline stages) → Validation (real mechanical/a11y counts)
   → Reproduce this (real, evidence-gated tool & repo credits)
```

The interactive before/after comparison is a fixed-size frame (16:9, so it
reads like a normal screen-recorded demo) with a real vertical scrollbar
inside it, so every part of both real pages stays reachable regardless of
how their lengths differ — neither page is cropped or stretched to match
the other.

Every chapter, and the before/after comparison itself, carries a "Copy the
code" button producing a real `<iframe>` snippet pointing at a small,
self-contained fragment page that reuses the main page's own stylesheet and
script, confirmed to work embedded cross-origin on a third-party page. Tool
and repo credits are evidence-gated `{name, repo, role}` entries — each
only appears if the real file/directory proving it ran actually exists —
including loomloom itself and this pipeline's own repository.

## 11. Repository layout

```
redesign-lab/
├── README.md
├── SKILL.md
├── docs/
│   └── design-spec.md                    # this document
├── pipelines/
│   └── redesign-existing-site.yaml
├── skills/
│   ├── discover-site.md
│   ├── extract-design-signal.md
│   ├── generate-directions.md
│   ├── explore-variants.md
│   ├── implement-design.md
│   ├── validate-design.md
│   └── build-case-study.md
├── references/
│   ├── design-brief.md                   # includes the brand_fidelity field
│   ├── design-authority.md               # one file: 8 direction_variants
│   ├── style-genome.md                   # 8 variants × 8 dimensions
│   ├── preservation-contract.md
│   ├── taste-profile-schema.md
│   ├── direction-schema.md
│   ├── evaluation-rubric.md
│   ├── model-policy.md
│   ├── approval-policy.md                # the 4 gates: outcomes + presentation contract
│   ├── price-report-zh.md
│   └── bug-report-asset-ref-zh.md
├── test-fixtures/
│   └── hello-world-site/                 # a real, owned local project for exercising
│                                          # Implement onward without a live external target
└── scripts/
    ├── discover.py                       # local: project inspection
    ├── capture-assets.py                 # local, free: real logo + hero visual capture
    ├── render-and-screenshot.py          # local: the render step for slices/variants
    ├── diff-transformations.py           # local, free: real computed-style before/after diff
    ├── mechanical-check.py               # local, free: the mechanical rubric, no model call
    ├── score.py                          # local: aesthetic-advisory notes, no model call
    ├── build-case-study.py               # the only script that calls loomloom (plan + generate)
    ├── render-case-study-web.py          # turns case-study-data.json into the real site folder
    └── package-share.py                  # builds the evidence-gated Reproduce section
```

## 12. Future work

- **`redesign-from-references` pipeline.** Everything in Pipeline 1, with
  one stage prepended: analyze N named reference sites via
  senlindesign/taste-skill and merge the results into a `taste-profile.yaml`
  that informs Direction Slices. Documented; not yet run end to end against
  real named references.
- **A second design authority.** `baoyu-design` (JimLiu/baoyu-design)
  supplies `build_rules` but nothing for `direction_variants` or
  `redesign_audit`. Next step is wiring it up against Pipeline 1 and seeing
  whether the one-file declaration is enough to run it, gaps and all — not
  building a registry or resolution mechanism ahead of that test.
- **Two unfilled direction-variant families.** No well-specified,
  installable skill has been found yet for `consumer-modern-ui`
  (mass-market mobile-first) or `experimental-ui` (novel
  navigation/interaction). Don't add either without a real constraint-set
  find.
- **Per-chapter social-card generation and a PDF/ebook export renderer**
  for the case study. Deliberately deferred; the chapter data model is
  already shaped for both.
- **Raising loomloom platform gaps directly with the loomloom team** —
  model-catalog routing (RFC-0003) and image-reference delivery — as
  reproducible reports rather than folded into this pipeline's own
  workarounds.
