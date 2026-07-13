# Build multi-step workflows

List every upstream step in `dependsOn`, then add explicit `step_output` bindings from source ports to compatible target ports. IDs and MIME must match. Multiple sources may target one port only when `allowMultiple=true`; the registry merge policy defines ordering or text concatenation. Choose require_all, allow_partial, or fail_fast for the intended failure behavior.

## Start with one upstream

Add the source step to `dependsOn`, bind its output to one compatible target port, and give the downstream step its own instruction and supported model. Validate this linear path before adding fan-in.

```json
{"stepId":"stp_summary1","dependsOn":["stp_product","stp_engineer"],"triggerPolicy":"require_all","upstreamBindings":[{"inputPort":"prompt","sourceType":"step_output","sourceStepId":"stp_product","sourcePort":"output"},{"inputPort":"prompt","sourceType":"step_output","sourceStepId":"stp_engineer","sourcePort":"output"}]}
```

The text-generate prompt port accepts multiple text artifacts and concatenates them. Video prompt and image ports do not allow repeated bindings. Validate IDs, acyclic topology, source-port output type, target-port accepted MIME, and trigger policy. See `examples/valid/multi-upstream-summary.json`.

## Verify and troubleshoot

```bash
loomloom template-spec check ./template.json
```

Unknown sources indicate a missing ID or dependsOn entry. Port errors indicate a source output and target accepts mismatch. Duplicate-port errors require `allowMultiple=true` or separate target ports. See `reference/steps.md`, `reference/bindings.md`, and `reference/execution-units.md`.
