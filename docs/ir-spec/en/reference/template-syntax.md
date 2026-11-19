# TemplateSpec v2 syntax

Public fields use lowerCamel. New version writes declare `specVersion=template-spec/v2` and place this object in `canonicalSpecV2`.

| Field | Required | Meaning |
| --- | --- | --- |
| `meta` | yes | Name and display metadata |
| `templateInputs` | no | Map keyed by stable input identity |
| `steps` | yes | Non-empty workflow DAG |
| `workbook` | yes | Instructions and sample rows |
| `deliveryOutputs` | no | Explicit final delivery ports from root Steps |

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

Execution binding kinds are `fixedModelContract` with `subjectRevisionId`, or
`capabilityProfile` with a stable `profileId`. Dynamic Profiles do not accept
`profileRevision`; omit it for normal authoring. First use `loomloom capability
resolve` to select a current match by business inputs and output. Use
`loomloom template-spec authoring-context --output json` when you need the full
Profile inventory. Profile Steps also require `modelSelection`.

A Capability Profile's `definition` is its fixed interface, while
`eligibleModels` is calculated dynamically from current model capability facts.
Profiles may describe standard text generation, image understanding, image
generation, or video generation. Obtain the exact ID, ports, current default,
and eligible models from the target environment. Bind image-understanding
inputs and generated image or video outputs as Artifacts using the returned
ports and MIME/cardinality constraints. Do not infer capability from a model
name or Provider endpoint.

When `capability resolve` returns a Profile and the workflow needs a replaceable
model set, use that Profile. Use a returned `fixedModelContract` when the
workflow instead requires one exact model or its dedicated interface. Do not
ignore a dynamic Profile for a standard capability merely because some models
also expose fixed contracts.

## inputBindings

The map key is the target contract input port. Sources are:

- `templateInput`
- `stepOutput`
- `literal`
- `platformContext`
- `composeValue`
- `sequence`
- `merge`
- `loopState`: only within a Loop body, referencing declared string state.

One target port permits one binding. Express multiple sources within one `merge` or `sequence` source, not by repeating a port.

## Unknown fields

Public objects use strict decoding. Misspelled fields fail rather than being silently ignored. See the exact [JSON Schema](../../machine/template-spec.schema.json).

<a id="ref-delivery-outputs-declaration"></a>
## Final delivery declaration

`deliveryOutputs` is optional. Set `schemaVersion: 1` and an ordered `items`
list; each item contains a stable `key`, a user-visible `label`, a root
`stepId`, and its `portId`. See [Code final delivery](../examples/valid/delivery-code-outputs.json).
A declaration selects the complete output port, including every file when it
produces multiple files; it cannot select just one file. Code files, Code
structured values, model outputs, and public Loop container export aliases are
supported. Loop body Steps, diagnostics, and input attachments cannot be selected.

Declare 1–128 items, with unique keys and source ports. Labels contain 1–128
characters without control characters. Saving validates ports against the exact
contract and freezes the mapping between logical ports and stored outputs.
The declaration contributes to the version hash and the Run's frozen definition.
Changing it requires a new version; existing Runs never read the latest template
for their delivery selection.

Omitting the field preserves historical semantics and the original hash: final
delivery is undefined, rather than inferred from the last Step or first file.
Historical Runs still expose all produced artifacts; the results scope reports
that no final delivery is defined. `workbook` contains filling instructions and
sample rows, not a delivery declaration. The existing `resultWorkbook` endpoint
continues to return its original workbook.
