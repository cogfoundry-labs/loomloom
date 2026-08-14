# 快速开始

先复制 [多步骤固定模型示例](../examples/valid/multi-step-fixed-model.json)，再替换其中的 `subjectRevisionId`。该 ID 必须来自目标环境当前可用的认证合同，不能照抄示例占位值。

创建 TemplateVersion 的请求外壳为：

```json
{
  "versionNote": "first v2 version",
  "specVersion": "template-spec/v2",
  "canonicalSpecV2": {
    "meta": {"name": "我的模板"},
    "templateInputs": {},
    "steps": [],
    "workbook": {}
  }
}
```

本地检查：

```bash
loomloom template-spec check template.json
```

检查通过只证明结构和本地规则正确。正式创建时 Core 还会确认 Subject/Profile 存在、端口匹配、模型可用，并冻结合同。创建成功后再下载 Workbook、填写一行、validate、precheck 和提交 Run。
