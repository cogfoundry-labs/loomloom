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

`executionBinding.kind=codeProfile` 时必须提供 `code`；作者无需填写 `profileId`、`profileRevision`。两者均省略时，Core 在校验及创建模板版本时补齐并冻结唯一的运行环境 `python@1`（CPython 3.12，标准库加 Pillow、numpy、pydantic、jinja2、PyAV）。显式提供时必须是 `python` / `1`；已退役的 `python-pure@1`、`python-image@pillow-12.3.0-1` 会被拒绝并提示省略这两个字段，未知环境不会回退。`executionBinding.resourceClass` 可选 `standard`（默认）或 `large`，档位随版本冻结（standard：30 秒、1 GiB 内存；large：120 秒、4 GiB）。可读取的输入文件为文本、JSON、PNG/JPEG/WebP 与 MP4/WebM/QuickTime。不允许作者在执行时安装依赖、访问网络或取得平台 Secret。用户异常的完整消息和 traceback 只在作者执行明细（`loomloom run step-executions`）中显示，Run 与结果行只显示异常类型。

| `code` 字段 | 合同 |
| --- | --- |
| `language` / `entrypoint` | 可省略；只支持 `python` / `main`，省略时按此冻结 |
| `source` | 非空内联源码，定义 `main(inputs)`；不是外部代码包引用 |
| `inputs` | 全部端口显式声明；无输入填写 `{}`。kind=value 支持文本及 JSON，kind=artifact 接收文件列表；全部输入都是必填、非 null，不需要也不建议再写 required/nullable（写 `required: true`、`nullable: false` 仍可接受，其他值拒绝） |
| `outputs` | 至少一个命名输出；`main` 返回对象的 key 与声明端口对应 |

输入 key 与 inputBindings 一一对应。Value 声明 valueType（string、boolean、integer、number、array、object），可声明对象形式的 schema；禁止外部 Schema 引用，不隐式转换类型或生成默认值。文件声明 acceptedMimeTypes、minItems、maxItems，满足 1 <= minItems <= maxItems <= 16；实际文件还受冻结运行环境约束。输入解析或运行结果不符时，在 Code 派发前拒绝。普通含 Code 工作流不设额外节点数量上限。

输出分为两种：

- `kind=value`：声明 `valueType` 和对象形式 `schema`，支持 string、boolean、integer、number、array、object；运行时按平台支持的 Schema 子集验证，不能把任意 JSON Schema 方言都视为可用。
- `kind=artifact`：声明具体 `mimeType`；返回在本次工作目录内创建的相对文件路径。平台验证文件、大小和类型后发布 Artifact，不返回任意本机文件或外部 URL。

所有输出验证通过后才发布，非法或缺失输出不会作为成功结果传给下游。完整可复制示例见 [Code Value 与 JSON 文件](../examples/valid/code-value-artifact.json)。保存后源码、运行环境与输入/输出合同冻结；修改代码要创建新模板版本。

## 条件执行 `when`

`when` 声明 `stepId`、`portId`、布尔 `equals`。来源必须是直接依赖中的 Code boolean Value，或 Loop 显式声明的接受出口 boolean Value。字符串 `"true"`、null、缺失或错误输出不会被隐式转成布尔值。

比较不匹配时当前节点为 skipped；`require_all` 后继继续跳过，不产生这些节点的 Code Attempt 或模型调用。技术失败和正常条件不匹配是不同结果。首期不支持互斥分支 OR 汇合、不同条件的嵌套，条件路径使用 `require_all`。示例见 [Code 布尔条件](../examples/valid/code-condition.json)。

<a id="ref-loop-scope"></a>

## TS-LOOP-001：有界质量返工

Loop 容器的 `executionBinding` 只写 `{"kind":"loop"}`；不带普通输入绑定、Code 或模型选择，但可以带 `when`：条件不满足（或依赖被跳过）时整个 Loop 为 `skipped`、零轮次，下游按普通规则跳过。Root 和 body 各自是无环依赖图，返工由容器控制，不向普通 `dependsOn` 加回边。

| `loop` 字段 | 合同 |
| --- | --- |
| `maxIterations` | 必填，1–10；包含首次执行，不额外增加一次 |
| `deadlineSeconds` | 必填，1–1800 秒；业务轮次和截止共同限制执行 |
| `body` | 非空普通子工作流；当前支持固定文本 Profile 与普通 Code Value/Artifact，使用 `require_all`，可声明普通 `when` |
| `control` | 必填，唯一的控制写法。`control.decision` 必填，`{stepId, portId}` 指向体内 Code string Value，只允许 continue/accept/reject，整轮完成后读取；`control.checkpoints` 可省略，填写时为按依赖顺序排列的 Code string Value，用于本轮中途提前结束 |
| `state` | 显式状态 map，可为空；每项声明 `initial` 和 `update`，每个状态必须有体内消费者 |
| `outputs` | 非空出口 map；alias 映射到体内 Code Value/Artifact 或冻结文本模型 Artifact，只在接受后对外可见 |

