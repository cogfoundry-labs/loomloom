# Setup

Use this reference for installation, `doctor`, browser login, explicit API Token fallback, logout, platform selection, credentials, balance guidance, server profiles, console access, and token safety.

## Contents

- [Installation](#installation)
- [Diagnose Before Account Guidance](#diagnose-before-account-guidance)
- [Browser-First Credential Flow](#browser-first-credential-flow)
- [Platform And Credential Messages](#platform-and-credential-messages)
- [Persist Verified Credentials](#persist-verified-credentials)
- [Multiple Servers](#multiple-servers)
- [Browser Logout](#browser-logout)
- [Installer Uninstall Credential Cleanup](#installer-uninstall-credential-cleanup)
- [Token And Server Safety](#token-and-server-safety)
- [Balance And Console Guidance](#balance-and-console-guidance)

## Installation

- Use GitHub as the default distribution source.
- If the user explicitly requests Gitee, use the Gitee installer directly.
- If the GitHub installer or release download is unavailable or fails, tell the user that the official Gitee mirror is available and ask whether they want to retry through Gitee. Do not switch to Gitee until the user agrees.
- GitHub and Gitee are distribution sources only. Choosing either source must not select or change the LoomLoom platform, Server, or credentials.
- After the user chooses Gitee, use the matching installer:

  ```bash
  curl -fsSL https://gitee.com/cogfoundry/loomloom/raw/main/install-gitee.sh | bash
  ```

  ```powershell
  & ([scriptblock]::Create((irm https://gitee.com/cogfoundry/loomloom/raw/main/install.ps1))) -Source gitee
  ```

- On macOS and Linux, the default installation uses Homebrew. Do not add `--no-brew` unless the user explicitly requests it.
- For an internal or beta CLI, install the prerelease channel explicitly:

  ```bash
  curl -fsSL https://raw.githubusercontent.com/cogfoundry-labs/loomloom/main/install.sh | bash -s -- --channel beta
  ```

## Diagnose Before Account Guidance

Run `loomloom doctor --output json` before advising about a token, server, platform, balance, recharge, or console. Treat these fields as authoritative:

- `server`, `profile`, `platform`, `platform_preset`, `platform_operational`
- `token_env`, `token_set`, `token_valid`
- `healthy`, `credential_action`, `credential_message`, `next_action`

If Doctor reports `healthy=true`, continue with the existing credential. Do not require browser login and do not ask the user for another Token.

If Doctor does not report an already selected Server profile and the user has neither explicitly selected a platform nor provided a Server, the platform is unknown. In that state, present both preset platforms with their Server, credential, recharge, region, and authentication guidance, then ask the user to choose. Do not start either platform's authentication flow before the choice.

Never treat any of these as a platform selection:

- an unbound `LOOMLOOM_TOKEN` without a verified Server profile;
- the LoomLoom repository owner or download source, including `cogfoundry-labs/loomloom`;
- the installed CLI version, release channel, or apparent documentation maturity;
- the user's language, location, region, or a platform recommendation;
- one platform having a more familiar or complete setup path.

When the user asks where to get a Token/API key while the platform is unknown, present both platform choices and ask them to select one. Do not answer with only one platform's credential URL. A recommendation based on region may be shown as part of the comparison, but it must not silently select that platform.

ShengSuanYun and CogFoundry are preset platforms, not a whitelist. If the user provides another Server, do not block it or replace it with a preset Server. Do not add `/loom/v1`; validate the exact URL the user provided.

Only a successful Doctor or a browser login whose returned credential has been verified against the selected LoomLoom Server may register and activate a Server profile. Ordinary business commands must not use an unverified Server.

## Browser-First Credential Flow

API Token authentication is available for every platform. Browser login is available for both preset platforms, ShengSuanYun and CogFoundry. Determine the authentication flow from the user's explicit platform selection or the already selected Server profile reported by Doctor.

1. If no platform or Server has been selected, show both preset platforms and ask the user to select one before starting authentication. Do not run `loomloom login` yet.
2. After the user selects ShengSuanYun or CogFoundry, or when the existing selected profile is either preset platform, prefer `loomloom login` if the user can complete authorization in a browser.
3. Ask the user to complete authorization in the browser. Do not handle, expose, or copy the browser credential yourself.
4. If browser login for either preset platform fails, is unavailable in a headless/CI environment, or the user explicitly chooses Token authentication, use the applicable explicit API Token fallback below.
5. After the user selects a custom platform, or when the existing selected profile is custom, skip browser login and use the explicit API Token flow directly.
6. After either authentication flow completes, run `loomloom doctor --output json` and continue only when `healthy=true`.

When an interactive user runs bare `loomloom login` without a selected Server profile, the CLI can present its preset-platform selector. An Agent, CI job, piped command, or `--output json` invocation must not depend on that interactive selector or on an implicit platform default. After the user chooses a preset platform, pass its exact Server explicitly:

```bash
loomloom login --server https://loomloom.shengsuanyun.com/loom/v1
loomloom login --server https://loomloom.cogfoundry.ai/loom/v1
```

If Doctor already reports a selected preset Server profile, bare `loomloom login` uses that profile. Do not override it with another platform unless the user explicitly requests a switch.

Do not force browser login when Doctor already reports a healthy existing credential. Existing users may be authenticated through the selected profile's environment Token and should continue without interruption.

When the user explicitly asks where to obtain a Token/API key or explicitly requests Token-based setup, skip the browser-first attempt and provide the applicable platform Token guidance directly.

If browser login succeeds while the selected profile's `token_env` is already set, explain that the browser credential was saved but that environment Token remains the effective higher-priority credential. Do not describe this as a login conflict.

`loomloom login` waits up to five minutes for browser authorization by default. If the user needs more time, run `loomloom login --login-timeout 10m`. If automatic browser opening is unavailable but a browser can reach the same machine's loopback callback, use `loomloom login --no-browser --login-timeout 10m` and open the printed URL manually. In a truly headless or CI environment, use API Token authentication instead. `--login-timeout` controls only the human authorization window; the global `--timeout` flag controls individual HTTP requests, including the authorization-code exchange and credential verification.

The browser callback page only confirms that the authorization response returned to the CLI; it is not the final login result. Wait for the terminal command to complete. Browser login verifies the returned credential against the selected LoomLoom Server before saving or activating the profile. If code exchange or verification fails, do not claim that login succeeded and do not persist or switch configuration manually.

## Platform And Credential Messages

All source templates in this reference are written in English and define required business content, not verbatim wording. Translate and localize every user-facing message under the global language rule while preserving commands, URLs, identifiers, amounts, and required actions. The official Chinese name of ShengSuanYun is `胜算云`: use `胜算云` in Chinese responses and `ShengSuanYun` in non-Chinese responses; `ShengSuanYun (胜算云)` may be used on first mention when both names help identification.

When no Server is configured, convey:

```text
LoomLoom does not yet have a configured platform and credential. Choose a platform:

1. ShengSuanYun: for users in Mainland China.
This service is jointly supported by CogFoundry and is recommended for users in Mainland China.
- Server: https://loomloom.shengsuanyun.com/loom/v1
- API key console: https://console.shengsuanyun.com/user/keys
- Recharge: https://console.shengsuanyun.com/user/recharge
- Authentication after selection: prefer browser login; use an API Token if browser login does not complete or the user explicitly chooses Token authentication.

2. CogFoundry: for users in Singapore and other countries or regions.
- Server: https://loomloom.cogfoundry.ai/loom/v1
- API key console: https://console.cogfoundry.ai/api-keys
- Recharge and balance: https://console.cogfoundry.ai/credits
- Authentication after selection: prefer browser login; use an API Token if browser login does not complete or the user explicitly chooses Token authentication.

Ask the user to choose one. Do not authenticate with either platform until they choose.
```

These are preset choices only. A user-provided compatible Server is also allowed.

When browser login fails or is unavailable for ShengSuanYun, or when the user explicitly requests Token-based setup, convey:

```text
Browser login did not complete. You can configure ShengSuanYun with an API Token instead.
Create or copy an API key in the ShengSuanYun console, then configure it in the local environment:
https://console.shengsuanyun.com/user/keys
```

When a command passively reports `credential_action=missing_token` for ShengSuanYun, do not immediately output the Token fallback message. Attempt browser login first. Output the fallback message only if browser login does not complete or the user chooses Token authentication.

When a command passively reports `credential_action=missing_token` for CogFoundry, do not immediately output the Token fallback message. Attempt browser login first. Output the fallback message only if browser login does not complete or the user chooses Token authentication.

When browser login fails or is unavailable for CogFoundry, or when the user explicitly requests Token-based setup, convey:

```text
Browser login did not complete. You can configure CogFoundry with an API Token instead.
No CogFoundry credential was detected. Create or copy an API key in the CogFoundry console, then configure it in the local environment:
https://console.cogfoundry.ai/api-keys
```

For a missing custom-platform token after browser login is unsupported or not selected, use `credential_message`. Never guess its console, key, balance, or recharge URL.

When Server authentication rejects a Token, convey:

```text
The current Server is reachable, but credential authentication failed. The credential may be invalid, expired, insufficiently privileged, or intended for another Server. Confirm that it came from the environment corresponding to the current Server, then retry.
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
- `token_set=true` from `server list` means only that a local environment Token or saved browser credential exists; it does not establish current validity.
- `verified_at` records the last successful registration or verification, not current health. Use Doctor's current `token_valid` and `healthy` fields when validity matters.
- `loomloom server remove <name-or-server>` removes local profile metadata and its saved browser credential, not the permanent Token variable.
- Do not switch profiles unless the user requests it.
- After a switch, run Doctor when platform or credential facts are needed.

## Browser Logout

`loomloom logout` removes only the browser credential saved by `loomloom login` for the selected Server profile. It does not remove that profile's explicit API Token from the environment or shell configuration.

Treat these logout fields as authoritative local state:

- `token_removed=true` means the selected profile's saved browser login credential was removed.
- `environment_token_set=true` means the selected profile's non-empty environment Token is still configured. It does not prove that the Token is valid.

When `environment_token_set=true`, explain that the browser credential was removed but the CLI will continue to prefer the selected profile's environment API Token. Ask whether the user wants the Agent to remove the local environment Token configuration.

Only after the user explicitly confirms, identify the selected profile's CLI-generated `token_env` from an existing Doctor result or `loomloom server list --output json`, then remove that variable's definitions without exposing its value or modifying unrelated configuration. Never assume a fixed environment variable name. Explain that an already-running parent shell may require the user to unset the variable or open a new terminal.

When `environment_token_set=false`, explain that no environment API Token was detected for the selected profile.

Do not run Doctor merely to determine whether an environment Token exists; logout reports that fact locally without a network request. Run Doctor after logout only when the user asks whether the remaining Token is valid or whether authenticated commands still work.

Never modify shell startup files or user-level environment configuration without the user's explicit confirmation.

## Installer Uninstall Credential Cleanup

The standalone LoomLoom uninstall scripts remove the CLI, the bundled LoomLoom Agent Skill, and `config.json`. They do not remove environment API Tokens from shell startup files or user-level environment configuration.

Treat every output line in this form as an environment variable name that may require separate cleanup:

```text
environment token cleanup required: LOOMLOOM_TOKEN_<PROFILE>
```

When one or more names are reported:

1. Show the exact reported variable names to the user without reading or exposing their values.
2. Ask whether the user wants the Agent to remove those variables from permanent environment configuration.
3. Only after explicit confirmation, remove definitions for those exact names without exposing their values or modifying unrelated configuration.

Reported names are cleanup candidates and may not currently be configured. Do not claim that an environment variable is set without checking its permanent configuration after the user confirms cleanup.

## Token And Server Safety

- Inspect the final URL scheme and host before every request.
- Send a remote token only over HTTPS and only to the explicitly selected Server.
- Never retain a token across a redirect to a different scheme or host.
- Never try one token against multiple Servers.
- Never echo a real token in replies, logs, generated files, examples, or diagnostics.

## Balance And Console Guidance

- ShengSuanYun keys: `https://console.shengsuanyun.com/user/keys`
- ShengSuanYun recharge: `https://console.shengsuanyun.com/user/recharge`
- CogFoundry keys: `https://console.cogfoundry.ai/api-keys`
- CogFoundry recharge and balance: `https://console.cogfoundry.ai/credits`
- For a custom Server, use only service-returned guidance.

When ShengSuanYun reports insufficient balance, convey:

```text
The current ShengSuanYun account has insufficient balance. Recharge in the ShengSuanYun console before continuing:
https://console.shengsuanyun.com/user/recharge
```

When CogFoundry reports insufficient balance, convey:

```text
The current CogFoundry account has insufficient balance. Recharge in the CogFoundry console before continuing:
https://console.cogfoundry.ai/credits
```
