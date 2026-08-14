# Input Bindings

`steps[].inputBindings` 的 key 是目标合同 `portId`。每个端口只能有一个 binding。

<a id="ref-ports-and-bindings-step-output"></a>

## Step 输出

```json
"image": {"source": "stepOutput", "stepId": "stp_source1", "portId": "output"}
```

来源 Step 必须存在、不能是自身，并且必须同时出现在当前 Step 的 `dependsOn`。`portId` 是冻结输出合同中的稳定身份，不使用 role、文件名或 native JSON pointer 代替。

## Template Input、literal 与平台上下文

```json
"prompt": {"source": "templateInput", "inputKey": "creativePrompt"}
"duration": {"source": "literal", "value": 5}
"user_id": {"source": "platformContext", "contextKey": "user.id"}
```

`templateInput` 和 `composeValue` 可以声明 `fallbackValue`；只有动态来源没有值时才使用 fallback。

## composeValue

只支持确定性的字符串 `concat`，parts 只能是字符串 Template Input 或非空 literal。它用于“固定作者要求 + 用户变量”，不是通用表达式语言。

## sequence

`sequence` 构造一个位置敏感的异构原生值。item 必须声明 `position`、`kind` 和 source；`kind` 为 text/image/video/audio。它不等于 Artifact merge。

## merge

`merge` 把多个同构 Artifact collection 按 sources 声明顺序合并。每个来源内部再按 Artifact ordinal 排序。首版 policy 为 `ordered_artifacts`；缺失策略为 `error` 或 `omit`，最终结果仍按目标合同的 minItems/maxItems 校验。
