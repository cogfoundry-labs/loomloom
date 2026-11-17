# TemplateSpec v2 语法参考

公开字段使用 lowerCamel。新建或更新版本时，请求必须声明 `specVersion=template-spec/v2`，并把本对象放在 `canonicalSpecV2` 中。

## 顶层对象

| 字段 | 必需 | 说明 |
| --- | --- | --- |
| `meta` | 是 | 名称与展示元数据 |
| `templateInputs` | 否 | 以 input key 为键的模板输入 map |
| `steps` | 是 | 至少一个 Step 的 DAG |
| `workbook` | 是 | 填写说明与样例行 |
| `deliveryOutputs` | 否 | 显式声明最终交付的根步骤输出端口 |

## 身份规则

- Template Input key、目标 `portId`：`^[a-z][A-Za-z0-9_]{0,63}$`。
- `stepId`：`stp_` 加 6 到 10 位小写字母或数字。
- Step 输入身份是 `<stepId>.<portId>`。
- map 的 JSON 顺序不表达语义；Workbook 列顺序使用 `presentation.order`。

## Step

```json
{
  "stepId": "stp_image01",
  "displayName": "生成图片",
  "dependsOn": ["stp_text01"],
  "triggerPolicy": "require_all",
  "executionBinding": {
    "kind": "fixedModelContract",
    "subjectRevisionId": "subject-revision-id"
  },
  "inputBindings": {
    "prompt": {"source": "stepOutput", "stepId": "stp_text01", "portId": "output"}
  }
}
```

`executionBinding.kind`：

- `fixedModelContract`：必须给 `subjectRevisionId`。
- `capabilityProfile`：必须给稳定的 `profileId`，并声明 `modelSelection`。动态 Profile 不接受 `profileRevision`；普通创作必须省略该字段。创建前先使用 `loomloom capability resolve` 按业务输入输出选择当前匹配结果；需要查看全部 Profile 时再调用目标环境的 `GET /loom/v1/templateAuthoringContext`（CLI：`loomloom template-spec authoring-context --output json`）。
- `codeProfile`：必须给 `profileId`、`profileRevision` 和 `code`，不声明 `modelSelection`。
- `loop`：必须给 `loop`，不带普通输入绑定、Code 或模型选择。字段与首期范围见 [Step 参考](steps.md)。

Capability Profile 的 `definition` 是固定接口，`eligibleModels` 是根据当前模型能力
事实动态计算的集合。Profile 可以描述文本生成、图片理解、图片生成或视频生成等
标准能力；具体 ID、输入输出端口、当前默认模型和可选模型必须以目标环境返回为准。
例如图片理解需要将符合 MIME/数量约束的 Artifact Template Input 绑定到返回的
图片端口；图片或视频生成的 Artifact 输出也必须按返回的稳定输出端口连接。不得因
模型名称或 Provider 接口路径推断能力。

若 `capability resolve` 返回 Profile，且业务需要可替换模型集合，使用该 Profile；
若业务要求一个精确模型或其专属接口，使用返回的 `fixedModelContract`。不要因为某个
模型还存在固定合同，就忽略同一标准能力已经返回的动态 Profile。

## inputBindings

map key 是目标合同输入端口。source 支持：

- `templateInput`
- `stepOutput`
- `literal`
- `platformContext`
- `composeValue`
- `sequence`
- `merge`
- `loopState`：仅在 Loop body 内引用已声明的字符串状态。

一个目标端口只允许一个 binding。多来源不是重复声明端口，而是在一个 `merge` 或 `sequence` source 内显式列出。

## 未知字段

公开对象采用严格解码；拼错字段名会失败，不会静默忽略。精确结构见 [JSON Schema](../../machine/template-spec.schema.json)。


<a id="ref-delivery-outputs-declaration"></a>
## 最终交付声明

`deliveryOutputs` 是可选声明，使用 `schemaVersion: 1` 和有序的 `items`，每项包含稳定 `key`、用户可见 `label`、根步骤 `stepId` 与其 `portId`。示例见 [Code 最终交付](../examples/valid/delivery-code-outputs.json)。一个端口有多个文件时，声明覆盖该端口的完整文件集合；不得挑选其中一个文件。允许 Code 文件、Code 结构化值、模型输出以及 Loop 容器公开导出别名，不允许引用 Loop body 内部步骤、调试信息或输入附件。

`items` 必须包含 1 至 128 项；key 和来源端口不能重复，label 为 1 至 128 字符且不能含控制字符。保存时按确切合同验证端口并冻结逻辑端口与实际输出载体的映射。声明进入版本哈希和 Run 冻结定义；修改声明需保存新版本，已创建 Run 的下载不读取最新模板。

省略该字段保持历史语义和原哈希，表示“未定义最终交付”，不推测最后一步或第一份文件。历史 Run 仍可查看全部已产生的产物；结果范围明确报告未定义。`workbook` 只提供填写说明与样例，不能代替交付声明；原 resultWorkbook XLSX 接口保持原用途。

交付声明作为可选字段随版本保存正常支持，无需额外启用开关；填写后必须通过校验并冻结到对应版本，不得静默丢弃。未填写时保留历史模板语义，普通下载默认包含全部步骤的可见产物。
