---
name: loomloom
description: Use this skill when the user mentions LoomLoom, loomloom, batch generation, batch processing, batch tasks, batch execution, template execution, template submission, Excel submission, template download, Excel validation, run results, result download, run id, task status, execution status, official templates, private templates, TemplateSpec, Excel/workbook templates, Market SkillBots, template market, public templates, SkillBot, listing, unlisting, relisting, price changes, fixed task fees, publishing to the Market, review submission, review status, creator earnings, template earnings, usage, orders, call records, installing LoomLoom, doctor, token, server, or when the user clearly wants multi-row input to produce batch outputs.
---

# loomloom

LoomLoom is a template-based AI workflow platform for structured, repeatable AI work. Users can run official templates, create and version private templates, publish private template versions as paid Market SkillBots, buy and run Market SkillBots, manage creator listings and pricing, install templates as local Agent Skills, and retrieve result workbooks or artifacts.

Use this skill when the user wants help with batch or table-shaped generation, template execution, private template creation, Market SkillBot publishing or running, Excel/workbook-based input, local Agent Skill installation, run status, result inspection, usage records, creator earnings, or artifact download.

Do not use this skill for ordinary one-off writing, rewriting, or chat requests unless the user asks for LoomLoom, templates, Excel/workbook input, Market SkillBots, run status, or batch/table-shaped processing.

## When To Use

- The user wants to generate copy, images, or videos in batches, including text-to-image or text-to-image-to-video batch workflows.
- The user wants to list templates, download official Excel templates, validate Excel files, submit Excel files, check run status, download result workbooks, or download artifacts.
- The user wants to create a custom template, especially from a natural-language reusable workflow description.
- The user wants to discover, quote, or run a published Market SkillBot.
- The user wants to publish a template to the Market, manage their listings, or check creator earnings and review requests.
- The task can be represented as structured rows rather than a one-off chat answer.
- The user is willing to use developer tooling or have an agent call the CLI.

## When Not To Use

- The user only needs one immediate generation, not batch processing.
- The request is still exploratory and cannot be shaped into row-level input or a template.
- `LOOMLOOM_TOKEN` is not configured and the user does not want to handle environment setup.

## Core Objects and Command Selection

Do not treat official templates, private templates, and SkillBots as three unrelated workflow systems. Their relationship is:

```text
Official template ── platform-maintained, executed directly

Private template ── created and maintained by a user through TemplateSpec
   └─ Private template version
        └─ Submitted to the Market for review
             └─ Listing Version (immutable publish snapshot)
                  └─ After approval, executable by buyers as a SkillBot
```

Terminology and selection rules:

- **Template** is the umbrella term.
- **Official template** is maintained by the platform. Use the `template` command group to discover and execute official templates.
- **Custom template** describes how something is created; the result is the user's **private template**. Use the `template-spec` command group to author, inspect versions, and directly execute private templates.
- **SkillBot** is the public, paid, executable form of a private template version after it passes Market review. Buyers use the `market` command group.
- **Listing** is the Market shelf object for a SkillBot. Creators use the `listing` and `creator review` command groups to publish and manage SkillBots.
- **Listing Version** is an immutable execution snapshot copied from a private template version at publish time. Later changes to the private template do not automatically update a live SkillBot.
- There is currently no separate "public template" resource. Do not use that term to refer to official templates or Market SkillBots.
- `asset list` is only an aggregated view of executable assets ("my private templates + Market SkillBots"); it is not a new kind of template and does not include official templates.

Choose the entry point by user intent:

- "what platform templates are there", "execute with an official Excel template" → `template list/schema/download/...`
- "create my workflow", "modify my template", "run my template version" → `template-spec ...`
- "publish my template as a SkillBot", "update or unlist my SkillBot" → `listing ...`
- "find a SkillBot and run it for a fee" → `market ...`
- When the user only says "template" and the context cannot tell official from private, clarify first; do not guess.

## Command Flow

