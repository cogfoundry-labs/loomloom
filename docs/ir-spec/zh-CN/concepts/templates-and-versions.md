# 模板与版本

Template 是长期业务对象，TemplateVersion 是不可变执行快照。改变输入、Step、binding、合同引用、Profile 契约或 Workbook 展示都应创建新版本。

创建 v2 版本时，Core 同时冻结作者 Spec、Canonical IR、模型合同 Bundle、Profile 契约和 Definition Hash。Run 再冻结该次实际选中的模型与合同，用于复现、计费关联和排查。

Workbook 由具体版本的 `templateInputs` 派生。创建新版本后应重新下载 Workbook，不能假设旧列仍兼容。
