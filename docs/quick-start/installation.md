# Install loomloom CLI

The main product of this project is the loomloom CLI. It is the developer interface for defining, compiling, executing, and managing AI work as software. Its command-line interface is documented in [CLI reference](../reference/cli.md).

The following sections describe how to install, configure, and uninstall loomloom CLI on your local development machine using different options.

**Install:**
[macOS / Linux](#macos--linux) · [Windows (PowerShell)](#windows-powershell) · [Agent-assisted setup](#agent-assisted-setup)

**Configure:**
[Manual setup](#manual-setup) · [Manual verification](#manual-verification)

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
  # Install a specific version
  curl -fsSL https://raw.githubusercontent.com/cogfoundry-labs/loomloom/main/install.sh | bash -s -- --version v0.1.0-beta.1

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
  # Install a specific version
  & ([scriptblock]::Create((irm https://raw.githubusercontent.com/cogfoundry-labs/loomloom/main/install.ps1))) -Version v0.1.0-beta.1

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

Copy the following prompt, replace `YOUR_API_KEY` with your [CogFoundry API key](https://console.cogfoundry.ai/api-keys), and send it to your AI agent:

```text
Install loomloom from:
https://github.com/cogfoundry-labs/loomloom

Configure loomloom:
Server: https://loomloom.cogfoundry.ai/loom/v1
API Key: YOUR_API_KEY

After installation, run:
loomloom doctor
```

Notes:

- Recommended for most users. If you use this approach, you can skip the manual setup and verification sections below.

## Configure

### Manual setup

To get the loomloom CLI working, you need to connect it to a loomloom server with a valid API key.

A loomloom server is an endpoint provided by a loomloom execution platform that provides a managed runtime for compiled AI systems. Choose a compatible platform, obtain an API key after registration, and replace `YOUR_LOOMLOOM_SERVER` and `YOUR_API_KEY` with the corresponding values.

| Execution platform | loomloom server | API key | Notes |
|:---|:---|:---|:---|
| <img src="../../assets/images/logo/logo-cogfoundry-light.svg" width="24" align="center" /> **CogFoundry** | `https://loomloom.cogfoundry.ai/loom/v1` | [Get API key](https://console.cogfoundry.ai/api-keys) | Official reference execution platform. |
| <img src="../../assets/images/logo/logo-shengsuanyun-light.svg" width="24" align="center" /> **ShengSuanYun** | `https://loomloom.shengsuanyun.com/loom/v1` | [Get API key](https://console.shengsuanyun.com/user/keys) | Managed platform deployed in mainland China. |

Configure the environment variables:

- macOS / Linux:

  ```bash
  export LOOMLOOM_SERVER="YOUR_LOOMLOOM_SERVER"
  export LOOMLOOM_TOKEN="YOUR_API_KEY"
  ```

- Windows (PowerShell):

  ```powershell
  $env:LOOMLOOM_SERVER = "YOUR_LOOMLOOM_SERVER"
  $env:LOOMLOOM_TOKEN = "YOUR_API_KEY"
  ```

- To persist these settings, add them to your `~/.zshrc`, `~/.bashrc`, or shell profile on macOS/Linux, or your `$PROFILE` on PowerShell.
- If you do not have an API key yet, choose a loomloom execution platform above and register for access.

### Manual verification

Run the following command to verify that your loomloom installation and configuration are correct:

```bash
loomloom doctor
```

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
