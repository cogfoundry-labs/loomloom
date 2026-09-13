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
 ^                                                 |
 └──────────── adjust the prompt, another round ───┘
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
  RESULTS is conversation, not a gate — but looping back to adjust the prompt
  and try another round is a brand-new spend, so it re-enters the gate in full,
  never shortcuts it.

### What v0.1 is — and is not

| | v0.1 | else |
|---|:---:|---|
| Any prompt → 1 / 2 / 4 / 8 parallel alternatives | ✅ | |
| Multi-model allocation — A + B at count 2 / 4, A + B + C at count 8 (A alone only when it's the sole survivor) | ✅ | |
| 9 image models; deterministic scorer picks; user overrides by name | ✅ | |
| Per-model valid `size`; estimated total before spend; actual cost after | ✅ | |
| Live per-branch progress; failed branch reported (not charged) | ✅ | |
| Shareable exploration page (a static folder, no spend) | ✅ | |
| Adjust the prompt after RESULTS and generate another round (own session folder, own gate); a session comparison page across every round | ✅ | |
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
art/portraits-heavy ones where B is Seedream 5.0 Pro, the one expensive model
in the catalog at ~$0.10/image (`blog hero` / `illustration` at count 8
≈ $0.34; see §4's worked allocations).

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
| image models available today | 1 (`gemini-2.5-flash-image`) | **9** |
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
| **GENERATE** | `image.py run --alloc … --progress-file … --confirm <fingerprint>` — submit the whole allocation as N gateway tasks, poll each (~3 s), download each image, verify it is a PNG. `--confirm` must be the exact `fingerprint` (a sha256 of `alloc_arg`) that `resolve` printed for this allocation — not a bare flag — so `run` refuses if the `--alloc` it receives isn't the one that fingerprint was issued for (2026-09-13; closes a gap where nothing previously tied approval to a specific plan). **Runs in the background**: it rewrites `progress.json` as each branch lands, and the agent posts a short "N/total done" update every ~30–60 s so the user is never left staring at nothing |
| **RESULTS** | agent sends each image as a labelled card, presents `label · model · time · cost · size` + actual-vs-estimate, **then builds the exploration page** and asks the user what's next: pick a favourite, adjust the prompt and generate another round, or stop |

"QUOTE" is a user-facing label — internally it is an *estimated total*
(`Σ price(model, size) × n`); the gateway has no quote endpoint. Balance is a
preflight check, not an authorization: `resolve` reports `sufficient: false` and
the agent stops before APPROVE; otherwise the gate stays uncluttered.

### The gate, as the user sees it

```
PLAN   4 alternative(s) for 'poster / flyer'
       GPT Image 2.5 Sunburst   x2   default        $0.0069/img x 2 = ~$0.0138
       Nano Banana 2            x2   2K · 2:3        $0.0060/img x 2 = ~$0.0120
       GPT Image 2.5 Sunburst is the best fit for 'poster / flyer'; weak-at-text_rendering
       models are ruled out; Nano Banana 2 is the runner-up
       output: ./out/round-1  (created by `run`, not by this preview)
QUOTE  Estimated total: ~$0.0258   (actual shown after the run)
APPROVE  [ Generate 4 — Sunburst ×2 + Nano Banana 2 ×2, ~$0.03 ]  [ Adjust ]  [ Stop ]
         ... 4 tasks running across 2 models ...
RESULTS  GPT Image 2.5 Sunburst ($0.0138) [A][B]   Nano Banana 2 ($0.0120) [C][D]   Actual: $0.02xx
         [ Pick a favourite ]  [ Adjust the prompt & regenerate ]  [ Stop ]
```

A failed branch shows as `FAILED` with its `fail_reason`, is not charged, and is
**not** retried in v0.1. Partial success is more visible at `count = 8`;
RESULTS names every failed / incomplete branch and the user picks from what
landed.

### Adjusting the prompt after RESULTS — another round, same session

RESULTS is not a dead end. If the user wants to try a tweaked prompt, the
workflow loops back to PLAN rather than treating that as a disconnected new
run: `--out` names a **session**, not one run's folder, so `run` writes each
attempt to `<out>/round-N/` — N auto-detected from `<out>/session.json`, a
small ledger `run` appends to after every completed round (`round`, `prompt`,
`intent`, `out_dir`, `actual_usd`, `estimated_usd`, `created_at`). Round 1
lands in `<out>/round-1/` too — there is no special-cased "first run" that
lives directly in `<out>` and could collide with a later round.

This costs nothing extra to guarantee correctness: **the existing `--confirm`
fingerprint already forces a fresh `resolve` call before every `run`**, since
it binds the model/size allocation, not the prompt text — there is no code
path that lets an edited prompt reuse an old approval. Adjusting is a full
lap of PLAN → QUOTE → APPROVE → GENERATE → RESULTS again, exactly like the
first round, just against the same session folder.

From round 2 onward, both the QUOTE and the RESULTS printout carry a soft
reminder — *"this is round 3 of this session — $0.34 spent across it so
far"* — visibility into cumulative iteration cost, not a hard cap; past
`SOFT_ROUND_LIMIT` (5, in `image.py`) the reminder adds a nudge to consider a
fresh PLAN instead, but never blocks another round.

### One case-study page per session, at a fixed address, that grows with it

`build-exploration-page.py --session ./out` builds a **single page per
session**, at `<out>/<slug>/index.html` — `<slug>` is derived from the
*first* round's `--title` and then frozen: rebuilding the page for round 2,
3, … never recalculates it, even though the round's own on-page title can
change every time. The address a case study lives at never moves just
because the brief got iterated on.

The page defaults to showing the **latest** round — same rich layout every
single-round page has always had (hero, thumbnail gallery, lightbox, run
facts, prompt, credits) — but every earlier round that was ever built into
this page is embedded alongside it. Once a second round exists, a nav strip
appears directly above the Gallery section — `← VIEW ROUND N` · `ROUND N / M`
· `VIEW ROUND N →` — naming the specific adjacent round each side goes to and
wrapping at both ends (round 1's "prev" is the last round, and vice versa)
rather than disabling a button. It sits at the point of use (right where the
content it controls begins), not in the topbar with the Share buttons — an
earlier version put it there as a small icon-only button and a user correctly
called it out as easy to miss and not obviously interactive; naming the
destination round explicitly on both sides reads as navigation on first
glance, not a utility action. Clicking either side swaps the round-specific
DOM (`#round-head`/`#round-body`) for that round's pre-rendered HTML and
re-runs the same gallery/lightbox JS every single-round page already relies
on — that JS itself is untouched, just wrapped in
`initGallery()`/`initPromptCopy()` (plus a small `initRoundNav()` for the
strip's own prev/next buttons, which carry their target round's index
directly via `data-round-index` so the click handler needs no wraparound
arithmetic of its own) so it can be re-run per switch instead of running once
at load. This replaced two earlier, separate mechanisms (a per-round page
nested under `<out>/round-N/<slug>/`, and an independent, visually simpler
`--compare` page) — one page, one address, growing with the session, is
strictly better once you can switch rounds in place.

A second small manifest, `<out>/case-study.json`, is this page-builder's own
record (separate from `image.py`'s `session.json`, which knows nothing about
titles): it remembers the frozen slug and, per round, whatever `--title`/
`--subject`/`--summary`/`--invocation`/`--selected` were passed for it — so
rebuilding the page after round 3 can still render round 1 exactly as it
looked when it was first published, not with round 3's title bleeding in.

---

## 4. The Model Advisor

Runs inside PLAN, deliberately split so the choice is **reproducible and
auditable** — not "an AI recommender":

```
prompt text
   │  agent: classify → one intent label (10 options; fuzzy; user can correct)
   │         + map the user's words to count (1 / 2 / 4 / 8, default 4)
   ▼
image.py (deterministic):
   generation-policy.md   → the intent's per-dimension requirement weights
   arena-scores.yaml      → real Arena.ai Elo + margin per model per category
                            (compute_arena_scores() turns this into each
                            model's per-dimension score, at load time)
   model-catalog.yaml     → each model's rate + how to actually call it
   1. disqualify a model weak (−1) at any HIGH (weight 2) requirement
   2. suitability(model) = Σ weight_d · score_d          — the single ranking number
   3. A = highest suitability   (exact ties → lower cost)
   4. B = the next-best surviving model, preferring a different provider than A
      when a credible one exists ("also worth trying")
   5. C = the third-best surviving model — used only to widen count 8, same
      provider-diversity preference as B
   6. allocate `count`: 1 → A · 2/4 → A + B · 8 → A + B + C
   ▼
plan: an allocation [{model, size, n, subtotal}] + estimated total + a "why" line
```

The agent's only jobs are **intent classification and the count** — both
low-stakes. Everything else is a deterministic function over real leaderboard
data, so *"why these two models?"* has a printable answer
(`image.py resolve --explain`).

### Suitability

- Requirement weight: `low`→0, `medium`→1, `high`→2.
- Model per-dimension score: `−1`…`2`, computed by `compute_arena_scores()`
  from real Arena.ai Elo ratings — not hand-assigned. Models are grouped into
  confidence-interval-aware tiers (two models land in the same tier only when
  their Elo gap doesn't exceed the quadrature combination of their published
  95%-CI margins, `√(marginA² + marginB²)` — see
  `docs/plans/2026-09-13-image-lab-arena-scoring-implementation.md`), then
  the best tier maps to `2` and the worst to `−1`, evenly interpolated in
  between. A category that resolves into more tiers is finer-grained, not
  more influential — every category is still bounded to the same `[−1, 2]`
  range.
- `suitability = Σ_d weight_d · score_d` — the weighted dot product. **Cost is
  not in it.**
- A `high` requirement + a `−1` model (the model's worst confirmed tier on
  that dimension) = disqualified.
- **A** = highest suitability; cost breaks only *exact* ties, so a free/cheap
  model never wins on price alone.
- **B** = the **next-best surviving model** — presented honestly as the runner-up
  ("also worth trying"), not claimed to be A's equal. There is no suitability
  floor on B: `count ≥ 2` means *at least two models* in v0.1. The catalog
  currently holds 3 OpenAI GPT Image skus that cluster at the top of most
  categories; left unchecked, B would almost always be "another GPT variant."
  `_secondary()` instead prefers the best surviving model from a **different
  provider** than A, but only when it's within 1 confirmed tier (on the
  intent's highest-weighted dimension) of the best candidate overall — a
  diverse pick several tiers behind isn't credible, and falls back to the raw
  best regardless of provider. "Diversity among credible alternatives, not
  diversity at any cost."
- **C** = the **third-best surviving model**, brought in only at `count = 8`
  ("deep exploration") — a wider net when the user has asked for the most
  chances. `count = 4` deliberately stays on two models; C is the one place
  where a bigger `count` also means a broader model spread. Same
  diversity-with-guardrail preference as B, against every provider already
  taken by A and B.
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
| launch / announcement image | GPT Image 2.5 Sunburst ×2 + GPT Image 2.5 Flare ×2 | $0.028 |
| profile / avatar | GPT Image 2 ×2 + Seedream 5.0 Pro ×2 | $0.215 |
| social post | GPT Image 2.5 Sunburst ×2 + Nano Banana 2 ×2 | $0.026 |
| blog hero / article cover | GPT Image 2.5 Sunburst ×2 + Seedream 5.0 Pro ×2 | $0.217 |
| poster / flyer | GPT Image 2.5 Sunburst ×2 + Nano Banana 2 ×2 | $0.026 |
| infographic / diagram | GPT Image 2.5 Sunburst ×2 + Nano Banana 2 ×2 | $0.026 |
| illustration / concept art | GPT Image 2.5 Sunburst ×2 + Seedream 5.0 Pro ×2 | $0.217 |
| 3D render / isometric illustration | GPT Image 2.5 Sunburst ×2 + GPT Image 2.5 Flare ×2 | $0.028 |
| product / e-commerce shot | GPT Image 2.5 Sunburst ×2 + Nano Banana 2 ×2 | $0.026 |
| generic | GPT Image 2.5 Sunburst ×2 + GPT Image 2.5 Flare ×2 | $0.028 |

GPT Image 2.5 Sunburst tops the Arena leaderboards broadly enough that it's A
for 9 of these 10 intents — the honest result of grounding scores in real
blind-preference data, not a sign the scorer is broken. B is a different
provider (Nano Banana 2 or Seedream 5.0 Pro) whenever one is a credible
alternative (within 1 confirmed tier), and otherwise another GPT variant —
e.g. `profile / avatar` is the one intent where a non-OpenAI model (GPT
Image 2, on Portraits) leads at all, with Seedream 5.0 Pro a credible B.
`3D render / isometric illustration` is the one intent where the *runner-up*
is also a GPT variant despite an active diversity check: on 3D Modeling, the
best non-OpenAI candidates (Nano Banana 2 / Seedream 5.0 Pro) sit 2 confirmed
tiers behind Sunburst and Flare — past `DOMINANT_TIER_SLACK`, so the guardrail
falls back to Flare rather than force an uncompetitive diverse pick.
Every intent lands on two models at count 4 with the current 9-model catalog
— none has a sole survivor. At **count 8** each of these widens to three,
e.g. `poster / flyer` → GPT Image 2.5 Sunburst ×3 + Nano Banana 2 ×3 + GPT
Image 2.5 Flare ×2 (~$0.05), `profile / avatar` → GPT Image 2 ×3 + Seedream
5.0 Pro ×3 + GPT Image 2.5 Sunburst ×2 (~$0.34), `3D render / isometric
illustration` → GPT Image 2.5 Sunburst ×3 + GPT Image 2.5 Flare ×3 + Seedream
5.0 Pro ×2 (~$0.24 — Seedream's Art-category strength as a secondary weight
now makes it the credible count-8 widen, even though it didn't clear the
guardrail for B at count 4). `image.py resolve --explain` shows the full
score table and why A / B / C were picked.

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

`preferred_sizes` (in `generation-policy.md`) are literal `WxH` strings — what
the *intent* wants, independent of any model. They are **not** necessarily the
literal value sent to the gateway: models differ in how they actually take a
size, checked live against each model's schema (2026-09-13; see
`docs/plans/2026-09-13-image-lab-lessons-from-model-image-arena.md`), not
assumed uniform:

| shape | example | what's sent |
|---|---|---|
| `wxh` (enumerated) | Seedream family | the literal `WxH` string closest to the preferred aspect ratio, drawn from that model's own `size_values` list |
| `wxh` (continuous) | `gpt-image-2` | a computed `WxH` that clears `size_min_px`, same floor-upsize algorithm as before — this is the one model shape where that legacy computation still applies |
| `tier` | Nano Banana Pro, Nano Banana 2 | a tier token (`"1K"`/`"2K"`/`"4K"`), **not** `WxH` |
| `aspect_ratio` | Nano Banana | no `size` at all — only an `aspect_ratio` field |
| `none` | GPT Image 2.5 Sunburst/Flare | no size-shaping parameter exists |

Each catalog entry's `request:` block (`model-catalog.yaml`) declares which
shape it is; `pick_size()` dispatches on that instead of assuming one shape
fits every model — a wrong assumption here previously computed a `WxH` outside
a model's real accepted range for more than one catalog entry (e.g. Seedream
5.0 Pro's old `size_min_px: 3686400` floor, when its real sizes top out around
1 MP), which is a **bug**, not a documented one-time upsize.

`aspect_ratio` is a separate, independent field from `size` — a model can (and
some do) take both at once, so it's computed and sent whenever a model's
`request.aspect_ratio_param` is true, regardless of its `size_param` shape.

**Size still feeds pricing**: each candidate is costed at the size it would
actually run (`estimate_px()` converts whatever shape/token was chosen back to
an approximate pixel count for the cost-tier lookup) — so `gpt-image-2`'s
size-dependent rate (~$0.006 at 1 MP → ~$0.042 at 1.5 MP) is still compared on
real cost.

---

## 5. Reference files

Three data files, deliberately separate, plus one operational reference.

### `references/generation-policy.md` — durable

Intent → per-dimension requirement weights + preferred sizes. **No model
names.** A team edits this to change Image Lab's taste; it does not rot when the
model catalog moves. Dimensions: `overall`, `commercial_design`,
`three_d_modeling`, `cartoon`, `photorealistic`, `art`, `portraits`,
`text_rendering` — exactly the 8 category leaderboards at
[arena.ai/leaderboard/text-to-image](https://arena.ai/leaderboard/text-to-image),
not a hand-invented axis (see `references/arena-scores.yaml` below). Each
intent's mapping onto these categories is annotated with a `confidence` —
`high` where a category matches 1:1 (`profile / avatar` → `portraits`),
`medium` where it composes a couple of adjacent categories because no exact
one exists, `low` for `infographic / diagram`, which has no matching Arena
category at all.

### `references/arena-scores.yaml` — the only hand-entered quality data

Raw Arena.ai Text-to-Image Arena data: Elo rating + margin (a published
95%-confidence-interval half-width) per model per category, plus one
`snapshot_date` for the whole pull. This is the **only** place a model's
quality is ever hand-entered — everything downstream is computed.
`image.py`'s `compute_arena_scores()` groups models into
confidence-interval-aware tiers per category (same tier only when the Elo gap
doesn't exceed the quadrature combination of both models' margins — see
`docs/plans/2026-09-13-image-lab-arena-scoring-implementation.md` for why
that, and not a linear sum, is correct) and rescales tier 0 → `2`, the worst
tier → `−1`, evenly interpolated between. Re-measuring a model's quality means
re-pulling this file from arena.ai, never hand-editing a score.

### `references/model-catalog.yaml` — temporary adapter

Exists **only because the gateway API has no model-list or pricing endpoint**.
It is not part of Image Lab's design — it is a hand-maintained shim, isolated
behind `load_model_catalog()`. Its header carries a `TODO` to delete it when the
gateway ships `GET /api/v1/models?modality=image`. Per model: `label`, `url` (the
CogFoundry page), `usd_per_image` (+ optional size-tiered `pricing`),
`size_min_px`, `verified_on` (for the `request:` shape only — unrelated to
`arena-scores.yaml`'s own `snapshot_date`), and a `request:` block declaring
how this specific model actually takes a size (`size_param`: `wxh` / `tier` /
`aspect_ratio` / `none`, plus `size_values`, `size_default`,
`watermark_param`, `aspect_ratio_param`) — see "Picking a `size`" above. It no
longer carries quality scores at all — see `arena-scores.yaml` above.

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

RESULTS **always** runs `build-exploration-page.py --session <out> --title …
--subject … --invocation … [--canonical-url …] [--selected <label>]` — it is
never a "do you want a page?" offer, because it costs nothing and the user picks
their favourite *from* the page. It builds (or rebuilds) the session's one
**self-contained static folder** (`<out>/<slug>/index.html` +
`assets/r<N>-alternative-NN.png` + `case-study-data.json`), following
redesign-lab's build → render seam: `case-study-data.json` is the data model a
future PDF / social-card renderer reads; the folder ships **real separate
asset files**. `--inline` additionally emits
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
  ($0.037–$0.10) were measured on 2026-09-06 at a forced ≥ 3.69 MP size —
  true for Seedream 5.0 Lite/4.5, but **not** Seedream 5.0 Pro, whose real
  sizes top out around 1 MP; that model's floor was wrongly copied from its
  siblings and fixed 2026-09-13 (see "Picking a `size`" above).
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
self-contained folder (`index.html` + `case-study-data.json` + `assets/` packed to
JPEG q95, full resolution, ~3–4 MB), exactly the way `redesign-lab/case-studies/`
works, so it can be hosted as a small standalone site. `case-studies/pack.py`
(a Pillow-based maintainer tool, not part of the runtime) does the packing. See
`case-studies/README.md`.

| Version | Adds | Layer |
|---|---|---|
| **v0.1** | 1/2/4/8 alternatives across best-fit models → quote → one approval → gallery → shareable page; 9 models | gateway API |
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