0. Install via the GitHub install script by default. On macOS/Linux the default install uses Homebrew. Unless the user explicitly asks not to use Homebrew, do not add `--no-brew`.
   If the user asks for an internal/beta CLI, explicitly install a prerelease channel instead of defaulting to stable:
   `curl -fsSL https://raw.githubusercontent.com/Cogfoundry-ai/loomloom/main/install.sh | bash -s -- --channel beta`
   If resolving the latest version fails because the GitHub release API is rate-limited (HTTP 403), retry after a short wait, install a specific version with `--version <tag>` (which skips the rate-limited lookup), or ask the user.
   Before giving account, token, balance, recharge, server, or console guidance, run `loomloom doctor --output json` when possible and use its `server`, `platform`, `platform_operational`, `token_set`, `credential_action`, and `credential_message` fields as the source of truth. Do not infer platform links yourself when the CLI already returned these fields. If the user has not provided `LOOMLOOM_SERVER` / `--server` and the CLI has no remembered server, guide them with the same platform-selection message below.
   If `credential_action` is `choose_platform`, output this exact fixed message:
   `你还没有完整配置 LoomLoom Server 和密钥。请选择要使用的平台：`
   ``
   `1. 胜算云：本服务由 CogFoundry 联合支持，面向中国大陆用户推荐使用，您可前往胜算云控制台获取密钥并完成充值。`
   `   - Server：https://loomloom.shengsuanyun.com/loom/v1`
   `   - 密钥控制台：https://console.shengsuanyun.com/user/keys`
   `   - 充值入口：https://console.shengsuanyun.com/user/recharge`
   `2. CogFoundry：面向新加坡及其他海外地区用户，当前支付和交易能力敬请期待；在 CogFoundry 计费功能上线前，请使用胜算云控制台创建 API 密钥。`
   ``
   `当前阶段请先选择胜算云。`
   Regardless of `credential_action` or whether a platform/token is already configured, whenever the user PROACTIVELY asks where to find or obtain their token / API key, which platforms are available, which server to use, or how to set up an account (as opposed to a command surfacing a missing-token or credential error mid-task), output the SAME full platform-selection message shown above for `choose_platform`, including the ShengSuanYun server URL and the CogFoundry availability note, but do not provide a CogFoundry website or Console URL. Do not collapse it to a single ShengSuanYun link. The single-platform messages below apply ONLY to passive credential/balance errors raised while running a command, not to a user's direct question about platforms or where to get a token.
   If `credential_action` is `missing_token` and `platform` is `shengsuanyun`, output this exact fixed message:
   `当前未检测到胜算云密钥。请前往胜算云控制台创建或复制密钥后配置到本地环境：`
   `https://console.shengsuanyun.com/user/keys`
   If balance is insufficient and `platform` is `shengsuanyun`, output this exact fixed message:
   `当前胜算云账户余额不足，请前往胜算云控制台充值后再继续：`
   `https://console.shengsuanyun.com/user/recharge`
   If `credential_action` is `cogfoundry_unavailable`, or the user expresses any intent to switch to, try, browse, or use CogFoundry, output this exact fixed message and then stop:
   `CogFoundry 面向新加坡及其他海外地区用户，当前支付和交易能力仍在建设中，敬请期待。当前阶段请继续使用胜算云。`
   After sending that message, do NOT take or propose any step toward CogFoundry. Specifically, you MUST NOT: offer or perform a server switch to a CogFoundry address; ask the user for a CogFoundry server URL, API key, or token; claim CogFoundry read-only features (browsing, doctor, etc.) are usable and invite the user to try them; describe or hint at how to switch; or present switching as an available option. Continuing on ShengSuanYun is the only available action; treat CogFoundry as fully closed for now, not a read-only preview. CogFoundry is for overseas users in the future and its payment and transaction capabilities are coming soon; do not provide a CogFoundry Console URL.
1. Check the environment:
   `loomloom doctor`
   The production default base URL is `https://loomloom.shengsuanyun.com/loom/v1`, but the active server is whatever the user sets in `LOOMLOOM_SERVER` / `--server`. Send the token only to that explicitly configured host, and only over HTTPS.
   Before each request, check the scheme and host of the final target URL. Only send the token to the host the user configured; do not send it to a host the user did not specify, and do not auto-follow redirects to a different domain.
   Use a token issued for the environment and platform you are targeting; do not reuse a token across environments or platforms it was not issued for. Do not send a ShengSuanYun token to a CogFoundry host, and do not send a CogFoundry token to a ShengSuanYun host.
   If the token is missing, use the platform selection guidance above. Do not echo a real token in replies, logs, or generated files.
2. Do not paste large files into context. Upload raw input assets first:
   `loomloom input-asset upload <file>`
3. Discover the official templates in the current environment:
   `loomloom template list`
4. Inspect official template fields, list available models, or list executable assets:
   `loomloom template schema <template-id>`
   `loomloom model list --step-type <text-generate|image-generate|video-generate>`
   `loomloom asset list`
   Note: `asset list` aggregates my private templates and Market SkillBots; it does not include official templates.
