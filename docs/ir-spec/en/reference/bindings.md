# Input bindings

Each `steps[].inputBindings` key is a target contract `portId`. One target port has one binding.

<a id="ref-ports-and-bindings-step-output"></a>

## Step output

```json
"image": {"source": "stepOutput", "stepId": "stp_source1", "portId": "output"}
```

The source Step must exist, cannot be the same Step, and must appear in `dependsOn`. `portId` is frozen output-contract identity; do not use role, file name, or native JSON pointer as a long-term identity.

## Template Input, literal, and platform context

```json
"prompt": {"source": "templateInput", "inputKey": "creativePrompt"}
"duration": {"source": "literal", "value": 5}
"user_id": {"source": "platformContext", "contextKey": "user.id"}
```

`templateInput` and `composeValue` can declare `fallbackValue`; it applies only when a dynamic source has no value.

## composeValue

Only deterministic string `concat` is supported. Parts are string Template Inputs or non-empty literals. It supports fixed author instruction plus user value; it is not an expression language.

## sequence

`sequence` constructs one position-sensitive heterogeneous native value. Each item declares `position`, `kind`, and source; kinds are text/image/video/audio. It is not Artifact merge.

## merge

`merge` explicitly declares ordered sources for one target port. The first implementation supports two mutually exclusive policies:

- `ordered_artifacts`: merge homogeneous Artifact collections. Sources follow `sources[]`, then Artifact ordinal within each source; the result must satisfy target minItems/maxItems.
- `concat_text`: accepts two or more `stepOutput` text sources, joins them with `\n\n` in `sources[]` order, and requires a target port that allows text multi-value and `concat_text`.

Missing-source policy is `error` or `omit`. `composeValue` combines author literals and Workbook fields only; it is not for runtime Step-output aggregation.
