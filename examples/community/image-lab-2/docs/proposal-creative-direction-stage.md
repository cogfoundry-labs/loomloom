# Proposal: a Creative Direction stage for Image Lab 2

Status: **proposal for review (revision 5), the skill is built and tested (`creative-direction-t1-results.md`, a local write-up not in the repository: four free planning tests and paid image tests).** Date: 2026-10-08.
Related: `design-v2.md` (stages 2-3, decisions 50-51), `skills/plan.md`, the adidas end-to-end runs
`out/adidas4` (prompt 1, one concept) and `out/adidas5` (prompt 2, three territories).
Revision 2 folds in an outside review (ChatGPT) and my own analysis of it; revision 3 your count feedback; section 9 lists what changed.

## 0. The idea in one paragraph

> **The planner varies within an idea. Creative Direction generates the ideas worth varying.**
> Creative Direction expands the user's creative search space; it never replaces the user's starting idea.

Today Image Lab can take a brief and explore camera, lighting and similar knobs of one concept. It cannot take one rich brief and
come back with several (3 to 10, default 4) genuinely different, deliberate creative directions of it. The adidas4 and adidas5 runs show the gap.
adidas5 is not the target; it is an example of the *reasoning* we want the system to do on its own.

## 1. The problem, with evidence

| | adidas4 (prompt 1) | adidas5 (prompt 2) |
|---|---|---|
| Input | one rich prompt for one collage | a three-territory campaign brief |
| Plan | 6 dimensions of one concept (accent color, pose, framing, graphic emphasis, color blob, type treatment) | 3 territories, each with its own idea, graphic system and headline, plus 5 variation dimensions and 11 scoping rules |
| Result | 32 images, one look with variations | 34 images in three clearly different, intentional styles |
| Judgement | useful, but "one prompt, varied" | "an art director thought about it" (user) |

Prompt 2 was written by ChatGPT *from* prompt 1. A real user brings something like prompt 1, and today that input never reaches the
thinking that made adidas5 good. Findings from the spec review:

1. The planner (stages 2-3) only picks production knobs (camera, lighting, environment, style...). It has no notion of creative
   directions, brand or audience reasoning, or brief analysis.
2. A rich single prompt is routed to **quick mode** (used verbatim, 2-3 models) unless the user says "explore".
3. The planner skill caps a plan at 4 dimensions, 4 values and about 150 valid combinations, and `scripts/planner_eval.py` enforces
   `MAX_DIMS 4`, `MAX_VALID 150`. adidas5 (6 dimensions, 546 combinations) would fail that check.
4. A hierarchy (a direction switch that changes what other dimensions mean) was unsupported until `batch_by` (decision 51); its
   scoping rules (11 excludes in adidas5) were written by hand.
5. The evaluation set has no rich campaign brief.

## 2. Goal, non-goals, principles

**Goal.** From a standard single prompt like prompt 1, produce **three to ten creative directions (default 4)** that are distinct,
deliberate and richly specified, behind **one** plan confirmation, with no new spend path and every existing hard rule intact. This is
a batch tool, so more choices are welcome; section 4.5 says how the count is kept affordable and honest.

**Non-goals.** Picking a "best image", a brand-guideline system, autonomous iteration, web research by default. Not for simple
prompts ("a red apple, studio photo"): those stay in quick mode.

**Principles**
1. **Your idea stays.** Direction 1 is the user's own brief; the others are alternatives around it.
2. **The user's copy is invariant** in every direction (MVP hard rule). Suggested headlines per direction can come later.
3. **Direction is a user choice, not a surprise.** Exploring is offered, not imposed.
4. **Richly specified, not necessarily long.** A direction resolves the creative decisions that matter for that brief and marks the
   rest not applicable.
5. **Real diversity.** A direction counts only if its defining choices change the *picture*, not just the adjectives.

## 3. Where it fits

```
BRIEF -> UNDERSTAND -> CREATIVE DIRECTION (new) -> SCHEMA -> MATRIX / BATCH -> PREFLIGHT -> APPROVE -> GENERATE
```

- **Creative Direction** answers: what ideas are worth exploring?
- **Schema** (existing stage 3) answers: which dimensions describe them?
- **Matrix and batch** (existing stages 4-5) answer: how do we sample them efficiently?

No existing stage is renamed. The output of Creative Direction is a **Direction Sheet**, which Schema turns into the same `plan.json`.

