# TemplateSpec v2

TemplateSpec v2 is the public JSON format used to create and update LoomLoom workflow versions. It separates four responsibilities:

```text
Template Input ── inputBindings ──> Step contract input port
Upstream output ── inputBindings ──> Downstream contract input port
executionBinding ────────────────> Fixed model contract or Capability Profile
Template Input ──────────────────> Workbook / Public API input
```

## Minimum rules

- New writes accept only `template-spec/v2`; v1 is read-only and can be migrated offline.
- `templateInputs` keys are template-level input identities and generate Workbook columns.
- Each `steps[].inputBindings` map key is a target contract `portId`; one port has one binding.
- `executionBinding` contains authority references only, never runtime values.
- A `stepOutput` source must also name its source Step in `dependsOn`.
- Scalars and Artifacts use the same source union. Use `sequence` for ordered multimodal values and `merge` for homogeneous Artifact collections.
- Capability Profile model routing uses the separate `modelSelection` field and never enters Provider-native JSON.
- Code uses `codeProfile` and `code` for its environment and ports; boolean `when` controls conditions, and explicit Loop containers control bounded rework. See the [Step reference](reference/steps.md) for implemented capabilities and controlled test boundaries.

Start with the [quickstart](get-started/quickstart.md), [syntax reference](reference/template-syntax.md), and [examples](examples/README.md). Machine validation is defined by the [JSON Schema](../machine/template-spec.schema.json) and Core `ValidateTemplateSpecV2`.

Use the [CLI Code review walkthrough](how-to/code-cli-review.md) to exercise the authoring and result flow.
