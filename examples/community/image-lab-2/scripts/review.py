"""Visual-check review for Image Lab 2: record what each image looked like against the plan's `visual_checks`, and (with approval) have a
vision model read them for you.

Every image is judged against the plan's `visual_checks` (at most six requirements that models follow only loosely: exact spelling, a
visible product, no extra words). A verdict is `pass`, `fail` or `cant_tell` per check, with a short reason, plus a one-line note. The
verdicts live in the ledger (`row["review"]`), so they survive a refresh, show in the workbook's Review column, label the montage tiles
and become the captions of a case study.

    python scripts/image.py review --dir <experiment> --record verdicts.json [--by "Claude Sonnet 5.5"] [--source agent|user]     free
    python scripts/image.py review --dir <experiment> [--rows r001,r002] [--only-unreviewed] [--model M] [--check "..."]          a quote
    python scripts/image.py review --dir <experiment> ... --confirm <fingerprint> [--max-usd N]                                   spends a little

The model run spends money (a few cents for dozens of images), so it has the same gate as every other spend: a quote with a
fingerprint, and the user's approval of exactly that fingerprint. It never generates or changes an image. A call that fails is
reported and never retried automatically."""
from __future__ import annotations

import base64
import hashlib
import io
import json
import re
import sys
import threading
import time
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))
import ledger as lg  # noqa: E402
import matrix as mx  # noqa: E402
import sheet as sh  # noqa: E402

VERDICTS = ("pass", "fail", "cant_tell", "na")      # na: the requirement does not apply to this image (it names another direction, or compares several images)
SOURCES = ("model", "agent", "user")
DEFAULT_MODEL = "openai/gpt-5.4-mini"          # measured on the vision benchmark: 1.00, and the cheapest model that scored it
LONG_SIDE = 1536                                # a long side that keeps small type readable (the 768 px benchmark images were for headlines)
IMAGE_TOKENS = 1100                             # input tokens one image costs (measured live on 2026-10-11: 1,385 in for a 1152x1440 image plus a 400-token prompt)
PROMPT_TOKEN_CHARS = 3.5
EXPECTED_OUT = 300                              # measured: 263 and 277 tokens out per image
CEILING_IN_EXTRA = 1500                         # tokens the ceiling allows beyond the expected input
CEILING_OUT = 3000                              # reasoning tokens are not held to max_tokens, so the ceiling is generous
MAX_TOKENS = 1500
PROMPT_VERSION = "review-v2"


class ReviewError(Exception):
    """A review that cannot be recorded or priced; the message says why."""


# --------------------------------------------------------------------------- #
# the verdicts in the ledger
# --------------------------------------------------------------------------- #
def latest_image(exp: Path, ledger: dict, row_id: str):
    """(the row's latest completed attempt, its image path) or (None, None)."""
    done = [a for a in ledger["attempts"] if a.get("row_id") == row_id and a.get("status") == "Completed" and a.get("file")]
    for a in reversed(done):
        p = sh._safe_image(exp, a["file"])
        if p is not None and p.exists():
            return a, p
    return None, None


def checks_of(plan: dict | None, extra: list[str] | None = None) -> list[str]:
    return [str(c) for c in ((plan or {}).get("visual_checks") or [])] + [str(c).strip() for c in (extra or []) if str(c).strip()]


def _verdict(v, where: str) -> dict:
    if isinstance(v, str):
        v = {"verdict": v}
    if not isinstance(v, dict) or v.get("verdict") not in VERDICTS:
        raise ReviewError(f"{where}: a verdict is one of {', '.join(VERDICTS)} (got {v!r})")
    return {"verdict": v["verdict"], "reason": str(v.get("reason") or "").strip()[:300]}


