# TemplateSpec Specification

## 1. Context

TemplateSpec is the LoomLoom template system's canonical specification format. It declaratively defines a reusable workflow template.

Three specification-level facts:

**TemplateSpec is the only source of truth.** Template versions store TemplateSpec snapshots, and all runtime behavior is derived from those snapshots.

**Workbook files are derived artifacts.** A workbook is compiled from TemplateSpec. It is not source data. When a template version changes, the workbook must be regenerated; old workbooks must not be reused.

**OpenAPI does not define TemplateSpec semantics.** OpenAPI describes HTTP paths and request/response structures. TemplateSpec rules, semantics, and capability boundaries are defined by this document.

---

## 2. TemplateSpec Pipeline

```text
TemplateSpec
  -> compile
WorkflowDefinition + Workbook
  -> user fills Workbook
Workbook Submission
  ->
Run -> Executions -> Artifacts
```

---

## 3. Terminology

| Term | Definition |
|---|---|
| **Step** | A processing unit in a template. It runs one ExecutionUnit capability. |
| **ExecutionUnit** | The capability type used by a step. Current built-ins are `text-generate`, `image-generate`, and `video-generate`. |
| **InputSchema** | The set of fields the user must fill. |
| **Field** | One input field in InputSchema, with type, visibility, and multi-value properties. |
| **FieldBinding** | A direct mapping from one Field to one parameter on one step. |
| **ParamBinding** | A binding that composes multiple sources, such as fields and literals, into one parameter on one step. |
| **UpstreamBinding** | A connection from an upstream step output port to a downstream step input port. |
| **Workbook** | The user-fillable interface compiled from TemplateSpec. |
| **Execution** | One actual runtime instance of a step. A fan-out step may produce multiple executions. |
| **Artifact** | An output produced by one execution. |

---

## 4. Top-Level Structure

TemplateSpec is a JSON object with these top-level fields:

| Field | Type | Required | Description |
|---|---|---|---|
| `meta` | object | Yes | Template metadata. |
| `steps` | array | Yes, non-empty | Execution unit chain. |
| `inputSchema` | object | Yes | User input field definitions. |
| `fieldBindings` | array | Optional | Direct field-to-step parameter bindings. Do not use this to bind `text_reference` directly to `prompt`. |
| `paramBindings` | array | Optional | Multi-source bindings that compose values into step parameters. |

---

## 5. Meta

`meta` describes template metadata.

| Field | Type | Required | Description |
|---|---|---|---|
| `name` | string | Yes | Non-empty template name. |
| `description` | string | Optional | Template description. |
| `scenario` | string | Optional | Scenario category. |
| `inputSummary` | string | Optional | Input summary for UI display. |
| `displayOutputType` | string | Optional | Human-readable output type. |
| `primaryOutputType` | string | Optional | If set, it must match the output type inferred from the terminal step. |
| `tags` | array<string> | Optional | Tags for grouping and search. |

---

## 6. Steps

`steps` is an ordered array. Each item is a StepSpec object.

### 6.1 StepSpec

| Field | Type | Required | Description |
|---|---|---|---|
| `stepId` | string | Yes | Format: `stp_` plus 6-10 letters or digits, such as `stp_ab12cd`. Must be unique within the spec. |
| `displayName` | string | Yes | Display name. |
| `executionUnit` | string | Yes | Capability type. Must be registered in the capability registry. |
| `defaultModelRef` | object | Conditional | Required when the execution unit needs a model. Must include `modelKey`. |
| `instruction` | string | Optional | Template-author system instruction. It is fixed and cannot be overridden by users. |
| `dependsOn` | array<string> | Optional | Upstream step IDs. Multiple upstream steps express step-level fan-in. |
| `upstreamBindings` | array | Optional | Connections from upstream step outputs to this step's input ports. See 6.2. |
| `triggerPolicy` | string | Optional | Fan-in trigger policy. See 6.3. Empty means `require_all`. |
| `allowModelOverride` | boolean | Optional | Whether users may override this step's model. Defaults to `false`. |
| `staticParams` | object | Optional | Fixed runtime parameters. Keys must be allowed by the execution unit. |

