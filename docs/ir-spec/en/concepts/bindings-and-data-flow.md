# Bindings and data flow

v2 separates what an input is from where it comes from. Template Inputs describe public values; Step `inputBindings` connect those values to contract ports.

A binding has one source. Direct sources are templateInput, stepOutput, literal, and platformContext. Composite sources are composeValue, sequence, and merge. Scalars and Artifacts use the same model.

`dependsOn` controls scheduling; `inputBindings` controls data. A Step output reference requires both.
