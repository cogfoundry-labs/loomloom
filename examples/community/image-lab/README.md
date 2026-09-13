# Image Lab

**What if you didn't have to pick an image model?**

You bring a prompt. Image Lab classifies what kind of image it is, works out which of 9 models actually fit that brief, shows you the estimated total, and — once you approve — generates several alternatives across those models in parallel. You see what it really cost, then choose the image you want from a gallery it builds for you.

Built as a Claude Code skill on top of [loomloom](https://github.com/cogfoundry-labs/loomloom) — but loomloom is optional here. v0.1 is plain parallel calls to a model gateway; loomloom only enters at v0.4, when the work becomes a real pipeline. If you just want several strong alternatives of one prompt, you don't need loomloom at all.

> "Pick an image, not a model."

## See it in action

One real run, start to finish — a One Piece-style 30-second storyboard sheet, eight ways.

<div align="center">
  <video src="https://github.com/user-attachments/assets/f1d670f1-d122-421b-bfa8-89624cdcce33" width="100%" controls></video>
</div>

> **Zoro & Robin storyboard** — one brief classified as `infographic / diagram`, `count 8` → **GPT Image 2 ×3 + Nano Banana Pro ×3 + Nano Banana 2 ×2**. Eight complete storyboard sheets, three models, **$0.5547** total. 9:16, ~14 s, rendered straight from the case study's own images. → [Full case study](https://maxaibuilds.github.io/zoro-robin-storyboard/)

The case study shows everything the run actually produced — every alternative, the model behind each one, the size each ran at, the per-image and total cost, and the exact prompt. Nothing staged, nothing a screenshot.

## Why I built this

I'm not a prompt engineer and I don't have a favourite image model. I kept doing the same thing anyway: pick whichever model I used last, run it four times, squint at the results, run it four more. Trying a *different* model meant another tab, another mental model of its quirks, another guess at what it would cost — so I mostly didn't.

That turned into a question I wanted an answer to:

> **If a good image is often just one model away, why is trying several so much friction?**

Image Lab is my attempt at removing that friction without hiding the cost. It doesn't write your prompt and it doesn't tell you which image is best — it spreads your prompt across the models that fit the brief, makes you approve one number, and hands you the choices.

I didn't invent a model-ranking system. The scoring is deliberately dumb and auditable — a weighted dot product you can read in about forty lines — precisely because I'm not the person who should be secretly deciding which model wins.

## Try it yourself

You don't need to understand the workflow before trying it.

**1. Install**

```bash
npx skills add cogfoundry-labs/loomloom --skill image-lab -a claude-code -g -y
```

No `git clone`, no `cd`.

**2. Give Claude a prompt**

```
generate that with Image Lab
```

with a prompt already in the conversation, or spell it out:

```
8 options of this with Image Lab —
"isometric 3D illustration of a developer workflow, soft studio lighting, muted palette, 16:9"
```

or force a head-to-head:

```
with Image Lab, compare Nano Banana Pro and GPT Image 2 on this
```

It triggers **only** when you name Image Lab — never on a bare "generate an image" — so it stays out of the way of native image generation and other image skills.

**What you'll get**

Image Lab will:

1. **PLAN** — classify the creative intent, then a deterministic scorer picks which models fit and how many alternatives each gets
2. **QUOTE** — the estimated total, printed by the same step
3. **APPROVE** — one yes/no. Nothing is spent before this. It is the only gate.
4. **GENERATE** — the whole allocation as independent gateway tasks, in parallel, in the background, reporting `N/8 done` as each lands
5. **RESULTS** — every image, the real cost, **and** a shareable exploration page built automatically — one brief, every alternative, a click-to-compare gallery you pick your winner from

You don't tell Claude how to do any of these steps. You make the one decision that spends money, and the one that picks the image.

**Nothing else to install first.** `scripts/image.py` checks the loomloom CLI, your token, and your balance at PLAN and stops with the single fix if anything's missing — someone who provides a prompt and stops at APPROVE never has to set anything up.

Want the source? `image-lab` lives inside the `loomloom` repo as a community example, not its own standalone repo:

```bash
git clone https://github.com/cogfoundry-labs/loomloom
cd loomloom/examples/community/image-lab
```

## Pick an image, not a model

Most "generate a few options" experiments look like this:

```
prompt
  ↓
pick one model
  ↓
generate ×4
  ↓
pick one
```

Image Lab takes a different path:

```
prompt
  ↓
Model Advisor  →  which models fit this brief, how many each
  ↓
estimate  →  you approve one number
  ↓
generate in parallel, across models
  ↓
gallery  →  you pick the image
```

It is **not** a model benchmark. Different models, different seeds, different sizes — the outputs are an unfair comparison on purpose. What you get is *more chances to find a great image, from genuinely different models*, for a cost you saw before you spent it.

## What actually happens

```
prompt
   │
   ▼
PLAN  →  QUOTE
   │
   ▼
APPROVE ──▶ THE ONE GATE — the wallet boundary
   │
   ▼
GENERATE  (N tasks across 1–3 models, in parallel)
   │
   ▼
RESULTS  →  exploration page (built automatically, spends nothing)
   │
   ▼
you pick your image ── or adjust the prompt and go around again
```

One gate, not zero — and it's the one that matters, the point where money gets spent. Picking a favourite at RESULTS is just conversation; the run is already done. Adjusting the prompt instead sends you back through the *whole* loop — a fresh QUOTE and a fresh APPROVE — as another round of the same session, never a shortcut that skips the gate. Each round gets its own folder (`./out/round-1`, `./out/round-2`, …), so nothing from an earlier round is ever overwritten, and once you've done a couple of rounds, one shareable page can show all of them side by side.

`count` is your exploration budget — **1, 2, 4, or 8**, default **4**:

| count | meaning | typical allocation |
|---:|---|---|
| 1 | just make one | the single best-fit model |
| 2 | a quick comparison | the top 2 models, one each |
| **4** | **standard exploration** | **top 2 models, two each** |
| 8 | deep exploration | top **3** models — A ×3 + B ×3 + C ×2 |

Say it naturally — *"just one"*, *"give me a couple"*, *"explore this"*, *"make 8"*. There is no slider. The allocation is deterministic; the images stay stochastic.

## How model selection works

Three files, deliberately separate:

- **`references/generation-policy.md`** — for each creative intent, a set of per-dimension requirement weights over Arena.ai's own 8 Text-to-Image Arena categories (`overall`, `commercial_design`, `three_d_modeling`, `cartoon`, `photorealistic`, `art`, `portraits`, `text_rendering`, each `low`/`medium`/`high`). **No model names.** This is Image Lab's *taste*, and it's meant to be hand-edited.
- **`references/arena-scores.yaml`** — the only hand-entered quality data: each model's real Elo rating + published confidence margin per Arena category, pulled from [arena.ai/leaderboard/text-to-image](https://arena.ai/leaderboard/text-to-image). `image.py` computes each model's `-1`…`2` score from this at load time via confidence-interval-aware tiering — nobody hand-types a score.
- **`references/model-catalog.yaml`** — each model's measured rate, size minimum, and real request shape. A temporary adapter: it exists only until the gateway ships a model endpoint.

`scripts/image.py` then, per run:

1. **disqualifies** any model that is weak (`-1`) at a `high` requirement
2. ranks the rest by **suitability** — the weighted dot product (cost is *not* in it)
3. picks **A** = top suitability (exact ties broken by lower cost)
4. picks **B** = the next-best surviving model — preferring a different provider than A when a credible one exists (within 1 confirmed Arena tier), otherwise the runner-up regardless of provider — honestly labelled either way
5. at `count = 8`, picks **C** = the third-best surviving model, same diversity preference, for a wider net
6. splits the `count`: `1 → A`, `2 / 4 → A + B`, `8 → A ×3 + B ×3 + C ×2`

Every candidate is priced **at the size it would actually run** — each model's own real size shape (a literal WxH, a resolution tier, or no size control at all) and GPT Image 2's size-dependent rate all count. `image.py resolve --explain` prints the whole score table, so *"why these models?"* always has a printable answer.

## The case-study page

RESULTS always runs `scripts/build-exploration-page.py` — it costs nothing, and it's what you pick your favourite from. It turns the session into one self-contained static folder, at **one address that never moves** no matter how many rounds you run:

```
<slug>/
  index.html          an image-selection gallery: framed hero + thumbnail strip + lightbox
  case-study-data.json  the data model (a future PDF / social-card renderer reads this)
  assets/…            every published round's images
```

An **image-selection gallery, not a benchmark report**: a topbar with a Share cluster (`Copy link` + one-click `X` / `LinkedIn`); a hero with an *"AI-generated · not a benchmark"* badge and a model legend; a thumbnail strip badged by **letter** (`A · GPT Image 2` — candidates, not steps); a click-to-compare hero with an `N / 8` counter and a per-alternative deep link (`…/index.html#E` opens straight to candidate E); a minimal full-screen lightbox; then **The run** (a facts grid), the full **Prompt** with a 3-step *"how to use this"*, **How this was made** (every model + tool, linked, with the real cost), and a **CTA**. Open Graph tags make a pasted link unfurl with the image.

It's in [`redesign-lab`](../redesign-lab)'s case-study house style — same token block, same Arial-Black headings and IBM Plex Mono labels, same hard edges. GitHub-Pages-ready, or published as a one-file artifact for a live URL. Curated ones live in [`case-studies/`](./case-studies/); some also carry a short 9:16 video, rendered from a [Remotion](https://remotion.dev) project alongside them.

Adjusted the prompt and gone a second (or third) round? The **same page** just grows: it now defaults to showing the latest round, and a nav strip above the gallery (`← VIEW ROUND 1  ·  ROUND 2 / 3  ·  VIEW ROUND 3 →`) switches to any adjacent round in place — same URL, no reload, no separate comparison page to maintain.

## What's in this folder

| Path | What it is |
|---|---|
| `SKILL.md` | Entry point — trigger, contract, the five steps |
| `docs/design-spec.md` | Why it's shaped this way — the Advisor, the two layers, the roadmap |
| `pipelines/generate.yaml` | The stage manifest the agent follows |
| `references/generation-policy.md` | **Durable:** creative intent → capability requirements + sizes. Hand-editable — bring your own taste. |
| `references/arena-scores.yaml` | **The only hand-entered quality data:** real Arena.ai Elo + margin per model per category. Re-measuring means re-pulling this file, never editing a score. |
| `references/model-catalog.yaml` | **Temporary adapter:** model ids, measured rates, real request shape. `TODO`: replace with a gateway endpoint. |
| `references/exploration-page.md` | how the shareable page is built — every flag + its section-by-section layout |
| `scripts/image.py` | `resolve` / `run` — the whole generation engine; standard-library Python, no SDK |
| `scripts/build-exploration-page.py` | builds/rebuilds the one case-study page for a session (`--session`), at a fixed address that grows to embed each new round; no spend |
| `test-fixtures/sample-prompts.json` | prompts + expected intent/allocation, for exercising PLAN without spend |
| `case-studies/<slug>/` | curated, committed exploration pages from real runs — self-contained, GitHub-Pages-ready |
| `showcase/` | **v0.4** — Image Lab as a real loomloom workflow (not wired into v0.1) |

## Bring your own taste

This is the part I'd most like people to push on. Image Lab separates the *workflow* (classify, allocate, gate, generate, gallery) from the *taste* (what each kind of image needs, which dimensions matter). The workflow doesn't know or care what's in `generation-policy.md`:

```
Image Lab  +  your generation policy  →  your model choices, your exploration
```

You don't have to agree with the default weights. Add an intent, re-weight `text_rendering` for posters, decide that `commercial_design` matters more than the catalog assumes — same gate, same gallery, same case-study output. The scores themselves come from `arena-scores.yaml`'s real Elo data, not a hand-picked number, so re-measuring a model's quality means re-pulling that file from arena.ai; the request shapes and rates in `model-catalog.yaml` are just as editable, and are a stopgap until the gateway can answer *"which models, and how much"* itself.

## Where loomloom fits

v0.1 doesn't touch loomloom's compiler or runtime. "Several alternatives of one prompt" is N calls with no dependencies between them, so it uses the gateway directly:

```
PLAN → QUOTE → APPROVE → GENERATE → RESULTS → exploration page
```

That's the whole skill today, for the cost of the images. loomloom arrives at **v0.4**, when the work becomes a dependency-aware pipeline — tidy → generate ×N → judge — with one run record:

| Version | Adds | Layer |
|---|---|---|
| **v0.1 (this)** | any prompt → 1/2/4/8 alternatives across best-fit models → estimate → approve → gallery → shareable page; 9 models | gateway |
| v0.2 | retry failed branches; natural-language allocation tuning | gateway |
| v0.3 | an LLM judge that ranks the alternatives (the first step dependency) | gateway + local LLM |
| **v0.4** | tidy → generate ×N → judge as one loomloom TemplateSpec, with a run record | **loomloom** |
| v0.5–v0.6 | reference image + edit chain; batch a file of prompts | loomloom |

Gateway = execution primitive. loomloom = workflow orchestration. Image Lab starts at the primitive and *graduates*. See [`showcase/README.md`](./showcase/README.md).

## What this is — and isn't

**It is:** a real Claude Code skill you can install today; a way to try several image models on one brief without pricing anxiety; a pluggable generation policy; a working example of a loomloom primitive that graduates into a loomloom workflow.

**It isn't:** a prompt writer (bring your own), an image editor, a model benchmark or leaderboard, or a replacement for native image generation when you just want one quick picture. It makes you approve a cost on purpose.

## Help me test this

Once it's run against your prompts, I want to know:

- Is the one cost gate in the right place? Does the allocation feel right, or do you keep overriding it?
- Does the PLAN + "why" line give you enough to approve confidently?
- What creative intent is missing from `generation-policy.md`?
- Where should it stop and ask, and where should it just run?

And if the whole shape seems wrong to you, I'd like to hear that too. Open an issue, or find me in [Show and tell](https://github.com/orgs/cogfoundry-labs/discussions/categories/show-and-tell).

## Built on open-source work, with real thanks

- **The prompt** — [`ai-image-prompts-skill`](https://github.com/YouMind-OpenLab/ai-image-prompts-skill) (YouMind, MIT). Image Lab consumes a finished prompt; this pairs with it, used unmodified, and its curated 10,000+ prompts are a good place to start.
- **The idea** — intent-based model selection is inspired by `runcomfy-com/skills` (MIT).
- **The models and the real cost** — [CogFoundry's model gateway](https://cogfoundry.ai): Google's Nano Banana family, OpenAI's GPT Image 2, ByteDance's Seedream family, and the authoritative per-image `cost` that RESULTS reports back.
- **The look** — the exploration page follows [`redesign-lab`](../redesign-lab)'s case-study house style; the showcase video is [Remotion](https://remotion.dev).

Thank you to every one of these projects and their maintainers. If you maintain one and want something changed about how it's credited or used here, open an issue and I'll fix it.

## Next step

If people find this genuinely useful, the thing I want to build next is a place where the generation policy, the model catalog, and eventually the judge are all things other people contribute and compare — so instead of one opinionated model picker, there's a workflow where different taste can be swapped in and argued about. That only makes sense to build if this first version holds up, which is what I'm trying to find out.

---

Apache-2.0, like the rest of loomloom.

[→ Try it yourself](#try-it-yourself) · [→ Case study](https://maxaibuilds.github.io/zoro-robin-storyboard/) · [→ Technical spec](docs/design-spec.md)
