---
name: image-lab
license: Apache-2.0
description: >
  Turn one image prompt into several strong alternatives, generated in parallel
  through CogFoundry's model gateway — the Advisor spreads the alternatives
  across the best-fit models for the brief — with an estimated total before any
  spend and the actual cost shown after. The prompt can come from any
  prompt-authoring skill (e.g. ai-image-prompts-skill), be pasted, or be in a
  local file. Use ONLY when the user names Image Lab: "generate that with Image
  Lab", "make alternatives with Image Lab", "run Image Lab on this prompt", "4
  options of this with Image Lab". Do NOT use for a bare "generate an image" /
  "make a picture" — that belongs to native image generation or another skill.
---

# Image Lab

**One prompt. Several strong possibilities. Pick the image you want.**
See the price. Approve once. Generate in parallel.

Image Lab helps you **pick an image, not pick a model**. It does not write
prompts and is not tied to any prompt source — it takes a finished prompt, and
its Model Advisor spreads your requested number of alternatives across the
models that best fit the brief. Pair it with `ai-image-prompts-skill`, any other
prompt skill, or use your own prompt.

## The five user-facing steps

```
PLAN  ->  QUOTE  ->  APPROVE  ->  GENERATE  ->  RESULTS
```

- **PLAN** — you classify the prompt to one creative intent; `image.py`
  deterministically scores the models and produces an **allocation** (which
  models, how many alternatives each, at what size).
- **QUOTE** — the estimated total (printed by the same `resolve` call). No
  server-side quote exists.
- **APPROVE** — one `AskUserQuestion`: Generate / Adjust / Stop. Nothing is
  spent before this. **This is the only gate — the wallet boundary.**
- **GENERATE** — the whole allocation submitted together as independent gateway
  tasks. `run` works in the **background**; you poll its progress file and post
  a short "N/total done" update every ~30–60s so the user is never staring at
  nothing.
- **RESULTS** — every image, the actual cost, **and** a shareable exploration
  page built automatically. The user picks a favourite from the page (just
  conversation, not a gate).

The exploration page (step 7) is a self-contained static folder of the brief +
every alternative + the pick. It spends nothing, so it is always built — never
ask "do you want a page?".

`pipelines/generate.yaml` is the stage manifest. Follow it.

## Input contract

Only the **prompt** is required. Everything else is inferred or optional.

| Field | Source |
|---|---|
| prompt text | conversation context (any skill or the user), or a local file path |
| creative intent | **you** classify it (see step 2) |
| model allocation | `image.py` decides deterministically; the user may override by name |
| size | `image.py` picks a valid size per model from the intent + each model's minimum |
| alternatives count | **4** by default; **1 / 2 / 4 / 8** supported. 1 → A only; 2 / 4 → A + B (two models); **8 → A + B + C (three models, a "deep exploration" widen)**. `count` is a number of images, never "top-N models". |
| reference image | out of scope in v0.1 — if the prompt needs one, say so and stop |

Do not read another skill's files. Do not rewrite the prompt.

### Mapping natural language to `count`

| The user says | `count` |
|---|---|
| "just one", "a single image" | 1 |
| "a couple", "compare a few", "two options" | 2 |
| *(nothing about count)*, "some options", "explore this" | **4** |
| "lots", "as many as makes sense", "deep dive", "make 8" | 8 |

Never invent other numbers. If they ask for 5, round to 4; for 6+, use 8.

## Workflow

### 1. capture

Take the final prompt from context (the last prompt an upstream skill produced,
or what the user pasted / a file they named). No prompt → ask for one. If the
prompt clearly needs a reference/input image, say Image Lab v0.1 is
text-to-image only and stop.

### 2. classify the intent

Pick the single best match from this list (from `references/generation-policy.md`):

`launch / announcement image` · `profile / avatar` · `social post` ·
`blog hero / article cover` · `poster / flyer` · `infographic / diagram` ·
`illustration / concept art` · `product / e-commerce shot` · `generic`

Use `generic` only if nothing else fits. This is your one judgement call — the
user can correct it ("no, treat it as a poster").

### 3. plan + quote  (PLAN → QUOTE)

Run (one call does both):

```
python scripts/image.py resolve --intent "<intent label>" --count <N> --explain
```

Add `--models "<id>[,<id>]"` **only** if the user named specific models
("use GPT Image 2", "compare Nano Banana Pro and Seedream"). The Advisor never
silently replaces a model the user asked for.

At `count 8` the allocation normally spans **three** models (A ×3 + B ×3 + C ×2)
— the "deep exploration" widen. That is expected, not a bug; `count` is still a
number of images.

