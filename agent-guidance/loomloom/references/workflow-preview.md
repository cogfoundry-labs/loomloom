# Workflow viewing

For local draft edits, start `loomloom template-spec preview <spec.json> --output json`
in a retained tool process. The file can be missing initially. Send the `ready`
event's complete `viewUrl` to the user before editing; keep the process running
so they see subsequent changes. This link works only on the CLI computer.

The CLI embeds the workbench viewer and serves it on loopback. No browser login,
Redis session, draft upload, template save, publication or model execution occurs.
Only contract binding metadata is read from the configured API. Unavailable
contracts show unresolved ports; do not claim formal validation succeeded.

Write normal UTF-8 JSON up to 1 MiB, preferably using atomic file replacement.
The viewer supports incomplete JSON and uses progressive rendering for later
changes. NDJSON events are ready, synced, waiting and ended. Synced means local
snapshot updated, not that a private version was saved. Do not print file content
or remote credentials to diagnose errors. The local URL itself grants local access.

Finish by stopping only the retained preview process. Closing a browser tab does
not stop it; after CLI exits the page clears and the link no longer works.
Restart generates a new port/token. Do not send local links as cross-device links.
No heartbeat, writer secret, source-template-id or source-version-id flags remain.

When the user asks to save, follow the normal validation and persistence policy.
`create` / `create-version` return the exact online version `viewUrl` when available.
A link error does not undo a successful save. Retry only
`loomloom template-spec view-link <template-id> <version-id>` with the actual IDs.
Saved-version links require the owner's website login and work without CLI running.

If no preview command exists in the installed binary, explain that the CLI must
be upgraded; never claim a source-only edit updates the installed binary.
