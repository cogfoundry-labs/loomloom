# TemplateSpec Examples

本目录中的 JSON 由 Core 自动测试。

## Valid

- `single-text-generation.json`：string → FieldBinding(prompt) → text-generate。
- `uploaded-text-reference.json`：text_reference → initial_input(reference)。
- `multi-step-review.json`：单上游线性文本处理。
- `multi-upstream-summary.json`：两个并行文本步骤汇总到允许 multiple 的 prompt 端口。
- `parallel-text-to-image-branches.json`：同一个 shared 输入启动两个固定文本分支，每个分支继续生成一张图片。
- `workbook-fields-and-model-override.json`：enum、hidden default、presentation、ParamBinding 和模型覆盖。
- `text-image-video-chain.json`：文本、图片和视频 unit 的端口兼容链路。

Valid examples 同时通过 JSON Schema 和真实 `ValidateTemplateSpec`。示例 model ID 只用于结构测试；创建到目标环境前仍需查询真实模型目录。

## Invalid

- `uploaded-text-as-string.json`：说明要求资产 ID，但字段仍声明 string。
- `text-reference-without-input-binding.json`：text_reference 被错误写入 prompt。

Invalid examples 必须在 manifest 中声明 expected=invalid 和对应 rule ID。它们可能通过结构 Schema，但应被 authoring 或 Core 语义规则拒绝。

复制示例后，先阅读相应 How-to，再修改相互引用的 field key 和 step ID。