### 6.2 UpstreamBinding

| Field | Type | Required | Description |
|---|---|---|---|
| `inputPort` | string | Yes | Input port name on this step. |
| `sourceType` | string | Optional | `step_output` by default, or `initial_input`. |
| `sourceStepId` | string | Conditional | Required when `sourceType=step_output`. Must appear in `dependsOn`. |
| `sourcePort` | string | Conditional | Required when `sourceType=step_output`. Must be an actual output port on the upstream step. |
| `sourceInputKey` | string | Conditional | Required when `sourceType=initial_input`. Selects the initial input field to read. |

### 6.3 triggerPolicy

`triggerPolicy` controls when a step with upstream dependencies runs. It is mainly used by multi-upstream aggregation steps.

| Value | Semantics | Use Case |
|---|---|---|
| `require_all` | Default. Run only after all upstream steps succeed. If any upstream step fails, is cancelled, or is blocked, this step does not run. | Strict workflows that require complete inputs. |
| `allow_partial` | Wait until all upstream steps reach terminal states, then run if at least one upstream step succeeded. The downstream step receives only successful upstream artifacts. | Summaries that can continue with partial results. |
| `fail_fast` | If any upstream step fails, is cancelled, or is blocked, this step does not run and downstream paths are blocked. | Workflows where any critical input failure makes later work useless. |

Constraints:

- Missing `triggerPolicy` is equivalent to `require_all`.
- `allow_partial` cannot be used on a root step. It requires at least one upstream dependency.
- Multi-upstream fan-in with `allow_partial` must declare explicit `upstreamBindings`.
- `allow_partial` does not pass failed upstream system error text into business input. Failure reasons remain available in step status, error columns, or result views.
- If all upstream steps fail, are cancelled, or are blocked under `allow_partial`, the downstream step does not run.

---

## 7. Input Schema

`inputSchema` defines the fields users must fill.

| Field | Type | Description |
|---|---|---|
| `fields` | array | Input fields. Required and non-empty. |
| `instructions` | array<string> | Filling instructions shown above the form. |
| `sampleRows` | array<object> | Sample data. Each row must be shaped as `{ "values": { "<field_key>": "<sample value>" } }`. Keys inside `values` must use field `key`, not `label`. |

Example:

```json
{
  "sampleRows": [
    {
      "values": {
        "prompt": "Write a short launch announcement"
      }
    }
  ]
}
```

### 7.1 TemplateInputField

| Field | Type | Required | Description |
|---|---|---|---|
| `key` | string | Yes | Unique field identifier. Reserved keys such as `model`, `provider`, and `mode` cannot be used. Must be unique within the spec. |
| `label` | string | Yes | Display label. Must be unique within the spec. |
| `valueType` | string | Yes | Value type. See 7.2. |
| `required` | boolean | Optional | Whether the field is required. Defaults to `false`. |
| `defaultValue` | string | Conditional | Required and non-empty when `sourceKind` is not `user_input`. |
| `sourceKind` | string | Optional | Controls workbook visibility. See 7.3. |
| `multiValue` | boolean | Optional | Whether the field accepts multiple values. Defaults to `false`. |
| `maxValues` | integer | Conditional | Required and greater than 0 when `multiValue=true`. |
| `enumValues` | array<string> | Conditional | Required and non-empty when `valueType=enum`. |
| `acceptedMimeTypes` | array<string> | Conditional | Required and non-empty when `valueType=asset_ref` or `text_reference`. |
| `order` | integer | Optional | Display order in the workbook. |
| `description` | string | Optional | Field description. |

