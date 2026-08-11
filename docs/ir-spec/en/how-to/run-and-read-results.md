# Run and read results

For workbooks: download, fill, validate, precheck, show the current estimate, obtain explicit confirmation, submit, watch, then download the result workbook. Use result rows for structured output and artifact commands for individual files. Input asset IDs and orchestration input file IDs are different identifiers.

```text
download -> fill -> validate -> precheck -> show estimate and confirm
-> submit -> watch -> result-workbook
```

Validation and precheck are preparation; submit creates the hosted run. For programmatic input, use field keys from the exact version and never guess step IDs. After completion, use run get/watch for status, result rows for structured output, result workbook for server-aligned delivery, and artifact list/download for individual files.

```bash
loomloom run watch <run-id>
loomloom run get <run-id>
loomloom run result-rows <run-id>
loomloom run result-workbook <run-id>
loomloom artifact list <run-id>
```
