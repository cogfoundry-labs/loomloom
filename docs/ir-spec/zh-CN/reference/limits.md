# 限制

- 每个目标 port 只有一个 binding。
- composeValue 只支持字符串 concat。
- merge 只支持有序 Artifact collection，至少两个来源。
- sequence item 不允许递归包含 compose、merge 或 sequence。
- stepOutput 必须引用直接依赖的 Step 和稳定输出 portId。
- Profile 的动态成员必须满足 Profile 端口契约；Template 不冻结成员列表。
- v2 没有动态创建 Step 数量的 Map/ForEach 语法。
