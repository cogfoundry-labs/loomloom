# CLI Command Reference

This document provides a complete reference for the **LoomLoom CLI**.

If you are new, start with:

- `loomloom doctor` (check setup)
- `loomloom template list` (see available workflows)
- `loomloom template download` (get your first workbook workflow)

For installation, authentication, and a quick start guide, see the main **[README](../README.md)**.

## How to read this CLI

LoomLoom's CLI is organized into the following command groups, in the order they appear below:

1. **Diagnostics** → check system health
2. **Inputs** → upload data for workflows
3. **Official Templates** → run official workflows (Excel-based)
4. **Runs** → monitor execution results
5. **Artifacts** → download generated files
6. **Catalog** → list available models and assets
7. **Private Templates** → build your own workflows with TemplateSpec
8. **Local Agent Skills** → install/uninstall workflows as agent skills
9. **Market — Buy** → discover and run SkillBots
10. **Market — Create** → publish and manage SkillBots

Most users only need:

> template download → fill Excel → validate-file → precheck-file → confirm → submit-file → watch → result-workbook

Useful inspection commands:

- `loomloom <command> --help` shows positional arguments and flags.
- `loomloom template schema <template-id> --output json` shows official template fields.
- `loomloom market show <listing-id> --output json` shows a Market SkillBot's public input schema.
- `loomloom template-spec docs spec|examples|conversation` shows TemplateSpec authoring docs.
- Use `--output json` when one command feeds another, and preserve returned IDs exactly.

## Monetary Values (Important)

Some commands involve pricing (Market / Usage / Billing).

### Unit system

All monetary fields like `taskFixedFeeT` and `amountT` use:

> 10,000,000 API units = 1 currency unit

## What you see in CLI output

### Human-readable mode (default)
- Prices are shown as normal currency: `USD 0.5000000`
- Raw `*T` fields are not shown as primary text output
- If a response does not include `currency`, the CLI must not guess CNY or USD; keep the raw `*T` value in the currency-unknown display.

### JSON mode (`--output json`)
- Returns raw backend values only
- No conversion is applied

- Market and usage-related commands automatically convert `*T` fields into currency values (e.g. `USD 0.5000000`)
- The original `*T` values remain available in JSON output for reconciliation

### Affected commands

These commands show monetary values:

- Market execution: `market list/show/quote/run`
- Workbook execution: `market workbook quote/run`
- Listings: `listing list/show/versions`
- Usage tracking: `usage list/get`
- Creator earnings: `creator transactions`

## 1. Diagnostics
Check whether your CLI is correctly configured.

| Command | Description |
|---|---|
| `loomloom doctor` | Validate CLI configuration, server connectivity, token wiring, and version info. |

## 2. Inputs (optional advanced)
Use these when you are not working directly with Excel workflows.

| Command | Description |
|---|---|
| `loomloom input-asset upload <file>` | Upload reusable raw assets (text/image) and return `input_asset_id`. |
| `loomloom orchestration-input upload <file.jsonl>` | Upload flat JSONL rows and get the `input_file_id` required by `template-spec precheck` and `template-spec run`. |

## 3. Official Templates

This is the **recommended starting point for most users**.

### Typical flow
> download → fill Excel → validate → precheck → confirm → submit → watch → download results

| Command | Description |
|---|---|
| `loomloom template list` | List available official templates. |
| `loomloom template schema <id>` | Show template input schema. |
| `loomloom template download <id>` | Download Excel workbook template. |
| `loomloom template validate-file <id> <xlsx>` | Validate workbook input. |
| `loomloom template precheck-file <id> <xlsx>` | Estimate cost without execution. |
| `loomloom template submit-file <id> <xlsx> --client-request-id <id>` | Execute template from workbook after confirmation. |
| `loomloom template backfill-results <run-id> <xlsx>` | Backfill results into a local workbook (legacy). |

## 4. Runs

After submitting a workflow, you use run commands.
`run submit` validates and prechecks official-template JSON/JSONL rows, then creates a hosted run. Use it only after the user has confirmed execution; prefer the workbook `validate-file` / `precheck-file` flow when you need a separate review step before submission.

