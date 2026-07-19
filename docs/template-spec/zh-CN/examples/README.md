# TemplateSpec Examples

本目录中的 JSON 由 Core 自动测试。

通过 CLI 查看本索引：

```bash
loomloom template-spec docs examples
```

选择一个最接近目标能力的 valid 示例作为本地草稿。示例随 CLI 分发；无需先查看 Core 源码。复制后保留相互引用的 field key、step ID、`dependsOn` 和 `upstreamBindings`，再逐步修改业务文案、字段和模型。

## Valid

| 目标能力 | 从这个示例开始 | 包含的关键能力 |
| --- | --- | --- |
| 单个文本输入生成内容 | `single-text-generation.json` | string → FieldBinding(prompt) → text-generate |
| 上传文本作为模型上下文 | `uploaded-text-reference.json` | text_reference → initial_input(reference) |
| 线性处理上一步输出 | `multi-step-review.json` | 单上游 `dependsOn` 和 step_output |
| 汇总多个上游文本 | `multi-upstream-summary.json` | 多上游、允许 multiple 的 prompt 端口 |
| 预先固定数量的并行分支 | `parallel-text-to-image-branches.json` | shared 输入、多个 root Step、一对一 text → image 分支 |
| 工作簿字段和模型选择 | `workbook-fields-and-model-override.json` | enum、hidden default、presentation、ParamBinding、模型覆盖 |
| 组合文本、图片和视频 | `text-image-video-chain.json` | execution unit 端口兼容链路 |

固定并行分支示例不是 `expanded`：分支数量由 Spec 中重复声明的 Step 决定，JSON 中没有 `branch` 或 `parallel` 字段。

Valid examples 同时通过 JSON Schema 和真实 `ValidateTemplateSpec`。示例 model ID 只用于结构测试；创建到目标环境前仍需查询真实模型目录。

## Invalid

- `uploaded-text-as-string.json`：说明要求资产 ID，但字段仍声明 string。
- `text-reference-without-input-binding.json`：text_reference 被错误写入 prompt。

Invalid examples 必须在 manifest 中声明 expected=invalid 和对应 rule ID。它们可能通过结构 Schema，但应被 authoring 或 Core 语义规则拒绝。

复制示例后，先阅读相应 How-to，再修改相互引用的 field key 和 step ID。最后执行：

```bash
loomloom template-spec check ./template.json
```
