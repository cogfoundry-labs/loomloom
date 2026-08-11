# 接收上传的文本文件

当使用者上传 `.txt` 等材料，并要求模型基于文件内容工作时，使用 `text_reference` 和 initial input binding。

## 1. 声明引用字段

```json
{
  "key": "source_text",
  "label": "原始材料",
  "description": "上传 UTF-8 文本文件",
  "valueType": "text_reference",
  "acceptedMimeTypes": ["text/plain"],
  "required": true,
  "order": 1
}
```

`acceptedMimeTypes` 是必填契约。只列出工作流实际能处理的 MIME。

## 2. 接入 Step 输入端口

```json
{
  "inputPort": "reference",
  "sourceType": "initial_input",
  "sourceInputKey": "source_text"
}
```

这段配置放在目标 Step 的 `upstreamBindings`。`text-generate.reference` 接受 `text/*`，因此与 `text/plain` 兼容。

## 3. 设置处理要求

把“只根据材料提取事实”等要求写入 `steps[].instruction`，不要把资产 ID 拼进 prompt。

## 4. 校验与运行输入

先执行 `template-spec check`。运行时先上传文件，拿到 input asset ID，再把它填入 `source_text` 对应的行字段。asset ID 不是 template run 使用的 input file ID，两者不能互换。

完整模板见[上传文本示例](../examples/valid/uploaded-text-reference.json)。

## 常见错误

- `string + prompt + ia_xxx`：模型只看到 ID 字符串。
- `text_reference` 又通过 FieldBinding 写入 prompt：Core 以 TS-IN-002 拒绝。
- MIME 不匹配端口：创建或运行校验失败。

精确规则见 [Bindings Reference](../reference/bindings.md#ref-ports-and-bindings-uploaded-text)。
