# Step

| 字段 | 必需 | 说明 |
| --- | --- | --- |
| `stepId` | 是 | 稳定 Step 身份 |
| `displayName` | 是 | 展示名称 |
| `dependsOn` | 否 | 调度依赖，必须无环 |
| `triggerPolicy` | 否 | `require_all`、`allow_partial` 或 `fail_fast` |
| `executionBinding` | 是 | 固定合同或能力 Profile 的权威引用 |
| `modelSelection` | Profile Step 必需 | 独立的模型路由声明 |
| `inputBindings` | 按合同需要 | 目标端口到输入来源的映射 |
| `code` | Code 节点必需 | Python 源码和命名输入/输出合同 |
| `when` | 否 | 直接依赖的布尔输出与 `equals` 比较，决定是否执行 |
| `loop` | Loop 容器必需 | 有限轮次、截止时间、体内节点、状态及接受出口 |

`dependsOn` 只说明调度关系，不自动传数据；真正的数据来源必须写在 `inputBindings`。反过来，引用 `stepOutput` 时也必须把来源 Step 放进 `dependsOn`。

<a id="ref-profiles-model-selection"></a>

## TS-PROFILE-002：Profile 模型选择

固定合同模型 Step 和 Code Step 不允许 `modelSelection`。Capability Profile 支持两种模型选择：

- `source=fixed`：指定 `defaultModelId`，不提供非空 `inputKey`。
- `source=templateInput`：`inputKey` 引用允许空值的字符串 Template Input；留空使用 `defaultModelId`，填写值由运行时按当前 Profile 成员校验。

模型 ID 和可用成员来自目标环境的作者目录，不能从模型名称推断合同。

<a id="ref-code-authoring"></a>

## TS-CODE-001：Code 节点

`executionBinding.kind=codeProfile` 时必须提供 `profileId`、`profileRevision` 和 `code`。当前已实现的运行环境为 `python-pure@1` 与 `python-image@pillow-12.3.0-1`；目标环境还必须具备对应运行绑定。图片环境包含 Pillow，不允许作者在执行时安装依赖、访问网络或取得平台 Secret。

| `code` 字段 | 合同 |
| --- | --- |
| `language` / `entrypoint` | 固定为 `python` / `main` |
| `source` | 非空内联源码，定义 `main(inputs)`；不是外部代码包引用 |
| `contractVersion` | 字符串 `"1"` 或 `"2"`，与 TemplateSpec v2 不是同一个版本号 |
| `inputs` | `"1"` 不声明显式输入合同；`"2"` 必须声明至少一个必填、非 nullable 的 string 输入 |
| `outputs` | 至少一个命名输出；`main` 返回对象的 key 与声明端口对应 |

输入的 key 是端口身份，与 `inputBindings` 的目标端口一致。`"2"` 的输入 Schema 可声明 `type=string` 及非负整数 `minLength`、`maxLength`；最小长度不得大于最大长度，不支持其他输入 Schema 关键字。输入解析会验证类型与长度，失败不执行代码。

输出分为两种：

- `kind=value`：声明 `valueType` 和对象形式 `schema`，支持 string、boolean、integer、number、array、object；运行时按平台支持的 Schema 子集验证，不能把任意 JSON Schema 方言都视为可用。
- `kind=artifact`：声明具体 `mimeType`；返回在本次工作目录内创建的相对文件路径。平台验证文件、大小和类型后发布 Artifact，不返回任意本机文件或外部 URL。

所有输出验证通过后才发布，非法或缺失输出不会作为成功结果传给下游。完整可复制示例见 [Code Value 与 JSON 文件](../examples/valid/code-value-artifact.json)。保存后源码、运行环境与输入/输出合同冻结；修改代码要创建新模板版本。

## 条件执行 `when`

`when` 声明 `stepId`、`portId`、布尔 `equals`。来源必须是直接依赖中的 Code boolean Value，或 Loop 显式声明的接受出口 boolean Value。字符串 `"true"`、null、缺失或错误输出不会被隐式转成布尔值。

比较不匹配时当前节点为 skipped；`require_all` 后继继续跳过，不产生这些节点的 Code Attempt 或模型调用。技术失败和正常条件不匹配是不同结果。首期不支持互斥分支 OR 汇合、不同条件的嵌套，条件路径使用 `require_all`。示例见 [Code 布尔条件](../examples/valid/code-condition.json)。

<a id="ref-loop-scope"></a>

## TS-LOOP-001：有界质量返工

Loop 容器的 `executionBinding` 只写 `{"kind":"loop"}`；不带普通输入绑定、Code、模型选择或 `when`。Root 和 body 各自是无环依赖图，返工由容器控制，不向普通 `dependsOn` 加回边。

| `loop` 字段 | 合同 |
| --- | --- |
| `maxIterations` | 必填，1–10；包含首次执行，不额外增加一次 |
| `deadlineSeconds` | 必填，1–1800 秒；业务轮次和截止共同限制执行 |
| `body` | 非空节点列表；首期为固定模型的 `text.basic.openai-chat.v1` 与 Code，使用无条件 `require_all` |
| `until` | `{stepId, portId}` 指向体内 Code boolean Value；判定节点须依赖整个 body |
| `state` | 显式状态 map，可为空；每项声明 `initial` 和 `update`，每个状态必须有体内消费者 |
| `outputs` | 非空出口 map；alias 映射到体内 Code Value，只在接受后对外可见 |

`state.initial` 支持非空字符串 literal、必填 string Template Input 或字符串 `composeValue`。`update` 指向体内 Code string Value；消费者使用 `{"source":"loopState","inputKey":"状态名"}`。模型只把状态绑定到 prompt；Code 目标必须声明必填、非 nullable string。未通过时更新状态并进入下一轮；通过时冻结接受输出。耗尽、取消或技术失败不发布最后一轮候选作为成功结果。

体内模型必须固定选择当前可用模型、没有体内上游依赖；Code 可消费模型文本并返回反馈。body 只引用自身作用域的依赖/输出；外部节点只读取容器声明的 alias，不直接读取体内节点。body 的 Code 输出首期只允许 Value；尚不支持图片/视频生成返工、嵌套 Loop、体内条件分支或 Loop 出口进入 sequence/merge。完整示例见 [文本生成与有界返工](../examples/valid/bounded-text-loop.json)。

Code 和 Loop 当前用于受控测试评审。Schema 和本地校验通过不等于目标环境已开放作者入口、已配置运行环境或具备正式收费能力；真实可用范围以目标环境交付记录为准。
