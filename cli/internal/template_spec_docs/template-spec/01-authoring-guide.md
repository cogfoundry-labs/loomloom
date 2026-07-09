# Authoring Guide

This guide is for template authors. It explains how to write a valid TemplateSpec and how to diagnose common errors. The full field definitions and constraints are in `00-template-spec.md`.

---

## Order For Writing A Spec

1. Define steps: what steps the template has and which capability each step uses.
2. Define inputSchema: what the user must fill.
3. Choose binding methods: how user input reaches each step.
4. Connect steps, when the workflow has multiple steps: how upstream results flow into downstream steps.
5. Enable model override, if needed.

---

## Step 1: Define Steps

Start from what the template should do. Decide how many steps are needed and which executionUnit each step uses.

**Naming rule:** `stepId` must be `stp_` plus 6-10 letters or digits, such as `stp_ab12cd`. It must be unique within one spec.

**instruction vs defaultValue:** If a step needs a fixed system instruction, such as "Translate the following content into English", put it in `instruction`. Do not pass it through a field. `instruction` is written by the template author, hidden from users, and not user-editable.

**Multiple steps:** Declare `dependsOn` on downstream steps. One step may declare multiple upstream steps to express step-level fan-in. Aggregation steps default to `require_all` when `triggerPolicy` is not set.

---

## Step 2: Define inputSchema

Decide what the user must fill, and choose each field's type and visibility.

### Choosing valueType

| Scenario | Use |
|---|---|
| Plain text input | `string` |
| Fixed options | `enum` with `enumValues` |
| Image URL | `image_url` |
| User-uploaded text file | `text_reference` with `acceptedMimeTypes`, then bind it to a text input port using `upstreamBindings` with `sourceType=initial_input` |
| User-uploaded non-text file | `asset_ref` with `acceptedMimeTypes` |

### Choosing sourceKind

| Scenario | Use |
|---|---|
| User must fill it | `user_input` (default; no need to write explicitly) |
| Has a reasonable default and users should not be forced to edit it | `default_value` (`defaultValue` must be non-empty) |
| Used internally by the template and hidden from users | `hidden` (`defaultValue` must be non-empty) |

### multiValue

When a step needs to process multiple inputs in parallel, set the field to `multiValue=true` and set `maxValues`. That field must later be bound with `bindMode=expanded` to trigger fan-out.

In a workbook, `multiValue=true` still uses one cell. Template users enter multiple values in the same cell, separated by semicolons (`;`) or newlines.

---

## Step 3: Choose Binding Methods

Binding rule: **the same input target on the same step can have only one source**. FieldBinding, ParamBinding, and UpstreamBinding are mutually exclusive for one target.

### FieldBinding vs ParamBinding

**Use FieldBinding** when one field maps directly to one step parameter and no composition is needed.

**Use ParamBinding** when multiple sources must be composed into one parameter, such as adding a fixed prefix before user input, or combining at most 3 regular visible text fields into one `prompt`.

If a model step needs the user to fill "Body", "Style requirements", and "Output format", use one `ParamBinding` to write the composed value into that step's `prompt`. Do not create multiple FieldBindings that repeatedly write to the same `stepId + prompt`.

Image, video, and file material inputs are not text parameter composition. They should still be connected through input ports and remain constrained by concrete model capabilities.

### Choosing bindMode

- `multiValue=false` field -> `bindMode=shared`
- `multiValue=true` field -> `bindMode=expanded`

They must match. A mismatch fails validation during `create`.

---

## Step 4: Connect Steps

Use `upstreamBindings` to connect upstream step output ports to downstream step input ports.

**Important:** UpstreamBinding `inputPort` and FieldBinding/ParamBinding `paramKey` belong to the same target namespace. If an UpstreamBinding already connects `prompt` on this step, you cannot also write to the same `prompt` through FieldBinding or ParamBinding.

Multiple upstream steps may bind to the same `inputPort` only when the target input port allows multiple sources and all upstream output types are accepted by that target input port. For example, multiple `text-generate.output` values may flow into downstream `text-generate.prompt`.

If the downstream step needs extra instructions, write them in `instruction`. Do not pass them through an extra binding.

### Choosing triggerPolicy

When a step has upstream dependencies, use `triggerPolicy` to control when it runs:

