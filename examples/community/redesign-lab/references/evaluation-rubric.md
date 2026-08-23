# Evaluation rubric: mechanical vs. aesthetic

Two kinds of finding, scored differently because vision-language models are
reliable at one and not the other.

## Mechanical: auto-scored, can eliminate

Run by `../scripts/mechanical-check.py`, zero model calls, always free.
Pulled from the active design authority's `preflight_check`
(`design-authority.md`): for the default, `design-taste-frontend` §14:

- `no-em-dash`: zero em/en-dash characters anywhere in page text, including
  content reachable via collapsed accordions/tabs, not just what's currently
  rendered on screen.
- `hero-line-count`: the `<h1>` renders in ≤ 2 lines.
- `cta-no-wrap`: CTA-scoped buttons/links (`.btn`, hero CTAs, nav links)
  render on one line; measured via the text's own line-box count, not
  container height (a padded button is not a wrapped button).
- `no-duplicate-cta-intent`: no two CTAs on the page share the same intent
  bucket (contact / signup / portfolio) under different labels.
- `contrast-aa`: every interactive element's text vs. its *effective*
  background (walking up the DOM past transparent ancestors) clears 4.5:1.
- `eyebrow-budget`: uppercase-tracked micro-labels sitting immediately
  before a heading, counted per `ceil(sectionCount / 3)`. Repeated sibling
  instances of one cohesive component (e.g. 3 labels inside 3 near-identical
  cards) count as one budget unit, not N: that's one component using
  consistent internal labeling, not N separate page-level eyebrow tells.
- `nav-single-line`: primary nav renders at one line, height ≤ ~80px.

Any Fail here blocks completion. These are observations, not taste: a
vision model (or a script) verifying them is checking a fact, not forming an
opinion.

## Aesthetic: advisory only, never eliminates

"Feels distinctive," "reads premium," "composition is interesting." Surfaced
to the human alongside the mechanical score at Gate 2, as reference
information for their own decision — never used by the agent to decide
which built variants even get shown. With the small default pool (rev 7:
3-4 variants, not 6-10), there's no curation step to begin with; genuinely
unreliable to automate, and exactly where the human stays the final judge
either way.

Written by the agent directly, sequentially, one note per variant
(`explore-variants.md`) — never batched through loomloom. Rev 6 confirmed
the `asset_ref` -> `reference` port binding an earlier loomloom-batched path
depended on is broken (17 real failing tasks, see `model-policy.md`); there
is no paid alternative to this rubric anymore, only this one free path.
