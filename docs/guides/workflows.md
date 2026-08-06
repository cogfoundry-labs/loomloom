# loomloom workflows

This guide covers the main end-to-end workflows supported by loomloom, from using official templates, creating and running private templates, and publishing them as SkillBots — all through CLI commands.

If you have more than one Server profile, select the platform for the workflow before continuing:

```bash
loomloom server list
loomloom server use <name-or-server>
loomloom template list
```

The commands below use the selected Server. Official template availability can differ between platforms, so use the returned template list as the source of truth.

Use a new `<client-request-id>` for every newly confirmed execution. Reuse an ID only when retrying the identical confirmed request after an ambiguous failure.

## 1. Batch copywriting with an official template

Rewrite a spreadsheet of product descriptions with `text-v1`:

```bash
loomloom template download text-v1 --output-file ./copy.xlsx
# ...fill in the columns provided by the downloaded workbook...
loomloom template validate-file text-v1 ./copy.xlsx
loomloom template precheck-file text-v1 ./copy.xlsx
# Review the estimate and confirm before submitting.
loomloom template submit-file text-v1 ./copy.xlsx --client-request-id <client-request-id>
loomloom run watch <run-id>
loomloom run result-workbook <run-id> --output-file ./copy.result.xlsx
```

Field definitions for `text-v1` are in [Official templates](../reference/official-templates.md#text-template-text-v1).

## 2. Batch image generation with an official template

Generate a batch of social images from a workbook of prompts (the canonical example):

```bash
loomloom template download text-image-v1 --output-file ./task.xlsx
loomloom template validate-file text-image-v1 ./task.xlsx
loomloom template precheck-file text-image-v1 ./task.xlsx
# Review the estimate and confirm before submitting.
loomloom template submit-file text-image-v1 ./task.xlsx --client-request-id <client-request-id>
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
loomloom template-spec submit-workbook <template-id> <version-id> ./input.xlsx --client-request-id <client-request-id>
```

Full authoring guide, including the row-data (JSONL) alternative to workbooks, is in [Understand your private template](../reference/private-template.md).

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
loomloom market run <listing-id> --input-file ./request.json --confirm --client-request-id <client-request-id>
loomloom usage get <run-transaction-id>
```

For the complete creator and user workflow, including publishing, purchasing, and running SkillBots, see [Build your first SkillBot](../guides/build-your-first-skillbot.md).