| Policy | When To Use | Runtime Behavior |
|---|---|---|
| `require_all` | Default. The summary must be based on all upstream results. | Runs only when all upstream steps succeed. Any upstream failure prevents it from running. |
| `allow_partial` | Some roles or branches may fail, but you still want to summarize successful results. | Waits until all upstream steps finish. Runs if at least one upstream step succeeded. Failed upstream steps are not passed into business input. |
| `fail_fast` | Any key upstream failure makes later results meaningless. | As soon as an upstream dependency fails, this node and downstream dependency paths do not run. |

`allow_partial` is a workflow runtime trigger policy. It is not an instruction for the template author to write natural language such as "if some parts fail, explain it." The system attaches structured input-completeness metadata to downstream steps. Failed upstream system error reasons should still be read from step status, error columns, or result views, not mixed into business text.

---

## Step 5: Enable Model Override

To let users choose a model in the workbook, both conditions must hold:

**Condition 1:** the step declares `allowModelOverride=true`.

**Condition 2:** a fieldBinding binds a field to that step's `paramKey=model`.

If only one condition is met, the model column does not appear in the workbook. `provider` and `mode` are not supported as exposed template parameters.

---

## Validation Layers

When TemplateSpec is submitted, the system validates it in four layers:

| Layer | What It Checks | When It Happens |
|---|---|---|
| Schema validation | Whether JSON structure and field types are valid. | `check` API |
| Semantic validation | Whether field references are valid and valueType constraints are satisfied. | `check` API |
| Authoring validation | Whether workflow shape is within v1 support. | `create` API |
| Runtime validation | Runtime conditions such as model availability. | During actual execution |

**schema valid does not mean authoring valid; authoring valid does not guarantee runtime success.** Passing one layer does not guarantee the next layer will pass.

---

## `check` Passes But `create` Fails

The JSON structure and field references are valid, but the workflow shape violates v1 constraints. Common causes:

| Cause | Description |
|---|---|
| Multi-upstream step lacks explicit binding | Multi-upstream fan-in must explicitly declare how each upstream output enters the downstream input port through `upstreamBindings`. |
| `inputPort` does not exist | The specified input port is not available on that executionUnit. |
| `sourcePort` does not exist | The specified upstream output port does not exist. |
| `sourceStepId` is not in `dependsOn` | The UpstreamBinding references a step that is not declared as a dependency. |
| Binding target conflict | FieldBinding / ParamBinding and UpstreamBinding write to the same target, or multiple upstream steps write to an input port with `allowMultiple=false`. |
| Upstream/downstream type mismatch | Upstream output type is not accepted by the downstream input port. |
| Invalid `triggerPolicy` | Only `require_all`, `allow_partial`, and `fail_fast` are allowed. |
| `allow_partial` used on a root step | `allow_partial` requires at least one upstream dependency. |
| Model binding not enabled | A binding writes `paramKey=model` to a step that does not declare `allowModelOverride=true`. |
| `bindMode` does not match `multiValue` | They must match. |

## `create` Succeeds But Runtime Fails

The template definition is valid, but runtime conditions are not satisfied. Common causes:

- Model ID does not exist, is not enabled, or has been taken offline.
- Provider service error.
- Input content triggers business-side restrictions.

---

## Reading Results

After a run completes, prefer server-side result views rather than local Excel backfill.

| Result View | Applies To | Description |
|---|---|---|
| `result-rows` | All new runs with input snapshots. | Returns input snapshots, statuses, errors, and artifacts for programmatic reading. |
| `result-workbook` | Workbook/Excel-submitted runs. | Downloads a server-generated result Excel that preserves original input columns and appends result columns. |

Artifact URLs are the canonical way to access result artifacts. For `text/*` artifacts, the server tries to return `inlineText` as a lightweight preview. Currently, only text up to 4KB is inlined. Larger text artifacts remain text results, but `result-rows` / `result-workbook` fall back to showing artifact URLs.

CLI commands:

```bash
loomloom run result-rows <run-id>
loomloom run result-workbook <run-id> --output-file ./result.xlsx
```

`template backfill-results` is the older local backfill flow. New templates and internal validation should prefer `run result-workbook`, because it uses the server-saved input snapshot and original workbook, avoiding mismatches between a local file version and run results.
