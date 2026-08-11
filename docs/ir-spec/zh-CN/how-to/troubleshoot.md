# 排查 TemplateSpec 问题

## 先判断失败层级

1. JSON 无法解析：修复语法和大小写。
2. Schema 失败：修复类型、required、enum 和简单条件。
3. local check 失败：修复 authoring guard 与明显引用错误。
4. create/version 失败：检查模型目录、拓扑、Binding、端口和服务版本。
5. workbook/input 失败：检查字段 key、必填值、枚举、MIME 和版本。
6. runtime 失败：检查 provider、资产读取、内容限制和 Step 错误。

## 高频问题

- 模型只看到 `ia_xxx`：字段错误声明为 string 或错误绑定到 prompt。
- unknown step/field：ID 或 key 拼写与声明不一致。
- duplicate target：多个 Binding 写同一 Step 参数。
- model unavailable：重新查询目标环境模型目录。
- old workbook incompatible：按当前 version 重新下载。
- downstream input incompatible：核对上游 MIME 与目标 port accepts。

错误索引见 [Validation Errors](../reference/validation-errors.md)，限制见 [Limits](../reference/limits.md)。
