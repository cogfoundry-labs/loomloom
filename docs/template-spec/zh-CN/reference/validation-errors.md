# Validation Errors Reference

按最早失败层修复：JSON → Schema → TemplateSpec validator → model/registry → workbook/input → runtime。

| Rule / 症状 | 原因 | 修复 |
| --- | --- | --- |
| TS-IN-001 | 直接文本未按普通字段使用 | string + compatible parameter binding |
| TS-IN-002 | text_reference 被写入 prompt，未作为 initial input | 绑定到接受 MIME 的 input port |
| TS-IN-003 | string 说明要求填写资产 ID | 改成普通文本，或改用引用字段 |
| meta.name is required | 名称为空 | 提供非空名称 |
| step_id pattern | ID 不满足 stp_ + base36 | 使用 6～10 位小写字母数字 |
| unknown execution unit | unit 不在 registry | 使用公开 unit |
| duplicate field/label/step | 标识重复 | 保持唯一 |
| source_kind default required | 隐藏/默认字段没有值 | 提供 defaultValue |
| duplicate target | 多条 Binding 写同一参数 | 只保留一种来源 |
| param not allowed | 参数不属于 unit | 查询 Execution Units |
| port/type not accepted | MIME 与 input port 不兼容 | 更换端口或上游类型 |
| model unavailable | model ID 不在当前环境或不支持 unit | 重新查询模型目录 |
| dependency cycle | Step 图有环 | 改为有向无环结构 |

稳定机器元数据见 [rules.json](../../machine/rules.json)。未分配 rule ID 的 validator 文本仍可能随实现改进，自动化不应依赖整段英文错误字符串。
