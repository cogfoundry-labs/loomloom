# 校验层级

1. JSON Schema：对象形状、必填字段和 source 判别联合体。
2. CLI check：本地 v2 Schema 与稳定 authoring rules，不访问远程环境。
3. Core validator：引用完整性、DAG、trigger、binding source 和 sample row。
4. 保存门禁：解析当前 Subject/Profile，校验端口、MIME、数量和值域安全子集，并冻结合同。
5. Run 校验：解析 Workbook/API 输入和 Artifact 权限。
6. Runtime：Provider、网络、内容安全和异步 Artifact 回导。

每层只证明自己的边界；本地校验通过不等于模型当前可运行。
