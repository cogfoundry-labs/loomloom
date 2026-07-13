# 接收直接粘贴的文本

当使用者会在工作簿单元格或 JSON 输入中直接填写正文、主题、风格要求时，使用 `string`。

## 1. 声明字段

```json
{
  "key": "content",
  "label": "正文",
  "description": "需要改写的原始正文",
  "valueType": "string",
  "required": true,
  "order": 1,
  "presentation": {
    "widget": "textarea",
    "placeholder": "粘贴正文",
    "examples": ["LoomLoom 是一个可复用 AI 工作流平台。"]
  }
}
```

## 2. 绑定到运行参数

```json
{
  "fieldKey": "content",
  "stepId": "stp_write01",
  "paramKey": "prompt",
  "bindMode": "shared"
}
```

`shared` 要求字段为单值。完整模板见[单步文本生成](../examples/valid/single-text-generation.json)。

## 3. 校验

```bash
loomloom template-spec check ./template.json
```

check 通过后再进入 create/create-version 流程。

## 常见错误

- 在 description 中要求填写 `ia_xxx`：string 不解析资产 ID，违反 TS-IN-003。
- 把同一字段重复绑定到同一个 Step 参数：目标只能有一个 Binding。
- 把作者固定要求也做成用户字段：固定要求应写入 `instruction`。

上传长文本文件时改用[接收上传文本文件](accept-uploaded-files.md)。
