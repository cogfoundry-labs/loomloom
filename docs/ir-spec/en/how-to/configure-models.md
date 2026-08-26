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

Text output does not imply text-only input. Choose the Profile returned by the
target environment:

- `text.basic.openai-chat.v1`: `prompt:string` input and text output.
- `text.vision.openai-chat.v1`: `prompt:string` plus one `image:Artifact` input
  and text output.

Bind the Vision Profile `image` port to an Artifact Template Input and honor
its `acceptedMimeTypes`, `minItems`, and `maxItems`. Do not declare an uploaded
asset ID as a string and place it in `prompt`. Use only models returned in that
Vision Profile's `eligibleModels`; never infer vision support from a model name.
