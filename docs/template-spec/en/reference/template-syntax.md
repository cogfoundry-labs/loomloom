# TemplateSpec syntax

Public JSON uses lowerCamel fields. Top-level `meta`, non-empty `steps`, and `inputSchema` are required; fieldBindings and paramBindings are optional. Unknown object properties are rejected by Schema. Field and step references are case-sensitive. See the machine JSON Schema for exact types.

| Top-level field | Required | Purpose |
| --- | --- | --- |
| `meta` | yes | identity and display metadata |
| `steps` | yes | one or more processing steps |
| `inputSchema` | yes | public row fields and guidance |
| `fieldBindings` | no | direct field-to-parameter mapping |
| `paramBindings` | no | composed parameter mapping |

Step IDs match `stp_<6-10 base36>`. Field keys model/provider/mode are reserved. Omitted sourceKind behaves as user_input, omitted triggerPolicy as require_all, and omitted upstream sourceType as step_output.
