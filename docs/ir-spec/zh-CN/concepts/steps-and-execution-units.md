# Step 与 Execution Unit

Step 是工作流中的一个处理节点。`executionUnit` 决定它可接收的参数、输入端口和输出类型；模型决定该能力由哪个具体模型执行。

## Step 的三类输入

1. `instruction` 和 `staticParams`：模板作者固定的行为。
2. FieldBinding / ParamBinding：每一行提供的运行参数。
3. UpstreamBinding：上传内容或上游 artifact 进入具名输入端口。

## 默认模型

模型驱动的 Step 通常设置 `defaultModelRef.modelKey`。该值必须来自当前环境真实模型目录，并且模型支持对应 execution unit。不要从文档示例猜测当前可用 model ID。

## 模型覆盖

只有 `allowModelOverride=true` 时，输入字段才能绑定到该 Step 的 `model` 参数。`provider` 和 `mode` 不向模板输入开放。普通模板应优先固定默认模型，只有确实需要使用者选择模型时才开放覆盖。

## 依赖与数据流

`dependsOn` 声明调度依赖，但不会自动说明 output 进入哪个 port。多步骤模板应使用 `upstreamBindings` 明确数据来源。一个 Step 可以依赖多个上游；此时必须声明显式 bindings，并确保目标端口允许对应的来源数量和 MIME。

详见 [Steps Reference](../reference/steps.md) 和 [Execution Units](../reference/execution-units.md)。
