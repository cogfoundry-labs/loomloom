# `plan.json` (the Creative Plan)

Written by the agent (or by hand) and validated by `image.py plan`. A worked example that
the tests use: `tests/fixtures/plan-mighty.json`. Design: `docs/design-v2.md` section 6.2.

```json
{
  "schema_version": 1,
  "brief": "Product photos of the refill bottle on a bathroom counter",
  "intent": "product / e-commerce shot",
  "references": [
    {"id": "ref1", "file": "refs/product.png", "role": "product", "contains_person": false}
  ],
  "consent_acknowledged": null,
  "fixed": {"product identity": "the bottle in the reference image, unchanged"},
  "prompt": {
    "text_prefix": "...", "reference_prefix": "...",
    "text_suffix": "...", "reference_suffix": "..."
  },
  "dimensions": {
    "camera": [
      "eye-level front",
      {"value": "low angle",
       "fragment": "Extreme low angle, the camera on the counter pointing sharply upward",
       "fragment_with_reference": "Low camera position at counter level, looking slightly upward",
       "reliability": {"reference": "unreliable"}}
    ],
    "lighting": ["soft daylight", "golden hour"],
    "aspect": ["1:1", "4:5"]
  },
  "constraints": [
    {"exclude": {"camera": "low angle", "lighting": "golden hour"}}
  ],
  "model_strategy": "single"
}
```

## Fields

| Field | Required | Meaning |
|---|---|---|
| `schema_version` | yes | `1` |
| `brief` | yes | what is being made, in a sentence |
| `intent` | yes | exactly one of the intent names in `references/generation-policy.md` (anything else is refused); drives model scoring |
| `references[]` | no | `id` (text, unique), `file` (a relative path inside the plan's folder, never absolute or with `..`; copied into the experiment), `role` (`product`, `person`, `style`, `general`), `contains_person`. The role sets the default wording: product/person keep identity or likeness, style borrows palette and mood only, general keeps the main subject. One reference is sent per row; a row picks one in its Reference cell |
| `consent_acknowledged` | when a person is pictured | an ISO timestamp (anything else counts as no consent); written by `image.py acknowledge-person` after the user confirms the notice. `plan`, `preflight` and `run` refuse without it, and such an experiment is never published |
| `fixed` | no | things that never change; used to build the default text prompt |
| `prompt` | no | `text_prefix`, `reference_prefix`, `text_suffix`, `reference_suffix`. Defaults: the prefix is built from `fixed` (text mode) or a role sentence ("use the reference only to identify the {role}...") followed by "Keep this the same in every image: " and your Fixed items (reference mode); the closing sentence is "Realistic, high quality.", or the neutral "High quality." when the plan has a `style` dimension. Set them yourself when a default does not suit (an illustration style, a very specific scene) |
| `dimensions` | yes (a name may not be `id`, `selected`, `model`, `qty`, `take`, `reference`, `prompt`, `status`, `images`, `image`, `file`, `cost` or `notes`: those are workbook columns or the model choice) | dimension name -> non-empty list of values. A value is a string, or `{value, fragment, fragment_with_reference, reliability}` |
| `visual_checks[]` | no | up to 6 short sentences naming requirements the models follow only loosely and nothing can verify before images exist ("no text anywhere in the image", "empty space on the left for a headline", "the label text matches the reference"). After a batch the agent looks at every image and reports which fail; they are also printed by the dry run and written into the workbook's Read me |
| `constraints[]` | no | `{"exclude": {dim: value, ...}}`: a combination containing all of these pairs is never generated |
| `quality` | no | the model's quality setting, sent with every request (for GPT Image 2.5 Sunburst/Flare: `low`, `medium`, `high`, `xhigh`, `max`; gpt-image-2: up to `high`; Gemini and Seedream models have none, and a plan that names such a model with a quality is refused). Omit or `auto` to let the model choose: then one request shape can bill two different prices (measured: $0.030 or $0.055 per image). An explicit quality has its own observed price, so the preflight shows it as **unverified** until one image has been billed; run with `--max-usd` and the first image sets the price for the rest |
| `wildcard`, `relaxes` (on a dimension value object) | no | marks the one value that deliberately leaves a rule of the brief: `{"value": "...", "fragment": "...", "wildcard": true, "relaxes": ["the off-white foundation"]}`. `relaxes` requires `wildcard: true`, and **at most one value in the whole plan** may be flagged (`check` refuses more). The dry run prints it. They are not part of the prompt |
| `traits` (on every value of a direction dimension) | no | `{"ground", "medium", "layout", "type", "density", "palette"}`, short text each, describing the picture at thumbnail size. When every value of a dimension has them, `check` refuses a plan in which two values differ in fewer than three of the six, or more than two share a palette. The dry run prints them. They are not part of the prompt |
| `batch_by` | no | name of a dimension (for example `territory`): the first batch is split evenly across its values and each value gets its own covering design. Use it when one dimension is a hierarchy that excludes different values of the others (three campaign territories); without it a plain pairwise design can be badly unbalanced across that dimension (8 / 16 / 7 rows in one measured plan) |
| `takes` | no | how many images each ticked row of the first batch gets, 1 to 3 (default 1). A row is always exactly one image: a second image of the same values is a second row with the next `Take` number (r001 Take 1, r002 Take 2). Leave it at 1 for a first exploration; add takes later to the rows that look promising (`add-takes`, or copy the row in the workbook and change Take) |
| `model_strategy` | no | `single` (default: the Advisor's best fit for every row), `spread` (the top 2 models become a dimension), or `fixed:<model id>`. A row's `Model` cell overrides it |

## Wording

A plain string value uses the controls catalog's wording if there is one
(`references/controls-catalog.json`; `image.py controls` lists it, with how far each value was tested and a scope tag for wording written for a product on a surface or for a portrait), otherwise "{value} as the {dimension}".
Rows using that fallback are flagged as custom at preflight. Give an object value with a
`fragment` to control the wording. Add `fragment_with_reference` when the text-mode wording
misleads the model with a reference photo. Measured in M0: with a reference, strong camera
wording makes the model copy-tilt the product, so use a milder phrase and mark values that
still do not work as `"reliability": {"reference": "unreliable"}` (preflight then warns).
Lighting wording that names an unmistakable cue (for example "a strong warm orange color
cast") works in both modes; mild wording is invisible.

## Dimensions and size

- 2-4 variable dimensions (a single-value `aspect` does not count; one with several values does) with 3-4 values each (2 when binary or when only two are safe); about 150 valid combinations is already wide, and over 1,000,000 is
  refused.
- `aspect` is optional and takes ratios such as `1:1`, `4:5`, `16:9`. Only the Gemini models
  take a real ratio; others get the nearest supported size, and preflight shows both.
- Values are matched by their text, so keep them unique within a dimension.

## What the plan does not hold

Selections, quantities, models per row, status and costs live in `ledger.json` and the
workbook, not in the plan.
