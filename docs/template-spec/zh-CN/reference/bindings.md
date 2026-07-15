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
| `bindMode` | shared / expanded |

单值字段必须 shared，多值字段必须 expanded。同一 `stepId + paramKey` 只能有一个 Binding。model 只支持 shared 且要求 allowModelOverride；provider/mode 不支持。

<a id="ref-ports-and-bindings-param-binding"></a>

## ParamBinding

ParamBinding 包含 stepId、paramKey、bindMode、可选 separator 和非空 sources。source kind：

- `field_ref`：需要存在的 fieldKey。
- `literal`：需要非空 literal。

最多一个 multiValue field source；有多值来源时必须 expanded。model/provider/mode 不支持组合 Binding。

<a id="ref-ports-and-bindings-upstream-binding"></a>

## UpstreamBinding

| sourceType | 必需字段 | 规则 |
| --- | --- | --- |
| `step_output` 或空 | inputPort, sourceStepId, sourcePort | source step 在 dependsOn；port 存在且 MIME 兼容 |
| `initial_input` | inputPort, sourceInputKey | 字段存在，字段类型/MIME 与 port 兼容 |

同一 input port 能否绑定多次由 `allowMultiple` 决定；合并方式由 mergePolicy 决定。