If `multiValue=true`, the workbook still uses one cell. Users enter multiple values in that cell, separated by semicolons (`;`) or newlines.

### 7.2 valueType

| Value | Meaning | Extra Constraints |
|---|---|---|
| `string` | Plain text. | None. |
| `enum` | Enum value. | `enumValues` must be non-empty. |
| `image_url` | Image URL. | Must be a valid HTTP/HTTPS URL. |
| `asset_ref` | Uploaded file asset ID. | Must be an `ia_` UUID. `acceptedMimeTypes` must be non-empty. |
| `text_reference` | Inline text or asset ref. | Supports inline text and asset refs, not URLs. `acceptedMimeTypes` must be non-empty. If users may fill `ia_xxx`, bind it to a text input port with `upstreamBindings` and `sourceType=initial_input`. |

`text_reference` must not be bound directly to `prompt` only through `fieldBindings`. Otherwise, when the user enters `ia_xxx`, the model receives the asset ID string rather than the file content. To let the model read uploaded text, use:

```json
{
  "inputPort": "reference",
  "sourceType": "initial_input",
  "sourceInputKey": "reference_text"
}
```

### 7.3 sourceKind

| Value | Workbook Behavior | Extra Constraints |
|---|---|---|
| `user_input` (default) | Visible and editable. | None. |
| `default_value` | Visible but not editable. Driven by default value. | `defaultValue` must be non-empty. |
| `hidden` | Hidden from users. | `defaultValue` must be non-empty. |

`hidden=true` is equivalent to `sourceKind=hidden`.

---

## 8. Bindings

### 8.1 FieldBinding

Maps one Field directly to one parameter on one step.

| Field | Type | Required | Description |
|---|---|---|---|
| `fieldKey` | string | Yes | References `inputSchema.fields[].key`. |
| `stepId` | string | Yes | Target step ID. |
| `paramKey` | string | Yes | Target parameter name. |
| `bindMode` | string | Yes | `shared` or `expanded`. Must match the field's `multiValue`. |

Fields with `multiValue=false` must use `bindMode=shared`. Fields with `multiValue=true` must use `bindMode=expanded`.

Special routing parameter rules: `paramKey=model` requires the target step to declare `allowModelOverride=true`. `paramKey=provider` and `paramKey=mode` must not be exposed through bindings.

### 8.2 ParamBinding

Composes multiple sources and maps the result to one parameter on one step.

| Field | Type | Required | Description |
|---|---|---|---|
| `stepId` | string | Yes | Target step ID. |
| `paramKey` | string | Yes | Target parameter name. Cannot be a routing parameter (`model`, `provider`, or `mode`). |
| `bindMode` | string | Yes | `shared` or `expanded`. |
| `separator` | string | Optional | Separator used when joining sources. |
| `sources` | array | Yes, non-empty | Source list. See ParamSource below. |

**ParamSource:**

| Field | Type | Description |
|---|---|---|
| `kind` | string | `field_ref` or `literal`. |
| `fieldKey` | string | Field key referenced when `kind=field_ref`. |
| `literal` | string | Fixed string when `kind=literal`. Must be non-empty. |

A ParamBinding may contain at most one `multiValue=true` `field_ref` source. If such a source exists, `bindMode` must be `expanded`.

A ParamBinding may contain at most three regular visible field sources. For example, "Body", "Style requirements", and "Output format" may be combined into the same `prompt` parameter. Multi-field composition should be expressed through `ParamBinding.sources`; do not create multiple FieldBindings that write repeatedly to the same `stepId + paramKey`.

The current productized scope is composing text fields into the `prompt` parameter for generation executors. Image, video, and file material inputs should still enter through UpstreamBinding / input ports and remain constrained by model capabilities. Do not treat multiple image input count as a ParamBinding capability.

### 8.3 Binding Target And Multi-Upstream Inputs

Within one step, each `paramKey` written by FieldBinding / ParamBinding must still be unique.

