# 排查 TemplateSpec v2

1. JSON 解析失败：检查 lowerCamel 字段和未知字段。
2. 本地 check 失败：按 rule ID 查 Schema、正反例和 Reference。
3. 创建版本失败：检查 Subject/Profile revision、目标 portId、DAG 和值域兼容。
4. Workbook 失败：检查 input key、必填/空白策略、MIME 和数量。
5. Run 失败：按 Step 查看实际模型、合同、Gateway request ID、Provider 错误和 Artifact。

不要通过反复提交同一请求定位不确定状态；先使用稳定 client request ID 查询既有 Run。
