# Execution Units Reference

本表直接对应 Core `KnownExecutionUnits()`。模型 ID 不在本表中，因为它们由环境目录动态提供。

## text-generate

| Port | Required | Multiple | Accepts | Merge |
| --- | --- | --- | --- | --- |
| `prompt` | 是 | 是 | `text/*` | concat_text |
| `reference` | 否 | 是 | `text/*`, `image/*` | ordered_artifacts |

- Output：`output` / `text/plain`
- Allowed run parameters：`prompt`

## image-generate

| Port | Required | Multiple | Accepts | Merge |
| --- | --- | --- | --- | --- |
| `prompt` | 是 | 是 | `text/*` | concat_text |
| `reference` | 否 | 是 | `image/*` | ordered_artifacts |

- Output：`output` / `image/png`
- Allowed run parameters：`prompt`, `aspect_ratio`, `size`

## video-generate

| Port | Required | Multiple | Accepts | Merge |
| --- | --- | --- | --- | --- |
| `prompt` | 是 | 否 | `text/*` | concat_text |
| `image` | 否 | 否 | `image/*` | ordered_artifacts |

- Output：`output` / `video/mp4`
- Allowed run parameters：`prompt`, `negative_prompt`, `aspect_ratio`, `duration_seconds`, `sample_count`, `generate_audio`, `resolution`, `seed`

`test-fail` 是受环境开关控制的内部测试 unit，不属于公开 TemplateSpec 能力。
