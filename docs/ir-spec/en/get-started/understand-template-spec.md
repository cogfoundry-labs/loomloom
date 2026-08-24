# Understand TemplateSpec v2

A v2 Spec has four top-level sections:

```json
{"meta":{"name":"Example"},"templateInputs":{},"steps":[],"workbook":{}}
```

- `meta` describes the template.
- `templateInputs` defines values and Artifacts supplied through Workbook or API.
- `steps` defines the DAG, execution authorities, routing, and input sources.
- `workbook` contains instructions and sample rows; columns come from `templateInputs`.

Each Step input is identified by `<stepId>.<portId>`. Two models may both expose `prompt` without being implicitly merged. They share data only when both bindings explicitly reference the same Template Input.

On version creation, Core resolves authority records and freezes an executable snapshot. Runtime consumes that frozen version instead of reinterpreting the current catalog.
