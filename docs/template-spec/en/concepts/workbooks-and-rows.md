# Workbooks and rows

A workbook belongs to one template version. Columns use field keys for mapping and labels for people, plus ordering, enum, hints, and source behavior. Usually one row is one task. Validate and precheck before submit; only submit creates a hosted run. Download a new workbook after version changes.

## Projection rules

The service derives columns from input fields. Keys remain machine identifiers, labels become headers, order controls placement, and enum/presentation data creates validation and help. Hidden fields are not entered by users.

## Recommended flow

```text
download-workbook -> fill -> validate-workbook -> precheck-workbook
-> show estimate and confirm -> submit-workbook
```

Validation creates no run. Precheck estimates execution and cost. Submit is the state-changing operation. A multi-value + expanded field can create several step executions within one row/task.
