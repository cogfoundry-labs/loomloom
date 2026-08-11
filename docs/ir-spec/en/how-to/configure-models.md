# Configure models

Run `loomloom template-spec models <execution-unit>` against the target environment. Put a returned executable ID in `defaultModelRef.modelKey`. Schema checks only non-empty structure; the server verifies existence and supported step type. Use only static parameter keys allowed by the execution unit.

```bash
loomloom template-spec models text-generate
loomloom template-spec models image-generate
loomloom template-spec models video-generate
```

`modelKey` is a compatibility field name whose value is the executable catalog model ID. Do not use a display label, provider name, or an ID copied from another environment. If local check passes but create fails, query the target catalog again and verify execution-unit support.
