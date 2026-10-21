# View a workflow while editing

`template-spec preview` serves the bundled workbench viewer on this computer.
The frontend repository owns the React UI; the CLI contains its compiled HTML,
JS and CSS. Users do not need Node, Vite or a second frontend checkout.

```bash
loomloom template-spec preview ./draft.json --title "Product workflow"
```

Keep the command running while the Agent edits the file. It returns a URL on
`127.0.0.1` with a random port and a per-process credential in the fragment.
Open that complete URL on the same computer. No browser login is required.
Other devices cannot use the link. Treat the complete link as private.

The file may not exist yet or contain incomplete JSON. The CLI reads a regular
UTF-8 file up to 1 MiB, coalesces stable changes, and keeps the latest text in
memory. Browser requests read this snapshot from the CLI. Draft text is never
uploaded to the backend, stored in Redis, or persisted in browser storage.
The configured API is used only for a read-only contract projection: step IDs,
contract/profile identifiers and default model IDs. Prompts, input values and
workbooks are excluded. If authentication or contract lookup is unavailable,
the local viewer still opens and reports unresolved ports; it never invents them.
Temporary authority failures, including partial responses, are retried without
requiring another edit; available ports remain visible.

First content renders directly. Later edits use the shared JSON/node/edge
animation controller. Missing, unreadable, invalid UTF-8 or oversized files
leave the last readable snapshot visible with a warning. Incomplete JSON is
shown as text with parse diagnostics. Neither case saves or runs a workflow.

Ctrl+C / SIGTERM closes the local service. The browser clears its graph and JSON
when it detects disconnection. Restarting produces a new URL; refreshing works
while the same process is alive. Multiple CLI processes use independent ports,
credentials and in-memory snapshots. Closing a browser tab does not stop CLI.

To edit an existing version, first use `get-version` to obtain a local file,
then run `preview` on it. The original version remains unchanged. The removed
`--source-template-id` / `--source-version-id` flags belonged to remote sessions.

## Saved versions

```bash
loomloom template-spec view-link <template-id> <version-id> --output json
```

Saved versions still use the online workbench and require browser authentication
as the owner. They can be viewed from other devices and do not require CLI to stay
running. Existing check/create/create-version commands remain the persistence
boundary. Successful save output includes `viewUrl` when link lookup succeeds.
If it fails, the save is still successful with `viewLinkError: "view_link_unavailable"`;
retry `view-link` using returned IDs, never repeat the mutation to obtain a link.

## Agent automation

Use `preview --output json` in a retained process. NDJSON events are `ready`
(with `viewUrl`), `synced` (local revision), `waiting` (file issue), and `ended`.
Send the ready URL to the user before editing further; retain and stop only this
process when finished. No full definition or remote credentials are printed.
The local link necessarily contains its local access credential.

Old remote preview links and writer/heartbeat/lease APIs are retired. Deploy the
new read-only `/loom/v1/workflowViewerContext` contract endpoint before distributing
this CLI. There is no draft persistence migration and unrelated Redis use remains.

## Maintaining the bundled viewer

In the workbench checkout run `pnpm install --frozen-lockfile` and
`pnpm build:viewer`. In this repository run:

```bash
python3 scripts/package-viewer.py /path/to/workbench/dist-viewer
```

Review the generated `src/cli/internal/viewerassets/viewer.zip` alongside frontend
changes. It contains a protocol manifest and SHA-256 for each file; Go tests and
runtime loading verify them. The deterministic archive is tracked so clean CLI
builds and every release platform embed the same viewer without accessing a
private frontend checkout. Never hand-edit its compiled JS or copy UI source into CLI.
After updating, run CLI tests and the frontend tests; release packaging verifies
the embedded viewer before cross-compiling. Update both artifacts as one release.
