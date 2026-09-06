# Image Lab — Design Specification (v1)

*One prompt → several strong image alternatives, spread across the models that
best fit the brief, behind a single cost gate. A loomloom community example
that starts at the router primitive and graduates into a loomloom workflow.*

> "See the price. Approve once. Get several strong possibilities in parallel.
> Pick the image you want."

**Status**: v0.1 implemented and verified against the live CogFoundry router —
`resolve` (plan + quote), `run` (the approved multi-model allocation), and
`build-exploration-page.py` (a shareable static gallery, no spend). Standard
library only, no SDK. **loomloom's role in v0.1**: none — the parallelism is N
independent router tasks; loomloom orchestration is the v0.4 `showcase/` story.

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
| Multi-model allocation (A alone, or A + a competitive B) | ✅ | |
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
model ×4 would, because A and B are rarely the same price. This is accepted for
v0.1: the value is *more chances to find a great image, from genuinely different
models*, the estimated total is shown before approval, and the user can override
to a single model. A run is cents, not dollars, for every intent except the
deliberately-expensive single-model ones (infographic → GPT Image 2 ×4 ≈ $0.17).

---

## 2. Two layers — and why v0.1 starts at the lower one

CogFoundry offers image generation at two levels, on the **same
`LOOMLOOM_TOKEN_COGFOUNDRY` and the same settled balance**:

| layer | what it is | Image Lab uses it |
|---|---|---|
| **Router API** — `POST /api/v1/tasks/generations` | the **execution primitive**: submit one task, poll it, get an image + a `cost` | v0.1–v0.2 — N independent tasks across ≤ 2 models need no orchestration |
| **loomloom** — TemplateSpec + runtime | **workflow orchestration**: a dependency-aware, metered DAG with one run record and a reusable, packageable spec | v0.4 — when the work becomes a real pipeline |

The task in v0.1 — several alternatives of one prompt, split across at most two
models — is N calls with **no dependencies between them**, so it uses the
primitive directly. Each later version adds structure until the work is a DAG
that only loomloom can express (tidy → generate ×N → judge, §5).

Why the router and not loomloom for v0.1:

| | loomloom TemplateSpec | router API |
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
| **PLAN** | agent captures the prompt, classifies it to one intent, maps the user's words to `count` → `image.py resolve` (reads the reference files, scores the models, allocates the count across the best-fit models, picks a valid `size` per model; runs `loomloom doctor` / `balance`; **no router call**) |
| **QUOTE** | the same `resolve` call prints per-model subtotals + estimated total + a preflight balance check (surfaced only if short) |
| **APPROVE** | one `AskUserQuestion` — Generate / Adjust / Stop — naming the actual model mix + total. **The only gate; one approval = one wallet boundary.** |
| **GENERATE** | `image.py run --alloc … --confirm` — submit the whole allocation as N router tasks, poll each (~3 s), redraw the live tree, download each image, verify it is a PNG |
| **RESULTS** | agent sends each image as a labelled card, presents `label · model · time · cost · size` + actual-vs-estimate; user names a favourite (conversation, not a gate) |

"QUOTE" is a user-facing label — internally it is an *estimated total*
(`Σ price(model, size) × n`); the router has no quote endpoint. Balance is a
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
   router-model-catalog.yaml → each model's per-dimension score + rate
   1. disqualify a model weak (−1) at any HIGH (weight 2) requirement
   2. suitability(model) = Σ weight_d · score_d          — the single ranking number
   3. A = highest suitability   (exact ties → lower cost)
   4. B = best OTHER model within QUALITY_TOLERANCE (1 pt) of A, or none
   5. allocate `count` across A (+ B)
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
- **B** = the best *other* model within `QUALITY_TOLERANCE` (1 pt) of A — the
  "also worth trying" pick — or none, and then the whole `count` goes to A.

### Allocation policy (v0.1 — fixed)

