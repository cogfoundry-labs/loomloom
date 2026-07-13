# Execution flow

Core validates and compiles TemplateSpec into a WorkflowDefinition. Each row becomes a task; ready steps run by dependency. Shared binding creates the default execution, while expanded binding fans out a multi-value field. Multi-upstream steps use explicit bindings; repeated ports require allowMultiple and use the registry merge policy. Runtime resolves inputs, validates required ports, invokes models, and persists artifacts.

```text
TemplateSpec -> shape validation -> compile WorkflowDefinition
-> parse each row -> resolve initial inputs -> schedule ready steps
-> merge parameters and artifacts -> invoke model -> persist artifacts
```

## Shared and expanded

Shared values participate in the row's default execution. Expanded binding creates one execution for each multi-value item. A ParamBinding may contain at most one multi-value field, preventing an undefined Cartesian product.

## Trigger policy and fan-in

An empty policy becomes `require_all`. `allow_partial` and `fail_fast` are also supported. Multi-upstream steps require explicit bindings. Multiple sources may target one port only when it allows multiple; runtime applies concat-text or ordered-artifact merge policy from the registry.
