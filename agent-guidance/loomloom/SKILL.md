---
name: loomloom
description: Use this skill when the user mentions LoomLoom, loomloom, batch generation, batch processing, batch tasks, batch execution, template execution, template submission, Excel submission, template download, Excel validation, run results, result download, run id, task status, execution status, official templates, private templates, TemplateSpec, Excel/workbook templates, Market SkillBots, template market, public templates, SkillBot, listing, unlisting, relisting, price changes, fixed task fees, publishing to the Market, review submission, review status, creator earnings, template earnings, usage, orders, call records, installing LoomLoom, doctor, token, server, or when the user clearly wants multi-row input to produce batch outputs.
---

# loomloom

LoomLoom is a template-based AI workflow platform for structured, repeatable AI work. Use it for batch or table-shaped generation, official and private template execution, TemplateSpec authoring, Market SkillBots, creator listing management, local Agent Skill installation, run monitoring, and result retrieval.

Do not use LoomLoom for ordinary one-off writing or chat unless the user explicitly asks for LoomLoom, templates, Excel/workbook input, a Market SkillBot, or batch/table-shaped processing.

## Install And Update LoomLoom

When asked to install or update LoomLoom:

1. Use the distributed `skills/loomloom` directory as the Skill source.
2. Determine the current Agent's supported Skill root from its runtime configuration or official conventions.
3. Use `<agent-skill-root>/loomloom` as the complete destination.
4. Pass that destination to the repository installer with `--skill-dir` on macOS/Linux or `-SkillDir` on Windows.
5. Do not guess the Skill root or fall back to another Agent's directory. If it is unknown, ask the user for it.
6. Verify that `<agent-skill-root>/loomloom/SKILL.md` exists after installation.

## Core Objects

Choose the product object before choosing a command:

```text
Official template ── platform-maintained and executed directly

Private template ── user-created through TemplateSpec
└─ Private template version
   ├─ optional private Agent Skill Package Head
   └─ Submitted to Market review
      └─ Listing Version (immutable publish snapshot)
         └─ Approved and executable as a SkillBot
```

- Use `template` and `run` for official templates.
- Use `template-spec` to author, version, and directly execute private templates.
- Use `market` when a buyer discovers, quotes, or runs a paid SkillBot.
- Use `listing` and `creator review` when a creator publishes or manages a SkillBot.
- A Listing is the Market shelf object. Buyers call the Listing; they do not hold a Listing Version.
- There is no separate "public template" resource. User-facing "public Market template" means a Market SkillBot.
- `asset list` aggregates private templates and Market SkillBots; it does not include official templates.

When "template" is ambiguous, clarify whether the user means an official template, their private template, or a Market SkillBot.

## Intent Routing

Read every required reference for the current intent before answering with business rules or taking action. References are mandatory routing targets, not optional background reading.

| Intent | Required references |
|---|---|
| Install/configure LoomLoom; token, server, platform, balance, console, or `doctor` | [setup.md](references/setup.md), [cli.md](references/cli.md) |
| Execute an official or private template; validate, precheck, run, monitor, or retrieve results | [execution.md](references/execution.md), [billing.md](references/billing.md), [cli.md](references/cli.md) |
| Create, explain, or version a private template or TemplateSpec | [template-spec.md](references/template-spec.md), [billing.md](references/billing.md), [cli.md](references/cli.md) |
| Discover, quote, or execute a Market SkillBot | [market.md](references/market.md), [billing.md](references/billing.md), [cli.md](references/cli.md) |
| Publish, change price/version/profile, list/unlist/relist, review, usage, settlement, or earnings | [market.md](references/market.md), [billing.md](references/billing.md), [cli.md](references/cli.md) |
| Install or uninstall a LoomLoom template as a local Agent Skill | [local-skills.md](references/local-skills.md), [cli.md](references/cli.md) |
| Diagnose a failure, ambiguous response, missing ID, or command syntax | [cli.md](references/cli.md) plus the relevant domain reference above |

