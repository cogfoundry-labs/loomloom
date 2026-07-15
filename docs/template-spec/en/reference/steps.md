# Steps reference

Steps require unique `stp_<6-10 base36>` ID, display name, and known execution unit. Optional instruction, dependencies, upstream bindings, trigger policy, default model, model override, and static params are validated. Multi-upstream steps require explicit bindings. The graph must be acyclic. Empty trigger policy defaults to require_all; allow_partial and fail_fast are also supported.

| Property | Rule |
| --- | --- |
| `stepId` | required, unique pattern |
| `displayName` | required human-readable name |
| `executionUnit` | required registry ID |
| `instruction` | fixed author policy |
| `dependsOn` | existing upstream step IDs |
| `upstreamBindings` | explicit content flow |
| `triggerPolicy` | require_all/allow_partial/fail_fast |
| `defaultModelRef.modelKey` | current executable model ID |
| `allowModelOverride` | permits shared model FieldBinding |
| `staticParams` | keys allowed by the unit |

A template may contain multiple root steps with no `dependsOn`. Steps whose dependencies and inputs are ready in the same round are scheduled concurrently; actual simultaneous execution remains subject to worker and model-provider limits.

Every step-output source must also appear in dependsOn. Multi-upstream and allow_partial configurations require explicit bindings. The server validates model existence and supported step type.
