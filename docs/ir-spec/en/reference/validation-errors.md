# Validation errors

Fix the earliest failing layer. TS-IN-001 covers pasted string input; TS-IN-002 rejects text references bound as prompt instead of initial input; TS-IN-003 rejects string fields described as asset IDs; TS-TOPOLOGY-001 rejects expanded execution in new authoring and publication. Other common failures are missing/duplicate IDs, reserved keys, source defaults, invalid binding modes, duplicate targets, unsupported parameters, incompatible ports, cycles, and unavailable models.

Machine metadata is in `../../machine/rules.json`. Validator messages without stable rule IDs may evolve; do not automate against full prose.

| Failure | Repair |
| --- | --- |
| TS-TOPOLOGY-001 expanded execution | use workbook rows for independent items, explicit Steps for fixed branches, or initial_input for a multi-content collection |
| missing/duplicate field or step | provide unique declared identifiers |
| reserved input key | rename model/provider/mode field |
| missing default | set defaultValue for hidden/default source |
| duplicate target | retain one parameter binding source |
| parameter not allowed | consult execution-unit parameters |
| incompatible port | match output MIME to target accepts |
| dependency cycle | make the graph acyclic |
| unavailable model | query the target environment catalog |

Stable IDs currently cover the input-transport rules. Other error prose may evolve even when the underlying rule remains.
