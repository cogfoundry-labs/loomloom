# 执行流程

TemplateSpec 在创建时被校验并编译为 WorkflowDefinition。运行时，每一行输入成为一个 task，task 内的 Step 按依赖图执行。

```text
TemplateSpec
  -> validate workflow shape
  -> compile steps, model selectors, params and input selectors
  -> validate compiled definition
  -> parse row / resolve initial inputs
  -> schedule ready steps
  -> merge parameters and input artifacts
  -> execute model
  -> persist artifacts and step status
```

## 三种并行与汇聚语义

### 固定并行分支

模板可以声明多个根 Step，或让多个 Step 在依赖满足后同时就绪。runtime 会把同一 DAG 轮次的 ready Step 并发调度。分支数量来自模板拓扑，每个 Step 仍只有该行默认的一次 execution；这种结构不需要特殊并行字段。

### shared 与 expanded

`shared` 表示字段值参与该行默认的一次执行。`expanded` 与 multiValue 字段配合，为每个值形成独立执行。ParamBinding 最多只能包含一个 multiValue 字段来源，避免无法定义的笛卡尔积。

因此，同一个 `shared` 输入分别绑定到十个根 Step，表示十个固定并行 Step；一个 multiValue 输入以 `expanded` 绑定到一个 Step，才表示同一 Step 展开为多次 execution。

### Fan-in 与 Trigger policy

空值会归一为 `require_all`。`allow_partial` 允许在部分上游可用时进入下游，`fail_fast` 用于上游失败时尽早停止。多上游 Step 必须显式声明 upstreamBindings；若多个来源写入同一端口，该端口必须 `allowMultiple=true`，runtime 按 registry 的 merge policy 合并。

## 必填端口

runtime 在执行前检查 required input port 是否获得内容。某些 unit 的 prompt 既可来自运行参数，也可由输入 artifact 合并提供；最终行为以 registry 和 runtime 的 required-bound-input 检查为准。