5. When authoring and saving a private template, use TemplateSpec JSON:
   `loomloom template-spec docs spec`
   `loomloom template-spec docs examples`
   `loomloom template-spec docs conversation`
   `loomloom template-spec models <text-generate|image-generate|video-generate>`
   `loomloom template-spec check <spec.json>`
   `loomloom template-spec create <spec.json>`
   `loomloom template-spec create-version <template-id> <spec.json>`
   `loomloom template-spec download-workbook <template-id> <version-id>`
   `loomloom template-spec validate-workbook <template-id> <version-id> <xlsx-path>`
   `loomloom template-spec precheck-workbook <template-id> <version-id> <xlsx-path>`
   `loomloom template-spec submit-workbook <template-id> <version-id> <xlsx-path>`
   Inspect existing private templates:
   `loomloom template-spec list`
   `loomloom template-spec get <template-id>`
   `loomloom template-spec versions <template-id>`
   To run a private template version directly from flat JSONL rows, upload the rows first and pass the returned input_file_id:
   `loomloom orchestration-input upload <file.jsonl>`
   `loomloom template-spec precheck <template-id> --version-id <version-id> --input-file-id <input_file_id>`
   `loomloom template-spec run <template-id> --version-id <version-id> --input-file-id <input_file_id>`
   Use `precheck-workbook` before `submit-workbook`, and `precheck` before `template-spec run`, to estimate model/API cost and balance without creating a run.
   For common single-root workflows, each non-empty line may be a flat JSON object with string values. Unified rows using `steps.<step-id>.executions[]` are also supported when exact workflow step mappings are available. In either format, execution parameter values must be strings and allowed by the private template version. Never guess step IDs.
6. If the user did not explicitly ask to create or use their own private template, use the official Excel workflow by default:
   `loomloom template download <template-id>`
   `loomloom template validate-file <template-id> <xlsx-path>`
   `loomloom template submit-file <template-id> <xlsx-path>`
   `loomloom run result-workbook <run-id>`
   Use the older local backfill flow only when the user explicitly needs it:
   `loomloom template backfill-results <run-id> <xlsx-path>`
7. Use JSON/JSONL only when the user explicitly asks for programmatic input:
   `loomloom run submit <template-id> -f rows.jsonl`
8. Watch progress:
   `loomloom run watch <run-id>`
9. List, inspect, or download runs and their results:
   `loomloom run list`
   `loomloom run get <run-id>`
   `loomloom run result-rows <run-id>`
   `loomloom run result-workbook <run-id>`
10. Inspect or download artifacts:
   `loomloom artifact list <run-id>`
   `loomloom artifact download <run-id>`

## Market Ecosystem

LoomLoom Market lets creators publish their private template versions as paid SkillBots, and lets buyers run those SkillBots. The same CLI serves two roles; decide which role the user is in before choosing commands.

At publish time, the Market copies an immutable Listing Version execution snapshot from the chosen private template version. Later changes to the private template do not automatically change a live SkillBot; to update the live execution version, submit a new template version for the same Listing and have it reviewed again.

### Buyer role (use and pay for a SkillBot)

- `loomloom market list` — browse published SkillBots. Supports `--keyword`, `--page-size`, `--page-token`, and `--order-by`.
- `loomloom market show <listing-id>` — show one SkillBot, including its `inputSchemaSnapshot`. Read the schema before building input.
- `loomloom market quote <listing-id> --input-file <request.json>` — estimate cost. Returns `estimatedBuyerPayableT`, `taskCount`, and `taskFixedFeeT`.
- `loomloom market run <listing-id> --input-file <request.json> --confirm --client-request-id <id>` — execute the SkillBot from JSON input rows. This is a paid action; see the Submission Confirmation Rule.
- `loomloom market workbook download <listing-id> --output-file <xlsx>` — download a public Market input workbook.
- `loomloom market workbook validate <listing-id> --file <xlsx>` — validate a filled Market workbook.
- `loomloom market workbook quote <listing-id> --file <xlsx>` — estimate cost for a filled Market workbook.
- `loomloom market workbook run <listing-id> --file <xlsx> --confirm --client-request-id <id>` — execute the SkillBot from a workbook. This is a paid action; see the Submission Confirmation Rule.
- `loomloom usage list` and `loomloom usage get <run-transaction-id>` — review the buyer's own SkillBot calls and settlement status. Use the returned `runTransactionId`.

For JSON input, `market quote` and `market run` use public `inputRows` built from `inputSchemaSnapshot.fields[].key`. Do not show this request JSON to users unless they explicitly ask for JSON/API details:

```json
{
  "inputRows": [
    {
      "prompt": "write a launch tweet"
    }
  ]
}
```

Read `market show` first to understand public fields and examples. Use `fields[].label` for user-facing prompts, `fields[].key` as the submitted `inputRows` key, `fields[].value_type` for type checks, `fields[].required` for required validation, and `sample_rows` as examples. Do not send `taskInputs`, `workflowDefinition`, `templateSpec`, or hidden Core / TemplateSpec structures to Market buyer execution endpoints. Do not infer hidden step IDs, hidden prompts, or internal mappings from `inputSchemaSnapshot`.

### Creator role (publish and manage a SkillBot)

