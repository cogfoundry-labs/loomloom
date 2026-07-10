package cmd

const choosePlatformMessage = `你还没有完整配置 LoomLoom Server 和密钥。请选择要使用的平台：

1. 胜算云：本服务由 CogFoundry 联合支持，面向中国大陆用户推荐使用，您可前往胜算云控制台获取密钥并完成充值。
   - Server：https://loomloom.shengsuanyun.com/loom/v1
   - 密钥控制台：https://console.shengsuanyun.com/user/keys
   - 充值入口：https://console.shengsuanyun.com/user/recharge
2. CogFoundry：面向新加坡及其他海外地区用户，当前支付和交易能力敬请期待；在 CogFoundry 计费功能上线前，请使用胜算云控制台创建 API 密钥。

当前阶段请先选择胜算云。`

const missingShengSuanYunTokenMessage = `当前未检测到胜算云密钥。请前往胜算云控制台创建或复制密钥后配置到本地环境：
https://console.shengsuanyun.com/user/keys`

const insufficientShengSuanYunBalanceMessage = `当前胜算云账户余额不足，请前往胜算云控制台充值后再继续：
https://console.shengsuanyun.com/user/recharge`

const cogFoundryUnavailableMessage = `CogFoundry 面向新加坡及其他海外地区用户，当前支付和交易能力仍在建设中，敬请期待。当前阶段请继续使用胜算云。`
