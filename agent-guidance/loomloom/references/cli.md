# CLI

Use this reference whenever an exact CLI command, flag, returned ID, JSON output, supported capability, or error-recovery decision is needed.

## Contents

- [Inspect current syntax and inputs](#inspect-current-syntax-and-inputs)
- [Command chaining](#command-chaining)
- [Error recovery](#error-recovery)
- [Current CLI capabilities](#current-cli-capabilities)

## Inspect Current Syntax And Inputs

- Command syntax, positional arguments, and flags: `loomloom <command> --help`
- TemplateSpec JSON contract: `loomloom template-spec docs spec`
- TemplateSpec patterns: `loomloom template-spec docs examples`
- Natural-language authoring: `loomloom template-spec docs conversation`
- Official-template fields: `loomloom template schema <template-id> --output json`
- Market public fields: `loomloom market show <listing-id> --output json`
- Workbook input shape: download the workbook and inspect its headers/instructions
- Machine-readable chaining: use `--output json`

Common persistent flags:

- `--server <url>` / `-s`: use the specified LoomLoom Server for this command
- `--token <token>` / `-t`: use the specified Bearer Token for authentication; treat it as a sensitive credential
- `--timeout <duration>`: set the per-request HTTP timeout; the default is `30s`
- `--output text|json` / `-o`: select text or JSON output
- `--verbose` / `-v`: write diagnostic logs to stderr

`loomloom --version` prints the CLI version and exits.

Treat CLI help and returned service data as authoritative for syntax, IDs, state, and available fields. Do not invent a flag or field from memory.

## Command Chaining

Preserve exact values:

- `orchestration-input upload` → `inputFileId` → private precheck/run `--input-file-id`
- hosted submission → `runId` → `run watch`, `run get`, `run result-rows`, `run result-workbook`; author debugging → `run step-executions`
- `listing publish` → `reviewRequestId` → creator review commands
- `market run` / `market workbook run` → `runTransactionId` and `runId` → usage/run/result commands

Never convert `inputAssetId` (`ia_xxx`) into `inputFileId`, and never guess an ID from a display name.

For execution commands, pass an explicit `--client-request-id`. Reuse it only for an identical retry of the same confirmed request after an ambiguous failure. A new confirmation or changed payload requires a new ID.

## Error Recovery

- Local flag, file, JSON, workbook, or schema error: correct the input and retry; do not run `doctor` automatically.
- Authentication, endpoint, network, service-version, or unexpected service error: run `loomloom doctor`.
- Ambiguous paid or remote-state-changing failure: query the relevant run, Listing, usage transaction, or review state before retrying.
- Never invent missing IDs, hidden step IDs, fee fields, server state, or a successful outcome.

Translate developer fields into user-facing language unless raw JSON/API details were requested. For example:

- `forced_unlisted` → forcibly removed from the Market and currently unavailable
- `reviewStatus=rejected` → review was not approved
- `executionAvailabilityStatus=blocked` → currently unavailable to run

Keep raw field names for internal command chaining, not ordinary user summaries.

## Current CLI Capabilities

### Environment and inputs

- `loomloom login [--no-browser] [--login-timeout <duration>]`
- `loomloom logout`
- `loomloom doctor`
- `loomloom balance`
- `loomloom doctor --server <url> [--name <profile>] --output json`
- `loomloom server list`
- `loomloom server use <name-or-server>`
- `loomloom server remove <name-or-server>`
- `loomloom input-asset upload <file>`
- `loomloom orchestration-input upload <file.jsonl>`

When no Server profile is selected, bare `loomloom login` offers the preset-platform selector only in an interactive terminal. Agents, CI jobs, piped commands, and `--output json` invocations must first obtain the user's platform choice and then pass the matching preset Server with `--server`; never treat an implicit fallback as platform selection. Browser login is supported for the ShengSuanYun and CogFoundry presets. Custom Servers use explicit API Token authentication.

`login` waits up to five minutes for browser authorization by default. `--login-timeout` changes that human authorization window. The global `--timeout` flag remains the per-request HTTP timeout and applies to both the authorization-code exchange and credential verification. Non-positive values are invalid. A login profile is saved and activated only after the returned credential passes verification against the selected Server.

On successful `login --output json`, the `token` field is masked and is not a reusable credential. Do not extract, persist, or present it as an API Token. Login failures use a non-zero exit status and stderr; do not assume every failure is a JSON object.

`logout` removes only the saved browser credential for the selected Server profile. It does not remove the profile or any environment API Token; follow the cleanup rules in `setup.md` when the user requests those additional changes.

### Official templates

- `loomloom template list`
- `loomloom template schema <template-id>`
- `loomloom template download <template-id>`
- `loomloom template validate-file <template-id> <xlsx-path>`
- `loomloom template precheck-file <template-id> <xlsx-path>`
- `loomloom template submit-file <template-id> <xlsx-path> --client-request-id <id>`
- `loomloom template backfill-results <run-id> <xlsx-path>`

### TemplateSpec and private templates

- `loomloom capability resolve --input <modality> --output-modality <modality> --output json`
- `loomloom template-spec docs [spec|authoring|examples|conversation|all]`
- `loomloom template-spec check <spec.json>`
- `loomloom template-spec models <text-generate|image-generate|video-generate>`
- `loomloom template-spec authoring-context --output json`
- `loomloom model types --output json`
- `loomloom model list --step-type <step-type> --output json`
- `loomloom template-spec contracts <model-id>`
- `loomloom template-spec create <spec.json>`
- `loomloom template-spec create-version <template-id> <spec.json>`
- `loomloom template-spec list`
- `loomloom template-spec get <template-id>`
- `loomloom template-spec versions <template-id>`
- `loomloom template-spec get-version <template-id> <version-id> [-f spec.json]`
- `loomloom template-spec preview <spec.json> [--viewer-url <HTTPS bootstrap>] [--no-open] [--title <title>]`
- `loomloom template-spec view-link <template-id> <version-id>`
- `loomloom template-spec download-workbook <template-id> <version-id>`
- `loomloom template-spec validate-workbook <template-id> <version-id> <xlsx-path>`
- `loomloom template-spec precheck-workbook <template-id> <version-id> <xlsx-path>`
- `loomloom template-spec submit-workbook <template-id> <version-id> <xlsx-path> --client-request-id <id>`
- `loomloom template-spec estimate <template-id> --version-id <version-id> --input-file-id <input_file_id>`
- `loomloom template-spec precheck <template-id> --version-id <version-id> --input-file-id <input_file_id>`
- `loomloom template-spec run <template-id> --version-id <version-id> --input-file-id <input_file_id> --client-request-id <id>`

`capability resolve` is the primary task-oriented discovery entry. It asks the
current Server to map input/output modalities to authorable Capability Profiles
and Fixed Model Contracts. A `supported` result describes TemplateSpec
authoring eligibility, not balance, channel capacity, or instantaneous provider
availability. Agents should use the returned match before choosing lower-level
commands.

`template-spec preview --output json` is a long-running NDJSON stream, not a
single JSON response. Keep its process alive while editing; return `ready.viewUrl`
before waiting for completion. Preview uses the ShengSuanYun production viewer
by default; `--viewer-url` overrides `LOOMLOOM_VIEWER_BOOTSTRAP_URL`, which
overrides that default. It does not save or run the draft. Read
[workflow-preview.md](workflow-preview.md) for lifecycle, errors, and cleanup.
`view-link` returns one exact saved version's link and needs no active preview.

`template-spec authoring-context` is the lower-level target-environment query
for all Capability Profiles. For a dynamic Profile it returns the stable
`profileId`, fixed input/output `definition`, current operational default and
`defaultModelAvailable`, plus the live `eligibleModels`. Dynamic Profiles have
no authoring revision; normal authoring submits only `profileId` and omits
`profileRevision`. Use this command for diagnosis or full Profile inventory;
normal authoring should consume the Profile returned by `capability resolve`.

`template-spec contracts` lists enabled per-model `fixedModelContract`
authoring contracts. It is not a general model-usability check. In particular,
`text-generate` models normally return no per-model contracts because private
text Steps use the shared `capabilityProfile` shown by
`loomloom template-spec docs examples`. Do not report text authoring as
unavailable solely because `template-spec contracts <text-model-id>` is empty.

`template-spec check` calls the selected LoomLoom Server and uses the same
current contract-authority resolver as version creation. It is not an offline
schema check. A successful check is evidence for that Server at that time;
version creation validates again because authoring authority can change.

`template-spec get-version` returns the stored owner-visible authoring spec for
one immutable historical version. It intentionally does not return the frozen
execution bundle or apply current TemplateSpec rules to historical authoring.
Successful readback means only that the stored authoring is a JSON object. Use
`-f` to save `canonicalSpec` as a JSON file before a manual v1-to-v2 rewrite;
run `check` separately only for the rewritten v2 candidate, and never overwrite
the historical version.

### Runs and artifacts

- `loomloom run validate <template-id> -f <rows.json-or-jsonl>`
- `loomloom run precheck <template-id> -f <rows.json-or-jsonl>`
- `loomloom run execute <template-id> -f <rows.json-or-jsonl> --client-request-id <id>`
- `loomloom run list`
- `loomloom run get <run-id>`
- `loomloom run step-executions <run-id> [--task-id <id>] [--page-size <n>] [--page-token <token>]` (own template runs only: node inputs/outputs, Code error traceback, Loop rounds)
- `loomloom run watch <run-id>`
- `loomloom run result-rows <run-id>`
- `loomloom run result-workbook <run-id>`
- `loomloom artifact list <run-id>`
- `loomloom artifact download <run-id>`

### Catalog

- `loomloom model list --step-type <step-type>`
- `loomloom asset list`

`asset list` aggregates private templates and Market SkillApps; it does not include official templates.

### Market buyer and usage

- `loomloom market list [--keyword <text>] [--category <code>] [--sort-by default|usage|newest] [--page <n>] [--page-size <1-15>]`
- `loomloom market show <resource-id> [--source market|official]`
- `loomloom market quote <listing-id> --input-file <request.json>`
- `loomloom market run <listing-id> --input-file <request.json> --confirm --client-request-id <id>`
- `loomloom market workbook download <listing-id> --output-file <xlsx>`
- `loomloom market workbook validate <listing-id> --file <xlsx>`
- `loomloom market workbook quote <listing-id> --file <xlsx>`
- `loomloom market workbook run <listing-id> --file <xlsx> --confirm --client-request-id <id>`
- `loomloom usage list`
- `loomloom usage get <run-transaction-id>`

### Subscription buyers

- `loomloom market bundle list [--creator-id <user-id>] [--name <name>]`
- `loomloom market bundle show <bundle-id>`
- `loomloom market bundle items <bundle-id>`
- `loomloom market subscribe <listing-id> --tier monthly|yearly [--confirm --expected-revision <bundle-revision> --client-request-id <id>]`
- `loomloom subscription list`
- `loomloom subscription show <subscription-id>`
- `loomloom subscription items <subscription-id> [--snapshot]`
- `loomloom subscription buy <bundle-id> --tier monthly|yearly [--confirm --expected-revision <bundle-revision> --client-request-id <id>]`
- `loomloom subscription renew <subscription-id> --tier monthly|yearly [--confirm --expected-revision <bundle-revision> --client-request-id <id>]`

Without `--confirm`, purchase and renewal commands return the selected tier and
bundle for price inspection without submitting a purchase. Read `market.md`
and `billing.md` before confirming a purchase or renewal. Renewal takes a
subscription ID, while purchase takes a bundle ID; `market subscribe` resolves
the Listing's automatic bundle. `subscription items --snapshot` reads the
immutable purchase membership rather than current bundle members.

Subscription and bundle lists, `market bundle items`,
`creator bundle list-available`, `creator bundle items`, and
`subscription items` use `--page-size` (default 20,
1–100) and `--page-token`. Read `nextPageToken` from the previous response;
these commands do not use `market list`'s page-number pagination.

### Market creator

- `loomloom listing publish <template-id> --template-version-id <id> --display-name <name> --task-fixed-fee <amount>`
- `loomloom listing publish <template-id> --template-version-id <id> --display-name <name> --task-fixed-fee 0 --pay-per-use=false --subscription=true --subscription-tiers-file <tiers.json>`
- `loomloom listing payment-settings <listing-id> --pay-per-use=<bool> --subscription=<bool> --expected-revision <revision> [--subscription-tiers-file <tiers.json>]`
- `loomloom creator bundle list`
- `loomloom creator bundle show <bundle-id>`
- `loomloom creator bundle set-tier <bundle-id> --tier monthly|yearly --price <amount> --currency CNY --enabled=<bool> --expected-revision <bundle-revision>`
- `loomloom creator bundle list-available`
- `loomloom creator bundle create --name <name> --client-request-id <id> [--description <text>]`
- `loomloom creator bundle items <bundle-id>`
- `loomloom creator bundle update <bundle-id> --name <name> --expected-revision <bundle-revision> [--description <text>]`
- `loomloom creator bundle add-item <bundle-id> --listing-id <listing-id> --expected-revision <bundle-revision>`
- `loomloom creator bundle publish <bundle-id> --expected-revision <bundle-revision>`
- `loomloom creator bundle unlist <bundle-id> --expected-revision <bundle-revision> [--reason <text>]`
- `loomloom listing list`
- `loomloom listing show <listing-id>`
- `loomloom listing versions <listing-id>`
- `loomloom listing download-skill-package <listing-id> --version-id <skill-package-version-id> --output-file <zip>`
- `loomloom listing update-skill-package <listing-id> [--skill-package-archive-hash <hash> --skill-package-validation-id <id>] [--request-id <id>]`
- `loomloom listing update <listing-id>`
- `loomloom listing unlist <listing-id>`
- `loomloom listing relist <listing-id>`
- `loomloom listing withdraw <listing-id>`
- `loomloom creator earnings [--source all|pay-per-use|subscription] [--limit 1..100] [--page-token <token>]`
- `loomloom creator transactions [--source pay-per-use|subscription]`
- `loomloom creator review list`
- `loomloom creator review get <review-request-id>`
- `loomloom creator review withdraw <review-request-id>`

`listing publish` and `listing update` accept `--category`, `--target-audience`,
`--problem-solved`, repeatable `--scenario`, `--not-suitable-for`, and
`--update-summary` for review. Read `market.md` for omission, clearing, merging,
and the pending-category readback limitation.

`creator bundle list` also accepts `--name` and `--status draft|published|unlisted`.
Read current bundle revision before each mutation; it is distinct from the
Listing's payment-settings revision. `add-item` accepts repeated `--listing-id`
flags or comma-separated IDs and applies only to manual bundles. `update`
requires a name and sends an empty description when `--description` is omitted;
pass the current description explicitly to preserve it. Follow `market.md` for
bundle creation, publication, and existing-subscription behavior. Read
`local-skills.md` before `listing update-skill-package`; archive hash and
validation ID must be supplied together, and omission selects `auto` mode.

### Local Agent Skills

- `loomloom skill package private upload <template-id> --file <agent-created.zip>`
- `loomloom skill package private show <template-id>`
- `loomloom skill package private download <template-id> --output-file <zip>`
- `loomloom skill package private detach <template-id> [--expected-archive-hash <hash> --expected-validation-id <id>]`
- `loomloom skill package install market <listing-id> --skill-root <current-agent-skill-root>`
- `loomloom skill package install official <template-slug> --skill-root <current-agent-skill-root>`

#### Legacy wrappers (compatibility)

These commands belong to the local-wrapper workflow described in [local-skills.md](local-skills.md). Use the ZIP commands above for Market and official-template package installation; wrapper uninstall does not support installed ZIP packages.

- `loomloom skill install market <listing-id> --agent <agent> --output-dir <dir>`
- `loomloom skill install template-spec <template-id> <version-id> --agent <agent> --output-dir <dir>`
- `loomloom skill uninstall --dir <skill-dir>`
