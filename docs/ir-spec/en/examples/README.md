# TemplateSpec v2 examples

| Scenario | File | Capability |
| --- | --- | --- |
| Multi-step model chain | `multi-step-fixed-model.json` | Step-scoped prompt, stepOutput, dependsOn |
| Artifact collection merge | `artifact-merge.json` | Parallel upstreams and ordered merge |
| Author preset plus user value | `compose-value.json` | composeValue concat |
| Ordered multimodal input | `content-sequence.json` | text/image sequence and role |
| Generated image into ordered input | `content-sequence-step-output.json` | literal, upstream Artifact, sequence, dependsOn |
| Selectable text model | `capability-profile.json` | Profile routing and default model |
| Code text and file delivery | [code-value-artifact.json](valid/code-value-artifact.json) | Required string input, Value, JSON Artifact |
| Code boolean condition | [code-condition.json](valid/code-condition.json) | Direct boolean when dependency and downstream consumption |
| Bounded text rework | [bounded-text-loop.json](valid/bounded-text-loop.json) | Fixed model, text-to-string input, explicit feedback, accepted exports and conditional delivery |

Subject revision and model IDs in examples are placeholders or test evidence. Replace them with records from the target environment. Ordinary Capability Profile bindings keep only the stable `profileId`; do not copy a discovered `profileRevision` into a template.

The `invalid` directory contains inputs rejected by Schema or the Core validator to preserve error boundaries.

New invalid cases cover nullable Code input, more than ten Loop iterations, and outside access to an unexported body output. Schema checks shape; Core/server checks scope, dependencies, port types, actual Profiles, and author permissions. Valid means structural/core validation and frozen readback tests pass, not that the example has executed on the current server. Replace the Loop model placeholder with a currently available target member. Code/Loop require controlled test admission.

Every example is a `canonicalSpecV2` object. Add `specVersion=template-spec/v2` and `canonicalSpecV2` in the version-save request envelope.
