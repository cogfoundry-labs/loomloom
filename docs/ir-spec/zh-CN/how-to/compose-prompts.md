# 组合 Prompt

作者预设固定要求、用户只填变量时，用 `composeValue`：

```json
"prompt": {
  "source": "composeValue",
  "compose": {
    "kind": "concat",
    "separator": " ",
    "parts": [
      {"source": "literal", "literal": "以专业语气介绍产品："},
      {"source": "templateInput", "inputKey": "productName"}
    ]
  }
}
```

首版只支持字符串 concat，不执行表达式、脚本或递归组合。
