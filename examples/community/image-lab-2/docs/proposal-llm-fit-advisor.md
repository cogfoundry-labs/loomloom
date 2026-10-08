# Proposal: an Assistant Fit advisor for Image Lab 2

Status: **proposal for review (revision 7), nothing built.** Date: 2026-10-08.
Related: `proposal-creative-direction-stage.md` (its first test doubles as evidence here), `design-v2.md` section 11 (the image-model
Advisor), the loomloom model-catalog RFC (`docs/rfc/0003-model-catalog-strategy.md`, work types for text models).
Revision 2 folds in an outside review (ChatGPT) and my own analysis of it; revision 3 records your decisions; revision 4 raises the cap and adds the default-model rule; revision 7 makes Fable gateway-only and adds a gated gateway judge; section 12 lists what changed.

**Naming.** In Image Lab "model" already means the *image* model (the workbook's Model column, `Models: 1` in preflight, the Advisor).
To avoid confusion this feature is called **Assistant fit** in user-facing text ("is your current assistant well suited for this
step?") and `llm-advice` in the code. The name is a decision for you (section 11).

## 0. Goal and thesis

> **Measure how well different assistants (the LLM your coding agent runs on) do the work Image Lab asks of them, tell the user whether
> their current one is good enough, and suggest an alternative only when the evidence is strong.**

- Image Lab controls the *image* model but not the assistant that does the creative, planning and review work. That asymmetry is real.
- It is not "find the best LLM". It is **task-specific and cost-aware**: which assistant is good enough for *this step*, at *this cost*.
- The thing measured is a **skill version running on a model**, not a model alone.
- A common, valid answer is **"your assistant is already well suited, keep it."**
- Assistant fit observes the workflow. It does not control it, and it is not an LLM router.

## 1. Where the assistant matters in the workflow

| Step | What the assistant does | How much quality matters |
|---|---|---|
| Route and capture | decides quick vs explore, asks one question | low |
| Creative Direction (proposed) | reads the brief, invents directions, writes rich wording | **high** (creative reasoning, brand knowledge) |
| Plan writing | turns the Direction Sheet into a valid `plan.json` under many rules | medium-high (rule following, long structured output) |
| Confirmation messages | explains matrix, cost and limits plainly | low-medium |
| **Result review** | looks at the generated images for text errors, anatomy, product, bleed | **high** (vision, small text) |
| Evaluation judging | scores plans or images against a rubric | high, best done by a different model than the one judged |

Evidence from this project: my review of adidas5 started from thumbnails and found one garbled headline only after zooming in. How
much a reviewer catches depends on the assistant, and the user cannot see that.

## 2. What we know today (read from the gateway, free)

`GET https://router.cogfoundry.ai/api/v1/models` returns **49 text models** (checked 2026-10-08). Per model: company, name, `api_name`,
context window, max output tokens, prompt-cache support, supported APIs, and `pricing` (`input_price`, `output_price`, `cached_price`,
`"currency": "USD"`; the unit is not stated, probably per million tokens, **unverified**).

- Every model supports `/v1/chat/completions`; many also `/v1/messages` and `/v1/responses`.
- Families: Anthropic, OpenAI, Google, DeepSeek, Alibaba, Moonshot, Z.ai, xAI, ByteDance, Tencent, MiniMax, Xiaomi, Meta.
- Prices differ about 100x (for example GPT-5.6 Luna 0.2 in / 1.2 out, Gemini 3 Flash 0.5 / 3, Claude Sonnet 5 2 / 10, Claude Opus 5 5 / 25,
  GPT-6 Astra 10 / 50).
- **Vision is listed for only 10 of 49** (`architecture.input` is empty for 34), so modality cannot be inferred from the listing.
- **The listing has no quality information.** It tells us **facts** (price, context, APIs, availability); only **evaluation** can tell
  us quality, reliability and task fit.

## 3. Constraints and rules

1. The skill cannot switch the host's model; each host has its own switch. The skill advises or (later, optionally) delegates.
2. The host model's name is **self-reported** by the agent and may be wrong or unmapped; the user can override. Unmapped is reported as
   unmapped, never guessed.
3. **Host capability is separate from model quality.** Many hosts cannot read image files at all. Then "I will look at every image"
   is impossible whichever LLM runs, and the review needs a second opinion or the user's own eyes. The report checks capability first
   (can read images, can run background commands, can ask structured questions).
4. **Two kinds of consent, never merged:**
   - **spend consent**: a quote (tokens and price basis, unverified costs shown as unverified) and an approval, as for images;
   - **data-transfer consent**: names the extra provider and what leaves the host ("this sends your brief to Google's Gemini").
5. **Person likeness.** Images generated from a person's photo contain that person's likeness and today "stay local". A vision review
   of them by a gateway model stays blocked unless the existing person consent has been given.
6. `Unknown` outcomes (no confirmed reply) are never retried automatically, as for images.
7. Advice never gates or nags: it shows once at the start and again only where a strong finding applies.
8. **No model is recommended merely because it is on the gateway**, and the advisor may name models outside it (in L1 the user switches
   their own assistant). Recommendations rest on task-specific measured evidence. This rule is written into the repository.

9. **A model is used in one of two ways, and each has its own gate.**
   - **As the coding agent's default model** (this session: Claude Sonnet 5.5): no gateway charge, and nothing needs confirming. It still uses your
     session's allowance.
   - **Through the CogFoundry API:** the quote names the model, the estimated tokens and price, with unverified costs shown as unverified, and **nothing
     runs until you confirm that spending**.
   An evaluation never runs on any *other* model that the coding agent makes available (for example a sub-agent on a different model) unless you
   select that model explicitly and confirm it for that run. Nothing is inferred from an earlier approval, and the agent never switches model on its
   own. A model that is not the default is evaluated only through the gateway (for example Claude Fable 5 and GPT-6 Astra here).

## 4. Three levels; build the first

| Level | What happens | Data leaves the host? | Spend | Build? |
|---|---|---|---|---|
| **L1 Advise** | a report compares the user's assistant with others per step, from evidence, and says keep, optional or suggested | no | none | **first** |
| L2 Second opinion | a gateway model from another family critiques the direction sheet or reviews the images, after a quote and a data-transfer consent | selected brief or images, to an **additional provider** (the brief already goes to the image provider) | cents | only if evidence shows a gap worth closing |
| L3 Delegate | a gateway model performs a step | possibly the whole context | cents to tens of cents | **not now**; if ever, explicit opt-in per step, never automatic, and a data-transfer decision, not only a spend one |

### 4.1 L1 in detail

`python scripts/image.py llm-advice [--model "<name>"]` (free):
1. reads `/models` live (price, context, APIs, modalities as listed);
2. maps the self-reported assistant to a gateway entry, or says "not on the gateway";
3. checks host capabilities;
4. looks up **evidence** and prints a compact block. Numbers stay internal; users see levels and an evidence line. Illustration:

```
ASSISTANT FIT   yours (self-reported): Claude Sonnet 5     evidence: measured, our tests 2026-10 (3 briefs, 34 images)
Step                 Your assistant      Best measured alternative                  Recommendation
Creative Direction   Strong              Opus 5: stronger in our small test         optional
Plan writing         Strong              same tier                                  keep
Result review        Limited             Gemini 3.1 Pro: stronger, about $0.16 per  suggested (strong evidence)
                                          batch, sends your images to Google
Host check: can read image files: yes. Prices assume USD per million tokens (unverified).
```

### 4.2 Evidence tiers (replace score thresholds)

| Tier | Meaning |
|---|---|
| **Strong** | measured by us, at least 3 briefs and 2 repeat runs, the difference is larger than the observed run-to-run spread on at least 2 of 3 briefs, scored blind by a judge from another family with a human spot-check |
| **Suggestive** | better in our evaluation but with a smaller sample or inconsistent across briefs |
| **Insufficient** | no meaningful difference measured, or not measured |

**Recommendation = tier plus cost.** *Keep* when the evidence is insufficient, the difference is within noise, or the user's assistant is
cheaper. *Optional* for suggestive evidence. *Suggested* only for strong evidence. A first release will honestly say "insufficient
evidence" for most cells, and that is correct. There are no fixed numeric thresholds.

## 5. Evaluation records (the source of truth)

Each measurement is one record in a data file (`references/llm-fit/records.jsonl`); `llm-fit.yaml` is a **derived** summary.

| Field | Example |
|---|---|
| skill | `direction` + version or content hash |
| model | `anthropic/claude-sonnet-5` |
| task and protocol | Direction; Vision review: individual images at about 1k px |
| brief set / image set | the 3 briefs / the 34 adidas5 images |
| metrics | validity, reliability (valid JSON, finished, obeyed constraints), quality (rubric), defect recall, false-alarm rate |
| score, n, repeats | 4.3 / 5, 3 briefs, 2 runs |
| judge | model and family, blind, human spot-check |
| cost, latency, tokens | text, image and output tokens; gateway-reported cost; seconds |
| date, evidence type | 2026-10-08, measured / reported |

A change to the skill makes older records stale for it. Cost and latency are recorded from the first run so a quality-cost-latency view
is possible later; it is not built now.

## 6. The three tasks

| Task | Input | Metrics |
|---|---|---|
| **Direction** | 3 rich briefs: prompt 1 (adidas), a skincare serum launch, a children's-book character (so models cannot copy adidas5) | the Creative Direction mechanical checks (**validity**), format and completion rates (**reliability**), and the 1-5 rubric (**quality**) by a blind judge from another family |
| **Plan** | the same briefs; each model writes `plan.json` | validity only for now: `check`, `plan --dry-run` and the planner rules pass; whether a valid plan is also a *good* plan is later work |
| **Vision review** | a small defect benchmark: the adidas5 images (known defects such as the garbled "GO FURTHER." on r518 and the skyline bleed in RAW SPEED) plus clean controls | **recall** of planted defects and the **false-alarm rate on clean images**, reported separately |

**Vision defect taxonomy** (the reusable benchmark, grown over time as real runs produce labelled images):

| Group | Cases |
|---|---|
| Text | correct, subtle typo, garbled, missing |
| Product | correct, wrong details, missing logo, distorted |
| Human | normal, subtle anatomy defect, extra limb, bad hand |
| Composition | required subject absent, wrong crop, wrong orientation |
| Direction bleed | a signature element of one direction in another |

Labels come from my review and are confirmed by you once. Protocol (individual images or montages, resolution) is fixed and recorded, since it
changes both results and cost. The first release uses the natural adidas5 defects and treats results as suggestive; the taxonomy fills in
later.

## 7. Portability: the most useful result

The key product question is not "which LLM is best" but **how much assistant quality does Image Lab need**. Example of the kind of table
that would be actionable (illustrative, not measured):

| Tier | Direction | Planning | Vision review |
|---|---|---|---|
| cheap | acceptable | reliable | weak |
| mid | good | reliable | good |
| premium | good | reliable | strong |

If cheap assistants handle Direction and Plan but not vision review, the answer is a safer skill path for weak assistants plus advice
on review, not a recommendation to buy a bigger model everywhere.

- **P1a (no gateway charge):** run the Creative Direction first test through the agent tool **on the default model only (Claude Sonnet 5.5 in this
  session)**. It uses your session's allowance, not gateway money. No other model is used for the work, and Fable 5 is **gateway-only** (you chose that).
  The default model's results are recorded with their harness, because the same model behaves differently inside the coding agent and through the bare API.
  This model is not on the gateway list ("Claude Sonnet 5" is, not 5.5), so the report will say "not on the gateway" for it.
- **P1b (paid):** the same briefs across families through the gateway (section 9).
- Findings feed back into the skill (for example an empty keep-out line, invalid JSON, copied examples, or fewer than 3 real directions)
  as a safer path for weaker assistants.

## 8. Pricing and accounting gate (P0, before any budget is quoted)

1. One very cheap text call (about GPT-5.6 Luna, expected well under $0.001) to confirm the **price unit**, input/output accounting,
   cached-token behaviour and whether the gateway returns usage or cost in the response.
2. One vision call with a single image (expected a few thousandths of a dollar) to **measure** how images are tokenised and charged,
   recorded as text tokens, image tokens, output tokens and gateway cost.
3. Only then are the section 9 numbers re-quoted from measured values. P0 should cost **under $0.01**.

## 9. Cost of testing (estimates until P0, assuming USD per million tokens)

Revision 4 overstated this. I had assumed outputs of 6-7k tokens; the real size of what the models write is smaller (the finished adidas5
plan is about 3.2k tokens, a four-direction sheet about 3k). Re-estimated from measured file sizes (about 4 characters per token):

| Task (per brief, one run) | Input tokens | Output tokens | Fable 5 / Astra (10 in / 50 out) |
|---|---|---|---|
| Direction | about 8k (skill 4k, brief 2k, guidance) | about 3k | about $0.23 |
| Plan | about 10k (skill 4k, schema 2k, brief 2k, sheet 2.5k) | about 3.5k | about $0.28 |
| Vision review, 34 images at 1152x1440 | about 60k (1.5-2.6k per image, the biggest uncertainty) | about 3k | about $0.75 |

One premium model on the full protocol (3 briefs of Direction and Plan, one vision review): 3 x $0.23 + 3 x $0.28 + $0.75 = **about $2.3**.
Direction plus Plan alone (text only) is about $1.5 of that; the vision review is a third, and it is not text.

| Model | Role | Price in/out | Protocol | Estimate |
|---|---|---|---|---|
| GPT-5.6 Luna | cheap floor | 0.2 / 1.2 | full, 2 runs | about $0.10 |
| Gemini 3 Flash | cheap floor, vision | 0.5 / 3 | full, 2 runs | about $0.25 |
| **Claude Fable 5** (gateway) | premium (your choice) | 10 / 50 | full, 1 run | about $2.3 |
| **GPT-6 Astra** (gateway) | premium (your choice) | 10 / 50 | full, 1 run | about $2.3 |
| P0 gate and pilot | pricing, accounting, one real Direction call per premium model | | | about $0.5 |
| **Judge (Grok 4.6, a family none of the workers belong to)** | scores the Direction sheets blind against the rubric: 15 sheets, about 6k in and 0.6k out each | 2 / 6 | one run | about $0.25 |
| **Total** | | | | **about $5.8** |

**Why it is not tiny even for text.** Output tokens cost $50 per million on these two models, so each text call is $0.23-0.28, and the
protocol makes 6 text calls per model plus the vision review. The estimate also cannot see **reasoning tokens**: if a premium model thinks
before answering, those tokens are billed as output and could multiply the text cost several times. That is why a pilot comes first.

- **P0b pilot (about $0.5 total):** one real Direction call on each premium model through the gateway, to read the actual billed input,
  output and reasoning tokens. The whole run is re-quoted from that, and **the cap is set after it**, not now.
- **Hard stop: $6**, set after P0b (the estimate is about $5.8, so the margin is small and the plan shrinks first if the pilot reads higher). If the pilot shows large reasoning overhead, the plan shrinks
  before it grows: drop Plan on the premium models first (a valid plan is easy at that tier), then use fewer vision images.
- **Text-only option:** skip vision on the premium models. Direction and Plan only is about $3.0 for the pair, but then the weakest spot (result
  review) goes unmeasured on premium models. I would keep at least a 15-image vision set (about $0.35 each).
- **Order:** P0, P0b, the cheap pair, Fable 5, then Astra. Gateway-reported cost is added up after each call and the run stops when the next
  call could pass the cap. `max_tokens` is set on every call.
- **The judge is gated like everything else.** The default model (Sonnet 5.5) does its own work in-session but never scores its own output, and a worker
  never grades its own family. A gateway judge scores the Direction sheets blind; its quote is shown to you and **nothing runs until you confirm that
  spending**. A human spot-check by you stays in the plan. The vision task needs no judge: it is scored mechanically against the confirmed defect labels.
- **What this evidence can be:** one run of each premium model is **suggestive** at most. Strong needs 2 repeats (about $4.6 more, outside the cap), and
  Fable has no free repeats, since it is gateway-only. Astra and Fable both stay at one gated run unless you raise the cap later.
- **Harness is always recorded** (agent tool, or a bare gateway call), so results from the two paths are never merged.

## 10. Test plan

| Phase | What | Spend | Pass |
|---|---|---|---|
| P0 | pricing and accounting gate (section 8); `llm-advice` against a saved `/models` fixture; self-report mapping (exact, alias, unlisted); host-capability prompt; honest "no measured evidence" mode; unit tests with no network | under $0.01 | unit tests pass; live report prints facts; costs quoted from measured values |
| P1a | Creative Direction first test through the agent tool on the **default model only (Claude Sonnet 5.5)**, recording evaluation records and the harness | none at the gateway; uses your session's allowance | records written; failures categorised |
| P1b | the full protocol of section 9 on Luna, Gemini 3 Flash, Fable 5 and Astra, with a gateway judge, after Creative Direction's first test, P1a and the P0b pilot; each spend quoted and confirmed by you | about $5.8 (hard stop $6, set after the pilot) | actual cost within 25% of the quote; **at least one step shows a repeatable quality or cost difference that could change a reasonable user's model choice** (for example "Sonnet and Opus are indistinguishable for planning, but Gemini Pro catches many more visual defects"); the judge and the human spot-check agree on most items, read as suggestive at this size |
| P2 | portability findings fed into the skill | none | a safe path exists for the weakest assistants tested |
| P3 | optional L2 pilot: a second-opinion review of the adidas5 images by the best vision model, with quote and data-transfer consent | about $0.04-0.40 | catches planted defects the host missed, at the quoted cost; blocked for a person-photo experiment without consent |

## 11. Risks

| Risk | Mitigation |
|---|---|
| **Conflict of interest** (the platform recommends its own catalog) | rule 8 in section 3; "keep" is a first-class result; recommendations may name models off the gateway; evidence and method are in the repo |
| False precision | evidence tiers, qualitative wording, no scores shown; thresholds only after much more data |
| Small samples | repeat runs for noise; "suggestive" labelling; expand only when ambiguous |
| Self-reported model wrong | user override; unmapped stated |
| Host cannot read images | capability check first; second opinion or the user's own eyes |
| LLM judges favour their own family | judge from another family, blind, human spot-check |
| New spend and data flow | quote, spend consent and separate data-transfer consent, `max_tokens`, logged calls; person-likeness block |
| Scope creep (becoming an LLM router) | L1 first; L3 not now and never automatic |
| Evidence goes stale | records carry skill version and date; a skill change invalidates them |
| Gateway behaves differently per model | the evaluation exercises the real path; failures are recorded, not hidden |
| Modality unknown for 34 of 49 models | treated as unknown unless measured |

## 12. What changed from revision 1

- The thesis became "measure, then advise only when evidence is strong"; added task-specific and cost-aware wording.
- Renamed user-facing to **Assistant fit** (so "model" keeps meaning the image model).
- Replaced the 0.05 / 0.15 score thresholds with **evidence tiers** and no user-facing numbers.
- Evidence is now **evaluation records** (skill version, judge, tokens, cost, latency), with `llm-fit.yaml` derived.
- Added **host capability** (can it read images?), **separate spend and data-transfer consent**, the **person-likeness block** and L3 as
  never automatic.
- Added the P0 pricing and accounting gate (under $0.01) before any budget.
- First run is **5 models plus repeats, about $4 (cap $5)**, not 8 models.
- Added portability as the main result and a P1a through the agent tool.
- Split validity, reliability and quality; vision reports recall and false alarms separately and uses a defect taxonomy.
- P1 success is "a difference that could change a user's choice".

## 13. Decisions

**Confirmed by you (2026-10-08)**
1. **Scope:** L1 only for now, L2 later if evidence warrants, L3 not now.
2. **Evidence:** our own measured evaluations first; public benchmarks only as a labelled secondary source, and only after our own evidence.
3. **Models:** Claude Fable 5 and GPT-6 Astra (premium pair), with the cheap pair (Luna, Gemini 3 Flash) as the floor.
4. **Budget:** a higher cap than $2; the number is item 9.
5. **Sequence:** Creative Direction first test, then P1a, then P1b.
6. **Name:** "Assistant fit" in user-facing text, `llm-advice` in code.
7. **Fable 5 is gateway-only** (you do not want a free Fable run); the free agent-tool run is the **default model only (Sonnet 5.5)**.
8. **A model is either the default (free of gateway charge) or goes through the gateway, where the estimated spend is shown and confirmed first**
   (rule 9, section 3); no other agent-side model is used unless you select and confirm it for that run.
9. **Cap:** a **$6 hard stop, set after the P0b pilot**. The pilot reads the real billed tokens; the plan is re-quoted from it and shrinks before it
   grows (drop Plan on the premium models first, then use fewer vision images) if the pilot shows a larger cost than estimated.
10. **Cheap floor kept:** GPT-5.6 Luna and Gemini 3 Flash (about $0.35 for the pair).
11. **Scoring:** a gateway judge from a family none of the workers belong to (proposed Grok 4.6, about $0.25), shown with an estimate and confirmed by
    you before it runs, so the default model does not grade itself.

**Open**
None for now. The judge choice (Grok 4.6) can be changed at its quote.

**Revision 4 changes:** premium pair on the full protocol (about $3.2 each), recommended cap $8 (corrected my earlier "about $5" for the premium pair);
rule 9 (default models only, explicit confirmation otherwise); P1a restricted to Fable 5 and the default model; both Fable paths kept with the harness
recorded; open questions now the cap number, the cheap pair and the P1a list.

**Revision 5 change (your question about cost):** corrected the token assumptions from measured sizes (outputs about 3k, not 6-7k): a premium model on the
full protocol is about $2.3, not $3.2; total about $5.5, recommended cap $6 instead of $8; added the P0b pilot (one real call per premium model, to
see reasoning-token billing) and a text-only option.

**Revision 6 change:** recorded the cap decision ($6 hard stop, set after the P0b pilot).

**Revision 7 changes:** recorded your answers: the in-session run is the default model only (Sonnet 5.5); Fable 5 is gateway-only and gated; the cheap pair stays;
a gateway judge scores the Direction sheets, with a quote you confirm; total about $5.8 against the $6 hard stop.
