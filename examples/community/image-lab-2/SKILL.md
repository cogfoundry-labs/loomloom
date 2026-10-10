---
name: image-lab-2
license: Apache-2.0
description: Generate image alternatives with a price shown before any spend, in two modes. Quick mode turns one finished prompt into several alternatives spread across the 2-3 image models that best fit the brief. Experiment mode runs a controlled set of variations (camera, lighting, environment, style, aspect) of a brief, optionally from a reference photo, tracked in an Excel workbook where the user ticks which rows to generate. Both show the estimated total first, need one approval per paid batch, generate in parallel, and keep a ledger of every attempt. Use when someone wants several options of one image prompt, wants to compare image models, wants to explore variations of a product or scene systematically, wants to start from a reference image, or wants a cost estimate before generating. Triggers include "generate that with Image Lab", "8 options of this with Image Lab", "run Image Lab on this photo", "with Image Lab, try different lighting and camera angles". It needs to be named: a bare "generate an image" belongs to native image generation, not here.
---

# Image Lab 2

**See the price. Approve each paid batch. Generate in parallel. Keep a record of everything.**

Three modes share one engine (`scripts/image.py`), one ledger and one gate. **Quick**: one finished prompt, spread across the best-fit models, no plan. **Variation** and
**Creative Direction** (together "experiment mode"): a `plan.json` becomes `experiment.xlsx`, where the user ticks the rows to generate.

Setup: run commands from this folder (`python scripts/image.py <command>`); `pip install -r requirements.txt` (experiment mode; quick mode needs nothing); a gateway key in
`LOOMLOOM_TOKEN_COGFOUNDRY` (https://console.cogfoundry.ai/api-keys), needed only for `run`; the loomloom CLI is **not** used. On Windows set `PYTHONUTF8=1` (bash:
`PYTHONUTF8=1 python ...`; PowerShell: `$env:PYTHONUTF8=1`). Without `AskUserQuestion` or `SendUserFile`, ask in plain chat and give file paths.

## Start here

First time? Read [`references/quickstart.md`](references/quickstart.md) (one page): which route to take, the five-command loop, the plan file in 30 seconds, what the
checker enforces for you, and what to show the user. Then **copy a complete example plan** and edit it: `references/examples/variation-plan.json` (a variation
experiment) or `references/examples/direction-plan.json` (creative directions). Run `check` and the dry run; they say what to fix.

Read the rest of this file as you reach each step, not before: "Rules that always apply" always; "Experiment mode" steps 3 to 8 when you get there; `skills/quick.md` only
for quick mode; `skills/plan.md` and `skills/direction.md` (about 17 KB each) only for the detail the quickstart points to or when `check` complains.

## Rules that always apply

1. **Each paid batch is approved once, and each approval authorizes exactly one execution snapshot.** A calibration, the main batch and a retry are separate batches, each with
   its own quote and approval. Nothing is spent before the user approves the numbers you show; `run` needs `--confirm <fingerprint>` and refuses anything else. The fingerprint
   binds the approval to an immutable snapshot (`snapshots/<fp>.json`, the only thing `run` reads). **What the fingerprint covers**, per row: its id (which images), the compiled prompt,
   model, size, quantity, mode, quality and reference-file hashes, plus the known total and the unverified rows. It does not cover file times.
   - **The one spend gate:** `run --confirm <fingerprint>` (and a repeat with `--again`) is the only command that spends. `retry`, `add-takes` and `recover` never spend by themselves:
     `retry` prepares a new quote for failed rows, `add-takes` adds ticked rows that need their own preflight and approval, `recover` only re-downloads images already billed and sends
     no new request. They are steps toward a batch, never a way around the approval.
   - **A calibration batch is approved on its own;** the main batch is quoted and approved again after the user has seen it.
   - **Editing and locking:** after the plan confirmation the user may keep editing the workbook; a `preflight` freezes a snapshot and the user's yes approves that one. A change after
     that, even dropping a single image, never alters the approved snapshot: preflight, quote and approve again.
   - **Edits after an approval are not in that run.** Before it starts, `run` compares the snapshot with a fresh preflight of the workbook **by content** (not file time) and names each
     differing row and field; after it, it lists the ticked rows that were not approved.
   - **Say the scope when you ask:** "This approves exactly these N images (about $X, fingerprint ...). It does not approve anything else; the other M ticked rows will be quoted separately." Name the rows from preflight's "Rows in this batch" list.
   - **Two different "yes" answers:** a *plan confirmation* (the plan or Direction Sheet is what the user wants; spends nothing; "the one confirmation" in `skills/plan.md` and
     `skills/direction.md`) is not a *spend approval* (a fingerprint).
