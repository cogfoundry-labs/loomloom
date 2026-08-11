# Input Schema Reference

## inputSchema

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `fields` | array | 至少一个输入字段 |
| `instructions` | string[] | 工作簿或输入界面的整体填写说明 |
| `sampleRows` | array | 示例行；字段值放在 `values` 对象中并使用 field key |

## Input Field

| 字段 | 必需 | 规则 |
| --- | --- | --- |
| `key` | 是 | 非空、唯一；不能为 model/provider/mode |
| `label` | 是 | 非空、唯一 |
| `description` | 否 | 业务含义，不得与 valueType 矛盾 |
| `required` | 否 | 是否要求 user input 提供值 |
| `valueType` | 是 | string/enum/image_url/asset_ref/text_reference |
| `enumValues` | enum 时 | 非空选项数组 |
| `acceptedMimeTypes` | asset_ref/text_reference 时 | 非空 MIME pattern 数组 |
| `multiValue` | 否 | 是否允许多个值 |
| `maxValues` | multiValue=true 时 | 大于 0 |
| `order` | 否 | 工作簿展示顺序 |
| `defaultValue` | 条件必需 | sourceKind 非 user_input 时必须非空 |
| `hidden` | 否 | true 时不向使用者展示 |
| `sourceKind` | 否 | user_input/default_value/hidden |
| `presentation` | 否 | widget/placeholder/hint/examples |

## Presentation

widget 允许 `input`、`textarea`、`select`。placeholder、hint 为字符串，examples 为字符串数组。Presentation 只影响界面提示，不改变校验或执行。

## Sample rows

公开 JSON 形状：

```json
{"values": {"content": "示例正文", "tone": "专业"}}
```

values 中的 key 必须存在于 fields。不要使用 label，也不要把 field key 直接放在 sample row 顶层。
