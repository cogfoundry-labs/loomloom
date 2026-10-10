# 协议兼容边界

<a id="ref-compatibility-write-version"></a>

## TS-VERSION-002：新写入只接受 v2

- 新建模板版本和更新版本只接受 `template-spec/v2`。
- 已存在的 v1 TemplateVersion 继续按自己的冻结快照读取和运行。
- v1 不再作为新写入格式；迁移器读取 v1 后创建新的 v2 TemplateVersion，不原地修改历史版本。
- 当前阶段不在运行时长期维护 v1/v2 双写和大量兼容分支。

生产迁移按测试环境演练、预发布全量、生产分批执行。各环境都通过正式服务创建自己的 v2 版本，不能跨环境复制生成后的数据库记录。

未来稳定阶段可承诺有限的大版本兼容窗口；该长期策略需要跨团队单独评审，不反向扩大本次 v2 的实现范围。

## 四种版本各自负责什么

| 版本 | 含义 | 本阶段规则 |
| --- | --- | --- |
| `specVersion` | 作者 JSON 格式及语义，如 `template-spec/v2` | 一份模板只采用一种格式；v1 与 v2 不拼接使用。Code、when、Loop 扩展当前 v2 |
| TemplateVersion / `versionId` | 一次保存的不可变模板 | 修改代码、绑定或 Loop 后追加新版本，旧 Run 保留原版本 |
| 局部执行合同 / `code.contractVersion` / Profile revision | 对应节点的输入、环境或冻结执行语义 | 只在发生变化的层演进，不要求 Spec 和 API 同步升版 |
| API `/loom/v1` | 外部传输与资源接口 | 路径中的 v1 不表示模板只能使用 Spec v1 |

新增节点不自动要求 Spec v3。只有无法通过新字段、新类型或局部合同隔离的既有语义变化，才评审新的主版本及存量迁移。旧 reader 必须明确拒绝不支持的能力，不得忽略 Code/Loop 配置后按旧模型节点执行。
