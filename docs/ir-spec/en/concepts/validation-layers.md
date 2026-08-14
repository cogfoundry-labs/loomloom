# Validation layers

1. JSON Schema validates object shape and source unions.
2. CLI check validates stable local authoring rules without remote access.
3. Core validates references, DAG, triggers, bindings, and sample rows.
4. The save gate resolves Subject/Profile records and validates ports and safe value domains.
5. Run validation resolves Workbook/API input and Artifact access.
6. Runtime exposes Provider, network, content, and asynchronous Artifact failures.

A pass at one layer proves only that layer.
