# Accept uploaded text files

Declare `valueType=text_reference` with accepted MIME types. Add an upstream binding with `sourceType=initial_input`, the field key as `sourceInputKey`, and a compatible port such as text-generate `reference`. Put processing requirements in instruction. Do not FieldBind the asset ID to prompt. See TS-IN-002.

## Prerequisites

Confirm that the target execution-unit port accepts the uploaded MIME type. For plain text, `text-generate.reference` accepts `text/*`.

```json
{"key":"source_text","label":"Source","valueType":"text_reference","acceptedMimeTypes":["text/plain"],"required":true}
```

```json
{"inputPort":"reference","sourceType":"initial_input","sourceInputKey":"source_text"}
```

Place the second object in the target step's `upstreamBindings`. Upload the file at run preparation time and put the returned input asset ID in the public field. It is not the orchestration input file ID. Common failures are string + prompt + asset ID, reference fields also bound as prompt, and MIME that the target port does not accept.

## Verify

```bash
loomloom template-spec check ./template.json
```

Use `examples/valid/uploaded-text-reference.json` as the complete fixture. If check reports TS-IN-002, remove the prompt FieldBinding and retain the initial-input binding. See `reference/bindings.md` and `reference/execution-units.md`.
