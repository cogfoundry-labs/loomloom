# Understand TemplateSpec

A TemplateSpec defines how every future input row is processed. `meta` describes the template, `steps` define processing, `inputSchema` defines user data, parameter bindings compose run parameters, and upstream bindings connect uploaded content or step outputs to typed ports.

The definition is separate from run input. Fixed requirements belong in step instructions; row-specific values use field keys. Versions are immutable snapshots, so structural changes require a new version and workbook. Local check is not execution: server creation validates current models, and runtime resolves assets and providers.

Continue with `concepts/inputs.md`, `concepts/bindings-and-data-flow.md`, and `concepts/validation-layers.md`. Use `reference/template-syntax.md` for exact JSON.