def normalize(raw: dict, n_checks: int, *, source: str, reviewer: str, image: str | None, at: str | None = None, where: str = "review") -> dict:
    """A review as stored on a row. `raw` has `checks` (a list of {n, verdict, reason}, or a {number: verdict} map; may be empty when only a
    note is given), `note` (one line) and optionally `text_seen`. Every check up to n_checks that the review does not mention is cant_tell."""
    if source not in SOURCES:
        raise ReviewError(f"source must be one of {', '.join(SOURCES)}")
    given: dict[int, dict] = {}
    checks = raw.get("checks") or []
    if isinstance(checks, dict):
        checks = [{"n": k, **(_verdict(v, where))} for k, v in checks.items()]
    for c in checks:
        try:
            n = int(c["n"])
        except (KeyError, TypeError, ValueError):
            raise ReviewError(f"{where}: every check needs its number n")
        if n_checks and not 1 <= n <= n_checks:
            raise ReviewError(f"{where}: check {n} does not exist (the plan has {n_checks} visual checks)")
        if n < 1:
            raise ReviewError(f"{where}: check numbers start at 1")
        given[n] = _verdict(c, f"{where} check {n}")
    if given and not n_checks:                                  # a quick folder has no plan checks: the verdicts define them
        n_checks = max(given)
    out = [{"n": n, **given.get(n, {"verdict": "cant_tell", "reason": "not reviewed"})} for n in range(1, n_checks + 1)] if given else []
    rev = {"source": source, "reviewer": reviewer, "at": at or lg.now(), "image": image, "checks": out,
           "note": str(raw.get("note") or "").strip()[:400]}
    if raw.get("text_seen"):
        rev["text_seen"] = str(raw["text_seen"])[:600]
    return rev


def tally(review: dict) -> dict:
    c = [x["verdict"] for x in review.get("checks") or []]
    return {"pass": c.count("pass"), "fail": c.count("fail"), "cant_tell": c.count("cant_tell"), "na": c.count("na"), "n": len(c) - c.count("na")}


def mark(review: dict | None) -> str:
    """A very short label for a tile or a cell: ok, fail 1,4, ?3 or note (a check that does not apply never counts)."""
    if not review:
        return ""
    cs = review.get("checks") or []
    if not cs:
        return "note"
    fails = [str(c["n"]) for c in cs if c["verdict"] == "fail"]
    if fails:
        return "fail " + ",".join(fails)
    unsure = [str(c["n"]) for c in cs if c["verdict"] == "cant_tell"]
    if unsure:
        return "?" + ",".join(unsure)
    return "ok" if any(c["verdict"] == "pass" for c in cs) else "n/a"


def is_stale(review: dict | None, exp: Path, ledger: dict, row_id: str) -> bool:
    """True when the row's latest image is not the one that was reviewed (a retake or a retry replaced it)."""
    if not review or not review.get("image"):
        return False
    a, _ = latest_image(exp, ledger, row_id)
    return bool(a) and a.get("file") != review["image"]


def cell_text(review: dict | None, stale: bool = False) -> str:
    """The workbook's Review cell: '5/6 pass; fail: 4 · the one-line note' (checks that do not apply are left out of the count)."""
    if not review:
        return ""
    t = tally(review)
    head = ""
    if t["n"] or t["na"]:
        head = f"{t['pass']}/{t['n']} pass" if t["n"] else "no check applies"
        fails = [str(c["n"]) for c in review["checks"] if c["verdict"] == "fail"]
        unsure = [str(c["n"]) for c in review["checks"] if c["verdict"] == "cant_tell"]
        if fails:
            head += f"; fail: {','.join(fails)}"
        if unsure:
            head += f"; can't tell: {','.join(unsure)}"
    parts = [p for p in (("(older image) " if stale else "") + head, review.get("note")) if p]
    return " · ".join(parts)


def caption(review: dict | None) -> str:
    """The one line a case study shows under an image: the note, else the reasons of the failed checks."""
    if not review:
        return ""
    if review.get("note"):
        return review["note"]
    bad = [f"check {c['n']}: {c['reason']}" if c.get("reason") else f"check {c['n']} failed" for c in review.get("checks") or [] if c["verdict"] == "fail"]
    return "; ".join(bad) if bad else ("All checks passed." if review.get("checks") and mark(review) == "ok" else "")


