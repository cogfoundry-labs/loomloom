# Billing

Use this reference whenever a task involves precheck, quote, balance, currency, paid execution, confirmation, creator fees, model/API cost, pre-authorization, settlement, failure, cancellation, partial completion, usage, or creator earnings.

## Contents

- [Preparation and execution consent](#preparation-and-execution-consent)
- [Fee and currency rules](#fee-and-currency-rules)
- [Confirmation content](#confirmation-content)
- [Confirmation templates](#confirmation-templates)
- [Market settlement](#market-settlement)
- [Persistent remote changes](#persistent-remote-changes)
- [Creator earnings](#creator-earnings)

## Preparation And Execution Consent

Installation, configuration, discovery, downloads, uploads, workbook filling, validation, quote, precheck, schema inspection, model lookup, artifact listing, read-only usage/earnings queries, and result backfill are preparation or read-only actions. They do not authorize a hosted run.

Treat the interaction as:

1. **Preparation**: prepare, validate, quote, and precheck only.
2. **Execution requested**: still prepare and show the current fee estimate; do not run yet.
3. **Confirmed execution**: run only after the user explicitly confirms the estimate in the current conversation.

Every run requires a fresh confirmation unless the active LoomLoom Skill Test Execution Mode explicitly covers that run. Confirmation for a different input, file, template, version, Listing, or conversation is not reusable.

Test Execution Mode is a conversation-scoped Agent prompt authorization, not a server-side billing control. It may skip repeated confirmation only within the user-approved Server, execution scope, task limits, per-run and total cost limits, and expiry. A missing, zero, or currency-unknown precheck estimate is never eligible for automatic submission: obtain a fresh confirmation before submission. If any other condition changes or is exceeded, also obtain a fresh confirmation before submission.

If input changes after validation, quote/precheck, or confirmation, validate and estimate again, show the new estimate, and obtain a new confirmation. In Chinese, say:

```text
输入内容在确认后发生变化，需要重新预估并确认。
```

Use natural confirmation wording. In English, use `Reply: Confirm`. In other languages, use the natural localized equivalent. Do not ask ordinary users to reply with a raw CLI phrase such as `confirm submit`.

Create a new client request ID for each newly confirmed execution. Reuse the original ID only for an identical retry of the same confirmed request after an ambiguous failure. A changed input, re-estimate, or new confirmation requires a new ID, even when values later happen to match an earlier request.

## Fee And Currency Rules

- For official and private precheck, use `estimatedTotalCostT` and the currency returned by the service.
- For Market quote, use `estimatedBuyerPayableT` as the estimated pre-authorization.
- Compute the creator call fee from `taskCount × taskFixedFeeT` only when both fields are returned.
- Do not invent a missing fee field.
- If a Market quote does not separately return model/API cost, say that it was not returned separately. Do not infer zero, subtract other fields to manufacture a value, or claim it is included in another field.
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
- estimated model/API cost or buyer payable amount
- currency
- available balance and sufficiency when returned
- a clear confirmation request

Show the buyer-facing Market fee summary only. Do not show platform commission, creator net earnings, or revenue-sharing details to a buyer.

Do not show raw CLI commands, raw JSON bodies, generated request filenames, or terse developer fields unless the user explicitly asks for CLI/API details.

## Confirmation Templates

### Official template

```text
This will execute an official template. Please confirm the fee before execution.

Template: <template_display_name>
Call type: official template
Template ID: <template_id>

Input:
- Source: <input_source>
- Task count: <task_count> task(s)

Fee estimate:
- Estimated model/API cost: <estimated_model_api_cost>
- Estimated pre-authorization: <estimated_model_api_cost>
- Available balance: <available_balance_if_returned>

Final billing rules:
- Final charge is based on actual model/API usage
- A failed or partially completed run may still incur model/API cost already consumed
- Unused pre-authorization is released or adjusted
- Official templates do not create creator call fees, platform commissions, or Market revenue sharing

Please confirm whether to execute.
Reply: Confirm
```

### Private template

```text
This will execute a private template. Please confirm the fee before execution.

Template: <template_display_name>
Call type: private template
Template ID: <template_id>
Template version: <version_id>

Input:
- Task count: <task_count> task(s)

Fee estimate:
- Estimated model/API cost: <estimated_model_api_cost>
- Estimated pre-authorization: <estimated_model_api_cost>

Final billing rules:
- Final charge = actual model/API cost
- A failed or partially completed run may still incur model/API cost already consumed
- Model/API cost is settled by actual usage; unused pre-authorization is released or adjusted
- Private templates do not create creator call fees, platform commissions, or Market revenue sharing

Please confirm whether to execute.
Reply: Confirm
```

### Public Market template / SkillBot

```text
This will make a paid call to a public Market template. Please confirm the fee before execution.

Template: <template_display_name>
Call type: public Market template
Listing ID: <listing_id>
Version selection: the service will use the current sellable Listing Version when creating the order

Input:
- Task count: <task_count> task(s)
- Pricing rule: creator call fee is quoted per task

Fee estimate:
- Creator call fee: <creator_call_fee> (<task_count> task(s) x <task_fixed_fee>)
- Estimated model/API cost: <estimated_model_api_cost_or_not_returned_separately>
- Estimated pre-authorization: <estimated_buyer_payable>

Final billing rules:
- The Listing Version and price are locked when the order is created
- For a completed run, final charge = applicable creator call fee + actual model/API cost
- For a failed or cancelled run, the buyer's final charge is zero, the reserved amount is released, and the creator receives no earning
- For a partially failed or partially cancelled run, the current implementation leaves the reserved amount and creator settlement pending resolution

Please confirm whether to execute.
Reply: Confirm
```

## Market Settlement

Interpret Market settlement from the returned transaction state:

- **Completed**: settle the buyer's actual model/API cost plus the applicable creator call fee, and credit the creator.
- **Failed or cancelled**: buyer final charge is zero, reserved amount is released, and creator earning is zero.
- **Partially failed or partially cancelled**: the current implementation does not capture or release the reserved amount and does not credit the creator; the transaction remains pending resolution.

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

"Describe the exact action" means explain what will change in business language. It does not require displaying the raw CLI command.

## Creator Earnings

Use `creator earnings` for the overview and `creator transactions` for recent line items. Do not show raw JSON unless requested.

Present:

```text
Here is the earnings overview for your public Market template:

Template: <template_display_name>

Cumulative:
- Calls: <total_call_count>
- Creator call fee: <gross_creator_call_fee>
- Platform commission: <platform_fee>
- Creator net receivable: <creator_net_receivable>

Settlement:
- Settled: <settled_amount>
- Pending: <pending_amount>
- Failed: <failed_amount>

Exception:
<failure count and explanation, or no settlement exceptions>

Latest 5:
1. Run <run_id>, net <amount>, status: <settled|failed|pending>
```

Show at most five recent transactions by default. If a response field is absent, omit the line or state that it was not returned. Never fabricate amounts, counts, run IDs, or settlement states.
