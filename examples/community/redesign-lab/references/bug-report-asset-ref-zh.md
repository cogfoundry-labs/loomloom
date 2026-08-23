# Bug 报告：`asset_ref` 通过 `reference` 端口绑定后，模型始终收不到真实内容

## 一句话总结

通过 `initial_input` 将 `asset_ref` 类型的输入字段绑定到 `reference` 端口后，
模型从未真正收到被引用的文件内容——无论是图片还是纯文本，无论是
`text-generate` 还是 `image-generate`，无论用哪个模型，均如此。这是服务端
问题，客户端（CLI 本身）代码经核实并无问题。另外还发现一个独立的计费
问题：`precheck` 预估费用可能与实际扣费相差数倍。

## 复现步骤（标准流程，完全按照 CLI 自带文档操作）

```bash
loomloom input-asset upload <file> --content-type <mime>      # 拿到 input_asset_id
echo '{"<字段名>":"<input_asset_id>"}' > rows.jsonl
loomloom orchestration-input upload rows.jsonl                 # 拿到 input_file_id
loomloom template-spec precheck <template-id> --version-id <v> --input-file-id <id>
loomloom template-spec run     <template-id> --version-id <v> --input-file-id <id>
```

## 三次真实付费测试，结论一致

| 运行 (runId) | 字段类型 | MIME 类型 | 模型 | 实际花费 | 模型返回内容 |
|---|---|---|---|---|---|
| `f1193db9-54e6-456a-9c8b-af5221004830`（历史记录，Bunnings 项目） | `asset_ref` | image/png | `google/gemini-3.1-pro-preview` | $0.0238（6个任务） | 全部 6 条回复均表示"未看到图片" |
| `2e479dae-fe83-45e5-a486-06ebb0daba6c` | `asset_ref` | image/png | `google/gemini-3.1-pro-preview` | $0.0032 | "I am assigning a neutral score because no design or image was provided for me to evaluate." |
| `2d4fdd00-a50e-4457-a2e9-7c430dd33c79` | `asset_ref` | image/png | `google/gemini-2.5-pro` | $0.0030 | **更严重**：模型编造了一个听起来合理但完全不符实际截图的描述（"dark palette... vibrant gradient"），而实际截图是纯白背景、黑色文字、单一红色强调色，没有任何深色或渐变元素 |
| `bce7b58d-7b93-4520-8116-981f557ee300` | `asset_ref` | text/plain | `google/gemini-2.5-flash` (text-generate) | $0.000147 | 上传的文本文件包含 3 个编造的、具体的事实（人名、批次号、数字），模型回复："No document was provided." |
| `045a0c85-24f6-4b8f-b23f-d428dea7fe1e` | `asset_ref` | image/png | `google/gemini-2.5-flash-image` (**image-generate**，非 text-generate) | $0.2979902 | 见下方第 5 点 |

第 3、4 次测试使用的是全新构建、结构完全正确的诊断用 TemplateSpec
（`template-spec check` 验证通过，`precheck` 成功返回真实费用估算），
用来排除"图片专属问题"或"某个模型专属问题"的可能性——结果证明这个问题
与 MIME 类型、模型选择均无关。

第 5 次测试换用 `image-generate`（而非 `text-generate`）执行单元，同样把
`reference` 端口绑定到上传的真实截图，并明确要求：如果确实收到了参考图，
就生成一个色调完全一致的变体；如果没收到，就生成一个纯灰色方块、上面用
红字写"NO REFERENCE IMAGE RECEIVED"——这样无论结果如何都能清楚判断。
实际返回的却是一张完全无关的、写实风格的"空白白色公寓室内"照片，
既不符合参考图的视觉风格（米白背景、黑色文字、单一红色强调色），
也没有执行明确要求的"未收到参考图"兜底指令。这说明该问题不局限于
`text-generate`，`image-generate` 的 `reference` 端口也存在同样的
内容送达缺失问题。

## 已排除的可能原因

1. **不是字段名写错**：TemplateSpec 中声明的字段 key（如 `screenshot`）与
   JSONL 行里使用的 key 完全一致，`template-spec check` 验证通过。
2. **不是命令用错**：确认这是私有模板（`template-spec` 系列命令），
   而非公开 Market SkillBot（`market` 系列命令）；已改用正确的命令族。
3. **不是内容格式问题**：按官方文档确认，asset_ref/text_reference 的行
   取值应为 `input-asset upload` 返回的字符串 ID，不能内联 base64、
   也不能用嵌套对象——这两种错误格式在 `orchestration-input upload`
   阶段就会被服务端直接拒绝（分别报 400 和 500 错误），而我们最终使用的
   正确格式（纯字符串 ID）能顺利通过 `precheck`。
