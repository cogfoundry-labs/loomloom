# Steps

A Step requires `stepId`, `displayName`, and `executionBinding`. It may declare `dependsOn`, `triggerPolicy`, `modelSelection`, and `inputBindings`. Code nodes require `code`; Loop containers require `loop`. Ordinary nodes may use boolean `when` conditions.

Dependencies control scheduling and must be acyclic. They do not move data automatically. A stepOutput binding must name a source that is also in `dependsOn`.

<a id="ref-profiles-model-selection"></a>

## TS-PROFILE-002: Profile model selection

A fixed-contract model or Code Step cannot declare `modelSelection`. A Capability Profile supports two sources:

- `fixed`: set `defaultModelId`, without a non-empty `inputKey`.
- `templateInput`: reference an optional string input without a static enum; blank uses `defaultModelId`, while a non-blank value is validated against current eligible Profile membership.

Model IDs and contracts come from the target environment's authoring catalog, not model names.

<a id="ref-code-authoring"></a>

## TS-CODE-001: Code nodes

`executionBinding.kind=codeProfile` requires `code`. Authors do not need to fill in `profileId` or `profileRevision`. When both are omitted, Core fills in and freezes the single runtime environment `python@1` (CPython 3.12, the standard library plus Pillow, numpy, pydantic, jinja2 and PyAV) during validation and template version creation. When provided, they must be `python` and `1`; the retired `python-pure@1` and `python-image@pillow-12.3.0-1` are rejected with a hint to omit both fields, and unknown environments are never replaced with a default. `executionBinding.resourceClass` may be `standard` (default) or `large`; the tier is frozen with the version (standard: 30 seconds and 1 GiB of memory; large: 120 seconds and 4 GiB). Input files may be text, JSON, PNG/JPEG/WebP and MP4/WebM/QuickTime. Authors cannot install dependencies at runtime, access the network, or obtain platform Secrets. The full message and traceback of a user exception are shown only in the author's execution details (`loomloom run step-executions`); the Run and result rows show only the exception type.

| `code` field | Contract |
| --- | --- |
| `language` / `entrypoint` | `python` / `main` |
| `source` | Non-empty inline source defining `main(inputs)`, not an external code-package reference |
| `inputs` | Declare every input explicitly, or `{}` for no inputs. `kind=value` supports text and JSON; `kind=artifact` receives file lists. All inputs require `required=true` and `nullable=false` |
| `outputs` | Non-empty named outputs; the returned object's keys identify declared ports |

Input keys must match `inputBindings` one-to-one. Value inputs declare `valueType` (string, boolean, integer, number, array, or object), with an optional object-form `schema`. External Schema references, implicit type coercion, and generated defaults are unsupported. File inputs declare `acceptedMimeTypes`, `minItems`, and `maxItems`, with 1 <= minItems <= maxItems <= 16; the frozen runtime also constrains supported files. Invalid resolved inputs are rejected before Code dispatch. Workflows containing Code have no additional node-count cap.

Outputs have two kinds:

- `value`: declare `valueType` and an object `schema`. Types are string, boolean, integer, number, array, and object. Validation uses the platform's supported Schema subset, not every JSON Schema dialect.
- `artifact`: declare a concrete `mimeType` and return the relative path of a file created in this execution's working directory. Publication validates the file, size, and type; arbitrary host files and external URLs are not output paths.

All outputs must validate before publication. Missing or invalid outputs do not become successful downstream inputs. See the complete [Code Value and JSON file example](../examples/valid/code-value-artifact.json). Saving freezes source, environment, and input/output contracts; changes require a new template version.

## Conditional execution with `when`

`when` declares `stepId`, `portId`, and boolean `equals`. The source is a directly dependent Code boolean Value or an explicitly exported, accepted Loop boolean Value. Strings such as `"true"`, null, missing values, and errors are not coerced to booleans.

A comparison mismatch skips the node and its `require_all` successors, without creating their Code Attempts or model calls. A technical failure is distinct from a normal mismatch. Mutually exclusive branch OR joins and nested different conditions are unsupported; conditional paths use `require_all`. See [Code conditions](../examples/valid/code-condition.json).

<a id="ref-loop-scope"></a>

## TS-LOOP-001: Bounded quality rework

A Loop container uses only `{"kind":"loop"}` as its execution binding. It has no ordinary input bindings, Code declaration, or model selection, but it may carry `when`: if the condition is not met (or a dependency was skipped) the whole Loop is `skipped` with zero rounds and downstream steps are skipped by the ordinary rules. Root and body dependencies are acyclic; the container controls repetition instead of adding back edges to `dependsOn`.

| `loop` field | Contract |
| --- | --- |
| `maxIterations` | Required, 1–10, including the first execution |
| `deadlineSeconds` | Required, 1–1800 seconds; both limits constrain execution |
| `body` | A non-empty ordinary child workflow; currently fixed text Profiles and ordinary Code Values/Artifacts, using require_all and ordinary when conditions |
| `until` | Exactly one of until/decision/control; selects a body Code boolean Value after the whole round completes; existing semantics remain unchanged |
| `decision` | Exactly one control mode; selects a body Code string Value containing continue/accept/reject, read after the whole round; deployed to test, named business acceptance passed |
| `control` | Exactly one control mode; requires a decision selector and a non-empty checkpoints array, selecting Code string continue/accept/reject Values; deployed to test, named business acceptance passed |
| `state` | Explicit state map, optionally empty; each entry has `initial` and `update` and an explicit body consumer |
| `outputs` | Non-empty aliases selecting body Code Values/Artifacts or frozen text model Artifacts; visible outside only after acceptance |

