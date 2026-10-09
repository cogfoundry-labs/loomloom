"""Preflight for Image Lab 2 (design-v2.md section 9). No gateway call, spends nothing.

    run_preflight(exp_dir) reads the workbook (read-only), merges the user's edits into the
    ledger, validates each selected row, compiles prompts, assigns models and sizes, prices
    the batch, writes the execution snapshot snapshots/<fingerprint>.json and returns a
    report. `run` (M2) will read ONLY that snapshot.

The Advisor, size picking and pricing are reused from image.py (v0.1).
"""
from __future__ import annotations

import hashlib
import json
import math
import re
import time
from pathlib import Path

import compile as cp
import image as il                       # v0.1 Advisor, catalog, sizes, prices
import ledger as lg
import matrix as mx
import workbook as wbk

REF_SUPPORT_FILE = Path(__file__).resolve().parent.parent / "references" / "reference-support.json"
REF_OK = ("verified", "documented")
INCLUDE_STATUSES = ("Draft", "Ready", "Failed", "Partial", "Completed")
SKIP_STATUSES = ("Unknown", "Blocked", "Queued", "Generating")
IMAGE_MAGIC = ((b"\x89PNG\r\n\x1a\n", "png"), (b"\xff\xd8\xff", "jpeg"), (b"RIFF", "webp"))


def load_reference_support(path=None) -> dict:
    p = Path(path) if path else REF_SUPPORT_FILE
    return json.loads(p.read_text(encoding="utf-8"))["models"] if p.exists() else {}


def promote_reference(model: str, usd: float | None, path=None) -> bool:
    """A real reference call succeeded on `model`: record it as verified (state, price, date).
    Support states are promoted by real usage, not by a probe sweep (design 11.4). Returns True
    when the file changed."""
    p = Path(path) if path else REF_SUPPORT_FILE
    if not p.exists():
        return False
    data = json.loads(p.read_text(encoding="utf-8"))
    entry = data["models"].get(model)
    if entry is None:
        return False
    before = json.dumps(entry, sort_keys=True)
    entry["state"] = "verified"
    if usd is not None:
        entry["usd_per_image_reference"] = round(float(usd), 6)
    entry["verified_on"] = time.strftime("%Y-%m-%d")
    if json.dumps(entry, sort_keys=True) == before:
        return False
    import ledger as _lg
    _lg.write_json_atomic(p, data)
    return True


def sha256_file(p: Path) -> str:
    h = hashlib.sha256()
    h.update(p.read_bytes())
    return h.hexdigest()


def looks_like_image(p: Path) -> bool:
    try:
        head = p.read_bytes()[:12]
    except OSError:
        return False
    return any(head.startswith(m) for m, _ in IMAGE_MAGIC)


def snapshot_fingerprint(snap: dict) -> str:
    """Hash of the canonical snapshot body (design 9): the rows plus the numbers `run` derives its
    spending limit from. created_at is not part of the approval."""
    body = {"rows": snap["rows"], "estimated_usd_known": snap.get("estimated_usd_known", 0.0),
            "unverified_rows": snap.get("unverified_rows", [])}
    canon = json.dumps(body, sort_keys=True, separators=(",", ":"), ensure_ascii=False)
    return hashlib.sha256(canon.encode("utf-8")).hexdigest()[:12]


MAX_REFERENCE_BYTES = 20 * 1024 * 1024


def price_key(model: str, token, quality, mode: str) -> str:
    """The key an observed or hinted price is stored under: model, size token, explicit quality, reference mode."""
    return (f"{model}|{token or 'default'}" + (f"|q={quality}" if quality and quality != "auto" else "")
            + ("|reference" if mode == "reference" else ""))


PRICE_HINTS_FILE = Path(__file__).resolve().parent.parent / "references" / "price-hints.json"
HINT_MAX_AGE_DAYS = 180


def load_price_hints(path=None) -> dict:
    try:
        return json.loads(Path(path or PRICE_HINTS_FILE).read_text(encoding="utf-8"))
    except (OSError, ValueError):
        return {}


def _hints_fresh(hints: dict) -> bool:
    """A hint older than HINT_MAX_AGE_DAYS (or with no readable date) is not shown: prices change and nothing else expires it."""
    import datetime
    try:
        age = (datetime.date.today() - datetime.date.fromisoformat(str(hints.get("measured")))).days
    except ValueError:
        return False
    return 0 <= age <= HINT_MAX_AGE_DAYS


