# Template Input 与 Workbook

`templateInputs` 是以稳定 input key 为键的 map。输入分两类。

## value

```json
"prompt": {
  "kind": "value",
  "valueType": "string",
  "required": true,
  "blankPolicy": "error",
  "constraints": {"minLength": 1, "maxLength": 1000},
  "presentation": {"label": "提示词", "widget": "textarea", "order": 10}
}
```

`valueType` 支持 `string`、`number`、`integer`、`boolean`、`array`、`object`。`required=true` 必须配 `blankPolicy=error`；可选输入使用 `blankPolicy=omit`。

## artifact

```json
"referenceImages": {
  "kind": "artifact",
  "required": false,
  "blankPolicy": "omit",
  "acceptedMimeTypes": ["image/*"],
  "minItems": 0,
  "maxItems": 4,
  "presentation": {"label": "参考图", "order": 20}
}
```

Artifact 必须声明 MIME 和最大数量。运行值是平台稳定 Artifact 引用，不是任意 URL、文件名或 Base64。

## Workbook

Workbook 列由 `templateInputs` 生成。`workbook.instructions` 提供填写说明；`sampleRows[].values` 的 key 必须引用现有 Template Input。展示信息属于模板，模型合同中的展示信息只作为 authoring hint，不会反向覆盖作者选择。
