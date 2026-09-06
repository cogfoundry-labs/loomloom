# Image Lab

**One prompt. Several strong possibilities. Pick the image you want.**
See the price. Approve once. Generate in parallel.

_Powered by CogFoundry's model gateway._

```
7 image models  ·  estimate before you spend  ·  live progress  ·  actual cost after
```

Image Lab helps you **pick an image, not pick a model**. You bring a prompt; its
Model Advisor spreads your requested number of alternatives across the models
that best fit the brief, shows you the estimated total, and — once you approve —
runs them all in parallel. You see what it actually cost, then choose your
favourite.

It is **not** a model benchmark. Different models and different random seeds
make the outputs unsuitable for a fair comparison. It gives you *choices*.

## What it does

You bring a prompt — from
[`ai-image-prompts-skill`](https://github.com/YouMind-OpenLab/ai-image-prompts-skill),
any other prompt skill, or your own. Image Lab:

1. **PLAN** — classifies the creative intent, then a deterministic scorer builds
   an *allocation*: which models, how many alternatives each, at what size
2. **QUOTE** — the estimated total (printed by the same step)
3. **APPROVE** — one yes/no. Nothing is spent before this. It is the only gate.
4. **GENERATE** — the whole allocation as independent gateway tasks, in parallel.
   It runs in the background and reports "N/8 done" as each lands, so a long run
   still shows motion
5. **RESULTS** — every image, the actual cost, **and** a shareable **exploration
   page** built automatically: one brief, every alternative, a click-to-compare
   gallery. You pick your winner from the page. Nothing is spent to build it.

## Your exploration budget

`count` is the number of alternatives you want — **1, 2, 4, or 8**, default **4**.

| count | meaning | typical allocation |
|---:|---|---|
| 1 | just make one | the single best-fit model |
| 2 | a quick comparison | the top 2 models, one each |
| **4** | **standard exploration** | **top 2 models, two each** |
| 8 | deep exploration | top **3** models — A ×3 + B ×3 + C ×2 |

The allocation is deterministic — the same intent + count always plans the same
mix — while the images themselves stay stochastic. When only one model genuinely
fits the brief, the whole budget goes to it. `count = 8` is the one level that
also widens the model spread (a broader net for a deep dive); how the budget is
split is an internal Advisor policy and may evolve without changing what `count`
means — it is always a number of images, never "top-N models".

Say it naturally: *"just one"*, *"give me a couple"*, *"explore this"*,
*"make 8"*. There is no slider.

## Try it

```bash
npx skills add cogfoundry-labs/loomloom --skill image-lab -a claude-code -g -y
```

Then, with a prompt in hand:

```
generate that with Image Lab
```

or

```
8 options of this with Image Lab —
"isometric 3D illustration of a developer workflow, soft studio lighting, muted palette, 16:9"
```

or force the models:

```
with Image Lab, compare Nano Banana Pro and GPT Image 2 on this
```

It triggers **only** when you name Image Lab — never on a bare "generate an
image" — so it stays out of the way of native image generation and other image
skills.

## Prerequisites

The loomloom CLI, a selected server, a token
(`LOOMLOOM_TOKEN_COGFOUNDRY` or `loomloom login`), and a positive balance. The
gateway API uses the **same token and the same balance** as the loomloom CLI.
`scripts/image.py` checks all of this at PLAN and stops with the one fix if
anything is missing — a user who stops at APPROVE never has to set it up.

## The one gate

Only **APPROVE** is a formal gate — the wallet boundary. One request can spend
across two models, so the approval screen shows the exact mix and per-model
subtotals. Picking a favourite at RESULTS is just conversation; the run is
already done.

## What's in this folder

| Path | What it is |
|---|---|
| `SKILL.md` | Entry point — trigger, contract, the five steps |
| `docs/design-spec.md` | Why it's shaped this way — the Advisor, the two layers, the roadmap |
| `pipelines/generate.yaml` | The stage manifest the agent follows |
| `references/generation-policy.md` | **Durable:** creative intent → capability requirements + preferred sizes. Hand-editable — bring your own taste. |
| `references/model-catalog.yaml` | **Temporary adapter:** model ids, per-dimension scores, measured rates, size minimums. `TODO`: replace with a gateway endpoint when one exists. |
| `scripts/image.py` | `resolve` / `run` — generation; standard-library Python, no SDK |
| `scripts/build-exploration-page.py` | turns a run into a shareable static folder (brief + alternatives + pick); no spend |
| `test-fixtures/sample-prompts.json` | Prompts + expected intent/allocation, for exercising PLAN without spend |
| `case-studies/<slug>/` | Curated, committed exploration pages from real runs — self-contained, GitHub-Pages-ready. See `case-studies/README.md`. |
| `showcase/` | **v0.4** — Image Lab as a real loomloom workflow (not wired into v0.1) |

## How model selection works

`generation-policy.md` gives each creative intent a set of per-dimension
requirement weights (`photorealism`, `typography`, `composition_control`,
`speed` — `low`/`medium`/`high` → `0`/`1`/`2`). `model-catalog.yaml`
scores each model `-1`…`2` on the same dimensions. `image.py`:

1. **disqualifies** any model that is weak (`-1`) at a `high` requirement;
2. ranks the rest by **suitability** — the weighted dot product;
3. picks **A** = the top-suitability model (exact ties broken by lower cost);
4. picks **B** = the next-best surviving model — the runner-up, the "also worth
   trying" pick (A takes the whole budget only when it's the sole survivor);
5. at `count = 8` only, picks **C** = the third-best surviving model for a wider
   net;
6. splits the `count`: `1 → A`, `2 / 4 → A + B`, `8 → A ×3 + B ×3 + C ×2`.

Each model is priced **at the size it would actually run** (Seedream's forced
upsize and GPT Image 2's size-dependent rate both count). Rates in
`model-catalog.yaml` were measured against the live gateway on 2026-09-06
and are a pre-flight guess of the `cost` the gateway reports back — RESULTS
always shows the real charge.

It is deterministic — the same intent + count always plans the same mix — and
`image.py resolve --explain` prints the full table.

## The exploration page

RESULTS always runs `scripts/build-exploration-page.py --from ./out` (it costs
nothing, and you pick your favourite from it), turning `./out/run.json` + the
images into a self-contained static folder:

```
out/<slug>/
  index.html          an image-selection gallery: hero + thumbnail strip, lightbox
  assets/alternative-01.png …
  exploration.json    the data model (a future PDF / social-card renderer reads this)
```

It's an **image-selection gallery, not a benchmark report** — a topbar with a
**Share** cluster (`Copy link` + one-click `X` / `LinkedIn`); a hero with an
*"AI-generated · not a benchmark"* badge, the tagline, and a **model legend**
(`GPT Image 2 ×3` chips, each linked to its model page); then *"Pick your image"*
— a framed hero and a thumbnail strip badged by **letter** (`A · GPT Image 2`,
not `01` — candidates, not steps). Click a thumbnail → the hero + a
`model · size · cost · time` line swap, with an `N / 8` counter and a *"Copy link
to alternative N"* button (a shared `…/index.html#E` opens straight to candidate
E). Click the hero → a minimal full-screen lightbox (`← →`, `Esc`, `Copy link`,
`Open raw ↗`). Below: **The run** (a facts grid), the full **Prompt** (reusable,
with Copy + a 3-step *"how to use this"*), **How this was made** (every model +
tool, linked, with the real cost), a **Coming soon** note, and a **CTA** to
install Image Lab. Open Graph tags make a pasted link unfurl with the image.

Every section shares one shape — mono eyebrow, Arial-Black headline, one lead
line, then the body — in `redesign-lab`'s case-study house style (same token
block, two-tier 1200/820 width, hairline grids). No cost — the images already
exist. GitHub-Pages-ready (`--canonical-url` sets the deploy URL); the skill
also publishes the one-file `index.inline.html` as an artifact so you have a
live URL to pick from.

