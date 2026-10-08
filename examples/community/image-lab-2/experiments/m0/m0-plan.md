# M0 — Controls experiment (go/no-go)

Status: **plan for review; nothing generated yet.** Gate defined in
`docs/design-v2.md` sections 14 and 16: M1 does not start until M0 passes.

## 1. Question

Do the creative controls (Planner values compiled into prompt wording) produce
**reliably distinguishable outcomes**, consistently across models, while keeping
the **Fixed** attribute (the product) recognizably the same?

If not, fix the vocabulary and the compiler wording and re-run M0 before building
the matrix, workbook and preflight around it.

## 2. Test material

`ref/mighty-product.png`: the Mighty toothpaste refill bottle and mint dispenser,
downloaded from mightymouthcare.com, flattened from a transparent cut-out onto
white (720 × 932). It is a third-party brand asset: **internal test only. The
generated images are not to be published or shared**, and `ref/` is git-ignored.

## 3. Experiment design

One fixed element, two varied dimensions, three values each.

| | |
|---|---|
| **Fixed** | the product (reference leg: "the product in the reference, unchanged"); the setting, a pale stone bathroom counter; realistic commercial photograph; square 1:1 |
| **Dimension 1: Lighting** | `soft daylight` · `golden hour` · `dramatic spotlight` |
| **Dimension 2: Camera** | `eye-level front` · `three-quarter high` · `low angle` |

The full 3 × 3 grid is 9 rows. For two dimensions this equals the pairwise
minimum, so M0 also exercises the covering design's smallest case.

### Wording (what the compiler would use)

| Dimension | Value | Fragment |
|---|---|---|
| Lighting | soft daylight | Soft, even, diffused daylight from a window, gentle shadows |
| Lighting | golden hour | Warm low golden-hour sunlight from the side, long soft shadows, amber glow |
| Lighting | dramatic spotlight | Hard directional spotlight with strong contrast and deep dark shadows, moody |
| Camera | eye-level front | Eye-level front view, the camera straight on at the product |
| Camera | three-quarter high | Three-quarter view from slightly above, looking down at about 30 degrees |
| Camera | low angle | Low angle from near counter level, looking up at the product |

### Prompt templates

Text leg (no reference; the product is described in words):

> A clean commercial product photograph of a pastel mint-green pump dispenser
> bottle standing next to a cream-colored toothpaste refill bottle with a silver
> cap, on a pale stone bathroom counter. {camera}. {lighting}. Sharp focus,
> realistic, square composition, no text overlays.

Reference leg (the photo is sent as the reference image):

> Use the reference image as the exact product: keep the cream refill bottle with
> its teal "MIGHTY Toothpaste Refill" label and the mint-green dispenser unchanged
> in shape, color, branding and proportions. Place them on a pale stone bathroom
> counter. {camera}. {lighting}. Commercial product photograph, realistic, square
> composition.

## 4. Legs, models and cost

| Leg | Models | Images | Est. cost |
|---|---|---|---|
| **A. Text-only** | Nano Banana (`google/gemini-2.5-flash-image`, ~$0.0032) and GPT Image 2.5 Sunburst (`openai/gpt-image-2.5-sunburst`, ~$0.0069), 9 each | 18 | ~$0.09 |
| **B. Reference** | Nano Banana only (the one model with a measured reference price, ~$0.039 per image on an earlier test) | 9 | ~$0.35 |
| **Total** | | **27** | **~$0.44** |
| *C. Optional probe* | GPT Image 2.5 Sunburst with the reference, 1 image. **Price unmeasured**, expected to be higher than text-only. Also moves that model's reference state from `documented` toward `verified`. | 1 | unknown, small |

Nano Banana is sent `aspect_ratio: 1:1`. GPT Image 2.5 Sunburst has **no size or
aspect parameter at all**, so it renders at its own default (roughly square) and
"square composition" is only in the prompt. There is no seed control, so each image
is one random sample; the blind test accounts for that by using several samples
per value.

## 5. Blind test

1. All 27 images are shuffled with a fixed seed and renamed to neutral codes.
2. A workbook `m0-blind-test.xlsx` is built: a thumbnail per image and dropdown
   columns for `Lighting?`, `Camera?`, and (reference images only) `Product
   recognizably the same? (Yes / Partly / No)`, plus a Notes column.
