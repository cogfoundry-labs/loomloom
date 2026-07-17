# Market And SkillBots

Use this reference for Market discovery and execution, SkillBots, Listings, Listing Versions, public input schemas, publishing, price changes, execution-version changes, listing lifecycle, and creator review.

For quotes, confirmation wording, currency, settlement, failure/cancellation, and creator earnings, also read `billing.md`. For exact command discovery and error recovery, also read `cli.md`.

## Contents

- [Core objects](#core-objects)
- [Buyer flow](#buyer-flow)
- [Creator flow](#creator-flow)

## Core Objects

```text
Private template
└─ Private template version
   └─ Submitted for Market review
      └─ Listing Version (immutable publish snapshot)
         └─ Approved and executable by buyers as a SkillBot
```

- **SkillBot** is the public, paid, executable form of an approved private template version.
- **Listing** is the Market shelf object that buyers discover and call.
- **Listing Version** is an immutable execution snapshot copied from a private template version at publish time.
- There is no separate "public template" resource. In user-facing language, "public Market template" may refer to a SkillBot, but internally it remains a Listing/SkillBot.
- Later changes to a private template do not automatically change a live SkillBot.

Buyers call the Listing, not a version they hold. When the order is created, the service resolves the current sellable Listing Version and locks that version and its price for the order. A quote does not require the agent to pass a version into execution.

Never bypass Market by directly running the underlying private template version.

## Buyer Flow

Discover and inspect:

```bash
loomloom market list
loomloom market show <listing-id>
```

Always inspect `market show` before preparing input.

### Public input boundary

Build buyer input only from `inputSchemaSnapshot`:

- Use `fields[].label` for user-facing prompts.
- Use `fields[].key` as the submitted input key.
- Use `fields[].value_type` for type validation.
- Use `fields[].required` for required-field validation.
- Use `sample_rows` as examples.

Do not reveal, reconstruct, infer, or submit hidden `taskInputs`, `workflowDefinition`, `templateSpec`, internal prompts, internal step IDs, or private mappings.

For Market JSON execution, build public `inputRows`:

```json
{
  "inputRows": [
    {
      "prompt": "write a launch tweet"
    }
  ]
}
```

Do not show raw request JSON unless the user explicitly asks for JSON/API details.

### JSON flow

1. `loomloom market show <listing-id>`
2. Collect input through an Excel-style experience or the user's natural-language values.
3. Build public `inputRows` internally.
4. `loomloom market quote <listing-id> --input-file <request.json>`
5. Show the Market confirmation template and wait for explicit confirmation.
6. `loomloom market run <listing-id> --input-file <request.json> --confirm --client-request-id <id>`

### Workbook flow

1. `loomloom market show <listing-id>`
2. `loomloom market workbook download <listing-id> --output-file <xlsx>`
3. Let the user fill or approve the workbook.
4. `loomloom market workbook validate <listing-id> --file <xlsx>`
5. `loomloom market workbook quote <listing-id> --file <xlsx>`
6. Show the Market confirmation template and wait for explicit confirmation.
7. `loomloom market workbook run <listing-id> --file <xlsx> --confirm --client-request-id <id>`

Use `usage list` and `usage get <run-transaction-id>` for the buyer's own calls and settlement.

## Creator Flow

Publish a private template version:

```bash
loomloom listing publish <template-id> \
  --template-version-id <id> \
  --display-name <name> \
  --task-fixed-fee <amount>
```

The private template version must already have one successful run. Use normal currency units such as `--task-fixed-fee 0.5`; the CLI converts them to backend units. A successful request returns a `reviewRequestId` with a pending review state.

### Change only the price

Publish to the same Listing with:

- `--listing-id <listing-id>`
- the currently published `--template-version-id`
- the current display name
- the new `--task-fixed-fee`

This submits a price change without requiring a new private template version.

### Change the execution version

Publish to the same Listing with the new private `--template-version-id`. The currently published Listing Version remains active until the new version is approved.

### Manage the Listing

Use:

```bash
loomloom listing list
loomloom listing show <listing-id>
loomloom listing versions <listing-id>
loomloom listing update <listing-id>
loomloom listing unlist <listing-id>
loomloom listing relist <listing-id>
loomloom listing withdraw <listing-id>
```

- `listing update` changes the public profile for review; it does not change price or execution version.
- `unlist` stops new executions.
- `relist` resumes listing when allowed.
- `listing withdraw` withdraws the single pending review for that Listing. If none exists, stop. If multiple are reported, inspect creator review requests and withdraw the intended request explicitly.

Review commands:

```bash
loomloom creator review list
loomloom creator review get <review-request-id>
loomloom creator review withdraw <review-request-id>
```

Creator financial commands:

```bash
loomloom creator earnings
loomloom creator transactions
```

All publishing, profile changes, price changes, execution-version changes, unlisting, relisting, and review withdrawals are persistent remote changes and require explicit confirmation before execution.
