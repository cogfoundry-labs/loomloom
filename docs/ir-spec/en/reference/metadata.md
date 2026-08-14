# Metadata reference

`meta.name` is required. Optional fields are description, scenario, inputSummary, displayOutputType, primaryOutputType, and tags.

| Field | Meaning |
| --- | --- |
| `name` | non-empty template name |
| `description` | purpose and main outcome |
| `scenario` | business scenario classification |
| `inputSummary` | short description of expected input |
| `displayOutputType` | display-oriented output hint |
| `primaryOutputType` | optional assertion of derived terminal capability |
| `tags` | string tags |

Core derives the primary output from frozen terminal contract outputs. If `primaryOutputType` is supplied, save-time validation requires it to match that result.

Metadata does not replace `templateInputs` or contract ports and does not change Artifact MIME. Market profile and pricing belong to a Listing, not TemplateSpec metadata.
