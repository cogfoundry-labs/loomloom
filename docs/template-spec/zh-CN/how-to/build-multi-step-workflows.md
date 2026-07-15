# 构建多步骤工作流

多步骤工作流把一个或多个上游 artifact 作为后一步输入。线性流程只有一个上游；固定并行分支可以从同一行输入启动多个独立根 Step；汇总流程可声明多个上游，但必须显式描述每条数据流。

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

## 固定并行分支

当分支数量和每条分支的处理过程由模板作者预先确定时，直接声明多个根 Step，并把同一个输入字段分别绑定到这些 Step。它们不需要 `branch`、`parallel` 或其他特殊字段；没有未完成依赖且输入已就绪的 Step 会进入同一轮调度。

```text
book_title
  ├─ scene_a ─> image_a
  └─ scene_b ─> image_b
```

每个下游图片 Step 只依赖对应的文本 Step，并用 `upstreamBindings` 把文本 `output` 连接到图片 `prompt`。模板需要十条分支时，重复声明十组 Step 和 binding；这仍然是固定 DAG 拓扑，不是 `expanded`。

完整 JSON 见[文本到图片的固定并行分支](../examples/valid/parallel-text-to-image-branches.json)。调度器会并发调度同一轮就绪的 Step，但实际同时执行数量仍受 worker 和模型服务并发限制。

## 多上游汇总

为每个来源同时添加 `dependsOn` 和 `upstreamBindings`。多个来源写入同一端口时，该端口必须允许 multiple；例如 `text-generate.prompt` 可按 `concat_text` 合并多个文本 artifact，而 `video-generate.prompt` 不允许重复绑定。使用 `allow_partial` 时仍需显式绑定；创建前通过真实 validator 检查 MIME、端口和 trigger policy。