| count | A + B present | A only |
|---:|---|---|
| 1 | A ×1 | A ×1 |
| 2 | A ×1 + B ×1 | A ×2 |
| 4 | A ×2 + B ×2 | A ×4 |
| 8 | A ×4 + B ×4 | A ×8 |

The split is **internal** — `count` means "alternatives I want", not "top-N
models". A future Advisor could do `8 → A×3 + B×3 + C×2` without changing the
`count` contract. User override (`--models "id[,id[,id]]"`) forces the set and
splits `count` evenly across it; the Advisor never silently replaces a named
model. Naming more models than `count` drops the surplus and reports
`dropped_models`.

### Worked allocations (count = 4, current catalog — deterministic)

| intent | allocation | ~total |
|---|---|---:|
| illustration / concept art | Nano Banana ×2 + Nano Banana 2 ×2 | $0.018 |
| launch / announcement image | Nano Banana Pro ×2 + Nano Banana 2 ×2 | $0.039 |
| social post | Nano Banana 2 ×4 *(A only — nothing else within 1 pt)* | $0.024 |
| poster / flyer | Nano Banana Pro ×2 + GPT Image 2 ×2 | $0.111 |
| infographic / diagram | GPT Image 2 ×4 *(A only — leads by > 1 pt)* | $0.168 |
| product / e-commerce shot | Seedream 5.0 Lite ×2 + Nano Banana 2 ×2 | $0.086 |

`test-fixtures/sample-prompts.json` pins the expected allocation for four of
these against the current reference files — run `resolve` on each and diff.

### When a model is unavailable — "plan changed", not "fallback"

Availability is **never probed**. If the *first* task's model is rejected before
any spend (`503 "supported model not found"`), `image.py` re-scores without it
and exits `3` with a suggested replacement; the agent re-runs
`resolve --models "<suggested>"` and re-approves — a second approval is warranted
because the plan changed. A *later* model failing to submit marks those branches
`FAILED` (not charged) and the rest proceed.

### Picking a `size`

`preferred_sizes` are literal `WxH` strings — the router's actual parameter.
The Advisor takes the first entry that clears the chosen model's `size_min_px`;
if none do, it upsizes to the smallest valid dimensions at that aspect ratio.
**Size feeds pricing**: each candidate is costed at the size it would actually
run — so Seedream's forced upsize to ≥ 3.69 MP, and `gpt-image-2`'s
size-dependent rate (~$0.006 at 1 MP → ~$0.042 at 1.5 MP), are compared on real
cost.

---

## 5. Reference files

Two files, deliberately separate.

### `references/generation-policy.md` — durable

Intent → per-dimension requirement weights + preferred sizes. **No model
names.** A team edits this to change Image Lab's taste; it does not rot when the
model catalog moves. Dimensions (v0.1): `photorealism`, `typography`,
`composition_control`, `speed` — a fixed vocabulary shared with the catalog's
`scores` block.

### `references/router-model-catalog.yaml` — temporary adapter

Exists **only because the router API has no model-list or pricing endpoint**.
It is not part of Image Lab's design — it is a hand-maintained shim, isolated
behind `load_model_catalog()`. Its header carries a `TODO` to delete it when the
router ships `GET /api/v1/models?modality=image`. Per model: `label`, `url` (the
CogFoundry page), `scores` (−1…2 per dimension), `usd_per_image` (+ optional
size-tiered `pricing`), `size_min_px`.

Rates were measured 2026-09-06 by submitting one real task per (model, size) and
confirming the charge against `loomloom balance` — the router's `data.cost`
matched the delta every time. They are a **pre-flight guess** of that
authoritative number; a wrong rate only skews the estimate, and RESULTS shows
the real charge.

---

## 6. The exploration page

`run` writes `<out>/run.json` — the record: `prompt`, `intent`, `actual_usd`,
`estimated_usd`, `out_dir`, and `alternatives[]` (`index`/`of`, `label`,
`model` + `model_label` + `model_url`, `requested_size`, `actual_size` from the
PNG header, `cost_usd`, `seconds` from the router's `finish_time − start_time`
epochs, `status`, `file` — a **basename**, no absolute paths, `note`), plus
derived `by_model` / `images` / `failed` / `incomplete` views.

