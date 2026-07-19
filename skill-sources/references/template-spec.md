# TemplateSpec

Use this reference when creating, modifying, versioning, or explaining a private template or TemplateSpec.

## Contents

- [Documentation sources](#documentation-sources)
- [Conversation flow](#conversation-flow)
- [Usage mode](#usage-mode)
- [TemplatePlan](#templateplan)
- [Modeling and input rules](#modeling-and-input-rules)
- [Creation and versioning](#creation-and-versioning)

## Documentation Sources

Before writing TemplateSpec, read the current CLI-bundled documentation:

```bash
loomloom template-spec docs spec
loomloom template-spec docs examples
loomloom template-spec docs conversation
```

- `spec` is the current JSON contract.
- `examples` contains examples and implementation patterns.
- `conversation` defines natural-language authoring behavior.

TemplateSpec docs default to English and are also available in Chinese. For example:

```bash
loomloom template-spec docs spec --lang zh-CN
```

Select the documentation language as appropriate for the conversation and task.

The installed Skill may contain a `generated-template-spec/` backup. Prefer the CLI docs command because it matches the currently installed CLI.

## Conversation Flow

When a user describes a reusable workflow:

1. Ask business questions, not TemplateSpec-field questions.
2. Ask one missing-detail question at a time.
3. For long-form material, explicitly ask whether future users will paste the content or upload a file; do not infer the transport from words such as "document" or "material".
4. Avoid user-facing terms such as `fieldBindings`, `upstreamBindings`, `fan-in`, `execution`, `outputSchema`, `provider`, and `mode`.
5. Restate complex workflows in business language.
6. Determine whether different roles or steps need different processing requirements.
7. Choose the future usage mode before drafting.
8. Draft a TemplatePlan first.
9. Show the TemplatePlan and wait for confirmation.
10. Generate TemplateSpec only after plan confirmation.
11. Run `loomloom template-spec check <spec.json>`.
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

Do not expose `prompt`, `binding`, `reference`, `field`, `hidden`, `paramBindings`, or `fieldBindings` in this question.

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

- Model the reviews as parallel `text-generate` steps.
- Model the summary as a downstream step.
- Connect steps with `dependsOn` and `upstreamBindings`.
- Do not author `bindMode=expanded`. It is compatibility-only for running historical template versions and is rejected for new templates, versions, and publication flows with `TS-TOPOLOGY-001`.
- `multiValue=true` may still represent an ordered content collection passed through `sourceType=initial_input` to a port that accepts multiple contents; it does not create additional executions.
- For independently processed dynamic items, use one workbook row per item. For a fixed number of parallel branches, declare multiple Steps and connect them with `dependsOn` / `upstreamBindings`. TemplateSpec v1 does not support dynamic-cardinality Step fan-out.

Add result and error columns for every user-visible step by default. If partial completion is allowed, make missing upstream results understandable and expose failed-step error columns.

Keep `provider` and `mode` internal.

## Modeling And Input Rules

### Input transport

Apply the canonical input rules consistently:

- Pasted text: `string -> prompt` (`TS-IN-001`).
- Uploaded text file: `text_reference -> initial_input -> compatible text port` (`TS-IN-002`).
- Reject a design that declares `string`, tells users to enter an asset ID, and binds it to `prompt` (`TS-IN-003`).

Before showing TemplateSpec, verify that field descriptions, sample rows, `valueType`, and bindings all describe the same transport.

### Contract rules

- Use only `text-generate`, `image-generate`, and `video-generate` by default unless an explicitly documented custom unit exists.
- Use lowerCamel fields such as `meta.name`, `steps[].stepId`, and `defaultModelRef.modelKey`.
- Wrap sample-row values in `inputSchema.sampleRows[].values`, for example `{ "values": { "prompt": "example" } }`.
- Connect steps through step-level `dependsOn` and `upstreamBindings`; the source output port is usually `output`.
- For multi-upstream fan-in, follow the current TemplateSpec docs and execution-unit registry, bind only to a port that supports all sources, and run `template-spec check`.
- Do not add registry-only port capability fields to TemplateSpec.
- Before choosing `defaultModelRef.modelKey`, run `loomloom template-spec models <execution-unit>` and use a returned `model_id`.
- Expose a model column only when `allowModelOverride=true` and a field binds to `paramKey=model`.
- Do not bind `provider` or `mode`.
- Never guess step IDs.

## Creation And Versioning

Before creation:

1. Generate the spec only after TemplatePlan confirmation.
2. Check it locally.
3. Explain the template name, purpose, and check result in business language.
4. Ask for explicit creation confirmation.
5. Run `loomloom template-spec create <spec.json>` only after confirmation.

Configuration, environment variables, tokens, "create a template", and "generate spec" do not constitute remote creation confirmation.

For changes to an existing template, do not promise in-place mutation of historical versions. Use:

```bash
loomloom template-spec create-version <template-id> <spec.json>
```

Later workbooks and runs must use the returned new `version_id`.