FieldBinding / ParamBinding use `paramKey` to name the target parameter. UpstreamBinding uses `inputPort` to name the target input port. In v1, both belong to the same target namespace.

Therefore, this structure is invalid:

```text
FieldBinding    -> stp_summary.prompt
UpstreamBinding -> stp_summary.prompt (also bound)
```

In other words, the same target on the same step cannot simultaneously come from a user field and an upstream artifact.

UpstreamBinding supports multiple upstream sources writing to the same `inputPort`, but the target input port must declare `allowMultiple=true`, and every upstream output type must be accepted by the target input port's `accepts`.

For example, multiple `text-generate.output` artifacts (`text/plain`) may bind to downstream `text-generate.prompt` (`accepts=text/*`, `allowMultiple=true`).

---

## 9. Execution Semantics

### 9.1 shared

The field value is shared by all executions of the step. Use this when all parallel executions should use the same value.

### 9.2 expanded And Fan-Out

`bindMode=expanded` expands the multiple values of a `multiValue` field into multiple independent executions of the same step. Each value corresponds to one execution. This is called **fan-out**.

```text
Input: ["value1", "value2", "value3"] (bindMode=expanded)
  -> execution 1: value1
  -> execution 2: value2
  -> execution 3: value3
```

### 9.3 Step-Level Fan-In

When a downstream step connects to multiple upstream steps through multiple UpstreamBindings, the system merges those upstream outputs into one downstream input port. This is valid only when the target input port allows multiple inputs.

```text
stp_product.output      \
stp_engineering.output   -> stp_summary.prompt
stp_risk.output         /
```

Conditions:

- `dependsOn` must declare all upstream steps.
- Every UpstreamBinding `sourceStepId` must appear in `dependsOn`.
- `sourcePort` must exist.
- Target `inputPort` must exist.
- `sourcePort.type` must match target `inputPort.accepts`.
- When multiple bindings write to one `inputPort`, that port must have `allowMultiple=true`.

---

## 10. Capability Registry

### `text-generate`

| Property | Value |
|---|---|
| Input ports | `prompt` (`text/*`, required, AllowMultiple=true); `reference` (`text/*`, optional). |
| Output port | `output` (`text/plain`). |
| Model override | Supported. Requires `allowModelOverride=true`. |

### `image-generate`

| Property | Value |
|---|---|
| Input ports | `prompt` (`text/*`, required, AllowMultiple=true). |
| Output port | `output` (`image/png`). |
| Model override | Supported. Requires `allowModelOverride=true`. |

### `video-generate`

| Property | Value |
|---|---|
| Input ports | `prompt` (`text/*`, required, AllowMultiple=false); `image` (`image/*`, optional, AllowMultiple=false). |
| Output port | `output` (`video/mp4`). |
| Model override | Not supported. |

---

## 11. Unsupported Patterns

### Type-Incompatible Step Dependencies

A step may declare multiple upstream steps, but dependencies must establish valid data connections. This structure is invalid:

```text
image-generate.output (image/png) -> text-generate.prompt (accepts only text/*)
```

Reason: source output type does not match target input accepts.

### Multiple Sources For A Single-Input Port

This structure is also invalid:

```text
stp_a.output \
stp_b.output  -> video-generate.prompt
```

Reason: `video-generate.prompt` accepts only one input source, because `allowMultiple=false`.

---

## 12. Versioning

This document describes **TemplateSpec v1** (internal identifier `template-spec/v1`).

Main v1 boundaries:

- Supports step-level fan-in / fan-out.
- Supports fan-in aggregation trigger policies: `require_all`, `allow_partial`, and `fail_fast`.
- FieldBinding / ParamBinding targets remain unique.
- Multiple UpstreamBindings may write to one inputPort only when the target port explicitly allows it through `allowMultiple` and `accepts`.
- Merge semantics are determined by the target input port's `mergePolicy`.
