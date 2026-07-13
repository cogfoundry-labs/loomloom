# TemplateSpec 快速开始

本教程从完整示例创建本地草稿并通过 check。不会创建远程模板或产生模型费用。

## 前提

- 已安装当前 LoomLoom CLI。
- CLI 能运行 `loomloom template-spec docs` 和 `check`。
- 准备一个空目录保存 `template.json`。

## 1. 复制最小模板

复制[单步文本生成示例](../examples/valid/single-text-generation.json)。它包含：

- 一个 `string` 输入字段 `content`。
- 一个 `text-generate` Step。
- 一条把 content 写入 prompt 的 FieldBinding。

## 2. 查询并选择模型

```bash
loomloom template-spec models text-generate
```

把示例中的 `defaultModelRef.modelKey` 替换为目标环境实际返回的 model ID。模型目录是动态事实，示例值不保证在所有环境可用。

## 3. 修改业务定义

先只修改：

- `meta.name` 和 `meta.description`。
- `steps[0].displayName` 与 `instruction`。
- 输入字段的 label、description 和 presentation。

第一次不要修改 stepId、field key、valueType 或 bindings；这些值互相引用。

## 4. 本地检查

```bash
loomloom template-spec check ./template.json
```

输出 valid 说明本地已知结构和 authoring 规则通过。它不验证目标环境所有动态能力。

## 5. 理解结果

运行时一行输入提供 `content`，Binding 把它写入 Step prompt；instruction 作为模板作者固定要求一起生效；Step 输出 `text/plain` artifact。

## 下一步

- [理解 TemplateSpec](understand-template-spec.md)
- [接收直接文本](../how-to/accept-pasted-text.md)
- [接收上传文件](../how-to/accept-uploaded-files.md)
- [校验与创建版本](../how-to/validate-create-and-version.md)

远程 create/create-version 是独立写操作，执行前应再次确认将创建的模板或版本。
