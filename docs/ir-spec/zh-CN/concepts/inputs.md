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

`multiValue=true` 表示一行可以提供多个内容，并要求 `maxValues > 0`。新模板中，多值字段只用于通过 `sourceType=initial_input` 向支持多内容的 input port 提供有序内容集合；它不会增加 Step 的 execution 数量。普通参数字段使用 `shared`。不要用逗号文本模拟多值，除非业务内容本身就是一段字符串。

历史版本可能包含 `bindMode=expanded`，将多值参数展开为多次 execution。该语法仅为历史运行兼容保留，不能用于新建模板或新版本。

## 展示信息

`presentation.widget` 可为 `input`、`textarea`、`select`；还可提供 placeholder、hint 和 examples。展示信息不会改变执行语义，真实约束仍来自 valueType、枚举、MIME 和 Binding。

完整字段见[输入 Schema Reference](../reference/input-schema.md)。