- `loomloom listing publish <template-id> --template-version-id <id> --display-name <name> --task-fixed-fee <amount>` — submit a template version for Market review. The version must already have at least one successful run. Returns a `reviewRequestId` with `reviewStatus: pending_review`. Use normal currency units such as `--task-fixed-fee 0.5`; the CLI converts this to raw API units internally.
- To submit a new version for an existing listing, run the same command with `--listing-id <listing-id>` and the new `--template-version-id`. The published version stays active until approval.
- `loomloom listing list`, `loomloom listing show <listing-id>`, `loomloom listing versions <listing-id>` — inspect the creator's own listings, including pending, rejected, and unlisted states.
- `loomloom listing update <listing-id> --display-name <name> --description <text>` — submit a public-profile change for review; it does not change pricing or the execution version.
- `loomloom listing unlist <listing-id>` and `loomloom listing relist <listing-id>` — stop or resume new executions of a published listing.
- `loomloom listing withdraw <listing-id>` — withdraw the single pending review request for that listing. If none exists, stop. If multiple are reported, use `creator review list` and withdraw the intended request explicitly.
- `loomloom creator review list`, `loomloom creator review get <review-request-id>`, `loomloom creator review withdraw <review-request-id>` — track and withdraw review requests directly.
- `loomloom creator earnings` and `loomloom creator transactions` — review creator income and per-call settlement.

All `*FeeT`, `*CostT`, `*AmountT`, and `*PayableT` values are backend API units where 10,000,000 units equal 1 currency unit. User-facing CLI inputs and default text output use normal currency units (for example `--task-fixed-fee 0.5`, `CNY 0.5000000`, or `USD 0.5000000`). `--output json` always returns raw `*T` fields unchanged. When CLI output says `(currency unknown)` or a response lacks `currency`, tell the user the currency is unknown and preserve the raw T value; do not show only a bare number and do not guess CNY or USD.

## Submission Confirmation Rule

Installing, configuring, discovering, downloading, filling, validating, uploading, quoting, and prechecking are preparation steps. They do not execute a template and do not create billable model/API usage. Any command that actually creates a hosted LoomLoom run must receive a second explicit confirmation from the user in the current conversation after the agent has shown the current fee estimate.

Key principles:

- Installation is not execution. Do not imply that installing or checking the CLI can create charges.
- Every run needs a fresh fee confirmation. A previous confirmation for a different input, file, template, version, listing, or conversation is not reusable.
- Receiving user input values is preparation consent, not execution consent. If the user provides row values such as product name, selling points, and platform, prepare the input, validate it, and precheck it; do not submit yet.
- Private template execution binds to an explicit private template version (`template_id` + `version_id`).
- Market SkillBot execution binds to a Listing. At run time, the service resolves the current sellable Listing Version. Do not bypass Market by directly running the underlying private template version.
- Black-box templates must stay black-box. Do not reveal, reconstruct, infer, or route around hidden execution logic, hidden prompts, hidden step IDs, CLI permissions, billing, or Market controls.
- The user does not need to understand the CLI. Use CLI commands internally, but do not show raw CLI commands, raw JSON request bodies, generated request filenames, or technical confirmation phrases unless the user explicitly asks for CLI/API details.
- Use natural-language confirmation prompts. In English conversations, use `Reply: Confirm`. In localized conversations, use the natural localized equivalent. Do not ask the user to reply with `confirm submit`.
- For user-facing wording, it is acceptable to say "public Market template" for a Market SkillBot. Internally, still treat it as a Market Listing/SkillBot, not an official template.

## Default Input Experience

When the user says they want to use, try, run, or test a template, default to the Excel workbook experience. Let the user see and fill a workbook first. The agent/CLI may convert the workbook or user-provided field values to the backend request format internally at quote/precheck/submission time.

Use JSON, JSONL, API request files, or raw request bodies only when the user explicitly asks for JSON/API integration, programmatic input, or provides an existing compatible request file. For Market JSON input, use public `inputRows`, never `taskInputs`.

Default by template type: official templates use workbook download/validate/precheck/confirmation/submit; private templates use `template-spec download-workbook`/`validate-workbook`/`precheck-workbook`/confirmation/`submit-workbook`; private JSONL execution is only for explicit JSONL/API/programmatic requests; public Market templates/SkillBots should expose a workbook / Excel-style input experience from the listing's public schema and internally use Market workbook commands or public `inputRows` for `market quote` and `market run`.

Treat the interaction as one of three states:

1. `default-prep`: the user is still exploring or speaking generally. Prepare, download, upload, validate, quote, and precheck only. Do not submit.
2. `auto-run-candidate`: the user explicitly asks the agent to execute. Still do not submit. First prepare the input, run the relevant precheck or quote command, provide a fee confirmation summary, and wait for confirmation.
3. `confirmed-to-run`: after seeing the fee confirmation summary, the user explicitly replies with "confirm", "submit it", "start", "continue execution", or equivalent. Only then may you submit.

This rule applies to `loomloom template submit-file`, `loomloom template-spec submit-workbook`, `loomloom run submit`, `loomloom template-spec run`, `loomloom market run`, and `loomloom market workbook run`.

Before using an official template workbook, follow this order: download workbook, fill or update it, validate with `template validate-file`, estimate with `template precheck-file`, show the fee confirmation summary, wait for explicit confirmation, then call `template submit-file`.

