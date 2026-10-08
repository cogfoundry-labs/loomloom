"""A small benchmark for the result-review step: can an assistant spot a defect in a generated image? (free to build and score).

16 poster images, half with a known text defect (a typo, a wrong word, an extra line, no text) and half clean, each shown to the
reviewer on its own under a neutral name (v01.jpg ...). The truth for every image is in labels.json. Scoring is mechanical:
recall on the defective images, false alarms on the clean ones, and whether the reviewer's transcription of the headline is right.

    python scripts/image.py llm-advice --score-vision <answers.json> [...]
"""
from __future__ import annotations

import base64
import io
import json
import random
import re
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
BENCH_DIR = ROOT / "references" / "llm-fit" / "vision-bench"
LONG_SIDE = 768                                   # the images the reviewer sees; recorded because it changes both results and cost
TARGET = "READ THE SUMMER"

PROMPT = (
    "You review one AI-generated poster image. It was meant to be a poster for a children's library summer reading challenge whose "
    f"only headline is {TARGET}. Check this requirement: the headline is spelled exactly {TARGET}, with no other lettering anywhere "
    "on the poster (ignore tiny unreadable pretend-text on book pages). Read the lettering letter by letter; do not assume it is "
    "correct. Reply with ONE JSON object and nothing else: "
    '{"headline_text": "<the headline exactly as printed, letter for letter, or null if there is none>", '
    '"other_lettering": "<any other readable words or letters, or null>", "requirement_met": true or false, '
    '"evidence": "<one short sentence>"}')


def build(sources: list[dict], out_dir: Path | None = None, seed: int = 20261008) -> Path:
    """sources: [{path, defect ('none' | 'typo' | 'wrong word' | 'extra text' | 'no text'), headline (as printed, or None), look, source}].
    Writes the neutral, shuffled, downscaled JPEGs and labels.json."""
    from PIL import Image
    out = Path(out_dir or BENCH_DIR)
    out.mkdir(parents=True, exist_ok=True)
    items = list(sources)
    random.Random(seed).shuffle(items)
    labels = {}
    for i, it in enumerate(items, 1):
        name = f"v{i:02d}"
        im = Image.open(it["path"]).convert("RGB")
        im.thumbnail((LONG_SIDE, LONG_SIDE))
        im.save(out / f"{name}.jpg", "JPEG", quality=82)
        labels[name] = {"defect": it["defect"], "headline": it.get("headline"), "extra_text": it["defect"] == "extra text",
                        "look": it.get("look"), "source": it.get("source")}
    (out / "labels.json").write_text(json.dumps(labels, indent=1, ensure_ascii=False), encoding="utf-8")
    return out


def load_labels(bench_dir: Path | None = None) -> dict:
    return json.loads((Path(bench_dir or BENCH_DIR) / "labels.json").read_text(encoding="utf-8"))


def image_data_uri(path: Path) -> str:
    return "data:image/jpeg;base64," + base64.b64encode(Path(path).read_bytes()).decode()


def parse_answer(text: str) -> dict | None:
    m = re.search(r"\{.*\}", text or "", re.S)
    try:
        js = json.loads(m.group(0)) if m else None
    except ValueError:
        return None
    if not isinstance(js, dict) or not isinstance(js.get("requirement_met"), bool):
        return None
    head = js.get("headline_text")
    return {"requirement_met": js["requirement_met"], "headline_text": head if isinstance(head, str) else None,
            "other_lettering": js.get("other_lettering") if isinstance(js.get("other_lettering"), str) else None}


def _norm(s) -> str:
    return re.sub(r"[^A-Z0-9% ]", "", re.sub(r"\s+", " ", str(s or "")).upper()).strip()


def score(answers: dict, labels: dict) -> dict:
    """answers: {image name: parsed answer or None}. An image with no readable answer counts as a miss on defective images and as no
    false alarm on clean ones is NOT assumed: it is counted as unanswered and excluded from both rates, and reported."""
    defective = [k for k, v in labels.items() if v["defect"] != "none"]
    clean = [k for k, v in labels.items() if v["defect"] == "none"]
    ans = {k: a for k, a in answers.items() if a}
    hit = [k for k in defective if k in ans and not ans[k]["requirement_met"]]
    alarm = [k for k in clean if k in ans and not ans[k]["requirement_met"]]
    d_ans = [k for k in defective if k in ans]
    c_ans = [k for k in clean if k in ans]
    recall = len(hit) / len(d_ans) if d_ans else None
    false_alarm = len(alarm) / len(c_ans) if c_ans else None
    # transcription: where the printed headline is known, does the reviewer's transcription match it letter for letter?
    with_text = [k for k, v in labels.items() if v["headline"] and k in ans]
    exact = [k for k in with_text if _norm(ans[k]["headline_text"]) == _norm(labels[k]["headline"])]
    by_defect: dict = {}
    for k in defective:
        if k in ans:
            d = by_defect.setdefault(labels[k]["defect"], [0, 0])
            d[1] += 1
            d[0] += 1 if k in hit else 0
    return {"recall": recall, "false_alarm": false_alarm,
            "balanced": None if recall is None or false_alarm is None else round((recall + (1 - false_alarm)) / 2, 3),
            "transcription": round(len(exact) / len(with_text), 3) if with_text else None,
            "answered": f"{len(ans)}/{len(labels)}", "by_defect": {k: f"{a}/{b}" for k, (a, b) in by_defect.items()},
            "missed": [k for k in d_ans if k not in hit], "false_alarms": alarm,
            "autocorrected": [k for k in with_text if k not in exact and labels[k]["defect"] in ("typo", "wrong word")]}
