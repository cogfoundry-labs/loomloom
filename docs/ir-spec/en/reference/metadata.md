# Metadata reference

`meta.name` is required. Optional fields are description, scenario, inputSummary, displayOutputType, primaryOutputType, and tags. If primaryOutputType is supplied, it must match the capability derived from terminal steps; different terminal capabilities derive `mixed`.

| Field | Meaning |
| --- | --- |
| `name` | non-empty template name |
| `description` | purpose and main outcome |
| `scenario` | business scenario classification |
| `inputSummary` | short description of expected input |
| `displayOutputType` | display-oriented output hint |
| `primaryOutputType` | optional assertion of derived terminal capability |
| `tags` | string tags |

Metadata does not change artifact MIME or replace the input schema. Market profile and pricing belong to a Listing, not TemplateSpec metadata.
