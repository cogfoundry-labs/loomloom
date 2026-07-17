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

### shared 与多内容输入

`shared` 表示字段值参与该行默认的一次执行。`multiValue` 可以通过 `sourceType=initial_input` 向允许多个内容的 input port 提供有序集合，但仍然只有一次 Step execution。

因此，同一个 `shared` 输入分别绑定到十个根 Step，表示十个固定并行 Step；多份参考材料进入一个 Step 则表示该 Step 在一次 execution 中同时接收多份内容。

### 历史 expanded 兼容

历史版本中的 `bindMode=expanded` 会把一个 Step 展开为多次 execution。runtime 继续支持这些已保存版本，但新模板、新版本和新的发布流程会以 `TS-TOPOLOGY-001` 拒绝该语法。需要动态处理多个独立对象时，每个对象使用一个 workbook row；需要固定并行时，显式声明多个 Step，并用 `dependsOn` / `upstreamBindings` 连接。TemplateSpec v1 不支持动态基数的 Step fan-out。

### Fan-in 与 Trigger policy

空值会归一为 `require_all`。`allow_partial` 允许在部分上游可用时进入下游，`fail_fast` 用于上游失败时尽早停止。多上游 Step 必须显式声明 upstreamBindings；若多个来源写入同一端口，该端口必须 `allowMultiple=true`，runtime 按 registry 的 merge policy 合并。

## 必填端口

runtime 在执行前检查 required input port 是否获得内容。某些 unit 的 prompt 既可来自运行参数，也可由输入 artifact 合并提供；最终行为以 registry 和 runtime 的 required-bound-input 检查为准。