3. The key (which image is which) is stored separately in `m0-key.json`; the
   reviewer does not open it until scoring.
4. The reviewer fills the workbook and saves it. The harness reads it back and
   scores it against the key.

This also exercises the real workbook stack: XlsxWriter writes, Excel opens and
saves it, openpyxl reads it back.

## 6. Go criteria (proposed thresholds, to confirm)

Chance level is 33% (three values per dimension).

| # | Criterion | Pass when |
|---|---|---|
| 1 | Each dimension is identified correctly | at least **80%** per dimension across all 27 images (22 of 27), and **no single cell** (a model/leg) below 6 of 9 |
| 2 | Differences correspond to the intended dimension, not a side effect | reviewer: changing lighting did not also change the scene or product, and camera did not change the lighting (unblinded grids) |
| 3 | Consistent across models | on the text leg the two models' accuracy per dimension is within 2 of 9 |
| 4 | The wording does not dominate or distort | reviewer: the product looks natural; no fragment forces odd artifacts |
| 5 | Fixed attributes stay fixed | reference leg: at least 8 of 9 rated Yes or Partly, and at least 5 of 9 Yes |

**Go** = all five pass. **No-go** = fix the failing vocabulary or wording (the
results show which fragment fails) and re-run only the affected part.

## 7. Steps

1. You review and approve this plan (values, wording, thresholds, cost).
2. I write the harness (no spend) and run a dry run that prints every request,
   the exact cost estimate and the legs.
3. You approve the spend with the final numbers. Generation runs.
4. The blind workbook is built; you fill it in Excel (about 10 minutes).
5. I score it, build unblinded comparison grids, and you give the qualitative
   verdicts for criteria 2 and 4.
6. Go or no-go is recorded in `docs/design-v2.md` section 17.

## 8. Notes and risks

- The reference leg tests one model only because reference pricing is known for
  that model only.
- Brand text on the refill may be garbled by the model; criterion 5 judges
  recognizability of the product, not letter-perfect text.
- 27 images is a small sample. It is enough for a go/no-go on whether the controls
  work at all, not for tuning wording.

## 9. Results, round 1 (2026-10-07)

27 images, actual cost **$0.7737** (estimate $0.4419; Nano Banana's price had risen
from $0.0032 to about $0.039 per image since the catalog was measured). No failed or
unknown tasks. Blind test filled in by the reviewer.

| Cell | Lighting | Camera |
|---|---|---|
| Nano Banana, text | 9/9 | 6/9 |
| GPT Image 2.5 Sunburst, text | 6/9 | 6/9 |
| Nano Banana, reference | 7/9 | 2/9 |
| **All 27** | **22/27 (81%)** | **14/27 (52%)** |

Confusions (intended -> answered):

- **Camera `low angle`**: 9/9 answered "eye-level front". The wording produced no
  visible low angle on any model or leg.
- **Camera `three-quarter high`**: 6/6 correct on the text legs, **0/3** with the
  reference: the reference photo's own pose overrode the camera wording.
- **Camera `eye-level front`**: 8/9 correct; one reference image (dramatic
  spotlight) came out as a top-down view, outside all three options.
- **Lighting `golden hour`**: 3/9 read as soft daylight. `soft daylight` 9/9;
  `dramatic spotlight` 7/9.
- **Product (reference leg)**: **9/9 Yes**. Identity preservation works.
- Reviewer note: one reference image had the two bottles' shadows pointing in
  opposite directions.

| # | Criterion | Result |
|---|---|---|
| 1 | each dimension >= 80%, no cell < 6/9 | **FAIL** (lighting 81% passes overall; camera 52%; reference camera cell 2/9) |
| 2 | differences match the intended dimension | **Mostly pass (reviewer)**: `low angle` is visible side by side but too subtle to identify blind. Exceptions: one spotlight image became top-down; with the reference, camera wording tilted the products instead of moving the camera |
| 3 | models within 2 of 9 | **FAIL** (lighting 9 vs 6) |
| 4 | wording does not distort | **Lighting wording: pass. Camera wording with a reference: fail.** In 6 of 9 reference images the products are tilted or floating, copying the reference photo's pose. "Wrong product" notes on the 18 text-only images are expected: with no reference the product comes only from the prompt's description |
| 5 | product stays recognizable | **PASS** (9/9 Yes) |

