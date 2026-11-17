# Optional template delivery outputs: CLI implementation and verification

Date: 2026-11-17. Development branch: `feature/template-delivery-outputs`.
CLI baseline: `e2c583c4665b0a5b0b4077b8b0c01f146f859105`.
Canonical backend: `feat/loom-task-console`, commit
`f49b134454258dce0e97fab912f71a859a0f568a`.

## Scope and implementation

Follows the backend `docs/2026-10/p1/loom-task-console/design.md` sections
8.2 / 8.2.1, plan's CLI author-contract section, and report E38/E39.

- Sync the canonical optional `deliveryOutputs` Schema and rules through
  `scripts/template-spec-docs.sh`. Preserve required fields `meta/steps/workbook`.
- Translate the delivery syntax and valid/invalid examples, record translation
  digests, and regenerate the Chinese snapshot and bundle manifests. Example
  comparison normalizes only translated delivery labels; selectors remain
  structurally checked against the backend source.
- Document author choice of root `stepId` + complete output `portId`, stable key,
  label, limits, new-version behavior, and omission semantics in the Agent guide.
- Keep existing `json.RawMessage` transport. Tests prove check/create/version/
  export/edit preserve the complete spec. No new run flag, client-side semantic
  validator, Console authentication path, or alternate result download contract.
- Preview test verifies the raw draft including a Chinese delivery label reaches
  the local snapshot endpoint unchanged. This does not assert browser rendering
  or a visual delivery editor.

## Local verification

macOS arm64, isolated official Go 1.27.4 toolchain; its download SHA-256 was
verified. No Go toolchain change is committed.

Passed:

- Canonical `template-spec-docs.sh check`, local check, and `build` (English and
  Chinese CLI documentation plus generated-example validation).
- `scripts/template-spec-docs.sh prepare-checked`, followed by
  `(cd src/cli && go test -race ./...)` and `go vet ./...`.
- CLI build and `scripts/skill-references.sh check`.
- All shell scripts' syntax checks and all seven `scripts/test-*.sh` suites.
- npm tooling: 24 passed, 1 Windows-only test skipped. Repository audit gate
  passed using its existing expiring advisory acceptances; this is not a claim
  of zero dependency advisories.
- `gofmt` and `git diff --check`.

PowerShell-specific checks and the Windows command-shim execution were not run
on this Mac. Release-script tests use isolated local/mock fixtures and do not
publish real releases. Installation tests clean generated docs, so prepare the
bundle again before subsequent Go tests/builds; do not run those steps together.

## Real test environment

Endpoint: `https://loomloom-test.shengsuanyun.com/loom/v1`.
Authentication passed with the replacement test credential. Credentials are not
part of the repository, fixtures, command examples, or this report.

### Authoring regression: passed

`TestTemplateSpecDeliveryIntegration` passed against the real endpoint:

- Omitted and valid declarations validate; a declaration changes definition hash.
- Eight invalid cases return HTTP 400 with a `deliveryOutputs` error: empty
  object/items, wrong schema version, unknown port/step, duplicate key/source,
  and whitespace-only label. Authentication or transport failures do not count
  as successful rejection tests.
- Create templates with and without a declaration; export exact versions,
  edit/export/save, and verify old versions remain unchanged.
- Add, rename, and remove a declaration through new versions. Read back latest
  and published pointers; invalid save does not append a version.

Dedicated final fixtures:

| Template | Initial state | Edited version |
| --- | --- | --- |
| `59b17cf5-44dd-4f85-a7df-078e2c70cdb0` | omitted | `5e3a451f-62d6-45e2-97c7-ed64853706d4` |
| `a7677db5-6faa-4125-a1f8-13068ef13545` | declared | `d01d67fa-1bda-4fa3-b446-73027daece44` |

An earlier authoring smoke run also retained dedicated templates
`38083b7e-1387-4d2e-a444-a8d14154821b` and
`b061f90d-a5e5-40a0-b279-c6e3e5cfcb30`.
These are synthetic test templates; the Public API has no template delete
route. Existing user templates were not modified or published to Market.

