# Agent Skill Packages

Use this reference for a private template that needs local Agent capabilities, and when a user explicitly asks their current Agent to install or use an official template or Market SkillBot.

Local package installation is not template execution. It must not create a run, quote/precheck execution cost, call a Market SkillBot, or create billable model/API usage.

## Creator: private template with Agent capabilities

Before deciding whether a private template needs a local Skill, determine what the current LoomLoom Server can author and execute. Resolve the requested input/output modalities with `loomloom capability resolve`; inspect `template-spec authoring-context`, `template-spec contracts`, or `model list` when needed. Do not classify work from its title or apparent simplicity alone.

- If the supported TemplateSpec and hosted LoomLoom execution can complete the work, create the private template normally. Do not mention a Skill Package, create a local Skill, or ask the creator to choose a package mode.
- If a required capability is unavailable from the current Server and must be supplied through online search or current information, external websites or APIs, browser interaction, local code or scripts, local files or tools, HTML or special-format handling, or runtime decisions, classify the template as Agent-assisted.

For an Agent-assisted template, explain the required local capabilities and include the local Skill in the TemplatePlan before creating the private template. Ask whether the creator wants to continue with that complete Agent-assisted solution. Do not present package modes or backend package-generation behavior as user choices.

After the creator confirms the complete solution, create the private template to obtain its `template_id`, then create the required local Skill. Show the creator a preview before uploading: name, purpose, inputs, outputs, effect, permissions, and local capabilities. Obtain explicit upload confirmation, run the package locally by default, then upload the Agent-created ZIP. If the creator declines the required local assistance or upload, or if creation, trial, upload, or validation fails, stop; do not publish the incomplete template to Market.

When a custom package invokes LoomLoom, use the current platform's official API documentation where available. The package must implement the applicable secure authentication flow and, before every paid run, precheck or quote, present the returned fee, obtain the user's explicit confirmation, and only then submit the run.

```bash
loomloom skill package private upload <template-id> --file <agent-created.zip>
```

Read the private Package Head back after upload:

```bash
loomloom skill package private show <template-id>
```

Require `validationStatus=passed` before declaring the Agent-assisted authoring flow complete. Preserve its `archiveHash` and `validationId`, and pass both values through `--skill-package-archive-hash` and `--skill-package-validation-id` when publishing; the CLI uses them to freeze the exact creator-confirmed ZIP with the review. On every later replacement, repeat preview, confirmation, the default local trial, upload, and validation. The server validates the ZIP and replaces the Head with compare-and-swap; it does not record that local trial.

Treat upload, review binding, and public distribution as three separate states:

- The private Package Head with `validationStatus=passed` proves that the ZIP was uploaded and validated.
- `skillPackageReview.pending.id` proves that a ZIP version was frozen and bound to the current Market review.
- `skillPackage.available=true` proves that the approved ZIP is publicly downloadable.

Never infer that upload or review binding failed from `skillPackage.available=false`, `unavailableReason=listing_not_listed`, or an empty Listing `packageHash`. Those fields describe public distribution or the execution snapshot, not the pending review binding. If a pending Listing response does not contain `skillPackageReview`, report the binding state as unknown for the current Server version; do not report it as unbound.

To inspect or remove the private Head:

```bash
loomloom skill package private show <template-id>
loomloom skill package private detach <template-id> \
  --expected-archive-hash <hash> --expected-validation-id <id>
```

Use `--expected-archive-hash` and `--expected-validation-id` for a replacement or detachment whenever a current Head exists, so a stale Agent cannot overwrite another change. Detachment only removes the current package binding; it does not delete the private template, template versions, or historical ZIP archives. Before detaching, explain this effect and obtain the creator's explicit confirmation.

## Consumer: official-template and Market packages

Requests to accomplish a task with an official template or Market SkillBot include its installation preparation. For example, "use this SkillBot to review my document" triggers installation once the target is identified. A request solely to browse, explain, or quote follows the discovery or quotation workflow.

Determine the current Agent's Skill root from its runtime configuration or supported conventions. If the root cannot be determined, ask for that missing information. Run the appropriate command with this directory:

```bash
loomloom skill package install market <listing-id> --skill-root <current-agent-skill-root>
loomloom skill package install official <template-slug> --skill-root <current-agent-skill-root>
```

These `skill package install` commands install the backend-provided package. The older `skill install market` command generates a local wrapper and serves a separate compatibility workflow.

The CLI checks the local version, downloads the Skill Package ZIP when needed, verifies it, and extracts it into a Skill directory beneath the supplied Skill root. Installation success or `unchanged=true` means the Skill Package is available locally in the returned `dir`. Run the installation command even when the package may already be installed; the CLI decides whether an update is needed.

Proceed with this preparation as part of the user's use request. Keep progress messages focused on the task and any decisions needed from the user. Paid execution follows the separate quote/precheck and confirmation rules in `billing.md`.

### Installation exceptions

- A command error means installation failed. Briefly explain the reported problem. The CLI preserves the previous local package on failure.
- `available=false` means installation was skipped. Explain the returned reason in the user's language: `listing_not_listed` means the Listing is not currently listed; `package_removed` means its package was removed; `distribution_blocked` means distribution is blocked; `distribution_unavailable` means distribution is not currently available. An unfamiliar reason should be reported without guessing its meaning.
- For Market packages, the CLI handles `no_published_package` by attempting to obtain a standard package. Report the final installation result. For official templates, an unavailable package is reported as skipped.

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
