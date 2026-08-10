# Text-to-image generation

`text-image-v1.input.jsonl` is a starter input for the hosted `text-image-v1` LoomLoom template.

Prefer workbook flows with precheck when you need a pre-submission estimate. Use this JSONL example only when you intentionally want programmatic submission and have confirmed execution.

```bash
# Confirm before submitting; this command creates a real hosted run.
./src/cli/loomloom run submit text-image-v1 -f examples/official/for-developers/text-image-generation/text-image-v1.input.jsonl --client-request-id <stable-id>
```
