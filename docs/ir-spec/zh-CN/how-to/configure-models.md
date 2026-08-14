# 配置模型

固定模型使用：

```json
"executionBinding": {"kind": "fixedModelContract", "subjectRevisionId": "..."}
```

可替换模型使用 Capability Profile，并单独声明 `modelSelection`。不要把完整合同、provider 参数或模型路由值塞进 `executionBinding`。目标环境的 Subject/Profile ID 必须通过当前目录查询。
