# Validation errors

| Rule | Rejected condition | Fix |
| --- | --- | --- |
| TS-VERSION-002 | Creating a new v1 version | Migrate offline and create a v2 version |
| TS-BINDING-002 | stepOutput without matching dependsOn | Declare scheduling dependency and data binding |
| TS-PROFILE-002 | Profile Step without valid modelSelection | Reference an optional string input and set a default model |

JSON Schema validates shape. Core validates cross-field, DAG, and source semantics. Save-time authority validation proves current environment executability.
