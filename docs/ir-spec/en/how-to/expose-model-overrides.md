# Let callers select a model

v2 replaces arbitrary model override with Capability Profiles. Reference a Profile contract, create an optional string Template Input, and point `modelSelection` to it with a default model. Runtime validates current eligible membership and records the actual selected model.

The routing value never enters Provider-native JSON.