Whenever a task involves fees, quote/precheck, currency, balance, confirmation, paid execution, failure, cancellation, partial completion, usage, settlement, or creator earnings, read `billing.md` before answering or acting.

## High-Level Workflows

### Official template

Default workbook flow:

```text
Discover → Download workbook → Fill → Validate → Precheck
→ Show fee confirmation → User confirms → Submit → Watch → Results
```

Programmatic input is opt-in:

```text
Prepare JSON/JSONL → Validate → Precheck
→ Show fee confirmation → User confirms → Execute
```

### Private template execution

Bind to an explicit private `template_id + version_id`:

```text
Download/prepare version-specific input → Validate → Precheck
→ Show fee confirmation → User confirms → Run → Results
```

### Private template authoring

```text
Business conversation → decide LoomLoom-only or Agent-assisted
→ TemplatePlan → User confirms plan → Generate TemplateSpec
→ Check with the current LoomLoom Server → User confirms creation
→ Create or append a version
→ LoomLoom-only: use automatic package handling; do not create or upload a custom ZIP
→ Agent-assisted: creator chooses custom Skill Package or automatic package handling
→ custom: Agent builds Skill ZIP locally → preview → creator confirms
→ local trial → upload private Package Head
```

An Agent-assisted package is built by the Agent, not by the CLI. Never create or upload one for a LoomLoom-only template. When Agent assistance is needed, explicitly offer the creator two business choices: create and upload a custom Skill Package, or use `auto` without uploading a custom ZIP. Every later custom package replacement repeats its preview, creator confirmation, and default local trial.

### Legacy TemplateSpec v1 upgrade gate

Historical v1 TemplateVersions remain readable and runnable, but v1 cannot be
used to create a new template or save a new version. When a user needs to
change, copy, or newly author a v1 template, create a new `template-spec/v2`
version; never overwrite or claim to repair the historical v1 version.

Before changing, copying, or appending a version to an existing private
template, inspect the exact target version reported by the current Server:

```text
loomloom template-spec get <template-id> --output json
loomloom template-spec versions <template-id> --output json
```

Resolve whether the user means the published, latest, or another explicit
version, then inspect that version's returned `specVersion`. Do not infer the
version from JSON shape, a 404, or an error message. If it is
`template-spec/v1`, do not submit its historical JSON to `create-version`.
Explain that the historical version remains readable/runnable but is read-only,
and guide the user through creating a new v2 version. Do not force this upgrade
when the user only wants to run an existing v1 version.

Do not infer a v2 contract from v1 `modelKey`, `staticParams`, field bindings,
or provider-native fields. Read the current v2 protocol and the target
environment's authoring facts first:

```text
loomloom model types --output json
loomloom template-spec docs spec --lang zh-CN
loomloom template-spec docs inputs --lang zh-CN
loomloom template-spec docs bindings --lang zh-CN
loomloom capability resolve --input <modality> --output-modality <modality> --output json
loomloom template-spec authoring-context --output json
loomloom template-spec contracts <model-id> --output json
loomloom model list --step-type <step-type> --output json
```

First classify every v1 Step by its business input and output modalities, then
call `capability resolve` for that shape. Its returned matches are the primary
authoring route: a `capabilityProfile` match carries the current Profile, ports,
and eligible models; a `fixedModelContract` match carries the exact
`subjectRevisionId` and ports. Use `model types`, `model list`,
`authoring-context`, and `contracts` only to diagnose or inspect the lower-level
facts behind that resolution. If the resolver returns `not_supported`, stop and
report `needs_authoring_capability`; never infer support from a raw model name
or modality table.

