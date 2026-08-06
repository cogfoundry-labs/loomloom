# Troubleshooting & FAQ

Common questions and fixes. If something isn't working, always run `loomloom doctor` first — it checks your server URL, token, and environment.

## Where do I get a token?

If `LOOMLOOM_TOKEN` is not configured yet, choose the platform you want to use:

1. **ShengSuanYun**
   Recommended for users in Mainland China. This service is jointly supported by CogFoundry. Create an API key and recharge your account in the ShengSuanYun Console.

   - API keys: <https://console.shengsuanyun.com/user/keys>
   - Recharge: <https://console.shengsuanyun.com/user/recharge>

2. **CogFoundry**
   Recommended for users in Singapore and other overseas regions. CogFoundry payment and transaction capabilities are coming soon.

   Until CogFoundry billing is available, use the ShengSuanYun Console to create an API key and recharge your account:

   - API keys: <https://console.shengsuanyun.com/user/keys>
   - Recharge: <https://console.shengsuanyun.com/user/recharge>

The API key is only valid for the `LOOMLOOM_SERVER` you explicitly configured.

## Where can I check run status?

Use the CLI polling commands, such as `loomloom run get <run-id>` or `loomloom run watch <run-id>`. Run details are not exposed via a stable or predictable URL format — do not construct run-detail links manually.

## `loomloom template list` returns empty results — why?

This usually means your workspace has no published or visible templates. Check with CogFoundry customer support to confirm template availability and permissions.

## Can I use the CLI without an agent?

Yes. All workflows can be executed directly via CLI commands. AI agent integrations in your development environment are optional and not required for execution, since the CogFoundry server already provides built-in agent execution support.

## Something is broken. What should I do?

Run `loomloom doctor` first, then open an issue at [github.com/cogfoundry-labs/loomloom/issues](https://github.com/cogfoundry-labs/loomloom/issues) with the doctor output attached.
