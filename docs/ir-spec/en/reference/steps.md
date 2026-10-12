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

`executionBinding.kind=codeProfile` requires `code`. Authors do not need to fill in `profileId` or `profileRevision`. When both are omitted, Core defaults to the Python 3.12 standard-library environment and fills in and freezes `python-pure@1` during validation and template version creation. Existing explicit bindings remain compatible, but both fields must be provided together. Unknown environments or revisions are rejected rather than replaced with the default. Implemented environments are `python-pure@1` and `python-image@pillow-12.3.0-1`; the target server must also have the corresponding runtime binding. The image environment includes Pillow. Authors cannot install dependencies at runtime, access the network, or obtain platform Secrets.

| `code` field | Contract |
| --- | --- |
| `language` / `entrypoint` | `python` / `main` |
| `source` | Non-empty inline source defining `main(inputs)`, not an external code-package reference |
| `contractVersion` | String `"1"` or `"2"`; independent of TemplateSpec v2 |
| `inputs` | Version `"1"` has no explicit input declarations; version `"2"` requires at least one required, non-nullable string input |
| `outputs` | Non-empty named outputs; the returned object's keys identify declared ports |

Input keys match target ports in `inputBindings`. Version `"2"` input Schema supports `type=string` and non-negative integer `minLength`/`maxLength`, with minimum no larger than maximum. Other input Schema keywords are unsupported. Type or length failures prevent code execution.

Outputs have two kinds:

- `value`: declare `valueType` and an object `schema`. Types are string, boolean, integer, number, array, and object. Validation uses the platform's supported Schema subset, not every JSON Schema dialect.
- `artifact`: declare a concrete `mimeType` and return the relative path of a file created in this execution's working directory. Publication validates the file, size, and type; arbitrary host files and external URLs are not output paths.

All outputs must validate before publication. Missing or invalid outputs do not become successful downstream inputs. See the complete [Code Value and JSON file example](../examples/valid/code-value-artifact.json). Saving freezes source, environment, and input/output contracts; changes require a new template version.

## Conditional execution with `when`

`when` declares `stepId`, `portId`, and boolean `equals`. The source is a directly dependent Code boolean Value or an explicitly exported, accepted Loop boolean Value. Strings such as `"true"`, null, missing values, and errors are not coerced to booleans.

A comparison mismatch skips the node and its `require_all` successors, without creating their Code Attempts or model calls. A technical failure is distinct from a normal mismatch. Mutually exclusive branch OR joins and nested different conditions are unsupported; conditional paths use `require_all`. See [Code conditions](../examples/valid/code-condition.json).

<a id="ref-loop-scope"></a>

## TS-LOOP-001: Bounded quality rework

A Loop container uses only `{"kind":"loop"}` as its execution binding. It has no ordinary input bindings, Code declaration, model selection, or `when`. Root and body dependencies are acyclic; the container controls repetition instead of adding back edges to `dependsOn`.

| `loop` field | Contract |
| --- | --- |
| `maxIterations` | Required, 1–10, including the first execution |
| `deadlineSeconds` | Required, 1–1800 seconds; both limits constrain execution |
| `body` | A non-empty ordinary child workflow; currently fixed text Profiles and ordinary Code Values/Artifacts, using require_all and ordinary when conditions |
| `until` | `{stepId, portId}` selecting a body Code boolean Value, read after the entire round completes; the judge need not depend on every node |
| `state` | Explicit state map, optionally empty; each entry has `initial` and `update` and an explicit body consumer |
| `outputs` | Non-empty aliases selecting body Code Values or Artifacts; visible outside only after acceptance |

State initializers are non-empty string literals, required string Template Inputs, or string `composeValue`. Updates select body Code string Values. Consumers use `{"source":"loopState","inputKey":"stateName"}`; models bind state only to prompt, and Code targets declare required non-nullable strings. Rejected rounds update state for another iteration. Acceptance freezes exports. Exhaustion, cancellation, and technical failure do not publish the last candidate as a successful result.

Body models use a fixed available text model and resolve built-in or published dynamic Profiles through the ordinary authority. They may depend on body Code nodes and consume string Values; Code may consume model text and produce feedback. Body dependencies/outputs stay within the body; outside nodes read declared container aliases, never body nodes directly. Body Code may generate files and consume them in the current round using the ordinary Profile. Contract version "1" file lists and version "2" required strings remain separate. Unaccepted candidates stay in private facts. Acceptance forwards selected Code file references and integrity metadata without copying bytes. File-only and mixed exports are supported; multiple aliases of one file have distinct stable export identities. File consumption and accepted file exports are deployed to the test server; real acceptance, outer consumption, downloads, and failure controls have passed. Image/video generation rework, nested Loops, and Loop exports in sequence/merge are unsupported. See [bounded text rework](../examples/valid/bounded-text-loop.json).

Unselected nodes are recorded as skipped under the current round identity without Attempts or model bills. Continuing requires current-round state updates; acceptance requires current-round exports. Missing required values fail explicitly and never reuse a prior round. Body text connections and conditions are deployed to the test server and have passed real text revision, cross-round skipping, and required-value failure acceptance. See [current-round JSON file consumption](../examples/valid/loop-code-file-consumption.json) and [accepted file delivery](../examples/valid/loop-code-file-export.json) for files. Text examples: [Code to text model](../examples/valid/loop-code-to-text-model.json) and [conditional delivery](../examples/valid/loop-conditional-delivery.json).

Code and Loop are currently for controlled test review. Schema/local validation does not prove that authoring is enabled, the runtime is configured, or formal charging is available on the target server. Check the environment's delivery record.
