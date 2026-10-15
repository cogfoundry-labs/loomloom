# Code / Loop test client alignment

Current development status (2026-10-15): rebased onto main `07833c6`, where unified catalog support is now merged. The listing-only candidate and release evidence below describe the immutable `v0.6.2-beta.1` release. Do not substitute the rebased binary against the older test backend until the backend is updated and the pair is verified.

Goal: provide one CLI candidate that matches the clean Code / Loop test backend, including Market discovery and typed results.

Architecture: retain main's Market listing endpoints and behavior; carry only the Code / Loop result readers and authoring documentation, updating the latter from the current Core source. Do not merge the unified catalog release branch or add API fallback paths.

Baseline: CLI origin/main 95d2c2c22cc6d28086f3621c743ae81d57d8cbc3; prior beta v0.6.1-beta.2 / 1a2ebb4 preserved by its published tag and local backup/cli-v061-before-code-clean-20261015. Backend recorded test-delivery baseline 695464e9, current documentation afef7b21. Work branch feature/code-loop-test-client.

1. Restore only `run.go`, its tests, the embedded documentation topic registration, the documentation packaging script and `docs/ir-spec` Code / Loop content from beta. Keep main's Market commands, publication, subscription and community examples unchanged.
2. Refresh the machine contract, Chinese source snapshot, English translations and examples from Core's unified Code input contract; remove retired Code contractVersion and binding valueType usage. Check source/generation hashes using the existing documentation script.
3. Test Market list/show against the supported listing contract, and typed Code values / Loop summaries / errors through the same CLI build. Exercise exact JSON integer retention and read-only queries of the existing test listing and completed Run if current credentials permit.
4. Run documentation checks, the Go test suite, vet and build. Compare changed files against main, verify no unified catalog dependency or unrelated feature change, and preserve original branches/tags.
5. Commit and push this task branch. Record verification and remaining release boundaries below. Do not merge main, overwrite published beta tags, change server deployments, or create paid Runs for this client-only change.

## Verification and delivery

2026-10-15: candidate built from the clean main baseline with only Code / Loop result DTOs/rendering/tests and authoring documentation packaging. Market list/show, publication and subscription source remain identical to main; no `/skillCatalog` reference exists under `src/cli/internal`. The existing beta branch and tag remain unchanged.

- Go `go test ./...`, `go vet ./...` and native build passed. Existing Market route tests and Code / Loop tests pass together; the latter checks exact integer 9007199254740993, structured JSON, false, accepted/exhausted summaries and step errors.
- Core-linked documentation check and standalone `prepare-checked` passed. Machine contract and Chinese source match Core; English examples/reference updated to unified required inputs, without retired Code contractVersion or binding valueType. Shell syntax and whitespace checks passed.
- Test-server doctor passed. One locally built CLI queried `https://loomloom-test.shengsuanyun.com/loom/v1`: market list found the existing product listing; show returned published listing `01a13a15-e239-7ace-a5c3-a4a13fcd92e5`.
- The same binary read completed Run `db50a242-dc8a-47b7-95f4-3fb9a450f511`: three Values, two Artifacts, Loop accepted at iteration 2. Result workbook downloaded and passed ZIP integrity checking (6763 bytes for this regenerated download).
- Public authoring checks accepted the unified Code example and existing mixed-input product spec; rejected nullable Code input with HTTP 400. Embedded English/Chinese step documentation uses the unified contract.
- These are read-only business queries and validation requests. No new Run, order, model call, template version, deployment, credential, or external resource was created. Temporary downloaded workbook was removed. Local ignored `bin/loomloom-code-loop` and generated documentation remain for review, owned by this task; review by 2026-10-16 and remove after the candidate is accepted or replaced.
- This is a task-branch candidate, not a new npm/GitHub beta release. Existing installed beta.2 binaries are unchanged. New execution/paid end-to-end testing was not repeated; the server's earlier execution evidence remains separate from this client readback.

## Published beta: v0.6.2-beta.1

2026-10-15, explicitly authorized after local testing. Release source remains `7ee173a6f2999459cd7aba456af89a9abed41d3d` on the new `release/v0.6.2` branch; the old mixed `release/v0.6.1` branch and published tags remain unchanged.

- GitHub release workflow 38699497439 succeeded, including artifact checks and Gitee/GitLab ref synchronization. Homebrew update was intentionally skipped for this prerelease.
- npm workflow 38699497703 succeeded, including Linux and Windows isolated Registry tests, all six platform packages, the main package, immutable readback and official Registry install verification.
- Registry `@cogfoundry/loomloom` beta points to `0.6.2-beta.1`; stable latest remains `0.5.1`.
- Independent local install from the official npm Registry returned `loomloom version v0.6.2-beta.1`. The installed package passed test-server doctor, Market list/detail, Code/Loop result readback and English/Chinese unified-contract documentation checks.
- Release: https://github.com/cogfoundry-labs/loomloom/releases/tag/v0.6.2-beta.1
- Release branch/tag, GitHub assets, npm versions and mirrored refs are retained as immutable release assets owned by the LoomLoom CLI maintainers; authority is the GitHub source tag and existing versioned release workflows. No server deployment, main merge or production configuration changed. Temporary isolated installation and release-note staging file were removed. Earlier local candidate artifacts remain subject to the review date above.

## Rebase onto updated main

Both main branches now include the formerly independent Market/catalog feature. CLI main is `07833c6f97dfe7f7fa4935ea6aabf65b947e9e6a`; Core main is `8676c08ce5ed0ae0b7cd496ace375e00a91553fd`. The two CLI patches rebased without conflicts to `491edb3` and `44a2e88`; range-diff reports both patches equivalent. Go tests, vet, build, standalone docs checks and the Core-linked documentation check passed.

Remote backup `backup/code-loop-client-before-main-rebase-20261015@9ccb963` preserves the previous task branch. The published source branch `release/v0.6.2`, tag and npm beta remain at `7ee173a`; no new release or server deployment occurred. The new development candidate uses main's unified catalog API. It requires the corresponding backend update and cannot inherit the old candidate's real-environment acceptance claim. Core's rebase also resolves conflicting Run Proto field numbers; its upgrade notes require coordinated Core/Public API delivery.

Backups and the ignored local `bin/loomloom-code-loop-rebased` binary are retained by the CLI maintainer for review by 2026-10-16. No external runtime, paid Run, credential or release artifact was created or modified.
