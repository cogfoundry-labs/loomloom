# Configure models

First query the target environment for authoring choices that match the Step's
business inputs and output:

```bash
loomloom capability resolve --input text --output-modality image --output json
```

The result may contain both kinds of choice:

- Use the returned `fixedModelContract` and real `subjectRevisionId` when the
  workflow requires one exact model and its dedicated interface.
- Use the returned `capabilityProfile` when callers should choose among models
  that implement the same interface.

Do not infer capability from a model name, Provider path, or historical
documentation. Prefer the match from `capability resolve`. To inspect every
Profile in the target environment, run:

```bash
loomloom template-spec authoring-context --output json
```

A dynamic Capability Profile returns:

- `profileId`: the stable identity referenced by a template;
- `definition`: the input, output, and constraints fixed at publication;
- `operations.defaultModelId` and `defaultModelAvailable`: the current
  operational default and whether it is currently usable;
- `eligibleModels`: the live model set calculated from current model capability
  facts.

Ordinary templates write only the stable `profileId`, without
`profileRevision`. A TemplateVersion stores the fixed interface snapshot, not
the complete model set at creation time. Later validation, precheck, and run
admission use the latest matching set after models are added or removed. An
already accepted Run continues with its selected concrete model.

The current environment may expose text, image, or video Profiles, for example:

- `text.basic.openai-chat.v1`: text prompt input and text output;
- `text.vision.openai-chat.v1`: text prompt plus image Artifact input and text
  output;
- `image.text-to-image.v1`: text prompt input and image Artifact output;
- `video.text-to-video.v1`: text prompt input and video Artifact output.

These IDs are examples. Always use the target environment's current response
for availability and port definitions. Capabilities a model has beyond the
Profile definition are not exposed through that Step interface.

A Profile Step still declares a separate `modelSelection`. The current
TemplateSpec v2 request shape requires a model-choice input and
`defaultModelId`; at creation, use the current returned default and verify that
it appears in `eligibleModels`. When a caller leaves the model field blank, a
dynamic Profile uses its current operational default at run time. An explicit
value must still be in the current `eligibleModels`. If the current default is
unavailable, the first release fails explicitly instead of silently replacing
it.

Artifact inputs and outputs must follow the `kind`, `acceptedMimeTypes`,
`minItems`, and `maxItems` in `definition`. Do not declare an uploaded asset ID
as a string and place it in `prompt`, and do not connect an Artifact output as
if it were a text port.
