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
