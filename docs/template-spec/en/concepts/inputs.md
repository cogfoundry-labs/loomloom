# Inputs

Use `string` for pasted text, `enum` for fixed choices, `image_url` for web image URLs, and `asset_ref` or `text_reference` for uploaded references. Reference fields require accepted MIME types. `sourceKind` defaults to user input; default or hidden fields require a default. Multi-value fields require `maxValues > 0` and expanded binding. Presentation changes UI hints, not execution.

## Value types

| Type | Use | Required companion data |
| --- | --- | --- |
| `string` | Text entered directly in a row | none |
| `enum` | A controlled choice | non-empty `enumValues` |
| `image_url` | An HTTP/HTTPS image URL | none |
| `asset_ref` | An uploaded file reference | `acceptedMimeTypes` |
| `text_reference` | Inline or uploaded text reference | `acceptedMimeTypes` and a compatible input binding |

## Source and cardinality

An omitted source kind behaves as `user_input`. `default_value` and `hidden` require non-empty `defaultValue`. A multi-value field requires a positive maximum and must feed an expanded binding. Do not encode a list as comma-separated text unless the comma-separated string is itself the business value.

Presentation may set input/textarea/select, placeholder, hint, and examples. It never overrides type, MIME, or binding semantics.
