# 校验、创建和追加版本

```bash
loomloom template-spec check template.json
```

创建请求必须使用 `specVersion=template-spec/v2` 和 `canonicalSpecV2`。先本地检查，再提交一次创建请求；服务端会解析当前权威合同并冻结版本。

修改模板时追加新的 TemplateVersion，不覆盖旧版本。新版本创建成功后重新下载 Workbook，validate/precheck 通过后再提交 Run。
