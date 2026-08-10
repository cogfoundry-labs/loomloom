# Examples

These are starter inputs for hosted LoomLoom templates and private TemplateSpec authoring.

If you still call the workflow `batchjob`, `batchflow`, or "batch processing", those names map to the same LoomLoom template flow.

- [Product image generation](official/for-developers/product-image-generation/)
- [Text-to-image generation](official/for-developers/text-image-generation/)
- [Text-to-image-to-video generation](official/for-developers/text-image-video-generation/)

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
./src/cli/loomloom run submit text-v1 -f /tmp/symphony-code-review.jsonl --client-request-id <stable-id>
```

Recommended follow-up:

```bash
./src/cli/loomloom run watch <run-id>
./src/cli/loomloom artifact download <run-id> --output-dir ./downloads
```

Current PoC assumptions:

- one code file = one task
- only single-file review, not cross-file reasoning
- best for first-pass screening such as security smells, leak risks, and poor patterns
- large files are truncated on purpose to keep each task bounded