| Command | Description |
|---|---|
| `loomloom run submit <id> -f rows.json --client-request-id <id>` | Submit input from a JSON array or JSONL file after confirmation. |
| `loomloom run list` | List runs with optional Market context. |
| `loomloom run get <run-id>` | Show one run's detail. |
| `loomloom run watch <run-id>` | Watch run progress until a terminal state. |
| `loomloom run result-rows <run-id>` | Show aligned input rows and results. |
| `loomloom run result-workbook <run-id>` | Download a server-generated result workbook. |

## 5. Artifacts (outputs)

Generated files from workflows (images, videos, documents, etc.)

| Command | Description |
|---|---|
| `loomloom artifact list <run-id>` | List generated artifacts. |
| `loomloom artifact download <run-id>` | Download generated artifacts. |

## 6. Catalog (advanced system info)

| Command | Description |
|---|---|
| `loomloom model list --step-type <type>` | List executable models for a step type. |
| `loomloom asset list` | Aggregated list of my private templates and available Market SkillBots; does not include official templates. |

## 7. Private Templates (created via TemplateSpec)
For building your own private workflows.

`template-spec create` and `template-spec create-version` create or change remote template resources. Agents should summarize the action and ask for explicit confirmation before invoking them.

| Command | Description |
|---|---|
| `loomloom template-spec check <spec.json>` | Validate a TemplateSpec used to create a private template. |
| `loomloom template-spec docs [topic]` | Show bundled TemplateSpec documentation. |
| `loomloom template-spec models <step-type>` | List models for a step type. |
| `loomloom template-spec create <spec.json>` | Create a private template. |
| `loomloom template-spec create-version <template-id> <spec.json>` | Add a new version to an existing private template. |
| `loomloom template-spec list` | List my private templates. |
| `loomloom template-spec get <template-id>` | Show one private template and its versions. |
| `loomloom template-spec versions <template-id>` | List versions of a private template. |
| `loomloom template-spec download-workbook <template-id> <version-id>` | Download a user-template workbook. |
| `loomloom template-spec validate-workbook <template-id> <version-id> <xlsx>` | Validate a user-template workbook. |
| `loomloom template-spec precheck-workbook <template-id> <version-id> <xlsx>` | Estimate cost and balance for a user-template workbook without submitting. |
| `loomloom template-spec submit-workbook <template-id> <version-id> <xlsx> --client-request-id <id>` | Submit a user-template workbook after confirmation. |
| `loomloom template-spec precheck <template-id> --version-id <id> --input-file-id <id>` | Estimate cost and balance for an uploaded JSONL input without submitting. |
| `loomloom template-spec run <template-id> --version-id <id> --input-file-id <id> --client-request-id <id>` | Run a private template version from an uploaded JSONL input after confirmation. |

## 8. Local Agent Skills
Install/uninstall LoomLoom workflows as local tools for AI agents (e.g. Claude Code, Codex).

| Command | Description |
|---|---|
| `loomloom skill install market <listing-id> --agent <agent> --output-dir <skill-dir>` | Install a Market SkillBot as a local agent skill by generating a local agent Skill wrapper. |
| `loomloom skill install template-spec <template-id> <version-id> --agent <agent> --output-dir <skill-dir>` | Install a private template version as a local agent skill by generating a local agent Skill wrapper. |
| `loomloom skill uninstall --dir <skill-dir>` | Remove a locally installed skill installed by loomloom. |

### Notes
- For install, use `--dry-run --output json` before writing final Skill files when an agent needs a stable installation preview for a confirmation card. Dry-run does not create the final `--output-dir` or write `SKILL.md` / `loomloom-skill.json`; it may create and immediately remove a temporary probe directory to verify writability.
- `--output-dir` is the directory for one generated Skill, not an agent skills root directory.
- Generated Skill names always use the `loomloom-` prefix, and the final `--output-dir` basename must match the previewed `skillName`.
- Installation only writes local wrapper files; it does not execute a template, quote/precheck costs, or create billable model/API usage.
- For uninstall, run `loomloom skill uninstall --dir <skill-dir> --dry-run --output json` first. The command only removes directories that contain valid LoomLoom skill metadata; pass `--force` only when you intentionally want to remove a directory with extra files in it.