## 4. What would change

### 4.1 Routing (agent judgment, one cheap question)

Explicit user words win. Otherwise ask once only when the brief is open to several interpretations.

| Input | Route |
|---|---|
| "generate exactly this", short or plain prompt, "N options of this" | quick mode (unchanged) |
| words like explore, directions, concepts, campaign, ideas | Creative Direction, no question |
| a brief that is creatively open-ended (campaign or brand language, several valid interpretations), no such words | **ask once**: *Generate as written* or *Explore creative directions* (recommended, with one line why) |
| reference photo, "vary X" | planner as today, plus Creative Direction when the brief is open-ended |

No character-count threshold: a 400-character brief can be a sophisticated campaign brief, and a 2,000-character prompt can be a
fully specified single image.

### 4.2 The Creative Direction stage (`skills/direction.md`)

**Step A. Read the brief.** Show one short table with four labels, so the user can see what was taken as given and what was guessed:

| Label | Meaning | Becomes |
|---|---|---|
| **Said** | facts and constraints the user stated: brand, product, audience, objective, mandatory words, aspect, "never" rules | the shared opening and closing of every prompt; never varied |
| **Inferred** | the agent's interpretation of the brief | the direction cards; editable |
| **Brand assumptions** | general knowledge used to make a direction appropriate (for example "adidas communication emphasizes movement and performance"), always labelled | shown for accept or reject |
| **Open gaps** | what the brief does not decide | assumptions the user can correct |

The brief's own lists ("electric blue, solar yellow, vivid red", "diagonal, centered break...") are collected as candidate values.

**Step B. Choose 3-10 directions (default 4; the user can ask for a number; see 4.5 for what a larger number costs).**
- **Direction 1 is the user's brief, kept faithfully by a coverage rule** (revision 4; the first test showed word-for-word is impractical for a long
  brief): every instruction kept, compressed where needed, with exact quotes for names, copy, numbers, proportions and "never" rules. A **coverage list** of the
  brief's named and numeric specifics shows where each is preserved, and the confirmation never claims "exactly as you wrote it" unless it is.
- The others contrast with it and respect the brief's **hard constraints**; contrast is on style, medium, setting, light, energy, density and typography. A
  direction that relaxes a rule says so on its card ("relaxes: ... needs your OK"). At least one direction deliberately contrasts with the user's own style, and
  from 4 directions up one **wildcard** may leave the brief's palette or setting.
- **Thumbnail test** (distinctness): each direction states its dominant background, medium and palette; no two may match on both background and medium, and at
  most two may share a base (the first test's weak spot was several directions sharing an off-white paper cutout base).
- **Coherence check** and **concise visual wording** per direction (no self-contradictions, no long ingredient lists).
- **Distinctness heuristic** (a guardrail, not a proof): score each direction on five feel axes (medium or treatment, setting,
  energy, graphic density, typographic voice); aim for at least 3 axes of difference between any two (2 once there are more than
  six directions, because there are only so many truly different feel points). Two directions can pass this and still look alike,
  so the real test is the human evaluation in section 5.
- If the brief honestly supports fewer than the requested number, the agent says so and offers fewer instead of padding with
  near-duplicates. Two directions only on the user's explicit request.
- Each direction has one line saying which brand or audience insight it serves, labelled as an assumption when it comes from
  general knowledge.

**Step C. Specify each direction (adaptive completeness checklist).** A direction resolves each creative decision that matters for
the brief, and writes **N/A** for the rest:

idea · subject and action · visual treatment · composition · color and light · relevant graphic or typographic treatment ·
emotional intent · differentiation · **keep out** (what this direction must not contain).

**Keep out is always required.** In adidas5 the RAW SPEED images leaked city skylines until the line "no city imagery" was added.
A portrait brief does not need a graphic system or exact typography; a campaign key visual does.

**Step D. Variation inside each direction.** A direction's defining traits are **fixed** (they live in its own wording). Variation is
allowed only where it does not weaken the direction. Each direction needs **at least two variable levers with at least two values
each**, so its images are related but not repetitive (the quality bar's item 10); everything else may be fixed. Values come from
the brief's own lists first, then a standard lever set (composition, movement or pose, color accent, graphic language, typography
treatment, texture or light). The agent also notes where a value applies to one direction only (see `only_in`, 4.3).

**Fake-diversity guard.** Directions that differ in words but compile to near-identical pictures (same camera, pose, framing,
background, hierarchy) do not count. The direction-defining choices must show up as different compiled prompt content and different
composition, setting or medium, and the evaluation looks at the generated images, not at the direction text.

### 4.5 How many directions, and what it costs

Each direction needs enough images to show its own variation: **at least 4** (two variable levers x two values), about 8 for a
comfortable first look. So the first batch scales with the count instead of staying at 30:

| Directions | Images per direction | First batch | Approx. cost (at the measured $0.0163/image) |
|---|---|---|---|
| 3 | about 10 | 30 | $0.49 |
| **4 (default)** | about 8 | 32 | $0.52 |
| 6 | about 6 | 36 | $0.59 |
| 10 | 4 | 40 | $0.65 |

These are estimates at one model, size and quality; the preflight quotes the real number. Two things change as the count grows:
1. **Review load.** Ten direction cards are a lot to confirm and ten territories' images a lot to read. Above 6 directions the agent
   shows a one-line-per-direction map first, and the evaluation (section 5) includes a run with 8 directions to see whether
   distinctness survives.
2. **A cheaper "contact round" above 6 directions.** Generate **one image per direction first** (10 directions = about $0.16),
   let the user pick which directions deserve depth, then spend the rest on those. This is the same 1-image-per-direction diversity
   gate used for the default case (section 5, T2), applied to all directions.

The batch size is `max(30, 4 x directions)` by default and stays editable in the workbook.

### 4.3 Code changes (all enablers; built after the first test)

1. **Value scoping**: `{"value": "hand-drawn", "fragment": "...", "only_in": {"direction": ["RAW SPEED"]}}`. `load_plan` expands it
   into today's `exclude` constraints, so nothing downstream changes, and it replaces the 11 hand-written excludes (a source of
   values and rules disagreeing).
