# Setup

Use this reference for installation, `doctor`, platform selection, credentials, balance guidance, server profiles, console access, and token safety.

## Contents

- [Installation](#installation)
- [Diagnose before account guidance](#diagnose-before-account-guidance)
- [Platform and credential messages](#platform-and-credential-messages)
- [Persist verified credentials](#persist-verified-credentials)
- [Multiple Servers](#multiple-servers)
- [Token and Server safety](#token-and-server-safety)
- [Balance and console guidance](#balance-and-console-guidance)

## Installation

- Install through the GitHub install script by default.
- On macOS and Linux, the default installation uses Homebrew. Do not add `--no-brew` unless the user explicitly requests it.
- For an internal or beta CLI, install the prerelease channel explicitly:

  ```bash
  curl -fsSL https://raw.githubusercontent.com/Cogfoundry-ai/loomloom/main/install.sh | bash -s -- --channel beta
  ```

## Diagnose Before Account Guidance

Run `loomloom doctor --output json` before advising about a token, server, platform, balance, recharge, or console. Treat these fields as authoritative:

- `server`, `profile`, `platform`, `platform_preset`, `platform_operational`
- `token_env`, `token_set`, `token_valid`
- `healthy`, `credential_action`, `credential_message`, `next_action`

ShengSuanYun and CogFoundry are preset platforms, not a whitelist. If the user provides another Server, do not block it or replace it with a preset Server. Do not add `/loom/v1`; validate the exact URL the user provided.

Only a successful Doctor may register a new Server. Ordinary business commands must not use an unverified Server.

## Platform And Credential Messages

The messages below define required business content, not verbatim wording. Render them under the global language rule while preserving commands, URLs, identifiers, and required actions.

When no Server is configured, convey:

```text
你还没有配置 LoomLoom 平台和密钥。请选择你的使用平台：

1. 胜算云：面向中国大陆用户。
本服务由 CogFoundry 联合支持，面向中国大陆用户推荐使用，您可前往胜算云控制台获取密钥并完成充值。
- Server：https://loomloom.shengsuanyun.com/loom/v1
- 密钥控制台：https://console.shengsuanyun.com/user/keys
- 充值入口：https://console.shengsuanyun.com/user/recharge

2. CogFoundry：面向新加坡及其他国家或地区用户。

如选择 CogFoundry，请使用当前环境提供的 Server 和密钥配置信息；相关地址未知时，我不会自行猜测。
```

These are preset choices only. A user-provided compatible Server is also allowed.

For a missing ShengSuanYun token, convey:

```text
当前未检测到胜算云密钥。请前往胜算云控制台创建或复制密钥后配置到本地环境：
https://console.shengsuanyun.com/user/keys
```

For a missing CogFoundry token, convey:

```text
当前未检测到 CogFoundry 密钥。请前往当前环境对应的 CogFoundry 密钥控制台创建或复制密钥，然后配置到本地环境。
CogFoundry 控制台地址必须读取当前环境配置，不得由 Agent 自行猜测。
```

For a missing custom-platform token, use `credential_message`. Never guess its console, key, balance, or recharge URL.

When Server authentication rejects a Token, convey:

```text
当前 Server 可以访问，但密钥认证未通过。该密钥可能无效、已过期、权限不足，或不适用于当前 Server。请确认密钥由当前 Server 对应的环境提供后重试。
```

## Persist Verified Credentials

The CLI generates `profile` and `token_env`; the Agent must not invent either name.

When the user provides both a Server URL and Token and asks to install, configure, use, switch, or health-check LoomLoom, treat that as a request to register and activate that Server. The user does not need to separately ask to add a platform, overwrite configuration, or switch profiles. Use the exact URL and its corresponding Token for Doctor; do not modify the URL or reuse another Server's Token. For the first Doctor check, pass the pair explicitly as `loomloom doctor --server <exact-server> --token <exact-token> --output json`. Do not use temporary `LOOMLOOM_SERVER=... LOOMLOOM_TOKEN=...` assignments because an existing active profile takes precedence over legacy environment configuration. If Doctor succeeds, persist the Token through the returned `token_env`, register the Server, and make it active. If the Server already exists, update its Token and make it active. If Doctor fails, do not persist or switch anything; keep the current configuration active.

1. Run Doctor with the exact user-provided Server and Token.
2. If Doctor fails, do not persist the Server or Token.
3. When `next_action=persist_token`, write the Token to the returned `token_env` in the user's existing permanent environment configuration (`~/.zshrc`, `~/.bashrc`, the active bash profile, PowerShell `$PROFILE`, or the applicable user-level Windows environment).
4. Update an existing variable instead of appending duplicates.
5. Run `loomloom doctor --output json` again.
6. Continue only when `healthy=true`, `token_valid=true`, and `next_action=none`.

Never echo the Token while persisting or verifying it.

## Multiple Servers

- `loomloom server list` lists verified Server profiles.
- `loomloom server use <name-or-server>` switches the active profile without rerunning Doctor.
- `loomloom server remove <name-or-server>` removes local profile metadata, not the permanent Token variable.
- Do not switch profiles unless the user requests it.
- After a switch, run Doctor when platform or credential facts are needed.

## Token And Server Safety

- Inspect the final URL scheme and host before every request.
- Send a remote token only over HTTPS and only to the explicitly selected Server.
- Never retain a token across a redirect to a different scheme or host.
- Never try one token against multiple Servers.
- Never echo a real token in replies, logs, generated files, examples, or diagnostics.

## Balance And Console Guidance

- ShengSuanYun keys: `https://console.shengsuanyun.com/user/keys`
- ShengSuanYun recharge: `https://console.shengsuanyun.com/user/recharge`
- CogFoundry console URLs must come from the current environment or service; never guess them.
- For a custom Server, use only service-returned guidance.
