# Quickstart: your first plan in one page

Read this, copy an example, run the checker. `skills/plan.md` and `skills/direction.md` are the full rules; you do not need them for a first plan, and the checker
tells you when you break one.

## 1. Pick the route (one line to the user)

Three modes answer three different questions. Decide on the user's **intent and constraints**, not on single words; first match wins.

| # | The user asks for | Route | Question it answers | Start from |
|---|---|---|---|---|
| 1 | to explore directions, concepts, campaign ideas or different looks for a brief | **Creative Direction**: 3 to 10 directions (default 4), the user's idea as Direction 1 | what different visual ideas can this brief become? | `references/examples/direction-plan.json` |
| 2 | to control or compare variables (lighting, camera, style, aspect), **or** a reference photo, **or** more than 8 images (Quick can do neither) | **Variation**: a plan with 2 to 4 variable dimensions | which variables change the picture, and how? | `references/examples/variation-plan.json` |
| 3 | an open-ended brief (campaign or brand language), without saying whether to explore | **ask once**: "Generate as written" or "Explore creative directions" | | |
| 4 | anything else: one finished prompt, or a few options of a concrete scene | **Quick**: no plan | the same prompt: how do I get a few good options fast? | `skills/quick.md` |

"Different" alone is weak evidence ("8 different product shots", no photo, is Quick; the approval message says Quick varies the model, not the composition). Variation changes one
thing at a time to learn what matters; Creative Direction changes the whole idea to see what is possible. They share one engine (the plan file, the workbook, preflight).

## 2. The loop (nothing here spends money until `run`)

Run from the skill folder; on Windows set `PYTHONUTF8=1` (PowerShell: `$env:PYTHONUTF8=1`).

```
python scripts/image.py check   --plan plan.json                       # is the plan valid? errors say what to fix
python scripts/image.py plan    --plan plan.json --out out/<name> --dry-run   # size, wording, price basis, warnings; writes nothing
python scripts/image.py plan    --plan plan.json --out out/<name>      # builds the experiment and experiment.xlsx (still free)
python scripts/image.py preflight --dir out/<name>                     # prices the ticked rows; prints a fingerprint
python scripts/image.py run --dir out/<name> --confirm <fingerprint> --max-usd <N>   # the ONLY step that spends; only after the user approves THIS batch
```

For creative directions, price **one image per direction first**: `preflight --dir out/<name> --one-per direction`, run it, look
(`montage --dir out/<name> --by direction`), then a plain `preflight` for the rest. A token (`LOOMLOOM_TOKEN_COGFOUNDRY`) is needed only for `run`.

## 3. The plan file in 30 seconds

Copy the example and change it. The fields that matter:

- `brief`, `intent` (one of the ten names in `references/generation-policy.md`; pick by what the image is *for*), `fixed` (what must never change, one phrase each).
- `dimensions`: name -> list of values. A value is a string (the starter wording from `python scripts/image.py controls` is used) or `{"value": ..., "fragment": "what the picture looks like"}`.
  Give a `fragment` for anything not in `controls`; the dry run says which values have no wording.
- `constraints`: `{"exclude": {dimension: value, ...}}` for combinations that make no sense. `aspect` takes ratios (`16:9`) and no wording.
- `visual_checks` (at most 6): things you can **see and check on each image** that models follow only loosely (spelling, text present or absent, headline space, the product visible).
  After the first batch you review every image against them (text on the full-size image, not a thumbnail) and report pass, fail or can't tell, as your own reading. Subjective qualities (premium, distinctive) are for the user's eyes, not pass or fail.
- `quality` (for example `medium`) so the cost is predictable; `model_strategy: "single"`.
- A reference photo goes under `references[]` (a relative file path, `role`, `contains_person`); a **person** needs the notice and the user's yes (SKILL.md, step 1).

The judgment you supply (no script can): the intent, what is fixed, which 2 to 4 things are worth varying with 3 or 4 concrete, visibly different values each, and wording that
says what the picture shows. Strong cues are seen, mild ones are not ("a strong warm orange cast", not "warm light").

