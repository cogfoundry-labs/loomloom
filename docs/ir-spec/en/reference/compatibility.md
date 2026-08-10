# Compatibility

Author new JSON in lowerCamel. Some CLI PascalCase normalization is legacy compatibility, not the canonical format. Versions and workbooks are immutable/version-bound. CLI historical docs topics may remain aliases to new pages. New clients use `/loom/v1`; `/batch/v1` is legacy.

Changing a field key, type, ordering contract, step ID, or binding requires a new version. A new private template version does not mutate an existing Market Listing Version. Old workbook compatibility is not promised. CLI aliases such as spec/authoring/examples/conversation are navigation compatibility only; they do not create additional documentation sources.
