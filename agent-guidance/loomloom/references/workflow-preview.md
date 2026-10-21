# Workflow viewing

Configure the deployment's real HTTPS bootstrap with
`LOOMLOOM_VIEWER_BOOTSTRAP_URL` or `preview --viewer-url`. Do not invent a hosted
address. For local edits, start `loomloom template-spec preview <spec.json>
--no-open --output json` in a retained tool process. Give the user the complete
ready.viewUrl and keep the process alive while editing. It works on that computer.

The CLI supplies only a tiny local startup page and a protected JSON snapshot.
The browser loads public Workbench JS/CSS directly, then renders locally. The
CLI never downloads or embeds the UI. Draft content, node information and local
credentials are not sent to any remote API. No cloud token, login, contract query,
Redis, upload, save or model execution occurs. Missing model metadata stays
unverified; a diagram does not mean the workflow passed validation.

Use a regular UTF-8 file up to 1 MiB. Missing/incomplete files produce explicit
waiting or parse states and recover on edits. NDJSON ready, synced, waiting and
ended describe local preview only. Do not log draft text or the private local
URL to telemetry. Closing a tab does not stop CLI; stop only this retained
process when finished. The page detects disconnection, but do not promise that
previously received browser data is erased. Refresh works with the original
fragment while the CLI is alive; restarting needs the new link.

If public UI assets are unavailable, report the loading error; do not switch to
an embedded resource package, cloud upload or automatic execution. No published
viewer URL or fully offline operation is assumed.

For saved versions, normal confirmation/validation/persistence rules still
apply. Use `loomloom template-spec view-link <template-id> <version-id>` with
exact IDs; its online link requires the owner to log in and is independent of
local preview. A view-link failure never undoes a successful save; retry only
the read, never the save mutation.