## 4. What the checker enforces for you

You do not need to memorize these; `check` and the dry run refuse or warn:

- a valid shape, known intent, no reserved dimension names, values that exist in every constraint; more than 1,000,000 combinations (refused);
- **Variation only:** more than 4 variable dimensions, more than 4 values in a dimension, about 150 valid combinations. Creative directions are exempt from those shape limits, **not from the
  workload warnings every plan gets**: a first batch over 60 images (rows x takes), a known cost over $2, more than 24 images with no verified price; for directions also fewer than 3 or
  more than 10 directions, or fewer than 4 images for one;
- wording missing or untested, camera wording that makes a model copy a reference photo's pose, a person photo without consent, a prompt longer than a model accepts;
- **directions:** `traits` on every direction (ground, medium, layout, type, density, palette; any two differ in at least 3, at most 2 share a palette), at most one `wildcard` (it names the rule
  it `relaxes`), `only_in` to scope a value to some directions, `batch_by: "direction"` to split the first batch evenly.

## 5. For Creative Direction, add these judgments

- Direction 1 is the user's own brief, every instruction kept; the user's copy is the same in every direction. The closing sentence (`prompt.text_suffix`) lists **every**
  piece of text the user gave (a headline **and** a date line, say) and says "do not add any other words"; the example lists one headline, so extend it.
- A shared lever (accent color, headline style) must make sense in **every** direction. Do not offer "small quiet letters" when one direction is defined by huge letters, or an accent that
  one direction already fixes: scope the value with `only_in` or leave the lever out. The checker cannot see this contradiction; reread each direction against each lever.
- The other directions change the **picture** (medium, setting, light, composition, type scale), not just the adjectives, and keep every hard constraint of the brief.
  Only the one wildcard may relax a rule.
- Give each direction its own spatial layout values (`only_in`), its own palette, and a keep-out sentence (what it must not contain).
- Show the user one message: what you took from the brief (said / inferred / assumed / open gaps), the direction cards, the dry-run numbers, and that the first spend is
  one image per direction. Ask for one **plan confirmation** (spends nothing).

## 6. Approval: one per paid batch, one snapshot each

Two different yeses: a **plan confirmation** (this plan or Direction Sheet is what the user wants; spends nothing) and a **spend approval** (one fingerprint). Do not mix them.

Before each paid batch, one message: what will be made, the models and size, the images, the price **with its basis** (a known total, or "unverified" plus the indicative range and `--max-usd`
the preflight prints), the limits, and **the scope**: "This approves exactly these N images (about $X, fingerprint ...). It does not approve anything else." Then `run --confirm <fingerprint>`
only after a clear yes.

- A **calibration** (`--one-per direction`) is its own batch with its own approval; the **main batch** is quoted again (a plain `preflight`) and approved again after the user has seen the images.
- A changed direction, selected rows, model, size or prompt is a new quote, a new fingerprint, a new approval. An approved snapshot never changes: Excel edits saved afterwards are not in that run,
  and `run` names each row and field that differs (by content, not file time).
- To change **wording** after the build (a direction that came out wrong), edit `out/<name>/plan.json` and `preflight`; rows that already have their image keep it. New values or directions need a new experiment.
- `run --confirm` is the **only** command that spends. `retry`, `add-takes` and `recover` never spend by themselves (new quote and approval, or only re-downloading images already billed).
  `--max-usd` limits new submissions; it is **not a hard cap**, because requests already in flight still bill.

## 7. Read more only when you need to

| You need | Read |
|---|---|
| the full planner rules, reference photos, person consent | `skills/plan.md` |
| the full Creative Direction method | `skills/direction.md` |
| every plan field | `references/plan-schema.md` |
| quick mode | `skills/quick.md` |
| the rules that always apply (approval, Unknown outcomes, spend limits) | `SKILL.md`, "Rules that always apply" |
| retry, recover, takes, the contact sheet | `SKILL.md`, "Experiment mode" steps 6 to 8 |
