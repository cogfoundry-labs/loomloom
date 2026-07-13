# 构建多步骤工作流

多步骤工作流把一个或多个上游 artifact 作为后一步输入。线性流程只有一个上游；汇总流程可声明多个上游，但必须显式描述每条数据流。

## 1. 声明依赖

```json
{
  "stepId": "stp_polish1",
  "displayName": "润色",
  "executionUnit": "text-generate",
  "dependsOn": ["stp_draft01"]
}
```

## 2. 声明数据流

```json
{
  "inputPort": "prompt",
  "sourceType": "step_output",
  "sourceStepId": "stp_draft01",
  "sourcePort": "output"
}
```

`sourceStepId` 必须也出现在 dependsOn。上游 output MIME 必须被目标 input port 接受：text output 可以进入 text-generate prompt，但不能直接进入 video-generate image。

## 3. 为下游选择模型和 instruction

每个模型驱动 Step 独立设置默认模型。下游 instruction 只描述本步骤处理要求，不复制上游正文。

## 4. 校验

检查 Step ID 唯一、图无环、端口名称存在，再执行本地 check 和服务端创建校验。完整示例见[线性多步骤模板](../examples/valid/multi-step-review.json)。

## 多上游汇总

为每个来源同时添加 `dependsOn` 和 `upstreamBindings`。多个来源写入同一端口时，该端口必须允许 multiple；例如 `text-generate.prompt` 可按 `concat_text` 合并多个文本 artifact，而 `video-generate.prompt` 不允许重复绑定。使用 `allow_partial` 时仍需显式绑定；创建前通过真实 validator 检查 MIME、端口和 trigger policy。