**Verdict: no-go on the current vocabulary**, per the rules fixed before the test.
Not a verdict on the approach: lighting mostly works, `three-quarter high` and
`eye-level front` work on text, and product identity is strong. The failures are
specific and fixable (section 10).

## 10. Proposed round 2 (for review, nothing run)

Re-run only the affected parts, as the plan says. Wording changes to test:

| Problem | Proposed change |
|---|---|
| `low angle` never visible | Stronger, unmistakable wording: camera placed on the counter surface pointing sharply upward, products towering over the lens, strong perspective foreshortening, ceiling visible behind |
| `golden hour` read as soft daylight 3/9 | Add explicit color and light cues: deep orange-amber cast over the whole scene, sun low on the horizon, glowing orange rim light |
| Reference photo's pose overrides the camera | Say so explicitly: "re-photograph the product from the camera angle below; do not copy the viewpoint or tilt of the reference image" |
| `dramatic spotlight` once produced a top-down view | State the light direction ("from the side at counter height") so it cannot be read as an overhead camera |
| Models disagree on lighting (9 vs 6) | Re-test with the new golden-hour and spotlight wording before judging |

Open question: add a fourth camera value `top-down`? The models produced one by
accident, so it is a natural value to offer.

Cost is set by a dry run before anything is spent. The reference leg on Nano Banana
alone is about $0.35 for 9 images at today's price.

### Added after reviewing the grids

- **The reference photo itself is tilted.** Both bottles float at an angle in the
  supplied photo, and in 6 of 9 reference images the model kept that pose instead
  of standing them on the counter; it also rotated the products rather than moving
  the camera. Round 2 should test an **upright** reference photo against this one,
  and add "the products stand upright on the counter; only the camera moves".
- **Text-only images vary the product shape** (tube, pouch or bottle; round or
  square dispenser) with no pattern across lighting or camera values. That is
  sampling variance with no reference, not distortion by the controls, and it is
  one more reason product work needs a reference.

## 11. Probe 1 results (2026-10-07, wording-only, 9 images, $0.1613)

Reviewed by eye, not blind (`out-probe1/probe1-sheet.png`).

| Test | Result |
|---|---|
| Text-only, Sunburst, `low angle` (strengthened) | **Works**: clearly looking up, ceiling and window visible, products towering, under both lightings |
| Text-only, Sunburst, `three-quarter high` (strengthened) | **Works**: clearly from above, counter top visible |
| Text-only, Sunburst, `golden hour` (color cues added) | **Works**: unmistakably amber, sun visible, clearly different from soft daylight |
| Reference, Nano Banana, `eye-level front` | Upright, labels legible, standing on the counter |
| Reference, Nano Banana, `three-quarter high` | Upright; the camera is only mildly higher |
| Reference, Nano Banana, `low angle` | **Still tilted and floating**: the model rotated the products despite "stand upright" |

Reading: the stronger camera and lighting wording fixes the text-only side, and the
"stand upright, only the camera moves" instruction fixes 2 of 3 reference images.
`low angle` with the reference is still read as "tilt the products", as it was in
round 1 (3 of 3 then). Next: step 2, an upright reference made from the supplied
photo, then re-test `low angle` on it, plus a milder `low angle` wording.

## 12. Probe 2 results (2026-10-07, 4 images, $0.1564)

- **2a, upright reference from the tilted photo on a plain white background
  ($0.0391): failed.** Nano Banana re-rendered the original floating, tilted pose
  almost exactly, despite "perfectly vertical, not tilted". It treats the reference
  as the composition to preserve.
- **2b, `low angle` re-test (3 images, $0.1173), using an upright scene image from
  probe 1 as the reference:**

| Case | Result |
|---|---|
| upright reference + strong wording | products upright; camera only slightly lower |
| upright reference + milder wording | nearly a copy of the reference; almost no camera change |
| **original tilted reference + milder wording** | **products upright, camera at counter level, labels legible** |

Findings:

