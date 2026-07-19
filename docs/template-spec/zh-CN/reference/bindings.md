# Bindings Reference

<a id="ref-ports-and-bindings-input-transport"></a>

## 输入传输规则

- **TS-IN-001**：直接粘贴文本使用 `string`，并绑定到允许的文本参数。
- **TS-IN-003**：不得声明 `string`，又要求填写资产 ID 并绑定到 prompt。运行时不会根据 `ia_*` 格式猜测语义。

<a id="ref-ports-and-bindings-uploaded-text"></a>

## 上传文本

**TS-IN-002**：上传文本使用 `text_reference`，通过 `sourceType=initial_input` 进入接受对应 MIME 的 input port。不得只用 FieldBinding 写入 prompt。

<a id="ref-ports-and-bindings-field-binding"></a>

## FieldBinding

| 字段 | 说明 |
| --- | --- |
| `fieldKey` | 已声明输入字段 |
| `stepId` | 已声明 Step |
| `paramKey` | unit 允许的运行参数，或受限的 model 路由参数 |
| `bindMode` | 新 authoring 使用 shared；expanded 仅用于读取和运行历史版本 |

新模板的参数字段必须使用 shared。同一 `stepId + paramKey` 只能有一个 Binding。model 只支持 shared 且要求 allowModelOverride；provider/mode 不支持。`multiValue` 内容集合不使用 FieldBinding，应通过 `initial_input` 进入允许多个内容的 input port。

<a id="ref-ports-and-bindings-param-binding"></a>

## ParamBinding

ParamBinding 包含 stepId、paramKey、bindMode、可选 separator 和非空 sources。source kind：

- `field_ref`：需要存在的 fieldKey。
- `literal`：需要非空 literal。

新模板只能组合单值 field source 和 literal，并使用 shared。multiValue field source 与 expanded 仅为历史版本运行兼容保留。model/provider/mode 不支持组合 Binding。

<a id="ref-ports-and-bindings-expanded-compatibility"></a>

## TS-TOPOLOGY-001：expanded 兼容边界

新建模板、新版本以及把其他历史版本切换为当前发布版本时，不得在 FieldBinding 或 ParamBinding 中使用 `bindMode=expanded`。当前已经发布的历史 expanded 版本可以继续读取、precheck 和运行，也允许对当前发布版本执行幂等发布操作。

迁移方式：独立对象使用 workbook 多行；固定数量的并行处理显式声明多个 Step；多个上游结果通过 `dependsOn` / `upstreamBindings` 汇聚。TemplateSpec v1 不支持按输入数组动态创建 Step 分支。

<a id="ref-ports-and-bindings-upstream-binding"></a>

## UpstreamBinding

| sourceType | 必需字段 | 规则 |
| --- | --- | --- |
| `step_output` 或空 | inputPort, sourceStepId, sourcePort | source step 在 dependsOn；port 存在且 MIME 兼容 |
| `initial_input` | inputPort, sourceInputKey | 字段存在，字段类型/MIME 与 port 兼容 |

同一 input port 能否绑定多次由 `allowMultiple` 决定；合并方式由 mergePolicy 决定。
