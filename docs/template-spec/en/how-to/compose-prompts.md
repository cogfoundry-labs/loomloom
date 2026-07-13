# Compose prompts

Use one ParamBinding for a prompt built from multiple field references and non-empty literals. Sources are joined in order using `separator`. A target has only one binding. At most one source may be multi-value; when present, use expanded mode. Uploaded content belongs on input ports, not in prompt composition.

```json
{"stepId":"stp_write01","paramKey":"prompt","bindMode":"shared","separator":"\n\n","sources":[{"kind":"field_ref","fieldKey":"content"},{"kind":"field_ref","fieldKey":"tone"},{"kind":"literal","literal":"Return Markdown."}]}
```

Every field reference must exist and every literal must be non-empty. Use FieldBinding for a single direct value. Model routing does not support ParamBinding. Before check, confirm no other binding writes the same step/parameter target.