State initializers are non-empty string literals, required string Template Inputs, or string `composeValue`. Updates select body Code string Values. Consumers use `{"source":"loopState","inputKey":"stateName"}`; models bind state only to prompt, and Code targets declare required non-nullable strings. A round that continues updates state for the next iteration. Acceptance freezes exports. Exhaustion, cancellation, and technical failure do not publish the last candidate as a successful result.

Whole-round decision continue advances to another round, or exhausted at the limit. Accept selects current-round exports; reject ends with business rejection without updating state or publishing a candidate. A completed Code node can cause a rejected Loop; public Task/Run counters use existing failed states, and incurred model fees remain. Exactly one of until/decision/control is required and cannot be null. Other actions fail rather than defaulting to continue. A decision-only payload omits until so an old reader explicitly rejects it. See [whole-round business decisions](../examples/valid/loop-business-decision.json).

In control mode, checkpoints are read after the dispatched batch completes and persists, before the next batch starts. Checkpoint continue proceeds within the current round; accept/reject return an exit intent and persist scoped skipped/LOOP_EXITED records for pending nodes, without Attempts or fees. The parent Loop validates all current exports before accepting; missing exports fail. Control.decision is read when the body completes normally, and its continue advances to the next round. Checkpoint source nodes must be unique and ordered by dependency ancestry. Skipped or invalid required checkpoints fail; dispatched parallel peers finish before evaluation, with system failure/cancellation taking precedence. A control-only payload omits both legacy selectors and must be rejected by old readers. See [mid-round checkpoints](../examples/valid/loop-checkpoint.json). Zero-round execution, skipping a round, immediate preemption, and parallel exit arbitration are unsupported; ordinary when does not end a Loop.

Body models use a fixed available text model and resolve built-in or published dynamic Profiles through the ordinary authority. They may depend on body Code nodes and consume string Values; Code may consume model text and produce feedback. Body steps use their own dependencies/outputs, and may also read the output of an outer step X when X is listed in the `dependsOn` of both the container and that body step (X cannot be another Loop); outside nodes read declared container aliases, never body nodes directly. Body Code may generate files and consume them in the current round using the ordinary Profile. Text, JSON, and files use one explicit input declaration to select their delivery type. Unaccepted candidates stay in private facts. Acceptance forwards selected file references and integrity metadata without copying bytes. File-only and mixed exports are supported; multiple aliases of one file have distinct stable export identities. File consumption and accepted file exports are deployed to the test server; real acceptance, outer consumption, downloads, and failure controls have passed. Direct model exports and Artifact aliases in ordinary sequence/merge bindings are deployed to the controlled test server. Real text merging, exhaustion without publication, and native video sequences have passed. The built-in text port is output; dynamic text Profiles declaring a text output may select text, while the exported file remains a text/plain Artifact. Model text artifacts are limited to 2 MiB; the platform reads actual bytes to verify integrity. Connections still validate target contracts, MIME types, source counts, dependencies, and missing-source policies. Values are not implicitly converted to files; exporting does not repeat model calls or fees. Image, video and other Profiles, and fixed model contracts, may also run in the body: one round may return up to 16 files of the declared MIME family (2 MiB for text, 64 MiB otherwise), and provider addresses are stored first. A fixed model contract's prompt must pass through a Code step before binding `loopState`. Nested Loops are unsupported. See [conditions and outer reads](../examples/valid/loop-outer-read-conditional.json). See [model Artifact exports and merging](../examples/valid/loop-model-artifact-merge.json) and [bounded text rework](../examples/valid/bounded-text-loop.json).

Loop vision text Profiles are deployed to the controlled test server, with real two-round acceptance/crop delivery and three-round exhaustion verified. Ordinary single-image vision Profiles accept one required PNG/JPEG/WebP Artifact and still output text/plain. Each round receives the authorized reference image through ordinary bindings; feedback remains string state. Built-in text.vision.openai-chat.v1 and corresponding published dynamic vision text Profiles follow ordinary model contracts. Image/video generation inside the body is described above. See [vision feedback rework](../examples/valid/loop-vision-feedback.json).

Unselected nodes are recorded as skipped under the current round identity without Attempts or model bills. Continuing requires current-round state updates; acceptance requires current-round exports. Missing required values fail explicitly and never reuse a prior round. Body text connections and conditions are deployed to the test server and have passed real text revision, cross-round skipping, and required-value failure acceptance. See [current-round JSON file consumption](../examples/valid/loop-code-file-consumption.json) and [accepted file delivery](../examples/valid/loop-code-file-export.json) for files. Text examples: [Code to text model](../examples/valid/loop-code-to-text-model.json) and [conditional delivery](../examples/valid/loop-conditional-delivery.json).

Code and Loop are currently for controlled test review. Schema/local validation does not prove that authoring is enabled, the runtime is configured, or formal charging is available on the target server. Check the environment's delivery record.
