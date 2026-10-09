---
name: image-lab-2
license: Apache-2.0
description: Generate image alternatives with a price shown before any spend, in two modes. Quick mode turns one finished prompt into several alternatives spread across the 2-3 image models that best fit the brief. Experiment mode runs a controlled set of variations (camera, lighting, environment, style, aspect) of a brief, optionally from a reference photo, tracked in an Excel workbook where the user ticks which rows to generate. Both show the estimated total first, need one approval per paid batch, generate in parallel, and keep a ledger of every attempt. Use when someone wants several options of one image prompt, wants to compare image models, wants to explore variations of a product or scene systematically, wants to start from a reference image, or wants a cost estimate before generating. Triggers include "generate that with Image Lab", "8 options of this with Image Lab", "run Image Lab on this photo", "with Image Lab, try different lighting and camera angles". It needs to be named: a bare "generate an image" belongs to native image generation, not here.
---

# Image Lab 2

**See the price. Approve each paid batch. Generate in parallel. Keep a record of everything.**

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

1. **Each paid batch is approved once, and each approval authorizes exactly one execution snapshot.** An experiment can have several paid batches (a calibration,
   then the rest, then a retry), and each needs its own quote and its own approval. Nothing is spent before the user approves the numbers you show. `run` needs
   `--confirm <fingerprint>` and refuses anything else. The fingerprint binds the approval to an immutable snapshot (`snapshots/<fp>.json`); `run` reads only that
   snapshot, never the workbook. **What the fingerprint covers**, for every row: its id (which images), the compiled prompt, model, size, quantity, mode, quality and the
   hash of each reference file, plus the known total and the unverified rows. It does not cover file times.
   - **The one spend gate:** `run --confirm <fingerprint>` (and a repeat with `--again`) is the only command that spends. `retry`, `add-takes` and `recover` never spend by
     themselves: `retry` prepares a new quote for failed rows (a new snapshot that needs its own approval); `add-takes` adds ticked rows that need their own preflight and approval;
     `recover` only re-downloads images that were already billed, from their recorded URL, and sends no new request. They are steps toward a batch, never a way around the approval.
   - **Editing and locking:** after the plan confirmation the user may keep editing the workbook; a `preflight` freezes a snapshot and the user's yes approves that one.
     A change after that, even dropping a single image, never alters the approved snapshot: preflight again, quote again, approve again.
   - **A calibration batch is approved on its own.** Approving one image per direction does not approve the rest of the experiment.
   - **The main batch is quoted and approved again,** after the user has seen the calibration.
   - **Anything that changes what would run needs a new `preflight` and approval:** a changed direction, selected rows, model, size or prompt.
   - **An approved snapshot never changes**, whatever is saved in Excel afterwards. Edits made after an approval are not in that run. Before it starts, `run` compares the snapshot with a fresh preflight of the
     current workbook **by content** (not file time: re-saving an unchanged workbook is no difference) and names each row and field that differs; after it, it lists the ticked rows that were not approved.
   - **Say the scope when you ask:** "This approves exactly these N images (about $X, fingerprint ...). It does not approve anything else; the other M ticked rows will be quoted separately."
   - **Two different "yes" answers:** a *plan confirmation* (the plan or Direction Sheet is what the user wants; spends nothing; "the one confirmation" in
     `skills/plan.md` and `skills/direction.md`) is not a *spend approval* (a fingerprint).
2. **Show the price honestly.** Quote the *known* total and say how many rows have an
   unverified price (those are not in the total). Never invent a number. The balance
   is "unread"; the gateway confirms.
3. **Unknown is never retried automatically.** If a submit gets no confirmed reply the
   attempt is `Unknown`: it may or may not be billed. Tell the user, give the submit
   time and request hash from the result, and let them check the console usage log.
   Only after the user has checked the usage log and accepted the billing risk, `retry --include-unknown` prepares a new quote, which needs its own approval.
4. **Never overwrite the user's cells.** The workbook is regenerated from the ledger
   (`experiment.prev.xlsx` is kept). If Excel has the file open you get
   `experiment.refresh-<time>.xlsx`; tell the user.
5. **One writer at a time.** While a `run` is going, `preflight`, `retry`, `refresh`, `recover`, `quick`
   and a second `run` of the same experiment answer `BUSY` (exit 2) and change nothing; the user can still
   edit the workbook, and you preflight after the run ends. A crashed run's lock expires after 5 minutes.
