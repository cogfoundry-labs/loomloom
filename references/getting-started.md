# Getting Started

## Contents

- [Key Concepts](#key-concepts)
- [Supported Agents](#supported-agents)
- [Agent-assisted install](#agent-assisted-install)
- [Install from script (macOS / Linux)](#install-from-script-macos--linux)
- [Install from script (Windows / PowerShell)](#install-from-script-windows--powershell)
- [Configure credentials](#configure-credentials)
- [Uninstall](#uninstall)

## Key Concepts

| Term | Meaning |
|---|---|
| **Template** | A reusable AI workflow run repeatedly with different inputs. |
| **Official Template** | Platform-maintained workflow (e.g. `text-v1`), discovered via `loomloom template list`. |
| **Private Template** | Your own workflow authored with `TemplateSpec`; supports multiple immutable versions. |
| **SkillBot** | A public workflow created from an approved Private Template version, published on the Market. |
| **Run** | A single execution of a template against your data. |

Deeper documentation:

- [Official Templates](official-templates.md) — input schemas for each official workflow
- [Private Templates & Authoring](private-templates.md) — build your own with TemplateSpec ([English handbook](../docs/template-spec/en/README.md), [中文手册](../docs/template-spec/zh-CN/README.md))
- [Market & SkillBots](market-skillbots.md) — publish, version, price, and run SkillBots
- [Complete CLI Reference](cli-reference.md)
- [Workflow Examples](workflow-examples.md)

---

## Supported Agents

LoomLoom separates workflow orchestration from agent execution. Installing LoomLoom installs the integration package for the selected agent.

| Agent | Status |
|---|---|
| Codex (OpenAI) | Supported |
| Claude Code (Anthropic) | Supported |
| OpenClaw | Supported |

---

## Quick Start

### 1. Install

**macOS / Linux**

```bash
curl -fsSL https://raw.githubusercontent.com/Cogfoundry-ai/loomloom/main/install.sh | bash
```

**Windows (PowerShell)**

```powershell
irm https://raw.githubusercontent.com/Cogfoundry-ai/loomloom/main/install.ps1 | iex
```

> For agent-assisted installation, version pinning, the Gitee mirror, credential configuration, and uninstall instructions, see the sections below.

### 2. Configure credentials

If `LOOMLOOM_TOKEN` is not configured yet, choose the platform you want to use:

1. **ShengSuanYun**
   Recommended for users in Mainland China. This service is jointly supported by CogFoundry. You can create an API key and recharge your account in the ShengSuanYun Console.

   - API keys: https://console.shengsuanyun.com/user/keys
   - Recharge: https://console.shengsuanyun.com/user/recharge

2. **CogFoundry**
   Recommended for users in Singapore and other overseas regions. CogFoundry payment and transaction capabilities are coming soon.

   Until CogFoundry billing is available, you can use the ShengSuanYun Console to create an API key and recharge your account:

   - API keys: https://console.shengsuanyun.com/user/keys
   - Recharge: https://console.shengsuanyun.com/user/recharge

```bash
export LOOMLOOM_SERVER="https://loomloom.shengsuanyun.com/loom/v1"
export LOOMLOOM_TOKEN="<your LoomLoom API key>"
```

On Windows PowerShell, use `$env:LOOMLOOM_SERVER = "..."`. Never commit or share your API key.

### 3. Verify

```bash
loomloom doctor
```

### 4. Inspect commands and inputs

- Use `loomloom <command> --help` to inspect positional arguments and flags.
- Use `loomloom template schema <template-id> --output json` to inspect official template fields.
- Use `loomloom market show <listing-id> --output json` to inspect a Market SkillBot's public input schema.
- Use `loomloom template-spec docs spec|examples|conversation` when authoring private templates.
- Use `--output json` when chaining commands, and preserve returned IDs exactly.

### 5. Run your first workflow

```bash
# Download a sample workbook for an official template
loomloom template download text-image-v1 --output-file ./task.xlsx

# Fill in the workbook, then validate and estimate cost
loomloom template validate-file text-image-v1 ./task.xlsx
loomloom template precheck-file text-image-v1 ./task.xlsx

# Review the estimate and confirm before submitting
loomloom template submit-file text-image-v1 ./task.xlsx --client-request-id <stable-id>

# Watch progress, then download results and artifacts
loomloom run watch <run-id>
loomloom run result-workbook <run-id> --output-file ./task.result.xlsx
loomloom artifact download <run-id> --output-dir ./downloads
```

More end-to-end walkthroughs are in **[Workflow Examples](workflow-examples.md)**.

---

## Agent-assisted install

If you already use Codex, Claude Code, or OpenClaw, you can have the agent install and configure LoomLoom for you. Send it this prompt (replace `your-api-key` with an API key from the [ShengSuanYun API Keys](https://console.shengsuanyun.com/user/keys) page):

```
Install LoomLoom from the official repository:
https://github.com/cogfoundry-labs/loomloom

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

## Market

The LoomLoom Market lets creators publish reusable workflows as **SkillBots**, and lets others discover and run them without rebuilding the workflow. CogFoundry payment and transaction capabilities are coming soon. For Market-related paid workflows, check availability, balance, and transaction status in the [ShengSuanYun Console](https://console.shengsuanyun.com/user/recharge). See **[Market & SkillBots](market-skillbots.md)** for publishing, versioning, and pricing.

---

## Security

- Only send `LOOMLOOM_TOKEN` to the `LOOMLOOM_SERVER` you configured, over HTTPS.
- Never put real tokens in source, docs, screenshots, or logs.
- AI agents must get explicit user confirmation before paid or state-changing operations (for example, `submit-file`, `run submit`, `template-spec run`, `market run`, `listing publish`, or `listing unlist`).
- Do not blindly retry paid or state-changing commands after an ambiguous failure; check the relevant run, listing, review, or usage state first.

Full guidance: **[Security Notes](security.md)**. LoomLoom is **beta** — breaking changes are possible before the first stable release.

---

## Links

- **API Base URL**: https://loomloom.shengsuanyun.com/loom/v1
- **API Keys**: https://console.shengsuanyun.com/user/keys
- **Recharge**: https://console.shengsuanyun.com/user/recharge
- **CogFoundry**: https://cogfoundry.ai
