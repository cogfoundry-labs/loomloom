# Binding 与数据流

Binding 回答“这个值如何进入这个 Step”。选择错误的 Binding，JSON 可能看似完整，但模型看到的上下文会完全不同。

## 参数 Binding

- FieldBinding：一个字段直接写入一个运行参数。
- ParamBinding：多个字段和固定 literal 按 separator 组合成一个参数。

两者的目标都是 `stepId + paramKey`。同一个目标只能有一个来源，不能让两条 Binding 互相覆盖。

## 输入端口 Binding

UpstreamBinding 连接内容型输入：

- `sourceType=initial_input`：从本行上传资产或引用字段进入端口。
- `sourceType=step_output`（或省略 sourceType）：从依赖 Step 的 output port 进入端口。

端口有 MIME、required、allowMultiple 和 merge policy。类型兼容由 execution-unit registry 校验，而不是由字段名称推断。

## 参数与内容的区别

prompt 参数是普通文本；上传资产是需要解析的内容引用。把 `ia_xxx` 填进 string 并绑定到 prompt，只会把 ID 字符串交给模型。正确做法是 `text_reference -> initial_input -> reference/prompt compatible port`。

详见 [Bindings Reference](../reference/bindings.md)。
