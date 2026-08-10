# Product image generation

`custom-template-text-image.spec.json` is a starter TemplateSpec for agent-authored
private templates. It creates a two-step template:

```text
text-generate.output -> image-generate.prompt
```

TemplateSpec uses step-level connections. `dependsOn` declares upstream steps, and
`upstreamBindings` connects a source step output port, usually `output`, into a target
input port such as `prompt`, `reference`, or `image`. Fan-in is accepted only when the
target input port contract allows it (`AllowMultiple=true`, matching `Accepts`, and a
defined `MergePolicy`).

Use it with:

```bash
./src/cli/loomloom template-spec docs spec
./src/cli/loomloom template-spec docs examples
./src/cli/loomloom template-spec check examples/official/for-developers/product-image-generation/custom-template-text-image.spec.json
# Confirm before creating a private template; this writes remote state.
./src/cli/loomloom template-spec create examples/official/for-developers/product-image-generation/custom-template-text-image.spec.json --version-note "initial version"
./src/cli/loomloom template-spec download-workbook <template-id> <version-id> --output-file ./custom-input.xlsx
./src/cli/loomloom template-spec validate-workbook <template-id> <version-id> ./custom-input.xlsx
```

Submitting the workbook creates a real run:

```bash
./src/cli/loomloom template-spec precheck-workbook <template-id> <version-id> ./custom-input.xlsx
# Review the estimate and confirm before submitting.
./src/cli/loomloom template-spec submit-workbook <template-id> <version-id> ./custom-input.xlsx --client-request-id <stable-id>
```
