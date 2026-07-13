# 运行并读取结果

## 工作簿路径

```text
download-workbook -> fill -> validate-workbook -> precheck-workbook
-> 展示费用并获得确认 -> submit-workbook -> run watch -> result-workbook
```

validate 和 precheck 不创建收费运行。实际 submit 前必须针对当前文件和报价获得单独确认。

## 程序化输入

JSONL 运行使用与版本匹配的字段 key。先上传 orchestration input，执行 precheck，再在确认后 run。不要猜 step ID 或把 input asset ID 当成 input file ID。

## 结果读取

- `run watch`：等待终态。
- `run get`：查看运行与 task 摘要。
- `run result-rows`：读取结构化结果。
- `run result-workbook`：下载服务端对齐后的结果工作簿。
- `artifact list/download`：访问单个产物。

不要根据前 100 条聚合结果推断全量完成情况；需要全量 artifact 时使用支持分页的专用结果入口。
