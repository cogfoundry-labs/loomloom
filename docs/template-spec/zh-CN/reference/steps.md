# Steps Reference

| 字段 | 必需 | 规则 |
| --- | --- | --- |
| `stepId` | 是 | `stp_` + 6～10 位 base36，小写数字字母；全局唯一 |
| `displayName` | 是 | 用户可读步骤名 |
| `executionUnit` | 是 | 当前 registry 中存在的 unit ID |
| `instruction` | 否 | 模板作者固定的处理要求 |
| `dependsOn` | 否 | 上游 step ID 数组；多个上游时必须有显式 upstreamBindings |
| `upstreamBindings` | 否 | 内容输入端口映射 |
| `triggerPolicy` | 否 | require_all/allow_partial/fail_fast；默认 require_all |
| `defaultModelRef` | 模型步骤通常需要 | `modelKey` 为当前目录 model ID |
| `allowModelOverride` | 否 | 是否允许 FieldBinding 覆盖 model |
| `staticParams` | 否 | 仅允许 execution unit 公布的运行参数 |

## 拓扑规则

- dependsOn 引用的 Step 必须存在。
- 依赖图不能包含环。
- step_output binding 的 sourceStepId 必须也在 dependsOn。
- 多个 upstream 已在当前公开服务装配中开启；每个 source step 必须显式映射到兼容 input port。

## Trigger policy

`allow_partial` 至少需要一个依赖；多上游时还要求显式 input bindings。`require_all`、`allow_partial` 和 `fail_fast` 的选择应与业务失败策略一致。

## 模型和静态参数

`defaultModelRef.modelKey` 非空。服务端还会检查模型存在并支持该 execution unit。staticParams 的 key 必须位于 unit 的 AllowedRunParameters。
