# Workflow viewing

## Viewer default and overrides

Preview defaults to the ShengSuanYun production viewer:

`https://www.shengsuanyun.com/zh/loomloom/workbench/viewer/v1/bootstrap.js`

No viewer configuration is required for this default. Resolution is explicit
`--viewer-url` first, then a non-empty `LOOMLOOM_VIEWER_BOOTSTRAP_URL`, then the
built-in production URL. Empty or whitespace-only environment values use the
default; existing non-empty overrides remain effective. Explicit invalid
overrides fail rather than silently choosing another viewer.

The viewer setting is independent of the cloud Server, profile, login and API
Key. The default does not switch the selected API platform. For another viewer
deployment, obtain its published HTTPS `/viewer/v1/bootstrap.js` address and
pass `--viewer-url`; do not infer one from the API Server. The viewer is trusted
executable code: check JavaScript MIME type, anonymous CORS and compatibility
when diagnosing resource failures. HTTP 200 containing HTML is not a working
viewer. Changing persistent shell configuration still requires the user's
authorization; exported overrides must reach the retained preview process.

## Hand off the local draft

1. After the plan is approved, generate a regular UTF-8 draft file (up to 1 MiB).
   To modify a saved version, export that exact version first.
2. Start the command in a retained tool process:

   ```bash
   loomloom template-spec preview ./draft.json --no-open --output json
   ```

   This uses the production viewer unless explicitly overridden. Keep the
   process alive; do not run it in a short-lived shell that terminates child
   processes on return.
3. Wait for the NDJSON event whose `type` is `ready`. Immediately send its full
   `viewUrl`, including `#token=...`, as a clickable **local draft preview** link
   in the user-facing reply, before further validation or save operations. Do
   not leave it only in tool output. Say that it works on the CLI computer while
   this process remains running. `ready` proves only local startup; only browser
   observation can establish that public resources loaded and the graph rendered.
4. Keep editing the same file during review. Follow `synced` and `waiting`
   events, and report parsing or file issues honestly. Still run server `check`
   before saving; a diagram is not proof of execution validity.
5. Save within the user's authorization after review, unless review was
   explicitly declined or saving without it was already authorized. Return the
   **saved-version page** separately. Stop only this preview process when done.

If startup fails, return the specific cause promptly. Check invalid overrides
and CLI version when configuration differs from the documented default. If
assets fail to load, report that browser state. If the tool cannot retain a
process, explain that a live link cannot be kept available. Resolve the issue or offer
local JSON review and obtain the user's decision before bypassing requested
visual review. Do not create remotely just to obtain a link, invent a link, or
silently replace a requested draft page with a saved-version page.

## Privacy and lifecycle

The CLI serves a tiny loopback startup page and protected JSON snapshot. The
browser loads public JS/CSS directly over HTTPS and renders locally. The CLI
never downloads or embeds UI resources. No cloud token, login, contract query,
Redis, upload, save or model execution is used by preview. Missing model metadata
stays unverified. Drafts and local session credentials stay on this computer.

Give the private local URL only to the user for this session; do not put it in
telemetry, logs or reports. Closing a tab does not stop CLI. Refresh uses the
original fragment; restarting CLI requires its new link. On exit the service
closes and the page can detect disconnection, but do not promise that browser
data already received is erased. There is no cloud draft expiry or storage.

## Saved versions

Use `loomloom template-spec view-link <template-id> <version-id>` with exact IDs
for an already saved version. Its online page requires owner login and is
independent of the local draft page. Successful save responses can include
`viewUrl`; if unavailable, retry only this read, never the successful mutation.

## CLI and Skill versions

`npm install -g` updates only the CLI. An existing Agent Skill can remain stale.
For a requested Beta, install both from that exact version with
`npx @cogfoundry/loomloom@<version> install --agent <agent-id> --yes`, then verify
the selected Agent's installed Skill and its `references/workflow-preview.md`.
See [setup.md](setup.md). Do not use stable `latest` or `loomloom update` to
select a Beta, and do not install into every Agent.
