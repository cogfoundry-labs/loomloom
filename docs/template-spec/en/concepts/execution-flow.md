# Execution flow

Core validates and compiles TemplateSpec into a WorkflowDefinition. Each row becomes a task; ready steps run by dependency. Runtime resolves inputs, validates required ports, invokes models, and persists artifacts.

```text
TemplateSpec -> shape validation -> compile WorkflowDefinition
-> parse each row -> resolve initial inputs -> schedule ready steps
-> merge parameters and artifacts -> invoke model -> persist artifacts
```

## Fixed parallel branches

A template may declare multiple root steps, or multiple steps may become ready after their dependencies complete. Runtime schedules all ready steps in the same DAG round concurrently. The branch count comes from the template topology, and each step still has the row's default single execution. No special parallel property is required.

## Shared bindings and multi-content inputs

Shared values participate in the row's default execution. A multi-value field may enter an input port that accepts multiple contents through `sourceType=initial_input`; the Step still has one execution and receives the ordered content collection together.

Binding one shared input to ten root steps creates ten fixed parallel steps. Passing several references to one Step is one execution with several content items.

## Historical expanded compatibility

`bindMode=expanded` in a historical version expands one Step into multiple executions. The runtime continues to support already-saved versions, but new templates, new versions, and new publication flows reject the syntax with `TS-TOPOLOGY-001`. Use one workbook row per independently processed item. For fixed parallel processing, declare multiple Steps and connect them with `dependsOn` / `upstreamBindings`. TemplateSpec v1 does not support dynamic-cardinality Step fan-out.

## Trigger policy and fan-in

An empty policy becomes `require_all`. `allow_partial` and `fail_fast` are also supported. Multi-upstream steps require explicit bindings. Multiple sources may target one port only when it allows multiple; runtime applies concat-text or ordered-artifact merge policy from the registry.
