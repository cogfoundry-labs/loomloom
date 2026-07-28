# Setup

Use this reference for installation, `doctor`, browser login, explicit API Token fallback, logout, platform selection, balance guidance, server configuration, console access, and token safety.

## Contents

- [Installation](#installation)
- [Diagnose Before Giving Account Guidance](#diagnose-before-giving-account-guidance)
- [Browser-First Credential Flow](#browser-first-credential-flow)
- [Platform And Credential Messages](#platform-and-credential-messages)
- [Browser Logout](#browser-logout)
- [Token And Server Safety](#token-and-server-safety)
- [Console And Run Status](#console-and-run-status)

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
- `healthy`
- `credential_action`
- `credential_message`

If Doctor reports `healthy=true`, continue with the existing credential. Do not require browser login and do not ask the user for another Token.

Do not infer a platform link when Doctor already returned the relevant state. If no Server is provided through `LOOMLOOM_SERVER` / `--server` and the CLI has no remembered Server, use the platform-selection guidance below.

## Browser-First Credential Flow

When Doctor does not find a valid credential, prefer browser login before asking the user to obtain or provide an API Token:

1. If the platform is operational and the environment is interactive, run `loomloom login` for the selected Server.
2. Ask the user to complete authorization in the browser. Do not handle, expose, or copy the browser credential yourself.
3. After browser login succeeds, run `loomloom doctor --output json` again and continue only when `healthy=true`.
4. Only after browser login fails, is unsupported, is unavailable in a headless/CI environment, or the user explicitly chooses Token authentication, use the explicit API Token fallback below.

Do not force browser login when Doctor already reports a healthy existing credential. Existing users may be authenticated through an explicit environment Token and should continue without interruption.

When the user explicitly asks where to obtain a Token/API key or explicitly requests Token-based setup, skip the browser-first attempt and provide the applicable platform Token guidance directly.

If browser login succeeds while an environment Token is already configured, explain that the browser credential was saved but the explicit environment Token remains the effective higher-priority credential. Do not describe this as a login conflict.

## Platform And Credential Messages

When `credential_action=choose_platform`, output this exact browser-first platform message:

```text
你还没有配置 LoomLoom 平台。请选择要使用的平台：

1. 胜算云：本服务由 CogFoundry 联合支持，面向中国大陆用户推荐使用。
   - Server：https://loomloom.shengsuanyun.com/loom/v1
2. CogFoundry：面向新加坡及其他海外地区用户，当前支付和交易能力敬请期待。

当前阶段请先选择胜算云。选择后将优先通过浏览器登录；只有浏览器登录未完成时，才需要配置 API Token。
```

When browser login fails or is unavailable for ShengSuanYun, or when the user explicitly requests Token-based setup, output this exact fallback message:

```text
浏览器登录未完成，你也可以使用胜算云 API Token 进行配置。
请前往胜算云控制台创建或复制密钥后配置到本地环境：
https://console.shengsuanyun.com/user/keys
```

When the user proactively asks which platforms are available, which Server to use, or how to set up an account, use the browser-first platform message above. When the user proactively asks where to obtain a Token/API key, use the Token fallback message. Do not provide a CogFoundry website or console URL.

When a command passively reports `credential_action=missing_token` for ShengSuanYun, do not immediately output the Token fallback message. Attempt browser login first according to the browser-first credential flow. Output the fallback message only if browser login does not complete or the user chooses Token authentication.

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

## Browser Logout

`loomloom logout` removes only the credential saved by `loomloom login`. It does not remove an explicit API Token from the user's environment or shell configuration.

Treat these logout fields as authoritative local state:

- `token_removed=true` means the saved browser login credential was removed.
- `environment_token_set=true` means a non-empty environment Token is still configured. It does not prove that the Token is valid.

When `environment_token_set=true`, explain that browser login was removed but the CLI will continue to prefer the environment API Token. Ask whether the user wants the Agent to remove the local environment Token configuration.

Only after the user explicitly confirms, locate and remove the definitions of `LOOMLOOM_TOKEN` and legacy `BATCHJOB_TOKEN` without exposing their values or modifying unrelated configuration. Explain that an already-running parent shell may require the user to unset the variables or open a new terminal.

When `environment_token_set=false`, explain that no environment API Token was detected.

Do not run Doctor merely to determine whether an environment Token exists; logout reports that fact locally without a network request. Run Doctor after logout only when the user asks whether the remaining Token is valid or whether authenticated commands still work.

Never modify shell startup files or user-level environment configuration without the user's explicit confirmation.

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
