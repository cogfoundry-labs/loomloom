# 🧵 LoomLoom — Repeatable AI Workflows

**Work is not made of tasks. It is made of loops.**

```text
plan → execute → verify → improve
```

[![Ask DeepWiki](https://deepwiki.com/badge.svg)](https://deepwiki.com/Cogfoundry-ai/loomloom)
![Status: Beta](https://img.shields.io/badge/status-beta-orange)
[![License: Apache 2.0](https://img.shields.io/badge/license-Apache%202.0-blue)](LICENSE)

LoomLoom is an open-source CLI and runtime for **repeatable AI workflows**. 

Simply describe the outcome you want. LoomLoom transforms it into an executable workflow that plans, executes, verifies, and improves results across structured datasets—including Excel, CSV, JSON, SQL, and more.

**No visual workflow builders. No orchestration code. Just scalable, observable, and reusable AI workflows.**

**Ideal for:**

- 📝 Batch content generation (text, images, and video)
- 📊 Data enrichment, transformation, and classification
- 📄 Document processing and knowledge extraction
- 💻 AI-assisted software engineering and code generation

[Quick Start](#quick-start) · [Installation](references/installation.md) · [Key Concepts](#key-concepts) · [CLI Reference](references/cli-reference.md) · [Template Docs](docs/template-spec/00-template-spec.md)

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

> For agent-assisted installation, version pinning, the Gitee mirror, credential configuration, and uninstall instructions, see **[Installation](references/installation.md)**.

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

More end-to-end walkthroughs are in **[Workflow Examples](references/workflow-examples.md)**.

---

## Why LoomLoom

Most AI tools optimize a single prompt. Most real work isn't a single prompt — it's a repeatable loop run across hundreds or thousands of records, with verification, retries, and refinement. LoomLoom is the runtime for those loops:

- **Natural language → executable workflows** — describe the outcome; LoomLoom plans, executes, verifies, and delivers.
- **Built for batch execution** — run one workflow across an entire dataset deterministically.
- **Excel-native** — read inputs from workbooks and write results back into the same files.
- **Agent-native** — orchestrates Codex, Claude Code, and OpenClaw, with human confirmation before paid or irreversible actions.
- **Reusable assets** — package workflows as templates, share them privately, or publish them as `SkillBots` on the Market.
- **Open by default** — run fully local, or connect to CogFoundry for cloud-scale execution.

---

## Key Concepts

| Term | Meaning |
|---|---|
| **Template** | A reusable AI workflow run repeatedly with different inputs. |
| **Official Template** | Platform-maintained workflow (e.g. `text-v1`), discovered via `loomloom template list`. |
| **Private Template** | Your own workflow authored with `TemplateSpec`; supports multiple immutable versions. |
| **SkillBot** | A public workflow created from an approved Private Template version, published on the Market. |
| **Run** | A single execution of a template against your data. |

Deeper documentation:

- [Official Templates](references/official-templates.md) — input schemas for each official workflow
- [Private Templates & Authoring](references/private-templates.md) — build your own with TemplateSpec ([formal spec](docs/template-spec/00-template-spec.md))
- [Market & SkillBots](references/market-skillbots.md) — publish, version, price, and run SkillBots
- [Complete CLI Reference](references/cli-reference.md)
- [Workflow Examples](references/workflow-examples.md)

---

## Supported Agents

LoomLoom separates workflow orchestration from agent execution. Installing LoomLoom installs the integration package for the selected agent.

| Agent | Status |
|---|---|
| Codex (OpenAI) | Supported |
| Claude Code (Anthropic) | Supported |
| OpenClaw | Supported |

---

## Market

The LoomLoom Market lets creators publish reusable workflows as **SkillBots**, and lets others discover and run them without rebuilding the workflow. CogFoundry payment and transaction capabilities are coming soon. For Market-related paid workflows, check availability, balance, and transaction status in the [ShengSuanYun Console](https://console.shengsuanyun.com/user/recharge). See **[Market & SkillBots](references/market-skillbots.md)** for publishing, versioning, and pricing.

---

## Security

- Only send `LOOMLOOM_TOKEN` to the `LOOMLOOM_SERVER` you configured, over HTTPS.
- Never put real tokens in source, docs, screenshots, or logs.
- AI agents must get explicit user confirmation before paid or state-changing operations (for example, `submit-file`, `run submit`, `template-spec run`, `market run`, `listing publish`, or `listing unlist`).
- Do not blindly retry paid or state-changing commands after an ambiguous failure; check the relevant run, listing, review, or usage state first.

Full guidance: **[Security Notes](references/security.md)**. LoomLoom is **beta** — breaking changes are possible before the first stable release.

---

## Contributing

Contributions are welcome — browse [open issues](https://github.com/Cogfoundry-ai/loomloom/issues) or open a pull request. A `CONTRIBUTING.md` with detailed guidelines will be added before the first stable release.

Having trouble? See **[Troubleshooting & FAQ](references/troubleshooting.md)**.

## License

Licensed under the [Apache License 2.0](LICENSE).

## Links

- **GitHub**: [github.com/Cogfoundry-ai/loomloom](https://github.com/Cogfoundry-ai/loomloom)
- **API Base URL**: `https://loomloom.shengsuanyun.com/loom/v1`
- **ShengSuanYun API Keys**: https://console.shengsuanyun.com/user/keys
- **ShengSuanYun Recharge**: https://console.shengsuanyun.com/user/recharge
- **CogFoundry**: https://cogfoundry.ai

<p align="center">Made with ❤️ by <a href="https://cogfoundry.ai">CogFoundry</a></p>
