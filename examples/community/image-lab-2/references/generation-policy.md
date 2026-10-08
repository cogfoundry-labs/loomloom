# Generation policy — the durable half of Image Lab

Maps a **creative intent** to **per-dimension requirement weights** and
**preferred sizes**. No model names. A team edits this to change Image Lab's
taste; it does not rot when CogFoundry's model catalog moves.

`image.py` scores every model in `model-catalog.yaml` against the matched
intent's weights and picks deterministically (§4 of `docs/design-spec.md`). This
file is *what the work needs*; the catalog is *which model delivers it*.

## Dimensions (fixed vocabulary — Arena.ai's Text-to-Image Arena categories)

`overall` · `commercial_design` · `three_d_modeling` · `cartoon` ·
`photorealistic` · `art` · `portraits` · `text_rendering`

These are exactly the 8 category leaderboards at
[arena.ai/leaderboard/text-to-image](https://arena.ai/leaderboard/text-to-image)
— not a hand-invented axis. A model's score on each is computed from real
blind-human-preference Elo ratings in `references/arena-scores.yaml`, not
assigned by hand (see that file and
`docs/plans/2026-09-13-image-lab-arena-scoring-implementation.md`). To add a
dimension here, it must correspond to a real Arena category with its own
leaderboard — there is nowhere else to source a score from.

## Weights

`low` → 0 · `medium` → 1 · `high` → 2. A `high` requirement that a model is
`weak` (-1) at disqualifies that model.

## Sizes

`preferred_sizes` are literal `WxH` strings, ordered — what *this intent*
wants, independent of any model. They are **not** necessarily the literal
`size` parameter sent to the gateway: `image.py`'s `pick_size()` translates
this into whatever the chosen model's own schema actually takes (see
`model-catalog.yaml`'s `request:` field) — a literal `WxH`, a resolution tier
("1K"/"2K"/"4K"), an aspect-ratio-only shape, or no size control at all.
Models differ here in practice, not just in theory (checked 2026-09-13; see
`docs/plans/2026-09-13-image-lab-lessons-from-model-image-arena.md`). The plan
line shows whatever token was actually resolved ("Portrait · 1024×1536",
"2K", or "default").

---

## intents

```yaml
- intent: launch / announcement image
  requirements: { photorealistic: high, commercial_design: medium, text_rendering: medium }
  preferred_sizes: ["1024x1536", "1024x1024"]
  confidence: medium   # arena has no single "launch photo" category; composed from adjacent ones

- intent: profile / avatar
  requirements: { portraits: high }
  preferred_sizes: ["1024x1024"]
  confidence: high

- intent: social post
  requirements: { commercial_design: high, photorealistic: medium, text_rendering: medium }
  preferred_sizes: ["1024x1280", "1024x1024"]
  confidence: medium   # no dedicated "social post" category exists on arena.ai; commercial_design
                        # made explicitly the priority (was 3-way tied at medium, which left the
                        # diversity guardrail's "dominant dimension" to an alphabetical tie-break
                        # instead of a stated choice)

- intent: blog hero / article cover
  requirements: { art: high, photorealistic: medium }
  preferred_sizes: ["1536x1024", "1280x1024"]
  confidence: medium   # same issue — composed from adjacent categories

- intent: poster / flyer
  requirements: { text_rendering: high, art: medium, commercial_design: medium }
  preferred_sizes: ["1024x1536"]
  confidence: medium

- intent: infographic / diagram
  requirements: { text_rendering: high, commercial_design: medium }
  preferred_sizes: ["1024x1536", "1024x1024"]
  confidence: low   # arena has no diagram/schematic category at all — the weakest mapping here.
                     # commercial_design (clean structured layout) is a closer fit than `art`
                     # (painterly quality) was, but this stays low-confidence either way.

- intent: illustration / concept art
  requirements: { art: high, cartoon: medium }
  preferred_sizes: ["1536x1024", "1024x1024"]
  confidence: high

- intent: 3D render / isometric illustration
  requirements: { three_d_modeling: high, art: medium }
  preferred_sizes: ["1536x1024", "1024x1024"]
  confidence: high   # matches Arena's "3D Imaging & Modeling" category directly

- intent: product / e-commerce shot
  requirements: { commercial_design: high }
  preferred_sizes: ["1024x1024", "1024x1280"]
  confidence: high

- intent: generic
  requirements: { overall: high }
  preferred_sizes: ["1024x1024"]
  confidence: high
```

`generic` is the fallback when the agent cannot confidently classify the prompt.
`confidence` documents how directly each intent maps onto a real Arena
category — `high` when one exists 1:1 (e.g. `profile / avatar` → `portraits`),
`medium` when the mapping composes a couple of adjacent categories because no
exact one exists, `low` for `infographic / diagram`, which has no matching
Arena category at all and is the weakest mapping in this file. It's
documentation only — `image.py` does not read this field.
