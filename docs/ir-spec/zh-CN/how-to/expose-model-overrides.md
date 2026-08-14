# 允许用户选择模型

v2 使用 Capability Profile 替代旧的任意模型覆盖。

1. executionBinding 引用固定 Profile 契约版本。
2. 创建一个可空 string Template Input 作为模型列。
3. `modelSelection` 引用该 input key，并给出默认模型。
4. Run 时按 Profile 当前合格成员校验选择，并冻结实际模型和合同证据。

模型列不进入 Provider 原生 JSON。固定模型合同不能声明 modelSelection。
