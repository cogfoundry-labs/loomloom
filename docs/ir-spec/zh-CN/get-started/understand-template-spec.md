# 理解 TemplateSpec

一份 TemplateSpec 不是一次运行输入，而是“以后每一行输入怎样被处理”的版本化定义。

## 四个核心部分

```json
{
  "meta": {},
  "steps": [],
  "inputSchema": {},
  "fieldBindings": [],
  "paramBindings": []
}
```

- `meta` 描述模板名称、场景和展示输出类型。
- `steps` 定义执行步骤、固定 instruction、默认模型和步骤依赖。
- `inputSchema` 定义使用者看到并填写的字段。
- `fieldBindings` 与 `paramBindings` 把字段值转换为 Step 运行参数。
- `steps[].upstreamBindings` 把上传内容或上游 artifact 接入 Step 的输入端口。

## 定义与运行输入分离

TemplateSpec 决定字段和工作流结构。真正运行时，每一行只提供 `inputSchema.fields[].key` 对应的值。模板作者固定的处理要求应写入 `steps[].instruction` 或 `staticParams`，不应要求每个使用者重复填写。

## 版本是不变快照

创建模板后，后续修改通过追加版本完成。已有版本、由其生成的工作簿以及基于它创建的 Market Listing Version 不会被新版本静默改写。因此修改字段 key、类型或步骤 ID 后，应重新下载工作簿并使用新 version ID。

## 校验不等于执行

本地 `template-spec check` 只检查客户端已知的结构和 authoring 规则；服务端创建还会检查真实模型目录、execution unit 和工作流形状；运行时还会解析上传资产、合并输入并调用模型。详见[校验层级](../concepts/validation-layers.md)。

下一步：阅读[输入字段](../concepts/inputs.md)和[Binding 与数据流](../concepts/bindings-and-data-flow.md)。
