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

在准备创建版本的目标 Server 上检查：

```bash
loomloom template-spec check template.json
```

`check` 使用与创建版本相同的服务端合同解析规则，确认 Subject/Profile、端口和模型当前可用；它不会写入版本。正式创建时服务端会再次校验并冻结合同，避免检查后权威数据变化。创建成功后再下载 Workbook、填写一行、validate、precheck 和提交 Run。
