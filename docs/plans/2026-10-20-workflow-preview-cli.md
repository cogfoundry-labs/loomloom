# Local workflow preview implementation

The fixed architecture is a tiny loopback HTML page loading independently
published Workbench HTTPS modules in the browser. Only local `/api/snapshot`
returns the designated file. The CLI never fetches or embeds frontend assets,
and never submits draft content or binding metadata to a cloud service.

Implementation scope: preview command and browser launch, the local read-only
service/file watcher/bootstrap/CSP, removal of unused ZIP/contract-resolver
consumers, and matching Workbench entry/build/static configuration. Preserve
unrelated frontend work. The original archive was backed up, then removed only
after the user explicitly requested its removal. Keep the existing v1
snapshot envelope, random per-process token, Host/Origin checks, 1 MiB/UTF-8
limits, file-change revisions and refresh fragment.

Verification: CLI tests/vet/race, frontend check/format/E2E, and a real CLI with
independently served HTTPS viewer build in Chromium/WebKit. Verify no remote
request carries a draft, token or cookie, and exercise file errors, session
isolation, refresh, resource failure and static-only UI updates.

The initial delivery stopped at local implementation and verification. On
2026-10-21, the user authorized committing the CLI and publishing the next beta,
v0.6.2-beta.2, through the normal release-branch/tag workflow. Stable/latest must
remain unchanged. Keep the previously released Code/Loop support when preparing
the beta branch; no viewer ZIP or cloud-draft resolver may reappear. Public
HTTPS viewer integration and private-version creation have been verified with
the source-built CLI. Old hashed asset retention remains a separate deployment
requirement. See [workflow viewing](../guides/workflow-preview.md).

On 2026-10-23, the production viewer became the requested built-in default:
`https://www.shengsuanyun.com/zh/loomloom/workbench/viewer/v1/bootstrap.js`.
Explicit flag and non-empty environment overrides retain precedence. The CLI
still makes no frontend resource requests. User guidance no longer requires
viewer setup or advertises a staging deployment.

Initial production verification on that date returned HTTP 200 with `text/html`
and no `Access-Control-Allow-Origin`, which prevented browser module loading.
After deployment, verification returned `application/javascript`, anonymous CORS
(`Access-Control-Allow-Origin: *`), and revalidation of the stable entry. The
source-built CLI rendered a 23-step local draft in Chromium with the production
viewer; saving and reading back a private version also succeeded through the
CLI. Saved-version browser authentication is a separate verification concern.
Local configuration tests alone do not establish deployment readiness.
