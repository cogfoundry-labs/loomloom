# RFC-0003: Model catalog strategy

- **Status:** Draft
- **Author:** Bob
- **Created:** 2026-08-12
- **Discussion:** open for developer discussion

## Abstract

This RFC sets out how CogFoundry picks the AI models in [loomloom](../../README.md)'s
catalog — and, just as important, how we keep those picks trustworthy for the
people who build on them. It proposes a starting catalog for CogFoundry's
execution platform.

The catalog here is CogFoundry's, but the framework is meant to be
platform-agnostic. Any execution platform built on the same open loomloom
standard — see the [IR spec](../ir-spec/en/README.md) — can apply the same
method. The specific picks will differ from platform to platform; the way you
arrive at them shouldn't.

loomloom is not a chat app. Creators compile AI work into
[SkillBots](../../README.md#skillbot--a-deployable-modular-ai-system), publish
them, and sell them — and a published SkillBot pins the models it runs on. So
"best model" can't mean "whatever tops a leaderboard this week." It has to mean
the model that does the job well, at a price that keeps a SkillBot viable, from a
provider we can keep serving. Cost, stability, and a reliable backup count as
much as raw capability.

The rule we land on is deliberately simple. The next section is the whole thing
in three questions.

## How the catalog works, in three questions

Every model in the catalog earns its place by answering three plain questions:

1. **Is it good enough?** It has to clear a **quality floor** for that kind of
   work. Cost can never buy its way below acceptable output.
2. **Best quality, or best value?** For each kind of work we publish two picks: a
   **Quality Leader** (the best output, cost aside) and an **Efficiency Leader**
   (the closest quality at much lower cost — the default most steps should use).
3. **Is there a reliable alternative?** Every pick has a cross-provider
   **Redundancy** backup, so one model's outage or retirement never breaks a
   running SkillBot.

Everything after this — gates, scored criteria, benchmarks, thresholds,
lifecycle — is just the method that answers these three questions in a way anyone
can audit. Want the picks? Jump to the [proposed catalog](#proposed-catalog).
The rest is for whoever maintains it.

→ **See the method:** [Selection framework](#selection-framework) ·
[Evidence base & method](#evidence-base-and-designation-method) ·
[Lifecycle & operations](#catalog-lifecycle-and-operations)

## Motivation

In the [IR](../ir-spec/en/README.md), a step names its model through
`defaultModelRef.modelKey` — a `provider/model` catalog ID that each environment
resolves with `loomloom template-spec models <execution-unit>`. The catalog is
organized by [execution unit](../ir-spec/en/reference/execution-units.md):
`text-generate`, `image-generate`, `video-generate`.

Right now that catalog has grown ad hoc, and our examples all default to one
economy model. That's fine for early development; it won't do for a public
product. Three things about loomloom make model choice matter more here than
almost anywhere else:

1. **Batch economics, with a quality floor.** A template runs over N rows, so a
   3× price gap can decide whether a SkillBot is worth shipping — cost and
   throughput matter more than in an interactive product. But cheap is never
   allowed to mean bad: the default optimizes cost *above* an acceptable-quality
   bar, it doesn't just minimize price. A model that produces unacceptable output
   is disqualified, not defaulted to.
2. **A published SkillBot has to keep running.** It pins a model. That does *not*
   mean any one model lives forever — models get retired. It means the *catalog*
   guarantees continuity: a pinned snapshot keeps resolving through the
   deprecation window, retirements come with notice and an overlap period, and a
   same-band backup from another provider is always ready. Availability is a
   promise about the catalog, not about one model.
3. **Redundancy is what makes routing real.** Runtime failover only means
   something if every pick has a cross-provider alternative in the same quality
   band.

## Goals

- A repeatable, auditable way to decide what belongs in the catalog.
- A right-sized **default** (Efficiency Leader) and a clear **step-up path**
  (Quality Leader) for every kind of work.
- A cross-provider **Redundancy** backup behind every published pick.
- A lifecycle that creators can count on — explicit pinning, deprecation, and
  degradation rules.
- **Start lean, grow on purpose** — ship a small, clean first catalog, then widen
  it over time, each addition earning its place.

## Non-goals

- **Literal `modelKey` strings.** The exact ID is environment-specific and comes
  from the CLI. We *do* name each pick as **provider family + model name +
  version** (e.g. `Anthropic · Claude Sonnet · 4.6`); we just don't hard-code the
  resolved ID.
- **IR schema changes.** No new runtime fields — including a "work type" field
  (see [Mapping to execution units](#mapping-work-types-to-execution-units)).
- Runtime routing algorithms, and pricing/settlement mechanics.
- Provider onboarding engineering — including the full G2 terms checklist (see
  [Gates](#gates-passfail)).

## Design principles

- **Work before model.** Fill a `work type × modality` grid first; authors pick a
  job, not a brand.
- **Two picks and a backup.** A **Quality Leader** and an **Efficiency Leader**
  per work type, each with a cross-provider **Redundancy** pick.
- **Cheap default, easy step-up.** The Efficiency Leader is the default; the
  Quality Leader is one override away, per step, via `allowModelOverride` /
  `defaultModelRef`.
- **Quality has a floor.** Cost-cutting stops at an absolute acceptable-quality
  bar for the work (see [thresholds](#designation-thresholds)). Being close to a
  weak leader isn't good enough.
- **Show your work.** Every pick cites a credible, public benchmark. No citation,
  no designation.
- **Pin snapshots and honor the deprecation contract.** SkillBots depend on it.
- **Start small, then grow.** The first catalog is intentionally lean; it's meant
  to widen over time. Each addition earns its place, and a work type gets its own
  entry only when its leader actually differs (see the
  [collapse rule](#work-type-taxonomy)).

## Selection framework

### Preferred model designations

For each kind of AI work, loomloom keeps a *Preferred AI Models* entry with three
picks:

- **Quality Leader** — the highest demonstrated output quality for that work,
  **cost aside**. The step-up choice for hard or final-quality steps.
- **Efficiency Leader** — quality *close enough* to the Quality Leader at
  *much lower* cost: the best value, and the default most batch steps should use.
  It has to clear **both** the absolute quality floor and the relative closeness
  bar; both, plus "much lower cost," are pinned down per modality in
  [Designation thresholds](#designation-thresholds). *Today we measure this as
  benchmark quality vs. effective price. The better long-run metric —* **cost per
  successful result** *— is [future work](#future-work).*
- **Redundancy** — a real backup, so failover works. It has to be all three of:
  (a) the same quality band as the leaders, (b) support for the same execution
  unit, and (c) a **different provider**. "Some other provider exists somewhere"
  doesn't count — it must be a model we could actually switch to.

### Work-type taxonomy

We organize by work type, not just execution unit, because inside
`text-generate` a coding step and a creative-writing step have different leaders:

| Execution unit | Work types |
| --- | --- |
| `text-generate` | general reasoning · coding/agentic · creative & long-form writing · summarization/extraction · long-context · tool use/function calling · multilingual |
| `image-generate` | prompt-faithful/compositional · aesthetic/photoreal |
| `video-generate` | text-to-video · image-to-video |

**Collapse rule — this is what keeps the catalog lean.** A work type gets its own
entry **only when its Quality or Efficiency Leader differs** from the execution
unit's base work type (general reasoning for text; prompt-faithful for image;
text-to-video for video). Otherwise it just inherits the base entry. Seven text
work types do *not* automatically mean seven separate entries.

### Mapping work types to execution units

The IR picks models **per execution unit** (`defaultModelRef` per step) and has
**no work-type field** — and we're not adding one (see Non-goals). So the mapping
is:

- **The published default for an execution unit** is the **Efficiency Leader of
  that unit's base work type** — what a step gets when the author sets nothing.
- **Finer work-type picks are guidance**, delivered through CLI hints, docs, and
  template scaffolds. Authors apply them by setting `defaultModelRef.modelKey`
  (and `allowModelOverride` when they want to let the end user choose). These are
  recommendations, not runtime routing.
- **If** a work-type signal is ever added to the IR, this mapping becomes
  automatic. Tracked as future work, not assumed.

### Gates (pass/fail)

Fail any gate and the model is out, full stop.

- **G1 — Availability & compliance** on CogFoundry: we can license/host it for the
  target regions on acceptable data-handling terms.
- **G2 — Commercial terms** allow resale, monetization, and programmatic batch use
  — SkillBots are commercial products. *We'll need a per-provider terms checklist
  here* (training-data use, output ownership, resale rights, batch allowance),
  since terms vary a lot for image/video. Building it is provider-onboarding work,
  out of scope for this RFC.
- **G3 — Execution-unit support**: the model actually supports the unit. The
  catalog checks this dynamically.

### Scored criteria (weighted, highest first)

- **C1 — Cost & throughput** — **effective** price/Mtok: list price **plus
  cached-input / prompt-cache pricing and batch discounts**. A batch template
  reuses the same instruction/context on every row, so caching can dominate real
  cost. Also rate limits, concurrency, latency.
- **C2 — Capability fit** for the work (fit, not raw benchmark rank).
- **C3 — Lifecycle stability** — pinnable snapshots, a real deprecation notice
  period, a track record.
- **C4 — Redundancy** — a genuine second-provider backup exists.
- **C5 — Portability** — running across runtimes keeps compiled IR interoperable.

**How this drives the picks:** **C2** sets the **Quality Leader**; **C1 against
C2** sets the **Efficiency Leader**; C3–C5 gate both. We weight **C5 higher for
Redundancy and open picks than for the Quality Leader** — we won't let
portability veto a single-runtime frontier model, or we'd throw away the whole
point of a quality-first pick.

## Evidence base and designation method

Every pick has to cite a credible, public benchmark. Two independent,
continuously-updated sources cover most of it; task-specific benchmarks back the
objective Quality Leader calls.

### Primary sources (use directly)

1. **Artificial Analysis** (`artificialanalysis.ai`) — our go-to for the
   **Efficiency Leader**, because it plots a composite quality score against
   **price and speed on the same axes** and normalizes cost across modalities:
   blended $/Mtok for text, cost per 1,000 images at 1024×1024, and cost per
   minute of 1080p video. Independently measured on live API data; covers text
   (Intelligence Index), image, and video.
2. **LMArena / Arena** (`lmarena.ai`) — human-preference Elo with **category and
   occupational leaderboards** (coding/WebDev, creative writing, math, vision,
   long query, instruction following; occupational: software, writing, legal,
   medical, finance, science…) plus **text-to-image and text-to-video arenas**.
   Our best signal for the **Quality Leader** on subjective/generative work.

### Capability benchmarks by work type

| Work type | Recommended benchmark(s) | Why |
| --- | --- | --- |
| General reasoning / knowledge | **GPQA-Diamond**, **MMLU-Pro**, **Humanity's Last Exam** | Google-proof, graduate-level; MMLU/HumanEval are saturated |
| Coding / agentic | **SWE-bench Verified**, **LiveCodeBench**, **Aider polyglot** | Real engineering; LiveCodeBench is contamination-resistant |
| Tool use / agents | **τ-bench (tau-bench)**, **BFCL**, **GAIA**, **Terminal-Bench** | Function-calling and multi-step reliability |
| Math | **AIME**, **MATH** | Verifiable reasoning |
| Long context | **RULER**, **MRCR**, **Fiction.liveBench**, **LongBench** | Real retrieval over long inputs, not needle-in-haystack toys |
| Instruction following | **IFEval** | Constraint adherence |
| Factuality / hallucination | **SimpleQA** | Closed-book accuracy |
| Multimodal understanding | **MMMU** | Vision + reasoning |
| Multilingual | **MMMLU**, **MGSM** | Non-English coverage |
| Image generation | **GenEval**, **DPG-bench** (compositional), **HEIM** (holistic) | Objective prompt-faithfulness alongside human preference |
| Video generation | **VBench / VBench++** (16 automated dimensions, T2V + I2V) | Standard automated video-quality suite |

### Designation procedure

1. **Pick the benchmark set** for the work type from the table above. Favor
   **contamination-resistant** (LiveCodeBench) and **independent or
   human-preference** metrics over vendor-reported numbers.
2. **Quality Leader** = the highest-ranked gate-passing (G1–G3) model on that
   set, cost ignored.
3. **Efficiency Leader** = on the Artificial Analysis quality-vs-cost frontier,
   the gate-passing model that clears the modality's **closeness** bar at its
   **cost** advantage (see thresholds). If nothing qualifies, there's **no
   separate Efficiency Leader** — the Quality Leader is also the default.
4. **Cite it.** Record source URL, metric, score, and retrieval date. No
   citation, no designation.
5. **Refresh** monthly (or on a major release) under the
   [change-control rule](#change-control--hysteresis).

**Measure closeness in this order** — use the first one available, don't mix:

1. **Same-source score ratio** (e.g. both from the AA Intelligence Index) — most
   reliable.
2. **Cross-source benchmark** (leader and candidate on comparable but different
   benchmarks) — fine when same-source coverage is missing.
3. **Arena Elo gap** — last resort, only where benchmark coverage is absent
   (usually subjective/generative work).

### Designation thresholds

> Treat these as **starting heuristics to calibrate after the first full catalog
> pass**, not fixed constants. They differ by modality because the cost and
> quality curves do — video has steeper cost curves and fewer options, so its
> bars are looser.

| Modality | Absolute quality floor | Closeness (quality) | Cost advantage |
| --- | --- | --- | --- |
| Text | ≥ the work type's defined minimum benchmark score | ≥ ~90% of leader's score / within ~15 Elo | ≥ 3× cheaper blended $/Mtok |
| Image | ≥ the work type's defined minimum benchmark score | ≥ ~95% of leader's score / within ~15 Elo | ≥ 3× cheaper $/1k images |
| Video | ≥ the work type's defined minimum benchmark score | ≥ ~90% of leader's score / within ~20 Elo | ≥ 2× cheaper $/video-minute |

**Floor and closeness are both required.** Closeness alone isn't enough — a
default also has to clear an absolute floor for its work type, so cost can't drag
output below acceptable even when the whole field is weak. We set each
work type's floor during the first catalog pass, anchored conservatively: no
default below the current mid-tier of a credible leaderboard for that work.

**Compare on _effective_ price, not sticker price.** Use the discounts the
workload actually gets — batch-API pricing, and **cached-input / prompt-cache
pricing** for the shared instruction/context every row reuses (see
[C1](#scored-criteria-weighted-highest-first)). Blended $/Mtok assumes a fixed
input:output ratio (e.g. AA's 3:1) — state the ratio so anyone can reproduce the
number.

Text sits at ~90% (not 95%) because a ~10% Index gap is usually imperceptible on
routine batch steps but unlocks a big cost saving; image stays at ~95% because
compositional benchmarks and arena Elo separate models more finely. All three are
things the first catalog pass should confirm.

### Tie-break: objective vs. subjective work

When benchmark rank and human preference disagree, decide by work type:

- **Verifiable work** (coding, math, extraction, tool use) → trust the
  **capability benchmarks** (SWE-bench, τ-bench, GPQA…).
- **Subjective/generative work** (creative writing, image, video) → trust the
  **human-preference arenas** (LMArena, AA arenas).

### Credibility caveats

- Prefer **independently measured** (Artificial Analysis) or **human-voted**
  (LMArena) results over vendor-reported ones.
- Watch for **saturation** (MMLU, HumanEval) and **contamination** — favor
  post-cutoff or verified subsets.
- Arena Elo has a **style/verbosity bias**; pair it with an objective benchmark
  for verifiable work.
- Require **≥ 2 sources** to agree before naming a pick; log any disagreement.

## Proposed catalog

> **Naming:** each pick reads as **family · name · version** (e.g.
> `Anthropic · Claude Sonnet · 4.6`). Resolve the executable `modelKey` with
> `loomloom template-spec models <unit>` and pin that snapshot — the version here
> says *which* release to pin, not the literal ID (see Non-goals). A trailing `x`
> (e.g. `GPT-5.x`) means the exact minor version gets fixed during the catalog
> pass. Everything below is an **illustrative snapshot as of August 2026** and
> **must be re-verified before adoption** — these leaderboards move on live data.

### Worked example: general reasoning (text)

The procedure end to end, on the Artificial Analysis Intelligence Index (a
composite score; higher is better). *Illustrative snapshot ~2026-03; re-verify.*

1. **Benchmark set:** AA Intelligence Index + GPQA-Diamond.
2. **Candidates (Index / blended price per Mtok in-out):**
   `Google · Gemini Pro · 3.1` ≈ **57** at **$2 / $12**;
   `Anthropic · Claude Opus · 4.6` ≈ **53** at ≈ $15 / $75;
   `Anthropic · Claude Sonnet · 4.6` ≈ **51** at ≈ $3 / $15.
3. **Quality Leader = `Google · Gemini Pro · 3.1` (57)** — highest, cost ignored.
4. **Closeness bar (text, ~90%):** 0.90 × 57 ≈ **51.3**.
5. **Efficiency Leader search.** The other two scored models both price *above*
   the leader, so neither can be a cheaper default — they're step-up alternates
   only. A cheaper default has to come from a lower-cost tier (e.g.
   `Google · Gemini Flash · 3.1`): it becomes the Efficiency Leader **iff**, once
   pinned, it clears the ~51.3 closeness bar and the floor at ≥ 3× lower effective
   cost. If nothing does, the leader — already cheap at $2 / $12 — is its own
   default.
6. **Takeaway.** When the quality leader is also cost-effective, the framework
   correctly refuses to force a weaker "cheaper" default: the Efficiency Leader is
   `Google · Gemini Flash · 3.1` where it clears the bars, otherwise
   `Google · Gemini Pro · 3.1` itself.
7. **Citation:** AA Intelligence Index, retrieved 2026-03,
   `https://artificialanalysis.ai/`.

### `text-generate`

Format: **family · name · version**. All entries illustrative (Aug 2026),
re-verify before adoption.

| Work type | Quality Leader | Efficiency Leader (default) | Redundancy | Primary benchmark(s) |
| --- | --- | --- | --- | --- |
| General reasoning | Google · Gemini Pro · 3.1 / Anthropic · Claude Opus · 4.6 | Google · Gemini Flash · 3.1 (else = Quality Leader) | OpenAI · GPT · 5.x | AA Intelligence Index, GPQA-Diamond, MMLU-Pro, HLE |
| Coding / agentic | Anthropic · Claude Opus · 4.6 | Anthropic · Claude Sonnet · 4.6 / Google · Gemini Flash · 3.1 | OpenAI · GPT · 5.x | SWE-bench Verified, LiveCodeBench, τ-bench, LMArena WebDev |
| Creative & long-form writing | Anthropic · Claude Opus · 4.6 / Google · Gemini Pro · 3.1 | Google · Gemini Flash · 3.1 / Anthropic · Claude Haiku · 4.5 | OpenAI · GPT · 5.x | LMArena creative-writing & occupational "Writing" |
| Summarization / extraction | Anthropic · Claude Sonnet · 4.6 / Google · Gemini Pro · 3.1 | Google · Gemini Flash · 3.1 / Anthropic · Claude Haiku · 4.5 | OpenAI · GPT-5-mini · 5.x | IFEval, SimpleQA, task-specific eval |
| Long context | Google · Gemini Pro · 3.1 | Google · Gemini Flash · 3.1 | OpenAI · GPT · 5.x (long-context) | RULER, MRCR, Fiction.liveBench |
| Tool use / function calling | Anthropic · Claude Opus · 4.6 / OpenAI · GPT · 5.x | Anthropic · Claude Sonnet · 4.6 / Google · Gemini Flash · 3.1 | Google · Gemini Pro · 3.1 | BFCL, τ-bench |
| Multilingual | Google · Gemini Pro · 3.1 / OpenAI · GPT · 5.x | Google · Gemini Flash · 3.1 | Alibaba · Qwen · 3 (open) | MMMLU, MGSM, LMArena (language) |

### `image-generate`

| Work type | Quality Leader | Efficiency Leader (default) | Redundancy | Primary benchmark(s) |
| --- | --- | --- | --- | --- |
| Prompt-faithful / compositional | OpenAI · GPT Image · 2 / Google · Imagen · 4 | Google · Gemini Flash Image ("Nano-Banana") · 2 / Google · Imagen · 4 (Fast) | Black Forest Labs · FLUX · 2 (open) / Ideogram · v3 | GenEval, DPG-bench, HEIM |
| Aesthetic / photoreal | Black Forest Labs · FLUX Pro · 2 / Google · Imagen Ultra · 4 | Google · Gemini Flash Image ("Nano-Banana") · 2 / OpenAI · GPT Image Mini · 2 | Stability AI · Stable Diffusion · 3.5 (open) / Recraft · V3 | LMArena T2I, Artificial Analysis Image Arena |

### `video-generate`

| Work type | Quality Leader | Efficiency Leader (default) | Redundancy | Primary benchmark(s) |
| --- | --- | --- | --- | --- |
| Text-to-video | Google · Veo · 3.1 | Kuaishou · Kling Turbo · 3.0 / Google · Veo Fast · 3.1 | Runway · Gen · 4.5 / Alibaba · Wan · 2.6 (open) | VBench, AA Text-to-Video Arena, LMArena video |
| Image-to-video | Google · Veo · 3.1 (I2V) / ByteDance · Seedance · 2.5 | Kuaishou · Kling · 3.0 / MiniMax · Hailuo · 2.3 | Runway · Gen · 4.5 / Alibaba · Wan · 2.6 (open) | VBench++ (I2V), AA Image-to-Video Arena |

> **Lifecycle note.** We leave OpenAI's Sora 2 off on purpose: its API is set to
> shut down (Sep 24, 2026). It's a perfect example of the risk the
> [lifecycle contract](#catalog-lifecycle-and-operations) exists to prevent —
> never pin a SkillBot to a model that's on its way out.

## Catalog lifecycle and operations

### The creator contract

1. **Pin snapshots, never floating aliases**, in the catalog.
2. **Deprecation** follows the committed SLA in [Decisions](#decisions) — a
   notice period plus an overlap window where both the old and new snapshots
   resolve, so published SkillBots keep running.
3. **One published default per execution unit**, versioned (see the
   [mapping](#mapping-work-types-to-execution-units)).
4. **Adding a model needs a filled scorecard** (gates + criteria); **removing one
   follows the deprecation SLA**.

### Growth path

The first catalog is small on purpose. We expect it to **grow toward more
diversified capability** — more work types, specialist/open entries, new
modalities like embeddings — as demand shows up. Growth is additive and
governed: every new entry passes the scorecard and gets its own
Quality/Efficiency/Redundancy picks and citations. "Start lean" is about the
first release, not a permanent cap.

### Change control & hysteresis

Leaderboards move constantly. The published **default must not flap**, or the
creators who pinned it get churned for no reason.

- **Ownership:** a named **catalog maintainer (or rota)** owns the monthly refresh
  and the citation log. Where the source APIs allow, that refresh is an automated
  pull from AA/LMArena that the maintainer confirms.
- **Hysteresis:** don't swap a published default unless a challenger either
  (a) holds the lead for **two monthly reviews in a row**, or (b) beats the
  incumbent by **more than the closeness threshold margin** in a single review.
- **Quarterly** deep review of the whole catalog against the scorecard, on top of
  the monthly refresh.

### Failure modes & graceful degradation

| Trigger | Response |
| --- | --- |
| **Quality Leader price spike or sunset** | Fall back to the Redundancy pick; open a scorecard to re-designate. A sunset triggers the deprecation SLA immediately. |
| **Efficiency Leader drops below the closeness bar** | Promote the Redundancy pick or the Quality Leader to default; re-run the procedure at the next refresh (hysteresis waived for a hard failure). |
| **Whole provider unavailable** | Route affected steps to their Redundancy pick (a different provider by construction); if none is healthy, surface a clear error rather than silently swapping in a weaker model. |
| **Nothing clears the thresholds** | Publish the Quality Leader as the sole pick and flag the entry for threshold calibration. |

## Decisions

Promoted from open questions, now committed:

- **D1 — Deprecation SLA: 90 days.** At least 90 days' public notice, plus an
  overlap window where old and new snapshots both resolve. This is the trust
  anchor of the creator contract, and it's no longer provisional.
- **D2 — Default is global, override is per step.** The published default for an
  execution unit is global; authors override per step via `allowModelOverride` /
  `defaultModelRef`. Per-account defaults wait until there's demand.

## Future work

### Work Efficiency — from token price to cost-per-successful-result

Today we pick the **Efficiency Leader** from *benchmark quality + effective price
per token*. For batch AI work, the metric that actually matters isn't $/Mtok —
it's **cost per successful (accepted) result**, and, more broadly, **Work
Efficiency**:

> **Work Efficiency = useful work produced ÷ total execution cost**

where total cost includes not just model price but **retries, failed runs,
latency, fallback execution, and human intervention**.

Why raw price misleads:

| Model | Price / run | Success rate | Expected cost / success |
| --- | --- | --- | --- |
| A | $0.10 | 60% | $0.10 ÷ 0.60 = **$0.167** |
| B | $0.20 | 95% | $0.20 ÷ 0.95 = **$0.211** |

On sticker price A looks 2× cheaper; on cost-per-success A still wins, narrowly —
**until** A also needs a human to fix ~30% of its output, and then B is clearly
cheaper. Token price alone would have picked the wrong default.

This is squarely loomloom's philosophy: the question was never "which model is
cheapest?" but **"which model gets the job done most efficiently?"** The runtime
already meters runs, retries, quality-gate outcomes, and fallbacks, so it's in a
good position to measure realized per-model success rates and feed them back.

**Why it's future work, not now:** it needs per-model success/acceptance
**telemetry** that a fresh catalog doesn't have yet, plus a definition of
"accepted result" per work type (quality-gate pass, buyer acceptance, or human
sign-off). Once that data exists, **Work Efficiency should become the primary
basis for the Efficiency Leader** — and probably a core loomloom concept — with
benchmark-quality-vs-price as the cold-start proxy until then.

## Open questions

- Do we want an **embeddings** execution unit (for retrieval-heavy SkillBots),
  which would add a fourth modality?
- Do we expose an **open/self-hostable** entry now, or wait? It would be gated on
  **reproducibility, offline operability, and permissive licensing** on top of
  G1–G3 — deferred until there's buyer demand for reproducible/offline SkillBots.
- Are the **threshold numbers** right after the first catalog pass (text ~90% /
  image ~95% / video ~90%; cost 3×/3×/2×)?
- Is a **two-review** hysteresis window right, or should high-value pinned models
  get a longer one?

## Next steps

1. Run the **first catalog pass** against live AA/LMArena data — this turns the
   illustrative picks above into pinned, cited entries and sets each work type's
   quality floor.
2. Name the **catalog maintainer** and stand up the citation log.
3. Draft the **G2 terms checklist** as part of provider onboarding.

Comments on the framework, thresholds, and open questions are welcome before the
first catalog pass.

## References

### loomloom

- [README — SkillBot & runtime](../../README.md)
- [Execution Units reference](../ir-spec/en/reference/execution-units.md)
- [Configure models](../ir-spec/en/how-to/configure-models.md)

### Evaluation sources (primary)

- Artificial Analysis — https://artificialanalysis.ai/
- Artificial Analysis, Text-to-Image Arena — https://artificialanalysis.ai/image/leaderboard/text-to-image
- Artificial Analysis, Text-to-Video Arena — https://artificialanalysis.ai/video/leaderboard/text-to-video
- LMArena / Arena leaderboard — https://lmarena.ai/leaderboard

### Capability benchmarks

- SWE-bench (Verified) — https://www.swebench.com/
- LiveCodeBench — https://livecodebench.github.io/
- GPQA — https://github.com/idavidrein/gpqa
- MMLU-Pro — https://huggingface.co/datasets/TIGER-Lab/MMLU-Pro
- τ-bench — https://github.com/sierra-research/tau-bench
- BFCL (Berkeley Function-Calling Leaderboard) — https://gorilla.cs.berkeley.edu/leaderboard.html
- RULER (long context) — https://github.com/NVIDIA/RULER
- IFEval — https://github.com/google-research/google-research/tree/master/instruction_following_eval
- MMMU — https://mmmu-benchmark.github.io/
- GenEval — https://github.com/djghosh13/geneval
- HEIM (holistic image eval) — https://crfm.stanford.edu/heim/
- VBench / VBench++ — https://vchitect.github.io/VBench-project/
