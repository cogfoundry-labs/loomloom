# Metadata Reference

`meta` 描述模板，不直接控制 Step 执行。

| 字段 | 必需 | 说明 |
| --- | --- | --- |
| `name` | 是 | 非空模板名称 |
| `description` | 否 | 模板用途与主要结果 |
| `scenario` | 否 | 业务场景分类 |
| `inputSummary` | 否 | 对输入内容的简要说明 |
| `displayOutputType` | 否 | 面向界面的输出展示提示 |
| `primaryOutputType` | 否 | 主要输出能力；若填写必须与终端 Step 派生结果一致 |
| `tags` | 否 | 字符串标签数组 |

Core 根据冻结合同的终端输出推导主要输出能力；手工填写的 `primaryOutputType` 若与保存门禁推导结果不同，创建校验失败。

元数据不会替代 `templateInputs` 或合同端口，也不会改变 Artifact MIME。Market 展示信息和价格属于 Listing，不应写进 TemplateSpec meta 作为执行契约。
