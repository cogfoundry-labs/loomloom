# Style genome

A cross-skill scoring table so "pick the exploratory directions that
diverge most" (`../skills/generate-directions.md`) can be a real distance
calculation instead of a judgment call. This does not replace any skill's
own rules: each score below is read directly from that skill's SKILL.md
(its stated knobs where it has them, its stated component/spacing/motion
specs where it doesn't), not invented. Re-score a variant here if its
SKILL.md changes.

**Default pool size is 2 exploratory directions, not 3** (rev 7 — see
`generate-directions.md`'s "The default: 3 slices, not 4"). A 3rd is built
only if the human opts in before Direction Slices runs, or asks for more
after seeing the first 3 real slices. The math below still describes the
general N-candidate method; at the default N=2 it collapses to a single
pairwise distance check (each candidate vs. current, and against the other
candidate), not a true minimum-of-many.

## The 8 dimensions (0-100 each)

- **Density**: how much content/UI per screen. Low = airy, one thing at a
  time. High = dashboard-dense, many things at once.
- **Whitespace**: macro spacing between sections/blocks, independent of
  density (a page can be dense *and* still breathe at the section level, or
  sparse and still cramped at the component level).
- **CornerRadius**: 0 = hard 90-degree corners everywhere. 100 = heavily
  rounded/squircle "double-bezel" surfaces.
- **Motion**: 0 = static or hover-only. 100 = choreographed, springy,
  scroll-linked, multi-phase.
- **Saturation**: 0 = monochrome/near-neutral with at most one restrained
  accent. 100 = saturated, multi-hue, expressive color.
- **GridRigidity**: 0 = organic/asymmetric composition. 100 = strict
  column/row grid, blueprint-like.
- **EditorialFeel**: 0 = product/utility register (the page reads as a
  tool). 100 = literary/magazine register (the page reads as a story).
- **DataDensity**: 0 = no metrics/tables/structured data. 100 = the page's
  entire point is showing verified numbers, states, or structured records.

## Scores

| Variant | Density | Whitespace | CornerRadius | Motion | Saturation | GridRigidity | EditorialFeel | DataDensity |
|---|---|---|---|---|---|---|---|---|
| `minimalist-ui` | 35 | 80 | 40 | 25 | 20 | 35 | 75 | 20 |
| `industrial-brutalist-ui` | 75 | 25 | 0 | 30 | 15 | 90 | 30 | 70 |
| `high-end-visual-design` | 55 | 75 | 85 | 75 | 55 | 40 | 60 | 30 |
| `design-taste-frontend` (base) | 50 | 60 | 50 | 50 | 45 | 50 | 50 | 35 |
| `ui-craft-editorial` | 20 | 90 | 30 | 30 | 20 | 30 | 95 | 15 |
| `ui-craft-dense-dashboard` | 90 | 15 | 15 | 25 | 30 | 90 | 10 | 95 |
| `product-proof-saas` | 60 | 55 | 55 | 45 | 35 | 80 | 35 | 65 |
| `operational-enterprise-ai` | 70 | 35 | 10 | 30 | 20 | 85 | 20 | 90 |
| `current-fixed` | n/a | n/a | n/a | n/a | n/a | n/a | n/a | n/a |

`current-fixed` has no score: it isn't exploring a point in the design
space, it's the project's own existing point, whatever that happens to be.
Never include it in a distance calculation; it's compared against
qualitatively (see `generate-directions.md`'s "baseline slice" section), not
via genome distance.

## Using it in "Selecting which exploratory directions"

For each candidate variant, treat its row as an 8-dimensional point.
Compute pairwise Euclidean distance between candidates (and, if the current
site's own design was scored the same way during Analyze, against that
point too, to satisfy "don't propose a direction that looks like what's
already there"). At the default pool of 2 exploratory directions, this is:
distance(A, current), distance(B, current), and distance(A, B) — pick
whichever pair of candidates makes all three of those real, not just one
big one. If the human opts into a 3rd, switch to maximizing *minimum
pairwise distance* across all 3, not just average distance: a set where two
picks are close and one is far apart still reads as "two similar options
and one weird one," not three genuinely different directions. A minimum
pairwise distance below roughly 40 (on this 0-100-per-dimension scale) is
the signal to reconsider the selection, not a hard gate: some briefs
genuinely only support one strongly-differentiated read plus one moderate
one, and forcing an artificial extreme onto a brief that doesn't support it
is worse than a slightly closer pair.

This is additive to, not a replacement for, the existing structural/IA vs
aesthetic/vibe framing in `generate-directions.md`: two variants can score
far apart on this genome while still both being aesthetic/vibe families (a
genome distance doesn't by itself guarantee an information-architecture
difference), so keep checking both.

## Known convergence: the ivory-serif cluster

Genome distance is necessary but not sufficient. A real test (2026-08:
lightweight, hero-only, un-colorway'd thumbnails of all 8 variants, built
against a live redesign target) found that `minimalist-ui`,
`design-taste-frontend` (base), and `ui-craft-editorial` converge visually at
the hero: warm ivory canvas, serif or mixed-serif headline, one restrained
accent. Each is a genuinely different skill, verified again at the time of
this test: corner treatment, CTA style (filled-plus-ghost vs. underlined text
links), column width, and internal rules all differ. But a human scanning
thumbnails read the three of them as "the same direction shown three times,"
which is exactly the failure the genome and the aesthetic/vibe-vs-structural/IA
split both exist to prevent — and it happened despite nonzero pairwise
distance between their rows above.

Treat this as a **hard exclusion, not a distance threshold**: never pick more
than one of `{minimalist-ui, design-taste-frontend, ui-craft-editorial}` among
the chosen exploratory directions (2 by default, up to 4 if the human opts
into more), regardless of what their pairwise genome distance computes to.
Canvas tone and hero silhouette aren't fully captured by the 8 dimensions
above; this cluster is the confirmed case where the genome's math and a
human's actual first impression diverge. If a 9th variant is ever added,
re-run this check for real (hero-only thumbnails against a live target)
rather than assuming a high genome distance alone rules out convergence
with the existing 8.
