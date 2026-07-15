# Bindings and data flow

FieldBinding maps one field to one parameter. ParamBinding combines field references and literals. UpstreamBinding connects initial uploaded content or an upstream artifact to a typed port. One target parameter has one source. Asset IDs are content references, never prompt strings.

## Parameter bindings

FieldBinding is the direct choice for one field. ParamBinding joins several fields and fixed literals in source order using a separator. Both target `stepId + paramKey`; duplicate writers are rejected.

## Content bindings

Use `initial_input` for uploaded/reference fields and `step_output` for artifacts produced by a dependency. Port names, accepted MIME, multiplicity, and merge behavior come from the execution-unit registry.

## Prompt text versus referenced content

A prompt parameter is ordinary text. An `ia_*` value only becomes referenced content when a reference field is resolved through initial input. Putting that ID into a string prompt sends the literal identifier to the model.
