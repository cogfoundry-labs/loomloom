# Input Schema reference

Fields require unique non-empty key and label plus valueType. Keys model/provider/mode are reserved. Enum requires values; asset and text references require MIME types; multi-value requires maxValues. Non-user source kinds require defaultValue. Presentation supports input/textarea/select, placeholder, hint, and examples. Sample row values use field keys inside `values`.

## inputSchema properties

| Property | Meaning |
| --- | --- |
| `fields` | required, non-empty field definitions |
| `instructions` | workbook/input-level filling instructions |
| `sampleRows` | examples keyed by field key inside `values` |

## input field properties

| Field property | Condition or meaning |
| --- | --- |
| `key`, `label`, `valueType` | always required |
| `description` | business meaning; must agree with valueType |
| `required` | user input must provide a value |
| `enumValues` | non-empty for enum |
| `acceptedMimeTypes` | non-empty for asset_ref/text_reference |
| `multiValue`, `maxValues` | positive max when multi-value |
| `order` | workbook column order |
| `defaultValue` | required for default_value/hidden source |
| `hidden`, `sourceKind` | visibility and value ownership |
| `presentation` | widget, placeholder, hint, examples |

Sample rows use `{ "values": { "field_key": "value" } }`; use keys, never labels. Presentation never weakens type or execution validation.
