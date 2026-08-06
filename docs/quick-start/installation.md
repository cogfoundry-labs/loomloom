# Install loomloom CLI

The main product of this project is the loomloom CLI. It is the developer interface for defining, compiling, executing, and managing AI work as software. Its command-line interface is documented in [CLI reference](../reference/cli.md).

The following sections describe how to install, configure, and uninstall loomloom CLI on your local development machine using different options.

**Install:**
[macOS / Linux](#macos--linux) · [Windows (PowerShell)](#windows-powershell) · [Agent-assisted setup](#agent-assisted-setup)

**Configure:**
[Browser login](#browser-login) · [API token fallback](#api-token-fallback) · [Verification and server profiles](#verification-and-server-profiles)

**Uninstall:**
[macOS / Linux](#macos--linux-uninstallation) · [Windows (PowerShell)](#windows-powershell-uninstallation)

## Install

### macOS / Linux

Install the latest loomloom CLI release using `curl`:

```bash
curl -fsSL https://raw.githubusercontent.com/cogfoundry-labs/loomloom/main/install.sh | bash
```

Notes:

- To install loomloom CLI for a specific AI agent, add the `--agent` option:
  ```bash
  # Claude Code
  curl -fsSL https://raw.githubusercontent.com/cogfoundry-labs/loomloom/main/install.sh | bash -s -- --agent claude

  # OpenClaw
  curl -fsSL https://raw.githubusercontent.com/cogfoundry-labs/loomloom/main/install.sh | bash -s -- --agent openclaw
  ```

- To install a specific version or release channel, use the `--version` or `--channel` option:
  ```bash
  # Install a specific release tag
  curl -fsSL https://raw.githubusercontent.com/cogfoundry-labs/loomloom/main/install.sh | bash -s -- --version <release-tag>

  # Install the latest beta release
  curl -fsSL https://raw.githubusercontent.com/cogfoundry-labs/loomloom/main/install.sh | bash -s -- --channel beta
  ```

- To install from a GitLab or Gitee mirror:

  ```bash
  # GitLab
  curl -fsSL https://gitlab.com/cogfoundry/loomloom/raw/main/install-gitee.sh | bash

  # Gitee
  curl -fsSL https://gitee.com/cogfoundry/loomloom/raw/main/install-gitee.sh | bash
  ```

### Windows (PowerShell)

Install the latest loomloom CLI release using `irm`:

```powershell
irm https://raw.githubusercontent.com/cogfoundry-labs/loomloom/main/install.ps1 | iex
```

Notes:

- To install loomloom CLI for a specific AI agent, add the `-Agent` option:
  ```powershell
  # Claude Code
  & ([scriptblock]::Create((irm https://raw.githubusercontent.com/cogfoundry-labs/loomloom/main/install.ps1))) -Agent claude

  # OpenClaw
  & ([scriptblock]::Create((irm https://raw.githubusercontent.com/cogfoundry-labs/loomloom/main/install.ps1))) -Agent openclaw
  ```

- To install a specific version or release channel, use the `-Version` or `-Channel` option:
  ```powershell
  # Install a specific release tag
  & ([scriptblock]::Create((irm https://raw.githubusercontent.com/cogfoundry-labs/loomloom/main/install.ps1))) -Version <release-tag>

  # Install the latest beta release
  & ([scriptblock]::Create((irm https://raw.githubusercontent.com/cogfoundry-labs/loomloom/main/install.ps1))) -Channel beta
  ```

- To install from a GitLab or Gitee mirror:

  ```powershell
  # GitLab
  & ([scriptblock]::Create((irm https://gitlab.com/cogfoundry/loomloom/raw/main/install.ps1))) -Source gitee

  # Gitee
  & ([scriptblock]::Create((irm https://gitee.com/cogfoundry/loomloom/raw/main/install.ps1))) -Source gitee
  ```

### Agent-assisted setup

If you already use Codex, Claude Code, or OpenClaw, you can ask your AI agent to install and configure loomloom for you.

Copy the following prompt and send it to your AI agent:

```text
Install loomloom from:
https://github.com/cogfoundry-labs/loomloom

After installation, run:
loomloom doctor --output json

If no healthy server profile is configured, show me both preset platforms and let me choose one. Use browser login for the selected platform when possible, then run doctor again.
```

Notes:

- Recommended for most users. If you use this approach, you can skip the manual setup and verification sections below.

## Configure

### Browser login

To get the loomloom CLI working, connect it to a loomloom server with a valid credential. Browser login is the preferred setup method for the CogFoundry and ShengSuanYun presets.

A loomloom server is an endpoint provided by a loomloom execution platform that provides a managed runtime for compiled AI systems. In an interactive terminal, `loomloom login` lets you choose between the preset platforms. You can also pass the selected server explicitly:

```bash
loomloom login --server https://loomloom.shengsuanyun.com/loom/v1
loomloom login --server https://loomloom.cogfoundry.ai/loom/v1
```

The CLI verifies the returned credential before saving and activating the server profile. Use `--no-browser` to print the authorization URL, or `--login-timeout 10m` when more than the default five minutes is needed.

| Execution platform | loomloom server | API key fallback | Account and balance |
|:---|:---|:---|:---|
| <img src="../../assets/images/logo/logo-cogfoundry-light.svg" width="24" align="center" /> **CogFoundry** | `https://loomloom.cogfoundry.ai/loom/v1` | [API keys](https://console.cogfoundry.ai/api-keys) | [Credits](https://console.cogfoundry.ai/credits) |
| <img src="../../assets/images/logo/logo-shengsuanyun-light.svg" width="24" align="center" /> **ShengSuanYun** | `https://loomloom.shengsuanyun.com/loom/v1` | [API keys](https://console.shengsuanyun.com/user/keys) | [Recharge](https://console.shengsuanyun.com/user/recharge) |

### API token fallback

Use an API token for a custom server, a headless or CI environment, or when you explicitly prefer token authentication. Verify the exact server/token pair first:

```bash
loomloom doctor --server <exact-server-url> --token <api-token> --output json
```

On success, `doctor` registers and activates the verified server profile. If it returns `next_action: "persist_token"`, store the token in the exact profile-specific environment variable returned in `token_env`. Do not reuse a token with another server or modify a custom server URL.

### Verification and server profiles

Run the following command after either authentication flow:

```bash
loomloom doctor --output json
```

Continue when it reports `healthy: true`. Manage verified server profiles with:

```bash
loomloom server list
loomloom server use <name-or-server>
loomloom server remove <name-or-server>
```

`loomloom logout` removes the saved browser credential for the selected profile. It does not remove an environment API token or the profile itself.

## Uninstall

The following commands uninstall the loomloom CLI and/or installed SkillBots from your local machine.

### macOS / Linux uninstallation

Uninstall using `curl`:

```bash
# Uninstall loomloom CLI and SkillBots
curl -fsSL https://raw.githubusercontent.com/cogfoundry-labs/loomloom/main/uninstall.sh | bash

# Uninstall loomloom CLI only
curl -fsSL https://raw.githubusercontent.com/cogfoundry-labs/loomloom/main/uninstall.sh | bash -s -- --cli-only

# Uninstall SkillBots only
curl -fsSL https://raw.githubusercontent.com/cogfoundry-labs/loomloom/main/uninstall.sh | bash -s -- --skill-only
```

### Windows (PowerShell) uninstallation

Uninstall using `irm`:

```powershell
# Uninstall loomloom CLI and SkillBots
irm https://raw.githubusercontent.com/cogfoundry-labs/loomloom/main/uninstall.ps1 | iex

# Uninstall loomloom CLI only
& ([scriptblock]::Create((irm https://raw.githubusercontent.com/cogfoundry-labs/loomloom/main/uninstall.ps1))) -CliOnly

# Uninstall SkillBots only
& ([scriptblock]::Create((irm https://raw.githubusercontent.com/cogfoundry-labs/loomloom/main/uninstall.ps1))) -SkillOnly
```

Notes:

- Uninstalling loomloom CLI does not remove your shell configuration or environment variables. Remove them manually if they are no longer needed.
