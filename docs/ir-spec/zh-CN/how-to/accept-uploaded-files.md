# 接收上传文件

1. 声明 artifact Template Input，给出 `acceptedMimeTypes`、`minItems` 和 `maxItems`。
2. 在目标 Step 的 `inputBindings[portId]` 中用 `source=templateInput` 引用它。
3. 确认 MIME 和数量是模型合同端口允许范围的安全子集。

单个 Template Input 已经可以是集合；只有需要把多个 Template Input 或上游输出合并为一个集合时才使用 `merge`。
