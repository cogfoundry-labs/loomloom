# 设计工作簿输入

工作簿列直接来自 inputSchema。设计字段时同时考虑机器契约和填写体验。

## 推荐顺序

1. 明确一行代表的业务对象。
2. 只暴露每行真正变化的内容。
3. 固定处理要求放到 Step instruction。
4. 使用 order 排列字段，required 标明必填。
5. enum 使用 select；长文本使用 textarea。
6. 为复杂字段提供 hint 和 examples。
7. 隐藏默认字段使用 sourceKind=hidden/default_value，并提供 defaultValue。

## 示例

```json
{
  "key": "tone",
  "label": "语气",
  "valueType": "enum",
  "enumValues": ["专业", "轻松", "简洁"],
  "required": true,
  "order": 2,
  "presentation": {
    "widget": "select",
    "hint": "选择输出语气"
  }
}
```

创建版本后下载工作簿，实际检查表头、提示、下拉和示例。版本变化后重新下载。
