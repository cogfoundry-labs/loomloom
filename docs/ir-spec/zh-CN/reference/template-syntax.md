# TemplateSpec v2 语法参考

公开字段使用 lowerCamel。新建或更新版本时，请求必须声明 `specVersion=template-spec/v2`，并把本对象放在 `canonicalSpecV2` 中。

## 顶层对象

| 字段 | 必需 | 说明 |
| --- | --- | --- |
| `meta` | 是 | 名称与展示元数据 |
| `templateInputs` | 否 | 以 input key 为键的模板输入 map |
| `steps` | 是 | 至少一个 Step 的 DAG |
| `workbook` | 是 | 填写说明与样例行 |

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
- `capabilityProfile`：必须给稳定的 `profileId`，并声明 `modelSelection`。`profileRevision` 只用于明确请求某个历史合同；普通创作必须省略，由 Core 在保存时解析并冻结当前 revision。创建前先调用目标环境的 `GET /loom/v1/templateAuthoringContext`（CLI：`loomloom template-spec authoring-context --output json`），再选择 Profile 和模型。

## inputBindings

map key 是目标合同输入端口。source 支持：

- `templateInput`
- `stepOutput`
- `literal`
- `platformContext`
- `composeValue`
- `sequence`
- `merge`

一个目标端口只允许一个 binding。多来源不是重复声明端口，而是在一个 `merge` 或 `sequence` source 内显式列出。

## 未知字段

公开对象采用严格解码；拼错字段名会失败，不会静默忽略。精确结构见 [JSON Schema](../../machine/template-spec.schema.json)。
