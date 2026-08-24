# TemplateSpec v2 示例

| 场景 | 文件 | 覆盖能力 |
| --- | --- | --- |
| 多步骤模型链 | `multi-step-fixed-model.json` | Step 级 prompt、stepOutput、dependsOn |
| 多 Artifact 合并 | `artifact-merge.json` | 并行上游、ordered merge |
| 作者预设 + 用户变量 | `compose-value.json` | composeValue concat |
| 多模态有序输入 | `content-sequence.json` | text/image sequence 与 role |
| 上游图片进入有序输入 | `content-sequence-step-output.json` | literal、上游 Artifact、sequence、dependsOn |
| 可替换文本模型 | `capability-profile.json` | Profile、动态模型选择、默认模型 |

示例中的 Subject revision 和 model ID 是结构占位或某次测试环境证据；创建版本前必须替换为目标环境当前返回的权威 ID。Capability Profile 的普通写法只保留稳定 `profileId`，不要把某次查询得到的 `profileRevision` 写进模板。

invalid 目录保存应被 Schema 或 Core validator 拒绝的示例，用于稳定错误边界。

所有示例只包含 `canonicalSpecV2` 对象本身。提交创建版本请求时，还要在外层增加 `specVersion=template-spec/v2` 和 `canonicalSpecV2` 字段。
