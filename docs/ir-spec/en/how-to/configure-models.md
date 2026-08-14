# Configure models

Use `fixedModelContract` with `subjectRevisionId` for an exact model. Use a Capability Profile plus separate `modelSelection` when callers may choose among eligible models.

Do not embed a complete contract, provider parameters, or routing values in `executionBinding`. Query authority IDs from the target environment.
