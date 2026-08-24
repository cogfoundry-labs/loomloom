# Validation Errors

| Rule ID | 拒绝条件 | 修复 |
| --- | --- | --- |
| TS-VERSION-002 | 用 v1 创建新版本 | 离线迁移为 v2 后创建新的 TemplateVersion |
| TS-BINDING-002 | stepOutput 未声明对应 dependsOn | 同时声明调度依赖和数据 binding |
| TS-PROFILE-002 | Profile Step 缺少合法 modelSelection | 引用可空 string Template Input，并设置默认模型 |

JSON Schema 负责对象形状；Core validator 负责跨字段、DAG 和 source 语义；保存门禁再验证当前 Subject/Profile、端口和值域。结构通过不等于目标环境可执行。
