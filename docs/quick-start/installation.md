# Install loomloom CLI and Agent Skill

The main product of this project is the loomloom CLI. It is the developer interface for defining, compiling, executing, and managing AI work as software. Its command-line interface is documented in [CLI reference](../reference/cli.md). The installation scripts install both the CLI and the bundled LoomLoom Agent Skill.

The LoomLoom Agent Skill has one distribution path, `skills/loomloom`. The installer does not detect or select an Agent. For manual installation, determine your Agent's supported Skill root and pass the complete LoomLoom Skill destination, ending in `loomloom`, through `--skill-dir` or `-SkillDir`. If you do not know that location, use [Agent-assisted setup](#agent-assisted-setup).

`--skill-dir` selects the Agent that will see the Skill; it is not the CLI
binary directory and it is not a cross-Agent default. Never use a Codex path
as a fallback when installing for another Agent. For example:

| Target Agent | Complete destination |
| --- | --- |
| Codex | `<your Codex Skill root>/loomloom` |
| WorkBuddy | `~/.workbuddy/skills/loomloom` |

If the target Agent's Skill root is unknown, stop and ask the user or inspect
that Agent's configuration. After installation, verify that the exact selected
path contains `SKILL.md`.

The following sections describe how to install, configure, and uninstall loomloom on your local development machine using different options.

**Install:**
[macOS / Linux](#macos--linux) · [Windows (PowerShell)](#windows-powershell) · [Agent-assisted setup](#agent-assisted-setup)

**Configure:**
[Browser login](#browser-login) · [API token fallback](#api-token-fallback) · [Verification and server profiles](#verification-and-server-profiles)

**Uninstall:**
[macOS / Linux](#macos--linux-uninstallation) · [Windows (PowerShell)](#windows-powershell-uninstallation)

## Install

### macOS / Linux

Set the complete LoomLoom Skill destination for your Agent, then install the latest release using `curl`. Replace the example path before running the command:

```bash
LOOMLOOM_SKILL_DIR="/absolute/path/to/your/agent/skills/loomloom"
curl -fsSL https://raw.githubusercontent.com/cogfoundry-labs/loomloom/main/scripts/install.sh | bash -s -- --skill-dir "$LOOMLOOM_SKILL_DIR"
```

Notes:

- `--skill-dir` must be the complete destination for the LoomLoom Skill, not the parent Skills directory.

- For WorkBuddy, use `--skill-dir "$HOME/.workbuddy/skills/loomloom"`; do not use `~/.codex/skills/loomloom` unless Codex is the target Agent.

- To install a specific version or release channel, use the `--version` or `--channel` option:
  ```bash
  # Install a specific release tag
  curl -fsSL https://raw.githubusercontent.com/cogfoundry-labs/loomloom/main/scripts/install.sh | bash -s -- --skill-dir "$LOOMLOOM_SKILL_DIR" --version vX.Y.Z

  # Install the latest beta release
  curl -fsSL https://raw.githubusercontent.com/cogfoundry-labs/loomloom/main/scripts/install.sh | bash -s -- --skill-dir "$LOOMLOOM_SKILL_DIR" --channel beta
  ```

- To install from a GitLab or Gitee mirror:

  ```bash
  # GitLab
  curl -fsSL https://gitlab.com/cogfoundry/loomloom/raw/main/scripts/install-gitee.sh | bash -s -- --skill-dir "$LOOMLOOM_SKILL_DIR"

  # Gitee
  curl -fsSL https://gitee.com/cogfoundry/loomloom/raw/main/scripts/install-gitee.sh | bash -s -- --skill-dir "$LOOMLOOM_SKILL_DIR"
  ```

### Windows (PowerShell)

Set the complete LoomLoom Skill destination for your Agent, then install the latest release using `irm`. Replace the example path before running the command:

```powershell
$LoomLoomSkillDir = "C:\path\to\your\agent\skills\loomloom"
& ([scriptblock]::Create((irm https://raw.githubusercontent.com/cogfoundry-labs/loomloom/main/scripts/install.ps1))) -SkillDir $LoomLoomSkillDir
```

Notes:

- `-SkillDir` must be the complete destination for the LoomLoom Skill, not the parent Skills directory.

- To install a specific version or release channel, use the `-Version` or `-Channel` option:
  ```powershell
  # Install a specific release tag
  & ([scriptblock]::Create((irm https://raw.githubusercontent.com/cogfoundry-labs/loomloom/main/scripts/install.ps1))) -SkillDir $LoomLoomSkillDir -Version vX.Y.Z

  # Install the latest beta release
  & ([scriptblock]::Create((irm https://raw.githubusercontent.com/cogfoundry-labs/loomloom/main/scripts/install.ps1))) -SkillDir $LoomLoomSkillDir -Channel beta
  ```

- To install from a GitLab or Gitee mirror:

  ```powershell
  # GitLab
  & ([scriptblock]::Create((irm https://gitlab.com/cogfoundry/loomloom/raw/main/scripts/install.ps1))) -SkillDir $LoomLoomSkillDir -Source gitee

  # Gitee
  & ([scriptblock]::Create((irm https://gitee.com/cogfoundry/loomloom/raw/main/scripts/install.ps1))) -SkillDir $LoomLoomSkillDir -Source gitee
  ```

### Agent-assisted setup

If you use an AI agent that supports Skills, you can ask it to install and configure loomloom for you. The Agent determines its supported Skill root and passes the complete destination to the installer.

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

The following commands uninstall the loomloom CLI and/or the bundled LoomLoom Agent Skill from your local machine. Complete and Skill-only uninstallations require the same complete Skill destination used during installation.

### macOS / Linux uninstallation

Uninstall using `curl`:

```bash
LOOMLOOM_SKILL_DIR="/absolute/path/to/your/agent/skills/loomloom"

# Uninstall loomloom CLI and the LoomLoom Agent Skill
curl -fsSL https://raw.githubusercontent.com/cogfoundry-labs/loomloom/main/scripts/uninstall.sh | bash -s -- --skill-dir "$LOOMLOOM_SKILL_DIR"

# Uninstall loomloom CLI only
curl -fsSL https://raw.githubusercontent.com/cogfoundry-labs/loomloom/main/scripts/uninstall.sh | bash -s -- --cli-only

# Uninstall the LoomLoom Agent Skill only
curl -fsSL https://raw.githubusercontent.com/cogfoundry-labs/loomloom/main/scripts/uninstall.sh | bash -s -- --skill-only --skill-dir "$LOOMLOOM_SKILL_DIR"
```

### Windows (PowerShell) uninstallation

Uninstall using `irm`:

```powershell
$LoomLoomSkillDir = "C:\path\to\your\agent\skills\loomloom"

# Uninstall loomloom CLI and the LoomLoom Agent Skill
& ([scriptblock]::Create((irm https://raw.githubusercontent.com/cogfoundry-labs/loomloom/main/scripts/uninstall.ps1))) -SkillDir $LoomLoomSkillDir

# Uninstall loomloom CLI only
& ([scriptblock]::Create((irm https://raw.githubusercontent.com/cogfoundry-labs/loomloom/main/scripts/uninstall.ps1))) -CliOnly

# Uninstall the LoomLoom Agent Skill only
& ([scriptblock]::Create((irm https://raw.githubusercontent.com/cogfoundry-labs/loomloom/main/scripts/uninstall.ps1))) -SkillOnly -SkillDir $LoomLoomSkillDir
```

Notes:

- Uninstalling loomloom CLI does not remove your shell configuration or environment variables. Remove them manually if they are no longer needed.