def indicative(keys: list[str], hints: dict | None = None) -> dict | None:
    """An indicative price range for rows whose real price is unknown (never part of a quote): an exact hint for the row's
    model, size and quality, else the range over the hints for that model and quality. None when nothing matches."""
    hints = hints if hints is not None else load_price_hints()
    table = hints.get("usd_per_image") or {}
    if not keys or not table or not _hints_fresh(hints):
        return None
    lo = hi = 0.0
    for key in keys:
        if key in table:
            lo += table[key]
            hi += table[key]
            continue
        parts = key.split("|")
        q = next((part for part in parts[1:] if part.startswith("q=")), None)
        if q is None or "reference" in parts:        # no explicit quality ("auto" bills differently) or a reference call (about 3.4x): no hint
            return None
        near = [v for k, v in table.items() if k.split("|")[0] == parts[0] and q in k.split("|") and "reference" not in k.split("|")]
        if not near:
            return None
        lo += min(near)
        hi += max(near)
    return {"low": round(lo, 4), "high": round(hi, 4), "measured": hints.get("measured"), "rows": len(keys),
            "max_usd": max(0.05, round(-(-hi * 1.5 * 100 // 1) / 100, 2))}


def reference_problem(exp_dir: Path, rel: str) -> str | None:
    """Why this reference file cannot be sent, or None. Confined to the experiment folder."""
    p = (Path(exp_dir) / rel).resolve()
    try:
        p.relative_to(Path(exp_dir).resolve())
    except ValueError:
        return f"reference file {rel} must be inside the experiment folder"
    if not p.is_file():
        return f"reference file {rel} not found"
    size = p.stat().st_size
    if size > MAX_REFERENCE_BYTES:
        return f"reference file {rel} is {size / 1e6:.1f} MB; the limit is {MAX_REFERENCE_BYTES // (1024 * 1024)} MB"
    if il._image_kind(p.read_bytes()[:12]) is None:
        return f"reference file {rel} is not a PNG, JPEG or WebP image"
    return None


def matches(a: dict, params: dict) -> bool:
    """A sample counts for a row only if the row still has the values it was generated with."""
    return a.get("params", params) == params


def accounted_samples(attempts: list[dict], include_unknown: bool = False, params: dict | None = None) -> tuple[int, dict]:
    """Samples of a row that already exist or are in flight, so a retry must not ask for them again,
    plus a count per reason for the report. Completed, still-running, Blocked (the model refused),
    Unknown (unless the user asked for it) and billed-but-undownloaded ones all count."""
    n, why = 0, {}
    for a in attempts:
        st = a["status"]
        if params is not None and not matches(a, params):
            continue                                       # made under other values; not part of this configuration
        if st == "Completed":
            n += 1
        elif st in ("Pending", "Submitting", "Running"):
            n += 1
            why["in flight"] = why.get("in flight", 0) + 1
        elif st == "Blocked":
            n += 1
            why["blocked"] = why.get("blocked", 0) + 1
        elif st == "Unknown" and not include_unknown:
            n += 1
            why["unknown"] = why.get("unknown", 0) + 1
        elif st == "Failed" and a.get("url") and a.get("cost"):
            n += 1
            why["billed, not downloaded"] = why.get("billed, not downloaded", 0) + 1
    return n, why


def aspect_to_wxh(ratio: str, px: int = 1024 * 1024) -> str | None:
    """'4:5' -> a WxH of about 1 megapixel with that ratio (a preference for pick_size)."""
    try:
        a, b = (float(x) for x in ratio.split(":"))
    except ValueError:
        return None
    if not (math.isfinite(a) and math.isfinite(b)) or a <= 0 or b <= 0:
        return None
    w = (px * a / b) ** 0.5
    return f"{round(w / 64) * 64}x{round(w * b / a / 64) * 64}"


def exact_wxh(ratio: str, floor_px: int = 1024 * 1024, step: int = 16, max_side: int = 3840,
              max_ratio: float = 3.0, max_px: int = 8294400) -> str | None:
    """The smallest WxH with exactly this aspect ratio, both sides a multiple of `step`, at least
    `floor_px` pixels and inside the model's documented limits; None if there is none. 4:5 -> 1024x1280."""
    try:
        a, b = (float(x) for x in ratio.split(":"))
    except ValueError:
        return None
    if not (math.isfinite(a) and math.isfinite(b)) or a <= 0 or b <= 0 or max(a / b, b / a) > max_ratio:
        return None
    from fractions import Fraction
    fr = Fraction(a).limit_denominator(1000) / Fraction(b).limit_denominator(1000)
    ra, rb = fr.numerator, fr.denominator
    m = 1
    while True:
        w, h = ra * m, rb * m
        if max(w, h) > max_side or w * h > max_px:
            return None
        if w % step == 0 and h % step == 0 and w * h >= floor_px:
            return f"{w}x{h}"
        m += 1


def ratio_text(w: int, h: int) -> str:
    from math import gcd
    g = gcd(w, h) or 1
    return f"{w // g}:{h // g}"


class Advisor:
    """Caches the v0.1 scoring for one intent."""

    def __init__(self, intent_label: str):
        self.catalog = il.load_model_catalog()
        policy = il.load_policy()
        self.intent = il.match_intent(intent_label, policy)
        self.observations = il.load_price_observations()
        arena = il.compute_arena_scores(il.load_arena_scores())
        self.ranked = il.score_models(self.intent["requirements"], self.intent["preferred_sizes"],
                                      self.catalog, arena, self.observations)

    def best(self, allowed: set | None = None, prompt_chars: int | None = None) -> dict | None:
        def fits(r) -> bool:
            lim = self.prompt_limit(r["model"])
            return prompt_chars is None or lim is None or prompt_chars <= lim

        for r in self.ranked:
            if (allowed is None or r["model"] in allowed) and not r["disqualified"] and fits(r):
                return r
        for r in self.ranked:                        # nothing fits: least-bad, as v0.1 does
            if allowed is None or r["model"] in allowed:
                return r
        return None

    def top_models(self, n: int, allowed: set | None = None) -> list[str]:
        out = [r["model"] for r in self.ranked if (allowed is None or r["model"] in allowed) and not r["disqualified"]]
        return out[:n]

    def prompt_limit(self, model: str) -> int | None:
        return self.catalog.get(model, {}).get("prompt_max_chars")

    def size_for(self, model: str, aspect: str | None) -> dict:
        entry = self.catalog[model]
        req = entry.get("request", {})
        if aspect and req.get("size_param") == "wxh" and not req.get("size_values") and str(req.get("size_step", "")).isdigit():
            pw, ph = il._wh((self.intent["preferred_sizes"] or ["1024x1024"])[0])
            floor = max(entry.get("size_min_px", 0) or 0, (pw * ph) if pw and ph else 0, 1024 * 1024)   # the intent's own size
            token = exact_wxh(aspect, floor, int(req["size_step"]))
            if token:                                   # this model takes any WxH on its grid: ask for the exact ratio
                w, h = il._wh(token)
                return {"token": token, "aspect_ratio": None, "px": w * h}
        preferred = list(self.intent["preferred_sizes"])
        if aspect:
            w = aspect_to_wxh(aspect)
            if w:
                preferred = [w] + preferred
        return il.pick_size(preferred, entry)

    def price(self, model: str, size: dict, mode: str, ref_support: dict,
              quality: str | None = None) -> tuple[float | None, str]:
        """(usd per image or None, source). None = no known price basis. An explicit quality changes what a
        model bills (one model billed $0.030 or $0.055 per image on `auto`), so it has its own observed price
        and the catalog's flat figure is never used for it."""
        entry = self.catalog[model]
        if quality and quality != "auto":
            key = price_key(model, size["token"], quality, mode)
            return (self.observations[key], "observed") if key in self.observations else (None, "unverified")
        if mode == "reference":
            key = price_key(model, size["token"], None, mode)
            if key in self.observations:
                return self.observations[key], "observed"
            p = (ref_support.get(model) or {}).get("usd_per_image_reference")
            return (p, "measured") if p is not None else (None, "unverified")
        usd, src = il._priced(model, size["token"], size["px"], entry, self.observations)
        return (None, "unverified") if src == "unverified" else (usd, src)


# --------------------------------------------------------------------------- #
def run_preflight(exp_dir, now: float | None = None, advisor: Advisor | None = None,
                  ref_support: dict | None = None, write: bool = True, retry: bool = False,
                  include_unknown: bool = False, only: list | None = None, one_per: str | None = None) -> dict:
    """`only` (row ids) or `one_per` (a dimension name: the first ticked row of each of its values) narrows the batch to
    a subset of the ticked rows, for example one image per creative direction as a calibration. Other ticked rows stay
    ticked and unchanged; the next plain preflight takes them."""
    exp_dir = Path(exp_dir)
    if write:
        with lg.writer_lock(exp_dir, "preflight"):        # a run or another writer holds the ledger: do not write underneath it
            return _run_preflight(exp_dir, now, advisor, ref_support, write, retry, include_unknown, only, one_per)
    return _run_preflight(exp_dir, now, advisor, ref_support, write, retry, include_unknown, only, one_per)


def _run_preflight(exp_dir, now, advisor, ref_support, write, retry, include_unknown, only=None, one_per=None) -> dict:
    ledger = lg.load(exp_dir / "ledger.json")
    quick = not (exp_dir / "plan.json").exists()         # quick mode: no plan, rows carry a verbatim prompt
    plan = ({"brief": ledger["experiment"]["brief"], "intent": ledger["experiment"]["intent"], "dimensions": {}}
            if quick else mx.load_plan(exp_dir / "plan.json"))
    names = ledger["experiment"]["dimensions"]
    ref_support = ref_support if ref_support is not None else load_reference_support()
    adv = advisor or Advisor(plan["intent"])
    catalog_controls = cp.load_catalog()
    wb_path = exp_dir / "experiment.xlsx"
    report = {"issues": [], "warnings": [], "info": [], "workbook_saved": None, "merge_warnings": []}

    if wb_path.exists():
        sheet = wbk.read_workbook(wb_path, names)
        report["workbook_saved"] = time.strftime("%H:%M", time.localtime(sheet["saved_at"]))
        report["merge_warnings"] = lg.merge_workbook(ledger, sheet["rows"]) + [
            f"the workbook has no {d!r} column (renamed or deleted?): the ledger's values for it were kept"
            for d in sheet.get("missing_columns", [])]
    elif not quick:
        report["warnings"].append("no experiment.xlsx; using the ledger as it is")
    saved = wb_path.stat().st_mtime if wb_path.exists() else 0
    for alt in sorted(exp_dir.glob("experiment.refresh-*.xlsx")):
        if alt.stat().st_mtime > saved:
            report["warnings"].append(f"{alt.name} is newer than experiment.xlsx and is NOT read: edits made in it are "
                                      f"ignored. Copy your edits into experiment.xlsx (or replace it) and save.")
    built = ledger["experiment"].get("plan_hash")
    if built and not quick and built != lg.plan_hash(plan):
        report["info"].append("plan.json was changed after the experiment was built (wording, checks or other plan content): the prompts below are rebuilt from the "
                              "changed plan, and rows that already have their image keep it. Adding or removing dimension values or directions needs a new experiment.")
    open_batches = {b["no"]: b["fingerprint"] for b in ledger["batches"] if not b.get("ended_at")}

    strategy = plan.get("model_strategy", "single")
    plan_refs = {r["id"]: r for r in plan.get("references", [])}
    verified = {m for m, v in ref_support.items() if v.get("state") == "verified"}
    # Prefer models with a verified reference call; fall back to documented ones only if none exist.
    ref_models = verified or {m for m, v in ref_support.items() if v.get("state") in REF_OK}
    changed = {m for m, v in ref_support.items() if (v.get("identity") or {}).get("result") == "changed"}
    ref_models = (ref_models - changed) or ref_models        # never auto-pick a model that altered the product
    rows_out, snapshot_rows, prompts = [], [], {}
    unverified, known_total = [], 0.0
    unverified_keys: list[str] = []
    models_used, refs_used = set(), set()
    seen_sigs: dict = {}
    by_model: dict[str, int] = {}

    subset = None
    if only or one_per:
        by_id = {r["id"]: r for r in ledger["rows"]}
        subset = set()
        for rid in only or []:
            if rid not in by_id:
                report["issues"].append({"id": rid, "problems": [f"--only: there is no row {rid}"]})
            else:
                subset.add(rid)
        if one_per:
            if one_per not in names:
                report["issues"].append({"id": "--one-per", "problems": [f"there is no dimension {one_per!r} (the dimensions are {', '.join(names)})"]})
            else:
                taken: set = set()
                for r0 in ledger["rows"]:
                    v = r0["params"].get(one_per)
                    if r0["status"] in ("Removed",) + tuple(SKIP_STATUSES) or not r0.get("selected") or v in taken:
                        continue
                    if r0["status"] == "Completed" and any(a["row_id"] == r0["id"] and a["status"] == "Completed" and matches(a, r0["params"])
                                                           for a in ledger["attempts"]):
                        continue
                    taken.add(v)
                    subset.add(r0["id"])

    imaged: dict = {}                     # (same values, take) -> the row that already has its image
    for r0 in ledger["rows"]:
        if r0["status"] != "Removed" and any(a["row_id"] == r0["id"] and a["status"] == "Completed" and matches(a, r0["params"])
                                             for a in ledger["attempts"]):
            imaged.setdefault((lg.same_image_key(r0), r0.get("take", 1)), r0["id"])

    for row in ledger["rows"]:
        if row["status"] == "Removed":
            continue
        rid = row["id"]
        first_sample = 1
        if retry:                                              # Failed/Partial rows, regardless of Selected
            allowed = ("Failed", "Partial") + (("Unknown",) if include_unknown else ())
            if row["status"] == "Unknown" and not include_unknown:
                report["warnings"].append(f"{rid}: Unknown, not retried (check the console usage log, then "
                                          f"use --include-unknown if you want it generated again)")
            if row["status"] not in allowed:
                continue
        elif not row.get("selected"):
            continue                                           # not in the batch; status unchanged
        if subset is not None and rid not in subset:
            continue                                           # ticked, but outside this subset; stays ticked
        if not retry and row["status"] in SKIP_STATUSES:
            hint = ""
            if row["status"] in ("Queued", "Generating") and open_batches:
                fp_open = list(open_batches.values())[-1]
                hint = (f"; batch {list(open_batches)[-1]} is unfinished: resume it with "
                        f"`image.py run --dir <experiment> --confirm {fp_open}`")
            report["warnings"].append(f"{rid}: status {row['status']}, not included "
                                      f"({'never retried automatically' if row['status'] == 'Unknown' else 'not eligible'}{hint})")
            continue
        problems, notes = [], []
        done_here = [a for a in ledger["attempts"] if a["row_id"] == rid and a["status"] == "Completed"]
        if done_here and not any(matches(a, row["params"]) for a in done_here):
            report["info"].append(f"{rid}: its values were edited after it was generated; this is a new "
                                  f"configuration and its earlier images are kept as they were")
            if not retry and row["status"] in ("Completed", "Partial", "Failed"):
                row["status"] = "Ready"
        elif not retry:
            # What already exists for these values decides, not the row's status (a row restored from Removed, or
            # edited back, starts again as Draft): an image, a sample in flight, a refusal, an Unknown outcome or an
            # image that was billed but not downloaded must never be paid for twice.
            mine0 = [a for a in ledger["attempts"] if a["row_id"] == rid and not a.get("superseded")]
            have_n, why0 = accounted_samples(mine0, include_unknown, row["params"])
            if have_n:
                got = next((a for a in reversed(done_here) if matches(a, row["params"])), None)
                if got:
                    report["warnings"].append(f"{rid}: already has its image, so it is skipped. For another image of the same values add a take "
                                              f"(`image.py add-takes --dir <experiment> --rows <row id>`); the files are in the ledger and the Image column")
                else:
                    reasons = ", ".join(f"{v} {k}" for k, v in why0.items()) or "a sample"
                    hint = (" Run `image.py recover --dir <experiment>` (free) to fetch the image that was billed."
                            if why0.get("billed, not downloaded") else "")
                    report["warnings"].append(f"{rid}: {reasons} already exist for these values, so it is not generated again.{hint}")
                continue

        take = row.get("take", 1)
        if not isinstance(take, int) or isinstance(take, bool) or take < 1:
            problems.append(f"Take must be a whole number from 1 (got {take!r})")
        twin = imaged.get((lg.same_image_key(row), row.get("take", 1)))
        if twin and twin != rid and not retry:
            report["warnings"].append(f"{rid}: same values and Take as {twin}, which already has its image (a duplicate; give it the "
                                      f"next Take number if you want another image of the same values)")
        qty = 1                                                 # a row is exactly one image
        mine = [a for a in ledger["attempts"] if a["row_id"] == rid and not a.get("superseded")]
        first_sample = max((a["sample"] for a in mine), default=0) + 1       # sample ids are never reused for a row
        if retry:                                              # only the samples still missing
            have, why = accounted_samples(mine, include_unknown, row["params"])
            qty = qty - have
            if why.get("billed, not downloaded"):
                report["warnings"].append(f"{rid}: {why['billed, not downloaded']} image(s) were billed but failed to "
                                          f"download: run `image.py recover --dir <experiment>` (free) instead of retrying")
            if qty <= 0:
                skipped = ", ".join(f"{v} {k}" for k, v in why.items() if k != "billed, not downloaded")
                if skipped:
                    report["warnings"].append(f"{rid}: nothing to retry ({skipped}); those are never regenerated "
                                              f"automatically")
                continue
            if why.get("unknown") and not include_unknown:
                report["warnings"].append(f"{rid}: {why['unknown']} Unknown sample(s) are not retried "
                                          f"(check the console usage log; `--include-unknown` to regenerate)")
        missing = [d for d in names if not str(row["params"].get(d, "")).strip()]
        if missing:
            problems.append("missing value for " + ", ".join(missing))

        # references -> mode
        ref_ids, ref_problem = cp.resolve_reference(plan, row)
        if ref_problem:
            problems.append(ref_problem)
        ref_entries = []
        for rf in ref_ids:
            if rf not in plan_refs:
                problems.append(f"unknown reference {rf!r}")
                continue
            p = exp_dir / plan_refs[rf]["file"]
            bad_ref = reference_problem(exp_dir, plan_refs[rf]["file"])
            if bad_ref:
                problems.append(bad_ref)
            else:
                person = bool(plan_refs[rf].get("contains_person") or plan_refs[rf].get("role") == "person")
                if person and not mx.consent_ok(plan):
                    problems.append("this reference shows a person and the plan has no consent_acknowledged "
                                    "(run `image.py acknowledge-person` after the user confirms)")
                ref_entries.append({"id": rf, "file": plan_refs[rf]["file"], "sha256": sha256_file(p),
                                    **({"person": True} if person else {})})
        mode = "reference" if ref_ids else "text"
        ref_role = plan_refs[ref_ids[0]].get("role") if ref_ids and ref_ids[0] in plan_refs else None
        verbatim = row.get("prompt")

        # constraints / custom values (warnings, user may proceed)
        assignment = {d: row["params"].get(d) for d in names}
        if mx.violates(assignment, mx.excludes(plan)):
            report["warnings"].append(f"{rid}: this combination violates an exclude rule")

        comp = ({"prompt": verbatim, "custom": [], "unreliable": []} if verbatim
                else cp.compile_prompt(plan, {d: v for d, v in assignment.items() if v}, mode, catalog_controls, ref_role))
        for dim, val in comp["custom"]:
            report["warnings"].append(f'{rid}: ⚠ custom value "{val}" for {dim} (not in the plan)')
        for dim, val in comp["unreliable"]:
            level = cp.reliability(plan, dim, val, mode, catalog_controls)
            report["info"].append(f'{rid}: {dim} "{val}" is {level} with a reference image')
        for dim, val in comp.get("unvalidated", []):
            report["info"].append(f'{rid}: {dim} "{val}" uses starter wording that has not been tested')

        # model
        row_quality = plan.get("quality") if plan.get("quality") not in (None, "auto") else None
        chosen = row.get("model")
        if not chosen:
            if strategy.startswith("fixed:"):
                chosen = strategy[6:]
            else:
                allowed = ref_models if mode == "reference" else None
                if row_quality:                                   # only models that have this quality setting
                    q_ok = {m for m, e in adv.catalog.items() if row_quality in il._shape(e)["quality_values"]}
                    allowed = q_ok if allowed is None else (allowed & q_ok)
                best = adv.best(allowed, prompt_chars=len(comp["prompt"]))
                chosen = best["model"] if best else None
        if row_quality and chosen in adv.catalog:
            qv = il._shape(adv.catalog[chosen])["quality_values"]
            if row_quality not in qv:
                problems.append(f"{chosen} has no quality setting {row_quality!r}"
                                + (f" (it accepts: {', '.join(qv)})" if qv else " (it has no quality setting at all)"))
        limit = adv.prompt_limit(chosen) if chosen in adv.catalog else None
        if limit and len(comp["prompt"]) > limit:
            problems.append(f"the prompt is {len(comp['prompt']):,} characters but {chosen} accepts at most {limit:,}; "
                            f"choose another model or shorten the prompt")
        if not chosen or chosen not in adv.catalog:
            problems.append(f"model {chosen!r} is not in the catalog")
        elif mode == "reference":
            state = (ref_support.get(chosen) or {}).get("state", "unknown")
            if state == "unsupported":
                problems.append(f"{chosen} does not support reference images")
            elif state in ("documented", "unknown"):
                report["warnings"].append(f"{rid}: reference support for {chosen} is {state} (not yet verified)")
            ident = (ref_support.get(chosen) or {}).get("identity") or {}
            if ident.get("result") == "changed":
                report["warnings"].append(f"{rid}: {chosen} altered the product in a reference test"
                                          f" ({ident.get('note', 'see reference-support.json')}); expect identity drift")

        size = {"token": None, "aspect_ratio": None, "px": il.DEFAULT_PX}
        usd = None
        src = "unverified"
        if chosen in adv.catalog:
            aspect = row["params"].get("aspect")
            size = adv.size_for(chosen, aspect)
            usd, src = adv.price(chosen, size, mode, ref_support, row_quality)
            if aspect:
                got = size.get("aspect_ratio")
                if not got and size["token"] and "x" in str(size["token"]):
                    w, h = il._wh(size["token"])
                    got = ratio_text(w, h) if w and h else None
                if got and got != aspect:
                    notes.append(f"requested {aspect} -> {size['token'] or 'default'} ({got})")
        row_usd = None if usd is None else round(usd * qty, 6)

        sig = (tuple(row["params"].get(d) for d in names), chosen, tuple(ref_ids), row.get("prompt"), take)
        if sig in seen_sigs and not problems:
            report["warnings"].append(f"{rid}: same values, model, reference and Take as {seen_sigs[sig]} (a duplicate; "
                                      f"give it the next Take number if you want another image of the same values)")
        else:
            seen_sigs.setdefault(sig, rid)
        if mode == "reference" and not any("proxy" in i for i in report["info"]):
            report["info"].append("Model choice for reference rows uses the text-to-image Arena scores as a proxy; "
                                  "identity preservation is not measured.")

        if problems:
            if not retry and row["status"] in ("Draft", "Ready"):
                row["status"] = "Draft"               # a row with history (Failed, Partial, Completed) keeps it
            report["issues"].append({"id": rid, "problems": problems})
            continue
        if not retry and row["status"] in ("Draft", "Ready"):
            row["status"] = "Ready"
        prompts[rid] = comp["prompt"]
        models_used.add(chosen)
        refs_used.update(e["file"] for e in ref_entries)
        if row_usd is None:
            unverified.append(rid)
            unverified_keys.append(price_key(chosen, size["token"], row_quality, mode))
        else:
            known_total += row_usd
        for n in notes:
            report["info"].append(f"{rid}: {n}")
        by_model[chosen] = by_model.get(chosen, 0) + qty
        snapshot_rows.append({
            "id": rid, "model": chosen, "prompt": comp["prompt"], "size": size["token"],
            "aspect_ratio": size.get("aspect_ratio"), "qty": qty, "mode": mode,
            "references": ref_entries, "est_usd": row_usd,
            **({"quality": row_quality} if row_quality else {}),
            **({"first_sample": first_sample} if first_sample > 1 else {}),
        })

    # coverage of the selected rows (info)
    try:
        if quick or retry:
            raise mx.PlanError("coverage is not reported for quick mode or a retry")
        dims = mx.dimensions(plan)
        valid = mx.enumerate_valid(dims, mx.excludes(plan))
        sel = [tuple(r["params"].get(d, "") for d in dims) for r in ledger["rows"]
               if r.get("selected") and r["status"] != "Removed"]
        valid_set = set(valid)
        cov = mx.coverage([t for t in sel if t in valid_set], valid)
        report["coverage"] = cov["fraction"]
    except mx.PlanError:
        report["coverage"] = None

    created = time.strftime("%Y-%m-%dT%H:%M:%S", time.localtime(now)) if now else lg.now()
    snapshot = {"created_at": created, "estimated_usd_known": round(known_total, 6),
                "unverified_rows": unverified, "rows": snapshot_rows, "retry": bool(retry)}
    fp = snapshot["fingerprint"] = snapshot_fingerprint(snapshot)
    if write:                                   # write=False: estimate only, no files touched
        if snapshot_rows:
            lg.write_json_atomic(exp_dir / "snapshots" / f"{fp}.json", snapshot)
        lg.save(exp_dir / "ledger.json", ledger)
    wrote_workbook = None
    report.update({
        "ready": len(snapshot_rows), "selected": sum(1 for r in ledger["rows"] if r.get("selected") and r["status"] != "Removed"),
        "images": sum(r["qty"] for r in snapshot_rows), "models": sorted(models_used),
        "references": sorted(refs_used), "known_usd": round(known_total, 6), "unverified_rows": unverified,
        "retry": retry, "by_model": by_model, "indicative": indicative(unverified_keys), "subset": subset is not None,
        "fingerprint": fp if snapshot_rows else None, "snapshot": snapshot,
        "prompts": prompts, "workbook_written": wrote_workbook,
    })
    report["text"] = format_report(report)
    return report


_ROW_MSG = re.compile(r"^(r\d{3,}): (.*)$")


def group_messages(messages: list[str]) -> list[str]:
    """Collapse "r001: text" lines that share the same text into "text: r001, r002 ... (N rows)"."""
    order, groups, plain = [], {}, []
    for m in messages:
        hit = _ROW_MSG.match(m)
        if not hit:
            plain.append(m)
            continue
        rid, text = hit.groups()
        if text not in groups:
            groups[text] = []
            order.append(text)
        groups[text].append(rid)
    out = list(plain)
    for text in order:
        ids = groups[text]
        shown = ", ".join(ids[:5]) + (f", ... ({len(ids)} rows)" if len(ids) > 5 else "")
        out.append(f"{text}: {shown}")
    return out


def format_report(r: dict) -> str:
    L = ["IMAGE LAB RETRY PREFLIGHT (failed or partial rows, missing samples only)" if r.get("retry")
         else "IMAGE LAB PREFLIGHT"]
    if r.get("retry") and not r["ready"] and not r["issues"]:
        L.append("Nothing to retry: no row is Failed or Partial with samples still missing. Rows that are Completed, "
                 "Blocked (the model refused) or Unknown are never retried automatically; Unknown needs "
                 "--include-unknown after you checked the console usage log.")
        for w in group_messages(r["warnings"]):
            L.append(f"  {w}")
        return "\n".join(L)
    if r["workbook_saved"]:
        L.append(f"Workbook saved:     {r['workbook_saved']}   (save it again and re-run if you changed it since)")
    L.append(f"Rows selected:      {r['selected']}   Ready: {r['ready']}   Issues: {len(r['issues'])}"
             + ("   (this batch is a subset of the ticked rows; the others stay ticked)" if r.get("subset") else ""))
    L.append(f"Images:             {r['images']}   Models: {len(r['models'])}   Reference assets: {len(r['references'])}")
    for m, n in sorted(r.get("by_model", {}).items()):
        L.append(f"  {m}: {n} image(s)")
    cost = f"${r['known_usd']:.4f} known"
    if r["unverified_rows"]:
        cost += f"  +  {len(r['unverified_rows'])} row(s) price unverified (not in the total)"
    L.append(f"Estimated cost:     {cost}")
    if r["unverified_rows"]:
        L.append("Price basis:        no observed price yet for these rows; pass --max-usd <N> to `run` (the first image sets the price)")
        ind = r.get("indicative")
        if ind:
            rng = f"${ind['low']:.2f}" if ind["low"] == ind["high"] else f"${ind['low']:.2f} to ${ind['high']:.2f}"
            L.append(f"Indicative:         about {rng} for these {ind['rows']} row(s), from prices measured {ind['measured']} for this model at this quality "
                     f"(a hint, not a quote). A safe limit: --max-usd {ind['max_usd']:.2f}")
    L.append("Balance:            unread (the gateway will confirm)")
    if r.get("coverage") is not None:
        L.append(f"Pairwise coverage:  {r['coverage'] * 100:.0f}% of valid pairs ("
                 + ("all ticked rows, not only this subset)" if r.get("subset") else "selected rows)"))
    if r["issues"]:
        L.append("\nIssues (these rows are not included):")
        for i in r["issues"]:
            L.append(f"  {i['id']}: " + "; ".join(i["problems"]))
    for key, title in (("merge_warnings", "Workbook"), ("warnings", "Warnings"), ("info", "Info")):
        if r[key]:
            L.append(f"\n{title}:")
            L.extend(f"  {w}" for w in group_messages(r[key]))
    if r["fingerprint"]:
        L.append(f"\nFingerprint: {r['fingerprint']}   (snapshots/{r['fingerprint']}.json)")
        n = r["images"]
        L.append(f"Approving this fingerprint authorizes exactly {n} image{'' if n == 1 else 's'} and nothing else; any other ticked row, or any change, "
                 f"needs its own preflight and approval.")
    else:
        L.append("\nNothing is ready to generate.")
    return "\n".join(L)
