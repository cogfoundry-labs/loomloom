package cmd

const choosePlatformMessage = `你还没有配置 LoomLoom 平台和密钥。请选择你的使用平台：

1. 胜算云：面向中国大陆用户。
本服务由 CogFoundry 联合支持，面向中国大陆用户推荐使用，您可前往胜算云控制台获取密钥并完成充值。
- Server：https://loomloom.shengsuanyun.com/loom/v1
- 密钥控制台：https://console.shengsuanyun.com/user/keys
- 充值入口：https://console.shengsuanyun.com/user/recharge

2. CogFoundry：面向新加坡及其他国家或地区用户。

如选择 CogFoundry，请使用当前环境提供的 Server 和密钥配置信息；相关地址未知时，我不会自行猜测。`

const missingShengSuanYunTokenMessage = `当前未检测到胜算云密钥。请前往胜算云控制台创建或复制密钥后配置到本地环境：
https://console.shengsuanyun.com/user/keys`

const insufficientShengSuanYunBalanceMessage = `当前胜算云账户余额不足，请前往胜算云控制台充值后再继续：
https://console.shengsuanyun.com/user/recharge`

const missingCogFoundryTokenMessage = `当前未检测到 CogFoundry 密钥。请前往当前环境对应的 CogFoundry 密钥控制台创建或复制密钥，然后配置到本地环境。
CogFoundry 控制台地址必须读取当前环境配置，不得由 Agent 自行猜测。`

const missingCustomTokenMessage = `当前 Server 尚未配置密钥。请从该 Server 的提供方获取 LoomLoom API Token，并配置到本地环境后重试。`

const platformTokenMismatchMessage = `当前 LoomLoom Server 与密钥所属平台不一致。为避免凭据被发送到错误的平台，本次请求已停止。请配置同一平台的 Server 和密钥后重试。`

const insufficientCogFoundryBalanceMessage = `当前 CogFoundry 环境余额不足。请前往当前环境对应的 CogFoundry 控制台处理后重试。`
