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

Core 会根据没有下游依赖的终端 Steps 推导 primary output capability。单一 capability 返回 text/image/video；多个终端能力不同则为 mixed。手工填写的 `primaryOutputType` 若与推导值不同，创建校验失败。

元数据不会替代 InputSchema，也不会改变 artifact MIME。Market 展示信息和价格属于 Listing，不应写进 TemplateSpec meta 作为执行契约。