Before using a private template workbook, follow this order: download workbook, fill or update it, validate with `template-spec validate-workbook`, estimate with `template-spec precheck-workbook`, show the fee confirmation summary, wait for explicit confirmation, then call `template-spec submit-workbook`.

Before using a private template with JSONL rows, follow this order: prepare JSONL, upload with `orchestration-input upload`, estimate with `template-spec precheck`, show the fee confirmation summary, wait for explicit confirmation, then call `template-spec run`.

Before using a Market SkillBot / public market template with JSON input, follow this order: `market show`, provide an Excel-style input experience from the public listing schema or fill fields from the user's natural-language values, internally build public `inputRows` using `inputSchemaSnapshot.fields[].key`, run `market quote`, show the public market template confirmation template including fee rules, wait for explicit confirmation, then call `market run --confirm --client-request-id <stable-id>`.

Before using a Market SkillBot / public market template with a workbook, follow this order: `market show`, `market workbook download`, let the user fill or approve the workbook values, `market workbook validate`, `market workbook quote`, show the public market template confirmation template including fee rules, wait for explicit confirmation, then call `market workbook run --confirm --client-request-id <stable-id>`.

If a requested execution path does not provide a separate quote or precheck command that can return the fee estimate before submission, do not submit. Explain that a pre-submission estimate is required and choose an equivalent workbook or private-template flow that supports precheck, or ask the user for a compatible prechecked path.

Validation, precheck commands, downloads, schema inspection, model lookup, quoting, `doctor`, asset upload, orchestration-input upload, artifact listing, listing/usage/earnings reads, and result backfill do not start a new paid run and do not require the second confirmation.

The fee confirmation summary must include the template or listing ID, template type, input source, row count or task size, action, estimated cost or buyer payable amount, currency, available balance and sufficiency when returned, and a clear natural-language confirmation prompt. In English conversations, use `Reply: Confirm`. For private templates, include the fixed `version_id`. For Market SkillBots, include the Listing ID and state that the service will use the current sellable Listing Version.

For private and official template precheck output, preserve the server-provided currency. Do not perform local USD/CNY conversion. `*T` values use API units where 10,000,000 units equal 1 currency unit. Prefer plain money text such as `CNY 0.0119` or `USD 0.0119`, not a bare currency symbol.

For Market quote/run confirmation, show the buyer-facing fee summary only: creator call fee, estimated model/API cost, and estimated pre-authorization. Do not show platform commission, creator net earnings, or any revenue-sharing breakdown — that information is only available to creators via the `creator earnings` command.

For user-facing confirmations, use these templates instead of raw CLI output, raw JSON, or terse key/value summaries.

Public Market template / SkillBot confirmation template:

```text
This will make a paid call to a public Market template. Please confirm the fee before execution.

Template: <template_display_name>
Call type: public Market template
Listing ID: <listing_id>
Locked version: <listing_version_id_or_current_sellable_version>

Input:
- Task count: <task_count> task(s)
- Billing rule: creator call fee is charged per task

Fee estimate:
- Creator call fee: <creator_call_fee> (<task_count> task(s) x <task_fixed_fee>)
- Estimated model/API cost: <estimated_model_api_cost_or_note>
- Estimated pre-authorization: <estimated_buyer_payable>

Final billing rules:
- Final charge = creator call fee + actual model/API cost
- Creator call fee is locked at order time and settled after the run completes
- Model/API cost is settled by actual usage; unused pre-authorization is released or adjusted
- Initial rule: if the task fails or partially fails, the creator call fee is still charged and is not refundable

Please confirm whether to execute.
Reply: Confirm
```

Private template confirmation template:

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
- Model/API cost is settled by actual usage; unused pre-authorization is released or adjusted
- Private templates do not create creator call fees, platform commissions, or Market revenue sharing

