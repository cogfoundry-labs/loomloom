# Troubleshooting & FAQ

Common questions and fixes. For authentication, server, network, or service-version problems, run `loomloom doctor --output json`. Correct local flag, file, JSON, workbook, and schema errors directly.

## Where do I get a token?

If no server profile is selected, choose the platform you want to use:

1. **ShengSuanYun**
   Recommended for users in Mainland China. This service is jointly supported by CogFoundry. Create an API key and recharge your account in the ShengSuanYun Console.

   - API keys: <https://console.shengsuanyun.com/user/keys>
   - Recharge: <https://console.shengsuanyun.com/user/recharge>

2. **CogFoundry**
   Recommended for users in Singapore and other countries or regions.

   - API keys: <https://console.cogfoundry.ai/api-keys>
   - Credits and balance: <https://console.cogfoundry.ai/credits>

For either preset platform, prefer `loomloom login --server <selected-server-url>`. Use an API token when browser login is unavailable or when you explicitly prefer token authentication. A token is only valid for the exact server for which it was issued.

## Where can I check run status?

Use the CLI polling commands, such as `loomloom run get <run-id>` or `loomloom run watch <run-id>`. Run details are not exposed via a stable or predictable URL format — do not construct run-detail links manually.

## `loomloom template list` returns empty results — why?

This usually means your workspace has no published or visible templates. Check with CogFoundry customer support to confirm template availability and permissions.

## Can I use the CLI without an agent?

Yes. All workflows can be executed directly via CLI commands. AI agent integrations in your development environment are optional and not required for execution, since the CogFoundry server already provides built-in agent execution support.

## Something is broken. What should I do?

For authentication, server, network, or service-version failures, run `loomloom doctor --output json`, redact private data, then open an issue at [github.com/cogfoundry-labs/loomloom/issues](https://github.com/cogfoundry-labs/loomloom/issues) with the relevant output attached.
