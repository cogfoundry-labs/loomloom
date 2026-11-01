# Billing

Use this reference whenever a task involves precheck, quote, balance, currency, paid execution, confirmation, creator fees, model/API cost, pre-authorization, settlement, failure, cancellation, partial completion, usage, or creator earnings.

## Contents

- [Preparation and execution consent](#preparation-and-execution-consent)
- [Fee and currency rules](#fee-and-currency-rules)
- [Confirmation content](#confirmation-content)
- [Confirmation templates](#confirmation-templates)
- [Actual fees after execution](#actual-fees-after-execution)
- [Market settlement](#market-settlement)
- [Persistent remote changes](#persistent-remote-changes)
- [Creator earnings](#creator-earnings)

## Preparation And Execution Consent

Installation, configuration, discovery, downloads, uploads, workbook filling, validation, quote, precheck, schema inspection, model lookup, artifact listing, read-only usage/earnings queries, and result backfill are preparation or read-only actions. They do not authorize a hosted run.

Treat the interaction as:

1. **Preparation**: prepare, validate, quote, and precheck only.
2. **Execution requested**: prepare for confirmation; do not run yet.
3. **Confirmed execution**: run only after the user explicitly confirms the execution in the current conversation.

Every run requires a fresh confirmation unless the active LoomLoom Skill Test Execution Mode explicitly covers that run. Confirmation for a different input, file, template, version, Listing, or conversation is not reusable.

Test Execution Mode is a conversation-scoped Agent prompt authorization, not a server-side billing control. It may skip repeated confirmation only within the user-approved Server, execution scope, task limits, per-run and total creator call fee limits, and expiry. If any other condition changes or is exceeded, also obtain a fresh confirmation before submission.

If input changes after validation, quote/precheck, or confirmation, validate and quote/precheck again and obtain a new confirmation. Tell the user in their language that the changed input must be validated and reconfirmed.

Use natural confirmation wording. In English, use `Reply: Confirm`. In other languages, use the natural localized equivalent. Do not ask ordinary users to reply with a raw CLI phrase such as `confirm submit`.

Create a new client request ID for each newly confirmed execution. Reuse the original ID only for an identical retry of the same confirmed request after an ambiguous failure. A changed input or new confirmation requires a new ID, even when values later happen to match an earlier request.

## Fee And Currency Rules

`loomloom balance` displays one LoomLoom available balance. Wallet assets, applicable vouchers and pending charges are internal calculation inputs, not user-facing CLI fields. Gateway still makes the final request-admission decision.

- Use the quote's `effectiveCreatorFee` as the total call fee; it accounts for the applicable payment mode, including subscription coverage. Pass its amount and currency to `market check-balance` before Market confirmation.
- Do not invent a missing fee field.
- Preserve the server-provided currency. Do not locally convert USD and CNY.

All `*FeeT`, `*CostT`, `*AmountT`, and `*PayableT` values use backend units:

```text
10,000,000 backend units = 1 currency unit
```

Default text output uses normal currency units. `--output json` preserves raw backend fields.

When currency is absent or the CLI reports `(currency unknown)`, state that the currency is unknown and preserve the raw T value. Do not display a bare number and do not guess CNY or USD.

## Confirmation Content

Before a hosted run, show:

- template or Listing display name
- template or Listing ID
- template type
- private `version_id`, when applicable
- for Market, that the service selects the current sellable Listing Version when creating the order
- input source
- row count or task size
- action to be performed
- currency
- creator call fee, model fee pending settlement, and current available balance in the three-line format below
- a clear confirmation request

Show the buyer-facing Market fee summary only. Do not show platform commission, creator net earnings, or revenue-sharing details to a buyer.

Do not show raw CLI commands, raw JSON bodies, generated request filenames, or terse developer fields unless the user explicitly asks for CLI/API details.

## Confirmation Templates

Present these templates in the user's language. For official/private templates, use the precheck's available balance; if absent, query `loomloom balance` before confirmation. For Market, use the call fee and balance returned by `market check-balance` after a successful check.

### Official template

```text
This will execute an official template. Please confirm the execution details.

Template: <template_display_name>
Call type: official template
Template ID: <template_id>
Template version: <version>

Input:
- Source: <input_source>
- Task count: <task_count> task(s)

1. Creator call fee: None
2. Model fee: Pending settlement; historical model fee per task: <minModelFee>–<maxModelFee> (reference only; actual usage applies)
3. Current balance: <available_balance>

Final billing rules:
- Final charge is based on actual model/API usage
- A failed or partially completed run may still incur model/API cost already consumed
- Unused pre-authorization is released or adjusted
- Official templates do not create creator call fees, platform commissions, or Market revenue sharing

Please confirm whether to execute.
Reply: Confirm
```

Use the returned official-template range when `availability` is `available`. For `no_data` or a failed lookup, show "Model fee: Pending settlement; no historical data available for reference" and continue confirmation.

### Private template

```text
This will execute a private template. Please confirm the execution details.

Template: <template_display_name>
Call type: private template
Template ID: <template_id>
Template version: <version_id>

Input:
- Task count: <task_count> task(s)

1. Creator call fee: None
2. Model fee: Pending settlement
3. Current balance: <available_balance>

Final billing rules:
- Final charge = actual model/API cost
- A failed or partially completed run may still incur model/API cost already consumed
- Model/API cost is settled by actual usage; unused pre-authorization is released or adjusted
- Private templates do not create creator call fees, platform commissions, or Market revenue sharing

Please confirm whether to execute.
Reply: Confirm
```

### Public Market template / SkillApp

```text
This will make a paid call to a public Market template. Please confirm the execution details.

Template: <template_display_name>
Call type: public Market template
Listing ID: <listing_id>
Version selection: the service will use the current sellable Listing Version when creating the order

Input:
- Task count: <task_count> task(s)
- Pricing rule: creator call fee is quoted per task

1. Creator call fee: <creator_call_fee> (<task_count> task(s) x <task_fixed_fee>)
2. Model fee: Pending settlement; historical actual model fee per task for this SkillApp: <minModelFee>–<maxModelFee> (reference only; billed by actual usage, which may exceed this range)
3. Current balance: <available_balance>

Final billing rules:
- The Listing Version and price are locked when the order is created
- For a completed run, final charge = applicable creator call fee + actual model/API cost
- Use the returned transaction amounts for failed, cancelled, or partially completed runs; a run's status alone does not determine its final charge

Please confirm whether to execute.
Reply: Confirm
```

For Market, take `effectiveCreatorFee` from the output of `market quote` or `market workbook quote`. Include the task-count calculation only when it matches that total. When subscription coverage applies, show the returned applicable call fee and explain the coverage.

Read the historical range with `market model-cost-range <listing-id> --listing-version-id <quote.listingVersionId>`. Use `minModelFee` and `maxModelFee` when `availability` is `available`. For `no_data` or a failed lookup, replace line 2 with "Model fee: Pending settlement; no historical data available for reference" and continue confirmation. If the quoted version changes, query that version again. Historical fees are per-task reference only; balance checks and Test Execution Mode limits use creator call fees.

## Actual Fees After Execution

For Market, use `usage get <run-transaction-id>` (`usage list` for history) and present in the user's language:

```text
Actual amount: <finalBuyerPayable>
Creator call fee: <actualCreatorFee>
Model fee: <actualExecutionCost>
```

For `feeStatus: pending`, show "Pending settlement". For `unavailable`, show known amounts and mark missing ones as unavailable; never substitute zero.

For official/private templates, use the actual cost from `run get` or the final `run watch` result for both total and model fee; creator call fee is "None". If absent, show "Pending settlement".

## Market Settlement

Interpret Market settlement from the returned transaction state:

- **Succeeded, failed, or cancelled transaction**: use the actual amounts returned by the usage record. Partial completion can still incur model and creator call fees; do not infer zero from cancellation alone.
- **Transaction still running or awaiting resolution**: fees are pending settlement, even if the run has finished.

Do not claim that a human or manual review process exists unless the service explicitly reports one.

## Persistent Remote Changes

Before executing any of these, describe the exact business action and ask for explicit confirmation:

- `template-spec create`
- `template-spec create-version`
- `listing publish`, including price-only and execution-version updates
- `listing update`
- `listing unlist`
- `listing relist`
- `listing withdraw`
- `creator review withdraw`
- `listing update-skill-package`
- `creator bundle create`, `update`, `add-item`, `set-tier`, `publish`, and `unlist`
- `market subscribe --confirm`, `subscription buy --confirm`, and `subscription renew --confirm`

"Describe the exact action" means explain what will change in business language. It does not require displaying the raw CLI command.

For subscription purchase or renewal, show the selected bundle, tier, current
price/currency, returned duration, and bundle revision before confirmation.
The preview without `--confirm` reads public pricing; it is not an execution
quote. Subscription payment waives applicable creator call fees during the
entitlement period; model/API usage remains billed separately. Quote and
confirm any later hosted execution through the normal execution flow.

## Creator Earnings

Use `creator earnings` for actual posted income and `creator transactions` for transaction facts. `creator earnings` defaults to `--source all`; `--source pay-per-use` selects posted settlement events and `--source subscription` selects subscription income only after release. A pay-per-use income event can aggregate a settlement window, so it is not necessarily one row per call. Locked subscription accruals are not posted income. Do not show raw JSON unless requested.

Present:

```text
Here are your posted creator earnings:

Total posted: <total_amount>
Income events: <total_count>

Latest 5:
1. <occurred_at>, <pay-per-use|subscription>, reference <source_event_id>, <amount>, status posted
```

Show at most five recent events unless the user requests more. If a response field is absent, omit the line or state that it was not returned. Never infer call count from income-event count, and never fabricate amounts, counts, run IDs, or settlement states.
