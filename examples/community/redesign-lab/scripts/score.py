#!/usr/bin/env python3
"""
score.py — the ONE optional, paid piece of the whole pipeline. Everything
else in scripts/ is local and free; this is the only script that shells out
to loomloom, and only runs at all if Gate 1 chose "optimized."

DO NOT TREAT AS WORKING, even after the fix below: the aesthetic-scoring
TemplateSpec is a *private* template (`loomloom template-spec ...`), not a
published Market listing — an earlier version of this script called
`loomloom market run`, sent rows shaped like {"variant_id", "prompt",
"image_b64": <base64>}, and never uploaded the screenshot as a real asset at
all. None of those keys match the TemplateSpec's real fields (`screenshot`,
`rubric`), so the model never received anything on the `screenshot` field —
a client-side bug, not a loomloom defect, and now fixed below: real
`input-asset upload` -> `orchestration-input upload` -> `template-spec
run`, matching loomloom's own documented flow.

Fixing that bug was NOT enough. Two fresh, real, paid runs against the
corrected request (see ../references/model-policy.md's "Known gap" section
for the full evidence, run IDs, and cost) still show the model not
receiving the image: one model honestly says so, a second one fabricates a
plausible-sounding description that doesn't match the real screenshot at
all (worse than a refusal). This looks like a genuine platform-level gap
survives a correctly-formatted request. Don't use this path until loomloom
confirms a fix — re-verify with the same real-upload + real-response-text
check described in model-policy.md, not just an absence of errors.

Usage:
    python score.py --screenshots-dir .output/variants \
        --template-id 0c15dc18-7509-49d0-b9d2-f4114283155d \
        --version-id <published-version-id> \
        --out .output/aesthetic-scores.json
"""

import argparse
import json
import subprocess
import sys
import time
from pathlib import Path

# The model is pinned in the TemplateSpec's own `defaultModelRef`
# (see references/model-policy.md), not passed by this script. There is no
# per-call model override on this template (`allowModelOverride` is not
# set), so there is nothing to hand-pin here — a prior version of this
# script defined an unused MODEL_KEY constant pointing at a model that
# isn't even in this environment's catalog; that was dead, misleading code,
# now removed.


def run_loomloom(args):
    try:
        return subprocess.run(["loomloom", *args, "-o", "json"], check=True, capture_output=True, text=True, encoding="utf-8")
    except FileNotFoundError:
        sys.exit("loomloom CLI not found on PATH — install it before running the optimized path")
    except subprocess.CalledProcessError as e:
        # Surface the CLI's own stderr/stdout, not just "exited non-zero":
        # that's the difference between an actionable message and a dead end.
        sys.exit(f"loomloom {' '.join(args)} failed (exit {e.returncode}):\n{e.stderr or e.stdout}")


def upload_screenshots(screenshots_dir):
    """Real per-file upload via `input-asset upload` — the only way an
    asset_ref field's value (a real input_asset_id) can be produced. Inlining
    base64 directly into an input row, as a prior version of this script
    did, does not populate an asset_ref field; it just gets ignored as an
    unrecognized key."""
    variant_ids = []
    asset_ids = []
    for shot in sorted(Path(screenshots_dir).glob("*/desktop.png")):
        variant_ids.append(shot.parent.name)
        result = run_loomloom(["input-asset", "upload", str(shot), "--content-type", "image/png"])
        asset_ids.append(json.loads(result.stdout)["inputAssetId"])
    return variant_ids, asset_ids


def build_and_upload_input_file(asset_ids, workdir):
    """Flat JSONL, one row per variant, keyed by the TemplateSpec's real
    field key (`screenshot`) — not an invented key like `image_b64`, and not
    wrapped in any extra envelope. `rubric` is left unset: the TemplateSpec
    declares it hidden/default_value, so the server fills it in."""
    jsonl_path = workdir / "score-input.jsonl"
    with jsonl_path.open("w", encoding="utf-8") as f:
        for asset_id in asset_ids:
            f.write(json.dumps({"screenshot": asset_id}) + "\n")
    result = run_loomloom(["orchestration-input", "upload", str(jsonl_path)])
    return json.loads(result.stdout)["inputFileId"]


def precheck(template_id, version_id, input_file_id):
    """Free cost estimate. Gate 1's "optimized" choice already implies the
    user saw and approved this estimate before run() is called — this
    function does not re-ask."""
    result = run_loomloom(
        ["template-spec", "precheck", template_id, "--version-id", version_id, "--input-file-id", input_file_id]
    )
    return json.loads(result.stdout)


def run(template_id, version_id, input_file_id):
    client_request_id = f"redesign-lab-score-{int(time.time())}"
    result = run_loomloom(
        [
            "template-spec", "run", template_id,
            "--version-id", version_id,
            "--input-file-id", input_file_id,
            "--client-request-id", client_request_id,
        ]
    )
    return json.loads(result.stdout)["runId"]


def watch_and_collect(run_id):
    run_loomloom(["run", "watch", run_id])
    result = run_loomloom(["run", "result-rows", run_id])
    return json.loads(result.stdout)


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--screenshots-dir", required=True)
    parser.add_argument("--template-id", required=True, help="loomloom private template id for the aesthetic-scoring TemplateSpec")
    parser.add_argument("--version-id", required=True, help="published version id to run (see model-policy.md for the current one)")
    parser.add_argument("--out", default=".output/aesthetic-scores.json")
    args = parser.parse_args()

    out_path = Path(args.out)
    out_path.parent.mkdir(parents=True, exist_ok=True)

    variant_ids, asset_ids = upload_screenshots(args.screenshots_dir)
    if not asset_ids:
        raise SystemExit(f"no variant screenshots found under {args.screenshots_dir}")

    input_file_id = build_and_upload_input_file(asset_ids, out_path.parent)

    estimate = precheck(args.template_id, args.version_id, input_file_id)
    cost = estimate["precheck"]["estimatedTotalCost"]
    print(f"estimated cost: {cost['amount']} {cost['currency']} for {len(asset_ids)} variant(s) — "
          f"this must already be approved (Gate 1's \"optimized\" choice) before calling run()")

    run_id = run(args.template_id, args.version_id, input_file_id)
    result = watch_and_collect(run_id)

    # rowIndex in the result maps back to variant_ids in upload order — the
    # TemplateSpec has no variant_id field of its own to round-trip.
    scored = []
    for row in result["rows"]:
        idx = row["rowIndex"]
        text = next((a["inlineText"] for a in row.get("artifacts", []) if a.get("portName") == "output"), "")
        scored.append({"variant_id": variant_ids[idx], "status": row["status"], "aesthetic_note": text})

    out_path.write_text(json.dumps(scored, indent=2), encoding="utf-8")
    print(f"scored {len(scored)} variant(s) -> {args.out}")


if __name__ == "__main__":
    main()
