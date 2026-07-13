# Execution Units reference

| Unit | Inputs | Output | Parameters |
| --- | --- | --- | --- |
| text-generate | prompt required text/* multiple; reference optional text/* or image/* multiple | output text/plain | prompt |
| image-generate | prompt required text/* multiple; reference optional image/* multiple | output image/png | prompt, aspect_ratio, size |
| video-generate | prompt required text/* single; image optional image/* single | output video/mp4 | prompt, negative_prompt, aspect_ratio, duration_seconds, sample_count, generate_audio, resolution, seed |

Text prompts merge with concat_text; artifact references use ordered_artifacts. Current model IDs and provider capabilities are dynamic. `test-fail` is internal-only.

## Port behavior

Required ports must be satisfied before execution. `allowMultiple` controls whether several bindings may target the same port in one fan-in stage. `concat_text` combines text in deterministic source order; `ordered_artifacts` preserves artifact ordering. Port compatibility uses MIME patterns such as `text/*` and `image/*`, not filename extensions.

Only keys listed in Allowed run parameters may appear in static or row-level parameters. The model catalog adds a second dynamic check: a model must support the selected unit.
