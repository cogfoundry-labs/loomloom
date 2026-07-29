package cmd

const choosePlatformMessage = `你还没有配置 LoomLoom 平台和密钥。请选择你的使用平台：

1. 胜算云：面向中国大陆用户。
本服务由 CogFoundry 联合支持，面向中国大陆用户推荐使用，您可前往胜算云控制台获取密钥并完成充值。
- Server：https://loomloom.shengsuanyun.com/loom/v1
- 密钥控制台：https://console.shengsuanyun.com/user/keys
- 充值入口：https://console.shengsuanyun.com/user/recharge

2. CogFoundry：面向新加坡及其他国家或地区用户。
- Server：https://loomloom.cogfoundry.ai/loom/v1
- 密钥控制台：https://console.cogfoundry.ai/api-keys
- 充值入口：https://console.cogfoundry.ai/credits

选择胜算云后优先通过浏览器登录，浏览器登录未完成时可配置 API Token；选择 CogFoundry 或自定义平台时直接使用 API Token。`

const missingShengSuanYunTokenMessage = `当前未检测到胜算云密钥。请前往胜算云控制台创建或复制密钥后配置到本地环境：
https://console.shengsuanyun.com/user/keys`

const insufficientShengSuanYunBalanceMessage = `当前胜算云账户余额不足，请前往胜算云控制台充值后再继续：
https://console.shengsuanyun.com/user/recharge`

const missingCogFoundryTokenMessage = `当前未检测到 CogFoundry 密钥。请前往 CogFoundry 控制台创建或复制密钥后配置到本地环境：
https://console.cogfoundry.ai/api-keys`

const missingCustomTokenMessage = `当前 Server 尚未配置密钥。请从该 Server 的提供方获取 LoomLoom API Token，并配置到本地环境后重试。`

const tokenAuthenticationFailedMessage = `当前 Server 可以访问，但密钥认证未通过。该密钥可能无效、已过期、权限不足，或不适用于当前 Server。请确认密钥由当前 Server 对应的环境提供后重试。`

const insufficientCogFoundryBalanceMessage = `当前 CogFoundry 账户余额不足，请前往 CogFoundry 控制台充值后再继续：
https://console.cogfoundry.ai/credits`
