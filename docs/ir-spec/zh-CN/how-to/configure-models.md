# 配置默认模型

模型目录是环境动态事实。编写 spec 前先查询目标环境支持对应 execution unit 的模型。

```bash
loomloom template-spec models text-generate
loomloom template-spec models image-generate
loomloom template-spec models video-generate
```

从返回结果选择真实 model ID，写入：

```json
"defaultModelRef": {
  "modelKey": "<returned-model-id>"
}
```

字段名为 `modelKey` 是兼容性命名，值存放可执行目录中的模型标识。不要把展示名称、provider 名或旧环境的 model ID 当成当前可执行 ID。

## 校验边界

JSON Schema 只能确认 modelKey 是非空字符串；服务端创建会确认模型存在并支持该 Step 的 execution unit。模型下线或能力变化属于动态环境错误，应重新查询目录。

## staticParams

只写 execution unit `AllowedRunParameters` 允许的 key。text-generate 只公开 prompt；image/video 的可选参数见 [Execution Units Reference](../reference/execution-units.md)。
