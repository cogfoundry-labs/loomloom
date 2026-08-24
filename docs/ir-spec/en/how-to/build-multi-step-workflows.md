# Build multi-step workflows

When a downstream Step consumes an upstream result, declare both `dependsOn` and a `stepOutput` binding. Use merge for multiple homogeneous Artifact sources and sequence for one position-sensitive multimodal value. Never rely on matching field names for implicit wiring.
