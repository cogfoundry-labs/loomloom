# Build multi-step workflows

Multi-step workflows may be linear, split into fixed parallel branches, or merge multiple upstreams. List every upstream step in `dependsOn`, then add explicit `step_output` bindings from source ports to compatible target ports. IDs and MIME must match.

## Start with one upstream

Add the source step to `dependsOn`, bind its output to one compatible target port, and give the downstream step its own instruction and supported model. Validate this linear path before adding fan-in.

```json
{"stepId":"stp_summary1","dependsOn":["stp_product","stp_engineer"],"triggerPolicy":"require_all","upstreamBindings":[{"inputPort":"prompt","sourceType":"step_output","sourceStepId":"stp_product","sourcePort":"output"},{"inputPort":"prompt","sourceType":"step_output","sourceStepId":"stp_engineer","sourcePort":"output"}]}
```

The text-generate prompt port accepts multiple text artifacts and concatenates them. Video prompt and image ports do not allow repeated bindings. Validate IDs, acyclic topology, source-port output type, target-port accepted MIME, and trigger policy. See `examples/valid/multi-upstream-summary.json`.

## Build fixed parallel branches

When the template author knows the branch count and processing path, declare multiple root steps and bind the same input field to each one. No `branch`, `parallel`, or other special property is required. Steps with resolved inputs and no unfinished dependencies become ready in the same scheduling round.

```text
book_title
  |-- scene_a --> image_a
  `-- scene_b --> image_b
```

Each image step depends only on its paired text step and binds that step's `output` to its `prompt`. To create ten branches, repeat ten step-and-binding pairs. This is fixed DAG topology, not `expanded` execution fan-out. See `examples/valid/parallel-text-to-image-branches.json`.

Ready steps in the same round are scheduled concurrently. Actual simultaneous execution remains subject to worker and model-provider concurrency limits.

## Verify and troubleshoot

```bash
loomloom template-spec check ./template.json
```

Unknown sources indicate a missing ID or dependsOn entry. Port errors indicate a source output and target accepts mismatch. Duplicate-port errors require `allowMultiple=true` or separate target ports. See `reference/steps.md`, `reference/bindings.md`, and `reference/execution-units.md`.
