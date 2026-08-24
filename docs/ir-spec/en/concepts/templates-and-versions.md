# Templates and versions

A Template is a durable business object. A TemplateVersion is an immutable execution snapshot. Changing inputs, Steps, bindings, authority references, or Workbook presentation creates a new version.

Core freezes authoring, Canonical IR, authority bundles, Profile contracts, and a definition hash. Each Run additionally records the model and contract selected for that execution.

Workbook columns derive from the version's `templateInputs`; download a new Workbook after changing versions.
