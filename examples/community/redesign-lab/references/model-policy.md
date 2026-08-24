# Model policy

RFC-0003 (loomloom's Quality-Leader/Efficiency-Leader model routing) is
unimplemented: verified `Status: Draft`, and a grep of the entire loomloom
Go source for `QualityLeader`/`EfficiencyLeader`/`Redundancy` returns nothing.
The only model-selection data loomloom exposes today is a flat list per
execution unit with a single `isDefault` boolean
(`src/cli/internal/cmd/template_spec.go:70`): no quality or cost tiering.

Consequence: every loomloom call this pipeline actually makes hand-pins a
specific model. Never rely on `isDefault`.

| Step | modelKey | Why this one, not isDefault |
|---|---|---|
| Share's narrative pass (`scripts/build-case-study.py`'s `case-study-narrative` template, `text-generate`) | `google/gemini-2.5-flash` | The only loomloom call this pipeline makes today — real, explicit `defaultModelRef` in the registered template, not left on `isDefault`. |

**Not active policy — kept for history only:** aesthetic-advisory scoring
(`scripts/score.py`'s optimized path) was originally meant to pin
`google/gemini-3.1-pro-preview`, but that path is confirmed broken (see
"Known gap" below) and doesn't run in this pipeline at all — Explore
Variants has been 100% agent-scored, free, since rev 6 (see
`evaluation-rubric.md`, `explore-variants.md`, `SKILL.md`). Don't read this
row as a live pin; it documents a call that was designed but never actually
wired up.

When a second real loomloom call is added to this pipeline, add its row to
the active table above before wiring it up: don't leave a step on
`isDefault` by omission.

Raise the RFC-0003 gap with the loomloom team directly. This file is a
documented workaround, not the intended long-term answer.

## Correction: `anthropic/claude-opus-5` does not exist in this environment's catalog

The original pin above was `anthropic/claude-opus-5`, chosen on the reasoning
alone, never checked against a real catalog. Running `loomloom template-spec
models text-generate` for real (2026-08, during the Bunnings redesign run)
returned zero Anthropic models of any kind: only `deepseek/*` and
`google/gemini-*`. `google/gemini-3.1-pro-preview` is the strongest available
model with real vision input support (DeepSeek's listed models are text-only,
no vision) — the correct real substitute given the same "worth the premium
model" reasoning, not a downgrade of it. Re-check this catalog before
trusting any modelKey pinned by reasoning rather than by running the actual
`models` command against the target environment.

## Known gap: `asset_ref` inputs do not reach the model on a `reference` port

**Correction (2026-08, hello-world-site run 2): the original diagnosis below
misattributed this to loomloom before ruling out this pipeline's own client
bug.** The original Bunnings finding was real (the model never saw the
image), but its cause — "`asset_ref` → `reference` port binding is
documented but not implemented" — was never actually isolated from a much
more mundane possibility: `scripts/score.py` itself was building a malformed
request. Re-reading `scripts/score.py` alongside loomloom's own docs
(`build-your-first-skillbot.md`, `template-spec docs examples`) surfaced a
real, independent bug: the script called `loomloom market run` (the command
family for a *published Market listing*) against what is actually a
*private* template, and built input rows shaped like
`{"variant_id": ..., "prompt": ..., "image_b64": <base64>}` — none of which
match the TemplateSpec's real field keys (`screenshot`, `rubric`). The
correct flow for a private template, confirmed against the CLI's own
`--help` output, is:

```
loomloom input-asset upload <screenshot.png> --content-type image/png   # -> input_asset_id
echo '{"screenshot":"<input_asset_id>"}' > rows.jsonl
loomloom orchestration-input upload rows.jsonl                          # -> input_file_id
loomloom template-spec precheck <template-id> --version-id <v> --input-file-id <id>   # free cost estimate
loomloom template-spec run     <template-id> --version-id <v> --input-file-id <id>    # real run
```

`scripts/score.py` used none of this — wrong command family, wrong field
key, base64 inlined directly instead of uploaded as a real asset. That alone
was sufficient to explain the original "no image was provided" result
without any platform-level defect. This is now fixed in `scripts/score.py`.

**But the corrected flow was then verified for real, twice, and the
underlying gap is real too — it was just masked by the client bug above.**
Two fresh, real, paid runs against the *correctly* formatted request (fresh
`input-asset upload` of `.output/helloworld-run2/variants/v5-spec-table/desktop.png`,
126,867 bytes, confirmed `image/png`, bound through the documented
`screenshot` → `reference` port path):

- Run `2e479dae-fe83-45e5-a486-06ebb0daba6c` (`google/gemini-3.1-pro-preview`,
  $0.0032): *"5/10. I am assigning a neutral score because no design or
  image was provided for me to evaluate."* — an honest refusal.
- Run `2d4fdd00-a50e-4457-a2e9-7c430dd33c79` (`google/gemini-2.5-pro`,
  diagnostic template version `138f5022-01e0-43f0-8db2-da22e6f060ef`,
  $0.0030): *"8/10... premium feel through its dark palette... vibrant
  gradient..."* — **worse than a refusal**: the actual screenshot is an
  off-white/paper Swiss-print design with black ink and one red accent, no
  dark palette, no gradient anywhere. This model fabricated plausible,
  specific-sounding aesthetic language completely disconnected from the real
  image, rather than reporting that it saw nothing.

So: real client-side bug, now fixed, confirmed not sufficient to explain the
whole failure. Across two different models with a request format now
verified correct by direct CLI experimentation (not just doc-reading), the
image still doesn't reach the model — and at least one model's failure mode
is silent fabrication rather than a clean refusal, which is more dangerous
for this pipeline's use case (a human could trust "8/10, dark palette" as
real feedback).

