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

Validate locally with `loomloom template-spec check template.json`. A local pass proves structure and stable authoring rules only. Core still resolves Subject/Profile records, validates ports, and freezes the contract at save time.