2. **Dry run shows each direction's "defines / varies" lines** and its share of the batch. The full matrix is available on request,
   not shown by default.
3. **`preflight --only r001,r163`** for a priced calibration without moving workbooks and editing the ledger by hand.
4. **`image.py montage --dir D --by direction`** (the hand-built montages of adidas5).
5. **`planner_eval.py`**: relax the fixed 4 / 4 / 150 limits for direction plans, add direction rules and 3 rich briefs.
6. **Docs**: `design-v2.md` (stage 2a, decision 52), `SKILL.md` routing, `skills/plan.md` (hand-off), `plan-schema.md` (`only_in`).

### 4.4 What the user sees (one confirmation, prompt 1 as input)

1. *Generate as written* or *Explore creative directions* (recommended: this reads as a campaign brief).
2. The brief read-out (Said / Inferred / Brand assumptions / Open gaps).
3. Direction cards (3-10, default 4): name, one-line idea, the insight it serves, what makes it different, keep-out, and the copy (always the
   user's).
4. For each direction, what is fixed and what varies. The usual numbers: images per direction, model, size, price basis, limits.
5. One confirmation, then the existing preflight and spend gate. The first spend is a **calibration with one image per direction**
   (4 images by default, section 5, T2), which doubles as the diversity gate.

## 5. The test, using prompt 1 as input

Prompt 1 = `out/adidas-src/brief2.txt` (the collage prompt, 8,512 characters). adidas5 is a reference point, not a target.

**Contamination rule.** The skill's worked examples must come from another domain (not adidas, not a running shoe), otherwise the
test agents copy adidas5 and T1 proves nothing. Limit to know: the test agents run on the same model family as the author, so T1
shows whether the skill reproduces good reasoning, not independent creativity; hence three runs, and a different fresh agent does
the scoring.

**T1. Blind planner runs (free: agent tokens only, no gateway call; needs only the skill, no new code).** Give **3 fresh agents** only
prompt 1 and the new skill (no prompt 2, no adidas5, no other agent's output). Each writes a `plan.json` with hand-written excludes
if needed. Score each plan:

*Mechanical checks (script plus read)*

| Check | Pass |
|---|---|
| Directions | 3-10 (default 4); Direction 1 passes the coverage rule (every instruction kept, exact quotes for copy, numbers and never-rules, coverage list shown) |
| Invariants | brand, product, copy, 4:5 and the quality bar in every compiled prompt; no words added that the brief did not allow |
| Completeness | every relevant checklist item resolved in each direction, keep-out present, N/A used only where truly irrelevant |
| Variation | at least two variable levers, two values each, per direction |
| Distinctness heuristic | at least 3 of 5 axes between any two directions (a guardrail) |
| Mechanics | `check` and `plan --dry-run` pass; `batch_by` set; prompt within the model limit; size and price basis printed |
| Confirmation | one message with the read-out, cards and numbers; no per-dimension questions |

*Human rubric, 1-5 each, scored blind by a different fresh agent from plan summaries with labels removed, and by you if you wish*

| Question | |
|---|---|
| Faithfulness | does Direction 1 genuinely preserve the user's idea? |
| Distinctness | do the directions feel meaningfully different? |
| Intentionality | does each feel like a deliberate art-direction choice? |
| Specificity | could a generator execute it reliably? |
| Coherence | do the choices inside each direction reinforce each other? |
| Usefulness | would a designer want to see these alternatives? |

Plus the deciding question: **would you choose one of these directions before seeing any images?** Success: at least 2 of 3 plans
average 4 or more and pass the mechanical checks. Add one **8-direction run** (a fourth agent, same brief) to see whether
distinctness and the rubric scores survive a larger count; it is informational and sets the guidance for counts above 6. A blind side-by-side with plan5 is shown as a reference, not a pass criterion
(the headline copy will differ because prompt 2's headlines came from ChatGPT; judge the ideas).

**T2. Paid validation (needs your approval; only if T1 succeeds).** Take the best T1 plan.
- **Diversity gate:** calibration of **4 images, one per direction (about $0.065)**. You and I check that the four look like
  different creative directions before anything more is approved. If not, stop or revise the wording; nothing else is spent.
- Then the other 28 images (about $0.46). **About $0.52 total (32 images)** at the measured $0.0163 per image (1152x1440, quality
  medium, GPT Image 2.5 Sunburst), ceiling **$0.65**.

| Check | Pass |
|---|---|
| Execution quality | at least 90% of images pass the visual checks (text spelled right, product visible, anatomy, no other brand); adidas5 was 29 of 30 on my by-eye review |
| Direction separation | a shuffled montage with labels removed is sorted into the right direction at least 90% of the time |
| Coherence | images within a direction clearly belong together |
| No repetition or bleed | images within a direction are related but not repetitive, and no direction contains another's signature elements |
| Usefulness | you would reasonably consider them different creative options |
| Creative quality | judged by you, at least the adidas5 level |

**T3. Regression (free).** "A red apple, 4 options" and "generate exactly this" still route to quick mode and quote as before. A plan
with no `only_in` behaves exactly as today. The existing 255 tests keep passing.

**Cost summary:** T1 and T3 spend nothing at the gateway. T2 is about $0.52 (ceiling $0.65), and the first $0.065 of it is the gate.

## 6. Build order (prove first, tools after)

| Step | Content | Spend |
|---|---|---|
| 6a | `skills/direction.md` (no adidas examples), routing text in `SKILL.md` and `plan.md`, planner-eval rubric and rich briefs | none |
| 6b | **T1**: three blind agents, scoring, your review | none |
| 6c | decide from T1: build only the tools it showed are needed (`only_in`, dry-run lines, `preflight --only`, `montage`, eval limits) with tests, plus T3 | none |
| 6d | **T2** (4-image gate, then 28), only if you approve and T1 succeeded | about $0.52 |

## 7. Risks and how they are handled

| Risk | Mitigation |
|---|---|
| **Fake diversity** (directions sound different but render alike) | the real-diversity principle; direction-defining choices must change compiled content and composition, setting or medium; the one-image-per-direction gate; blind sort in T2 |
| **Over-direction** (the agent replaces the user's idea) | Direction 1 keeps every instruction (coverage rule); other directions respect the hard constraints or say they relax one; copy is invariant; the user confirms before anything is built |
| Generic directions ("bold", "minimal", "vintage") | the brand or audience line, keep-out line, the 5-axis heuristic, T1's blind rubric |
| Over-specified template | the checklist is adaptive with N/A; what counts is that every relevant decision is resolved |
| Invented brand facts | the Said / Inferred / Brand assumptions split; assumptions are labelled and rejectable |
| Over-fixed directions (images repeat) | at least two variable levers per direction |
| More directions mean more images, cost and review | batch size scales as max(30, 4 x directions) and is quoted before spending (4.5); above 6 directions a cheap one-image-per-direction contact round comes first; same spend gate |
| Distinct directions run out as the count grows | the agent offers fewer rather than pad with near-duplicates; the 2-axes minimum above 6; the 8-direction evaluation run |
| Long prompts | direction wording is about 1,700 characters each, so about 4,000 per prompt; models limited to 500 characters (Seedream) are excluded as today |
| Test contamination or same-model bias | no adidas examples in the skill; three runs; a different fresh agent scores |
| Not all briefs are campaigns | the stage runs only for open-ended briefs or on request; "direction" also fits a portrait (documentary, studio, painterly) |

## 8. Decisions (defaults assumed; change any you disagree with)

1. **Routing:** ask once for an open-ended brief with no "explore" words; explicit words win. *(default)*
2. **Copy:** the user's copy in every direction, a hard MVP rule. *(default)*
3. **Count:** **3 to 10, default 4** (your feedback; revision 3), fewer only when the brief honestly supports fewer or on request. *(default)*
4. **Name:** "direction" everywhere; the skill is `direction.md`, the artifact a Direction Sheet. *(default)*
5. **T2:** run only if T1 succeeds, starting with the one-image-per-direction gate (4 images by default). *(default)*
6. **Brand knowledge:** general knowledge with labelled assumptions, kept apart from what the brief says. *(default)*
7. **Build order:** skill and T1 first, tools after. *(new)*

## 9. What changed

**Revision 3 (your feedback on the count):** directions are now **3 to 10, default 4**. Added 4.5 (how many directions and what it
costs; a first batch of max(30, 4 x directions); a one-image-per-direction contact round above 6), a 2-axes distinctness minimum
above 6, an 8-direction evaluation run, a 4-image diversity gate and updated T2 costs (about $0.52, ceiling $0.65).

**Revision 2 (outside review and my analysis), changes from revision 1:**

- Added the thesis, the product principles and the three-question layering (Direction, Schema, matrix).
- Renamed "Direct" and "territory" to **Creative Direction** and **direction**.
- Routing now relies on explicit words and agent judgment instead of length thresholds.
- Added the **Said / Inferred / Brand assumptions / Open gaps** read-out and made Direction 1 the user's text verbatim.
- The richness checklist became an **adaptive completeness checklist** (N/A allowed), with **keep out** always required.
- The "at least 2 values per lever" rule became "**at least two variable levers per direction**", and fixed defining traits are
  allowed; the matrix is derived, not shown by default.
- The 5-axis rule is now a **heuristic**, not a rule.
- Added **fake diversity** and **over-direction** to the risks, plus a test-contamination rule.
- T1 gained a 1-5 rubric, blind scoring by a different agent and the deciding question; adidas5 is a reference, not a target.
- T2 gained a **3-image diversity gate** and coherence, no-repetition and usefulness checks.
- The build order is now **prove first, tools after**, and the code changes are labelled enablers.

**Revision 4 (after the first test, `creative-direction-t1-results.md` (a local write-up with the test images, not in the repository)):** Direction 1 is now kept by a **coverage rule** with a coverage list instead of being
verbatim; a **thumbnail test** for distinctness (background, medium, palette; no two alike, at most two share a base); directions **respect the brief's hard
constraints** and mark any rule they relax; a per-direction **coherence check**; **concise visual wording**; an optional **wildcard** direction. The first test's
findings: verbatim impractical for an 8.5k-character brief, and distinctness the weak spot.

**Revision 5 (after the paid test T2, your review: richer than before but not as rich as adidas5, and the directions too alike):** the 30 images differed in ground, medium and palette
but not in layout, type scale or density, because every direction was the same poster template and no direction had a layout lever; blue was the accent in three of five directions.
Changes: **look traits** (ground, medium, layout, type, density, palette) on every direction, enforced by `check` (any two directions differ in at least three of six; at most two share a
palette); a **picture-structure contrast** rule (at most one "cutout runner on a poster ground with a stacked headline", at least one full-bleed photograph with the subject in the scene,
one with large negative space and quiet type); a mandatory **composition lever** with at least three values, at least three variable levers per direction and one lever with three or
more values; a **color lever** when the brief names colors; each direction **owns its palette**; **one runner description for the campaign** in the shared opening.
