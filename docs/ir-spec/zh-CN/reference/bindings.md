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

`merge` 在一个目标端口内显式声明多个有序来源，首版支持两个互斥 policy：

- `ordered_artifacts`：合并同构 Artifact collection。来源按 `sources[]` 排序，每个来源内部再按 Artifact ordinal 排序，最终结果按目标合同的 minItems/maxItems 校验；
- `concat_text`：仅接受两个及以上 `stepOutput` 文本来源，按 `sources[]` 顺序用 `\n\n` 拼接；目标端口必须允许文本多值输入和 `concat_text`。

缺失策略均为 `error` 或 `omit`。`composeValue` 只组合作者 literal 与 Workbook 字段，不用于运行期 Step 输出汇聚。
