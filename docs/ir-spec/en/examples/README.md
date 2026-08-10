# TemplateSpec examples

Use `loomloom template-spec docs examples` to view this index. Choose the valid example closest to the capability you need as a local draft. The examples ship with the CLI, so reading the Core source checkout is not required. Preserve referenced field keys, step IDs, `dependsOn`, and `upstreamBindings` while you first change business text, fields, and models.

## Valid

| Capability | Start with | Key behavior |
| --- | --- | --- |
| Generate content from one text field | `valid/single-text-generation.json` | string → FieldBinding(prompt) → text-generate |
| Use uploaded text as model context | `valid/uploaded-text-reference.json` | text_reference → initial_input(reference) |
| Process one upstream output linearly | `valid/multi-step-review.json` | one upstream `dependsOn` and step_output |
| Combine several upstream text results | `valid/multi-upstream-summary.json` | several upstreams and a prompt port that allows multiple |
| Use a fixed number of parallel branches | `valid/parallel-text-to-image-branches.json` | shared input, multiple root steps, paired text → image branches |
| Define workbook fields and model selection | `valid/workbook-fields-and-model-override.json` | enum, hidden default, presentation, ParamBinding, model override |
| Combine text, image, and video | `valid/text-image-video-chain.json` | execution-unit port compatibility |

The fixed-parallel example is not `expanded`: the number of branches comes from repeated Step declarations in the Spec. JSON has no `branch` or `parallel` property.

Valid examples pass JSON Schema and the real Core validator. Fixture model IDs are structural values; query the target environment before creation.

## Invalid

- `invalid/uploaded-text-as-string.json`
- `invalid/text-reference-without-input-binding.json`

Invalid examples link to stable rules in the manifest. They can pass structural Schema validation but must be rejected by authoring or Core semantic rules.

After copying an example, read the relevant how-to, then run:

```bash
loomloom template-spec check ./template.json
```
