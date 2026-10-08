---
name: direction
description: Image Lab 2 experiment mode, stage 2a (Creative Direction). Turns one rich brief into 3 to 10 genuinely different creative directions (default 4) that serve the same product, audience and objective, specifies each one richly, and plans the variation inside each, then hands a Direction Sheet to the plan stage. Judgment only; scripts compute the matrix, the workbook and every price.
---

# direction

The planner (`skills/plan.md`) varies **within an idea**. This stage generates **the ideas worth varying**.

> Creative Direction expands the user's creative search space. It never replaces the user's starting idea.

Nothing here spends money. Your output is a **Direction Sheet** (shown to the user, then turned into `plan.json` by the plan stage).
Read `../references/plan-schema.md` for the plan shape. Run commands from the skill folder (`PYTHONUTF8=1 python scripts/image.py ...` on Windows).

## When this stage runs

Decided by SKILL.md (routing). In short: the user said explore, directions, concepts, campaign or ideas; or the brief is creatively
open-ended and the user chose "Explore creative directions" when asked once. If the user said "generate exactly this" or the prompt is a plain
single image, this stage does **not** run.

## Principles

1. **Your user's idea stays.** Direction 1 is their brief (every instruction kept, compressed where needed); the others are alternatives around it.
2. **Their copy is invariant** in every direction: every word they asked for, no new slogans.
3. **Richly specified, not necessarily long.** Resolve the creative decisions that matter for this brief; write N/A for the rest.
4. **Real diversity.** A direction counts only if its defining choices change the *picture* (medium, setting, composition, light, energy), not just adjectives.
5. **Say what you assumed.** Keep what the user said apart from what you inferred.

## Step A. Read the brief

Show one short table with four labels:

| Label | Meaning | What you do with it |
|---|---|---|
| **Said** | facts and constraints the user stated: brand, product, audience, objective, mandatory words, aspect, "never" rules | goes into the shared opening and closing of every prompt; never varied |
| **Inferred** | your interpretation of the brief | shown on the direction cards; editable |
| **Brand assumptions** | general knowledge about the brand or audience used to make a direction appropriate, **always labelled as an assumption** | shown for accept or reject; never presented as fact |
| **Open gaps** | what the brief does not decide | stated as assumptions the user can correct |

Collect the brief's own lists (colors, poses, framings, styles it names) as candidate values for Step D.

## Step B. Choose the directions

- **3 to 10, default 4.** Offer fewer if the brief honestly supports fewer; never pad with near-duplicates. Two only on the user's explicit request.
- **Direction 1 is the user's brief, kept faithfully.** A long brief cannot go in word for word, so apply the **coverage rule**: keep **every instruction**,
  compressed where needed, and keep **exact quotes for names, copy, numbers, proportions and "never" rules**. Direction 1 may be longer than the other
  directions (the dry run shows the prompt length against the model limit). Build a **coverage list**: the brief's named and numeric specifics (colors,
  proportions, counts, positions, ages, "never" rules) and where each one lives in the plan. Show how many are kept and say what was compressed. **Never write
  "exactly as you wrote it"** unless it is word for word.
- The other directions **contrast** with it and with each other, but they **keep every hard constraint of the brief**: its "never" rules, color foundation, crop and
  pose rules, aspect, product and copy. Contrast on **medium, setting, light, composition, energy, density and typography**, not on the rules. A night
  photograph, a flat color field or a print can all stay inside a black, white and off-white foundation with one accent.
- **Only the wildcard may relax a rule.** From 4 directions up you may add **one** wildcard: a direction that deliberately leaves one rule of the brief (for example its
  palette or setting). Mark it in the plan data (`"wildcard": true, "relaxes": ["the rule it leaves"]` on its value; `check` refuses more than one flagged value
  in the whole plan) and on its card as **"wildcard, relaxes: ... (needs your OK)"**. No other direction relaxes anything. If you are tempted to relax a rule to make a
  direction different enough, change its medium or setting instead.
- At least one direction deliberately contrasts with the user's own style (within the hard constraints).
- **Look traits (the distinctness check, enforced by `check`).** For every direction write six traits, in a few words each, describing the picture at
  thumbnail size: **ground** (the surface or background), **medium** (photograph, print, collage, illustration, graphic), **layout** (where the subject, camera
  and negative space sit: viewpoint, subject placement, how much empty space), **type** (headline scale and placement), **density** (how many graphic elements)
  and **palette**. Put them in the plan as `traits` on each direction value. `check` refuses a plan where **any two directions differ in fewer than three of the
  six**, or where **more than two share a palette**. A direction that only changes the ground and the color of the same poster (cutout runner, stacked headline
  on the left, brush shape behind) fails on layout, type and density, which is exactly the failure to avoid.