Before writing or checking the v2 candidate, build a per-Step semantic
preservation ledger from the exported v1 definition. Every non-empty v1
`Steps[].Instruction` must still reach a model-bound v2 input. For a Capability
Profile Step, bind that instruction to `systemInstruction` with `literal`,
`composeValue`, or an appropriate text merge. `workbook.instructions` is only a user filling guide and never substitutes for a model instruction. Preserve the
v1 model-selection policy: when `AllowModelOverride=false`, use fixed
`modelSelection` and do not invent a `modelChoice` Template Input. Map image,
video, audio, file, and step-output references by their actual transport and
target port, not by converting them to strings.

`template-spec check` proves current schema, port, model, and authority
compatibility only. It does not prove v1-to-v2 semantic equivalence. After a
successful check, compare at least the Step topology, every non-empty
Instruction, input transports, upstream bindings, model-selection policy,
user-visible outputs, and failure policy. If any meaning is missing or changed,
stop with `semantic_review_required`; do not create the version. Show this
semantic diff before asking for creation confirmation. On the current LoomLoom
Server, `create-version` advances both `latestVersionId` and
`publishedVersionId`; disclose that impact before confirmation and read both
pointers back afterward.

An empty fixed-contract result means that exact target model/operation is not
currently authorable. Report `missing_fixed_contract` or choose another
returned target model; never produce a v2 draft that claims the missing
contract exists. Rewrite the v1 definition as v2
`templateInputs`, `executionBinding`, and `inputBindings`. Use
`loomloom template-spec get-version <template-id> <version-id> -f historical.json`
to retrieve an owner-visible historical definition when needed. Then run
`check` against the same Server where the new version will be saved and create
a new immutable version only after explicit confirmation. Do not promise a lossless or automatic v1-to-v2 conversion.

### Market buyer

```text
Discover Listing → Inspect public schema → Prepare public input
→ Quote → Show Market fee confirmation
→ User confirms → Run through Listing → Usage/results
```

When the user explicitly says to install or use a Market/official SkillBot, first use its public ZIP package when available. Install or update it automatically in the current Agent's Skill root; do not trigger this for browsing or quoting.

### Market creator

```text
Select a successfully run private version → Explain intended change
→ User confirms → Submit for review → Track review
```

Changing only the price and changing the execution version are different operations; follow `market.md`.

### Local Agent Skill

```text
Explicit install/use request → Agent identifies its own Skill root
→ Download current public ZIP → validate and atomically install/update
```

This automatic package installation does not authorize a paid run. Creator package uploads remain a separate, explicitly confirmed flow.

## Global Rules

**Match the user's language.** Respond in the language evident from the user's messages. If it is not yet evident, default to Chinese for ShengSuanYun and English for CogFoundry. Apply this to every user-facing message, including predefined templates, confirmations, warnings, and errors. Preserve commands, URLs, identifiers, amounts, and currency codes when localizing.

Determine the current platform only from the user's explicit selection or a successful `loomloom doctor --output json` result. Never infer it from a hostname, location, language, or other context.

