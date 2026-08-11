# Template Execution

Use this reference for official or private template execution, workbook and JSON/JSONL input, validation, precheck, submission, run monitoring, results, artifacts, large files, and trusted result sources.

## Contents

- [Input mode](#input-mode)
- [Official templates](#official-templates)
- [Private templates](#private-templates)
- [Runs and results](#runs-and-results)
- [Large files and trusted sources](#large-files-and-trusted-sources)

Before creating any hosted run, also read `billing.md`. For exact command discovery and error recovery, also read `cli.md`.

## Input Mode

Default to the Excel workbook experience. Let the user see and fill a workbook before execution.

Use JSON, JSONL, raw request bodies, or API-style input only when the user explicitly asks for programmatic input or supplies an existing compatible request file.

Receiving input values is preparation consent, not execution consent. Installation, discovery, downloads, uploads, validation, and precheck do not authorize a run.

If input changes after validation, precheck, or confirmation:

1. Validate again.
2. Precheck again.
3. Show the new estimate.
4. Obtain a new confirmation.
5. Use a new client request ID.

Tell the user in their language that the changed input must be re-estimated and reconfirmed.

## Official Templates

Use official templates unless the user explicitly asks to create or execute their own private template.

Discover and inspect:

```bash
loomloom template list
loomloom template schema <template-id>
loomloom model list --step-type <text-generate|image-generate|video-generate>
```

### Workbook flow

1. `loomloom template download <template-id>`
2. Let the user fill or approve the workbook.
3. `loomloom template validate-file <template-id> <xlsx-path>`
4. `loomloom template precheck-file <template-id> <xlsx-path>`
5. Show the official-template fee confirmation and wait for explicit confirmation.
6. `loomloom template submit-file <template-id> <xlsx-path> --client-request-id <id>`
7. Watch the run and retrieve the result workbook.

Use `template backfill-results` only when the user explicitly needs the older local backfill flow.

### JSON/JSONL flow

Use only for an explicit programmatic-input request:

1. Prepare JSON or JSONL rows.
2. `loomloom run validate <template-id> -f <rows.json-or-jsonl>`
3. `loomloom run precheck <template-id> -f <rows.json-or-jsonl>`
4. Show the official-template fee confirmation and wait for explicit confirmation.
5. `loomloom run execute <template-id> -f <rows.json-or-jsonl> --client-request-id <id>`

## Private Templates

Private execution always binds to an explicit `template_id + version_id`. Do not silently use another version.

### Workbook flow

1. `loomloom template-spec download-workbook <template-id> <version-id>`
2. Let the user fill or approve the workbook.
3. `loomloom template-spec validate-workbook <template-id> <version-id> <xlsx-path>`
4. `loomloom template-spec precheck-workbook <template-id> <version-id> <xlsx-path>`
5. Show the private-template fee confirmation and wait for explicit confirmation.
6. `loomloom template-spec submit-workbook <template-id> <version-id> <xlsx-path> --client-request-id <id>`

### JSONL flow

Use only for an explicit JSONL/API/programmatic request:

1. Prepare the JSONL rows.
2. `loomloom orchestration-input upload <file.jsonl>`
3. Preserve the returned `inputFileId`.
4. `loomloom template-spec precheck <template-id> --version-id <version-id> --input-file-id <input_file_id>`
5. Show the private-template fee confirmation and wait for explicit confirmation.
6. `loomloom template-spec run <template-id> --version-id <version-id> --input-file-id <input_file_id> --client-request-id <id>`

For common single-root workflows, each non-empty JSONL line may be a flat object with string values. Unified rows using `steps.<step-id>.executions[]` are also supported when exact step mappings are known. Parameter values must be strings and supported by the selected version. Never guess step IDs.

If a requested path cannot return an estimate before submission, do not submit. Choose an equivalent precheck-capable path or explain what compatible input is required.

## Runs And Results

Use:

```bash
loomloom run list
loomloom run get <run-id>
loomloom run watch <run-id>
loomloom run result-rows <run-id>
loomloom run result-workbook <run-id>
loomloom artifact list <run-id>
loomloom artifact download <run-id>
```

Preserve `runId` exactly from the submission response.

Do not print workbook base64 `content`. Do not copy temporary signed `accessUrl` values into long-lived logs or documentation. Prefer `inlineText` for small text artifacts and state that a download URL is available for file artifacts.

## Large Files And Trusted Sources

Do not paste large files into model context. Upload raw source files first:

```bash
loomloom input-asset upload <file>
```

Preserve the returned `input_asset_id` and use it only in a schema field that accepts an asset reference. It is not the `inputFileId` used by private-template JSONL execution.

For TemplateSpec, use a compatible `asset_ref` or `text_reference` field and follow the bundled TemplateSpec contract.

For submitted workbooks and result workbooks:

- The submitted workbook and the server-side run input snapshot are the input sources of truth.
- Prefer `run result-workbook` after completion because the server aligns original input snapshots and artifacts.
- Use `template backfill-results` only for an explicit legacy-backfill request.

When a template version changes, do not promise that an older workbook remains compatible. Download a workbook for the new version.
