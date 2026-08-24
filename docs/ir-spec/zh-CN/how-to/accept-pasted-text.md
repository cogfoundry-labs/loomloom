# 接收直接填写的文本

声明 string Template Input，再绑定到目标合同端口：

```json
{
  "templateInputs": {
    "prompt": {
      "kind": "value", "valueType": "string", "required": true, "blankPolicy": "error",
      "presentation": {"label": "提示词", "order": 10}
    }
  },
  "steps": [{
    "inputBindings": {"prompt": {"source": "templateInput", "inputKey": "prompt"}}
  }]
}
```

固定作者要求使用 literal 或 composeValue，不要求每一行重复填写。
