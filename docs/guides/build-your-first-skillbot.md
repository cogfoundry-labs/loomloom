# Build your first SkillBot

loomloom allows developers to publish a private template version as a SkillBot.

A published SkillBot is created from an immutable execution snapshot of the selected private template version. The original private template remains private and continues to evolve independently.

After publishing:
- Changes to the private template do not affect the existing SkillBot.
- New template versions require a new publishing process before they become available as an updated SkillBot.
- Each SkillBot version remains tied to the exact template version used at publication time.


## Publish and manage your SkillBot

Developers can publish a private template version as a SkillBot, manage listings, track reviews, and monitor earnings.

```bash
# 1. Publish a template version for review (requires at least one successful run)
loomloom listing publish <template-id> \
  --template-version-id <version-id> \
  --display-name "My SkillBot" \
  --task-fixed-fee 0.1

# Submit a new version for an existing SkillBot listing
loomloom listing publish <template-id> \
  --listing-id <listing-id> \
  --template-version-id <new-version-id> \
  --display-name "My SkillBot" \
  --task-fixed-fee 0.1

# 2. Track listings and review status
loomloom listing list
loomloom listing versions <listing-id>
loomloom creator review list

# 3. Manage listing status and public profile
loomloom listing unlist <listing-id>
loomloom listing relist <listing-id>
loomloom listing update <listing-id> --description "Updated description"
loomloom listing withdraw <listing-id>
loomloom creator review withdraw <review-request-id>

# 4. Review earnings and transactions
loomloom creator earnings
loomloom creator transactions
```

## Use a published SkillBot

```bash
# 1. Browse and inspect available SkillBots
loomloom market list --keyword "tweet"
loomloom market show <listing-id>

# 2A. JSON input: estimate cost from input data
loomloom market quote <listing-id> --input-file ./request.json

# 3A. JSON input: review the quote, confirm, and run
loomloom market run <listing-id> --input-file ./request.json --confirm --client-request-id <stable-id>

# 2B. Workbook input: download, fill, validate, and estimate cost
loomloom market workbook download <listing-id> --output-file ./market-input.xlsx
loomloom market workbook validate <listing-id> --file ./market-input.xlsx
loomloom market workbook quote <listing-id> --file ./market-input.xlsx

# 3B. Workbook input: review the quote, confirm, and run
loomloom market workbook run <listing-id> --file ./market-input.xlsx --confirm --client-request-id <stable-id>

# 4. Review usage and download results
loomloom usage list
loomloom usage get <run-transaction-id>
loomloom run result-workbook <run-id> --output-file ./market-result.xlsx
```

Example `request.json`:

```json
{
  "inputRows": [
    {
      "prompt": "write a launch tweet"
    }
  ]
}
```

Use `market show` to understand public fields and examples before building JSON input. Show users `inputSchemaSnapshot.fields[].label`, submit `inputRows` with `inputSchemaSnapshot.fields[].key`, and treat `fields[].required` as required input. Do not send `taskInputs`, `workflowDefinition`, `templateSpec`, or other hidden Core / TemplateSpec structures to Market buyer execution endpoints.

## Install a template or SkillBot as a local Agent Skill

Local Agent Skills are lightweight usage wrappers that teach Codex, Claude Code, or OpenClaw how to use a specific template or SkillBot.

They define when to use it, what inputs to collect, how to estimate cost, and how to submit executions only after explicit confirmation. They do **not** include server-side execution logic, hidden prompts, model settings, credentials, or Market internals.


```bash
# Preview installation data before confirmation
loomloom skill install market <listing-id> \
  --agent codex \
  --output-dir /path/to/loomloom-one-skill-dir \
  --dry-run \
  --output json

# Install a SkillBot wrapper
loomloom skill install market <listing-id> \
  --agent codex \
  --output-dir /path/to/loomloom-one-skill-dir

# Install a private template wrapper
loomloom skill install template-spec <template-id> <version-id> \
  --agent codex \
  --output-dir /path/to/loomloom-one-skill-dir

# Preview uninstalling a local Agent Skill wrapper
loomloom skill uninstall \
  --dir /path/to/loomloom-one-skill-dir \
  --dry-run \
  --output json

# Uninstall a local Agent Skill wrapper
loomloom skill uninstall \
  --dir /path/to/loomloom-one-skill-dir
```

Notes:
- SkillBot wrappers are linked to a listed SkillBot. The installed Listing version is recorded for traceability only; executions always use the current public Listing through `market quote`/`run` or `market workbook quote`/`run`.
- Private template wrappers are linked to the exact `template_id` + `version_id` and do not enter the Market path.

## Notes

- `market run` creates a real paid run; agents should run `market quote` first and ask for explicit confirmation before executing.
- For Market workbook runs, validate and quote the workbook before execution, then download the result with `run result-workbook`.
- `listing publish` and `listing update` submit changes for review; they do not take effect until approved.
- `listing publish --listing-id <listing-id>` submits a new version for the existing listing. The currently published version stays active until the new review is approved.
- `listing update`, `listing unlist`, `listing relist`, and review withdrawal change remote state. Agents should summarize the action and ask for explicit confirmation before invoking them.
- For Market SkillBots, `market quote` estimates the buyer payable amount before execution. The platform takes 10% from each call fee; creator net earnings are 90% of the call fee.
- Reuse the same `--client-request-id` only when retrying the identical paid Market payload. Use a new ID when any input changes.
- Workbook `content` is sent as Base64 inside JSON requests; do not print the full Base64. `accessUrl` values in result rows are temporary signed URLs; do not put them in long-lived logs or docs.
- User-facing monetary inputs and default text output use normal currency units, for example `--task-fixed-fee 0.1` and `CNY 0.1000000`. JSON output preserves raw `*FeeT` / `*AmountT` / `*PayableT` API fields where 10,000,000 units equal 1 currency unit.
- If a response does not include `currency`, do not guess CNY or USD; show the currency as unknown and preserve the raw `*T` value in that display.
