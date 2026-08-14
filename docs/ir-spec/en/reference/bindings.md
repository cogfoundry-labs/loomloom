# Input bindings

Each `steps[].inputBindings` key is a target contract `portId`. One target port has one binding.

<a id="ref-ports-and-bindings-step-output"></a>

## Step output

A stepOutput source identifies `stepId` and stable output `portId`. The source Step must exist and appear in `dependsOn`. Do not use role, file name, or native JSON pointer as long-term port identity.

templateInput, literal, and platformContext are direct sources. composeValue performs deterministic string concat. sequence creates one ordered heterogeneous native value. merge combines homogeneous Artifact collections in source declaration order and then Artifact ordinal order.

Merge policy is `ordered_artifacts`; missing sources use `error` or `omit`. The merged collection still must satisfy the target contract cardinality.
