# Review the Code authoring flow with the CLI

Use a target test environment where the reviewed Code/Loop version has been delivered and enabled. The CLI must include this manual and Code Value/Loop result support; the presence of a command in an older release is insufficient. The test identity must be allowed to save its own templates, execute controlled Code, and read results. This flow does not qualify formal charging, production availability, or native Sandbox cleanup receipts.

## 1. Prepare a Spec

Copy [the Code text and file example](../examples/valid/code-value-artifact.json) to `code.json`. Use a configured test identity and explicitly select the target Server on every command, for example with `--server https://loomloom-test.shengsuanyun.com/loom/v1`. Do not put Tokens in Specs or shared command records.

```bash
loomloom template-spec check code.json
loomloom template-spec create code.json --version-note "Code review"
```

Record templateId/versionId. Check does not save; creation validates again and freezes the contract. The Code Profile/revision needs a server runtime binding. Authors do not select Sandbox IDs, clusters, or credentials.

## 2. Read back and edit

```bash
loomloom template-spec get-version <template-id> <version-id> -f saved.json
loomloom template-spec create-version <template-id> code.json --version-note "Updated Code"
```

Verify source, bindings, and declared ports. Edits create a new version while retaining the old ID. Select the reviewed version explicitly for later execution.

## 3. Submit inputs

Create `rows.jsonl`; each line maps template input keys to values:

```jsonl
{"text":" Hello Code "}
```

```bash
loomloom orchestration-input upload rows.jsonl
loomloom template-spec run <template-id> --version-id <version-id> --input-file-id <input-file-id> --client-request-id <request-id>
```

The upload returns the batch's inputFileId, not a material inputAssetId. Submission revalidates identity, version, input, contracts, and test admission. Disabled Loop authoring or missing runtime bindings require the engineering delivery configuration.

Cost precheck for a template containing Code is currently unsupported. This command may return `CODE_PRICING_PENDING`; it does not mean Code is free or priced at zero:

```bash
loomloom template-spec precheck <template-id> --version-id <version-id> --input-file-id <input-file-id>
```

Pricing precheck and controlled test submission are separate paths. An authorized test identity may use the run command above for business review; ordinary unauthorized submission remains rejected. The existing precheck error mentions missing internal test admission, but that message alone does not establish test submission eligibility. Cost precheck is not a prerequisite for this business demonstration. Do not bypass guards with zero prices, substituted identity, or direct database writes; permission, input, and model-cost errors still require resolution and must not be ignored together.

Use a new requestId for a new execution. After an ambiguous network result, retrying identical version/input/requestId should return the original Run. Changed input is a new execution. This does not authorize automatic re-execution of an unknown Code Attempt.

## 4. Inspect and download

```bash
loomloom run watch <run-id>
loomloom run result-rows <run-id>
loomloom run result-rows <run-id> --output json
loomloom run result-workbook <run-id> --output-file result.xlsx
```

The first row should complete with text `Hello Code`, passed=true, and an independently delivered JSON Artifact. Text output shows a preview; JSON preserves complete typed Values, Loop status/iterations, and stepErrors. Model fee snapshots are not wallet settlement receipts; unknown-cost markers must remain visible in JSON.

## 5. Conditions and rework

- [Condition example](../examples/valid/code-condition.json): submit `{"text":"Hello Code"}` and `{"text":"REJECT"}` separately. These are demonstration rules. The second source Code completes with passed=false, and its conditional successor is skipped, not a code failure. The source file belongs to that source node, not the skipped node.
- [Rework example](../examples/valid/bounded-text-loop.json): use `loomloom template-spec authoring-context --output json` to replace the text Profile model ID with an available member, then check/save/run with `{"request":"Reply exactly HELLO"}`. Inspect the accepted iteration and final exports. Other responses trigger at most three iterations; exhaustion publishes no accepted result or last candidate. First-round versus later acceptance depends on actual model responses; formal feedback acceptance cites specific Run evidence.
- Nullable explicit Code inputs, more than ten iterations, and outside references to unexported body nodes must fail check. Static Schema cannot replace server permission, scope, or authority validation.

## 6. Record the review

Record the CLI version, Server, template version, Run, inputs, expected/actual results, and product questions about creation, configuration, validation, execution, errors, skipping, rework, and downloads. Track product-flow decisions separately from technical-contract questions. Delivery review confirms the usable window, support owner, and departmental documentation/publicity/sales preparation. A successful demonstration does not confirm those decisions or production launch.
