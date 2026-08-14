# Steps

A Step requires `stepId`, `displayName`, and `executionBinding`. It may declare `dependsOn`, `triggerPolicy`, `modelSelection`, and `inputBindings`.

Dependencies control scheduling and must be acyclic. They do not move data automatically. A stepOutput binding must name a source that is also in `dependsOn`.

<a id="ref-profiles-model-selection"></a>

## TS-PROFILE-002: Profile model selection

A fixed model Step cannot declare `modelSelection`. A Profile Step requires it. The selected input must be an optional string without a static enum; blank selects `defaultModelId`, while a non-blank value is validated against current eligible Profile membership.
