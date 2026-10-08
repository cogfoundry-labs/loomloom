---
name: image-lab-2
license: Apache-2.0
description: Generate image alternatives with a price shown before any spend, in two modes. Quick mode turns one finished prompt into several alternatives spread across the 2-3 image models that best fit the brief. Experiment mode runs a controlled set of variations (camera, lighting, environment, style, aspect) of a brief, optionally from a reference photo, tracked in an Excel workbook where the user ticks which rows to generate. Both show the estimated total first, need one approval per paid batch, generate in parallel, and keep a ledger of every attempt. Use when someone wants several options of one image prompt, wants to compare image models, wants to explore variations of a product or scene systematically, wants to start from a reference image, or wants a cost estimate before generating. Triggers include "generate that with Image Lab", "8 options of this with Image Lab", "run Image Lab on this photo", "with Image Lab, try different lighting and camera angles". It needs to be named: a bare "generate an image" belongs to native image generation, not here.
---

# Image Lab 2

**See the price. Approve once. Generate in parallel. Keep a record of everything.**

Two modes share one engine (`scripts/image.py`), one ledger and one gate:

| | Quick mode | Experiment mode |
|---|---|---|
| For | "give me 4 options of this prompt" | "explore lighting x camera x environment of this product", or "start from this photo" |
| Input | one finished prompt | a brief, optionally reference photos, turned into a `plan.json` |
| Varies | the model | creative dimensions you choose (and optionally the model) |
| Prompt | used verbatim | compiled from the plan's wording (you may write the wording) |
| Control surface | the chat | `experiment.xlsx`: tick the rows to generate |
| Page | gallery page built for you | the workbook and the images folder |

