# Step

| 字段 | 必需 | 说明 |
| --- | --- | --- |
| `stepId` | 是 | 稳定 Step 身份 |
| `displayName` | 是 | 展示名称 |
| `dependsOn` | 否 | 调度依赖，必须无环 |
| `triggerPolicy` | 否 | `require_all`、`allow_partial` 或 `fail_fast` |
| `executionBinding` | 是 | 固定合同或能力 Profile 的权威引用 |
| `modelSelection` | Profile Step 必需 | 独立的模型路由声明 |
| `inputBindings` | 按合同需要 | 目标端口到输入来源的映射 |

`dependsOn` 只说明调度关系，不自动传数据；真正的数据来源必须写在 `inputBindings`。反过来，引用 `stepOutput` 时也必须把来源 Step 放进 `dependsOn`。

<a id="ref-profiles-model-selection"></a>

## TS-PROFILE-002：Profile 模型选择

固定模型 Step 不允许 `modelSelection`。Profile Step 的模型选择输入必须是可空字符串 Template Input，留空使用 `defaultModelId`，填写值由运行时按当前 Profile 成员校验。
