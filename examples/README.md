# Examples

These are starter inputs for hosted LoomLoom templates.

If you still call the workflow `batchjob`, `batchflow`, or "batch processing", those names map to the same LoomLoom template flow.

- `text-image-v1.input.jsonl`
- `text-image-video-v1.input.jsonl`
- `custom-template-text-image.spec.json`

Use them with:

Prefer workbook flows with precheck when you need a pre-submission estimate. Use these JSONL examples only when you intentionally want programmatic submission and have confirmed execution.

```bash
# Confirm before submitting; these commands create real hosted runs.
./cli/loomloom run submit text-image-v1 -f examples/text-image-v1.input.jsonl --client-request-id <stable-id>
./cli/loomloom run submit text-image-video-v1 -f examples/text-image-video-v1.input.jsonl --client-request-id <stable-id>
```

## Custom Template Spec

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
./cli/loomloom template-spec docs spec
./cli/loomloom template-spec docs examples
./cli/loomloom template-spec check examples/custom-template-text-image.spec.json
# Confirm before creating a private template; this writes remote state.
./cli/loomloom template-spec create examples/custom-template-text-image.spec.json --version-note "initial version"
./cli/loomloom template-spec download-workbook <template-id> <version-id> --output-file ./custom-input.xlsx
./cli/loomloom template-spec validate-workbook <template-id> <version-id> ./custom-input.xlsx
```

Submitting the workbook creates a real run:

```bash
./cli/loomloom template-spec precheck-workbook <template-id> <version-id> ./custom-input.xlsx
# Review the estimate and confirm before submitting.
./cli/loomloom template-spec submit-workbook <template-id> <version-id> ./custom-input.xlsx --client-request-id <stable-id>
```

## Code Review PoC

`text-v1` can also be used as a batch code-review proof of concept.

The helper below scans one local repository, turns each selected code file into one
task row, and writes a JSONL file that can be submitted with `run submit`.

Example:

```bash
python3 scripts/generate-code-review-jsonl.py \
  --repo /Users/zhouyang/project/github/symphony \
  --output /tmp/symphony-code-review.jsonl \
  --max-files 20
```

Then submit it with:

Prefer a workflow path with precheck when you need a pre-submission estimate.

```bash
# Confirm before submitting; this command creates a real hosted run.
./cli/loomloom run submit text-v1 -f /tmp/symphony-code-review.jsonl --client-request-id <stable-id>
```

Recommended follow-up:

```bash
./cli/loomloom run watch <run-id>
./cli/loomloom artifact download <run-id> --output-dir ./downloads
```

Current PoC assumptions:

- one code file = one task
- only single-file review, not cross-file reasoning
- best for first-pass screening such as security smells, leak risks, and poor patterns
- large files are truncated on purpose to keep each task bounded