- **Contrast the picture structure, not just the surface.** At most **one** direction may be "a cutout runner on a poster ground with a stacked headline". Where the
  brief allows, include at least one **full-bleed photograph with the subject in the scene** (no cutout, small or integrated type), and at least one with **large
  negative space** (a third of the frame or more) and a quiet, small headline. Directions should differ in viewpoint, subject scale, headline scale and graphic
  density as much as in color.
- **Each direction owns its palette.** No palette is shared by more than two directions. If the brief names accent colors, give each direction a different dominant
  one and use the others as the within-direction color lever (Step D).
- **Five feel axes (an extra guardrail, not proof):** medium or treatment, setting, energy, graphic density, typographic voice. Aim for at least 3 axes of
  difference between any two directions (2 above six directions).
- Each direction gets one line saying **which brand or audience insight it serves**, labelled **assumption** when it comes from general knowledge.
- Do not invent facts about a brand. Do not add logos, slogans or text beyond what the brief names.
- Above 6 directions, show a one-line-per-direction map first.

## Step C. Specify each direction (adaptive completeness)

For each direction resolve every item that matters for this brief and write **N/A** for the rest:

idea (one line) · subject and action · visual treatment (medium, finish, texture) · composition · color and light · relevant graphic or typographic
treatment · emotional intent · differentiation from the other directions · **keep out**