On request, `build-exploration-page.py --from <out> --title … --subject …
--invocation … [--selected <label>]` turns that into a **self-contained static
folder** (`<out>/<slug>/index.html` + `assets/alternative-NN.png` +
`exploration.json`). **Nothing is spent** — a render step, not a gate. It
follows redesign-lab's build → render seam: `exploration.json` is the data model
a future PDF / social-card renderer reads; the HTML ships **real separate asset
files** (a multi-MB base64 blob breaks browser rendering).

The page is an **image-selection gallery, not a case-study report**, in
redesign-lab's house style (serif body, uppercase-Arial headings, IBM Plex Mono
labels, hard edges, the loomloom green accent, 3-state dark mode, fonts from
Google Fonts with real fallbacks):

- **Header → Gallery → Prompt → footer.** The gallery is one component: a framed
  hero + a thumbnail strip + a live meta line. Click a thumbnail → the hero and
  a `Alternative N · model (link) · size (asked …) · cost · time` line swap
  (~50 lines of inline vanilla JS; with JS off every thumbnail is a real `<img>`
  linking to its full file).
- Initial hero = the `--selected` image, else alternative 01. The creator's pick
  carries a permanent ✓ badge and a *"the creator's pick"* prefix on the meta
  only while it is the hero — never "Best", never a score.
- Failed branches are omitted from the gallery with one line beneath
  (*"2 of 4 generated — alternatives C, D did not (…)"*).
- Tagline composed from the run's real numbers: *"One brief. N models. M ways to
  see {subject}."* Stats line: *"M candidates · N models · $Y total"* (the actual
  charge; the estimate lives in the conversation RESULTS, not on the page).
- The **Prompt** section shows the exact `run.json` prompt in full, in italic
  serif — the reusable artifact — with a *"→ use Image Lab"* trigger line below.
  The Copy button puts the full invocation on the clipboard.
- Footer: *"Generated with CogFoundry's model router · cogfoundry.ai"* — never
  the token.

Robustness (from a pre-PR code review): a moved `<out>/` still renders (basename
fallback); assets are re-verified before the page states a count; `--selected`
on a failed branch → no Selected section, never a broken `<img>`.

---

## 7. Verified router-API facts (2026-09-06)

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
  references/generation-policy.md        # DURABLE
  references/router-model-catalog.yaml   # TEMPORARY ADAPTER
  scripts/image.py                       # resolve | run
  scripts/build-exploration-page.py      # run.json → shareable folder (no spend)
  test-fixtures/sample-prompts.json
  showcase/                              # v0.4 stub — Image Lab AS a loomloom workflow
```

No dependencies. Generated pages (`out/`) are **gitignored** — no case studies
are committed.

| Version | Adds | Layer |
|---|---|---|
| **v0.1** | 1/2/4/8 alternatives across best-fit models → quote → one approval → gallery → shareable page; 7 models | router API |
| v0.2 | retry failed branches; natural-language allocation tuning | router API |
| v0.3 | an LLM judge that ranks the alternatives (first step dependency) | router + local LLM |
| **v0.4** | tidy → generate ×N → judge as one loomloom TemplateSpec, with a run record | **loomloom** |
| v0.5–v0.6 | reference image + edit chain; batch a file of prompts | loomloom |

### `showcase/` (v0.4)

Where Image Lab *is* a loomloom workflow. The pitch: *the router can fan out N
calls; it cannot run tidy → generate → judge as one dependency-aware metered job
with a single run record.* `variants-4.spec.json` (already
`loomloom template-spec check` → valid) is the seed — four `image-generate`
branches; the `stp_tidy` / `stp_judge` steps and the wiring are the v0.4 build.

---

## Appendix — router API reference

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
- CogFoundry router API — not publicly documented; behaviour here is from direct
  verification on 2026-09-06.
