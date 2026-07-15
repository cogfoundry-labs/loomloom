# 模板与版本

Template 是长期身份，Template Version 是一次不可变定义快照。模板名称和用途可以延续，但每次结构变化都应创建新版本。

## 为什么版本不可变

运行记录、输入工作簿和 Market Listing Version 都需要能够追溯到执行时的确切定义。若原地修改旧版本，同一个 version ID 会在不同时间产生不同字段、步骤或成本，历史结果将无法解释。

## 哪些变化需要新版本

- 新增、删除或重命名输入字段。
- 修改 `valueType`、`required`、`multiValue` 或默认值。
- 增删 Step，修改依赖或 Binding。
- 修改固定 instruction、默认模型或 static params。
- 调整用户可见输出结构。

只修改本地草稿且尚未创建远程版本时，可以继续编辑同一个 JSON 文件。

## 工作簿兼容性

工作簿由具体版本的 `inputSchema` 派生。新版本即使只增加一个字段，也不应假设旧工作簿仍兼容。创建版本后重新下载，并在提交前执行 validate/precheck。

## 与 Market 的关系

Market 上架使用某个私有模板版本创建不可变 Listing Version 快照。以后创建新的私有模板版本不会自动替换正在售卖的 SkillBot；发布更新需要独立的 Market 审核流程。
