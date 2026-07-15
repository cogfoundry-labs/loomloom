# TemplateSpec syntax

Public JSON uses lowerCamel fields. Top-level `meta`, non-empty `steps`, and `inputSchema` are required; fieldBindings and paramBindings are optional. Unknown object properties are rejected by Schema. Field and step references are case-sensitive. See the machine JSON Schema for exact types.

| Top-level field | Required | Purpose |
| --- | --- | --- |
| `meta` | yes | identity and display metadata |
| `steps` | yes | one or more processing steps |
| `inputSchema` | yes | public row fields and guidance |
| `fieldBindings` | no | direct field-to-parameter mapping |
| `paramBindings` | no | composed parameter mapping |

Step IDs match `stp_<6-10 base36>`. Field keys model/provider/mode are reserved. Omitted sourceKind behaves as user_input, omitted triggerPolicy as require_all, and omitted upstream sourceType as step_output.

## Field quick reference

This page lists the minimum fields needed to author a Spec independently. Follow the linked references for conditional validation and port-compatibility rules.

| Object | Required fields | Common optional fields | Key rule |
| --- | --- | --- | --- |
| `meta` | `name` | `description`, `scenario`, `inputSummary`, `displayOutputType`, `primaryOutputType`, `tags` | Describes the template only; a declared `primaryOutputType` must match the terminal-step result |
| `inputSchema` | `fields` | `instructions`, `sampleRows` | At least one field; sample rows use `{ "values": { "field_key": "value" } }` |
| `inputSchema.fields[]` | `key`, `label`, `valueType` | `description`, `required`, `enumValues`, `acceptedMimeTypes`, `multiValue`, `maxValues`, `order`, `defaultValue`, `hidden`, `sourceKind`, `presentation` | `valueType` is `string`, `enum`, `image_url`, `asset_ref`, or `text_reference`; enum needs `enumValues`, asset/text reference needs `acceptedMimeTypes` |
| `steps[]` | `stepId`, `displayName`, `executionUnit` | `instruction`, `dependsOn`, `upstreamBindings`, `triggerPolicy`, `defaultModelRef`, `allowModelOverride`, `staticParams` | IDs are globally unique and dependencies are acyclic; model steps normally need a catalog `defaultModelRef.modelKey` |
| `fieldBindings[]` | `fieldKey`, `stepId`, `paramKey`, `bindMode` | — | Single-value fields use `shared`; multi-value fields use `expanded`; one binding per `stepId + paramKey` |
| `paramBindings[]` | `stepId`, `paramKey`, `bindMode`, `sources` | `separator` | Combines `field_ref` and `literal` sources; use `expanded` with a multi-value source |
| `steps[].upstreamBindings[]` | `inputPort`, source fields | `sourceType` | `step_output` uses `sourceStepId` and `sourcePort`; `initial_input` uses `sourceInputKey` |

Use the lowerCamel names shown here: an input field type is `valueType`, not `type`. Omitted `sourceKind` means `user_input`; omitted `triggerPolicy` means `require_all`; omitted upstream `sourceType` means `step_output`.

## Continue in the CLI

`loomloom template-spec docs spec` is the authoring entry point. For complete conditions or port information for one object, run:

```text
loomloom template-spec docs metadata
loomloom template-spec docs inputs
loomloom template-spec docs steps
loomloom template-spec docs bindings
loomloom template-spec docs execution-units
loomloom template-spec docs examples
```

The JSON files listed by `examples` are canonical examples shipped in the CLI documentation bundle; they do not require access to the Core source checkout. Validate an authored JSON file with `template-spec check <file>`.

## Fixed parallel branch syntax

Fixed branches are expressed by `steps` and binding topology. There is no additional `branch` or `parallel` property. The following structure omits display metadata and model configuration to focus on one input starting two text steps, each connected to its own image step:

```json
{
  "steps": [
    {"stepId": "stp_scenea", "executionUnit": "text-generate"},
    {"stepId": "stp_sceneb", "executionUnit": "text-generate"},
    {
      "stepId": "stp_imagea",
      "executionUnit": "image-generate",
      "dependsOn": ["stp_scenea"],
      "upstreamBindings": [
        {"inputPort": "prompt", "sourceType": "step_output", "sourceStepId": "stp_scenea", "sourcePort": "output"}
      ]
    },
    {
      "stepId": "stp_imageb",
      "executionUnit": "image-generate",
      "dependsOn": ["stp_sceneb"],
      "upstreamBindings": [
        {"inputPort": "prompt", "sourceType": "step_output", "sourceStepId": "stp_sceneb", "sourcePort": "output"}
      ]
    }
  ],
  "fieldBindings": [
    {"fieldKey": "book_title", "stepId": "stp_scenea", "paramKey": "prompt", "bindMode": "shared"},
    {"fieldKey": "book_title", "stepId": "stp_sceneb", "paramKey": "prompt", "bindMode": "shared"}
  ]
}
```

The two text steps have no `dependsOn`, so they become ready in the same scheduling round. Each image step waits for its paired text step. Repeat the same step-and-binding structure for ten fixed branches; do not replace it with `expanded`. See `examples/valid/parallel-text-to-image-branches.json` for a complete valid TemplateSpec.