`state.initial` 支持非空字符串 literal、必填 string Template Input 或字符串 `composeValue`。`update` 指向体内 Code string Value；消费者使用 `{"source":"loopState","inputKey":"状态名"}`。模型只把状态绑定到 prompt；Code 目标必须声明必填、非 nullable string。未通过时更新状态并进入下一轮；通过时冻结接受输出。耗尽、取消或技术失败不发布最后一轮候选作为成功结果。

`control.decision` 的 continue 进入下一轮（最后一轮则 exhausted），accept 接受本轮出口，reject 业务拒绝并结束，不更新 state 或发布候选。只需"通过就交付、否则返工"时，判断 Code 返回 accept 或 continue 即可；可在输出 schema 中声明 `enum`。判断 Code 的成功与 Loop 的业务拒绝分开：Loop 摘要为 rejected，公共 Task/Run 仍沿既有失败状态收口，已执行模型费用保留。`control` 不能缺失或为 null；其他 decision 值失败，不隐式转为继续。旧的顶层 `until` 与 `decision` 已停用，保存时拒绝。示例见 [整轮三路业务决策](../examples/valid/loop-business-decision.json)。control.checkpoints 在批次执行并持久化后、下批调度前求值，continue 继续本轮，accept/reject 返回退出意图并为未运行节点保存 LOOP_EXITED，无 Attempt/费用。父 Loop 对 accept 校验本轮全部出口；检查点提前 accept 时，位于检查点下游的出口尚未产生，会失败并指明出口名，因此只用于 reject 的检查点可以位于出口上游。control.decision 在正常整轮完成后读取，continue 才进入下一轮。检查点来源节点唯一且按依赖祖先顺序排列；来源被跳过或非法时失败，已派发并行节点先收齐，系统失败/取消优先。示例见 [中途检查点](../examples/valid/loop-checkpoint.json)。当前不支持跳过本轮、零轮、即时抢占或并行退出仲裁；普通 when 不等于结束 Loop。

体内模型固定选择当前可用文本模型，按普通规则解析内置或动态 Profile；可依赖体内 Code 并消费字符串 Value，Code 也可消费模型文本并返回反馈。body 引用自身作用域的依赖/输出，也可以读取外层步骤 X 的输出，前提是 X 同时写在容器和该 body 步骤的 `dependsOn` 中（X 不能是另一个 Loop）；外部节点只读取容器声明的 alias，不直接读取体内节点。body 的 Code 可沿用普通 Profile 产生文件并在本轮消费；文本、JSON 和文件通过一套显式输入声明选择交付类型。未接受候选保存在私有事实；接受时可转发所选轮次文件的引用和完整性信息，不复制文件内容。文件-only 和混合出口都支持，同源多别名有各自稳定的导出身份。轮内消费与接受轮文件出口已交付测试环境，真实接受/外层消费/下载与失败边界通过；模型直接出口与 Artifact alias 进入普通 sequence/merge 已交付受控测试环境，真实文本合并、耗尽不发布及 native 视频 sequence 通过。内置文本端口为 output；声明 text 输出的动态文本 Profile 可引用 text，导出仍为 text/plain Artifact，模型文本产物最多 2 MiB；平台读取实际字节验证完整性。接线仍验证目标合同、MIME、来源数量、依赖和缺失策略，不把 Value 转成文件，不重复模型调用或费用。图片、视频等 Profile 与固定模型合同也可放在 body 中（一轮可返回至多 16 个同 MIME 族的文件，文本 2 MiB、其他 64 MiB，供应商地址会先转存）；固定模型合同的 prompt 需经 Code 步骤转换后再绑定 `loopState`。尚不支持嵌套 Loop。示例见 [条件与外层读取](../examples/valid/loop-outer-read-conditional.json)。新增示例见 [模型产物出口与合并](../examples/valid/loop-model-artifact-merge.json)。完整示例见 [文本生成与有界返工](../examples/valid/bounded-text-loop.json)。

Loop 视觉文本 Profile 已交付受控测试环境，真实二轮接受/裁切交付与三轮耗尽通过：普通单图视觉 Profile 接收 PNG/JPEG/WebP 的必填单张 Artifact，输出仍为 text/plain；参考图按普通绑定进入每轮请求，反馈仍为 string state。内置 text.vision.openai-chat.v1 与对应已发布动态文本视觉 Profile 沿用普通模型合同，图像/视频生成 body 见上文。示例见 [视觉反馈返工](../examples/valid/loop-vision-feedback.json)。

条件未选择的节点按本轮身份记录 skipped，不产生 Attempt 或模型费用。继续下一轮需要本轮 state 更新值，接受需要本轮出口值；缺失必需值明确失败，不复用上一轮。文本连接与体内条件已交付测试环境，并通过真实文本返工、跨轮跳过及必需值缺失失败验收。文件示例见 [本轮 JSON 文件消费](../examples/valid/loop-code-file-consumption.json) 与 [接受轮文件交付](../examples/valid/loop-code-file-export.json)。文本示例见 [Code 到文本模型](../examples/valid/loop-code-to-text-model.json) 与 [接受条件交付](../examples/valid/loop-conditional-delivery.json)。

Code 和 Loop 当前用于受控测试评审。Schema 和本地校验通过不等于目标环境已开放作者入口、已配置运行环境或具备正式收费能力；真实可用范围以目标环境交付记录为准。