1. **Protect credentials.** Send a token only over HTTPS to the explicitly configured host. Never expose it, follow it across domains, or reuse it across platforms/environments.
2. **Default to Excel.** Use JSON/JSONL only when the user explicitly requests programmatic input or supplies a compatible request file.
3. **Separate preparation from execution.** Downloads, uploads, validation, quote, and precheck do not authorize a hosted run.
4. **Confirm every run by default.** Show the current server-provided estimate and obtain explicit confirmation in the current conversation before creating a hosted run, except during an explicitly activated Test Execution Mode defined below.
5. **Reconfirm changed input.** If input changes after validation, estimate, or confirmation, validate and estimate again and obtain a new confirmation. Tell the user that the changed input must be re-estimated and reconfirmed.
6. **Use execution IDs safely.** Every newly confirmed execution gets a new `client-request-id`. Reuse one only for an identical retry of the same confirmed request after an ambiguous failure.
7. **Do not bypass Market.** Execute a SkillBot through its Listing and public schema. Never expose or reconstruct hidden prompts, steps, mappings, TemplateSpec, or private execution logic.
8. **Respect order locking.** Market resolves the current sellable Listing Version when creating the order and locks that version and price for the order.
9. **Confirm persistent changes.** Creating or versioning a template, publishing or changing a Listing, unlisting/relisting, and withdrawing review require explicit confirmation.
10. **Do not invent facts.** Never guess IDs, versions, flags, currency, fee fields, server state, hidden step IDs, settlement outcomes, or URLs.
11. **Use current sources of truth.** Use CLI help for syntax, CLI-bundled TemplateSpec docs for the contract, downloaded workbooks for input shape, and returned service fields for state and IDs.
12. **Keep technical details internal by default.** Translate raw JSON and backend fields into business language unless the user asks for CLI/API details.
13. **Avoid loading large files into context.** Upload them and preserve returned IDs according to `execution.md`.
14. **Use platform-specific official API documentation when needed.** When a task requires HTTP API details not covered by the local references—such as endpoints, authentication, request or response fields, OpenAPI definitions, or newly introduced APIs—first determine the current LoomLoom platform from the user's explicit selection or a successful `loomloom doctor --output json` result.

    For ShengSuanYun, consult the official LoomLoom API documentation:

    `https://lean.shengsuanyun.com/apidocs/loomloom/api`

    Treat this documentation as the authoritative source for ShengSuanYun API contracts. Do not guess or fabricate behavior that can be verified there.

    CogFoundry API documentation is not yet publicly available. Do not use ShengSuanYun-specific API contracts to infer CogFoundry behavior. For CogFoundry, rely on the local references, installed CLI help, and actual service responses until official CogFoundry API documentation becomes available.

## Test Execution Mode

Test Execution Mode is an optional, conversation-scoped prompt authorization for repeated paid test runs. It is disabled by default and available to every user and Agent that uses this Skill. It is not a server-side authorization or billing-control mechanism.

Activate it only when the user explicitly says they do not need a confirmation before each test run and supplies, or explicitly accepts, all of these limits:

- target environment / Server;
- permitted template, private-template version, or Listing scope;
- maximum tasks per run;
- maximum cost per run and total cost, with currency;
- expiry (default: the current conversation only).

On activation, restate the limits once and record that Test Execution Mode is active. Within those exact limits, the Agent may validate, precheck, submit, watch, and retrieve results without requesting another per-run confirmation. However, a missing, zero, or currency-unknown precheck estimate is not eligible for automatic submission: stop and obtain a fresh explicit confirmation before creating that run. Each run must still use a new `client-request-id` and report its run ID, status, and actual cost when returned.

Test Execution Mode immediately stops and normal per-run confirmation resumes when any limit is exceeded, the input scope changes, the Server/platform changes, the mode expires, or the user withdraws it. Never infer activation from a testing context or from an earlier conversation.

The mode never covers persistent or high-impact operations: creating or versioning templates, publishing or changing Listings, unlisting/relisting, withdrawing reviews, deleting data, changing credentials or Server configuration, or any operation outside the approved execution scope. Those operations always require explicit confirmation.

## References

- [Setup and security](references/setup.md) — installation, `doctor`, credentials, platform messages, console, token/host safety.
- [Template execution](references/execution.md) — official/private workbook and JSON flows, runs, results, artifacts, large files.
- [Market and SkillBots](references/market.md) — Listings, public input, buyer and creator workflows, price/version changes.
- [Billing and confirmation](references/billing.md) — fee fields, currency, confirmation templates, settlement, earnings.
- [Template authoring](references/template-spec.md) — TemplatePlan conversation, usage modes, TemplateSpec modeling and creation.
- [Agent Skill Packages](references/local-skills.md) — Agent-created private packages, public ZIP install/update, and package review binding.
- [CLI and errors](references/cli.md) — syntax sources, ID chaining, recovery rules, complete capability inventory.
