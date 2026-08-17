# TemplateSpec v2 examples

| Scenario | File | Capability |
| --- | --- | --- |
| Multi-step model chain | `multi-step-fixed-model.json` | Step-scoped prompt, stepOutput, dependsOn |
| Artifact collection merge | `artifact-merge.json` | Parallel upstreams and ordered merge |
| Author preset plus user value | `compose-value.json` | composeValue concat |
| Ordered multimodal input | `content-sequence.json` | text/image sequence and role |
| Generated image into ordered input | `content-sequence-step-output.json` | literal, upstream Artifact, sequence, dependsOn |
| Selectable text model | `capability-profile.json` | Profile routing and default model |

Subject revision and model IDs in examples are placeholders or test evidence. Replace them with records from the target environment. Ordinary Capability Profile bindings keep only the stable `profileId`; do not copy a discovered `profileRevision` into a template.

The `invalid` directory contains inputs rejected by Schema or the Core validator to preserve error boundaries.

Every example is a `canonicalSpecV2` object. Add `specVersion=template-spec/v2` and `canonicalSpecV2` in the version-save request envelope.
