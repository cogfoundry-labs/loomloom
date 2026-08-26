# TemplateSpec

Use this reference when creating, modifying, versioning, or explaining a private template or TemplateSpec.

## Contents

- [Documentation sources](#documentation-sources)
- [Conversation flow](#conversation-flow)
- [Usage mode](#usage-mode)
- [TemplatePlan](#templateplan)
- [Modeling and input rules](#modeling-and-input-rules)
- [Legacy v1 semantic preservation](#legacy-v1-semantic-preservation)
- [Creation and versioning](#creation-and-versioning)

## Documentation Sources

Before writing TemplateSpec, read the current CLI-bundled documentation:

```bash
loomloom template-spec docs spec
loomloom template-spec docs examples
loomloom template-spec docs conversation
loomloom capability resolve --input <modality> --output-modality <modality> --output json
```

- `spec` is the current JSON contract.
- `examples` contains examples and implementation patterns.
- `conversation` defines natural-language authoring behavior.

TemplateSpec docs default to English and are also available in Chinese. For example:

```bash
loomloom template-spec docs spec --lang zh-CN
```

Select the documentation language as appropriate for the conversation and task.

The installed Skill may contain a `generated-template-spec/` backup. Prefer the CLI docs command because it matches the currently installed CLI. Once each Step's business modalities are known, use `capability resolve`; its server response is the primary authority-backed selection result. Bundled docs are never authority for changing Profile revisions, eligible models, ports, or fixed contracts.

## Conversation Flow

When the request modifies, copies, or appends to an existing private template,
first run `loomloom template-spec get <template-id> --output json` and
`loomloom template-spec versions <template-id> --output json`. Resolve the exact
target version and inspect its server-returned `specVersion` before drafting.
For `template-spec/v1`, switch to the legacy upgrade gate in `SKILL.md`: keep
the historical version unchanged, export it only as source material, and draft
a new v2 version against current authoring facts. Do not apply this authoring
gate when the user only wants to execute the existing historical version.

When a user describes a reusable workflow:

1. Ask business questions, not TemplateSpec-field questions.
2. Ask one missing-detail question at a time.
3. For long-form material, explicitly ask whether future users will paste the content or upload a file; do not infer the transport from words such as "document" or "material".
4. Avoid user-facing terms such as `inputBindings`, `portId`, `fan-in`, `execution`, `outputSchema`, `provider`, and `mode`.
5. Restate complex workflows in business language.
6. Determine whether different roles or steps need different processing requirements.
7. Choose the future usage mode before drafting.
8. Draft a TemplatePlan first.
9. Show the TemplatePlan and wait for confirmation.
10. Generate TemplateSpec only after plan confirmation.
11. Run `loomloom template-spec check <spec.json>` against the same selected
    Server where the version will be created. This is the server-authoritative
    contract check, not an offline schema check.
12. Ask for a separate creation confirmation.
13. Create only after explicit creation confirmation.

Offer options when asking questions and include a reasonable default.

## Usage Mode

For multi-role, multi-step, multi-perspective, multi-style, or multi-channel templates, ask:

```text
When other people use this template later, how should the review/generation requirements be handled?

1. Preset in the template: users only fill the core material, and the system follows your predefined requirements automatically.
2. Let users fill them: users can fill or modify the requirements for each step/role.
3. Generate both versions: one simple version and one customizable version.

If you are unsure, choose 1 first to make a simple, usable standard template.
```

Do not expose `prompt`, `binding`, `reference`, `field`, `hidden`, `portId`, or `inputBindings` in this question.

Typical triggers include:

- multi-perspective PRD review
- multi-role contract review
- multi-agent event planning
- rewriting content in multiple styles
- resume review from multiple interviewer perspectives
- marketing content for multiple channels

### Simple mode

- Users fill only core material.
- Role/step processing requirements are preset by the template author.
- Processing requirements are not user-visible input columns.

### Custom mode

- Users fill core material and role/step processing requirements.
- Each requirement is visible and editable.

### Both versions

- Draft two TemplatePlans.
- Distinguish names with labels such as `Standard Version` and `Custom Version`.
- Confirm both plans before generating two TemplateSpecs.
- Each remote creation still requires its own explicit creation confirmation.

## TemplatePlan

Cover:

- template name and goal
- what one workbook row represents
- user input fields
- workflow steps and each step's goal
- serial, parallel, and summary relationships
- usage mode
- user-visible intermediate outputs
- final outputs
- failure policy when upstream steps fail or partially complete
- system error display in Excel
- default model selection
- special requirements

For separate product, engineering, and risk reviews followed by a summary:

- Model the reviews as parallel Steps with explicit execution bindings.
- Model the summary as a downstream Step.
- Declare `dependsOn`, then connect each consumed result with an explicit `stepOutput` input binding.
- Use `triggerPolicy=require_all` when every review is required, or `allow_partial` when the summary may run with the successful subset.
- For independently processed dynamic items, use one workbook row per item. For a fixed number of parallel branches, declare multiple Steps. TemplateSpec v2 does not provide dynamic-cardinality Step fan-out.

Add result and error columns for every user-visible step by default. If partial completion is allowed, make missing upstream results understandable and expose failed-step error columns.

Keep `provider` and `mode` internal.

## Modeling And Input Rules

### Template inputs and transport

Apply the canonical input rules consistently:

- Pasted text: declare a value Template Input and bind it to the target contract input port (`TS-IN-001`).
- Uploaded text file: declare an Artifact Template Input with compatible MIME types and bind it to the target Artifact port (`TS-IN-002`).
- Reject a design that declares a string value, tells users to enter an asset ID, and binds it as text (`TS-IN-003`).

Before showing TemplateSpec, verify that Template Input descriptions, Workbook sample rows, value or Artifact definitions, and bindings all describe the same transport.

### Contract rules

- Author only `template-spec/v2`; v1 is historical and read-only.
- Use lowerCamel fields such as `meta.name`, `templateInputs`, `steps[].stepId`, and `steps[].inputBindings`.
- Put user-facing inputs in the top-level `templateInputs` map. Put instructions and sample rows under `workbook`.
- Bind an exact model with `executionBinding.kind=fixedModelContract` and a real `subjectRevisionId` from the target environment.
- Prefer the exact contract returned by `loomloom capability resolve`; use `loomloom template-spec contracts <model-id> --output json` only for lower-level inspection. Do not infer contracts from the model catalog.
- Bind a replaceable model set with `executionBinding.kind=capabilityProfile`; also declare the separate Step-level `modelSelection` rule.
- Treat every `inputBindings` map key as the target contract `portId`. Never guess a port ID or use a role, file name, native JSON pointer, or shared field name as its identity.
- A target port has exactly one binding. Use `templateInput`, `stepOutput`, `literal`, `platformContext`, `composeValue`, `sequence`, or `merge` according to the current bundled documentation.
- Connect upstream values with both `dependsOn` and a `stepOutput` source that names the upstream stable output `portId`.
- Use `sequence` for one ordered heterogeneous multimodal value. Use `merge` for homogeneous Artifact collections; do not treat these as interchangeable.
- Do not bind `provider`, routing mode, complete contracts, or provider-native request objects.
- Never guess Step IDs, authority IDs, or port IDs.

### Text-generation Steps

`text-generate` uses the shared OpenAI-compatible capability Profile. It does
not require one `fixedModelContract` or Certification Subject per text model.
An empty result from `template-spec contracts <text-model-id>` is therefore
expected and does **not** mean that the model cannot be used in a private
TemplateSpec.

For a text-generation Step:

1. Run `loomloom capability resolve` with the Step's input and output
   modalities. Choose one Profile match and an eligible model returned by that
   exact target environment.
2. Set `executionBinding.kind=capabilityProfile` and use the response's stable
   `profileId`. Omit `profileRevision` for normal authoring; Core resolves and
   freezes the current revision. Do not require or fabricate a
   `subjectRevisionId`.
3. Use the response's Profile ports and write `modelSelection.defaultModelId`
   from its eligible model list. Do not copy a revision, port, or model list
   from bundled docs or an older installed Skill.
4. Choose the returned Profile by its input contract. Use
   `text.basic.openai-chat.v1` for text-only input. Use a returned vision Profile
   such as `text.vision.openai-chat.v1` only when it exposes an Artifact image
   port and the chosen model appears in that Profile's eligible model list.
   Bind uploaded images as Artifact Template Inputs; never pass an asset ID as
   a string prompt. Keep the optional model selector as a Template Input when
   users may choose another eligible model; leaving it blank uses the frozen
   default model.
5. A downstream image, video, or other fixed-model Step may consume the text
   Step's stable output through `stepOutput`. The absence of a per-model text
   authoring contract must never be reported as blocking such a workflow.

Use `template-spec contracts <model-id>` only when authoring a
`fixedModelContract` Step for one exact model, such as a model with its own
multimedia or provider-native input structure.

## Legacy v1 Semantic Preservation

A v2 candidate is not an upgrade merely because `template-spec check` returns
`valid=true`. Check validates the v2 contract against the current Server; it
does not compare business meaning with the historical v1 source.

Before drafting, record a semantic ledger for every v1 Step:

- Step ID, display purpose, dependencies, trigger policy, and user-visible
  output;
- the length or digest of every non-empty `Instruction`, and the exact v2
  model-bound input that will carry it;
- default model and `AllowModelOverride` policy;
- every initial-input and step-output binding, including its text or Artifact
  transport;
- static parameters and failure/partial-completion behavior.

Apply these migration rules:

1. A non-empty v1 `Instruction` must not disappear. For a Capability Profile,
   normally bind it to `systemInstruction`. `workbook.instructions` is not sent
   to the model.
2. Preserve fixed model selection. If v1 has `AllowModelOverride=false`, use
   `modelSelection.source=fixed`; do not add a model selector Template Input.
3. Map a v1 image/file/media reference to a compatible Artifact port. Never
   append an Artifact input to `prompt` or `systemInstruction` as a string.
4. Preserve ordered text composition and author precedence. Use `composeValue`
   or `merge` only where the current binding contract supports it.
5. After `check`, compare the candidate with the semantic ledger. A missing or
   changed item requires explicit human review and must be reported as
   `semantic_review_required` before any version creation.

The creation confirmation must describe both the semantic diff and pointer
impact. The current LoomLoom `create-version` behavior makes the new version
both latest and published; read back `latestVersionId`, `publishedVersionId`,
and the full version list after creation.

### Legacy v1 migration routing

Before translating a v1 Step, resolve its business input and output modalities
with `loomloom capability resolve`. Use `loomloom model types --output json`
only when diagnosing the lower-level execution-unit inventory.

| v1 Step shape | v2 route |
| --- | --- |
| text input → text output | returned `capabilityProfile` match |
| image/video/audio/3D generation or editing | returned `fixedModelContract` match |
| image/video/audio input → text output | returned vision/multimodal Profile or Fixed Contract match |

Do not look for an image-generation Capability Profile. Media generation uses
the exact target model's Fixed Model Contract. If the original v1 model has no
target contract, report `missing_fixed_contract`; do not invent a
`subjectRevisionId`, silently switch models, or call `create-version`.

## Creation And Versioning

Before creation:

1. Generate the spec only after TemplatePlan confirmation.
2. Check it locally.
3. Explain the template name, purpose, and check result in business language.
4. For a v1 migration, show the semantic preservation ledger and state that
   the current Server will advance latest and published pointers.
5. Ask for explicit creation confirmation.
6. Run `loomloom template-spec create <spec.json>` only after confirmation.

Configuration, environment variables, tokens, "create a template", and "generate spec" do not constitute remote creation confirmation.

For changes to an existing template, do not promise in-place mutation of historical versions. Use:

```bash
loomloom template-spec create-version <template-id> <spec.json>
```

Later workbooks and runs must use the returned new `version_id`.
