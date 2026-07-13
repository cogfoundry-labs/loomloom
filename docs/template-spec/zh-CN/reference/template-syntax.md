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

分项参考：[Metadata](metadata.md)、[Input Schema](input-schema.md)、[Steps](steps.md)、[Bindings](bindings.md)。
