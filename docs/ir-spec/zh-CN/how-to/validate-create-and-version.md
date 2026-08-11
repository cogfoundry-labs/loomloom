# 校验、创建和追加版本

## 1. 本地检查

```bash
loomloom template-spec check ./template.json
```

修复结构、引用和 authoring guard，直到返回 valid。本步骤不创建远程资源。

## 2. 创建新模板

确认名称、输入字段、步骤、默认模型和本地结果后，再单独确认远程创建：

```bash
loomloom template-spec create ./template.json
```

保存返回的 template ID 和 version ID。

## 3. 修改已有模板

读取已有模板与版本，基于真实 spec 修改，然后追加版本：

```bash
loomloom template-spec create-version <template-id> ./template.json
```

历史版本不会被覆盖。新版本创建成功后重新下载工作簿。

## 4. 服务端失败

若本地通过但创建失败，优先检查动态模型目录、execution unit 能力、MIME/端口兼容和服务端当前版本，不要反复提交同一错误内容。
