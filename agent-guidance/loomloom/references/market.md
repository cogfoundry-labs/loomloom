# Market And SkillBots

Use this reference for Market discovery and execution, SkillBots, Listings, Listing Versions, public input schemas, publishing, price changes, execution-version changes, listing lifecycle, and creator review.

For quotes, confirmation wording, currency, settlement, failure/cancellation, and creator earnings, also read `billing.md`. For exact command discovery and error recovery, also read `cli.md`.

## Contents

- [Core objects](#core-objects)
- [Unified catalog](#unified-catalog)
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

## Unified Catalog

The existing `market list` and `market show` commands use the current unified
public directory, containing official templates and Market SkillBots:

```bash
loomloom market list --keyword "writing" --sort-by usage --page 1 --page-size 15
loomloom market list --category <category-code>
loomloom market show <listing-id>
loomloom market show <template-slug> --source official
```

`source + resourceId` identifies a directory entry. `market show` defaults to
`--source market`; select `--source official` for an official template slug.
Route official execution to `template`, and Market execution to `market`.
Always inspect the selected resource's detail before preparing input.

`market list` uses one-based pages, with page sizes 1–15 and sort modes
`default`, `usage`, or `newest`. The Server ranks pinned entries first for every
sort. Search is a literal substring in name, description or category label, at
most 200 Unicode characters. Read `totalCount`, `page`, and `pageSize`;
`--page-token`, `--order-by` and `--limit` are removed from this command.
Creator listing and subscription pagination keep their own existing contracts.

Read enabled category codes and labels from `market list`'s `categoryOptions`
(or its text category table), including categories with no matching entries.
Categories are dynamic and single-select, not arbitrary tags or a compiled
list. If the directory fails, report the error; never invent category options.
`successfulTaskCount` counts successful tasks across versions and differs from
`trust.runCount` (successful run batches).

Public detail includes value, fields, instructions, samples, outputs, Skill
Package availability and `pricing`. List fees are `invocationFee`; detail fees
are `pricing.creatorFee`. A zero creator fee does not imply free model
execution. Subscription offers are public prices, not an execution quote or
personal entitlement. Preserve `subscriptionOffersStatus` (`available`,
`unavailable`, or `notApplicable`) and nullable `payPerUseEnabled`; null does not
mean payment is disabled. Subscription purchase waives creator call fees,
while model fees remain billed by actual usage. Use `market quote` or workbook
quote for the actual user's execution estimate.

## Buyer Flow

### Public input boundary

Build buyer input from `market show`'s current public `input.fields`:

- Use `fields[].name` for user-facing prompts.
- Use `fields[].key` as the submitted input key.
- Use `fields[].type` for the public type and `fields[].required` for required input.
- Present `fields[].options` as allowed choices without guessing or changing them.
- Use top-level `instructions` and `sampleRows[].values` as filling guidance and examples.

The CLI's quote/run validation and local Skill export still read the backend's
public execution schema contract (`inputSchemaSnapshot`). Unified detail is a
presentation projection and omits execution-specific fields such as defaults
and limits; it must not replace that validation contract. Commands handle
schema retrieval and current-version binding internally.

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

The Server decides the required trial evidence for the deployed review policy. First publication, execution changes and a new ZIP candidate use execution review; static profile or price updates may use static review without a new mandatory trial. Do not claim one successful run is sufficient for every deployed policy. Use normal currency units such as `--task-fixed-fee 0.5`; the CLI converts them to backend units. A successful request returns a `reviewRequestId` with a pending review state.

### Configure initial subscription prices

`--subscription=true` enables the automatic single-SkillBot subscription mode.
`--task-fixed-fee` is the creator's per-call fee, not a subscription price.
For a new Listing, provide initial prices in the same publication request:

```bash
loomloom listing publish <template-id> \
  --template-version-id <id> \
  --display-name <name> \
  --task-fixed-fee 0 \
  --pay-per-use=false \
  --subscription=true \
  --subscription-tiers-file <tiers.json>
```

Publication requires an explicit per-call fee even when pay-per-use is disabled;
the `0` above is that fee, not the subscription amount.

The file is a JSON array of `tierType`, `price`, and `enabled` objects:

```json
[
  {"tierType":"monthly","price":{"amount":"0.01","currency":"CNY"},"enabled":true},
  {"tierType":"yearly","price":{"amount":"0.02","currency":"CNY"},"enabled":false}
]
```

Use each supported type (`monthly`, `yearly`) at most once. Prices are CNY decimal
strings, non-negative with at most four decimal places; zero is legal and must
be explicit. `enabled` must be an explicit boolean. Invalid files fail locally
before any HTTP request; an explicit `null` or empty array is rejected rather
than silently interpreted as omitted. No flag means no initial tier field.
The Server rejects subscription mode without an enabled tier with HTTP 409,
code `subscription_tier_required`, including all-disabled input; CLI exit is
non-zero. Do not interpret a failed submission as a created or approved Listing.

`listing payment-settings` accepts the same file only to initialize a missing
or empty automatic bundle. Pass both mode booleans explicitly and the current
payment-settings revision. The Server rejects attempts to overwrite existing
prices. Use `creator bundle set-tier` and the current bundle revision for later
price or enabled-state changes. These two revision counters are distinct.
Do not retry a conflict with a guessed or automatically incremented revision.

Existing Listing publications cannot carry this initial-tier flag. Omit
unchanged payment flags when publishing a new version to preserve saved modes;
existing subscription configuration is still checked by the Server.

After publication, read `creator review get <review-request-id>` and the saved
automatic bundle (`creator bundle list`, then `creator bundle show <bundle-id>`).
Match `sourceListingId` and verify actual amounts, currency, enabled state, and
revision. A pending review or draft bundle is not approval or availability for
purchase. Initialization keeps the automatic bundle draft; authorized formal
approval or explicit bundle publication is a separate step. Profile/Skill
Package reviews must not be treated as formal subscription publication.

### Confirm duplicate names before publishing

When creating a new Listing (without `--listing-id`), `listing publish` checks the authenticated creator's Market SkillBots across all states, including pending review and unlisted entries. Private templates are excluded. Matching ignores outer whitespace and case, but preserves internal whitespace. Updating an existing Listing with `--listing-id` skips this check.

- No matching names: the CLI continues with the normal submission.
- Matching names: the CLI displays all matches, including their IDs, names, and states, and requires confirmation. Duplicate names are allowed.
- Check failure, including `Unimplemented`: the CLI warns that the result is unknown and requires confirmation. Do not describe this as no duplicates or as a failed template trial run.

In an interactive terminal, only `y` or `yes` confirms. Enter, refusal, EOF, or the independent five-minute confirmation timeout cancels submission. Name checking and subsequent submission each have their own HTTP timeout (default 30 seconds, controlled by `--timeout`).

Agent and other non-interactive calls do not receive a `y/N` prompt. On a duplicate-name warning or check failure, the CLI exits and asks for `--confirm-name-warning`. Explain the warning to the user and obtain explicit confirmation before retrying the same command with that flag. For a failed check, explicitly say that duplicate names could not be checked. Do not add the flag automatically to bypass a warning, and do not substitute a public Market search for the creator-scoped check.

Only after the user confirms:

```bash
loomloom listing publish <template-id> \
  --template-version-id <id> \
  --display-name <name> \
  --task-fixed-fee <amount> \
  --confirm-name-warning
```

The flag still performs the check and prints warnings; it allows submission to continue after either kind of warning. It does not bypass the Server's trial-evidence policy, other validation, or review. A successful submission is a pending review request, not an approved public listing. `Unimplemented` concerns the check service's RPC availability; verify the target environment and deployed services separately from trial-run validation.

### Submit category and public details

Read `market list` categoryOptions before selecting an enabled code. Add these optional
flags to `listing publish` or `listing update`:

```bash
loomloom listing update <listing-id> \
  --category <category-code> \
  --target-audience "Content teams" \
  --problem-solved "Create first drafts" \
  --scenario "News articles" --scenario "Emails" \
  --not-suitable-for "Legal advice" \
  --update-summary "Clarified audience and use cases"
```

`--category` submits one code for review. Omitting it preserves the effective
classification; an explicit empty code is rejected and cannot clear it. The
Server verifies that the code exists and is enabled. Approval applies the
candidate; rejection leaves the effective classification unchanged. There is
no creator `--tags`, `--pin-position`, category-label edit, or category-directory
management command.

The four public value fields map to `value.targetAudience`, `problemSolved`,
`scenarios`, and `notSuitableFor`. The CLI merges only explicit value flags with
the current owner-visible value so omitted siblings survive. `--scenario` is
repeatable and keeps commas within a scenario; `--scenario ""` clears the list.
Explicit empty text flags clear their corresponding value field. Re-publishing
an existing Listing preserves its current value unless a value flag changes it.
Name, description and task fee retain their existing publish flag behavior.

Both submission paths replace an existing pending request with a new one;
inspect pending review state before resubmitting. Submission does not approve
or list the candidate. `listing show` exposes the effective category and
unlisting reason. At the audited backend revision, creator review list/get
omit the pending category and request type from HTTP responses; do not use the
effective Listing category as proof of the submitted candidate. Report that
readback limitation, and verify effective classification after approval.

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

For a Listing under review, distinguish the pending review package from the public package. `skillPackageReview.pending` means the ZIP is bound to that review. `skillPackage.available=false` with `unavailableReason=listing_not_listed` only means it is not publicly downloadable before approval; it is not evidence of a failed upload or binding. An empty Listing `packageHash` is also not a Skill Package binding signal.

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

`creator earnings` reads actual Billing-posted income and defaults to both pay-per-use settlement and released subscription income. Use `--source pay-per-use` or `--source subscription` to narrow it. It is not a per-call order list. Use `creator transactions` when transaction-level context is required.

All publishing, profile changes, price changes, execution-version changes, unlisting, relisting, and review withdrawals are persistent remote changes and require explicit confirmation before execution.
