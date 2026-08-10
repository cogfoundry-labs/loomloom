# Local Skills

Use this reference when installing a Market SkillBot or private template version as a local Codex, Claude Code, or OpenClaw Skill, or when uninstalling such a Skill.

Local Skill installation is not template execution. It must not create a run, quote/precheck execution cost, call a Market SkillBot, or create billable model/API usage.

## Install

Always preview before writing files:

```bash
loomloom skill install market <listing-id> \
  --agent <codex|claude|openclaw> \
  --output-dir <skill-dir> \
  --dry-run \
  --output json

loomloom skill install template-spec <template-id> <version-id> \
  --agent <codex|claude|openclaw> \
  --output-dir <skill-dir> \
  --dry-run \
  --output json
```

Show an installation confirmation containing:

- generated Skill name
- source type and source ID
- binding behavior
- target agent
- exact output directory
- main inputs
- when Preview fields include `enumValues`, the allowed choices as part of the main input summary
- reminder that every future real run still requires quote/precheck and explicit confirmation

Generated Skill names always use the `loomloom-` prefix. The final output directory basename must equal the previewed `skillName`.

If the user supplies only a parent skills directory and preview returns `blockingReason=output_dir_name_mismatch`:

1. Read the returned `skillName`.
2. Append it to the parent directory.
3. Rerun preview with that exact directory.
4. Show the confirmation only after the corrected preview succeeds.

Do not install into an unprefixed directory. If no directory is supplied, ask for one rather than guessing a platform default.

After explicit installation confirmation, repeat the same command without `--dry-run`.

## Binding Rules

- A Market Skill install binds to the Listing. The installed Listing Version is traceability only. Future runs must inspect the current Listing and use Market quote/run or Market workbook quote/run.
- A private template install binds to the exact `template_id + version_id` and must use `template-spec` commands.
- Stop if the Listing is unavailable, permissions fail, or the required version cannot be used.

## Uninstall

Ask for the exact Skill directory when it is not already known.

Preview:

```bash
loomloom skill uninstall --dir <skill-dir> --dry-run --output json
```

Show:

- Skill name
- source and agent
- exact directory
- files that will be deleted

Wait for explicit confirmation, then run:

```bash
loomloom skill uninstall --dir <skill-dir>
```

Do not delete the directory manually. If preview reports unexpected files, explain them and use `--force` only after the user explicitly confirms removal of the entire directory.
