# Conversational Template Authoring

This document defines how an agent should help users create LoomLoom custom templates through conversation. The goal is that users describe business intent, and the agent fills in template design details before generating a valid TemplateSpec.

---

## Core Principle

Conversational template authoring is not about teaching users TemplateSpec. Users only need to answer business questions:

- What template they want to create.
- What users must input each time they use the template.
- How many steps the AI should use.
- Which steps can run in parallel.
- Whether results need to be summarized.
- Whether intermediate results should be shown.
- Whether the workflow should continue after failures.

The agent turns those answers into a template design draft, then converts that design into TemplateSpec.

---

## Fixed Flow

```text
Natural-language user request
  -> agent asks a few questions
  -> TemplatePlan draft
  -> user confirmation
  -> TemplateSpec JSON
  -> loomloom template-spec check
  -> creation confirmation
  -> loomloom template-spec create
```

Do not create a template before the user confirms the design draft.

Note: "Create a template for me" or "Create a PRD review template" only starts the creation flow. It is not confirmation of the design draft and not authorization to create a remote template.

---

## Three Confirmation Gates

Conversational authoring must pass three gates:

| Gate | Allowed | Forbidden |
|---|---|---|
| Requirements collection | Ask questions, organize the business workflow, and produce a TemplatePlan. | Generate final TemplateSpec or call `template-spec create`. |
| Design confirmation | After the user explicitly confirms the TemplatePlan, generate TemplateSpec and run `template-spec check`. | Directly create the template. |
| Creation confirmation | After `check` passes, proceed only when the user explicitly replies with "confirm create template" or equivalent. | Submit a batch run. |

These are not creation confirmation:

- "Create a template for me."
- "Use the default."
- "Continue."
- Providing only `LOOMLOOM_SERVER` / `LOOMLOOM_TOKEN`.
- Saying only "generate the spec."

Before `template-spec create`, the agent must show:

- Template name.
- TemplateSpec file path.
- `template-spec check` result.
- The creation command to be executed.
- A clear prompt asking the user to reply `confirm create template`.

---

## Conversation Rules

The agent must ask like a template consultant, not like a configuration form.

Rules:

1. Ask one question at a time.
2. Do not ask with technical terms.
3. Offer a default when the user is unsure.
4. When the user describes a complex flow, restate the flow first, then continue asking.
5. Do not ask the user to write prompts. Ask them to describe each step's goal.
6. Output a TemplatePlan draft first for user confirmation.
7. Generate TemplateSpec JSON only after the user confirms the TemplatePlan.
8. Ask for separate creation confirmation after TemplateSpec check passes.

Prefer questions with options and include a "use the default" choice.

Do not ask:

```text
Should I configure fieldBindings?
Should I declare upstreamBindings?
Should this use fan-in?
Does it need outputSchema?
How should error columns be bound?
```

Ask instead:

```text
Should these steps run one after another, or can they run at the same time?
Do these results need a final summary?
Should users see intermediate results?
If one step fails, should later steps continue?
```

---

## Recommended Question Order

When information is missing, ask in this order. Ask only one question each time.

1. What kind of template do you want to create?
2. What information should users provide each time they use the template?
3. How should the AI process that information?
4. What steps, roles, or perspectives are involved?
5. Should intermediate results be shown?
6. What final outputs do you want?
7. What should happen if one step fails?
8. Any special requirements?

If the user already provided enough information, do not mechanically ask every question. Draft the design directly.

Example:

```text
How should the AI process this content?
1. Single-step processing
2. Continuous multi-step processing
3. Process from multiple perspectives and summarize
4. Generate multiple versions
5. Use the default: infer from my description
```

---

## TemplatePlan Draft

TemplatePlan is an intermediate design for user confirmation. It is not backend API source data.

Suggested shape:

```json
{
  "templateName": "",
  "goal": "",
  "rowMeaning": "",
  "inputFields": [
    {
      "key": "",
      "label": "",
      "type": "string",
      "required": true,
      "description": ""
    }
  ],
  "steps": [
    {
      "id": "",
      "name": "",
      "executionUnit": "text-generate",
      "goal": "",
      "input": "",
      "output": "",
      "showIntermediateResult": true
    }
  ],
  "relations": [
    {
      "from": ["step_a", "step_b"],
      "to": "step_summary",
      "meaning": "Summarize multiple upstream results"
    }
  ],
  "outputs": [
    {
      "label": "",
      "sourceStep": "",
      "includeErrorColumn": true
    }
  ],
  "failurePolicy": "Default: record the error reason when a single step fails; steps that can continue still run, and final results explain missing items.",
  "specialRequirements": []
}
```

When showing the plan to users, a natural-language table is fine. The JSON shape can be used internally to check completeness before generating TemplateSpec.

---

## Automatic Completion Rules

When the user does not specify details, apply these defaults:

