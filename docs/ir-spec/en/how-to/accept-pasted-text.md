# Accept pasted text

Declare a required `string` field and map it to an allowed text parameter with shared FieldBinding. Keep fixed processing requirements in the step instruction. Run `loomloom template-spec check`. Never instruct users to place an `ia_*` asset ID in a string field; use a reference field instead.

```json
{"key":"content","label":"Content","valueType":"string","required":true,"presentation":{"widget":"textarea"}}
```

```json
{"fieldKey":"content","stepId":"stp_write01","paramKey":"prompt","bindMode":"shared"}
```

Use FieldBinding when one value maps directly. Use ParamBinding when prompt combines several fields. A single-value field requires shared mode. Verify with local check. Duplicate target bindings and descriptions that request asset IDs are invalid authoring patterns.
