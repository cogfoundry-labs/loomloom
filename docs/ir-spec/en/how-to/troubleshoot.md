# Troubleshoot

Identify the first failing layer: JSON, Schema, local check, server creation, workbook input, or runtime. Common causes include asset IDs bound as prompt strings, unknown field/step references, duplicate targets, unavailable models, stale workbooks, and incompatible port MIME. Fix contract errors before retrying runtime.

| Symptom | First check |
| --- | --- |
| Model sees `ia_*` | field type and initial_input binding |
| Unknown field/step | exact case-sensitive key or ID |
| Duplicate target | competing FieldBinding/ParamBinding |
| Model unavailable | target environment model catalog |
| Workbook rejected | version, required values, enum, MIME |
| Downstream incompatible | source output and target accepts |

Do not repeatedly submit or run while a deterministic contract check still fails.
