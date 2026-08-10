# Compose prompts

Use one ParamBinding for a prompt built from single-value field references and non-empty literals. Sources are joined in order using `separator`, and new authoring uses shared mode. A target has only one binding. Multi-value content belongs on an input port through `sourceType=initial_input`, not in prompt composition. Historical expanded bindings remain runtime-compatible but cannot be authored or newly published.

```json
{"stepId":"stp_write01","paramKey":"prompt","bindMode":"shared","separator":"\n\n","sources":[{"kind":"field_ref","fieldKey":"content"},{"kind":"field_ref","fieldKey":"tone"},{"kind":"literal","literal":"Return Markdown."}]}
```

Every field reference must exist and every literal must be non-empty. Use FieldBinding for a single direct value. Model routing does not support ParamBinding. Before check, confirm no other binding writes the same step/parameter target.
