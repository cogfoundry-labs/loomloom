# 限制

- 每个目标 port 只有一个 binding。
- composeValue 只支持字符串 concat。
- merge 只支持有序 Artifact collection，至少两个来源。
- sequence item 不允许递归包含 compose、merge 或 sequence。
- stepOutput 必须引用直接依赖的 Step 和稳定输出 portId。
- Profile 的动态成员必须满足 Profile 端口契约；Template 不冻结成员列表。
- v2 没有动态创建 Step 数量的 Map/ForEach 语法。
- Code 运行环境和资源上限由平台 Profile 冻结；不允许运行时安装依赖、网络访问或注入 Secret。
- Loop 最多十轮、截止最多 1800 秒；首次执行计入轮次，首期仅文本固定模型与 Code Value 返工。
- 条件路径使用 require_all；不支持互斥分支 OR 汇合、不同条件嵌套或 Loop body 条件。
- 完整结构通过 JSON Schema 不代表目标环境授权、合同解析或运行可用；必须走服务端 check 和真实执行验收。
