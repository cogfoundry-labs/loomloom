# Agent Skill Packages

Use this reference for a private template that needs local Agent capabilities, and when a user explicitly asks their current Agent to install or use an official template or Market SkillBot.

Local package installation is not template execution. It must not create a run, quote/precheck execution cost, call a Market SkillBot, or create billable model/API usage.

## Creator: private template with Agent capabilities

Before deciding whether a private template needs a local Skill, determine what the current LoomLoom Server can author and execute. Resolve the requested input/output modalities with `loomloom capability resolve`; inspect `template-spec authoring-context`, `template-spec contracts`, or `model list` when needed. Do not classify work from its title or apparent simplicity alone.

- If the supported TemplateSpec and hosted LoomLoom execution can complete the work, create the private template normally. Do not mention a Skill Package, create a local Skill, or ask the creator to choose a package mode.
- If it needs Agent-local code, files, scripts, HTML output handling, local tools, or runtime decisions, explain the Agent-side work. The Agent creates the required local Skill and asks the creator, during the private-template creation flow, whether that local Skill Package should be uploaded and bound to the template:

```text
A. Upload and bind the local Skill Package
B. Do not upload the local Skill Package
```

Do not expose raw request fields as the user-facing choice. The actual upload and binding require a `template_id`, so obtain it before uploading. Internally, A maps to `skillPackage.mode=archive`.

For B, keep the local Skill unbound and do not upload a custom ZIP. When publishing, omit `--skill-package-archive-hash` and `--skill-package-validation-id`.

For A, show the creator a preview before uploading: name, purpose, inputs, outputs, effect, permissions, and local capabilities. Obtain explicit confirmation, run the package locally by default, then upload the Agent-created ZIP:

When a custom package invokes LoomLoom, use the current platform's official API documentation where available. The package must implement the applicable secure authentication flow and, before every paid run, precheck or quote, present the returned fee, obtain the user's explicit confirmation, and only then submit the run.

```bash
loomloom skill package private upload <template-id> --file <agent-created.zip>
```

The upload response is the private Package Head. Preserve its `archiveHash` and `validationId`. When publishing A, pass both values through `--skill-package-archive-hash` and `--skill-package-validation-id`; the CLI sends `skillPackage.mode=archive`. On every later replacement, repeat preview, confirmation, and the default local trial before uploading. The server validates the ZIP and replaces the Head with compare-and-swap; it does not record that local trial.

To inspect or remove the private Head:

```bash
loomloom skill package private show <template-id>
loomloom skill package private detach <template-id> \
  --expected-archive-hash <hash> --expected-validation-id <id>
```

Use `--expected-archive-hash` and `--expected-validation-id` for a replacement or detachment whenever a current Head exists, so a stale Agent cannot overwrite another change. Detachment only removes the current package binding; it does not delete the private template, template versions, or historical ZIP archives. Before detaching, explain this effect and obtain the creator's explicit confirmation.

## Consumer: official-template and Market packages

Only trigger package installation when the user clearly tells the current Agent to **install** or **use** a selected official template or Market SkillBot. Listing, browsing, quoting, explaining, or merely showing it does not authorize installation. Once the template slug or Listing ID is known and the user has made that explicit request, check or install the package before downloading a workbook, reading the concrete input schema, preparing actual input, quoting, or executing.

The Agent determines its own Skill root. Do not hard-code Codex, Claude, or OpenClaw paths, and do not ask the user to choose a platform-specific destination. Pass the root to the CLI:

```bash
loomloom skill package install market <listing-id> --skill-root <current-agent-skill-root>
loomloom skill package install official <template-slug> --skill-root <current-agent-skill-root>
```

Do not use `loomloom skill install market`. It is a legacy local-wrapper generator retained only for compatibility; it does not download the reviewed backend ZIP and is not the Market SkillBot installation flow.

This is an automatic action after the user's explicit install/use request: do not ask a second confirmation. First read the public package summary; do not skip this check. If no public ZIP is available, do not download or trigger package generation; continue with the normal LoomLoom cloud workflow. When a public ZIP is available, compare its `archiveHash` with the current Agent's local `.loomloom-package.json`: if it matches, keep the installed Skill; if it differs, download, verify, and atomically replace the same-source local Skill directory. A same-name directory owned by another source or not managed by LoomLoom is a name conflict and must not be overwritten. Package Skill names must remain stable across versions. If download, validation, extraction, or replacement fails, retain the previous local package.

If the public package is unavailable, do not install anything and do not treat the SkillBot as unexecutable: continue with the normal LoomLoom cloud workflow.

## Market publication

When publishing a composite template, freeze the confirmed private Head together with the template version:

```bash
loomloom listing publish <template-id> \
  --template-version-id <version-id> \
  --display-name <name> \
  --task-fixed-fee <amount> \
  --skill-package-archive-hash <archive-hash> \
  --skill-package-validation-id <validation-id>
```

For an already published Listing, submit only the new Package Head for review:

```bash
loomloom listing update-skill-package <listing-id> \
  --skill-package-archive-hash <archive-hash> \
  --skill-package-validation-id <validation-id>
```

When `listing update-skill-package` is used without the archive/validation tuple, the CLI sends `skillPackage.mode=auto`. Supplying both tuple flags sends `skillPackage.mode=archive`. Do not supply only one tuple field.

Pure LoomLoom templates are published normally. Creators can use the CLI to view listing and review status. If rejected, show the returned `reviewReason`; do not invent a separate “suggestions” field.
