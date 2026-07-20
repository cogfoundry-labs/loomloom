# Setup

Use this reference for installation, `doctor`, platform selection, credentials, balance guidance, server configuration, console access, and token safety.

## Installation

- Install through the GitHub install script by default.
- On macOS and Linux, the default installation uses Homebrew. Do not add `--no-brew` unless the user explicitly requests it.
- For an internal or beta CLI, install the prerelease channel explicitly:

  ```bash
  curl -fsSL https://raw.githubusercontent.com/Cogfoundry-ai/loomloom/main/install.sh | bash -s -- --channel beta
  ```

- If the GitHub release API returns HTTP 403 while resolving the latest release, retry after a short wait, install a known tag with `--version <tag>`, or ask the user which version to use.

## Diagnose Before Giving Account Guidance

Run `loomloom doctor --output json` when possible before advising about the account, token, balance, server, recharge, or console. Treat these returned fields as authoritative:

- `server`
- `platform`
- `platform_operational`
- `token_set`
- `credential_action`
- `credential_message`

Do not infer a platform link when `doctor` already returned the relevant state.

If no server is provided through `LOOMLOOM_SERVER` / `--server` and the CLI has no remembered server, use the platform-selection guidance below.

## Fixed Platform And Credential Messages

When `credential_action=choose_platform`, output this exact message:

```text
你还没有完整配置 LoomLoom Server 和密钥。请选择要使用的平台：

1. 胜算云：本服务由 CogFoundry 联合支持，面向中国大陆用户推荐使用，您可前往胜算云控制台获取密钥并完成充值。
   - Server：https://loomloom.shengsuanyun.com/loom/v1
   - 密钥控制台：https://console.shengsuanyun.com/user/keys
   - 充值入口：https://console.shengsuanyun.com/user/recharge
2. CogFoundry：面向新加坡及其他海外地区用户，当前支付和交易能力敬请期待；在 CogFoundry 计费功能上线前，请使用胜算云控制台创建 API 密钥。

当前阶段请先选择胜算云。
```

Whenever the user proactively asks where to obtain a token/API key, which platforms are available, which server to use, or how to set up an account, output the same full platform-selection message. This applies even if a platform or token is already configured. Do not replace it with only one ShengSuanYun link, and do not provide a CogFoundry website or console URL.

When a command passively reports `credential_action=missing_token` for ShengSuanYun, output this exact message:

```text
当前未检测到胜算云密钥。请前往胜算云控制台创建或复制密钥后配置到本地环境：
https://console.shengsuanyun.com/user/keys
```

When a command passively reports insufficient ShengSuanYun balance, output this exact message:

```text
当前胜算云账户余额不足，请前往胜算云控制台充值后再继续：
https://console.shengsuanyun.com/user/recharge
```

When `credential_action=cogfoundry_unavailable`, or the user asks to switch to, try, browse, or use CogFoundry, output this exact message and stop:

```text
CogFoundry 面向新加坡及其他海外地区用户，当前支付和交易能力仍在建设中，敬请期待。当前阶段请继续使用胜算云。
```

After that message:

- Do not offer or perform a switch to a CogFoundry host.
- Do not ask for a CogFoundry URL, API key, or token.
- Do not claim that CogFoundry read-only capabilities are usable.
- Do not explain or hint at switching steps.
- Do not provide a CogFoundry website or console URL.

## Token And Server Safety

- The production default server is `https://loomloom.shengsuanyun.com/loom/v1`, but the active server is the explicit `LOOMLOOM_SERVER` / `--server` value.
- Before every request, inspect the final URL scheme and host.
- Send a token only over HTTPS and only to the explicitly configured host.
- Do not automatically follow a redirect to another domain while retaining the token.
- Use a token only for the environment and platform that issued it.
- Never send a ShengSuanYun token to a CogFoundry host or a CogFoundry token to a ShengSuanYun host.
- Never echo a real token in replies, logs, generated files, examples, or diagnostics.

## Console And Run Status

For ShengSuanYun:

- API keys: `https://console.shengsuanyun.com/user/keys`
- Recharge: `https://console.shengsuanyun.com/user/recharge`

For run status, prefer `loomloom run watch <run-id>`, `loomloom run get <run-id>`, and `loomloom run result-workbook <run-id>`.

There is no stable URL template for a Workflow Run detail page. Use a URL only when the service explicitly returns it; otherwise do not construct one from a `runId`.
