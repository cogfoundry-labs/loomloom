# 用 CLI 评审 Code 节点闭环

本流程用于目标测试环境已交付并开放的 Code/Loop 版本。CLI 需包含本手册及 Code Value、Loop 结果读取支持；旧发行版不能仅凭命令存在认定支持。测试身份须能保存自己的模板、执行受控 Code，并读取对应结果。正式收费、生产开放及 Sandbox 原生回收回执不属于本流程的完成证明。

## 1. 准备 Spec

先复制 [Code 文本与文件示例](../examples/valid/code-value-artifact.json) 为 `code.json`。用测试环境认证身份执行以下命令；每条命令明确选择目标测试 Server，例如追加 `--server https://loomloom-test.shengsuanyun.com/loom/v1`。认证使用已配置的凭据，不把 Token 写入 Spec 或共享命令记录。

```bash
loomloom template-spec check code.json
loomloom template-spec create code.json --version-note "Code review"
```

记录返回的 templateId/versionId。校验不保存；创建时会再次校验并冻结。Code 的 Profile/revision 必须有服务端运行绑定，用户不选择 Sandbox ID、集群或凭据。

## 2. 读回和修改

```bash
loomloom template-spec get-version <template-id> <version-id> -f saved.json
loomloom template-spec create-version <template-id> code.json --version-note "Updated Code"
```

核对读回的源码、绑定和输入/输出声明。修改后创建新版本，保留原版本 ID；后续运行显式选择需要评审的版本。

## 3. 提交输入

创建 `rows.jsonl`，每行是模板输入 map：

```jsonl
{"text":" Hello Code "}
```

```bash
loomloom orchestration-input upload rows.jsonl
loomloom template-spec precheck <template-id> --version-id <version-id> --input-file-id <input-file-id>
loomloom template-spec run <template-id> --version-id <version-id> --input-file-id <input-file-id> --client-request-id <request-id>
```

上传返回的 inputFileId 是批次行数据身份，不是素材 inputAssetId。预检不执行代码；受控测试中模型估算/费用不等于 Code/Sandbox 正式价格。`CODE_PRICING_PENDING` 表示没有有效测试准入，不能通过改成零价、替换身份或直接写数据库绕过。Loop 未开放或运行绑定缺失也应交由研发按交付配置处理。

一次新执行使用新的 requestId；网络结果不确定时，相同版本、输入和 requestId 重试应返回原 Run。修改输入属于新执行。该机制不表示可以自动重执行结果 unknown 的 Code Attempt。

## 4. 查看和下载

```bash
loomloom run watch <run-id>
loomloom run result-rows <run-id>
loomloom run result-rows <run-id> --output json
loomloom run result-workbook <run-id> --output-file result.xlsx
```

第一行预期 completed，Value text 为 `Hello Code`，passed 为 true，JSON Artifact 是独立交付文件。文本模式显示结果预览；JSON 模式保留完整 Value 及类型、Loop 状态/轮次和 stepErrors。模型费用快照不是钱包结算回执；未知费用标记不应丢失。

## 5. 条件和返工

- [条件示例](../examples/valid/code-condition.json)：分别提交 `{"text":"Hello Code"}` 和 `{"text":"REJECT"}`。示例规则仅用于演示；后者的源 Code 正常返回 passed=false，下游条件节点跳过，不把正常跳过当成代码异常。源节点文件仍是它自己的结果，不是被跳过节点的产物。
- [返工示例](../examples/valid/bounded-text-loop.json)：先用 `loomloom template-spec authoring-context --output json` 查询并替换可用 text Profile 的模型 ID，再 check/save/run；输入为 `{"request":"Reply exactly HELLO"}`。观察接受轮次与最终出口。模型返回其他文本时最多三轮，耗尽没有接受结果，不发布最后候选。首次通过和返工通过受真实模型响应影响，固定反馈案例的正式验收引用具体 Run 证据。
- 修改为非法输入合同或超过十轮时，check 应失败；有依赖却引用体内未导出的节点也应失败。静态 Schema 只校验结构，不能替代服务端权限、作用域和权威合同验证。

## 6. 评审记录

记录操作入口/CLI 版本、Server、模板版本、Run、输入样例、预期与实际结果，以及创建、配置、校验、执行、错误、跳过、返工和下载各步骤的产品问题。产品流程与研发合同的待定项分别记录。测试可用窗口、支持 owner、部门文档/宣发/销售准备由交付评审确认；演示通过不等于这些事项已确认或产品已生产上线。
