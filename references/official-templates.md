# Templates

Templates define the input schema for each official workflow. For the full template taxonomy (official, private, SkillBot), see the main [README](../README.md) .

For CLI usage details, see [`cli-reference.md`](cli-reference.md).

The actual templates available depend on your environment. Always rely on `loomloom template list` rather than this static list.
The tables below are human-readable summaries. For exact field keys, labels, enum values, versions, and the latest schema, run `loomloom template schema <template-id> --output json`.

## PRD review template: `prd-four-perspective-review-v1`

| Field | Required | Description |
|---|---|---|
| PRD content | Required | Full PRD content to review. One row represents one PRD review task. |

## Text template: `text-v1`

| Field | Required | Description |
|---|---|---|
| Text prompt | Required | Main task prompt (e.g. "Rewrite this introduction in 80-120 words"). |
| Writing requirements | Optional | Style, tone, or formatting constraints. |
| Reference text | Optional | Short inline text, or a large file uploaded via `input-asset upload` (returns  `input_asset_id`). |

## Image template: `text-image-v1`

| Field | Required | Description |
|---|---|---|
| Image prompt | Required | Description of the image to generate. |
| Style requirements | Optional | For example, watercolor, photorealistic, or studio style. |
| Image aspect ratio | Required | `1:1`, `4:5`, `16:9`, or `9:16`. |

## Video template: `text-image-video-v1`

Current schema version in the test environment: `v2`.

| Field | Required | Description |
|---|---|---|
| Scene description | Required | Description of the video scene. |
| Visual style requirements | Optional | e.g. cinematic, anime, documentary style. |
| Reference image URL | Optional | One public HTTP/HTTPS image URL. |
| Image aspect ratio | Required | `1:1`, `4:5`, `16:9`, or `9:16`. |
| Image model | Optional | Optional image generation model. Leave empty to use the default model. |
| Video aspect ratio | Required | `16:9` or `9:16`. |
| Video duration | Required | `4`, `6`, or `8` seconds. |
| Generate audio | Required | `true` or `false` . |
| Video model | Optional | Optional video generation model. Leave empty to use the default model. |

## Run status

| Status | Meaning |
|---|---|
| `pending` / `queued` | Run accepted and waiting to execute. |
| `running` | Execution in progress. |
| `completed` | All tasks finished successfully and results are available. |
| `partially_failed` | Some tasks failed, but partial results are available. |
| `failed` | Run failed completely. |
| `cancelled` | Run was cancelled. |

You can monitor runs in two ways:

- CLI polling with `loomloom run get <run-id>` / `loomloom run watch <run-id>` (see [`cli-reference.md`](cli-reference.md#runs) for full details)
- For account balance, recharge, and transaction status, use the [ShengSuanYun Console](https://console.shengsuanyun.com/user/recharge).
