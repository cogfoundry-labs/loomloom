# TemplateSpec v2 examples

| Scenario | File | Capability |
| --- | --- | --- |
| Multi-step model chain | `multi-step-fixed-model.json` | Step-scoped prompt, stepOutput, dependsOn |
| Artifact collection merge | `artifact-merge.json` | Parallel upstreams and ordered merge |
| Author preset plus user value | `compose-value.json` | composeValue concat |
| Ordered multimodal input | `content-sequence.json` | text/image sequence and role |
| Generated image into ordered input | `content-sequence-step-output.json` | literal, upstream Artifact, sequence, dependsOn |
| Selectable text model | `capability-profile.json` | Profile routing and default model |

Authority IDs in examples are placeholders or test evidence. Replace them with records from the target environment. The files contain `canonicalSpecV2` objects; add the request envelope when creating a version.
