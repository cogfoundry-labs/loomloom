# Validate, create, and version

Run `loomloom template-spec check template.json` against the target Server, then create with `specVersion=template-spec/v2` and `canonicalSpecV2`. Check and create share current authority resolution; creation validates again and freezes the contracts.

Use `loomloom template-spec get-version <template-id> <version-id> -f historical.json` to export an owner-visible historical authoring spec. It does not export the frozen execution bundle. Historical v1 definitions must be manually rewritten as v2 before creating a new immutable version.

Append a new immutable TemplateVersion for changes. Download a new Workbook and validate/precheck before submitting a Run.