**Keep out is always required.** One sentence naming what this direction must not contain (for example "no city skyline inside the collage: the pieces are
paper, ink and texture"). Without it, directions leak into each other.

**Write concise, visual wording.** Each direction states what the picture looks like, in the fewest concrete phrases; a long list of ingredients ("hairline
grid, registration marks, measurement ticks, halftone dots...") reads as vague. A portrait brief needs no graphic system or exact typography; a campaign key
visual does. The criterion is "every relevant decision resolved", not a count.

**Coherence check.** After writing each direction, reread it for self-contradiction (a coarse halftone runner next to a sharp, detailed runner; a flat
minimal field next to a layered collage; "no people" next to a subject who is a person). Fix it before moving on.

## Step D. Variation inside each direction

A direction's **defining traits are fixed**; they live in its own wording. Vary only where variation does not weaken the direction.

- Each direction needs **at least three variable levers**, one of them **composition or layout**, and **at least one lever with three or more values**.
  The composition lever's values must change where the subject, the camera and the type sit (for example centered break, diagonal flow, edge-framed, deep
  perspective), not mirror the same layout; the first test's images all had the headline stacked on the left because no direction had a layout lever.
- **Layout values belong to the direction.** Give each direction its **own** composition values, written as physical, spatial instructions (where the camera is, how big the
  subject is, where the type physically sits: "stencilled on the ground in perspective", "the shoe huge in the foreground", "headline filling the frame with the runner small
  in a lower corner"). A composition lever shared by every direction ("rising diagonal" for all) gets drowned by the shared opening and produces the same poster in each: in
  the second paid test the plan with a shared layout lever rendered three of four directions alike, while the plan with direction-specific layouts rendered five different pictures.
- If the brief names colors, add a **color lever** (the brief's own colors) inside each direction.
- Variation must be **visible at thumbnail size**: if two values of a lever would produce nearly the same picture, replace one.
- Levers come from the brief's own lists first, then a standard set: composition, movement or pose, color accent, graphic language, typography treatment,
  texture or light. Give values wording that states the visible result, with the proportions the brief uses (negative space, element counts, type scale); strong
  concrete cues are seen, mild ones are not.
- Mark where a value belongs to **one direction only**. Until value scoping exists, express this as `constraints` (`exclude` of a value in the directions it
  does not belong to). Every excluded pair must name values that exist.
- Keep the total workable: the plan may have more than four variable dimensions and more than 150 valid combinations here (the direction dimension multiplies
  the rest), and the batch is chosen per direction (below). The hard limits are the validity ones (`check`), not the planner's usual four.
- A single-value `aspect` is not a variable and does not count.

**Fake-diversity guard.** If two directions would compile to the same camera, pose, framing, background and hierarchy, merge them or change what defines them. The `traits` check catches the obvious cases; also ask whether the six images of one direction would still look related but not identical.

## Step E. Write `plan.json`

- Dimension `direction` first, one value per direction. Each value's `fragment` is that direction's full wording (idea, treatment, composition, light,
  graphics, typography, keep out), written as a plain description of the visible result. For Direction 1 apply the coverage rule (Step B): every instruction kept, exact quotes for names, copy, numbers and "never" rules.
- The **shared opening** goes in `prompt.text_prefix`: the Said items (brand, product, audience, objective, quality bar, "no other brand"). The **shared closing**
  goes in `prompt.text_suffix` and **lists the only allowed text** (the user's copy, small numerals or data marks if the brief wants them) followed by
  "do not add any other words, slogans or taglines".
- Every direction value carries its **`traits`** (six keys, Step B); `check` enforces the differences.
- **One runner for the campaign.** If the brief does not say who the runner is, choose one description (age range, build, hair, apparel) and put it in the shared opening so the
  campaign has one cast; state the choice under Open gaps. Vary the runner only if the user asks for variety.
- Flags live in the plan data: the wildcard value carries `wildcard: true` and `relaxes: [...]`, so anyone reading `plan.json` sees them.
- Then the variation dimensions (Step D) with `fragment` wording; `aspect` as a single ratio if the brief fixes one.
- `batch_by: "direction"` so the first batch is split evenly and each direction gets its own covering design. Choose the batch size with
  `plan --target max(30, 4 x directions)`; each direction needs at least 4 images.
- `intent`: pick from the intent list by what the image is for. `model_strategy`: `single`. `quality`: set it (see plan.md) so cost is predictable.
- `fixed`: record the Said items for the user's reading (the compiler uses `prompt.text_prefix`).
- `visual_checks` (at most six): the copy is spelled exactly with no other text; the product is clearly visible and plausible; the subject's anatomy; each
  direction is unmistakable from its description; graphics and type never hide the subject; the variation levers match what was asked.
- Run `check` and `plan --dry-run` as in plan.md, read the sample prompt, and read the dry run's `Why` line (the per-direction shares).

## Step F. The one confirmation

One message with: the **Said / Inferred / Brand assumptions / Open gaps** table; the **direction cards** (name, one-line idea, the insight it serves, what makes it
different, keep out, the copy, and the one "wildcard, relaxes: ... (needs your OK)" mark if there is a wildcard); the **traits table** (ground, medium, layout, type, density and palette per direction, from the plan's `traits`); the **Direction 1 coverage line** (how many of the brief's specifics are kept and what was compressed); for each direction **what is fixed and what varies**; the dry-run numbers (images per direction, model, size, price basis,
limits); and that **the first spend is a calibration with one image per direction**, which doubles as the check that the directions really look different.
Say plainly that image models follow wording loosely, so the first batch is how you find out. Ask for **one** confirmation or edits, in one turn.
Show the full lever matrix only if asked.

## What not to do

- Do not replace the user's idea, change their copy, relax a hard constraint in any direction except the one marked wildcard, or invent brand facts.
- Do not write directions that differ only in ground and color while sharing layout, type and density, or that share a palette beyond two.
- Do not claim a direction is "exactly as you wrote it" unless it is word for word.
- Do not pad to the requested count with near-duplicates.
- Do not leave a keep-out line empty.
- Do not edit `experiment.xlsx`, put a price in the plan, or run `acknowledge-person` yourself.

## A worked example (different domain)

Brief: *"Hero image for the autumn launch of a small-batch coffee roaster: warm, honest, a bag of beans on a wooden counter. Headline: SLOW MORNINGS. Portrait."*

| Direction | Idea | Differs on | Keep out |
|---|---|---|---|
| 1 Your brief | a warm still life, bag of beans on a wooden counter, morning light | the baseline | no people, no steam clichés |
| 2 Process | the craft: hands, scoops, a roaster, close and tactile | subject (hands), setting (workshop), energy | no finished retail shelf |
| 3 Ritual | a quiet pour-over moment in a home, window light | setting (home), energy (slow), subject (a person) | no bag as the hero |
| 4 Graphic | flat-lay, bold color blocks, large headline | medium (graphic), density, typography | no wooden counter |

Each direction then varies two or more levers (for example light, framing, color accent), and the batch is split evenly across the four.
