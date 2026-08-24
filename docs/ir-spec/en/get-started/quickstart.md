# Quickstart

Copy [the multi-step fixed-model example](../examples/valid/multi-step-fixed-model.json), then replace each `subjectRevisionId` with an authority ID returned by the target environment.

Create a TemplateVersion with this request envelope:

```json
{
  "versionNote": "first v2 version",
  "specVersion": "template-spec/v2",
  "canonicalSpecV2": {
    "meta": {"name": "My template"},
    "templateInputs": {},
    "steps": [],
    "workbook": {}
  }
}
```

Validate with `loomloom template-spec check template.json` against the Server where the version will be created. The command uses the same current Subject/Profile authority resolution as version creation without saving. Creation validates again and freezes the contract.