It checks readiness, resolves the token, and returns `plan` with:

- `allocation` — a list of `{ model, label, size, usd_per_image, n, subtotal_usd }`
- `alloc_arg` — the exact string to hand to `run` at step 5 (copy it verbatim)
- `estimated_usd` — the total; `sufficient` — whether the balance covers it
- `why` — one line explaining the model choice
- the full score table on stderr (from `--explain`)

If it errors (loomloom not ready, no token), relay the one fix and stop. If
`sufficient` is false, say the balance is short and stop — do not reach APPROVE.

Present the PLAN as the script prints it — each model, its count, its size, its
subtotal — then the `why` line and the estimated total. The estimate is a
pre-flight guess of the gateway's own `cost`; the actual can be lower (e.g.
`gemini-3.1-flash-image` currently bills $0 under a launch preview but is quoted
conservatively so the gate stays honest). RESULTS shows the real charge.

### 4. approve  (APPROVE)

Call `AskUserQuestion` with exactly:

- **Generate <N>** — proceed
- **Adjust** — change intent / models / count, then re-run step 3
- **Stop** — halt; nothing spent

Because one request can spend across multiple models, the question text must
name the actual mix and the total (e.g. "Generate 4 — Nano Banana Pro ×2 + GPT
Image 2 ×2, ~$0.11?").

### 5. generate  (GENERATE)

Run it **in the background** with a progress file:

```
python scripts/image.py run --alloc "<alloc_arg from step 3>" \
  --prompt "<final prompt>" --intent "<intent label>" --out ./out \
  --progress-file ./out/progress.json --confirm
```

While it runs, every ~30–60s read `./out/progress.json` and post a compact
update — do not leave the user with no signal for minutes:

```
GENERATING 8 · 3 models
  GPT Image 2       A ✓115s  B ✓114s  C ⋯      D …
  Nano Banana Pro   E ✓35s   F ⋯      G …
  Nano Banana 2     H …
  4/8 done · $0.37 so far
```

`progress.json` fields: `phase` (`submitted` → `generating` → `done`), `done`,
`failed`, `total`, `actual_usd_so_far`, and `branches[]`
(`label`, `model_label`, `status`, `seconds`, `cost_usd`). When `phase` is
`done`, read the run record from stdout / `./out/run.json` and go to RESULTS.
Each branch downloads to `./out/variant-a.png` …

**If it exits with `status: model_unavailable`** (exit code 3): the first model
was rejected by the gateway. It prints a `suggested_model` / `suggested_size`.
Show the user:

```
MODEL UNAVAILABLE
<failed_label> could not be generated right now.
Suggested replacement: <suggested_label>  (~$<rate>/image)
[ Use replacement ]   [ Stop ]
```

On "Use replacement" → re-run step 3 with `--models "<suggested_model>"` (plus
any still-good model), then step 4. This is a *changed plan*, so a fresh
approval is correct.

### 6. results  (RESULTS)

`run` prints the run record (JSON) to stdout and writes it to `./out/run.json`.
`alternatives[]` carries each branch's `label`, `model_label`, `seconds`
(generation time), `cost_usd`, and `actual_size`; `file` is a basename (the
image is `./out/<file>`).

- **Send each image as its own `SendUserFile` call** with a caption that names
  its label and model — e.g. `A · Nano Banana Pro`, `C · GPT Image 2` — so the
  user can tell which is which. The files are `./out/variant-a.png` …
- Present a RESULTS table with columns **label · model · time · cost · size**
  (from `alternatives[]`), then `Actual total: $X.XXXX (estimated ~$Y.YYYY)`.
- If a branch failed or is incomplete, name it plainly — no retry in v0.1; the
  user still picks from what succeeded.
- Then **build the exploration page (step 7)** and hand the user the link,
  asking them to pick their favourite from there. Do not ask first — the page
  costs nothing. Picking is ordinary conversation, not a gate.

### 7. shareable page  (part of RESULTS — always built, no gate)

`run` wrote `./out/run.json`. Build the page:

```
python scripts/build-exploration-page.py --from ./out --inline \
  --title "<3-5 word title you propose>" \
  --subject "<short noun phrase, e.g. Murree x Toronto>" \
  --invocation "<the exact message the user sent to trigger Image Lab>" \
  [--canonical-url <where the folder will live>] [--selected <label>]
```

It builds a self-contained static folder `./out/<slug>/` (`index.html`, the
images in `assets/`, `exploration.json`). **Nothing is spent** — the images
already exist; this is a render step, not a gate. `--inline` also writes
`./out/<slug>/index.inline.html` — one self-contained file; **publish that** as
an artifact for the user's live URL. The result's stderr says its size; if it is
over ~15 MB, recompress the PNGs to JPEG first.

The page (redesign-lab's case-study house style — Source Serif 4 body, Arial-
Black uppercase headings, IBM Plex Mono labels, hard edges, loomloom green,
3-state dark mode). **Every section has the same shape**: a mono eyebrow, an
Arial-Black headline, one lead line, then the body.

- **topbar** — wordmark + a **Share** cluster: `Copy link` + one-click `X` /
  `LinkedIn` (hrefs built from the canonical URL + the OG tags)
- **hero** — `AI-generated · not a benchmark` badge, the tagline, a **model
  legend** (`GPT Image 2 ×3` chips, each linked to its cogfoundry.ai page), the
  stats line
- **Gallery / Pick your image** — framed hero + letter-badged thumbnails (`A ·
  GPT Image 2` — candidates, not steps) + live meta + `N / 8` counter; click a
  thumbnail to swap, click the hero for a minimal full-screen lightbox (`← →`,
  `Esc`, `Copy link`, `Open raw ↗`). Each alternative has a **deep link** — the
  page reads `…/index.html#E` on load and selects alternative E, updates the
  hash as you browse; a "Copy link to alternative N" button copies it.
- **Details / The run** — a hairline facts grid (intent · models · candidates ·
  size · actual cost · date)
- **Reuse / The prompt** — the exact run prompt in italic + a "→ use Image Lab"
  trigger + Copy button + a 3-step **"how to use this"** (copy → paste into an
  agent with the skill → run as-is or swap details to make it yours)
- **Provenance / How this was made** — every model and tool, linked, with the
  run's real cost
- **Roadmap / From image exploration to AI work** — the v0.4 loomloom workflow
- **Your turn / Bring your own prompt** — a CTA with the `npx skills add …` line
- Open Graph / Twitter-card meta so a pasted link unfurls with the image

Flags:
- `--title` — a short name for the exploration.
- `--subject` — the page composes the tagline *"One brief. N models. M ways to
  see {subject}."* from the run's real numbers.
- `--invocation` — the user's actual triggering message, **verbatim**. Shown as-
  is in the Prompt block; omitted → `<prompt>\n\nuse Image Lab`.
- `--canonical-url` — the URL the folder will live at (GitHub Pages, etc). Makes
  `og:image` absolute (so link unfurls work) and the topbar Share cluster point
  at that URL. Omit for the artifact flow (the artifact URL isn't known until
  after publish; `location.href` covers it).
- `--summary` — optional; overrides the auto tagline.
- `--selected` — omit on the first build (the user hasn't picked yet). When they
  name a favourite, re-run **with** `--selected <label>`: its thumbnail gets a ✓
  badge and becomes the initial hero — the creator's pick, never "best".
- With JS off the hero is still a plain link to the full-resolution file. Give
  the user the folder path (GitHub-Pages-ready) and publish `index.inline.html`
  as an artifact so they have a live URL to pick from.

## Prerequisites

`image.py` checks these at step 3 and stops with the one fix if anything is
missing: the loomloom CLI installed, a server selected, a valid token
(`LOOMLOOM_TOKEN_COGFOUNDRY` env var, or `loomloom login`), and enough balance.
A user who provides a prompt and stops at APPROVE never has to set anything up.

## What NOT to do

- Do not write or rewrite the prompt.
- Do not read `ai-image-prompts-skill`'s files, categories, or manifest.
- Do not spend without an explicit Generate at APPROVE.
- Do not offer counts other than 1, 2, 4, 8.
- Do not present this as a model benchmark or leaderboard — it is not a fair
  comparison (different models, different seeds). It gives the user *choices*.
- Do not probe model availability by sending a test request — the catalog is the
  source of truth; a `503` at GENERATE is the only availability signal.
- Do not claim "loomloom parallel execution" — v0.1 uses the gateway directly.
- Do not attempt reference-image / image-to-image generation.

## Attribution

- `ai-image-prompts-skill` (YouMind, MIT) — the natural prompt pairing, used
  unmodified. Keep its attribution footer on any hand-off message.
- `runcomfy-com/skills` (MIT) — design inspiration for intent-based model
  selection. `references/model-catalog.yaml` is our own stopgap shim.
- Built as a community example on
  [loomloom](https://github.com/cogfoundry-labs/loomloom); the v0.4 `showcase/`
  turns Image Lab into a real loomloom workflow. See `showcase/README.md`.
