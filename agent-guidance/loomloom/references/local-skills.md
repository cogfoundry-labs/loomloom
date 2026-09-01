# Agent Skill Packages

Use this reference for a private template that needs local Agent capabilities, and when a user explicitly asks their current Agent to install or use a Market or official SkillBot.

Local package installation is not template execution. It must not create a run, quote/precheck execution cost, call a Market SkillBot, or create billable model/API usage.

## Creator: private template with Agent capabilities

Before creating a private template, classify the intended work:

- If LoomLoom alone can execute it, create the private template normally and use automatic package handling. Do not create or upload a custom Agent Skill Package, and do not ask the creator to choose a package mode.
- If it needs Agent-side code, files, HTML generation, scripts, or runtime decisions, explain the Agent-side work and, after the private template has a `template_id`, ask the creator to choose:

```text
A. Create and upload a custom Skill Package
B. Use automatic package handling without uploading a custom ZIP
```

Do not expose raw request fields as the user-facing choice. Internally, A maps to `skillPackage.mode=archive`. For B, a new Listing or changed Template Version uses `auto`, while an unchanged Template Version preserves the current public package.

For B, do not build or upload a custom ZIP. When publishing, omit `--skill-package-archive-hash` and `--skill-package-validation-id`. For a new Listing, the Server applies `auto`. For an existing Listing, the CLI compares the requested Template Version with the currently published Listing Version: it preserves the current package when the Template Version is unchanged, and sends `auto` when the Template Version changes.

For A, the Agent builds the generic Skill folder and ZIP locally. Show the creator a preview before uploading: name, purpose, inputs, outputs, effect, permissions, and local capabilities. Obtain explicit confirmation, run the package locally by default, then upload the Agent-created ZIP:

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

## Consumer: Market and official packages

Only trigger package installation when the user clearly tells the current Agent to **install** or **use** that Market/official SkillBot. Browsing, quoting, explaining, or merely showing a SkillBot does not authorize installation.

The Agent determines its own Skill root. Do not hard-code Codex, Claude, or OpenClaw paths, and do not ask the user to choose a platform-specific destination. Pass the root to the CLI:

```bash
loomloom skill package install market <listing-id> --skill-root <current-agent-skill-root>
loomloom skill package install official <template-slug> --skill-root <current-agent-skill-root>
```

Do not use `loomloom skill install market`. It is a legacy local-wrapper generator retained only for compatibility; it does not download the reviewed backend ZIP and is not the Market SkillBot installation flow.

This is an automatic action after the user's explicit install/use request: do not ask a second confirmation. First read the public package summary. If no public ZIP is available, do not download or trigger package generation; continue with the normal LoomLoom cloud workflow. When a public ZIP is available, compare its `archiveHash` with the current Agent's local `.loomloom-package.json`: if it matches, keep the installed Skill; if it differs, download, verify, and atomically replace the same-source local Skill directory. A same-name directory owned by another source or not managed by LoomLoom is a name conflict and must not be overwritten. Package Skill names must remain stable across versions. If download, validation, extraction, or replacement fails, retain the previous local package.

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

Pure LoomLoom templates are published normally; the platform generates their standard ZIP internally. A creator receives the detail/preview page immediately while review is pending. After approval, the package is publicly downloadable. If rejected, the detail stays creator-visible and the CLI must show the returned `reviewReason`; do not invent a separate “suggestions” field.