1. The tilt in round 1 was caused by the **strong** low-angle wording ("pointing
   sharply upward, products tower above the lens"), which the model implemented by
   rotating the products. The milder wording ("low camera position at counter level,
   looking slightly upward at the standing products") keeps them upright.
2. A separate upright reference is **not needed**, and it makes the model copy the
   reference composition (almost no camera change).
3. **Wording must differ by mode.** Strong camera wording works without a reference
   (Sunburst, probe 1) and fails with one. The controls vocabulary needs a separate
   wording for reference runs.

Cumulative M0 spend: $0.7737 (round 1) + $0.1613 (probe 1) + $0.1564 (probe 2) = **$1.0914**.

## 13. Formal round 2 results (2026-10-07, 27 images, $0.5008)

Revised vocabulary; text-only on GPT Image 2.5 Sunburst and GPT Image 2.5 Flare
(the second model changed because Nano Banana 2 now costs $0.0672 per image, not
$0.006; Flare is the same family as Sunburst, so criterion 3 is a weaker test than
in round 1), reference on Nano Banana. Blind test, `out-r2/`.

| Cell | Lighting | Camera |
|---|---|---|
| Sunburst, text | 9/9 | 8/9 |
| Flare, text | 9/9 | 6/9 |
| Nano Banana, reference | 9/9 | 5/9 |
| **All 27** | **27/27 (100%)** | **19/27 (70%)** |

Camera confusions (intended -> answered):

- `eye-level front`: 9/9.
- `low angle`: 6/9. Works on both text models (6/6); **0/3 with the reference**
  (the milder wording is too subtle to see).
- `three-quarter high`: 4/9. **4 images were "none of the options"** (all on the text
  models): the strengthened wording ("about 35 degrees, counter top clearly visible")
  overshoots to a near top-down view. Round 1's gentler wording scored 6/6 here.
  One reference image was read as eye-level.
- Product (reference leg): 9/9 Yes. One note: the top of the product was shadowed
  under the side spotlight (expected from the lighting wording).

| # | Criterion | Result |
|---|---|---|
| 1 | each dimension >= 80%, no cell < 6/9 | **FAIL**: lighting 100%; camera 70% and the reference cell 5/9 |
| 3 | models within 2 of 9 | PASS (same-family pair) |
| 5 | product stays recognizable | PASS (9/9) |
| 2, 4 | reviewer | pending |

Two localized causes, both fixable by **per-mode wording**:

1. `three-quarter high` in **text** mode: go back to round 1's gentler wording
   (6/6 there) instead of the strengthened one.
2. `low angle` in **reference** mode: the strong wording tilts the products and the
   mild one is invisible; an intermediate wording is needed.

Lighting is settled. Cumulative M0 spend: $1.6666.

## 14. M0 verdict: conditional go (2026-10-07)

Reviewer verdicts on round 2: **criterion 2 much better; criterion 4 the same as in
round 1** (some products look wrong; in round 1 this was the text-only images, which
have no reference, plus tilted products with the strong camera wording).

| # | Criterion | Final |
|---|---|---|
| 1 | each dimension >= 80%, no cell < 6/9 | fail on one cell (reference camera 5/9); lighting 100%, camera 70% |
| 2 | differences match the dimension | **much better** (reviewer) |
| 3 | consistent across models | pass (same-family pair) |
| 4 | wording does not distort | **same as round 1** (reviewer): reference-mode camera wording and text-only product variance |
| 5 | product stays recognizable | pass, 9/9 |

**Decision (option B): conditional go.** The reviewer accepted proceeding to M1
without a third round. Reasons: lighting wording is settled (27/27); text camera
reaches 18/18 once `three-quarter high` is restored to its round-1 wording; product
identity is 9/9; and M1 (matrix, compiler, workbook, preflight) does not depend on the
camera wording.

Carried forward as open items for the controls catalog (M4), not forgotten:

1. `three-quarter high`: text mode = round-1 gentle wording; reference mode = stronger
   wording (subtle effect).
2. `low angle` with a reference is **unreliable** (strong wording tilts the products,
   mild wording is invisible). Needs an intermediate wording, and until then preflight
   must flag it (`camera "low angle" is unreliable with a reference image`).
3. Controls need per-mode wording (`fragment` / `fragment_with_reference`).

Total M0 spend: **$1.6666** over 27 + 9 + 4 + 2 + 27 images.