Run commands from this skill's folder. Everything is `python scripts/image.py <command>`.
Setup once: `pip install -r requirements.txt` (experiment mode; quick mode needs nothing) and a gateway
token in `LOOMLOOM_TOKEN_COGFOUNDRY` (an API key from https://console.cogfoundry.ai/api-keys); the
loomloom CLI is **not** used. On Windows set `PYTHONUTF8=1` so non-ASCII text prints (bash: `PYTHONUTF8=1 python scripts/image.py ...`; PowerShell: `$env:PYTHONUTF8=1` once, then `python scripts/image.py ...`). Where the host has no
`AskUserQuestion` or `SendUserFile`, ask in plain chat and give the file paths instead.

## Start here

First time? Read [`references/quickstart.md`](references/quickstart.md) (one page): which route to take, the five-command loop, the plan file in 30 seconds, what the
checker enforces for you, and what to show the user. Then **copy a complete example plan** and edit it: `references/examples/variation-plan.json` (a variation
experiment) or `references/examples/direction-plan.json` (creative directions). Run `check` and the dry run; they say what to fix.

Read the rest of this file as you reach each step, not before: "Rules that always apply" always; "Experiment mode" steps 3 to 8 when you get there; `skills/quick.md` only
for quick mode; `skills/plan.md` and `skills/direction.md` (about 17 KB each) only for the detail the quickstart points to or when `check` complains.

## Rules that always apply

1. **One approval per paid batch.** Nothing is spent before the user approves the numbers
   you show. `run` needs `--confirm <fingerprint>` and refuses anything else. The
   fingerprint binds the approval to an immutable snapshot (`snapshots/<fp>.json`);
   `run` reads only that snapshot, never the workbook, so edits after approval cannot
   change what runs. After any edit, preflight again and ask again.
2. **Show the price honestly.** Quote the *known* total and say how many rows have an
   unverified price (those are not in the total). Never invent a number. The balance
   is "unread"; the gateway confirms.
3. **Unknown is never retried automatically.** If a submit gets no confirmed reply the
   attempt is `Unknown`: it may or may not be billed. Tell the user, give the submit
   time and request hash from the result, and let them check the console usage log.
   Only retry it if they ask (`retry --include-unknown`).
4. **Never overwrite the user's cells.** The workbook is regenerated from the ledger
   (`experiment.prev.xlsx` is kept). If Excel has the file open you get
   `experiment.refresh-<time>.xlsx`; tell the user.
5. **One writer at a time.** While a `run` is going, `preflight`, `retry`, `refresh`, `recover`, `quick`
   and a second `run` of the same experiment answer `BUSY` (exit 2) and change nothing; the user can still
   edit the workbook, and you preflight after the run ends. A crashed run's lock expires after 5 minutes.
6. **Quality and price.** For models with a `quality` setting, `auto` can bill two different prices for the same
   request (measured $0.030 vs $0.055). Set `quality` in the plan for a predictable cost; an explicit quality is
   unpriced until one image bills, so the preflight says `unverified` and `run` needs `--max-usd`. After the
   first images the guard counts in-flight samples at the highest real price seen.
7. **Spend limits.** `run` stops submitting past `--max-usd` (default 1.25x the known
   estimate; required when a row has no verified price). It cannot stop tasks already
   in flight, so say so if the user asks for a hard cap.
8. A fingerprint that already ran needs `--again`: a repeat is a new spend, so ask first.

## Quick mode

One finished prompt, no photo, no variation, a few options: no plan, no workbook. The full steps (capture, classify the intent, quote, approve, generate, results,
gallery page, adjust) are in [`skills/quick.md`](skills/quick.md). The rules below apply to it too.

## Experiment mode

```
BRIEF -> PLAN -> WORKBOOK -> PREFLIGHT -> APPROVE -> GENERATE -> RESULTS -> retry / next batch
```

**0. Route.** Quick mode, the planner, or Creative Direction? Explicit user words win: "generate exactly this" or a plain single-image prompt
is quick mode; words like explore, directions, concepts, campaign or ideas go to **Creative Direction** (`skills/direction.md`) without a question. When the
brief is creatively open-ended (campaign or brand language, several valid interpretations) and the user used none of those words, **ask once**: *Generate as
written* or *Explore creative directions* (recommend exploring, with one line why). Do not decide by prompt length. A reference photo or "vary X" is the planner
(`skills/plan.md`), plus Creative Direction when the brief is open-ended. Creative Direction produces 3 to 10 directions (default 4) with the user's idea as
Direction 1 and the user's copy unchanged, then hands a Direction Sheet to the plan stage.

**1. Brief.** Ask only what you cannot infer: what is being made, for what use, and
whether there is a reference photo. Put reference files in a `refs/` folder next to the
plan. Reference images work on all nine catalog models (live-checked 2026-10-07), but they bill
more than text (OpenAI about 3.4x; see `references/reference-support.json` for each model's
measured reference price) and two models altered the product in the test (seedream-4.5,
gemini-3-pro), so they are not auto-picked for reference rows. The request carries the photo,
so tell the user it is sent to the model provider. If a reference shows a **person**, include
this notice in the plan confirmation (step 2) and wait for the user's yes before building: *"This uses a photo of a person.
Confirm you have the right to use it: it is your own photo, or you have the permission of the
person pictured. The photo is sent to CogFoundry and the image model's provider for processing,
and the results stay local."* Once they confirm, run
`python scripts/image.py acknowledge-person --plan ./plan.json` (it writes
`consent_acknowledged` with a timestamp). This is enforced: `plan`, `preflight` and `run`
all refuse a plan with a pictured person and no acknowledgment, and the gallery page
builder refuses to publish a person experiment. Never run `acknowledge-person` unless the
user has said yes in chat.

**2. Plan.** Which stage writes the plan follows from step 0: Creative Direction (`skills/direction.md`) for an open brief, which ends by writing
`plan.json` through the plan stage; otherwise the planner alone. Put `plan.json` next to the `refs/` folder (any folder; `--plan` takes the path). Follow the planner stage, `skills/plan.md`: pick the intent, what is **fixed**,
2-4 dimensions with 3-4 values each, wording (the starter vocabulary is
`python scripts/image.py controls`; values marked UNTESTED work but are unproven), and
`constraints` for combinations that make no sense. Write `plan.json` (shape:
`references/plan-schema.md`), check it with
`python scripts/image.py check --plan plan.json` and
`python scripts/image.py plan --plan plan.json --out ./out/<name> --dry-run` (writes nothing,
prints the size of the experiment and what to fix), then show the user the plan in plain
words and get **one** confirmation of it, including the person notice if one applies. Do not
ask per dimension. Camera wording with a reference photo needs care: strong wording makes the
model copy-tilt the product (use `fragment_with_reference`).

**3. Build the experiment** (spends nothing; it refuses a folder that already holds an experiment, so use a new
`--out`, or edit `experiment.xlsx` and `preflight` to change an existing one):

```
python scripts/image.py plan --plan ./plan.json --out ./out/<name>
```

It validates the plan, finds every valid combination, recommends a first batch (about 30
rows, every pair of values appearing at least once), and writes `experiment.xlsx` with
those rows ticked, one image per row. Tell the user to open it, tick or untick rows, save, and say when done. For another
image of the same values they run `image.py add-takes --dir <experiment> --rows r001` (or copy the row and change Take). Do not edit the workbook yourself. The Reference
column is blank (use the plan's reference), a reference id such as `ref2` (choose one when the
plan has several; blank is then an issue), or `none` (a text-only row).

**4. Preflight** after they save:

```
python scripts/image.py preflight --dir ./out/<name>
```

It reads the workbook (read-only), validates, compiles
the prompts, chooses models and sizes, prices the batch and writes
`snapshots/<fingerprint>.json`. Relay the report: rows ready, issues (those rows are
left out), images, known cost and unverified rows, warnings (custom values, unreliable
wording with a reference, rows that are already Completed and still ticked, so would be
generated again). Issues are fixed in the workbook, then preflight again.
For a **calibration** (one image per creative direction before the full batch) add `--one-per direction`
(`preflight --dir ./out/<name> --one-per direction`); `--only r001,r009` prices named rows. The other ticked rows stay ticked, and a plain
preflight afterwards takes the rest. On a fresh clone there is no observed price yet, so rows show as unverified (nothing is added to the known total) and `run` needs `--max-usd`; preflight prints an "Indicative" range from `references/price-hints.json` and a safe limit to use. That range is a hint, not a quote: say so, and name the safe limit when you ask for approval.

**5. Approve** with `AskUserQuestion` (Generate / Edit the workbook / Stop), naming
images, models and the known total.

**6 to 8. Generate, results, retry.** After the user's approval: `run --dir ./out/<name> --confirm <fingerprint> --max-usd <N>` in the background with a progress
file (about 30 s updates), then results (contact sheet, the plan's `visual_checks` against every image, real cost), then retry, recover and next batch. The commands, the
`Unknown` and `Blocked` rules and the folder layout are in [`skills/results.md`](skills/results.md). Read it when you reach step 6.

## What NOT to do

- Do not spend without an explicit Generate at the approval step, and never reuse an old
  fingerprint after anything changed.
- Do not call a cost "the total" if rows have unverified prices.
- Do not retry `Unknown` samples on your own; do not resubmit to "fix" an unfinished run
  (run the same command again to resume).
- Do not present results as a fair model benchmark (different models, different seeds).
- Do not probe model availability with a test request; a `503` at generation is the signal
  (that row fails, nothing is charged; offer another model and re-preflight).
- Do not use the loomloom CLI.
- Do not offer counts other than 1, 2, 4, 8 in quick mode, or more than one image per row (a second image is a second row with the next Take).

## Status

Built through M6 (design `docs/design-v2.md`): generator, the experiment contact sheet (`scripts/sheet.py`), retry, quick mode, references (one
per row), person-notice enforcement, reference-support states promoted by real use, the
planner stage (`skills/plan.md`), Creative Direction (`skills/direction.md`, M6), the controls vocabulary (`references/controls-catalog.json`,
only lighting and camera are blind-tested) and a planner evaluation set
(`tests/planner-eval/`, `scripts/planner_eval.py`). The planner was run by fresh agents twice (design item 41); the newest
rules (`visual_checks`, the revised person notice) have not been exercised by a fresh agent yet.

## Attribution

- `ai-image-prompts-skill` (YouMind, MIT): a natural prompt source for quick mode, used
  unmodified; keep its attribution footer on any hand-off message.
- `runcomfy-com/skills` (MIT): design inspiration for intent-based model selection.
  `references/model-catalog.yaml` is our own stopgap shim.
- A community example on [loomloom](https://github.com/cogfoundry-labs/loomloom);
  Image Lab 2 talks to the CogFoundry gateway directly.
