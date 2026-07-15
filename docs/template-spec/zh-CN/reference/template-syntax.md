# TemplateSpec 语法参考

公开 JSON 使用 lowerCamel 字段名，当前版本为 `template-spec/v1`。精确结构由 [JSON Schema](../../machine/template-spec.schema.json) 校验，本页解释字段关系和执行语义。

## 顶层对象

| 字段 | 必需 | 类型 | 说明 |
| --- | --- | --- | --- |
| `meta` | 是 | object | 名称、场景和展示元数据 |
| `steps` | 是 | array | 至少一个处理步骤 |
| `inputSchema` | 是 | object | 行级输入字段、说明和示例 |
| `fieldBindings` | 否 | array | 单字段到 Step 参数 |
| `paramBindings` | 否 | array | 多来源组合到 Step 参数 |

完整 Spec 至少要让每个 required Step 输入能够由固定参数、字段 Binding、initial input 或上游输出满足。

## 命名与引用

- field key、step ID 和引用值大小写敏感。
- step ID 必须匹配 `stp_<6-10 base36 chars>`。
- field key 不能使用保留字 `model`、`provider`、`mode`。
- field label 也必须唯一，避免工作簿列歧义。

## 默认值

- `sourceKind` 无有效值时按 `user_input` 处理。
- `triggerPolicy` 为空时按 `require_all`。
- UpstreamBinding `sourceType` 为空时按 `step_output`。
- 未声明 bindings 数组等同空数组。

## 未知字段

Schema 对公开对象使用 `additionalProperties=false`。拼错字段名不会被静默忽略，应在结构校验阶段失败。

## 固定并行分支语法

固定分支由 `steps` 和 bindings 的拓扑表达，不存在额外的 `branch` 或 `parallel` 字段。下面省略了展示字段和模型配置，只保留“一份输入启动两个文本 Step，每个文本 Step 连接一个图片 Step”的关键结构：

```json
{
  "steps": [
    {"stepId": "stp_scenea", "executionUnit": "text-generate"},
    {"stepId": "stp_sceneb", "executionUnit": "text-generate"},
    {
      "stepId": "stp_imagea",
      "executionUnit": "image-generate",
      "dependsOn": ["stp_scenea"],
      "upstreamBindings": [
        {"inputPort": "prompt", "sourceType": "step_output", "sourceStepId": "stp_scenea", "sourcePort": "output"}
      ]
    },
    {
      "stepId": "stp_imageb",
      "executionUnit": "image-generate",
      "dependsOn": ["stp_sceneb"],
      "upstreamBindings": [
        {"inputPort": "prompt", "sourceType": "step_output", "sourceStepId": "stp_sceneb", "sourcePort": "output"}
      ]
    }
  ],
  "fieldBindings": [
    {"fieldKey": "book_title", "stepId": "stp_scenea", "paramKey": "prompt", "bindMode": "shared"},
    {"fieldKey": "book_title", "stepId": "stp_sceneb", "paramKey": "prompt", "bindMode": "shared"}
  ]
}
```

两个文本 Step 都没有 `dependsOn`，所以会在输入就绪后进入同一轮调度。两个图片 Step 各自等待对应文本 Step。需要十条固定分支时，重复相同的 Step 和 binding 结构；不要把它改成 `expanded`。可直接校验的完整 Spec 见[固定并行配图示例](../examples/valid/parallel-text-to-image-branches.json)。

分项参考：[Metadata](metadata.md)、[Input Schema](input-schema.md)、[Steps](steps.md)、[Bindings](bindings.md)。
