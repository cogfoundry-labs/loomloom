# Workbook 与行

服务按 `templateInputs` 生成 Workbook 列。input key 是机器映射，`presentation.label` 是表头，`presentation.order` 控制顺序。

一行表示一个 Task 的模板输入集合。可选输入留空时按 `blankPolicy=omit` 省略；必填输入留空会在 Run 前失败。Artifact 单元格保存稳定资产引用，不保存临时下载 URL。

Workbook 是 v2 的一种输入载体，Public API 也可以提交相同的 Template Input 值；两者编译到同一 Canonical IR。