6. **Quality and price.** For models with a `quality` setting, `auto` can bill two different prices for the same
   request (measured $0.030 vs $0.055). Set `quality` in the plan for a predictable cost; an explicit quality is
   unpriced until one image bills, so the preflight says `unverified` and `run` needs `--max-usd`.
7. **Spend limits.** `--max-usd` (default 1.25x the known estimate; required when a row has no verified price) limits **new submissions**: in-flight samples
   are counted at the highest real price seen, but a sample with no known price counts as zero until it bills, and requests already in flight still bill. So the final cost
   can pass it. It is **not a hard cap**: never describe it as one. The result reports the limit and any overshoot.
8. A fingerprint that already ran needs `--again`: a repeat is a new spend, so ask first.

## Quick mode

One finished prompt, no photo, no variation, a few options: no plan, no workbook. The full steps (capture, classify the intent, quote, approve, generate, results,
gallery page, adjust) are in [`skills/quick.md`](skills/quick.md). The rules below apply to it too.

## Experiment mode

```
BRIEF -> PLAN -> WORKBOOK -> PREFLIGHT -> APPROVE -> GENERATE -> RESULTS -> retry / next batch
```

**0. Route.** Three modes, three different questions. Decide on the user's **intent and constraints**, not on single words or prompt length; first match wins:

| | Question it answers | What the user buys |
|---|---|---|
| **Quick** | the same prompt: how do I get a few good options fast? | speed and convenience |
| **Variation** | which variables change the picture, and how? | a controlled experiment |
| **Creative Direction** | what different visual ideas can this brief become? | creative exploration |

1. The user asks to **explore creative directions** (explore, concepts, directions, campaign ideas, different ideas or looks for a brief) -> **Creative Direction**
   (`skills/direction.md`), no question needed. A reference photo can be part of it. It produces 3 to 10 directions (default 4) with the user's idea as Direction 1 and the
   user's copy unchanged, then hands a Direction Sheet to the plan stage.
2. The user asks to **control or compare variables** ("try these lighting setups", "vary the camera"), **or the request needs something Quick cannot do**: a reference photo
   (Quick takes none) or more than 8 images (Quick makes 1, 2, 4 or 8) -> **Variation** (`skills/plan.md`). A photo used only as a reference is still this route today.
3. The brief is **creatively open-ended** (campaign or brand language, several valid interpretations) but the user did not say whether to explore -> **ask once**:
   *Generate as written* or *Explore creative directions* (recommend exploring, with one line why).
4. Anything else -> **Quick**: one finished prompt, or a plain request for a few options of a concrete scene.

A word such as "different" is weak evidence: "8 different product shots" with no photo is Quick, and the approval message says what Quick gives (see `skills/quick.md`).
Say which route you chose in one line.

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
words and get **one plan confirmation** of it (not a spend approval), including the person notice if one applies. Do not
ask per dimension. Camera wording with a reference photo needs care: strong wording makes the
model copy-tilt the product (use `fragment_with_reference`).

**3. Build the experiment** (spends nothing; it refuses a folder that already holds an experiment, so use a new
`--out`, or change an existing one: edit `experiment.xlsx` for values, rows, Model and Take, or `./out/<name>/plan.json` for wording and checks, then `preflight`; rows that
already have an image keep it, and new dimension values or directions need a new experiment):

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

**5. Approve** this batch with `AskUserQuestion` (Generate / Edit the workbook / Stop), naming
images, models and the known total, and **its scope** (rule 1): exactly these images, nothing else. For creative directions the first batch is the calibration, one image per
direction, and its approval says so.

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

Usable from a checkout; not published. Design, decisions and open items: `docs/design-v2.md`. The planner and Creative Direction stages have been run by fresh agents
(three cold-start tests) and measured on several models (`docs/proposal-llm-fit-advisor.md`, `references/llm-fit/`).

## Attribution

- `ai-image-prompts-skill` (YouMind, MIT): a natural prompt source for quick mode, used
  unmodified; keep its attribution footer on any hand-off message.
- `runcomfy-com/skills` (MIT): design inspiration for intent-based model selection.
  `references/model-catalog.yaml` is our own stopgap shim.
- A community example on [loomloom](https://github.com/cogfoundry-labs/loomloom);
  Image Lab 2 talks to the CogFoundry gateway directly.
