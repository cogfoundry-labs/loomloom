# TemplateSpec v2

TemplateSpec v2 是 LoomLoom 新建和更新工作流版本使用的公开 JSON 格式。它把四类职责分开：

```text
Template Input ── inputBindings ──> Step 合同输入端口
上游 Step 输出 ── inputBindings ──> 下游 Step 合同输入端口
executionBinding ────────────────> 固定模型合同或能力 Profile
Template Input ──────────────────> Workbook / Public API 输入
```

## 最小规则

- 新写入只接受 `template-spec/v2`；v1 仅用于读取已有冻结版本和离线迁移。
- `templateInputs` 的 key 是模板级输入身份；Workbook 列由它生成。
- `steps[].inputBindings` 的 map key 是目标合同 `portId`，每个端口只能声明一个 binding。
- `executionBinding` 只保存权威合同引用，不保存运行输入。
- `stepOutput` 必须同时在 `dependsOn` 中声明来源 Step。
- 标量和 Artifact 使用同一套 source union；有序多模态值使用 `sequence`，同构 Artifact 集合使用 `merge`。
- Capability Profile 的模型选择使用独立 `modelSelection`，不会写入 Provider 原生请求。

## 开始使用

1. [快速开始](get-started/quickstart.md)
2. [理解 v2 对象关系](get-started/understand-template-spec.md)
3. [完整语法](reference/template-syntax.md)
4. [输入与 Workbook](reference/input-schema.md)
5. [Step 与执行绑定](reference/steps.md)
6. [输入绑定](reference/bindings.md)
7. [示例](examples/README.md)

机器校验以 [JSON Schema](../machine/template-spec.schema.json) 和 Core `ValidateTemplateSpecV2` 为准。模型合同、Profile 成员和输出端口是环境动态事实，创建版本时由 Core 解析并冻结。

当前文档 revision 见 [manifest.json](../manifest.json)。
