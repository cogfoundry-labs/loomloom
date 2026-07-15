# Execution flow

Core validates and compiles TemplateSpec into a WorkflowDefinition. Each row becomes a task; ready steps run by dependency. Runtime resolves inputs, validates required ports, invokes models, and persists artifacts.

```text
TemplateSpec -> shape validation -> compile WorkflowDefinition
-> parse each row -> resolve initial inputs -> schedule ready steps
-> merge parameters and artifacts -> invoke model -> persist artifacts
```

## Fixed parallel branches

A template may declare multiple root steps, or multiple steps may become ready after their dependencies complete. Runtime schedules all ready steps in the same DAG round concurrently. The branch count comes from the template topology, and each step still has the row's default single execution. No special parallel property is required.

## Shared and expanded

Shared values participate in the row's default execution. Expanded binding creates one execution for each multi-value item. A ParamBinding may contain at most one multi-value field, preventing an undefined Cartesian product.

Binding one shared input to ten root steps creates ten fixed parallel steps. Binding one multi-value input to one step with `expanded` creates multiple executions of that step.

## Trigger policy and fan-in

An empty policy becomes `require_all`. `allow_partial` and `fail_fast` are also supported. Multi-upstream steps require explicit bindings. Multiple sources may target one port only when it allows multiple; runtime applies concat-text or ordered-artifact merge policy from the registry.
