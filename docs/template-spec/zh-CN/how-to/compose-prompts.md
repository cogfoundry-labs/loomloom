# 组合多个文本字段

当一个 prompt 由多个用户字段和固定文字组成时，使用一条 ParamBinding。

## 示例

```json
{
  "stepId": "stp_write01",
  "paramKey": "prompt",
  "bindMode": "shared",
  "separator": "\n\n",
  "sources": [
    {"kind": "field_ref", "fieldKey": "content"},
    {"kind": "field_ref", "fieldKey": "style"},
    {"kind": "literal", "literal": "输出 Markdown，不要添加未提供的事实。"}
  ]
}
```

sources 按声明顺序合并。literal 必须是非空文本，field_ref 必须引用存在的字段。

## 多值字段

ParamBinding 最多包含一个 `multiValue=true` 的字段来源；存在多值来源时必须使用 `bindMode=expanded`。其他单值字段和 literal 会被共享到每次展开执行。

## 不适用场景

- 只有一个字段：FieldBinding 更直接。
- 上传文件：通过 UpstreamBinding 接入内容端口。
- 模型覆盖：model 只允许单独的 shared FieldBinding，不允许 ParamBinding。

## 校验

确认同一 `stepId + paramKey` 没有其他 FieldBinding 或 ParamBinding，再运行 `loomloom template-spec check`。

详见 [Bindings Reference](../reference/bindings.md#ref-ports-and-bindings-param-binding)。