## Roadmap — primitive → workflow

| Version | Adds | Execution layer |
|---|---|---|
| **v0.1 (this)** | any prompt → 1/2/4/8 alternatives across the best-fit models → estimate → approve → gallery → **shareable exploration page**; 7 models | gateway API |
| v0.2 — Retry + mix control | retry failed branches; tune the allocation ("more of A", pin a third model) | gateway API |
| v0.3 — Judge | an LLM ranks the alternatives — the first step dependency | gateway + local LLM |
| **v0.4 — Workflow** | tidy → generate → judge as one loomloom TemplateSpec, with a run record | **loomloom** |
| v0.5 — Reference + Edit | reference image in; generate → edit chain | |
| v0.6 — Batch | a file of prompts → a gallery per row | loomloom workbook |

Gateway = execution primitive. loomloom = workflow orchestration. Image Lab
starts at the primitive because "several alternatives of one prompt" is several
independent calls, and *graduates* into a loomloom workflow as the work gains
structure. See `showcase/README.md`.

## What this is — and isn't

**It is:** a standalone skill that turns any image prompt into a parallel,
cost-gated set of alternatives across the best models for the brief; a loomloom
community example that graduates into a loomloom workflow.

**It isn't:** a prompt writer (bring your own), an image editor, a model
benchmark, or a replacement for native image generation when you want one quick
image. It makes you approve a cost on purpose.

## Help us test this

Once it runs against your prompts, feedback wanted:

- Is the one cost gate in the right place? Does the allocation feel right, or do
  you keep overriding it?
- Does the PLAN + "why" give you enough to approve confidently?
- What creative intent is missing from `generation-policy.md`?
- Where should it stop and ask, and where should it just run?

## Credits & licence

Apache-2.0, like the rest of loomloom. Pairs naturally with
[`ai-image-prompts-skill`](https://github.com/YouMind-OpenLab/ai-image-prompts-skill)
(YouMind, MIT) for the prompt, used unmodified. Intent-based model selection is
inspired by `runcomfy-com/skills` (MIT); the exploration page follows
`redesign-lab`'s case-study house style. `references/model-catalog.yaml`
is our own stopgap until the gateway ships a model endpoint.
