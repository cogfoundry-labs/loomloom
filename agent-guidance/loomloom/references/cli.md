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
- hosted submission → `runId` → `run watch`, `run get`, `run result-rows`, `run result-workbook`
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

- `loomloom template-spec docs [spec|authoring|examples|conversation|all]`
- `loomloom template-spec check <spec.json>`
- `loomloom template-spec models <text-generate|image-generate|video-generate>`
- `loomloom template-spec authoring-context --output json`
- `loomloom template-spec contracts <model-id>`
- `loomloom template-spec create <spec.json>`
- `loomloom template-spec create-version <template-id> <spec.json>`
- `loomloom template-spec list`
- `loomloom template-spec get <template-id>`
- `loomloom template-spec versions <template-id>`
- `loomloom template-spec download-workbook <template-id> <version-id>`
- `loomloom template-spec validate-workbook <template-id> <version-id> <xlsx-path>`
- `loomloom template-spec precheck-workbook <template-id> <version-id> <xlsx-path>`
- `loomloom template-spec submit-workbook <template-id> <version-id> <xlsx-path> --client-request-id <id>`
- `loomloom template-spec precheck <template-id> --version-id <version-id> --input-file-id <input_file_id>`
- `loomloom template-spec run <template-id> --version-id <version-id> --input-file-id <input_file_id> --client-request-id <id>`

`template-spec authoring-context` is the current target-environment discovery
entry for Capability Profiles. It returns Profile IDs, current revisions and
ports, plus eligible models. Agents must call it before authoring a
`capabilityProfile` Step and normally submit only `profileId`.

`template-spec contracts` lists enabled per-model `fixedModelContract`
authoring contracts. It is not a general model-usability check. In particular,
`text-generate` models normally return no per-model contracts because private
text Steps use the shared `capabilityProfile` shown by
`loomloom template-spec docs examples`. Do not report text authoring as
unavailable solely because `template-spec contracts <text-model-id>` is empty.

### Runs and artifacts

- `loomloom run validate <template-id> -f <rows.json-or-jsonl>`
- `loomloom run precheck <template-id> -f <rows.json-or-jsonl>`
- `loomloom run execute <template-id> -f <rows.json-or-jsonl> --client-request-id <id>`
- `loomloom run list`
- `loomloom run get <run-id>`
- `loomloom run watch <run-id>`
- `loomloom run result-rows <run-id>`
- `loomloom run result-workbook <run-id>`
- `loomloom artifact list <run-id>`
- `loomloom artifact download <run-id>`

### Catalog

- `loomloom model list --step-type <step-type>`
- `loomloom asset list`

`asset list` aggregates private templates and Market SkillBots; it does not include official templates.

### Market buyer and usage

- `loomloom market list`
- `loomloom market show <listing-id>`
- `loomloom market quote <listing-id> --input-file <request.json>`
- `loomloom market run <listing-id> --input-file <request.json> --confirm --client-request-id <id>`
- `loomloom market workbook download <listing-id> --output-file <xlsx>`
- `loomloom market workbook validate <listing-id> --file <xlsx>`
- `loomloom market workbook quote <listing-id> --file <xlsx>`
- `loomloom market workbook run <listing-id> --file <xlsx> --confirm --client-request-id <id>`
- `loomloom usage list`
- `loomloom usage get <run-transaction-id>`

### Market creator

- `loomloom listing publish <template-id> --template-version-id <id> --display-name <name> --task-fixed-fee <amount>`
- `loomloom listing list`
- `loomloom listing show <listing-id>`
- `loomloom listing versions <listing-id>`
- `loomloom listing update <listing-id>`
- `loomloom listing unlist <listing-id>`
- `loomloom listing relist <listing-id>`
- `loomloom listing withdraw <listing-id>`
- `loomloom creator earnings`
- `loomloom creator transactions`
- `loomloom creator review list`
- `loomloom creator review get <review-request-id>`
- `loomloom creator review withdraw <review-request-id>`

### Local Agent Skills

- `loomloom skill install market <listing-id> --agent <agent> --output-dir <dir>`
- `loomloom skill install template-spec <template-id> <version-id> --agent <agent> --output-dir <dir>`
- `loomloom skill uninstall --dir <skill-dir>`
