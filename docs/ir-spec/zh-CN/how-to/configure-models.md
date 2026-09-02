# 配置模型

先按 Step 的业务输入和输出查询目标环境当前支持的创作方式：

```bash
loomloom capability resolve --input text --output-modality image --output json
```

查询结果可能同时包含两种选择：

- 需要使用某个精确模型及其专属接口时，使用返回的 `fixedModelContract` 和真实 `subjectRevisionId`；
- 希望使用者能在一组接口兼容的模型之间选择时，使用返回的 `capabilityProfile`。

不要根据模型名称、Provider 路径或历史文档猜测能力。优先使用
`capability resolve` 的匹配结果；需要查看全部 Profile 时再运行：

```bash
loomloom template-spec authoring-context --output json
```

动态 Capability Profile 返回：

- `profileId`：模板引用的稳定身份；
- `definition`：发布后固定的输入、输出和约束；
- `operations.defaultModelId` 与 `defaultModelAvailable`：当前运营默认模型及其可用状态；
- `eligibleModels`：根据当前模型能力事实实时计算的可选模型集合。

普通模板只写稳定的 `profileId`，不要写 `profileRevision`。TemplateVersion
会保存固定接口快照，但不会冻结当时的完整模型集合。模型上架或下架后，后续
Validate、Precheck 和 Run 都会使用最新匹配集合；已经接受的 Run 使用其已经
选定的具体模型。

当前环境可能提供文本、图片或视频 Profile，例如：

- `text.basic.openai-chat.v1`：文本提示词输入、文本输出；
- `text.vision.openai-chat.v1`：文本提示词和图片 Artifact 输入、文本输出；
- `image.text-to-image.v1`：文本提示词输入、图片 Artifact 输出；
- `video.text-to-video.v1`：文本提示词输入、视频 Artifact 输出。

这些 ID 只是示例，是否可用以及端口结构必须以目标环境实时返回为准。模型拥有
Profile 未声明的额外能力时，这些能力不会出现在当前 Step 接口中。

Profile Step 仍需单独声明 `modelSelection`。当前 TemplateSpec v2 请求结构要求
模型选择输入和 `defaultModelId`；创建时使用当前返回的默认模型，并确保它出现在
`eligibleModels` 中。使用者将模型列留空时，动态 Profile 会采用运行时当前运营
默认模型；显式填写时只能选择当时仍在 `eligibleModels` 中的模型。若当前默认模型
不可用，第一阶段直接报错，不会静默替换。

Artifact 输入输出必须严格遵守 `definition` 中的 `kind`、`acceptedMimeTypes`、
`minItems` 和 `maxItems`。不要把上传返回的 asset ID 声明为 string 后塞进
`prompt`，也不要把 Artifact 输出当作文本端口连接。
