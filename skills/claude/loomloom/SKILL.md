---
name: loomloom
description: Use this skill when the user mentions LoomLoom, loomloom, batch generation, batch processing, batch tasks, batch execution, template execution, template submission, Excel submission, template download, Excel validation, run results, result download, run id, task status, execution status, official templates, private templates, TemplateSpec, Excel/workbook templates, Market SkillBots, template market, public templates, SkillBot, listing, unlisting, relisting, price changes, fixed task fees, publishing to the Market, review submission, review status, creator earnings, template earnings, usage, orders, call records, installing LoomLoom, doctor, token, server, or when the user clearly wants multi-row input to produce batch outputs.
---

# loomloom

Use LoomLoom for structured batch work: official/private template execution, TemplateSpec authoring, Market SkillBots, creator listing management, local Agent Skill installation, run monitoring, and result retrieval. Do not use it for ordinary one-off writing unless the user explicitly asks for LoomLoom, templates, workbooks, Market SkillBots, or batch/table-shaped processing.

## Choose The Product Object

```text
Official template ── platform-maintained; use template/run

Private template ── user-created through TemplateSpec
└─ Private template version
   └─ Market review
      └─ immutable Listing Version
         └─ approved SkillBot; buyers use market
```

- Creators publish and manage SkillBots with `listing` and `creator review`.
- Buyers call a Listing; they do not hold a Listing Version.
- "Public Market template" may be used for users, but internally means a Market SkillBot.
- There is no separate public-template resource.
- `asset list` contains private templates and Market SkillBots, not official templates.
- Clarify when "template" could mean official, private, or Market.

## Required Reference Routing

Read all references required by the intent before answering with business rules or acting:

- Setup, install, token, server, platform, balance, console, `doctor`: [setup.md](references/setup.md) and [cli.md](references/cli.md).
- Official/private execution, validation, precheck, run, results: [execution.md](references/execution.md), [billing.md](references/billing.md), and [cli.md](references/cli.md).
- Private template creation, explanation, or versioning: [template-spec.md](references/template-spec.md), [billing.md](references/billing.md), and [cli.md](references/cli.md).
- Market discovery, quote, or buyer execution: [market.md](references/market.md), [billing.md](references/billing.md), and [cli.md](references/cli.md).
- Creator publishing, price/version/profile changes, listing lifecycle, review, usage, settlement, or earnings: [market.md](references/market.md), [billing.md](references/billing.md), and [cli.md](references/cli.md).
- Local Agent Skill install/uninstall: [local-skills.md](references/local-skills.md) and [cli.md](references/cli.md).
- Failures, ambiguous responses, missing IDs, or syntax: [cli.md](references/cli.md) plus the relevant domain reference.

Whenever fees, currency, balance, confirmation, paid execution, failure, cancellation, partial completion, usage, settlement, or earnings are involved, `billing.md` is mandatory.

## Workflows

- Official workbook: discover → download → fill → validate → precheck → show fee confirmation → confirm → submit → watch → results.
- Official JSON/JSONL, only when explicitly requested: prepare → validate → precheck → show fee confirmation → confirm → execute.
- Private execution: bind explicit template/version → prepare version-specific input → validate/precheck → show fee confirmation → confirm → run → results.
- Private authoring: business conversation → TemplatePlan → confirm plan → generate TemplateSpec → check → confirm creation → create/version.
- Market buyer: inspect Listing/public schema → prepare public input → quote → show Market confirmation → confirm → run through Listing → usage/results.
- Market creator: choose a successfully run private version → explain publish/price/version/profile action → confirm → submit → track review.
- Local Skill: preview exact files, binding, and destination → confirm → install/uninstall. Installation is not execution.

## Global Rules

1. Send tokens only over HTTPS to the explicitly configured host. Never expose them, retain them across redirects, or reuse them across platforms/environments.
2. Default to Excel. Use JSON/JSONL only for an explicit programmatic-input request or compatible supplied file.
3. Downloads, uploads, validation, quote, and precheck are preparation; they do not authorize a hosted run.
4. Before every hosted run, show the current server estimate and obtain explicit confirmation in the current conversation, except during an explicitly activated Test Execution Mode defined below.
5. If input changes after validation, estimate, or confirmation, validate and estimate again and obtain a new confirmation. In Chinese say `输入内容在确认后发生变化，需要重新预估并确认。`
6. Give every newly confirmed execution a new `client-request-id`; reuse one only for an identical retry after an ambiguous failure.
7. Execute SkillBots through the Listing and public schema. Never reveal or reconstruct hidden prompts, steps, mappings, TemplateSpec, or private logic.
8. Market locks the current sellable Listing Version and its price when creating the order.
9. Require explicit confirmation before template creation/versioning, Listing publish/change, unlist/relist, or review withdrawal.
10. Never guess IDs, versions, flags, currency, fee fields, server state, hidden step IDs, settlement outcomes, or URLs.
11. Use CLI help for syntax, CLI-bundled TemplateSpec docs for the contract, workbooks for input shape, and returned service fields for state/IDs.
12. Translate CLI/JSON/internal fields into business language unless raw CLI/API detail was requested.
13. Upload large files instead of placing them in context.

## Test Execution Mode

Test Execution Mode is an optional, conversation-scoped prompt authorization for repeated paid test runs. It is disabled by default and available to every user and Agent that uses this Skill. It is not a server-side authorization or billing-control mechanism.

Activate it only when the user explicitly says they do not need a confirmation before each test run and supplies, or explicitly accepts, the target environment / Server, permitted template/version or Listing scope, maximum tasks per run, per-run and total cost limits with currency, and expiry (default: the current conversation only).

On activation, restate the limits once and record that Test Execution Mode is active. Within those exact limits, the Agent may validate, precheck, submit, watch, and retrieve results without another per-run confirmation. A missing, zero, or currency-unknown precheck estimate is never eligible for automatic submission: stop and obtain a fresh explicit confirmation before creating that run. Each run still needs a new `client-request-id` and a report of its run ID, status, and actual cost when returned.

The mode stops and normal per-run confirmation resumes when a limit is exceeded, input scope or Server/platform changes, it expires, or the user withdraws it. Never infer activation from testing context or an earlier conversation.

It never covers persistent or high-impact operations: template creation/versioning, Listing publish/change/unlist/relist, review withdrawal, deletion, credential or Server configuration changes, or anything outside the approved execution scope. Those operations always require explicit confirmation.

## References

- [Setup and security](references/setup.md)
- [Template execution](references/execution.md)
- [Market and SkillBots](references/market.md)
- [Billing and confirmation](references/billing.md)
- [Template authoring](references/template-spec.md)
- [Local Agent Skill management](references/local-skills.md)
- [CLI and errors](references/cli.md)