Please confirm whether to execute.
Reply: Confirm
```

Do not invent fee fields. For private/official precheck, use `estimatedTotalCostT` and the server-provided currency. For Market quote, prefer `estimatedBuyerPayableT` for the total pre-authorization. Compute creator call fee from `taskCount x taskFixedFeeT` only when those fields are present. If Market quote does not separately return model/API cost, show `CNY 0.00` only when the quoted payable equals the creator call fee; otherwise say "included in the estimated pre-authorization" rather than inventing a number.

If the user says "do not run yet", "wait", or similar, stay in preparation mode. For `template submit-file`, `template-spec submit-workbook`, `run submit`, `template-spec run`, `market run`, and `market workbook run`, pass an explicit stable `--client-request-id` and retain it for safe retry of the identical payload.

Do not print full workbook base64 `content`. Do not copy result-row `accessUrl` values into long-lived logs or docs; they are temporary signed URLs. Prefer displaying `inlineText` for small text artifacts and saying that a download URL is available for file artifacts.

Present the confirmation summary in plain business language (what will happen, which template or SkillBot, how many tasks, and the cost). Do not show the raw CLI command in the confirmation unless the user explicitly asks to see it. Do not show platform commission, creator earnings, or any revenue-sharing breakdown to the user — that information is only shown to creators via the `creator earnings` command.

## Creator Earnings Response

When the user asks about creator income, template earnings, settlement, revenue, failed settlement, or how much a public market template earned, use `creator earnings` and, when recent line items are needed, `creator transactions`. Do not show raw CLI output or raw JSON unless the user asks.

Use this response shape: start with `Here is the earnings overview for your public Market template:`, show the template name, cumulative call count, creator call fee, platform fee, creator net receivable, settled/pending/failed amounts, an exception line when settlement failures exist, and the latest five transactions in the form `Run <run_id>, net <amount>, status: <settled|failed|pending>`. End with `Full details can be exported to Excel if needed.` If a field is missing from the API response, omit that line or say it was not returned. Do not fabricate counts, run IDs, settlement status, or amounts.

## Remote State Change Confirmation Rule

Before creating or changing persistent remote resources, show the exact action and ask for explicit confirmation. This applies to `template-spec create`, `template-spec create-version`, `listing publish` (including `--listing-id` version updates), `listing update`, `listing unlist`, `listing relist`, `listing withdraw`, and `creator review withdraw`.

Describe the action in plain business language; "show the exact action" does not mean printing the CLI command. Do not show the raw CLI command unless the user explicitly asks to see it.

Read-only commands, local checks, uploads, downloads, and quotes do not require this confirmation. A paid run still follows the stricter Submission Confirmation Rule above.

## Where To Inspect Commands And Inputs

- Command syntax, positional arguments, and flags: run `loomloom <command> --help`.
- TemplateSpec JSON format for creating private templates: run `loomloom template-spec docs spec|examples|conversation`.
- Official template input fields: run `loomloom template schema <template-id> --output json`.
- Market SkillBot public input fields: run `loomloom market show <listing-id> --output json`.
- Workbook input shape: download the workbook and read its headers/instructions.
- Command outputs for chaining: use `--output json` and preserve returned IDs exactly.
- Safety, confirmation, billing, and workflow rules: follow this Skill.

## Agent Command Chaining

Prefer `--output json` whenever one command feeds another. Preserve exact fields:

- `orchestration-input upload` → `inputFileId` → `template-spec precheck --input-file-id` → `template-spec run --input-file-id`
- run submission → `runId` → run watch/result commands
- `listing publish` → `reviewRequestId` → creator review commands
- `market run` and `market workbook run` → `runTransactionId` and `runId` → usage/run commands and `run result-workbook`

Never convert `inputAssetId` (`ia_xxx`) into `inputFileId`, and never guess IDs from names.
For the supported submission commands listed in the Submission Confirmation Rule, pass an explicit `--client-request-id`, retain it with the request, and reuse it only for an identical retry. A changed payload requires a new ID.

## Error Handling

Inspect the returned error before choosing a recovery step:

- For local flag, file, JSON, or schema errors, correct the input and retry without running `doctor`.
- For authentication, endpoint, network, service-version, or unexpected server errors, run `loomloom doctor`.
- Never invent missing IDs, hidden step IDs, or server state.
- When explaining CLI JSON or backend responses to a user, translate technical field names and enum values into user-facing wording. Do not expose developer field names such as `saleStatus`, `reviewStatus`, `executionAvailabilityStatus`, `executionBlockReason`, `forced_unlisted`, `inputSchemaSnapshot`, or `taskFixedFeeT` unless the user explicitly asks for raw JSON/API fields. For example, explain `forced_unlisted` as "This SkillBot has been forcibly removed from the Market and cannot currently be listed or run"; explain `reviewStatus=rejected` as "the review was not approved"; explain `executionAvailabilityStatus=blocked` as "currently unavailable to run". Keep raw field names only for command chaining or `--output json` parsing, not user-facing summaries.
- Do not blindly retry paid or remote-state-changing commands after an ambiguous failure. First query the relevant run, listing, or review state. If the identical submission must be retried, reuse its original `--client-request-id`; use a new ID only for a changed payload.

## Current MVP Capabilities

The public CLI currently supports these command groups:

- Environment: `doctor`.
- Inputs: `input-asset upload`, `orchestration-input upload`.
- Discovery: `template list`, `template schema`, `model list`, `asset list`.
- Official Excel workflow: `template download`, `template validate-file`, `template precheck-file`, `template submit-file`, `template backfill-results`.
- Custom templates: `template-spec docs`, `template-spec check`, `template-spec models`, `template-spec create`, `template-spec create-version`, `template-spec list`, `template-spec get`, `template-spec versions`, `template-spec download-workbook`, `template-spec validate-workbook`, `template-spec precheck-workbook`, `template-spec submit-workbook`, `template-spec precheck`, `template-spec run`.
- Runs: `run submit`, `run list`, `run get`, `run watch`, `run result-rows`, `run result-workbook`.
- Artifacts: `artifact list`, `artifact download`.
- Market (buyer): `market list`, `market show`, `market quote`, `market run`, `market workbook download`, `market workbook validate`, `market workbook quote`, `market workbook run`, `usage list`, `usage get`.
- Market (creator): `listing publish`, `listing list`, `listing show`, `listing versions`, `listing update`, `listing unlist`, `listing relist`, `listing withdraw`, `creator earnings`, `creator transactions`, `creator review list`, `creator review get`, `creator review withdraw`.

## Large File Handling

When the user wants to batch-process local code files, large text, local images, or other large files, avoid pasting full files into the agent context. Prefer:

1. `loomloom input-asset upload <file>`
2. Save the returned `input_asset_id`
3. Put it only in a schema field that accepts an asset reference

For TemplateSpec, use a compatible `asset_ref` or `text_reference` field and follow the bundled binding guidance. An `input_asset_id` is reference material; it is never the `inputFileId` used by `template-spec run`.

## Default Behavior

Unless the user explicitly asks for JSON/JSONL, use the official Excel template workflow by default.

When the user asks to create or customize a workflow/template, prefer TemplateSpec JSON. TemplateSpec JSON is source data; downloaded workbooks are derived artifacts. When a template version changes, do not promise that old workbooks remain compatible. Download a new workbook.

## Conversational Template Authoring

When the user describes a new template in natural language, do not immediately write TemplateSpec JSON. First run or reference `loomloom template-spec docs conversation`.

Flow:

1. Ask business questions, not TemplateSpec technical-field questions.
2. Ask one missing-detail question at a time.
3. Avoid user-facing technical terms such as `fieldBindings`, `upstreamBindings`, `fan-in`, `execution`, `outputSchema`, `provider`, and `mode`.
4. Restate complex workflows in business language before continuing.
5. After identifying workflow steps, roles, perspectives, or generated agents, determine whether each role/step has its own processing requirements. If yes, ask about future template usage before drafting TemplatePlan.
6. Draft TemplatePlan first.
7. Show TemplatePlan and wait for user confirmation.
8. Generate TemplateSpec JSON only after confirmation.
9. Before creation, run `loomloom template-spec check <spec.json>`.
10. After check passes, ask for separate creation confirmation.
11. Run `loomloom template-spec create <spec.json>` only after explicit creation confirmation.

When an existing user template needs a fix, do not promise in-place mutation of historical versions. Use `loomloom template-spec create-version <template-id> <spec.json>` to append a new version, and make later workbooks/runs use the new `version_id`.

### Template Usage Mode Selection

When a template has multiple roles, steps, review perspectives, or generated agents, and each role/step may have its own processing requirements, the agent must ask before generating TemplatePlan:

```text
When other people use this template later, how should the review/generation requirements be handled?

