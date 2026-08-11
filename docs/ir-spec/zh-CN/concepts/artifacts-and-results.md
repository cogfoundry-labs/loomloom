# Artifact 与结果

每次 Step execution 可以产生一个或多个 artifact。Artifact 记录 MIME type、port、producer step/run 和存储引用。

## 不要解析 artifact URI

代码将 URI 定义为平台存储返回的 opaque reference。客户端不应假设它是 OSS、S3、GS 或 data URI，也不应自行拼接下载地址。应使用公开 artifact/result API 或 CLI 下载。

## 三种结果视图

- Result rows：按原始输入行和步骤结果组织，适合程序读取。
- Result workbook：服务端把输入快照和 artifacts 对齐到可下载工作簿。
- Artifact list/download：访问单个文件或媒体产物。

结果工作簿比本地回填更可靠，因为服务端持有提交时的输入快照和真实 artifact 关系。

## 中间结果与最终结果

每个用户可见 Step 都可能有自己的 artifact。终端 Step 决定模板派生的 primary output capability，但多终端且能力不同的模板可能是 `mixed`。不要只根据文件扩展名推断步骤成功；同时检查 Step 状态、artifact MIME 和业务验收条件。