**Third real test, definitive: the gap is general, not image-specific.** To
rule out "maybe it's just images, or just these two models," a minimal
diagnostic TemplateSpec (`asset_ref`, `acceptedMimeTypes: ["text/plain"]`,
same `initial_input` → `reference` port binding, `google/gemini-2.5-flash`
— cheapest catalog model) was built and run against a real 231-byte text
file containing three distinctive, made-up facts (a named dog, a specific
batch number, a specific bench color/clamp count) that a model could only
report if it had actually read the file. Run `bce7b58d-7b93-4520-8116-981f557ee300`,
real cost $0.000147: *"No document was provided."* A second attempt using
the `text_reference` valueType instead of `asset_ref` for the same file
failed even earlier, at the free `precheck` step, with a distinct server
error (`"row 0: no non-empty fields found"`) — a separate bug specific to
`text_reference` rows, not investigated further since `asset_ref` (the type
this pipeline actually needs) was already conclusively shown broken on its
own.

This is now airtight: 3 real runs, 3 different models
(`gemini-3.1-pro-preview`, `gemini-2.5-pro`, `gemini-2.5-flash`), 2 MIME
types (image/png, text/plain), a request format verified correct against
every relevant doc topic (`inputs`, `input-schema`, `bindings`, `steps`,
`execution-units`, `spec`) and the CLI's own `--help` output. `asset_ref` →
`reference` port binding via `initial_input` simply does not deliver
content to the model, for any input type tested. `scripts/score.py`'s
optimized path must not be treated as usable until loomloom confirms a fix
— re-run the same real-upload + real-response-text check (not just an
absence of errors) before trusting it again. Free mechanical checking is
unaffected: it never depended on loomloom at all.

Raise this gap with the loomloom team directly, same as RFC-0003 — this is a
platform-level finding, not something fixable from this pipeline's side. The
three run IDs above are real, reproducible evidence to hand them directly.

## Confirmed working: two loomloom paths that never touch the broken port

Both tested for real, 2026-08, specifically because they avoid `asset_ref` →
`reference` port binding entirely — the exact mechanism confirmed broken
above.

**Pure text-in/text-out (`text-generate`, no reference port at all).** A
minimal TemplateSpec with two plain `string` fields (`before_text`,
`after_text`) combined into the `prompt` param via `paramBindings` — no
`asset_ref`, no upload, no `reference` port. Run
`68c958ab-0b2b-4dd1-96a4-b443273056e1`, real cost $0.0007777: given a real
before/after nav-label snapshot with one deliberately injected change
("About" → "Studio"), the model correctly identified exactly that change and
nothing else. This is the mechanism behind the proposed preservation-diff
check at Validate (see the design-authority.md discussion) — confirmed
viable.

**Prompt-only `image-generate` (no reference image at all).** A minimal
TemplateSpec with only a hidden default-value `prompt` field, no input
fields requiring upload. Run `edc5c520-edd6-4968-ae9e-19314e5d2577`
(`google/gemini-2.5-flash-image`), real cost $0.2979902 (same 7x-over-precheck
pattern noted above — precheck said $0.0429957): produced a real, on-topic
abstract geometric texture in the exact requested muted terracotta/taupe/
off-white palette, no text, no logos, nothing unrelated. This is the
mechanism `implement-design.md` already reserves for supplementary
decorative assets (textures, not content photos) — confirmed viable, though
note the real cost can run ~7x the precheck estimate for this specific
model/unit combination.

