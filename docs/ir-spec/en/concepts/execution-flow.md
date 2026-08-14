# Execution flow

1. Validate v2 structure and DAG when creating a version.
2. Resolve fixed contracts or Profiles and validate `<stepId>.<portId>`.
3. Freeze authoring, Canonical IR, authority contracts, and definition hash.
4. Decode Workbook rows or API values into Template Inputs.
5. Schedule Steps by `dependsOn` and `triggerPolicy`.
6. Compile bindings into Provider-native requests and record actual model, contract, and Artifacts.

`allow_partial` controls Step scheduling. A merge `missingSourcePolicy` independently controls missing data at one target port.
