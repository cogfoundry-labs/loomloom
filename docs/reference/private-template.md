# Understand your private template

In loomloom, templates are reusable AI work IR that define how AI work is executed.

A private template is a user-created template defined with `TemplateSpec`. It supports multiple immutable versions and can be executed either through a workbook or directly from row data as inputs.

See the [TemplateSpec Manual](../ir-spec/en/README.md) (or [TemplateSpec 手册](../ir-spec/zh-CN/README.md) for Chinese) to learn more.


## Input assets vs. orchestration inputs

These are two different upload mechanisms — don't confuse them:

- **`input-asset upload`** returns an `input_asset_id` for reference material (a file or image) placed **inside a single template field**.
- **`orchestration-input upload`** returns an `input_file_id` that supplies the **row data** for a `template-spec run`.

```bash
loomloom input-asset upload ./brief.txt --content-type text/plain
loomloom input-asset upload ./diagram.png --content-type image/png
```

An orchestration input file is JSONL. For the common single-root workflow, each non-empty line can be a flat JSON object whose values are strings:

```json
{"prompt":"first request"}
{"prompt":"second request"}
```

The backend also supports unified rows shaped as `steps.<step-id>.executions[]` when a workflow needs explicit per-step execution input. In both formats, execution parameter values must be strings and must match the private template version's allowed input parameters. Do not invent step IDs — use unified input only when the exact workflow step mapping is available.

## Authoring flow

A typical agent-assisted authoring flow:

```bash
# 1. Resolve the Step's business input/output capability
loomloom capability resolve --input image --output-modality text --output json

# 2a. For lower-level Profile diagnosis, inspect the full authoring context
loomloom template-spec authoring-context --output json

# 2b. For lower-level fixed-model diagnosis, inspect one model's contracts
loomloom template-spec contracts <model-id> --output json

# 3. Validate against the same Server where the version will be created
loomloom template-spec check ./my-template.spec.json

# 4. Confirm, then create a private template
loomloom template-spec create ./my-template.spec.json --version-note "initial version"

# 5. Confirm, then add a new version when the template changes
loomloom template-spec create-version <template-id> ./my-template.spec.json

# 6. Download, fill, validate, precheck, confirm, and submit the workbook
loomloom template-spec download-workbook <template-id> <version-id> --output-file ./input.xlsx
loomloom template-spec validate-workbook <template-id> <version-id> ./input.xlsx
loomloom template-spec precheck-workbook <template-id> <version-id> ./input.xlsx
# Review the estimate and confirm before submitting.
loomloom template-spec submit-workbook <template-id> <version-id> ./input.xlsx --client-request-id <client-request-id>
```

Notes:

- TemplateSpec JSON is the source of truth; workbooks are generated artifacts.
- `capability resolve` is the primary server-authoritative selection path. It reports authoring eligibility, not instantaneous execution availability.
- `template-spec check` is server-authoritative and does not create a version. Creation validates again before freezing current contracts.
- Use `template-spec get-version <template-id> <version-id> -f historical.json` to export an owner-visible historical authoring spec. Readback checks only that the stored authoring is a JSON object; it neither exports the frozen execution bundle nor applies current TemplateSpec rules. Run `check` separately on a rewritten v2 candidate.
- `template-spec contracts` is only for an exact-model `fixedModelContract`. Text-generation models intentionally use the shared `capabilityProfile`, so an empty contract list for a `text-generate` model is expected and does not prevent private template authoring.
- For a text Step, choose the default model, Profile identity, and ports from `template-spec authoring-context`; normal templates omit `profileRevision`. Do not fabricate a `subjectRevisionId`.
- Review the bundled spec with `loomloom template-spec docs spec` before writing custom specs.
- Use `loomloom template-spec docs examples` for patterns.
- Use `loomloom template-spec docs conversation` for agent-assisted conversational authoring.
- Template changes require downloading a new workbook.
- Run `precheck-workbook` before submitting when you want to estimate model/API cost and balance; precheck does not create a run or execute tasks.
- Precheck text output includes `estimated_cost`, `available_balance`, and `sufficient`; JSON output uses `estimatedTotalCostT`.
- `submit-workbook` creates a real hosted run; agents should ask for explicit confirmation before submitting.
- `template-spec run` also creates a real hosted run and requires the same confirmation.
- Use a new `--client-request-id` for every newly confirmed `submit-workbook` or `template-spec run` execution. Reuse it only for an identical retry of the same confirmed request after an ambiguous failure.

## Running from JSONL rows (no workbook)

You can also run a private template version directly from flat JSONL rows, without filling a workbook:

```bash
# 1. Upload the rows and capture the returned input_file_id
loomloom orchestration-input upload ./rows.jsonl

# 2. Optional fast budget reference; this does not validate referenced resources
loomloom template-spec estimate <template-id> --version-id <version-id> --input-file-id <input_file_id>

# 3. Required full input precheck, cost estimate, and balance status
loomloom template-spec precheck <template-id> --version-id <version-id> --input-file-id <input_file_id>

# 4. Review the precheck, confirm, then run the version with that input
loomloom template-spec run <template-id> --version-id <version-id> --input-file-id <input_file_id> --client-request-id <client-request-id>
```