**Net implication:** the confirmed platform bug is scoped specifically to
`initial_input` → `reference` port delivery of an uploaded `asset_ref`. Any
loomloom call that avoids that one binding — plain text fields, or a
prompt-only image-generate with no reference input — works normally. Two
paths use this pipeline could adopt safely: preservation-diff checking (text
fields, no image) and prompt-only supplementary asset generation (no
reference image). Aesthetic-scoring of a variant's actual screenshot remains
broken, since scoring a specific rendered image is inherently a
reference-port task.

**Fourth real test: `image-generate`'s `reference` port shows the same gap,
plus a new, separate cost-estimation problem.** To check whether the defect
is specific to `text-generate`'s handling of the `reference` port, a
diagnostic TemplateSpec bound the same real screenshot (`ia_0b737ea9-...`)
to `image-generate`'s `reference` port (`google/gemini-2.5-flash-image`),
with an explicit instruction: match the reference's exact palette, or — if
no reference was actually received — generate a plain grey square with "NO
REFERENCE IMAGE RECEIVED" in red text, so either outcome would be
unambiguous. Run `045a0c85-24f6-4b8f-b23f-d428dea7fe1e` produced neither: a
photorealistic empty apartment interior with grey tile flooring, matching
nothing about the reference image's actual style (off-white paper,
black/red Swiss-print design) nor the explicit fallback instruction. The
model appears to have received little to no grounding from either the
reference asset or, seemingly, much of the prompt text either — a generic,
unrelated output. This extends the finding beyond `text-generate`: the same
`asset_ref` → `reference` port → `initial_input` gap affects `image-generate`
too.

**Fifth real test: the actual production flow, not an isolated diagnostic.**
After fixing `scripts/score.py` (correct command family, correct field key,
real upload flow — see above), it was run for real against all 6 real
variants from a live `redesign-existing-site` run
(`.output/helloworld-run2/variants/`), through the exact same
`input-asset upload` → `orchestration-input upload` → `template-spec
precheck` → `template-spec run` flow the fixed script now uses. Run
`0d67cb57-e87f-42e6-9dbc-81805059c709`, real cost $0.0232848 for 6/6
"completed" tasks: **every one of the 6 responses said some version of "no
image or design description was provided,"** with inconsistent scores
attached anyway (1/10, N/A, N/A, no score given, N/A, 5/10) despite each one
admitting it saw nothing. This is not a diagnostic-template artifact — it's
the real production path this pipeline would actually use, run at real
6-variant scale, still 0/6.

Across every test run today: 11 total failing task executions (1 Bunnings +
2 single-image + 1 text + 1 image-generate + 6 in this batch), 100% failure
rate at delivering referenced content to the model, across 4 models and 2
execution units. This is as confirmed as evidence gets without loomloom's
own server logs.

**Sixth real test: a second live-site run (larstornoe.com), by explicit user
request specifically to gather one more real data point.** Run
`cf47317d-8668-4b95-a8fa-60b572964c9d`, real cost $0.0238392, 6/6 "completed"
tasks, against the fixed `scripts/score.py` scoring 6 real
`industrial-brutalist-ui` variants of a furniture-designer's portfolio site.
Every one of the 6 responses again reported no image/description was
provided (one gave "Score: 0", one "Pending/10", the rest "N/A" — the
inconsistent-scoring-despite-blindness pattern holds too). Running total
across all real tests today: 17 failing task executions, 0 successes, 4
models, 2 execution units, 2 completely different real target sites.

**Separate finding, from the fourth (image-generate) run above: `precheck`
can badly underestimate real cost.** `template-spec precheck` estimated
$0.0429957 for that run; the actual charged cost was $0.2979902 — about 7x
higher. The fifth (6-variant text-generate) run above shows the opposite
direction instead — precheck estimated $0.0528, actual came in lower at
$0.0233 — so this looks specific to `image-generate`, not a general
precheck defect, but it's still a distinct problem from the asset-delivery
gap (billing accuracy, not content delivery) and worth raising with the
loomloom team separately: a user approving "optimized" at Gate 1 based on a
precheck estimate could be charged substantially more than what they
approved, at least for image-generate steps.
