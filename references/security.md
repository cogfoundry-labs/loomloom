# Security Notes

LoomLoom sends your data and credentials to a remote server for execution. Follow these practices to keep them safe.

- **Protect your access token.** Only send `LOOMLOOM_TOKEN` to the `LOOMLOOM_SERVER` you explicitly configured (or the server specified with `--server`), and always use HTTPS.
- **Verify the server endpoint.** The default production server is `https://loomloom.shengsuanyun.com/loom/v1`. Never send your token to an unexpected host, and do not follow redirects that change the destination.
- **Never expose credentials.** Do not include real access tokens in source code, documentation, screenshots, logs, or public conversations.
- **Handle workbook data carefully.** Workbook contents are transmitted as Base64-encoded data within JSON requests. Avoid logging or printing complete request payloads. `accessUrl` values returned by the server are temporary signed URLs and should not be stored in long-term logs or shared publicly.
- **Require explicit user confirmation.** AI agents should always obtain explicit user approval before performing any paid operation or any action that modifies remote state, such as `submit-file`, `run submit`, `template-spec run`, `market run`, `listing publish`, or `listing unlist`.
- **Retry carefully.** Do not blindly retry paid or remote-state-changing commands after an ambiguous failure. Check the relevant run, listing, review, or usage state first. Reuse a `--client-request-id` only for an identical payload; use a new ID when the payload changes.

> **Beta notice:** LoomLoom is in beta. CLI commands, workflow specifications, template APIs, Market integration, and plugin interfaces may change before the first stable release. Backward compatibility is maintained whenever practical, but breaking changes may occur.
