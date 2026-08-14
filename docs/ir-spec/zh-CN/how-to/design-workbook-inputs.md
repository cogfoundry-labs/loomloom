# 设计 Workbook 输入

- 用业务含义命名 input key，不用裸模型参数名充当全局身份。
- 每个 Input 声明清楚 kind、类型、必填/空白策略和展示顺序。
- 同一个 Input 可以显式绑定到多个 Step 端口；不同语义或不同约束的输入使用不同 key。
- Artifact 明确 MIME 和数量；不要把临时 URL 当输入身份。
- 模型路由列必须可空，留空使用 Profile 默认模型。
