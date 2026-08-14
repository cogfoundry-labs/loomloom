# Template Inputs and Workbook

A value input declares `kind=value`, `valueType`, `blankPolicy`, optional constraints/default, and presentation. Types are string, number, integer, boolean, array, and object.

An Artifact input declares `kind=artifact`, accepted MIME types, min/max items, blank policy, and presentation. Artifact values are stable platform references, not arbitrary URLs or Base64.

`required=true` pairs with `blankPolicy=error`; optional inputs use `blankPolicy=omit`. Workbook columns derive from these inputs. Sample row keys must reference existing inputs.
