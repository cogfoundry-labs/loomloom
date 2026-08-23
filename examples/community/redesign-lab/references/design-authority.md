# Design authority

The single source of truth for which installed skill answers each of the four
capabilities Redesign Lab's skills need. One flat file, resolved once per
project (by `skills/discover-site.md`) and locked for the run: never mixed
mid-project. This is not a plugin system; it's a declaration. See the design
spec's design-authority section for the reasoning behind keeping it this thin.

## Active authority

```yaml
name: leonxlnx-taste-skill
version: v2
status: default
capabilities:
  build_rules: design-taste-frontend        # §2-13 of Leonxlnx/taste-skill
  direction_variants:                       # each a genuinely distinct constraint set
    - minimalist-ui                         # warm editorial minimalism (aesthetic/vibe family)
    - industrial-brutalist-ui               # rectilinear, single red accent (aesthetic/vibe family)
    - high-end-visual-design                # premium agency, squircle/motion-rich (aesthetic/vibe family)
    - design-taste-frontend                 # base read, 4th aesthetic/vibe option
    - ui-craft-editorial                    # serif/reading-column, from educlopez/ui-craft (aesthetic/vibe family)
    - ui-craft-dense-dashboard               # operator-tool density, from educlopez/ui-craft (structural/IA family)
    - product-proof-saas                    # SaaS product-as-proof, from mengto/skills (structural/IA family)
    - operational-enterprise-ai             # enterprise governance/audit/rollback, from mengto/skills (structural/IA family)
  redesign_audit: redesign-existing-projects # Scan → Diagnose → Fix
  preflight_check: design-taste-frontend     # §14 mechanical Pre-Flight Check
```

The first four map to skills already installed in this project's
`.agents/skills/` (verified: `design-taste-frontend`, `minimalist-ui`,
`industrial-brutalist-ui`, `high-end-visual-design`,
`redesign-existing-projects`). That's what makes this the default: not a
policy choice, just the only complete option today.

`ui-craft-editorial` and `ui-craft-dense-dashboard` are from a different
repo (`educlopez/ui-craft`, requires its base `ui-craft` skill installed
too for reference-file pointers to resolve) but are used the same way: an
additive constraint set layered on top of `design-taste-frontend`'s base
rules, same as the original four. Verified against the same bar (exact
color values, exact spacing/typography, an explicit anti-patterns list, not
just a vibe description) before being added here: see
`generate-directions.md`'s "Selecting which 3" for why
`ui-craft-dense-dashboard` matters more than a 5th aesthetic option: it's
the pool's only genuinely **structural/IA** family (density, operator-tool
layout, no card-grid default) rather than another **aesthetic/vibe** read
of the same hero-plus-grid skeleton.

`product-proof-saas` and `operational-enterprise-ai` are from a third repo
(`mengto/skills`, MengTo of Design+Code; each is a standalone file, no base
skill needed) and add 2 more **structural/IA** families, verified against the
same bar: `product-proof-saas` structures a SaaS/AI product page around a
real workflow demo as evidence (input → processing → draft → edit →
approval → publish states, honest pricing, no fake dashboards);
`operational-enterprise-ai` structures an enterprise/ops page around
system boundaries, permissions, audit, and rollback (solution rows with
summary/permissions/action/approval/output/audit/exception/rollback as a
stable data model). Neither is a card-grid dressed differently: both change
what the page is actually organized around.

All 8 variants are scored on 8 shared dimensions in
`style-genome.md`, so `generate-directions.md`'s "Selecting which 2"
(the default exploratory pool, rev 7) can compute an actual distance
between candidates instead of relying only on judgment. That file also has
the reasoning for why `current-fixed` deliberately isn't scored alongside
them.

### Known gap: `consumer-modern-ui` and `experimental-ui`

Real registry research (2026-08, two passes) found no well-specified,
installable skill for these two families, despite covering `educlopez/ui-craft`
and `mengto/skills` (both otherwise strong sources) plus the wider skills.sh
registry. `premium-product-ui` is resolved (`product-proof-saas`,
`operational-enterprise-ai`, above). Candidates for the remaining two were
checked and rejected for real reasons, not just low install counts: a
"progressive-disclosure" hit was about splitting Claude skill files
themselves, not the UX pattern; a "consumer-apps" hit was a growth/ASO
playbook with no visual rules at all; several "editorial"-labeled hits were
either slide-deck templates or thin auto-generated meta-templates with only
placeholder tokens; MengTo's catalog (500+ entries checked) has no
consumer/mass-market-mobile family either. Don't re-add either without a
real constraint-set find, and don't let this gap block real projects: 8
variants already gives `generate-directions.md` far more room than the
original 4 to avoid drawing 3 slices that read as "the same site in
different colors."

## Registered but partial: baoyu-design

```yaml
name: baoyu-design
version: unreleased-in-this-project
status: proof-case
capabilities:
  build_rules: baoyu-design                 # its own system-prompt.md methodology
  direction_variants: null                  # not shipped: documented gap
  redesign_audit: null                      # not shipped: documented gap
  preflight_check: null                     # not shipped: documented gap
```

Not installed in this project yet. Selecting it means Gate 1 loses the
structural-divergence guarantee (`generate-directions.md` falls back to one
design read improvising 3 times) and Validate falls back to `webapp-testing` +
`a11y-audit` only, with no authority-specific mechanical Pre-Flight Check.
`discover-site.md` must say this plainly if a project ever selects it: never
silently.

## Resolution rule

One field in the Discover output: `design_authority: leonxlnx-taste-skill`
(or `baoyu-design`). Chosen once, at Discover time, never re-asked or mixed
mid-project. If a third authority is ever proposed, add a new `## Registered
but partial` (or `## Registered, full coverage`) block above: do not build a
directory of per-authority files until a second authority has actually run
through Pipeline 1 once.