1. Preset in the template: users only fill the core material, and the system follows your predefined requirements automatically.
2. Let users fill them: users can fill or modify the requirements for each step/role.
3. Generate both versions: one simple version and one customizable version.

If you are unsure, choose 1 first to make a simple, usable standard template.
```

Do not expose technical concepts such as `prompt`, `binding`, `reference`, `field`, `hidden`, `paramBindings`, or `fieldBindings`. Typical trigger scenarios include multi-perspective PRD review, multi-role contract review, multi-agent launch event planning, rewriting an article in multiple styles, resume review from multiple interviewer perspectives, and generating marketing content for multiple channels.

Three modes:

1. Simple usage mode: users fill only core material, and the system executes with preset processing requirements.
2. Custom usage mode: users fill both core material and processing requirements for each step/role.
3. Generate both versions: generate two TemplatePlans and create two templates, naming them as `Standard Version` / `Custom Version`.

Automatic generation rules: in simple mode, role/step processing requirements are template preset instructions, not user-fillable columns; in custom mode, core material and role/step processing requirements are both user input columns; when generating both versions, show both TemplatePlans first and get user confirmation before generating two TemplateSpecs.

Acceptance criteria: multi-role/multi-step/multi-perspective template creation must ask about usage mode; simple mode hides role/step requirement columns; custom mode exposes and allows editing those columns; user-facing conversation does not expose technical concepts; generated TemplateSpec correctly expresses both simple and custom shapes.

Creation confirmation gate:

- "Create a PRD review template" only starts the flow; it does not confirm remote creation.
- Environment variables, token, and server URL are configuration, not creation confirmation.
- "Generate spec" only means generate and locally check the spec; it does not confirm running `template-spec create`.
- Before running `template-spec create`, describe in plain language what will be created (template name, what it does, the local check result) and ask for a natural confirmation. In English conversations, use `Reply: Confirm creation`. Do not show the raw CLI command unless the user explicitly asks to see it.

TemplatePlan should cover template name and goal, row meaning, input fields, workflow steps, serial/parallel/summary relationships, template usage mode, user-visible outputs, failure policy, error columns, default model, and special requirements.

Default modeling rules:

- "Product, engineering, and risk review separately, then summarize" should be modeled as multiple parallel `text-generate` steps plus one downstream summary step, using step-level `dependsOn` and `upstreamBindings`.
- Do not model multi-role review as one `expanded` step. `expanded` is only for dynamic multi-value input.
- Add result and error columns for every user-visible step by default.
- If partial completion is allowed, the summary step should explain missing upstreams and expose failed step error columns.
- Keep `provider` and `mode` internal.

TemplateSpec authoring constraints:

- Before writing custom TemplateSpec, run `loomloom template-spec docs spec` and use the CLI-bundled document as the current contract.
- Use `loomloom template-spec docs examples` for examples and patterns.
- Use `loomloom template-spec docs conversation` for the natural-language creation flow.
- Use only `text-generate`, `image-generate`, and `video-generate` by default.
- Use OpenAPI lowerCamel fields such as `meta.name`, `steps[].stepId`, and `defaultModelRef.modelKey`.
- `inputSchema.sampleRows[]` must wrap sample field values in a `values` object, for example `{ "values": { "prompt": "example" } }`; do not put field keys directly on the sample row object.
- Connect steps with step-level `dependsOn` and `upstreamBindings`; the source output port is usually `output`.
- Before choosing `defaultModelRef.modelKey`, run `loomloom template-spec models <execution-unit>` and use a returned `model_id`.
- Expose a model column only when the step sets `allowModelOverride=true` and a field binds to `paramKey=model`.
- Do not bind `provider` or `mode`.

## Installing Templates As Local Agent Skills

When the user asks to install, add, or turn a LoomLoom template into a local Codex / Claude Code / OpenClaw skill, treat that as installation, not execution. Installation must not submit a run, quote/precheck costs, execute a Market SkillBot, or create billable usage.

Use the install preview before writing files:

`loomloom skill install market <listing-id> --agent <codex|claude|openclaw> --output-dir <skill-dir> --dry-run --output json`

`loomloom skill install template-spec <template-id> <version-id> --agent <codex|claude|openclaw> --output-dir <skill-dir> --dry-run --output json`

Show an installation confirmation card with the Skill name, source, binding, target agent, output directory, main inputs, and the fact that every real run still needs quote/precheck plus explicit confirmation. Generated Skill names always use the `loomloom-` prefix, and the final output directory basename must match the previewed `skillName`. If the user provides a skills parent/root directory instead of an exact Skill directory, the first preview may return `blockingReason=output_dir_name_mismatch`; use the returned `skillName` to append the final directory name, rerun preview with that exact `--output-dir`, and only then show the confirmation card. Do not install into an unprefixed directory. If the user has not provided a directory, ask for one; do not guess the agent's default skill directory in this phase. Only after the user confirms installation, call the same command without `--dry-run`.

Market Skill installs bind to the Listing. The installed listing version is only traceability; each future execution must read the current Listing and use Market quote/run or Market workbook quote/run. Private template installs bind to the exact `template_id + version_id` and must use `template-spec` commands, not Market commands. If the listing is unavailable, permissions fail, or a version is unavailable, stop and explain the issue.

When the user asks to delete, uninstall, remove, disconnect, or stop using a local LoomLoom Agent Skill, treat that as local Skill uninstall. Ask for the exact Skill directory if it is not already known. First run `loomloom skill uninstall --dir <skill-dir> --dry-run --output json`, show the Skill name, source, agent, directory, and files that will be deleted, then wait for explicit confirmation before running `loomloom skill uninstall --dir <skill-dir>`. Do not delete directories manually. If the preview reports unexpected files, explain them and use `--force` only after the user explicitly confirms removing the whole directory.

## Trusted Result Sources

The submitted workbook and server-side run input snapshot are the sources of truth. After the run completes, prefer `loomloom run result-workbook <run-id>` because the server aligns original input snapshots and artifacts. Use `template backfill-results` only when the user explicitly needs the older local Excel backfill flow.

## Console Access

Current MVP console guidance is platform-bound. For ShengSuanYun users, account keys and recharge are handled in the ShengSuanYun Console:

- API keys: `https://console.shengsuanyun.com/user/keys`
- Recharge: `https://console.shengsuanyun.com/user/recharge`

For run status, prefer CLI commands such as `loomloom run watch <run-id>`, `loomloom run get <run-id>`, and `loomloom run result-workbook <run-id>`. Do not provide CogFoundry Console links in the current MVP; CogFoundry account, payment, and transaction capabilities are coming soon.

There is currently no URL template for a Workflow Run detail page. Do not guess or construct a detail-page link from a `runId`; if the CLI returns a URL explicitly provided by the server, use it as-is, otherwise use CLI query commands instead of inventing a console link.

Do not provide a CogFoundry website or CogFoundry Console URL in user guidance.
