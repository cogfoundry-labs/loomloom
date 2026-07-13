# Validate, create, and version

Run `loomloom template-spec check <file>` first. After reviewing the name, fields, steps, models, and check result, separately confirm and run `create`. Modify an existing template with `create-version`; historical versions are not overwritten. Re-download the workbook for the new version.

```bash
loomloom template-spec check ./template.json
loomloom template-spec create ./template.json
loomloom template-spec create-version <template-id> ./template.json
```

Check is local and creates nothing. Create and create-version are remote writes and require explicit confirmation. Preserve returned template/version IDs. If the server rejects a locally valid spec, inspect dynamic model availability, unit capability, port MIME, and the deployed server version.
