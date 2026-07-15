# TemplateSpec quickstart

Copy `examples/valid/single-text-generation.json`. Query the target environment with `loomloom template-spec models text-generate` and replace the fixture model ID. Change the template name, description, step instruction, and input presentation without breaking referenced field keys or step IDs.

Run:

```bash
loomloom template-spec check ./template.json
```

A valid result confirms local structure and authoring rules only. Server creation still validates dynamic models and capabilities. Remote create/create-version requires separate confirmation.
