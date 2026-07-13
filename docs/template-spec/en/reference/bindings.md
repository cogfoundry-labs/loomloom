# Bindings reference

<a id="ref-ports-and-bindings-input-transport"></a>
TS-IN-001 uses string for pasted text. TS-IN-003 forbids describing a string as an asset ID input.

<a id="ref-ports-and-bindings-uploaded-text"></a>
TS-IN-002 requires text_reference to enter a compatible port through initial_input, not prompt FieldBinding.

<a id="ref-ports-and-bindings-field-binding"></a>
FieldBinding maps one field to one allowed parameter. Single values use shared; multi-values use expanded.

It contains fieldKey, stepId, paramKey, and bindMode. Model routing only supports shared FieldBinding and requires allowModelOverride. Provider and mode are rejected.

<a id="ref-ports-and-bindings-param-binding"></a>
ParamBinding combines field_ref and literal sources. It allows at most one multi-value source.

Sources are ordered and joined by separator. Literals must be non-empty; field references must exist. A multi-value source requires expanded mode. Routing parameters do not support composition.

<a id="ref-ports-and-bindings-upstream-binding"></a>
UpstreamBinding uses step_output or initial_input. Source IDs, ports, dependency, MIME, multiplicity, and merge policy must be compatible.

For step_output, provide sourceStepId and sourcePort and include the step in dependsOn. For initial_input, provide sourceInputKey. Duplicate target-port bindings are accepted only when that port allows multiple.
