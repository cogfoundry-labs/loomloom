# Expose model overrides

Set `allowModelOverride=true`, create a non-reserved input key such as `text_model`, and map it to `paramKey=model` with shared FieldBinding. Model does not support expanded or ParamBinding. Provider and mode are not template routing inputs. Keep a fixed default unless user choice is required.

```json
{"fieldKey":"text_model","stepId":"stp_write01","paramKey":"model","bindMode":"shared"}
```

The input key itself cannot be the reserved key `model`. Validate every candidate model against the target execution unit. A stable default without user override is safer for standard templates; expose choice only when it is part of the product experience.
