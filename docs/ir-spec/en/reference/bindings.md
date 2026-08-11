# Bindings reference

<a id="ref-ports-and-bindings-input-transport"></a>
TS-IN-001 uses string for pasted text. TS-IN-003 forbids describing a string as an asset ID input.

<a id="ref-ports-and-bindings-uploaded-text"></a>
TS-IN-002 requires text_reference to enter a compatible port through initial_input, not prompt FieldBinding.

<a id="ref-ports-and-bindings-field-binding"></a>
FieldBinding maps one field to one allowed parameter. New authoring uses shared bindings. A multi-value content collection uses `sourceType=initial_input` and a port that accepts multiple contents instead of FieldBinding.

It contains fieldKey, stepId, paramKey, and bindMode. Model routing only supports shared FieldBinding and requires allowModelOverride. Provider and mode are rejected.

<a id="ref-ports-and-bindings-param-binding"></a>
ParamBinding combines single-value field_ref and literal sources in shared mode for new authoring.

Sources are ordered and joined by separator. Literals must be non-empty; field references must exist. Routing parameters do not support composition.

<a id="ref-ports-and-bindings-expanded-compatibility"></a>

## TS-TOPOLOGY-001: expanded compatibility boundary

New templates, new versions, and switching another historical version into the published position must not use `bindMode=expanded` in FieldBinding or ParamBinding. An already-published historical expanded version remains readable, precheckable, and runnable, and idempotently publishing that same current version remains allowed.

Migration: use workbook rows for independent items; declare multiple Steps for a fixed number of parallel branches; aggregate upstream results with `dependsOn` / `upstreamBindings`. TemplateSpec v1 does not support dynamically creating Step branches from an input array.

<a id="ref-ports-and-bindings-upstream-binding"></a>
UpstreamBinding uses step_output or initial_input. Source IDs, ports, dependency, MIME, multiplicity, and merge policy must be compatible.

For step_output, provide sourceStepId and sourcePort and include the step in dependsOn. For initial_input, provide sourceInputKey. Duplicate target-port bindings are accepted only when that port allows multiple.