2. **Show the price honestly.** Quote the *known* total and say how many rows have an unverified price (not in the total). Never invent a number. The balance is "unread"; the gateway confirms.
3. **Unknown is never retried automatically.** A submit with no confirmed reply is `Unknown`: it may or may not be billed. Tell the user, give the submit time and request hash, and let
   them check the console usage log. Only after they have checked it and accepted the billing risk does `retry --include-unknown` prepare a new quote, which needs its own approval.
4. **Never overwrite the user's cells.** The workbook is regenerated from the ledger (`experiment.prev.xlsx` is kept); if Excel has the file open you get
   `experiment.refresh-<time>.xlsx`: tell the user.
5. **One writer at a time.** While a `run` is going, `preflight`, `retry`, `refresh`, `recover`, `quick` and a second `run` answer `BUSY` (exit 2) and change nothing; the user can
   still edit the workbook, and you preflight after the run ends. A crashed run's lock expires after 5 minutes.
6. **Quality and price.** With a `quality` setting, `auto` can bill two prices for the same request (measured $0.030 vs $0.055): set `quality` in the plan. An explicit quality is
   unpriced until one image bills, so preflight says `unverified` (with an indicative range and a safe limit) and `run` needs `--max-usd`.
7. **Spend limits.** `--max-usd` (default 1.25x the known estimate; required when a row has no verified price) limits **new submissions**: in-flight samples count at the highest real
   price seen, but a sample with no known price counts as zero until it bills, and requests already in flight still bill. The final cost can pass it: it is **not a hard cap**, so never
   describe it as one. The result reports the limit and any overshoot.
8. A fingerprint that already ran needs `--again`: a repeat is a new spend, so ask first.

## Quick mode

One finished prompt, no photo, no variation, a few options: no plan, no workbook. The full steps (capture, classify the intent, quote, approve, generate, results,
gallery page, adjust) are in [`skills/quick.md`](skills/quick.md). The rules above apply to it too.

## Experiment mode

```
BRIEF -> PLAN -> WORKBOOK -> PREFLIGHT -> APPROVE -> GENERATE -> RESULTS -> retry / next batch
```

**0. Route.** Decide on the user's **intent and constraints**, not on single words or prompt length; first match wins (the three modes and the question each answers are in
`references/quickstart.md`, section 1):

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

**1. Brief.** Ask only what you cannot infer: what is being made, for what use, and whether there is a reference photo. Put reference files in a `refs/` folder next to the plan.
References work on all nine catalog models but bill more than text (OpenAI about 3.4x; `references/reference-support.json`), and two models altered the product in the test
(seedream-4.5, gemini-3-pro), so they are not auto-picked for reference rows. The photo is sent to the model provider: tell the user. If a reference shows a **person**, put this
notice in the plan confirmation and wait for the user's yes before building: *"This uses a photo of a person. Confirm you have the right to use it: it is your own photo, or you
have the permission of the person pictured. The photo is sent to CogFoundry and the image model's provider for processing, and the results stay local."* Then run
`python scripts/image.py acknowledge-person --plan ./plan.json` (it writes `consent_acknowledged` with a timestamp). `plan`, `preflight` and `run` refuse a pictured person without it,
and the gallery builder refuses to publish a person experiment. Never run `acknowledge-person` unless the user said yes in chat.