To rerun authoring tests, supply the test endpoint and credential via
`LOOMLOOM_DELIVERY_TEST_SERVER` and `LOOMLOOM_DELIVERY_TEST_TOKEN`, then run:

```bash
scripts/template-spec-docs.sh prepare-checked
(cd src/cli && go test -run '^TestTemplateSpecDeliveryIntegration$' -count=1 -v ./internal/cmd)
```

The live test is opt-in and creates only dedicated test templates/versions.
It does not run models or Code.

### Hosted execution: backend blocker, not accepted

A one-row model-only template was built using the test server's actual
`capability resolve` and authoring-context response:
`text.basic.openai-chat.v1`, available default `ali/qwen-plus-latest`, logical
output port `text`. Check, create, get-version, and precheck all passed.

Declared fixture:

- Template: `a67ca8fd-a61d-4660-aaef-8351a3b15d86`.
- Version: `3b929090-5fd1-404f-8e75-e3f40652a775`.
- Input: `a50c5c46-d8e0-412e-91d9-e2bbddce7332`.

Both the CLI and a direct authenticated HTTP POST to
`/users/me/templates/{template}:run`, supplying only `versionId`, `inputFileId`,
and a fresh `clientRequestId`, returned:

```text
HTTP 400
rpc error: code = InvalidArgument desc = definition identity does not match workflow definition
```

Control: create another version of the same synthetic template omitting only
`deliveryOutputs`. Direct HTTP returned 201. Run
`a19b4141-4206-4fcd-8f5b-58099d344937` completed successfully. CLI watch,
result-rows (containing `Hello Delivery`), XLSX result workbook, artifact list,
and artifact download all passed, including downloaded content checks.

The local backend source explains the difference:

1. `workflow_run_service.go` builds definition snapshot/hash before preparing
   the execution plan.
2. `prepareWorkflowExecution` calls `cloneWorkflowDefinition` in
   `application/service/template_service.go:1033`. Its copied top-level fields
   omit `DeliveryOutputs`.
3. `startBatchOrchestrationV1` replaces the request definition with the prepared
   clone, while retaining the original snapshot/hash.
4. `execution_core.go:150` rejects their mismatch. A definition without the
   declaration does not expose that field loss.

The CLI must not drop the declaration or bypass definition identity checks to
hide this failure. Backend repair needs to preserve a deep copy of the frozen
selection in all relevant definition clones and add submission/hash regression
coverage, followed by deployment and another real declared-version execution.
No backend code or deployed service was changed by this CLI task.

The opt-in `TestTemplateSpecDeliveryExecutionIntegration` reproduces the
failure and can verify its eventual repair. In addition to the server/token
variables above, set `LOOMLOOM_DELIVERY_RUN_TEMPLATE`,
`LOOMLOOM_DELIVERY_RUN_VERSION`, and `LOOMLOOM_DELIVERY_RUN_INPUT` to the dedicated
fixture IDs, then run:

```bash
(cd src/cli && go test -run '^TestTemplateSpecDeliveryExecutionIntegration$' -count=1 -v ./internal/cmd)
```

This separate test submits hosted work after precheck and therefore requires
explicit fixture selection. It then checks completion and result downloads.

### Code precheck: existing environment limitation

The Code example validates and persists with its file/value delivery selection,
but its execution precheck returns HTTP 402 `CODE_PRICING_PENDING`: Code pricing
is not integrated. No Code run was submitted after that response. Code runtime,
Loop execution, Console Cookie/JWT download scopes, production data, mixed
backend deployment, and browser rendering were not validated by this CLI task.

## Acceptance status

CLI author-contract implementation, local regressions, and real authoring
round trips passed. Full declared-delivery execution acceptance failed on the
backend identity mismatch above. This branch alone must not be described as
end-to-end delivery execution verified or production release-ready.