# --------------------------------------------------------------------------- #
# recording (free)
# --------------------------------------------------------------------------- #
def _plan_of(exp: Path):
    return mx.load_plan(exp / "plan.json") if (exp / "plan.json").exists() else None


def apply_reviews(ledger: dict, exp: Path, reviews: dict, n_checks: int, *, source: str, reviewer: str) -> list[str]:
    """Put reviews on rows. `reviews` is {row id: raw review}. Returns warnings (an unknown row, a row with no image yet)."""
    warnings: list[str] = []
    rows = lg.by_id(ledger)
    for rid, raw in reviews.items():
        row = rows.get(rid)
        if row is None or row["status"] == "Removed":
            warnings.append(f"{rid}: no such row, skipped")
            continue
        a, _ = latest_image(exp, ledger, rid)
        if a is None:
            warnings.append(f"{rid}: no finished image yet, skipped")
            continue
        row["review"] = normalize(raw, n_checks, source=source, reviewer=reviewer, image=a["file"], where=rid)
    return warnings


def record(exp_dir, reviews: dict, *, source: str = "agent", reviewer: str = "the assistant") -> dict:
    """Record reviews on the ledger (and refresh the workbook so its Review column shows them). Free."""
    import experiment as ex
    exp = Path(exp_dir)
    if not (exp / "ledger.json").exists():
        raise ReviewError(f"{exp} has no ledger.json")
    plan = _plan_of(exp)
    n_checks = len(checks_of(plan))
    warnings: list[str] = []
    if (exp / "plan.json").exists():
        rf = ex.refresh(exp, after_merge=lambda ledger: warnings.extend(apply_reviews(ledger, exp, reviews, n_checks, source=source, reviewer=reviewer)))
        return {"warnings": warnings, "workbook": rf.get("workbook")}
    with lg.writer_lock(exp, "review"):                       # a quick folder has no plan or workbook
        ledger = lg.load(exp / "ledger.json")
        warnings.extend(apply_reviews(ledger, exp, reviews, 0, source=source, reviewer=reviewer))
        lg.save(exp / "ledger.json", ledger)
    return {"warnings": warnings, "workbook": None}


def load_record_file(path) -> dict:
    try:
        data = json.loads(Path(path).read_text(encoding="utf-8"))
    except (OSError, ValueError) as e:
        raise ReviewError(f"cannot read {path}: {e}")
    if not isinstance(data, dict) or not all(isinstance(v, dict) for v in data.values()):
        raise ReviewError(f"{path} must be a JSON object {{row id: {{checks, note}}}}")
    return data


# --------------------------------------------------------------------------- #
# the model run (a quote, then an approved spend)
# --------------------------------------------------------------------------- #
def review_prompt(brief: str, params: dict, checks: list[str]) -> str:
    made = "; ".join(f"{k}: {v}" for k, v in (params or {}).items() if v)
    numbered = " ".join(f"{i}. {c}" for i, c in enumerate(checks, 1))
    return (
        "You review one AI-generated image against numbered requirements. "
        f"The image was made for this brief: {brief} "
        + (f"It was made with these choices: {made}. " if made else "")
        + "For EACH requirement answer pass, fail, cant_tell or na. Use na when the requirement does not apply to THIS image (it names another "
          "direction, or compares several images). Use cant_tell only when this image does not show enough to judge (for example lettering too "
          "small to read); a matter of taste (premium, distinctive, beautiful) is cant_tell, never pass or fail. Read any lettering letter by "
          "letter and do not assume it is correct. "
        f"Requirements: {numbered} "
        'Reply with ONE JSON object and nothing else: {"text_seen": "<every readable word or letter in the image, verbatim, or null>", '
        '"checks": [{"n": 1, "verdict": "pass|fail|cant_tell|na", "reason": "<one short sentence>"}, ...one entry per requirement], '
        '"note": "<one short sentence summing up the image for a caption>"}')


