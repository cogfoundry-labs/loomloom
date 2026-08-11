# TemplateSpec

TemplateSpec 是 LoomLoom 定义可复用 AI 工作流的公开 JSON 格式。它把用户填写的字段、模型处理步骤、步骤间的数据流和可见结果保存在一个可版本化的契约中。

```text
输入字段 ── FieldBinding / ParamBinding ──> Step 参数
上传内容 ── initial_input binding ────────> Step 输入端口
上游结果 ── step_output binding ─────────> 下游 Step 输入端口
Step 执行 ───────────────────────────────> Artifact / 结果工作簿
```

## 开始使用

1. [快速开始](get-started/quickstart.md)：复制一个完整示例并通过本地检查。
2. [理解 TemplateSpec](get-started/understand-template-spec.md)：了解输入、步骤、Binding 和版本。
3. 根据任务选择 [How-to](#how-to)。
4. 编写时查询 [Reference](#reference)，不要猜字段或端口。

## Concepts

- [模板与版本](concepts/templates-and-versions.md)
- [输入字段](concepts/inputs.md)
- [步骤与 Execution Unit](concepts/steps-and-execution-units.md)
- [Binding 与数据流](concepts/bindings-and-data-flow.md)
- [执行流程](concepts/execution-flow.md)
- [工作簿与输入行](concepts/workbooks-and-rows.md)
- [Artifact 与结果](concepts/artifacts-and-results.md)
- [校验层级](concepts/validation-layers.md)

## How-to

- [接收直接粘贴文本](how-to/accept-pasted-text.md)
- [接收上传文本文件](how-to/accept-uploaded-files.md)
- [组合多个文本字段](how-to/compose-prompts.md)
- [构建多步骤工作流](how-to/build-multi-step-workflows.md)
- [配置默认模型](how-to/configure-models.md)
- [允许用户覆盖模型](how-to/expose-model-overrides.md)
- [设计工作簿输入](how-to/design-workbook-inputs.md)
- [校验、创建和追加版本](how-to/validate-create-and-version.md)
- [运行并读取结果](how-to/run-and-read-results.md)
- [排查模板问题](how-to/troubleshoot.md)

## Reference

- [完整语法](reference/template-syntax.md)
- [元数据](reference/metadata.md)
- [输入 Schema](reference/input-schema.md)
- [Step](reference/steps.md)
- [Bindings](reference/bindings.md)
- [Execution Units](reference/execution-units.md)
- [Validation Errors](reference/validation-errors.md)
- [Limits](reference/limits.md)
- [Compatibility](reference/compatibility.md)

## 规范与实现边界

- [JSON Schema](../machine/template-spec.schema.json) 定义可由通用 JSON 工具判断的结构。
- [规则目录](../machine/rules.json) 保存稳定 rule ID、enforcement 和正反例关系。
- Core validator、compiler、execution-unit registry 和 runtime 决定最终可执行语义。
- 模型目录和 provider 能力是环境动态事实，必须通过当前环境查询。
- 本目录只人工维护中文；CLI 英文手册是带 revision 的生成产物。

当前手册 revision 见 [manifest.json](../manifest.json)。
