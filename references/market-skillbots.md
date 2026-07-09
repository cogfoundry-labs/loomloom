# Market / SkillBot

The LoomLoom Market lets creators publish a private template version as a paid **SkillBot**. CogFoundry payment and transaction capabilities are coming soon. For Market-related paid workflows, check availability, balance, and transaction status in the [ShengSuanYun Console](https://console.shengsuanyun.com/user/recharge).

See the main [README](../README.md) for the Template → Private Template → SkillBot taxonomy, and [`cli-reference.md`](cli-reference.md#market--buyer) for the full command list.

Publishing a SkillBot does not expose or convert the underlying private template into a public object. Instead, the system creates an immutable Listing Version execution snapshot from the selected template version.

Once published, the SkillBot is frozen at that snapshot. Subsequent edits to the private template, or creation of new template versions, do not affect the live SkillBot. To update a live SkillBot, a new template version must be submitted and re-published for review.

## Buyer: Discover and Run a SkillBot

```bash
# 1. Browse and inspect SkillBots
loomloom market list --keyword "tweet"
loomloom market show <listing-id>

# 2A. JSON input: estimate cost from public input rows
loomloom market quote <listing-id> --input-file ./request.json

# 3A. JSON input: review the quote, confirm, then execute (paid)
loomloom market run <listing-id> --input-file ./request.json --confirm --client-request-id <stable-id>

# 2B. Workbook input: download, fill, validate, and quote
loomloom market workbook download <listing-id> --output-file ./market-input.xlsx
loomloom market workbook validate <listing-id> --file ./market-input.xlsx
loomloom market workbook quote <listing-id> --file ./market-input.xlsx

# 3B. Workbook input: review the quote, confirm, then execute (paid)
loomloom market workbook run <listing-id> --file ./market-input.xlsx --confirm --client-request-id <stable-id>

# 4. Review your own usage and download results
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

## Install a Template as a Local Agent Skill

Local Agent Skills are usage wrappers for existing LoomLoom templates. They teach Codex, Claude Code, or OpenClaw when to use a specific template, what inputs to collect, how to quote/precheck, and how to submit only after explicit confirmation. They do **not** copy server-side execution logic, hidden prompts, model settings, credentials, or Market internals.

```bash
# Preview installation data for a confirmation card
loomloom skill install market <listing-id> \
  --agent codex \
  --output-dir /path/to/loomloom-one-skill-dir \
  --dry-run \
  --output json

# Install a Market SkillBot wrapper
loomloom skill install market <listing-id> \
  --agent codex \
  --output-dir /path/to/loomloom-one-skill-dir

# Install a private template-version wrapper
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

Market Skill wrappers bind to the Listing. The installed listing version is recorded only for traceability; every execution must read the current public Listing and use `market quote`/`run` or `market workbook quote`/`run`. Private template wrappers bind to the exact `template_id` + `version_id` and do not enter the Market path.

## Creator: Publish and Manage a SkillBot

```bash
# 1. Confirm, then publish a template version for review (it must already have one successful run)
loomloom listing publish <template-id> \
  --template-version-id <version-id> \
  --display-name "My SkillBot" \
  --task-fixed-fee-t 1000000

# Confirm, then submit a new version for an existing listing
loomloom listing publish <template-id> \
  --listing-id <listing-id> \
  --template-version-id <new-version-id> \
  --display-name "My SkillBot" \
  --task-fixed-fee-t 1000000

# 2. Track listings and reviews
loomloom listing list
loomloom listing versions <listing-id>
loomloom creator review list

# 3. Confirm, then manage sale status and public profile
loomloom listing unlist <listing-id>
loomloom listing relist <listing-id>
loomloom listing update <listing-id> --description "Updated description"
loomloom listing withdraw <listing-id>
loomloom creator review withdraw <review-request-id>

# 4. Review income
loomloom creator earnings
loomloom creator transactions
```

## Notes

- `market run` creates a real paid run; agents should run `market quote` first and ask for explicit confirmation before executing.
- For Market workbook runs, validate and quote the workbook before execution, then download the result with `run result-workbook`.
- `listing publish` and `listing update` submit changes for review; they do not take effect until approved.
- `listing publish --listing-id <listing-id>` submits a new version for the existing listing. The currently published version stays active until the new review is approved.
- `listing update`, `listing unlist`, `listing relist`, and review withdrawal change remote state. Agents should summarize the action and ask for explicit confirmation before invoking them.
- For Market SkillBots, `market quote` estimates the buyer payable amount before execution. The platform takes 10% from each call fee; creator net earnings are 90% of the call fee.
- Reuse the same `--client-request-id` only when retrying the identical paid Market payload. Use a new ID when any input changes.
- Workbook `content` is sent as base64 inside JSON requests; do not print the full base64. `accessUrl` values in result rows are temporary signed URLs; do not put them in long-lived logs or docs.
- Monetary `*FeeT` / `*AmountT` / `*PayableT` values are in API units where 10,000,000 units equal 1 currency unit.
- If a response does not include `currency`, do not guess CNY or USD; preserve the raw `*T` value and show the currency as unknown.
