# TemplateSpec v2 syntax

Public fields use lowerCamel. New version writes declare `specVersion=template-spec/v2` and place this object in `canonicalSpecV2`.

| Field | Required | Meaning |
| --- | --- | --- |
| `meta` | yes | Name and display metadata |
| `templateInputs` | no | Map keyed by stable input identity |
| `steps` | yes | Non-empty workflow DAG |
| `workbook` | yes | Instructions and sample rows |

Template Input keys and port IDs match `^[a-z][A-Za-z0-9_]{0,63}$`. Step IDs match `stp_` plus 6-10 lowercase letters or digits. A Step input identity is `<stepId>.<portId>`.

JSON map order has no semantic meaning; Workbook column order uses `presentation.order`.

## Step

```json
{
  "stepId": "stp_image01",
  "displayName": "Generate image",
  "dependsOn": ["stp_text01"],
  "triggerPolicy": "require_all",
  "executionBinding": {
    "kind": "fixedModelContract",
    "subjectRevisionId": "subject-revision-id"
  },
  "inputBindings": {
    "prompt": {"source": "stepOutput", "stepId": "stp_text01", "portId": "output"}
  }
}
```

Execution binding kinds are `fixedModelContract` with `subjectRevisionId`, or `capabilityProfile` with a stable `profileId`. `profileRevision` is optional and only used to request a specific historical contract; normal authoring omits it and Core freezes the current revision. Run `loomloom template-spec authoring-context --output json` against the target environment before choosing a Profile or model. Profile Steps also require `modelSelection`.

## inputBindings

The map key is the target contract input port. Sources are:

- `templateInput`
- `stepOutput`
- `literal`
- `platformContext`
- `composeValue`
- `sequence`
- `merge`

One target port permits one binding. Express multiple sources within one `merge` or `sequence` source, not by repeating a port.

## Unknown fields

Public objects use strict decoding. Misspelled fields fail rather than being silently ignored. See the exact [JSON Schema](../../machine/template-spec.schema.json).
