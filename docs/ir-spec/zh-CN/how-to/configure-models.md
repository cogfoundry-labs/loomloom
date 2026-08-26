# 配置模型

固定模型使用：

```json
"executionBinding": {"kind": "fixedModelContract", "subjectRevisionId": "..."}
```

可替换模型使用 Capability Profile，并单独声明 `modelSelection`。不要把完整合同、provider 参数或模型路由值塞进 `executionBinding`。先查询目标环境的创作上下文：

```bash
loomloom template-spec authoring-context --output json
```

它返回当前 Profile、实时 revision、端口和可选模型。普通模板只写返回的 `profileId`，不写 `profileRevision`；Core 会在保存版本时冻结当时的 revision 与 hash。

文本输出不等于只能输入文本。根据实时返回选择对应 Profile：

- `text.basic.openai-chat.v1`：`prompt:string` 输入、文本输出；
- `text.vision.openai-chat.v1`：`prompt:string` + 单个 `image:Artifact` 输入、文本输出。

Vision Profile 的 `image` 端口必须绑定 Artifact Template Input，并严格遵守
`acceptedMimeTypes`、`minItems` 和 `maxItems`。不要把上传返回的 asset ID 声明为
string 后塞进 `prompt`。只有该 Vision Profile `eligibleModels` 中返回的模型才可
用于图片理解；不要根据模型名称猜测视觉能力。