def parse_answer(text: str, n_checks: int) -> dict | None:
    m = re.search(r"\{.*\}", text or "", re.S)
    try:
        js = json.loads(m.group(0)) if m else None
    except ValueError:
        return None
    if not isinstance(js, dict) or not isinstance(js.get("checks"), list):
        return None
    try:
        normalize(js, n_checks, source="model", reviewer="x", image=None)
    except ReviewError:
        return None
    return js


def image_data_uri(path: Path) -> str:
    try:
        from PIL import Image
        im = Image.open(path).convert("RGB")
        im.thumbnail((LONG_SIDE, LONG_SIDE))
        buf = io.BytesIO()
        im.save(buf, "JPEG", quality=88)
        return "data:image/jpeg;base64," + base64.b64encode(buf.getvalue()).decode("ascii")
    except ImportError:
        mime = "image/png" if Path(path).suffix.lower() == ".png" else "image/jpeg"
        return f"data:{mime};base64," + base64.b64encode(Path(path).read_bytes()).decode("ascii")


def _sha(path: Path) -> str:
    return hashlib.sha256(Path(path).read_bytes()).hexdigest()[:12]


def quote(exp_dir, models: list[dict], model: str | None = None, rows: list[str] | None = None, extra_checks: list[str] | None = None,
          only_unreviewed: bool = False, budget: float | None = None) -> dict:
    exp = Path(exp_dir)
    if not (exp / "ledger.json").exists():
        raise ReviewError(f"{exp} has no ledger.json")
    ledger = lg.load(exp / "ledger.json")
    plan = _plan_of(exp)
    checks = checks_of(plan, extra_checks)
    if not checks:
        raise ReviewError("there is nothing to review against: this experiment has no visual_checks in its plan. Pass --check \"...\" (repeatable)")
    if len(checks) > 6:
        raise ReviewError(f"{len(checks)} checks: a review works with at most 6 (the plan allows 6; drop some --check)")
    if plan and mx.consent_problem(plan):
        raise ReviewError("a person is pictured in this plan: run `acknowledge-person` first (the review sends the images to a model provider)")
    model = model or DEFAULT_MODEL
    m = next((x for x in models if x["api_name"] == model), None)
    if m is None:
        raise ReviewError(f"{model} is not in the gateway's model list")
    if m.get("can_read_images") is False:
        raise ReviewError(f"{model} cannot read images")
    pin, pout = m.get("input_price"), m.get("output_price")
    if pin is None or pout is None:
        raise ReviewError(f"{model} has no price in the gateway listing, so it cannot be quoted")
    wanted = set(rows) if rows else None
    jobs, skipped = [], []
    for row in ledger["rows"]:
        if row["status"] == "Removed" or (wanted is not None and row["id"] not in wanted):
            continue
        a, p = latest_image(exp, ledger, row["id"])
        if p is None:
            continue
        rev = row.get("review")
        if only_unreviewed and rev and rev.get("checks") and not is_stale(rev, exp, ledger, row["id"]):
            skipped.append(row["id"])
            continue
        jobs.append({"row": row["id"], "file": a["file"], "sha": _sha(p), "params": row["params"]})
    if wanted is not None:
        missing = sorted(wanted - {j["row"] for j in jobs} - set(skipped))
        if missing:
            raise ReviewError(f"no finished image to review for: {', '.join(missing)}")
    brief = ledger["experiment"]["brief"]
    prompt_chars = max((len(review_prompt(brief, j["params"], checks)) for j in jobs), default=0)
    in_tokens = IMAGE_TOKENS + int(prompt_chars / PROMPT_TOKEN_CHARS)
    n = len(jobs)
    expected = round(n * (in_tokens * pin + EXPECTED_OUT * pout) / 1e6, 4)
    ceiling = round(n * ((in_tokens + CEILING_IN_EXTRA) * pin + CEILING_OUT * pout) / 1e6, 4)
    fp = hashlib.sha256(json.dumps({"v": PROMPT_VERSION, "model": model, "checks": checks, "jobs": jobs, "budget": budget, "brief": brief},
                                   sort_keys=True, ensure_ascii=False).encode("utf-8")).hexdigest()[:12]
    return {"model": model, "checks": checks, "jobs": jobs, "skipped": skipped, "images": n, "expected_usd": expected, "max_usd": ceiling,
            "budget": budget, "fingerprint": fp, "in_tokens": in_tokens,
            "args": {"model": model, "rows": rows, "extra_checks": extra_checks, "only_unreviewed": only_unreviewed, "budget": budget}}


