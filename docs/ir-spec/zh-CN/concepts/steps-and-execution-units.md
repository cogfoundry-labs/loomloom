# Step 与执行合同

Step 是 DAG 节点。v2 不再让作者用公共执行单元加模型 ID 猜测参数，而是显式引用：

- `fixedModelContract`：一个确定模型的一份认证合同。
- `capabilityProfile`：一组满足共同端口契约的可选模型。

合同决定目标端口、类型、MIME、数量和原生映射；Template 决定这些端口的值从哪里来。完整合同由 Core 查询权威目录并冻结，不接受前端提交合同对象。
