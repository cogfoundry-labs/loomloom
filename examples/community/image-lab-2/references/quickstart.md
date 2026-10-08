# Quickstart: your first plan in one page

Read this, copy an example, run the checker. `skills/plan.md` and `skills/direction.md` are the full rules; you do not need them for a first plan, and the checker
tells you when you break one.

## 1. Pick the route (one line to the user)

| The user asks for | Route | Start from |
|---|---|---|
| one finished prompt, a few options, no photo, no variation | **quick mode**: no plan | `skills/quick.md` |
| to vary something (lighting, camera, style, aspect), or a reference photo, or more than 8 images | **planner**: a plan with 2 to 4 variable dimensions | `references/examples/variation-plan.json` |
| to explore directions, concepts, ideas or a campaign from an open brief | **Creative Direction**: 3 to 10 directions (default 4), the user's idea as Direction 1 | `references/examples/direction-plan.json` |

Doubt between quick and planner: the user's words decide ("explore", "vary", "different X" or a photo means a plan). An open brief with none of those words: ask once,
"Generate as written" or "Explore creative directions".

## 2. The loop (nothing here spends money until `run`)

Run from the skill folder; on Windows set `PYTHONUTF8=1` (PowerShell: `$env:PYTHONUTF8=1`).

```
python scripts/image.py check   --plan plan.json                       # is the plan valid? errors say what to fix
python scripts/image.py plan    --plan plan.json --out out/<name> --dry-run   # size, wording, price basis, warnings; writes nothing
python scripts/image.py plan    --plan plan.json --out out/<name>      # builds the experiment and experiment.xlsx (still free)
python scripts/image.py preflight --dir out/<name>                     # prices the ticked rows; prints a fingerprint
python scripts/image.py run --dir out/<name> --confirm <fingerprint> --max-usd <N>   # the ONLY step that spends; after the user's one approval
```

For creative directions, price **one image per direction first**: `preflight --dir out/<name> --one-per direction`, run it, look
(`montage --dir out/<name> --by direction`), then a plain `preflight` for the rest. A token (`LOOMLOOM_TOKEN_COGFOUNDRY`) is needed only for `run`.

## 3. The plan file in 30 seconds

Copy the example and change it. The fields that matter:

- `brief`, `intent` (one of the ten names in `references/generation-policy.md`; pick by what the image is *for*), `fixed` (what must never change, one phrase each).
- `dimensions`: name -> list of values. A value is a string (the starter wording from `python scripts/image.py controls` is used) or `{"value": ..., "fragment": "what the picture looks like"}`.
  Give a `fragment` for anything not in `controls`; the dry run says which values have no wording.
- `constraints`: `{"exclude": {dimension: value, ...}}` for combinations that make no sense. `aspect` takes ratios (`16:9`) and no wording.
- `visual_checks` (at most 6): what the models follow only loosely (no text, headline space, spelling); you look at every image against them after the first batch.
- `quality` (for example `medium`) so the cost is predictable; `model_strategy: "single"`.
- A reference photo goes under `references[]` (a relative file path, `role`, `contains_person`); a **person** needs the notice and the user's yes (SKILL.md, step 1).

The judgment you supply (no script can): the intent, what is fixed, which 2 to 4 things are worth varying with 3 or 4 concrete, visibly different values each, and wording that
says what the picture shows. Strong cues are seen, mild ones are not ("a strong warm orange cast", not "warm light").

## 4. What the checker enforces for you

You do not need to memorize these; `check` and the dry run refuse or warn:

- a valid shape; known intent; no reserved dimension names; values that exist in every constraint;
- more than 1,000,000 combinations (refused) and about 150 valid combinations or more than 4 variable dimensions (warned; creative directions are exempt);
- wording missing for a value; wording not yet tested; camera wording that makes a model copy a reference photo's pose;
- a person photo without consent; a prompt longer than a model accepts;
- for directions: every direction needs `traits` (ground, medium, layout, type, density, palette) and any two must differ in at least 3 of the 6, at most 2 may share a palette;
  at most one value may be a `wildcard` (it must say what rule it `relaxes`); `only_in` scopes a value to some directions; `batch_by: "direction"` splits the first batch evenly.

## 5. For Creative Direction, add these judgments

- Direction 1 is the user's own brief, every instruction kept; the user's copy is the same in every direction. The closing sentence (`prompt.text_suffix`) lists **every**
  piece of text the user gave (a headline **and** a date line, say) and says "do not add any other words"; the example lists one headline, so extend it.
- A shared lever (accent color, headline style) must make sense in **every** direction. Do not offer "small quiet letters" when one direction is defined by huge letters, or an accent that
  one direction already fixes: scope the value with `only_in` or leave the lever out. The checker cannot see this contradiction; reread each direction against each lever.
- The other directions change the **picture** (medium, setting, light, composition, type scale), not just the adjectives, and keep every hard constraint of the brief.
  Only the one wildcard may relax a rule.
- Give each direction its own spatial layout values (`only_in`), its own palette, and a keep-out sentence (what it must not contain).
- Show the user one message: what you took from the brief (said / inferred / assumed / open gaps), the direction cards, the dry-run numbers, and that the first spend is
  one image per direction. Ask for one confirmation.

## 6. The one confirmation

Before any spend, one message: what will be made, the models and size, the images, the price **with its basis** (a known total, or "unverified" plus the indicative range and
`--max-usd` the preflight prints), and the limits. Then `run --confirm <fingerprint>` only after a clear yes. A changed plan or workbook needs a new preflight and a new yes.

## 7. Read more only when you need to

| You need | Read |
|---|---|
| the full planner rules, reference photos, person consent | `skills/plan.md` |
| the full Creative Direction method | `skills/direction.md` |
| every plan field | `references/plan-schema.md` |
| quick mode | `skills/quick.md` |
| the rules that always apply (approval, Unknown outcomes, spend limits) | `SKILL.md`, "Rules that always apply" |
| retry, recover, takes, the contact sheet | `SKILL.md`, "Experiment mode" steps 6 to 8 |
