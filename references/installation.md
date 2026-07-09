# Installation & Uninstall

Full installation options for LoomLoom. For the fastest path, see [Quick Start](../README.md#quick-start) in the README — this document covers agent-assisted install, per-agent packages, version pinning, the Gitee mirror, credential setup, and uninstall.

## Contents

- [Agent-assisted install](#agent-assisted-install)
- [Install from script (macOS / Linux)](#install-from-script-macos--linux)
- [Install from script (Windows / PowerShell)](#install-from-script-windows--powershell)
- [Configure credentials](#configure-credentials)
- [Uninstall](#uninstall)

---

## Agent-assisted install

If you already use Codex, Claude Code, or OpenClaw, you can have the agent install and configure LoomLoom for you. Send it this prompt (replace `your-api-key` with an API key from the [ShengSuanYun API Keys](https://console.shengsuanyun.com/user/keys) page):

```
Install LoomLoom from the official repository:
https://github.com/Cogfoundry-ai/loomloom

Configure the runtime with the following settings:
Server URL: https://loomloom.shengsuanyun.com/loom/v1
API key: your-api-key

After installation and configuration, run the doctor command to verify that your environment is correctly configured.
```

---

## Install from script (macOS / Linux)

### Default installation

```bash
curl -fsSL https://raw.githubusercontent.com/Cogfoundry-ai/loomloom/main/install.sh | bash
```

### Install for a specific agent

```bash
# Claude Code
curl -fsSL https://raw.githubusercontent.com/Cogfoundry-ai/loomloom/main/install.sh | bash -s -- --agent claude

# OpenClaw
curl -fsSL https://raw.githubusercontent.com/Cogfoundry-ai/loomloom/main/install.sh | bash -s -- --agent openclaw
```

### Install a specific version or release channel

```bash
# Install a specified version
curl -fsSL https://raw.githubusercontent.com/Cogfoundry-ai/loomloom/main/install.sh | bash -s -- --version v0.1.0-beta.1

# Install the latest beta release
curl -fsSL https://raw.githubusercontent.com/Cogfoundry-ai/loomloom/main/install.sh | bash -s -- --channel beta
```

### Install from the Gitee mirror

```bash
curl -fsSL https://gitee.com/cogfoundry/loomloom/raw/main/install-gitee.sh | bash
```

---

## Install from script (Windows / PowerShell)

### Default installation

```powershell
irm https://raw.githubusercontent.com/Cogfoundry-ai/loomloom/main/install.ps1 | iex
```

### Install for a specific agent

```powershell
# Claude Code
& ([scriptblock]::Create((irm https://raw.githubusercontent.com/Cogfoundry-ai/loomloom/main/install.ps1))) -Agent claude

# OpenClaw
& ([scriptblock]::Create((irm https://raw.githubusercontent.com/Cogfoundry-ai/loomloom/main/install.ps1))) -Agent openclaw
```

### Install a specific version or release channel

```powershell
# Install a specific version
& ([scriptblock]::Create((irm https://raw.githubusercontent.com/Cogfoundry-ai/loomloom/main/install.ps1))) -Version v0.1.0-beta.1

# Install the latest beta release
& ([scriptblock]::Create((irm https://raw.githubusercontent.com/Cogfoundry-ai/loomloom/main/install.ps1))) -Channel beta
```

### Install from the Gitee mirror

```powershell
& ([scriptblock]::Create((irm https://gitee.com/cogfoundry/loomloom/raw/main/install.ps1))) -Source gitee
```

> **Notes**
>
> - By default, the installer downloads the latest stable release.
> - On macOS and Linux, Homebrew is used automatically when available.
> - Use the Gitee mirror only if you explicitly want to install from CogFoundry's mirrored release assets (for example, if GitHub is inaccessible or slow in your region).

---

## Configure credentials

```bash
export LOOMLOOM_SERVER="https://loomloom.shengsuanyun.com/loom/v1"
export LOOMLOOM_TOKEN="<your LoomLoom API key>"
```

On Windows PowerShell, set them with `$env:` instead:

```powershell
$env:LOOMLOOM_SERVER = "https://loomloom.shengsuanyun.com/loom/v1"
$env:LOOMLOOM_TOKEN  = "<your LoomLoom API key>"
```

Add these to `~/.zshrc`, `~/.bashrc`, or your shell profile (or your PowerShell `$PROFILE`) to persist them. If you do not have an API key yet, choose a platform:

1. **ShengSuanYun**
   Recommended for users in Mainland China. This service is jointly supported by CogFoundry. Create an API key and recharge your account in the ShengSuanYun Console.

   - API keys: https://console.shengsuanyun.com/user/keys
   - Recharge: https://console.shengsuanyun.com/user/recharge

2. **CogFoundry**
   Recommended for users in Singapore and other overseas regions. CogFoundry payment and transaction capabilities are coming soon.

   Until CogFoundry billing is available, use the ShengSuanYun Console to create an API key and recharge your account:

   - API keys: https://console.shengsuanyun.com/user/keys
   - Recharge: https://console.shengsuanyun.com/user/recharge

Never commit or share your API key. Legacy `BATCHJOB_SERVER` / `BATCHJOB_TOKEN` variables still work, but new setups should use `LOOMLOOM_*`.

Verify the setup:

```bash
loomloom doctor
```

---

## Uninstall

### macOS / Linux

```bash
# Remove CLI and skill package
curl -fsSL https://raw.githubusercontent.com/Cogfoundry-ai/loomloom/main/uninstall.sh | bash

# Remove CLI only
curl -fsSL https://raw.githubusercontent.com/Cogfoundry-ai/loomloom/main/uninstall.sh | bash -s -- --cli-only

# Remove skill package only
curl -fsSL https://raw.githubusercontent.com/Cogfoundry-ai/loomloom/main/uninstall.sh | bash -s -- --skill-only
```

### Windows (PowerShell)

```powershell
# Remove CLI and skill package
irm https://raw.githubusercontent.com/Cogfoundry-ai/loomloom/main/uninstall.ps1 | iex

# Remove CLI only
& ([scriptblock]::Create((irm https://raw.githubusercontent.com/Cogfoundry-ai/loomloom/main/uninstall.ps1))) -CliOnly

# Remove skill package only
& ([scriptblock]::Create((irm https://raw.githubusercontent.com/Cogfoundry-ai/loomloom/main/uninstall.ps1))) -SkillOnly
```
