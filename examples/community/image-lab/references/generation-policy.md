# Generation policy — the durable half of Image Lab

Maps a **creative intent** to **per-dimension requirement weights** and
**preferred sizes**. No model names. A team edits this to change Image Lab's
taste; it does not rot when CogFoundry's model catalog moves.

`image.py` scores every model in `router-model-catalog.yaml` against the matched
intent's weights and picks deterministically (§4 of `docs/design-spec.md`). This
file is *what the work needs*; the catalog is *which model delivers it*.

## Dimensions (fixed vocabulary — shared with the catalog)

`photorealism` · `typography` · `composition_control` · `speed`

To add a dimension, add it here **and** to every model's `scores` block in
`router-model-catalog.yaml` in the same change.

## Weights

`low` → 0 · `medium` → 1 · `high` → 2. A `high` requirement that a model is
`weak` (-1) at disqualifies that model.

## Sizes

`preferred_sizes` are literal `WxH` strings — the router's actual `size`
parameter, ordered. `image.py` picks the first entry the chosen model allows
(its `size_min_px`); if none qualify, it upsizes to the smallest valid
dimensions at that aspect ratio. There is no "resolution tier"; the plan line
shows orientation + dimensions ("Portrait · 1024×1536").

---

## intents

```yaml
- intent: launch / announcement image
  requirements: { photorealism: high, typography: medium, composition_control: high, speed: medium }
  preferred_sizes: ["1024x1536", "1024x1024"]

- intent: profile / avatar
  requirements: { photorealism: high, typography: low, composition_control: medium, speed: medium }
  preferred_sizes: ["1024x1024"]

- intent: social post
  requirements: { photorealism: medium, typography: medium, composition_control: medium, speed: high }
  preferred_sizes: ["1024x1280", "1024x1024"]

- intent: blog hero / article cover
  requirements: { photorealism: medium, typography: low, composition_control: high, speed: medium }
  preferred_sizes: ["1536x1024", "1280x1024"]

- intent: poster / flyer
  requirements: { photorealism: medium, typography: high, composition_control: high, speed: low }
  preferred_sizes: ["1024x1536"]

- intent: infographic / diagram
  requirements: { photorealism: low, typography: high, composition_control: high, speed: low }
  preferred_sizes: ["1024x1536", "1024x1024"]

- intent: illustration / concept art
  requirements: { photorealism: low, typography: low, composition_control: medium, speed: medium }
  preferred_sizes: ["1536x1024", "1024x1024"]

- intent: product / e-commerce shot
  requirements: { photorealism: high, typography: low, composition_control: high, speed: medium }
  preferred_sizes: ["1024x1024", "1024x1280"]

- intent: generic
  requirements: { photorealism: medium, typography: low, composition_control: medium, speed: medium }
  preferred_sizes: ["1024x1024"]
```

`generic` is the fallback when the agent cannot confidently classify the prompt.
