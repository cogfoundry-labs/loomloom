# 执行流程

1. 创建 TemplateVersion 时校验 v2 结构和 DAG。
2. Core 解析 fixedModelContract 或 Capability Profile，并校验每个 `<stepId>.<portId>`。
3. Core 冻结作者 Spec、合同 Bundle、Profile 契约与定义 Hash。
4. Workbook 行解析为 Template Input 值。
5. 调度器按 `dependsOn` 和 `triggerPolicy` 运行 Step。
6. binding 编译器生成每个 Step 的原生模型输入；Run 记录实际选中模型、合同与 Artifact。

`allow_partial` 允许部分上游失败后继续，但 merge 的 `missingSourcePolicy` 仍单独决定某个目标端口如何处理缺失来源。控制流和数据合并不能互相替代。
