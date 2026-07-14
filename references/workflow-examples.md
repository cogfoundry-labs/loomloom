# Example Workflows

End-to-end walkthroughs for the main ways to use LoomLoom. Each one assumes you've already completed [Quick Start](../README.md#quick-install) (`loomloom doctor` reports healthy).

## 1. Batch copywriting with an official template

Rewrite a spreadsheet of product descriptions with `text-v1`:

```bash
loomloom template download text-v1 --output-file ./copy.xlsx
# ...fill in "Text prompt" / "Writing requirements" / "Reference text" columns...
loomloom template validate-file text-v1 ./copy.xlsx
loomloom template precheck-file text-v1 ./copy.xlsx
# Review the estimate and confirm before submitting.
loomloom template submit-file text-v1 ./copy.xlsx --client-request-id <stable-id>
loomloom run watch <run-id>
loomloom run result-workbook <run-id> --output-file ./copy.result.xlsx
```

Field definitions for `text-v1` are in [`official-templates.md`](official-templates.md#text-template-text-v1).

## 2. Batch image generation with an official template

Generate a batch of social images from a workbook of prompts (the canonical example, also shown in the [README Quick Start](../README.md#quick-install)):

```bash
loomloom template download text-image-v1 --output-file ./task.xlsx
loomloom template validate-file text-image-v1 ./task.xlsx
loomloom template precheck-file text-image-v1 ./task.xlsx
# Review the estimate and confirm before submitting.
loomloom template submit-file text-image-v1 ./task.xlsx --client-request-id <stable-id>
loomloom run watch <run-id>
loomloom run result-workbook <run-id> --output-file ./task.result.xlsx
loomloom artifact download <run-id> --output-dir ./downloads
```

## 3. Author and run your own private template

Build a custom multi-step workflow with TemplateSpec, then run it via workbook:

```bash
loomloom template-spec models text-generate
loomloom template-spec check ./my-template.spec.json
# Confirm before creating a new private template.
loomloom template-spec create ./my-template.spec.json --version-note "initial version"
loomloom template-spec download-workbook <template-id> <version-id> --output-file ./input.xlsx
# ...fill in the workbook...
loomloom template-spec validate-workbook <template-id> <version-id> ./input.xlsx
loomloom template-spec precheck-workbook <template-id> <version-id> ./input.xlsx
# Review the estimate and confirm before submitting.
loomloom template-spec submit-workbook <template-id> <version-id> ./input.xlsx --client-request-id <stable-id>
```

Full authoring guide, including the row-data (JSONL) alternative to workbooks, is in [`private-templates.md`](private-templates.md).

## 4. Publish a private template as a paid SkillBot

Once a private template version has at least one successful run, publish it to the Market:

```bash
# Confirm before submitting this listing review request.
loomloom listing publish <template-id> \
  --template-version-id <version-id> \
  --display-name "My SkillBot" \
  --task-fixed-fee 0.1
loomloom creator review list
```

## 5. Buy and run a SkillBot from the Market

```bash
loomloom market list --keyword "tweet"
loomloom market show <listing-id>
loomloom market quote <listing-id> --input-file ./request.json
# Review the quote and confirm before running.
loomloom market run <listing-id> --input-file ./request.json --confirm --client-request-id <stable-id>
loomloom usage get <run-transaction-id>
```

Full buyer/creator command flows, fee structure, and the local Agent Skill wrapper commands are in [`market-skillbots.md`](market-skillbots.md).
