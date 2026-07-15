# Compatibility Reference

## JSON 字段

公开 TemplateSpec 使用 lowerCamel。CLI 可以对部分旧 PascalCase 输入做归一化，但新文档和新示例不得依赖兼容转换。

## 版本快照

Template Version 是不可变执行快照。新字段、新 Binding 或新模型配置只进入新版本，不修改历史版本。

## Workbook

工作簿是版本派生产物。字段 key、顺序、枚举、提示或隐藏规则变化后，旧工作簿不保证兼容。

## CLI docs topic

CLI 可能保留 `spec`、`authoring`、`examples`、`conversation` 等历史 topic 名作为兼容入口；它们可以指向新手册页面，但不构成多份规范源。

## Legacy external APIs

新 CLI/Web-facing LoomLoom API 使用 `/loom/v1`。`/batch/v1` 是 legacy Core 路径，不应成为新 TemplateSpec 手册或客户端契约。
