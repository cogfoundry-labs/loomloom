# 理解 TemplateSpec v2

一个 v2 Spec 只有四个顶层部分：

```json
{
  "meta": {"name": "示例"},
  "templateInputs": {},
  "steps": [],
  "workbook": {}
}
```

- `meta` 描述模板。
- `templateInputs` 定义用户或 API 可以提供的值与 Artifact，也是 Workbook 列来源。
- `steps` 定义 DAG、执行合同、模型路由和输入来源。
- `workbook` 只保存填写说明与样例行；列定义不再重复保存。

每个 Step 输入都用 `<stepId>.<portId>` 唯一定位。两个模型即使都叫 `prompt`，也分别属于自己的 Step，不会按裸字段名隐式合并。需要共享时，让两个 binding 显式引用同一个 Template Input。

TemplateSpec 是作者协议；创建 TemplateVersion 时，Core 会解析权威合同、校验端口和值域，并冻结可执行快照。运行时读取冻结版本，而不是重新猜测当前目录。
