# Limits Reference

## 当前公开 v1 限制

- 至少一个 Step、至少一个 input field。
- 多 upstream Step 必须声明显式 upstreamBindings；重复写入同一 input port 需要该端口 allowMultiple。
- Step ID 格式固定为 `stp_<6-10 base36 chars>`。
- ParamBinding 最多一个 multiValue field source。
- 同一 Step 参数只能有一个 FieldBinding 或 ParamBinding 来源。
- video-generate 的 prompt/image port 不允许同一 fan-in stage 多次绑定。
- provider 和 mode 不能由模板字段覆盖。
- 工作簿与 Template Version 绑定，版本变化后应重新下载。

## 不是固定文档常量的限制

- 当前可用模型和 provider。
- 模型支持的图片/视频参数取值。
- 账户额度、费用和 Market 定价。
- 上传文件大小、保留时间和环境级配额。

这些值必须从目标环境 API/CLI 或对应产品手册查询。

## 底层能力不等于公开能力

代码通过 `AllowMultiUpstreamFanIn` 控制能力。当前 server DI、UserTemplate 和 OfficialTemplate 均已开启；其他独立装配若未开启，仍会拒绝多个 upstream，因此跨环境使用前应以实际服务版本为准。
