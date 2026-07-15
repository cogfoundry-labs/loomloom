# 校验层级

TemplateSpec 会经过多层校验。某一层通过不代表后续所有层都必然成功。

## 1. JSON Schema

检查 JSON 树结构、类型、枚举、必填属性和简单条件。它不解析跨数组引用，也不知道当前环境有哪些模型。

## 2. 本地 CLI check

检查 CLI 已知的结构和 authoring guard，例如错误的 text_reference prompt Binding。它不创建远程资源，也不产生模型费用。

## 3. Core 创建校验

服务端检查字段与 Step 唯一性、ID pattern、拓扑、Binding 目标、sourceKind/default、multiValue/bindMode、端口 MIME、默认模型和当前环境目录。

## 4. Compiler 校验

TemplateSpec 被编译为 WorkflowDefinition，固定参数、模型 selector、input selector 和行级绑定被归一化，再经过定义校验。

## 5. Workbook / 输入校验

检查实际输入是否满足版本字段、必填值、枚举、MIME、行结构和允许参数。

## 6. Runtime

解析资产、读取上游 artifacts、合并端口输入并调用 provider。网络、模型和内容限制只能在这一层暴露。

排错时从最早失败层开始，不要用 runtime 重试掩盖结构错误。
