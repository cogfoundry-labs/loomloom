# 执行合同与端口

TemplateSpec v2 不再把公共 Execution Unit 清单作为作者协议。作者引用固定模型 Subject 或 Capability Profile，Core 从权威记录解析：

- 输入和输出稳定 portId；
- 标量类型与约束；
- Artifact MIME 和 cardinality；
- Provider 原生 JSON 映射；
- 适配器与执行面版本。

CLI 文档不能静态列出目标环境所有合同。创建 Spec 前应查询当前环境目录；保存时 Core 会再次解析并冻结，目录漂移或合同不匹配会失败关闭。