## 9. Market (Buy workflows)

Typical flow
> list → show → quote → confirm → run

| Command | Description |
|---|---|
| `loomloom market list` | Browse published Market SkillBots. |
| `loomloom market show <listing-id>` | Show one SkillBot, including its input schema. |
| `loomloom market quote <listing-id> --input-file <json>` | Estimate execution cost. |
| `loomloom market run <listing-id> --input-file <json> --confirm --client-request-id <id>` | Execute a SkillBot from JSON input rows (paid). |
| `loomloom market workbook download <listing-id> --output-file <xlsx>` | Download a Market workbook template. |
| `loomloom market workbook validate <listing-id> --file <xlsx>` | Validate a filled Market workbook. |
| `loomloom market workbook quote <listing-id> --file <xlsx>` | Estimate execution cost for a workbook. |
| `loomloom market workbook run <listing-id> --file <xlsx> --confirm --client-request-id <id>` | Execute a SkillBot from a workbook (paid). |
| `loomloom usage list` | List my Market SkillBot usage records. |
| `loomloom usage get <run-transaction-id>` | Show one usage record. |

## 10. Market (Create Workflows)

Typical flow
> confirm → publish → review → earnings

| Command | Description |
|---|---|
| `loomloom listing publish <template-id> --template-version-id <id> --display-name <name> --task-fixed-fee <amount>` | Submit a template version for Market review after confirmation. Use normal currency units such as `--task-fixed-fee 0.5`. |
| `loomloom listing publish <template-id> --listing-id <listing-id> --template-version-id <new-id> ...` | Submit a new version for an existing listing after confirmation. |
| `loomloom listing list` | List my Market listings. |
| `loomloom listing show <listing-id>` | Show one of my listings. |
| `loomloom listing versions <listing-id>` | List versions of one of my listings. |
| `loomloom listing update <listing-id> --display-name <name>` | Submit a public-profile update for review after confirmation; pass a display name, description, or both. |
| `loomloom listing unlist <listing-id>` | Stop new executions of a listing after confirmation. |
| `loomloom listing relist <listing-id>` | Restore a previously unlisted listing after confirmation. |
| `loomloom listing withdraw <listing-id>` | Withdraw the pending review request for a listing after confirmation. |
| `loomloom creator earnings` | List Market earnings. |
| `loomloom creator transactions` | List Market transactions. |
| `loomloom creator review list` | List my review requests. |
| `loomloom creator review get <review-request-id>` | Show one review request. |
| `loomloom creator review withdraw <review-request-id>` | Withdraw a pending review request after confirmation. |

## Multi-Step Workflows

When building multi-step workflows, agents must treat CLI commands as a deterministic pipeline.

### General rules
- Use `--output json` when command output is passed into another command
- Never modify IDs (treat them as opaque values)
- Do not infer missing IDs — always read them from output
- Use returned identifiers exactly as provided by the CLI

### 1. Template Spec execution flow
> orchestration-input upload → inputFileId → template-spec precheck → confirm → template-spec run

### 2. Run lifecycle flow
> confirmed template-spec run / run submit → runId → run watch / result commands

### 3. Listing review flow
> confirm → listing publish → reviewRequestId → creator review get/withdraw

Confirm before any listing or review state change, including `listing publish`, `listing update`, `listing unlist`, `listing relist`, `listing withdraw`, and `creator review withdraw`.

### 4. Market (JSON input) flow
> market quote → confirm → market run → runTransactionId and runId → usage get / run watch

### 5. Market (Workbook) flow
> market workbook quote → confirm → market workbook run → runTransactionId and runId → usage get / run watch / result-workbook

### Notes

- Text output uses labels such as `input_file_id`; JSON output uses Product API field names such as `inputFileId`.

- For `template submit-file`, `template-spec submit-workbook`, `run submit`, `template-spec run`, `market run`, and `market workbook run`, pass an explicit `--client-request-id`, retain it with the request, and reuse it only when retrying the identical payload.
- Use a new ID if the payload changes.
- Do not blindly retry paid or remote-state-changing commands after an ambiguous failure — first check whether the original request succeeded.
