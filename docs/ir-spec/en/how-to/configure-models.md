# Configure models

Use `fixedModelContract` with `subjectRevisionId` for an exact model:

```json
"executionBinding": {"kind": "fixedModelContract", "subjectRevisionId": "..."}
```

Use a Capability Profile plus separate `modelSelection` when callers may choose among eligible models. Do not embed a complete contract, provider parameters, or routing values in `executionBinding`. First query the target environment:

```bash
loomloom template-spec authoring-context --output json
```

It returns current Profiles, their revision, ports, and eligible models. Ordinary templates write the returned `profileId` only, without `profileRevision`; Core freezes the revision and hash when the version is saved.
