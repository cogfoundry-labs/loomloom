# Steps and execution units

A step selects an execution unit, fixed instruction, default model, optional static parameters, and dependencies. The registry defines allowed parameters, typed input ports, and output MIME. Multiple upstream dependencies are enabled in the public services, but require explicit bindings with compatible ports, multiplicity, MIME, and trigger policy.

## Three input channels

1. `instruction` and `staticParams` are fixed by the author.
2. FieldBinding and ParamBinding build row-specific run parameters.
3. UpstreamBinding supplies uploaded content or upstream artifacts to named ports.

Model-driven steps normally provide `defaultModelRef.modelKey`. Query the target environment instead of copying a sample model ID. Model override is available only when `allowModelOverride=true`; provider and mode are not exposed as template inputs.

`dependsOn` controls scheduling. Data does not flow merely because a dependency exists: use upstream bindings to identify source and target ports.
