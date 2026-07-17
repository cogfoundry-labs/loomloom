# 工作簿与输入行

工作簿是 Template Version 的派生输入界面，不是 TemplateSpec 的另一种源码格式。

## 列如何生成

服务根据 `inputSchema.fields` 生成列，使用 key 作为机器映射、label 作为表头，并结合 order、required、enumValues、presentation 和 sourceKind 生成提示与校验。隐藏字段不会要求使用者填写。

## 一行代表什么

通常一行代表一个 task。普通字段给该 task 提供值；multiValue initial input 可以让一次 Step execution 同时接收多份内容。模板作者应在手册说明“一行是什么业务对象”，例如一篇文章、一个商品或一个候选人。需要独立处理多个对象时，应使用多行，而不是在一行中展开多个 execution。

## 推荐流程

```text
download-workbook
  -> 填写
  -> validate-workbook
  -> precheck-workbook
  -> 显示费用并确认
  -> submit-workbook
```

validate 不创建运行；precheck 估算执行与费用；submit 才会创建远程 run。版本变化后重新下载工作簿，不要依赖旧列布局。

JSON/JSONL 是程序化输入方式。除非调用者明确需要，面向人的批量流程优先使用工作簿。
