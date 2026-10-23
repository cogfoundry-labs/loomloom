# View a local workflow while editing

`template-spec preview` starts a tiny page and read-only data service on this
computer. The browser loads the independently published Workbench viewer's
JS/CSS directly over HTTPS. The CLI does not download, proxy, cache, decompress
or embed those resources. Users need a browser, not Node or a frontend checkout.

The default viewer entry is the ShengSuanYun production URL:

`https://www.shengsuanyun.com/zh/loomloom/workbench/viewer/v1/bootstrap.js`

No viewer configuration is needed for the default. Run:

```bash
loomloom template-spec preview ./draft.json --title "Product workflow"
```

The CLI opens the browser and prints a private `127.0.0.1` URL with a random
port and a per-process credential in its fragment. Use `--no-open` for automation
or open the complete link manually if the browser launcher fails. Keep this
process running while editing. Refreshing retains the fragment and reconnects
to the same process; restarting creates a new port/token.

## Agent setup and draft review

Installing only the npm CLI does not update an existing Agent Skill. To install
this Beta and its matching guidance for Codex, use:

```bash
npx @cogfoundry/loomloom@0.6.2-beta.3 install --agent codex --yes
```

Choose the explicit target Agent for other tools. `loomloom update` follows
stable latest, so use the exact Beta setup command when selecting a Beta.
The viewer URL is separate from both installation and the cloud API profile.
Resolution order is explicit `--viewer-url`, then a non-empty
`LOOMLOOM_VIEWER_BOOTSTRAP_URL`, then the built-in production URL. An empty or
whitespace-only environment value uses the default. Existing non-empty overrides
remain effective; remove an old override to return to the production viewer.
For another deployment, provide its published HTTPS entry:

```bash
loomloom template-spec preview ./draft.json --no-open --output json \
  --viewer-url 'https://viewer.example/workbench/viewer/v1/bootstrap.js'
```

Explicit invalid overrides fail rather than silently selecting a different
viewer. For persistent configuration, set the variable in the appropriate shell
setup only with the user's authorization. Existing API Keys and Servers are
unchanged, and selecting an API Server does not select a different viewer.

The authoring handoff is: generate the local draft, start preview in a retained
process, immediately return the complete `ready.viewUrl` as a clickable draft
link, then review/edit, check, and save within the user's authorization. A user
can decline review. On configuration or resource failure, explain the cause and
resolve it or agree on local JSON review instead of silently saving remotely.
`ready` confirms the local server started; it does not prove browser rendering.
Return the saved-version link separately after a successful save.

## Data and lifecycle

The CLI reads only the designated regular UTF-8 file, up to 1 MiB. The file can
be missing initially. Stable file changes replace an in-memory snapshot; the
browser polls same-origin `/api/snapshot` using Authorization and no-store.
Draft text, prompts, input values, node metadata and session credentials never
leave the computer. No API key, website login, contract query, Redis, database,
extra draft copy or browser draft storage is required. Only public UI resources
come from the configured static origin; no local file is uploaded there.

Local rendering follows declared nodes, bindings, step-output references and
execution dependencies. Referenced ports are marked as coming from JSON, not
certified model metadata. A graph does not prove validation or executability.
Invalid JSON is shown with an explicit error; an unreadable file can leave the
last readable snapshot visible, clearly labelled. No script or remote URL in
the document is executed or automatically fetched.

Ctrl+C / SIGTERM closes the service. The page reports disconnection and unmounts
its viewer. This does not guarantee erasure of data the browser already received.
Multiple processes have independent ports, tokens and snapshots. Links are local
session capabilities, not cross-device links or website accounts.

## Errors and compatibility

The browser needs network access when public assets are not cached. An unavailable
entry, stylesheet or module displays a load error; there is no embedded ZIP or
CLI download fallback. An invalid explicit bootstrap URL fails at startup. A
200 response containing HTML is not a working module: the deployment must serve
JavaScript with a JavaScript MIME type and anonymous CORS. The
bootstrap uses anonymous CORS and the configured trusted origin; API requests
stay on the local origin. Layout workers are created by browser code, not CLI.

The snapshot keeps `local-workflow-preview/v1`, decimal-string `sequence`,
`title`, raw nullable `content`, and `problem`; `context` remains null for older
v1 readers. Unauthorized requests cannot read content. A future incompatible
protocol produces an explicit error. Compatible UI changes can be deployed
independently; breaking protocol changes need a coordinated compatibility plan.

## Agent automation

Run `preview --no-open --output json` in a retained process. NDJSON events are
`ready` (with private `viewUrl`), `synced` (local revision), `waiting` (file issue)
and `ended`. Give the ready link to the user before editing; never place it in
telemetry or diagnostics. Stop only that retained process when finished.

## Saved versions

`loomloom template-spec view-link <template-id> <version-id>` remains a separate,
owner-authenticated online operation. Successful create/create-version responses
can include `viewUrl`; link errors do not undo saves or justify retrying mutations.
This online path is not used by local preview.

## Workbench deployment

The frontend owns `pnpm build:viewer`, which emits a stable `bootstrap.js` and
hashed assets. Deployment publishes them under the configured `/viewer/v1/`
path, with anonymous CORS, revalidated bootstrap and immutable hashed resources.
Retain assets referenced by older pages. The historical viewer.zip and its
runtime consumers have been removed; the CLI distribution contains no viewer
archive.

Before selecting a viewer deployment, verify its actual HTTPS/CORS/cache behavior
and protocol compatibility. The configured bootstrap is trusted executable code.
A release of the public UI does not upload a local draft or publish a CLI version.
Publishing the CLI beta follows the repository release/tag pipeline separately.