def run(q: dict, confirm: str, exp_dir, models: list[dict], post=None, workers: int = 4, source_label: str | None = None) -> dict:
    """One call per image, a few at a time, never past the budget. Verdicts go into the ledger; the answers, usage and cost of every call
    are kept in <experiment>/reviews/. A failed call is recorded and never retried."""
    from concurrent.futures import ThreadPoolExecutor
    exp = Path(exp_dir)
    if confirm != q["fingerprint"] or q["fingerprint"] != quote(exp, models, **q["args"])["fingerprint"]:
        raise PermissionError("the confirmation does not match the quote (or an image changed since): nothing was sent")
    if post is None:
        import image as il
        import llm_fit as lf
        tok = il.token()

        def post(body):
            return lf.stream_chat(tok, body)
    ledger = lg.load(exp / "ledger.json")
    brief = ledger["experiment"]["brief"]
    checks = q["checks"]
    m = next(x for x in models if x["api_name"] == q["model"])
    pin, pout = m["input_price"], m["output_price"]
    per_call_max = q["max_usd"] / q["images"] if q["images"] else 0.0
    budget = q["budget"] if q.get("budget") is not None else q["max_usd"]
    lock = threading.Lock()
    committed = [0.0]

    def one(job):
        with lock:
            if committed[0] + per_call_max > budget:
                return {"row": job["row"], "status": "skipped", "error": "budget", "answer": None, "usage": {}, "implied_usd": None}
            committed[0] += per_call_max
        path = sh._safe_image(exp, job["file"])
        content = [{"type": "text", "text": review_prompt(brief, job["params"], checks)}, {"type": "image_url", "image_url": {"url": image_data_uri(path)}}]
        t0 = time.time()
        status, js, err = post({"model": q["model"], "max_tokens": MAX_TOKENS, "messages": [{"role": "user", "content": content}]})
        text = (((js or {}).get("choices") or [{}])[0].get("message") or {}).get("content") or ""
        usage = (js or {}).get("usage") or {}
        implied = round(((usage.get("prompt_tokens") or 0) * pin + (usage.get("completion_tokens") or 0) * pout) / 1e6, 6) if usage else None
        with lock:
            committed[0] += (implied if implied is not None else 0.0) - per_call_max
        ans = parse_answer(text, len(checks))
        return {"row": job["row"], "file": job["file"], "status": status, "error": err, "seconds": round(time.time() - t0, 1), "answer": ans,
                "raw": None if ans else text[:300], "usage": usage, "implied_usd": implied}

    with ThreadPoolExecutor(max_workers=workers) as ex_:
        results = list(ex_.map(one, q["jobs"]))
    good = {r["row"]: r["answer"] for r in results if r["answer"]}
    rec = record(exp, good, source="model", reviewer=source_label or q["model"]) if good else {"warnings": [], "workbook": None}
    out_dir = exp / "reviews"
    out_dir.mkdir(exist_ok=True)
    cost = round(sum(r["implied_usd"] or 0 for r in results), 4)
    (out_dir / f"review-{q['fingerprint']}.json").write_text(json.dumps(
        {"at": lg.now(), "model": q["model"], "checks": checks, "cost_usd": cost, "results": results}, indent=1, ensure_ascii=False), encoding="utf-8")
    return {"results": results, "cost_usd": cost, "recorded": len(good), "workbook": rec.get("workbook"), "warnings": rec["warnings"],
            "failed": [r["row"] for r in results if not r["answer"]], "skipped": [r["row"] for r in results if r["status"] == "skipped"]}


