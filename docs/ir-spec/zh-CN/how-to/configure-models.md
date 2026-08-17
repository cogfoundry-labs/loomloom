# 配置模型

固定模型使用：

```json
"executionBinding": {"kind": "fixedModelContract", "subjectRevisionId": "..."}
```

可替换模型使用 Capability Profile，并单独声明 `modelSelection`。不要把完整合同、provider 参数或模型路由值塞进 `executionBinding`。先查询目标环境的创作上下文：

```bash
loomloom template-spec authoring-context --output json
```

它返回当前 Profile、实时 revision、端口和可选模型。普通模板只写返回的 `profileId`，不写 `profileRevision`；Core 会在保存版本时冻结当时的 revision 与 hash。