**2. Plan.** Creative Direction (`skills/direction.md`) for an open brief, which ends by writing `plan.json` through the plan stage; otherwise the planner alone (`skills/plan.md`).
Put `plan.json` next to `refs/`. Pick the intent, what is **fixed**, 2 to 4 dimensions with 3 to 4 values each, wording (the starter vocabulary: `python scripts/image.py controls`;
UNTESTED values work but are unproven) and `constraints`. Check with `check --plan plan.json` and `plan --plan plan.json --out ./out/<name> --dry-run` (writes nothing; prints the size
and what to fix), then show the plan in plain words and get **one plan confirmation** (not a spend approval), including the person notice if one applies. Do not ask per dimension.
Camera wording with a reference photo needs care: strong wording makes the model copy-tilt the product (use `fragment_with_reference`).

**3. Build the experiment** (spends nothing; it refuses a folder that already holds one, so use a new `--out`, or change an existing one: edit `experiment.xlsx` for values, rows,
Model and Take, or `./out/<name>/plan.json` for wording and checks, then `preflight`; rows that already have an image keep it, and new dimension values or directions need a new
experiment): `python scripts/image.py plan --plan ./plan.json --out ./out/<name>`. It validates the plan, recommends a first batch (about 30 rows, every pair of values at least once)
and writes `experiment.xlsx` with those rows ticked, one image per row. Tell the user to open it, tick or untick rows, save, and say when done. For another image of the same values:
`image.py add-takes --dir <experiment> --rows r001` (or copy the row and change Take). Do not edit the workbook yourself. Reference column: blank (the plan's reference), a reference
id such as `ref2`, or `none` (text-only row).

**4. Preflight** after they save: `python scripts/image.py preflight --dir ./out/<name>`. It reads the workbook (read-only), validates, compiles the prompts, chooses models and
sizes, prices the batch and writes `snapshots/<fingerprint>.json`. Relay the report: rows ready, issues (those rows are left out), images, known cost and unverified rows, warnings.
Fix issues in the workbook and preflight again. For a **calibration** (one image per creative direction first) add `--one-per direction`; `--only r001,r009` prices named rows; the
other ticked rows stay ticked. With no observed price yet (a fresh clone) rows show as unverified, nothing is added to the known total, and preflight prints an "Indicative" range
(`references/price-hints.json`) and a safe `--max-usd`: a hint, not a quote; say so.

**5. Approve** this batch with `AskUserQuestion` (Generate / Edit the workbook / Stop), naming images, models and the known total, and **its scope** (rule 1). For creative
directions the first batch is the calibration, and its approval says so.

**6 to 8. Generate, results, retry.** After the user's approval: `run --dir ./out/<name> --confirm <fingerprint> --max-usd <N>` in the background with a progress
file (about 30 s updates), then results (contact sheet, the plan's `visual_checks` against every image, real cost), then retry, recover and next batch. The commands, the
`Unknown` and `Blocked` rules and the folder layout are in [`skills/results.md`](skills/results.md). Read it when you reach step 6.

## What NOT to do

- Do not spend without an explicit Generate at the approval step, and never reuse an old fingerprint after anything changed.
- Do not call a cost "the total" if rows have unverified prices.
- Do not resubmit to "fix" an unfinished run (run the same command again to resume).
- Do not present results as a fair model benchmark (different models, different seeds).
- Do not probe model availability with a test request; a `503` at generation is the signal (that row fails, nothing is charged; offer another model and re-preflight).
- Do not offer counts other than 1, 2, 4, 8 in quick mode, or more than one image per row (a second image is a second row with the next Take).

## Status and attribution

Usable from a checkout; not published. Design, decisions and open items: `docs/design-v2.md`; measured evidence: `docs/proposal-llm-fit-advisor.md`, `references/llm-fit/`.
`ai-image-prompts-skill` (YouMind, MIT) is a natural prompt source for quick mode, used unmodified (keep its attribution footer on a hand-off message); `runcomfy-com/skills`
(MIT) inspired intent-based model selection. A community example on [loomloom](https://github.com/cogfoundry-labs/loomloom); Image Lab 2 talks to the CogFoundry gateway directly.