# --------------------------------------------------------------------------- #
# the command
# --------------------------------------------------------------------------- #
def add_args(ap) -> None:
    ap.add_argument("--dir", required=True, help="the experiment folder")
    ap.add_argument("--record", default=None, metavar="FILE", help="record verdicts you made yourself: {row id: {checks: {1: pass, 2: fail}, note}} (free)")
    ap.add_argument("--by", default=None, help="who made the verdicts you record (default: the assistant)")
    ap.add_argument("--source", choices=("agent", "user"), default="agent", help="for --record: whose reading it is (default agent)")
    ap.add_argument("--rows", default=None, help="comma-separated row ids to review (default: every row with a finished image)")
    ap.add_argument("--only-unreviewed", action="store_true", help="skip rows that already have a current review")
    ap.add_argument("--model", default=None, help=f"the vision model (default {DEFAULT_MODEL})")
    ap.add_argument("--check", action="append", default=None, help="an extra requirement to check (repeatable); a quick folder has none of its own")
    ap.add_argument("--max-usd", type=float, default=None, help="never reserve past this much for the run (default: the quote's ceiling)")
    ap.add_argument("--confirm", default=None, help="the fingerprint of the quote you approved: this spends")


def format_quote(q: dict) -> str:
    L = [f"REVIEW QUOTE (a vision model reads each image against {len(q['checks'])} requirement(s); it never generates or changes an image)"]
    L.append(f"Model:              {q['model']}   Images: {q['images']} (sent at up to {LONG_SIDE} px on the long side)"
             + (f"   Skipped (already reviewed): {len(q['skipped'])}" if q["skipped"] else ""))
    for i, c in enumerate(q["checks"], 1):
        L.append(f"  {i}. {c}")
    L.append(f"Estimated cost:     about ${q['expected_usd']:.3f}, at most ${q['max_usd']:.3f} (listed prices; the ceiling assumes {q['in_tokens'] + CEILING_IN_EXTRA:,} tokens in and {CEILING_OUT:,} out per image)")
    L.append("It sends each generated image and the requirements to the model's provider; nothing else from your project.")
    return "\n".join(L)


def run_command(a, models: list[dict]) -> int:
    exp = Path(a.dir)
    try:
        if a.record:
            rec = record(exp, load_record_file(a.record), source=a.source, reviewer=a.by or ("the user" if a.source == "user" else "the assistant"))
            print(f"recorded verdicts from {a.record}")
            for w in rec["warnings"]:
                print(f"warning: {w}")
            if rec["workbook"]:
                print(f"workbook: {rec['workbook']} (Review column updated)")
            return 0
        rows = [r.strip() for r in a.rows.split(",") if r.strip()] if a.rows else None
        q = quote(exp, models, a.model, rows, a.check, a.only_unreviewed, a.max_usd)
        print(format_quote(q))
        if not a.confirm:
            print(f"Fingerprint: {q['fingerprint']}   (run again with --confirm {q['fingerprint']} after the user approves the cost)")
            return 0
        res = run(q, a.confirm, exp, models)
    except (ReviewError, mx.PlanError, lg.LedgerBusy, PermissionError) as e:
        print(f"error: {e}", file=sys.stderr)
        return 2
    for r in res["results"]:
        ans = r["answer"]
        if ans:
            rev = normalize(ans, len(q["checks"]), source="model", reviewer=q["model"], image=None)
            print(f"  {r['row']}  {mark(rev):<10} {rev['note']}")
        else:
            print(f"  {r['row']}  NO ANSWER ({r.get('error') or r['status']})")
    print(f"Reviewed {res['recorded']} of {q['images']}; cost about ${res['cost_usd']:.4f}. Answers in {exp / 'reviews'}.")
    if res["failed"]:
        print(f"Not reviewed: {', '.join(res['failed'])} (a failed call is never retried automatically: run again with --only-unreviewed after approving a new quote)")
    for w in res["warnings"]:
        print(f"warning: {w}")
    return 1 if res["failed"] else 0