| Item | Default Rule |
|---|---|
| What one row means | One row is one independent batch task. |
| Input fields | Use business names from the user's description. Use stable snake_case keys. |
| Step prompt | Generate fixed `instruction` from the step goal. Do not ask users to write a full prompt. |
| Output columns | Every user-visible step gets a result column. |
| Error columns | Every user-visible step gets an error reason column. |
| Step connections | Derive serial, parallel, and summary relationships from the business flow. |
| Input merge method | Multi-upstream summary uses the target port merge policy, such as `concat_text` for text summary. |
| allow_partial explanation | If partial results may continue, set `triggerPolicy=allow_partial` on the aggregation step. Do not rely on a natural-language prompt to perform platform trigger behavior. |
| System error display | Keep error reason columns for user-visible steps in Excel. |
| Business stop condition | If a step can decide to stop the business flow, final output should show the stop reason and completed results. |
| Intermediate results | Show by default for multi-step review, decomposition, or generation templates. Hide pure internal cleanup steps by default. |
| Failure policy | Use `require_all` by default. Use `allow_partial` only when the user accepts continuing with partial results. Use `fail_fast` when any key step failure should stop the path. |
| Model | First run `loomloom template-spec models <execution-unit>` and choose an available default model. |
| Routing parameters | Do not expose `provider` or `mode`. |

Users do not need to ask for error reason columns. Whenever a step result is visible to users, the agent should plan a corresponding error reason column.

---

## Business Pattern Mapping

### Single-Step Processing

User says:

```text
Input a product description and generate one marketing copy version.
```

Mapping:

```text
1 text-generate step
user input field -> step.prompt
```

### Serial Multi-Step Processing

User says:

```text
First organize the requirements, then generate the formal document.
```

Mapping:

```text
stp_extract -> stp_write
downstream step receives upstream output through upstreamBindings
```

### Multi-Perspective Parallel Processing Then Summary

User says:

```text
Review from product, engineering, and risk perspectives, then summarize.
```

Mapping:

```text
stp_product_review     \
stp_engineering_review  -> stp_summary
stp_risk_review        /
```

Implementation rules:

- The three review steps are sibling steps.
- Each review step has its own input binding.
- They may bind the same input field or different input fields.
- The summary step declares all three upstream steps in `dependsOn`.
- The summary step uses multiple `upstreamBindings` to receive the three upstream `output` ports.
- Do not model this scenario as one internal `expanded` step.

TemplatePlan should explicitly show:

```text
Step 1: Product review
Step 2: Operations review
Step 3: Engineering review
Step 4: Result summary

Output columns:
Product review result / Product review error reason
Operations review result / Operations review error reason
Engineering review result / Engineering review error reason
Result summary / Result summary error reason
```

### Generate Multiple Versions

User says:

```text
Generate copy in three different styles from one topic.
```

Mapping:

- If the user wants three fixed styles, prefer three sibling steps with different instructions.
- If the user wants dynamic expansion from a multi-value field, use `multiValue=true` + `bindMode=expanded`.

Multi-perspective structured workflows should prefer multiple sibling steps. Use `expanded` only for dynamic counts.

---

## Pre-TemplateSpec Checklist

Before generating TemplateSpec, confirm at least:

1. The TemplatePlan was shown to the user.
2. The user confirmed the design draft.
3. Every step has a clear goal.
4. Every root step has user input or a static instruction.
5. Every downstream step has a clear input source.
6. Multi-upstream summary uses step-level `dependsOn` + `upstreamBindings`.
7. `provider` and `mode` are not exposed.
8. Models have been looked up with `loomloom template-spec models` when needed.

---

## CLI Validation Loop

After generating the spec, run:

```bash
loomloom template-spec check ./spec.json
```

If it fails, fix the spec from the error. Do not create the template directly.

After it passes, do not immediately create the template. Show the user the template and check result, then wait for explicit confirmation:

```text
Reply "confirm create template" and I will run template-spec create.
```

After the user confirms, run:

```bash
loomloom template-spec create ./spec.json --version-note "initial version"
```

When fixing an existing user template, append a new version instead of modifying historical versions:

```bash
loomloom template-spec create-version <template-id> ./spec.json --version-note "fix version"
```

After creation succeeds, download the workbook for validation:

```bash
loomloom template-spec download-workbook <template-id> <version-id> --output-file ./input.xlsx
```

Before submitting a real batch run, request a second explicit confirmation again.

---

## Currently Unsupported

The first conversational authoring version only covers workflows expressible by current TemplateSpec.

Do not promise:

- Loops.
- Conditional branches.
- Dynamic routing.
- Long-term memory.
- Human intervention during a run.
- Arbitrary tool-calling agents.
- Arbitrary custom execution units.

If the user asks for these, explain that the current version does not support them and offer a usable approximation.
