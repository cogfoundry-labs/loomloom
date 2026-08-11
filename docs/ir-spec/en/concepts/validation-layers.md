# Validation layers

Validation proceeds through JSON Schema, local authoring checks, Core creation validation, compilation, workbook/input checks, and runtime resolution. Passing an earlier layer does not guarantee later model, provider, network, or content checks. Fix the earliest failing layer first.

1. JSON Schema checks structure, types, enums, required properties, and simple conditions.
2. CLI check adds known authoring guards without creating a resource.
3. Core creation validates IDs, references, graph, bindings, ports, models, and current capabilities.
4. The compiler normalizes model selectors, parameters, and input selectors and validates the resulting WorkflowDefinition.
5. Workbook/input validation checks actual rows, required values, enum, MIME, and version compatibility.
6. Runtime resolves assets and artifacts, merges ports, calls providers, and reports dynamic failures.

Do not retry runtime to hide a contract error that can be fixed earlier.