4. **不是端口/MIME 不兼容**：`loomloom template-spec docs execution-units`
   明确写明 `text-generate` 的 `reference` 端口同时接受 `text/*` 和
   `image/*`。
5. **不是 CLI 客户端代码的 bug**：逐行阅读了相关 Go 源码
   （`src/cli/internal/cmd/input_asset.go`、`orchestration_input.go`、
   `template_spec.go` 中 `precheck`/`run` 的实现，以及共用的
   `src/cli/internal/client/http.go`）。上传命令把本地文件的原始字节
   （通过 Go 的 `[]byte` JSON 编码，即标准 base64）完整发给
   `/inputAssets:upload`；JSONL 上传命令把文件内容原封不动发给
   `/orchestrationInputs:upload`，没有做任何解析或字段过滤；`precheck`/`run`
   只是把 `{versionId, inputFileId}` 发给对应端点。`PostProductJSON` 和
   `PostJSON` 底层是同一个函数、同一个 base URL，不存在"上传和运行打到
   不同环境"的可能。CLI 这一层没有问题。

## 目前的结论

`asset_ref`（可能也包括 `text_reference`，但这一类型在更早的 `precheck`
阶段就直接报错 `"row 0: no non-empty fields found"`，属于另一个独立问题，
未继续深入排查）通过 `initial_input` 绑定到 `reference` 端口后，服务端
似乎从未真正把已上传资产的内容解析并交给模型——这段逻辑不在本地 CLI
代码仓库范围内，无法进一步定位具体原因，需要 loomloom 后端团队协助排查。

**风险提示**：`google/gemini-2.5-pro` 在收不到真实图片时，并未诚实说明
"没有收到图片"，而是编造了一段听起来专业、但完全不符合实际内容的评价。
这比"明确报错"更危险——如果人类信任这类反馈，可能会基于凭空捏造的内容
做出真实的设计决策。

## 第 6 次测试：真实生产流程，规模化验证（6 个变体）

修复 `scripts/score.py` 之后，直接用真实的 6 个设计变体（同一次
`redesign-existing-site` 流水线运行产出的真实截图）跑了一次完整的
"optimized" 流程——同样的 upload → orchestration-input upload → precheck
→ run。运行 ID `0d67cb57-e87f-42e6-9dbc-81805059c709`，实际花费 $0.0232848，
6/6 任务都显示"completed"，但**6 条回复无一例外都表示"未提供图片或设计
描述"**，同时又各自给出了不一致的分数（1/10、N/A、N/A、未给分、N/A、5/10）
——即便每一条都承认自己什么都没看到。这不是某个孤立诊断模板的问题，而是
本项目实际会使用的生产流程，在真实的 6 个变体规模下运行，结果仍然是 0/6。

截至目前，今天所有测试加总：**11 次真实付费任务执行，无一成功**——覆盖
4 个不同模型、2 种执行单元（text-generate / image-generate）、2 种 MIME
类型。这是在不查看 loomloom 服务端日志的前提下，能拿到的最确凿的证据了。

## 第 7 次测试：应用户明确要求，在第二个真实网站上再测一次

运行 ID `cf47317d-8668-4b95-a8fa-60b572964c9d`，实际花费 $0.0238392，6/6 任务
"completed"，这次是针对另一个完全不同的真实网站（larstornoe.com，一位家具
设计师的作品集）生成的 6 个真实设计变体。结果依旧一致：6 条回复全部表示
未收到图片或设计描述（其中一条给出"Score: 0"，一条给出"Pending/10"，其余为
"N/A"——即便声称看不到内容，仍然给出不一致的分数这一现象再次出现）。

截至目前，今天全部测试累计：**17 次真实付费任务执行，0 次成功**，覆盖 4 个
不同模型、2 种执行单元、2 个完全不同的真实目标网站。

## 另一个独立发现：`precheck` 费用预估严重偏低

与本 bug 无直接关系，但同一次运行（image-generate 测试）中发现：
`template-spec precheck` 给出的预估费用是 $0.0429957，实际扣费却是
$0.2979902——实际费用约为预估的 7 倍。这是计费准确性问题，与上面的
"内容未送达"问题性质不同，但同样建议一并反馈给 loomloom 团队：如果用户
是基于 precheck 的预估金额做出"是否执行"的决定，实际扣费可能远超预期。

## 相关文件（本仓库内）

- `references/model-policy.md`：完整调查记录（英文）
- `scripts/score.py`：调用方脚本，其中一个独立的客户端 bug
  （命令族用错、字段名用错）已在本次调查中一并修复
- 涉及的私有模板：`0c15dc18-7509-49d0-b9d2-f4114283155d`
  （`redesign-lab-aesthetic-scoring`）
