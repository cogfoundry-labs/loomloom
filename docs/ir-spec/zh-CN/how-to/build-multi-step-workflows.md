# 构建多步骤工作流

下游使用上游结果时同时声明：

```json
{
  "dependsOn": ["stp_source1"],
  "inputBindings": {
    "image": {"source": "stepOutput", "stepId": "stp_source1", "portId": "output"}
  }
}
```

多个独立分支直接声明多个 Step。多个 Artifact 来源汇到一个 collection 端口时使用 `merge`；一个原生多模态数组需要位置和类型时使用 `sequence`。不要依赖字段同名自动连线。
