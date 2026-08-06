# Create your first reusable AI work IR

AI work can be created by developers, AI agents, or both working together. It can range from a simple prompt to a complete AI system.

Using the loomloom CLI to compile AI work into **templates** (reusable AI work IR) is straightforward and requires little or no coding.

## Create your first template

Once loomloom CLI is [installed](../quick-start/installation.md), tell your AI agent what you want to build — it will guide you through the rest. A typical workflow looks like this:

1. **Describe your goal.** Tell the agent what you want to automate or build.
2. **Refine the requirements.** The agent asks follow-up questions only when needed to understand your AI work, inputs, outputs, and workflow.
3. **Review the proposed template.** The agent generates a reusable template, recommends an execution strategy, and may suggest alternative designs with estimated execution costs.
4. **Run the template.** Provide your inputs (for example, an Excel workbook), execute the template, and review the results.
5. **Iterate.** Ask the agent to refine any part of the template until you are satisfied.
6. **Publish (optional).** Publish the template as a SkillBot so others can use it through the loomloom Marketplace.

The workflow is fully collaborative — you can ask questions, change requirements, compare different approaches, or let the agent make recommendations at any stage.

## Use official templates

Alternatively, you can start by exploring official templates created, approved, and published by CogFoundry. A typical workflow looks like this:

1. **Choose a template.** Tell your AI agent that you want to explore available official templates.
2. **Configure the template.** The agent explains how the template works, what inputs are required, and guides you through the setup.
3. **Run the template.** Provide your inputs (for example, an Excel workbook), execute the template, and review the results.

## Learn more

- [Understand your template](../reference/private-template.md)
- [Official templates](../reference/official-templates.md)
- [Build your first SkillBot](build-your-first-skillbot.md)
- [CLI reference](../reference/cli.md)

---

## Supported agents

loomloom separates workflow orchestration from agent execution. Installing loomloom installs the integration package for the selected agent.

| Agent | Status |
|---|---|
| Codex (OpenAI) | Supported |
| Claude Code (Anthropic) | Supported |
| OpenClaw | Supported |

## Security

- Only send `LOOMLOOM_TOKEN` to the `LOOMLOOM_SERVER` you configured, over HTTPS.
- Never put real tokens in source, docs, screenshots, or logs.
- AI agents must get explicit user confirmation before paid or state-changing operations (for example, `submit-file`, `run submit`, `template-spec run`, `market run`, `listing publish`, or `listing unlist`).
- Do not blindly retry paid or state-changing commands after an ambiguous failure; check the relevant run, listing, review, or usage state first.

Full guidance: **[Security Notes](../../SECURITY.md)**. loomloom is **beta** — breaking changes are possible before the first stable release.
