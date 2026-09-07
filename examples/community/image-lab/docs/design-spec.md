# Image Lab — Design Specification (v1)

*One prompt → several strong image alternatives, spread across the models that
best fit the brief, behind a single cost gate. A loomloom community example
that starts at the gateway primitive and graduates into a loomloom workflow.*

> "See the price. Approve once. Get several strong possibilities in parallel.
> Pick the image you want."

**Status**: v0.1 implemented and verified against the live CogFoundry gateway —
`resolve` (plan + quote), `run` (the approved multi-model allocation), and
`build-exploration-page.py` (a shareable static gallery, no spend). Standard
library only, no SDK. **loomloom's role in v0.1**: none — the parallelism is N
independent gateway tasks; loomloom orchestration is the v0.4 `showcase/` story.

---

## 1. Product concept

You bring an image prompt — from
[`ai-image-prompts-skill`](https://github.com/YouMind-OpenLab/ai-image-prompts-skill),
any other prompt skill, or written by hand. Image Lab's Model Advisor spreads
your requested number of **alternatives** across the models that best fit the
brief, shows the estimated total, gets **one** approval, generates them in
parallel, and shows a gallery with the actual cost. You pick the image you want.

```
PLAN  ->  QUOTE  ->  APPROVE  ->  GENERATE  ->  RESULTS
```

- **`count` is an exploration budget — 1 / 2 / 4 / 8, default 4.** One is *just
  make one*; two is *a quick comparison*; four is *standard exploration*; eight
  is *deep exploration*. No slider — the user says "just one" / "a couple" /
  "explore this" / "make 8". **How that budget is spread across models is an
  internal Advisor policy** (§4) that can evolve without changing what `count`
  means.
- **Pick an image, not a model.** Image Lab is not a benchmark — different models
  and different seeds make the outputs an unfair comparison. It gives the user
  *strong choices*; multi-model generation is a user-value feature.
- **Not tied to any prompt source.** Image Lab consumes a prompt string; it never
  reads another skill's files and never writes or rewrites prompts.
- **The one gate is cost.** Nothing is submitted until the user approves the
  complete plan — even when it spends across two models. Picking a favourite at
  RESULTS is conversation, not a gate.

### What v0.1 is — and is not

| | v0.1 | else |
|---|:---:|---|
| Any prompt → 1 / 2 / 4 / 8 parallel alternatives | ✅ | |
| Multi-model allocation — A + B at count 2 / 4, A + B + C at count 8 (A alone only when it's the sole survivor) | ✅ | |
| 7 image models; deterministic scorer picks; user overrides by name | ✅ | |
| Per-model valid `size`; estimated total before spend; actual cost after | ✅ | |
| Live per-branch progress; failed branch reported (not charged) | ✅ | |
| Shareable exploration page (a static folder, no spend) | ✅ | |
| Retry failed branches / allocation-tuning UI | ❌ | v0.2 |
| LLM judge that ranks the alternatives | ❌ | v0.3 |
| loomloom TemplateSpec / run record / SkillBot | ❌ | v0.4 (`showcase/`) |
| Reference image · edit · batch | ❌ | v0.5–v0.6 |
| Counts other than 1 / 2 / 4 / 8; seed controls; model leaderboards | ❌ | never |
| Writing or rewriting the prompt | ❌ | never — bring your own |

**Cost note.** `count = 4` → A ×2 + B ×2 generally costs about **twice** what one
model ×4 would, because A and B are rarely the same price; `count = 8` →
A ×3 + B ×3 + C ×2 adds a third model's rate on top. This is accepted for v0.1:
the value is *more chances to find a great image, from genuinely different
models*, the estimated total is shown before approval, and the user can override
to a single model. A run is cents, not dollars, for every intent except the
typography-heavy ones where A is GPT Image 2 (infographic / poster at count 8
≈ $0.18–$0.37).

---

## 2. Two layers — and why v0.1 starts at the lower one

CogFoundry offers image generation at two levels, on the **same
`LOOMLOOM_TOKEN_COGFOUNDRY` and the same settled balance**:

| layer | what it is | Image Lab uses it |
|---|---|---|
| **Gateway API** — `POST /api/v1/tasks/generations` | the **execution primitive**: submit one task, poll it, get an image + a `cost` | v0.1–v0.2 — N independent tasks across ≤ 2 models need no orchestration |
| **loomloom** — TemplateSpec + runtime | **workflow orchestration**: a dependency-aware, metered DAG with one run record and a reusable, packageable spec | v0.4 — when the work becomes a real pipeline |

The task in v0.1 — several alternatives of one prompt, split across at most two
models — is N calls with **no dependencies between them**, so it uses the
primitive directly. Each later version adds structure until the work is a DAG
that only loomloom can express (tidy → generate ×N → judge, §5).

Why the gateway and not loomloom for v0.1:

| | loomloom TemplateSpec | gateway API |
|---|---|---|
| image models available today | 1 (`gemini-2.5-flash-image`) | **7** |
| measured cost / image | $0.04–$0.30 | **$0.003–$0.10** |
| `size` control | none | `size` param |
| a failed task | counts | **not charged** |
| a multi-model fan-out | not one metered job yet | just N `POST`s |

---

## 3. The workflow

`pipelines/generate.yaml` is a manifest the agent follows stage by stage (the
redesign-lab convention); it is not executed by loomloom.

| User step | Internal work |
|---|---|
| **PLAN** | agent captures the prompt, classifies it to one intent, maps the user's words to `count` → `image.py resolve` (reads the reference files, scores the models, allocates the count across the best-fit models, picks a valid `size` per model; runs `loomloom doctor` / `balance`; **no gateway call**) |
| **QUOTE** | the same `resolve` call prints per-model subtotals + estimated total + a preflight balance check (surfaced only if short) |
| **APPROVE** | one `AskUserQuestion` — Generate / Adjust / Stop — naming the actual model mix + total. **The only gate; one approval = one wallet boundary.** |
| **GENERATE** | `image.py run --alloc … --progress-file … --confirm` — submit the whole allocation as N gateway tasks, poll each (~3 s), download each image, verify it is a PNG. **Runs in the background**: it rewrites `progress.json` as each branch lands, and the agent posts a short "N/total done" update every ~30–60 s so the user is never left staring at nothing |
| **RESULTS** | agent sends each image as a labelled card, presents `label · model · time · cost · size` + actual-vs-estimate, **then builds the exploration page** and asks the user to pick their favourite from it (conversation, not a gate) |

"QUOTE" is a user-facing label — internally it is an *estimated total*
(`Σ price(model, size) × n`); the gateway has no quote endpoint. Balance is a
preflight check, not an authorization: `resolve` reports `sufficient: false` and
the agent stops before APPROVE; otherwise the gate stays uncluttered.

### The gate, as the user sees it

```
PLAN   4 alternative(s) for 'poster / flyer'
       Nano Banana Pro   x2   1024x1536   ~$0.0268
       GPT Image 2       x2   1024x1536   ~$0.0840
       Nano Banana Pro and GPT Image 2 both top the fit; weak-at-typography models are ruled out
QUOTE  Estimated total: ~$0.1108   (actual shown after the run)
APPROVE  [ Generate 4 — NB Pro ×2 + GPT Image 2 ×2, ~$0.11 ]  [ Adjust ]  [ Stop ]
         ... 4 tasks running across 2 models ...
RESULTS  Nano Banana Pro ($0.0268) [A][B]   GPT Image 2 ($0.0840) [C][D]   Actual: $0.11xx
```

A failed branch shows as `FAILED` with its `fail_reason`, is not charged, and is
**not** retried in v0.1. Partial success is more visible at `count = 8`;
RESULTS names every failed / incomplete branch and the user picks from what
landed.

---

## 4. The Model Advisor

Runs inside PLAN, deliberately split so the choice is **reproducible and
auditable** — not "an AI recommender":

```
prompt text
   │  agent: classify → one intent label (9 options; fuzzy; user can correct)
   │         + map the user's words to count (1 / 2 / 4 / 8, default 4)
   ▼
image.py (deterministic):
   generation-policy.md      → the intent's per-dimension requirement weights
   model-catalog.yaml → each model's per-dimension score + rate
   1. disqualify a model weak (−1) at any HIGH (weight 2) requirement
   2. suitability(model) = Σ weight_d · score_d          — the single ranking number
   3. A = highest suitability   (exact ties → lower cost)
   4. B = the next-best surviving model ("also worth trying")
   5. C = the third-best surviving model — used only to widen count 8
   6. allocate `count`: 1 → A · 2/4 → A + B · 8 → A + B + C
   ▼
plan: an allocation [{model, size, n, subtotal}] + estimated total + a "why" line
```

The agent's only jobs are **intent classification and the count** — both
low-stakes. Everything else is a ~40-line function, so *"why these two models?"*
has a printable answer (`image.py resolve --explain`).

### Suitability

- Requirement weight: `low`→0, `medium`→1, `high`→2.
- Model per-dimension score: `−1` weak, `0` neutral, `1` good, `2` strong.
- `suitability = Σ_d weight_d · score_d` — the weighted dot product. **Cost is
  not in it.**
- A `high` requirement + a `−1` model = disqualified.
- **A** = highest suitability; cost breaks only *exact* ties, so a free/cheap
  model never wins on price alone.
- **B** = the **next-best surviving model** — presented honestly as the runner-up
  ("also worth trying"), not claimed to be A's equal. There is no suitability
  floor on B: `count ≥ 2` means *at least two models* in v0.1.
- **C** = the **third-best surviving model**, brought in only at `count = 8`
  ("deep exploration") — a wider net when the user has asked for the most
  chances. `count = 4` deliberately stays on two models; C is the one place
  where a bigger `count` also means a broader model spread.
- A takes the whole budget only when it is the **sole** surviving model, or the
  user forced one model.

### Allocation policy (v0.1 — fixed)

| count | A + B (+ C) — surviving models | A only (A is the sole survivor / user forced one) |
|---:|---|---|
| 1 | A ×1 | A ×1 |
| 2 | A ×1 + B ×1 | A ×2 |
| 4 | A ×2 + B ×2 | A ×4 |
| 8 | A ×3 + B ×3 + C ×2  (A ×4 + B ×4 if only two survive) | A ×8 |

The split is **internal** — `count` means "alternatives I want", not "top-N
models". `count = 8` widening to three models is a UX call ("deep exploration
should also mean a broader net"), not a change to the contract; a future Advisor
could tune the `8 → 3+3+2` shape freely. User override
(`--models "id[,id[,id]]"`) forces the set and splits `count` evenly across it
(front-loaded); the Advisor never silently replaces a named model. Naming more
models than `count` drops the surplus and reports `dropped_models`.

### Worked allocations (count = 4, current catalog — deterministic)

| intent | allocation | ~total |
|---|---|---:|
| illustration / concept art | Nano Banana ×2 + Nano Banana 2 ×2 | $0.018 |
| launch / announcement image | Nano Banana Pro ×2 + Nano Banana 2 ×2 | $0.039 |
| social post | Nano Banana 2 ×2 + Nano Banana ×2 | $0.018 |
| poster / flyer | Nano Banana Pro ×2 + GPT Image 2 ×2 | $0.111 |
| infographic / diagram | GPT Image 2 ×2 + Nano Banana Pro ×2 | $0.111 |
| product / e-commerce shot | Seedream 5.0 Lite ×2 + Nano Banana 2 ×2 | $0.086 |

Every intent lands on two models at count 4 with the current 7-model catalog —
none has a sole survivor. At **count 8** each of these widens to three, e.g.
infographic → GPT Image 2 ×3 + Nano Banana Pro ×3 + Nano Banana 2 ×2 (~$0.18),
poster → Nano Banana Pro ×3 + GPT Image 2 ×3 + Seedream 5.0 Pro ×2 (~$0.37).
`image.py resolve --explain` shows the full score table and why A / B / C were
picked.

`test-fixtures/sample-prompts.json` pins `expected_allocation_4` and
`expected_allocation_8` for four of these against the current reference files —
run `resolve` at each count and diff.

### When a model is unavailable — "plan changed", not "fallback"

Availability is **never probed**. If the *first* task's model is rejected before
any spend (`503 "supported model not found"`), `image.py` re-scores without it
and exits `3` with a suggested replacement; the agent re-runs
`resolve --models "<suggested>"` and re-approves — a second approval is warranted
because the plan changed. A *later* model failing to submit marks those branches
`FAILED` (not charged) and the rest proceed.

### Picking a `size`

`preferred_sizes` are literal `WxH` strings — the gateway's actual parameter.
The Advisor takes the first entry that clears the chosen model's `size_min_px`;
if none do, it upsizes to the smallest valid dimensions at that aspect ratio.
**Size feeds pricing**: each candidate is costed at the size it would actually
run — so Seedream's forced upsize to ≥ 3.69 MP, and `gpt-image-2`'s
size-dependent rate (~$0.006 at 1 MP → ~$0.042 at 1.5 MP), are compared on real
cost.

---

## 5. Reference files

Two data files, deliberately separate, plus one operational reference.

### `references/generation-policy.md` — durable

Intent → per-dimension requirement weights + preferred sizes. **No model
names.** A team edits this to change Image Lab's taste; it does not rot when the
model catalog moves. Dimensions (v0.1): `photorealism`, `typography`,
`composition_control`, `speed` — a fixed vocabulary shared with the catalog's
`scores` block.

### `references/model-catalog.yaml` — temporary adapter

Exists **only because the gateway API has no model-list or pricing endpoint**.
It is not part of Image Lab's design — it is a hand-maintained shim, isolated
behind `load_model_catalog()`. Its header carries a `TODO` to delete it when the
gateway ships `GET /api/v1/models?modality=image`. Per model: `label`, `url` (the
CogFoundry page), `scores` (−1…2 per dimension), `usd_per_image` (+ optional
size-tiered `pricing`), `size_min_px`.

Rates were measured 2026-09-06 by submitting one real task per (model, size) and
confirming the charge against `loomloom balance` — the gateway's `data.cost`
matched the delta every time. They are a **pre-flight guess** of that
authoritative number; a wrong rate only skews the estimate, and RESULTS shows
the real charge.

### `references/exploration-page.md` — operational

The agent-facing "how to run `build-exploration-page.py`" — every flag, the
folder it writes, the page's section-by-section layout. §6 below is the *why*;
that file is the *how*, so `SKILL.md` step 7 can stay short.

---

## 6. The exploration page

`run` writes `<out>/run.json` — the record: `prompt`, `intent`, `actual_usd`,
`estimated_usd`, `out_dir`, and `alternatives[]` (`index`/`of`, `label`,
`model` + `model_label` + `model_url`, `requested_size`, `actual_size` from the
PNG header, `cost_usd`, `seconds` from the gateway's `finish_time − start_time`
epochs, `status`, `file` — a **basename**, no absolute paths, `note`), plus
derived `by_model` / `images` / `failed` / `incomplete` views.

RESULTS **always** runs `build-exploration-page.py --from <out> --title …
--subject … --invocation … [--canonical-url …] [--selected <label>]` — it is
never a "do you want a page?" offer, because it costs nothing and the user picks
their favourite *from* the page. It turns the run into a **self-contained static
folder** (`<out>/<slug>/index.html` + `assets/alternative-NN.png` +
`exploration.json`), following redesign-lab's build → render seam:
`exploration.json` is the data model a future PDF / social-card renderer reads;
the folder ships **real separate asset files**. `--inline` additionally emits
`index.inline.html` — the same page with the PNGs as base64 data URIs (stdlib
`base64`, no recompression) — which the agent publishes as an artifact so the
user has a live URL to choose from; if the run's PNGs push that past the 16 MB
artifact ceiling the agent recompresses them to JPEG first. `--canonical-url` is
the URL the folder will live at — it makes `og:image` absolute and the topbar
"Copy link" copy that URL (omitted for the artifact flow, where `location.href`
suffices). `--selected` is omitted on the first build and added on a rebuild
once the user names a pick.

The page is an **image-selection gallery, not a case-study report**, in
redesign-lab's house style — the *same token block* as
`maxaibuilds.github.io/aider-redesign` (`--bg #f4f4f0` / `--ink #0b0b0b` /
`--accent #007a5c` / `--rule #d8d6ce` / `--shadow`, Source Serif 4 body, Arial-
Black uppercase headings, IBM Plex Mono labels, 3-state dark mode, Google Fonts
with real fallbacks). Layout:

- **topbar → header → Gallery → Details → Reuse → Provenance → Roadmap → CTA →
  footer.** Page column 1200 px; every prose section wraps head + body in `.col`
  (820 px) so they align to one left edge — the gallery is the only full-width
  section. **Every section has one shape**: mono eyebrow (`.section-label`),
  Arial-Black `h2`, one `.lead` line, then the body. One ghost-button style
  (`.btn-ghost`) and one solid-button style (`.btn-solid`) across the whole page.
- **topbar**: wordmark + a **Share** cluster — `Copy link` (copies `link[rel=
  canonical]` if `--canonical-url` was given, else `location.href`) plus
  one-click `X` and `LinkedIn` `<a>`s whose hrefs are built on load from that
  URL + the OG `title`/`description`.
- **header**: an `AI-generated · not a benchmark` badge, the intent as the
  eyebrow, the title, the tagline, a **model legend** (`GPT Image 2 ×3` mono
  chips, each linked to its cogfoundry.ai page — the analog of the case study's
  hero colour swatches), the stats line.
- **Pick your image**: section label + a one-line lead ("… each a candidate, not
  a step"). A framed hero, a **letter-badged** thumbnail strip (`A · GPT Image
  2`, not `01` — numbers read as *steps* over storyboard art), a live
  `Alternative N · model (link) · size (asked …) · cost · time` line, a hint row
  (`click to open full size · ← → to browse`), an `N / 8` counter, and a **"Copy
  link to alternative N"** button.
- Click a thumbnail → hero + meta swap and the URL hash becomes `#N`. Click the
  hero (or the corner **"↗ Open full size"** button) → a **minimal full-screen
  lightbox**: the image, `Alternative N · model`, `N / 8`, `← →`, `Esc`, `Copy
  link`, and an **"Open raw ↗"** escape hatch (the artifact sandbox blocks
  downloads, so the raw-file link is how a viewer saves an image). Hero edges
  carry subtle prev/next arrows.
- **Per-alternative deep link.** The page reads `location.hash` on load *and*
  `hashchange`, selecting that alternative — so a shared `…/index.html#E` lands
  on candidate E, not the menu. `history.replaceState` keeps the hash current as
  the user browses (wrapped in try/catch for the sandbox).
- **The run**: a hairline facts grid (`gap:1px; background:var(--rule)`, cells
  `var(--surface)` — the case study's "this is data" texture) — intent · models ·
  candidates · requested size · actual cost (+ the estimate, small) · date.
- **Reuse / The prompt**: the exact `run.json` prompt in full, italic serif — the
  reusable artifact — with a *"→ use Image Lab"* trigger, a Copy button (full
  invocation), and a 3-step **"how to use this"** list (copy → paste into an
  agent that has the skill / install it → run as-is or swap details to make it
  yours) — the actionable framing learned from YouMind's prompt pages.
- **How this was made**: a `.tool-list` — the gateway (with the run's real total),
  every model used (linked, with its alternative labels), and Image Lab itself
  (repo link). Radical credit transparency, straight from the case study.
- **Coming soon**: one paragraph — *"From image exploration to AI work"* —
  pointing at the v0.4 loomloom agentic workflow (generate → evaluate → select →
  refine). Static copy, no per-run data.
- **CTA**: *"Bring your own prompt"* + the `npx skills add …` line + a GitHub
  button — a shared page converts into a new user instead of dead-ending.
- **Open Graph / Twitter-card meta** (`og:image` = the selected or first asset;
  absolute when `--canonical-url` is set) so a pasted link unfurls with the
  image, and the topbar `X` / `LinkedIn` share links have something to carry.
  Stripped from `index.inline.html` (scrapers ignore `data:` URIs).
- ~180 lines of inline vanilla JS, no library. With JS off the hero is still a
  plain link to the full-resolution file; everything else (thumbnail strip,
  lightbox, Share cluster) is progressive enhancement. `rebuildMeta()` in the
  script mirrors `_meta_html()`.
- Initial hero = the `#hash` alternative, else `--selected`, else the first. The
  creator's pick carries a permanent ✓ badge and a *"the creator's pick"* prefix
  on the meta only while it is the hero — never "Best", never a score.
- Failed branches are omitted from the gallery with one line beneath
  (*"2 of 4 generated — alternatives C, D did not (…)"*).
- Tagline + stats composed from the run's real numbers (the actual charge; the
  estimate lives in the conversation RESULTS and the facts grid, not the hero).
- Footer: one mono line — *"Generated with CogFoundry's model gateway ·
  cogfoundry.ai · <date>"* — never the token.

What it deliberately does **not** take from the case study: the scrollytelling
depth, the multi-chapter narrative, the before/after compare widget, per-chapter
`embed/*.html` files. Image Lab's page is a *tool* (pick an image), kept
interaction-first and scannable.

Robustness (from a pre-PR code review): a moved `<out>/` still renders (basename
fallback); assets are re-verified before the page states a count; `--selected`
on a failed branch → no Selected section, never a broken `<img>`.

---

## 7. Verified gateway-API facts (2026-09-06)

- **Endpoint**: `POST https://router.cogfoundry.ai/api/v1/tasks/generations`
  (async) → `GET .../{request_id}` (poll). Statuses
  `SUBMITTING|PENDING|IN_PROGRESS|COMPLETED|FAILED`.
- **Cloudflare** 403s the default `Python-urllib/*` User-Agent ("error code:
  1010") — `image.py` sends a custom UA on every request.
- **Auth**: `Authorization: Bearer <the loomloom token>`. No auth → `401`.
  Unknown model → `503 "supported model not found"`.
- **Cost**: `data.cost` (USD) on `COMPLETED`, authoritative — matched the
  `loomloom balance` delta on every measured run. `FAILED` tasks cost nothing.
- **Pricing shape**: Google + OpenAI bill per image; only `openai/gpt-image-2`
  changes price with output size. `google/gemini-3.1-flash-image` billed
  **$0.00** on 2026-09-06 (a launch preview — the catalog quotes it at a
  conservative $0.006 so the cost gate stays honest). Seedream rates
  ($0.037–$0.10) are measured at the forced ≥ 3.69 MP size.
- **Timing**: `submit_time` / `start_time` / `finish_time` are epoch seconds;
  `finish_time − start_time` is the true generation time.
- **Images**: `data.data.image_urls[]` are presigned OSS URLs that **expire** —
  download on completion. `run` checks the PNG magic bytes so an OSS error body
  is a failed branch, not a broken image.
- **`size` is per-model**: Gemini accepts `1024x1024`; Seedream rejects it. The
  Gemini Flash models also silently return a different aspect ratio than
  requested — `actual_size` records what landed.
- **No model-list endpoint** — `/api/v1/models` returns only chat/LLM models.

| Family | `model` id | CogFoundry page | $/img (measured) |
|---|---|---|---|
| Nano Banana | `google/gemini-2.5-flash-image` | `/models/media/185/` | 0.0032 |
| Nano Banana 2 | `google/gemini-3.1-flash-image` | `/283/` | 0.00 billed / 0.006 quoted |
| Nano Banana Pro | `google/gemini-3-pro-image` | `/241/` | 0.0134 |
| GPT Image 2 | `openai/gpt-image-2` | `/314/` | 0.0059 → 0.042 (by size) |
| Seedream 5.0 Pro | `bytedance/doubao-seedream-5-0-pro` | `/352/` | 0.1015 |
| Seedream 5.0 Lite | `bytedance/doubao-seedream-5-0-lite` | `/281/` | 0.0371 |
| Seedream 4.5 | `bytedance/doubao-seedream-4.5` | `/248/` | 0.0422 |

---

## 8. Auth

`image.py` resolves the token from, in order:
`LOOMLOOM_TOKEN_COGFOUNDRY` / `LOOMLOOM_TOKEN` env var → the `token` field of the
active profile in `%APPDATA%/loomloom/config.json` /
`~/.config/loomloom/config.json` → else one clear instruction. `loomloom doctor`
/ `loomloom balance` cover readiness and balance. The token is only ever sent in
the `Authorization` header — never logged, never written to `run.json`.

---

## 9. Layout & roadmap

```
examples/community/image-lab/
  README.md  SKILL.md  .gitignore
  pipelines/generate.yaml
  references/generation-policy.md        # DURABLE  (intent -> weights)
  references/model-catalog.yaml          # TEMPORARY ADAPTER  (model scores + rates)
  references/exploration-page.md         # OPERATIONAL  (how to run the page build)
  scripts/image.py                       # resolve | run
  scripts/build-exploration-page.py      # run.json → shareable folder (no spend)
  test-fixtures/sample-prompts.json
  case-studies/<slug>/                   # curated, committed exploration pages (see below)
  showcase/                              # v0.4 stub — Image Lab AS a loomloom workflow
```

No runtime dependencies (`scripts/` is standard-library only). **`out/` is
scratch and gitignored** — every skill run publishes its page as a private
artifact, not a commit. **`case-studies/<slug>/` is a curated, committed
showcase** — a maintainer picking one run worth keeping and committing its
self-contained folder (`index.html` + `exploration.json` + `assets/` packed to
JPEG q95, full resolution, ~3–4 MB), exactly the way `redesign-lab/case-studies/`
works, so it can be hosted as a small standalone site. `case-studies/pack.py`
(a Pillow-based maintainer tool, not part of the runtime) does the packing. See
`case-studies/README.md`.

| Version | Adds | Layer |
|---|---|---|
| **v0.1** | 1/2/4/8 alternatives across best-fit models → quote → one approval → gallery → shareable page; 7 models | gateway API |
| v0.2 | retry failed branches; natural-language allocation tuning | gateway API |
| v0.3 | an LLM judge that ranks the alternatives (first step dependency) | gateway + local LLM |
| **v0.4** | tidy → generate ×N → judge as one loomloom TemplateSpec, with a run record | **loomloom** |
| v0.5–v0.6 | reference image + edit chain; batch a file of prompts | loomloom |

### `showcase/` (v0.4)

Where Image Lab *is* a loomloom workflow. The pitch: *the gateway can fan out N
calls; it cannot run tidy → generate → judge as one dependency-aware metered job
with a single run record.* `variants-4.spec.json` (already
`loomloom template-spec check` → valid) is the seed — four `image-generate`
branches; the `stp_tidy` / `stp_judge` steps and the wiring are the v0.4 build.

---

## Appendix — gateway API reference

```bash
TOK=$LOOMLOOM_TOKEN_COGFOUNDRY

curl -s -X POST https://router.cogfoundry.ai/api/v1/tasks/generations \
  -H "Authorization: Bearer $TOK" -H "Content-Type: application/json" \
  -d '{"model":"google/gemini-2.5-flash-image","prompt":"...","size":"1024x1024","watermark":false}'
# → { "data": { "request_id": "...", "status": "SUBMITTING", ... } }

curl -s https://router.cogfoundry.ai/api/v1/tasks/generations/$REQUEST_ID \
  -H "Authorization: Bearer $TOK"
# → { "data": { "status": "...", "progress": "0%..100%", "cost": 0.0032,
#               "start_time": <epoch>, "finish_time": <epoch>, "fail_reason": "",
#               "data": { "image_urls": ["https://...aliyuncs.com/...png"] } } }
```

## Attribution & prior art

- `ai-image-prompts-skill` (YouMind, MIT) — the natural prompt pairing, used
  unmodified.
- `runcomfy-com/skills` (MIT) — design inspiration for intent-based model
  selection.
- `examples/community/redesign-lab` — structural precedent: manifest stages, one
  gate that matters, a clearly-scoped paid step, the exploration-page house style.
- CogFoundry gateway API — not publicly documented; behaviour here is from direct
  verification on 2026-09-06.
