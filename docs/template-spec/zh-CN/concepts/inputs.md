# 输入字段

`inputSchema.fields` 定义一行运行输入能提供什么。字段 key 是机器契约，label、description 和 presentation 帮助人填写。

## 选择 valueType

| 类型 | 适用内容 | 关键约束 |
| --- | --- | --- |
| `string` | 直接填写的正文、主题、要求 | 值按普通文本处理，不解析 `ia_*` |
| `enum` | 固定选项 | 必须提供非空 `enumValues` |
| `image_url` | HTTP/HTTPS 图片地址 | 不是上传资产 ID |
| `asset_ref` | 上传文件引用 | 必须声明 `acceptedMimeTypes` |
| `text_reference` | 内联文本或上传文本引用 | 必须声明 MIME，并通过 initial input 接入文本端口 |

## 输入来源

`sourceKind` 省略时等同 `user_input`。`default_value` 和 `hidden` 不要求使用者填写，但必须提供非空 `defaultValue`。`hidden=true` 也会使字段成为隐藏输入。

## 单值与多值

`multiValue=true` 表示一行可以提供多个值，并要求 `maxValues > 0`。多值字段要通过 `expanded` binding 产生 fan-out；普通单值字段使用 `shared`。不要用逗号文本模拟多值，除非业务内容本身就是一段字符串。

## 展示信息

`presentation.widget` 可为 `input`、`textarea`、`select`；还可提供 placeholder、hint 和 examples。展示信息不会改变执行语义，真实约束仍来自 valueType、枚举、MIME 和 Binding。

完整字段见[输入 Schema Reference](../reference/input-schema.md)。
