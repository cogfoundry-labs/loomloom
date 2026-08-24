# 校验、创建和追加版本

```bash
loomloom template-spec check template.json
```

创建请求必须使用 `specVersion=template-spec/v2` 和 `canonicalSpecV2`。`check` 在当前 Server 上执行与创建相同的权威合同解析，但不写入版本；创建时会再次校验并冻结版本。

读取 owner 自己的历史定义时使用：

```bash
loomloom template-spec get-version <template-id> <version-id> -f historical.json
```

该命令只导出 authoring spec，不导出 frozen execution bundle。v1 历史版本仍然不可直接追加或覆盖，必须参考当前 v2 文档和目标环境 authoring facts 手工重写为 v2。

修改模板时追加新的 TemplateVersion，不覆盖旧版本。新版本创建成功后重新下载 Workbook，validate/precheck 通过后再提交 Run。
