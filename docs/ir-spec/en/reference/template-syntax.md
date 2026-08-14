# TemplateSpec v2 syntax

Public fields use lowerCamel. New version writes declare `specVersion=template-spec/v2` and place this object in `canonicalSpecV2`.

| Field | Required | Meaning |
| --- | --- | --- |
| `meta` | yes | Name and display metadata |
| `templateInputs` | no | Map keyed by stable input identity |
| `steps` | yes | Non-empty workflow DAG |
| `workbook` | yes | Instructions and sample rows |

Template Input keys and port IDs match `^[a-z][A-Za-z0-9_]{0,63}$`. Step IDs match `stp_` plus 6-10 lowercase letters or digits. A Step input identity is `<stepId>.<portId>`.

Execution binding kinds are `fixedModelContract` with `subjectRevisionId`, or `capabilityProfile` with `profileId` and `profileRevision`. Profile Steps also require `modelSelection`.

Input binding sources are templateInput, stepOutput, literal, platformContext, composeValue, sequence, and merge. One target port has one binding. Unknown fields fail strict decoding.
