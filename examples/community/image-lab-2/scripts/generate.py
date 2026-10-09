"""Generator for Image Lab 2 (design-v2.md section 10). Gateway API only, no loomloom CLI.

    run_batch(exp_dir, fingerprint, ...)   run an approved snapshot: one sample per row (a snapshot row's qty is 1; more images are more rows)

Hard rules this module enforces (design section 2):
  - `run` reads ONLY snapshots/<fingerprint>.json (never the workbook) and refuses unless the
    snapshot hashes to the fingerprint and every reference file still has its recorded hash;
  - the attempt (submit time, request-body hash) is written to the ledger BEFORE the POST and
    the gateway request id is written the moment it returns, before polling;
  - an ambiguous submit (no response, 500/502/504) becomes `Unknown` and is never retried here
    (only a 429 rate-limit rejection is retried, once);
  - one run at a time per experiment (ledger.lock); other writers are refused while it runs;
  - on an authentication or insufficient-balance rejection nothing further is submitted;
  - `max_usd` stops NEW submissions (tasks already in flight can still bill).

The network is behind a small `Gateway` object so tests can drive the whole thing offline.
"""
from __future__ import annotations

import base64
import concurrent.futures as cf
import hashlib
import json
import re
import threading
import time
from pathlib import Path

import image as il
import ledger as lg
import preflight as pf

POLL_SECONDS = 3
POLL_TIMEOUT = 480
FAILED_LIKE = ("FAILED", "CANCELED", "CANCELLED", "EXPIRED", "TIMEOUT", "TIMED_OUT", "ABORTED")
AMBIGUOUS_HTTP = (0, 500, 502, 504)       # the gateway may have accepted the task before the reply was lost
MAX_REFERENCE_BYTES = pf.MAX_REFERENCE_BYTES
MIME = {"png": "image/png", "jpg": "image/jpeg", "webp": "image/webp"}
DONE = ("Completed", "Failed", "Blocked", "Unknown")           # attempt states that need no more work
BLOCKED_WORDS = re.compile(r"moderat|safety|policy|blocked|refus|prohibit|violat|sensitive", re.I)
AUTH_CODES = ("invalid_api_key", "unauthorized", "authentication", "insufficient_balance",
              "insufficient_quota", "insufficient_funds", "payment_required")


class RunRefused(Exception):
    """Raised before any spend when the approved snapshot cannot be honored."""


# --------------------------------------------------------------------------- #
# gateway (real and fake share this interface)
# --------------------------------------------------------------------------- #
class Gateway:
    def __init__(self, tok: str | None = None):
        self.tok = tok or il.token()

    def submit(self, body: dict) -> tuple[int, dict, str]:
        """POST a task. Only a 429 (an explicit rate-limit rejection, before any work) is retried, once.
        A 500/502/504 or a lost connection may have come after the gateway accepted and started billing
        the task, so it is returned as is and the caller records it as Unknown."""
        for attempt in range(2):
            st, resp, err = il._req("POST", "/tasks/generations", self.tok, body, retries=0)
            if st == 429 and attempt == 0:
                time.sleep(il.RETRY_SLEEP_SECONDS)
                continue
            return st, resp, err
        return st, resp, err

    def poll(self, request_id: str) -> tuple[int, dict, str]:
        return il._req("GET", f"/tasks/generations/{request_id}", self.tok)

    def download(self, url: str, dest: Path) -> None:
        il._download(url, dest)


# --------------------------------------------------------------------------- #
# request builder: the one place a request body is assembled (design 11.1)
# --------------------------------------------------------------------------- #
def build_request(srow: dict, catalog: dict, exp_dir: Path, with_image: bool = True) -> dict:
    sh = il._shape(catalog[srow["model"]])
    body = {"model": srow["model"], "prompt": srow["prompt"]}
    if sh["size_param"] in ("wxh", "tier") and srow.get("size") not in (None, "default"):
        body["size"] = srow["size"]
    if sh["aspect_ratio_param"] and srow.get("aspect_ratio") not in (None, "-"):
        body["aspect_ratio"] = srow["aspect_ratio"]
    if sh["watermark_param"]:
        body["watermark"] = False
    if srow.get("quality") and srow["quality"] in sh["quality_values"]:
        body["quality"] = srow["quality"]
    if srow.get("references"):
        ref = srow["references"][0]                       # one reference per request (the verified form)
        as_array = bool((pf.load_reference_support().get(srow["model"]) or {}).get("image_is_array"))
        if with_image:
            f = (exp_dir / ref["file"]).resolve()
            try:
                f.relative_to(Path(exp_dir).resolve())
            except ValueError:
                raise RunRefused(f"reference {ref['file']} is outside the experiment folder")
            raw = f.read_bytes()
            if len(raw) > MAX_REFERENCE_BYTES:
                raise RunRefused(f"reference {ref['file']} is {len(raw) / 1e6:.1f} MB; the limit is "
                                 f"{MAX_REFERENCE_BYTES // (1024 * 1024)} MB")
            if ref.get("sha256") and hashlib.sha256(raw).hexdigest() != ref["sha256"]:
                raise RunRefused(f"reference {ref['file']} changed after approval")
            kind = il._image_kind(raw[:12])
            if kind is None:
                raise RunRefused(f"reference {ref['file']} is not a PNG, JPEG or WebP image")
            uri = f"data:{MIME[kind]};base64," + base64.b64encode(raw).decode()
            body["image"] = [uri] if as_array else uri
        else:
            body["image"] = [f"<reference {ref['file']}>"] if as_array else f"<reference {ref['file']}>"
    return body


def body_hash(body: dict) -> str:
    return hashlib.sha256(json.dumps(body, sort_keys=True).encode()).hexdigest()[:16]


# --------------------------------------------------------------------------- #
# snapshot verification (before any spend)
# --------------------------------------------------------------------------- #
def load_snapshot(exp_dir: Path, fingerprint: str) -> dict:
    p = exp_dir / "snapshots" / f"{fingerprint}.json"
    if not p.exists():
        raise RunRefused(f"no snapshot for fingerprint {fingerprint!r}. Run preflight again and use "
                         f"the fingerprint it prints.")
    snap = json.loads(p.read_text(encoding="utf-8"))
    if pf.snapshot_fingerprint(snap) != fingerprint or snap.get("fingerprint") != fingerprint:
        raise RunRefused("the snapshot does not match its fingerprint (it was modified after approval). "
                         "Run preflight again.")
    for srow in snap["rows"]:
        for ref in srow.get("references", []):
            f = exp_dir / ref["file"]
            if not f.exists():
                raise RunRefused(f"reference {ref['file']} is missing; run preflight again.")
            if pf.sha256_file(f) != ref["sha256"]:
                raise RunRefused(f"reference {ref['file']} changed after approval; run preflight again.")
    if any(ref.get("person") for srow in snap["rows"] for ref in srow.get("references", [])):
        plan_p = exp_dir / "plan.json"
        plan = json.loads(plan_p.read_text(encoding="utf-8")) if plan_p.exists() else {}
        if not (isinstance(plan.get("consent_acknowledged"), str) and plan["consent_acknowledged"].strip()):
            raise RunRefused("this snapshot uses a photo of a person and the plan has no consent_acknowledged. "
                             "Show the person notice, record the user's confirmation with "
                             "`image.py acknowledge-person`, and preflight again.")
    return snap


# --------------------------------------------------------------------------- #
# ledger helpers
# --------------------------------------------------------------------------- #
def batch_prompt(snap: dict) -> str:
    prompts = {r["prompt"] for r in snap["rows"]}
    return prompts.pop() if len(prompts) == 1 else "(the prompt varies by row; see experiment.xlsx)"


def settle_extension(dest: Path) -> Path:
    """Rename a downloaded image to the extension its bytes say (Seedream sends JPEG)."""
    kind = il._image_kind(Path(dest).read_bytes()[:12])
    if kind and kind != "png":
        final = Path(dest).with_suffix("." + kind)
        Path(dest).replace(final)
        return final
    return Path(dest)


def recover_downloads(exp_dir, gateway=None, log=print, record_prices: bool = True) -> dict:
    """Free recovery: samples that were generated and billed but whose download failed still have
    their signed URL in the ledger. Fetch them again (no gateway generation call, no spend) and
    mark them Completed. URLs expire, so do this soon after a run."""
    exp = Path(exp_dir)
    with lg.writer_lock(exp, "recover"):
        return _recover_downloads(exp, gateway, log, record_prices)


def _recover_downloads(exp, gateway, log, record_prices) -> dict:
    ledger = lg.load(exp / "ledger.json")
    gw = gateway or Gateway()
    fixed, failed, touched = [], [], set()
    for a in ledger["attempts"]:
        if a["status"] != "Failed" or not a.get("url") or a.get("file"):
            continue
        dest = exp / f"round-{a['batch']}" / f"{a['sample_id']}.png"
        dest.parent.mkdir(parents=True, exist_ok=True)
        try:
            gw.download(a["url"], dest)
            dest = settle_extension(dest)
        except Exception as e:
            failed.append((a["sample_id"], str(e)))
            continue
        a.update(status="Completed", file=f"round-{a['batch']}/{dest.name}", size_actual=il._png_size(str(dest)),
                 error=None, note="recovered from the signed URL without resubmitting")
        fixed.append(a["sample_id"])
        touched.add((a["batch"], a["sample_id"]))
    for b in ledger["batches"]:
        if not any(bn == b["no"] for bn, _ in touched):
            continue
        finalize_rows(ledger, b["no"])
        if all(x["status"] in DONE for x in batch_attempts(ledger, b["no"])) and not b.get("ended_at"):
            b["ended_at"] = lg.now()
        b["actual_usd"] = round(sum(float(x.get("cost") or 0) for x in batch_attempts(ledger, b["no"])), 6)
    lg.save(exp / "ledger.json", ledger)
    lg.write_json_atomic(exp / "session.json", session_export(ledger))
    if fixed and record_prices:
        remember_observations([a for a in ledger["attempts"] if (a["batch"], a["sample_id"]) in touched], log)
    for sid in fixed:
        log(f"recovered {sid}")
    for sid, e in failed:
        log(f"could not recover {sid}: {e}")
    return {"recovered": fixed, "failed": failed}


def sample_id(row_id: str, k: int) -> str:
    """A row is one image, so its first attempt is just the row id (r001). A second attempt (the row was
    retried or its values were edited) is r001-2: ids and files are never reused."""
    return row_id if k == 1 else f"{row_id}-{k}"


def find_batch(ledger: dict, fingerprint: str) -> dict | None:
    for b in reversed(ledger["batches"]):
        if b["fingerprint"] == fingerprint and not b.get("ended_at"):
            return b
    return None


def batch_spent(ledger: dict, batch_no: int) -> bool:
    """True if anything in the batch was accepted by the gateway or may have been billed: a request id,
    a cost, or an Unknown outcome. A batch where every sample was rejected up front (a wrong API key, a
    model that is down) spent nothing, so its snapshot can simply be run again."""
    return any(a.get("request_id") or float(a.get("cost") or 0) > 0 or a["status"] == "Unknown"
               for a in batch_attempts(ledger, batch_no))


def last_finished_batch(ledger: dict, fingerprint: str) -> dict | None:
    for b in reversed(ledger["batches"]):
        if b["fingerprint"] == fingerprint and b.get("ended_at"):
            return b
    return None


def batch_attempts(ledger: dict, batch_no: int) -> list[dict]:
    return [a for a in ledger["attempts"] if a["batch"] == batch_no]


def finalize_rows(ledger: dict, batch_no: int) -> None:
    """Derive each row's status from its samples (design 10.2). The batch decides whether the row
    is still in flight; completed samples from earlier batches count toward the row's Qty, so a
    retry that fills in the missing samples makes the row Completed."""
    rows = lg.by_id(ledger)
    per_row: dict[str, list[dict]] = {}
    for a in batch_attempts(ledger, batch_no):
        per_row.setdefault(a["row_id"], []).append(a)
    for rid, atts in per_row.items():
        row = rows.get(rid)
        if row is None or row["status"] == "Removed":
            continue
        n = {s: sum(1 for a in atts if a["status"] == s) for s in
             ("Completed", "Failed", "Blocked", "Unknown", "Pending", "Submitting", "Running")}
        open_ = n["Pending"] + n["Submitting"] + n["Running"]
        total_completed = sum(1 for a in ledger["attempts"] if a["row_id"] == rid and a["status"] == "Completed"
                              and a.get("params", row["params"]) == row["params"])   # edited since: it no longer counts
        if open_:
            started = n["Submitting"] or n["Running"] or n["Completed"] or n["Failed"] or n["Blocked"] or n["Unknown"]
            row["status"] = "Generating" if started else "Queued"
        elif n["Completed"] == len(atts) or total_completed >= 1:
            row["status"] = "Completed"
        elif n["Completed"] or total_completed:
            row["status"] = "Partial"
        elif n["Unknown"]:
            row["status"] = "Unknown"
        elif n["Blocked"] == len(atts):
            row["status"] = "Blocked"
        else:
            row["status"] = "Failed"
        lg.record_used_model(ledger, row)


# --------------------------------------------------------------------------- #
class Run:
    """One batch execution. Threads share `self.lock`; every ledger mutation saves atomically."""

    def __init__(self, exp_dir, snapshot: dict, ledger: dict, batch: dict, gateway, catalog: dict,
                 concurrency: int = 6, max_usd: float | None = None, progress_file=None,
                 poll_seconds: float = POLL_SECONDS, poll_timeout: float = POLL_TIMEOUT,
                 record_prices: bool = True, log=print):
        self.exp = Path(exp_dir)
        self.snap, self.ledger, self.batch = snapshot, ledger, batch
        self.gw, self.catalog = gateway, catalog
        self.concurrency, self.max_usd = concurrency, max_usd
        self.progress_file = Path(progress_file) if progress_file else None
        self.poll_seconds, self.poll_timeout = poll_seconds, poll_timeout
        self.record_prices, self.log = record_prices, log
        self.lock = threading.RLock()
        self.cond = threading.Condition(self.lock)
        self.price_known: dict[str, float] = {}      # model -> a real billed price seen in this batch
        self.price_max: dict[str, float] = {}        # model -> the highest real cost seen so far in this batch
        self.probing: set[str] = set()               # models whose first unpriced sample is in flight
        self.heartbeat = None                        # a ledger.Lock to touch while running
        self.stop_submitting = threading.Event()
        self.stop_reason: str | None = None
        self.out_dir = self.exp / f"round-{batch['no']}"
        self.srows = {r["id"]: r for r in snapshot["rows"]}

    # -- persistence ------------------------------------------------------- #
    def save(self) -> None:
        with self.lock:
            lg.save(self.exp / "ledger.json", self.ledger)

    def update(self, att: dict, **kw) -> None:
        with self.lock:
            att.update(kw)
            finalize_rows(self.ledger, self.batch["no"])
            self.save()
            if att["status"] == "Completed" and att.get("cost") is not None:
                self.price_max[att["model"]] = max(self.price_max.get(att["model"], 0.0), float(att["cost"]))
            if att["status"] in DONE and att.get("probe"):
                if att["status"] == "Completed" and att.get("cost") is not None:
                    self.price_known[att["model"]] = float(att["cost"])
                self.probing.discard(att["model"])
                self.cond.notify_all()

    # -- budget ------------------------------------------------------------ #
    def committed_usd(self) -> float:
        """Billed so far plus the estimate for every sample in flight (so a burst of parallel
        submissions cannot overshoot the guard the way counting completed costs only would)."""
        tot = 0.0
        for a in batch_attempts(self.ledger, self.batch["no"]):
            cost = float(a.get("cost") or 0)
            if a["status"] in ("Submitting", "Running", "Unknown"):
                est = a.get("est_usd")
                if est is None:
                    est = self.price_known.get(a["model"], 0.0)
                # once real costs are seen they beat the plan's estimate: a model that bills 4x what the
                # catalog said must not keep being counted at the catalog price
                tot += max(cost, float(est), self.price_max.get(a["model"], 0.0))
            else:
                tot += cost                      # Completed, and also Failed ones that were billed (a failed download)
        return tot

    # -- one sample -------------------------------------------------------- #
    def work(self, att: dict) -> None:
        if att["status"] in DONE:
            return
        if att["status"] == "Submitting":                  # crashed between "about to POST" and a reply
            self.update(att, status="Unknown", error="interrupted while submitting; the gateway may or may not "
                        "have accepted it. Check the console usage log against submitted_at and request_hash.")
            return
        if att["status"] == "Pending":
            if not self.submit(att):
                return
        self.poll_until_done(att)

    def submit(self, att: dict) -> bool:
        srow = self.srows[att["row_id"]]
        with self.lock:
            if self.stop_submitting.is_set():
                return False
            est = att.get("est_usd")
            while est is None and att["model"] not in self.price_known and att["model"] in self.probing \
                    and not self.stop_submitting.is_set():
                self.cond.wait(1.0)                 # one unpriced sample per model goes first, so its real
            if self.stop_submitting.is_set():       # price is known before the rest are committed
                return False
            if est is None:
                est = self.price_known.get(att["model"])
            if self.max_usd is not None:
                committed = self.committed_usd()
                if est is None and committed >= self.max_usd - 1e-9:      # no price basis: stop once the limit is reached
                    self.stop_submitting.set()
                    self.stop_reason = (f"budget guard: ${committed:.4f} already billed reaches --max-usd "
                                        f"${self.max_usd:g} (the next sample has no verified price)")
                    return False
                if est is not None:
                    est = max(est, self.price_max.get(att["model"], 0.0))
                if est is not None and committed + est > self.max_usd + 1e-9:
                    self.stop_submitting.set()
                    self.stop_reason = (f"budget guard: ${committed:.4f} committed + ${est:.4f} next "
                                        f"would exceed --max-usd ${self.max_usd:g}")
                    return False
            try:
                body = build_request(srow, self.catalog, self.exp)
            except RunRefused as e:
                self.update(att, status="Failed", error=f"not submitted: {e}")
                self.stop_submitting.set()
                self.stop_reason = f"a reference file problem stopped the run before this sample: {e}"
                return False
            if att.get("est_usd") is None and att["model"] not in self.price_known:
                self.probing.add(att["model"])
                att["probe"] = True
            att["request_hash"] = body_hash(body)
            att["submitted_at"] = lg.now()
            att["submitted_ts"] = time.time()
            att["status"] = "Submitting"
            finalize_rows(self.ledger, self.batch["no"])
            self.save()                                    # BEFORE the POST
        st, resp, err = self.gw.submit(body)
        data = (resp or {}).get("data") or {}
        rid = data.get("request_id")
        code = str(((resp or {}).get("error") or {}).get("code", "")).lower()
        msg = ((resp or {}).get("error") or {}).get("message") or err or f"HTTP {st}"
        if rid:
            self.update(att, request_id=rid, status="Running")      # request id first, before polling
            return True
        if st in (401, 402, 403) or any(c in code for c in AUTH_CODES):
            self.update(att, status="Failed", error=f"rejected, not charged: {msg}")
            with self.lock:
                if not self.stop_submitting.is_set():
                    self.stop_submitting.set()
                    self.stop_reason = f"gateway rejected the request (HTTP {st}): {msg}. Nothing further was submitted."
            return False
        if st == 503 or code == "service_unavailable":
            self.update(att, status="Failed", error=f"model unavailable, not charged: {msg}")
            return False
        if st in AMBIGUOUS_HTTP:                            # no usable reply: it may have been accepted
            self.update(att, status="Unknown", error=f"no confirmed reply to the submit ({msg}); it may or may "
                        f"not be billed. Never retried automatically.")
            return False
        self.update(att, status="Failed", error=f"submit rejected, not charged: {msg}")
        return False

    def poll_until_done(self, att: dict) -> None:
        deadline = time.time() + self.poll_timeout
        while time.time() < deadline:
            _, r, err = self.gw.poll(att["request_id"])
            if err or not r:                                # transient; try again
                time.sleep(self.poll_seconds)
                continue
            d = r.get("data") or {}
            status = d.get("status", "")
            kw = {"progress": d.get("progress", att.get("progress"))}
            if d.get("cost") is not None:
                kw["cost"] = float(d["cost"])
            if isinstance(d.get("start_time"), (int, float)) and isinstance(d.get("finish_time"), (int, float)):
                kw["seconds"] = round(d["finish_time"] - d["start_time"], 1)
            if status == "COMPLETED":
                urls = (d.get("data") or {}).get("image_urls") or []
                if not urls:
                    self.update(att, status="Failed", error="completed with no image url", **kw)
                    return
                dest = self.out_dir / f"{att['sample_id']}.png"
                dest.parent.mkdir(parents=True, exist_ok=True)
                try:
                    self.gw.download(urls[0], dest)
                    dest = settle_extension(dest)
                except Exception as e:                      # already paid for: keep the url; `recover` re-fetches free
                    self.update(att, status="Failed", error=f"image generated but download failed: {e}",
                                url=urls[0], **kw)
                    return
                self.update(att, status="Completed", file=f"round-{self.batch['no']}/{dest.name}",
                            size_actual=il._png_size(str(dest)), url=urls[0], **kw)
                return
            if status in FAILED_LIKE:
                reason = d.get("fail_reason") or status.lower()
                self.update(att, status="Blocked" if BLOCKED_WORDS.search(reason) else "Failed",
                            error=reason, **kw)
                return
            if kw["progress"] != att.get("progress"):
                self.update(att, **kw)
            time.sleep(self.poll_seconds)
        # still running after the timeout: leave it Running (resumable) and say so
        self.update(att, note=f"still running after {self.poll_timeout:.0f}s; run again with the same "
                              f"fingerprint to resume polling")

    # -- the batch --------------------------------------------------------- #
    def execute(self) -> dict:
        atts = batch_attempts(self.ledger, self.batch["no"])
        with cf.ThreadPoolExecutor(max_workers=self.concurrency) as pool:
            futures = [pool.submit(self.safe_work, a) for a in atts]
            last = ""
            while not all(f.done() for f in futures):
                time.sleep(min(self.poll_seconds, 1.0))
                if self.heartbeat:
                    self.heartbeat.touch()
                frame = self.tree()
                self.write_progress("generating")
                if frame != last:
                    self.log(frame)
                    last = frame
        return self.finish()

    def safe_work(self, att: dict) -> None:
        """A worker that crashes must not abandon the batch: record what is known and carry on, so
        finish() still runs (ended_at, run.json, observed prices) and no row is left Queued."""
        try:
            self.work(att)
        except Exception as e:                                # noqa: BLE001
            self.log(f"{att['sample_id']}: internal error {type(e).__name__}: {e}")
            with self.lock:
                if att["status"] == "Submitting":
                    att.update(status="Unknown", error=f"internal error after the submit was sent "
                               f"({type(e).__name__}: {e}); it may or may not be billed. Never retried automatically.")
                elif att["status"] == "Pending":
                    att.update(status="Failed", error=f"internal error before submitting: {type(e).__name__}: {e}")
                else:
                    att["note"] = f"internal error while polling: {type(e).__name__}: {e}; run again to resume"
                if att.get("probe"):
                    self.probing.discard(att["model"])
                    self.cond.notify_all()
                finalize_rows(self.ledger, self.batch["no"])
                try:
                    self.save()
                except OSError:
                    pass

    def tree(self) -> str:
        atts = batch_attempts(self.ledger, self.batch["no"])
        done = sum(1 for a in atts if a["status"] == "Completed")
        lines = [f"GENERATE  {done}/{len(atts)} done   ${sum(float(a.get('cost') or 0) for a in atts):.4f} so far"]
        for a in atts:
            extra = f"  {a['progress']}" if a["status"] == "Running" and a.get("progress") else ""
            lines.append(f"  {a['sample_id']:<8} {a['model'].split('/', 1)[-1]:<26} {a['status']}{extra}")
        return "\n".join(lines)

    def write_progress(self, phase: str) -> None:
        if not self.progress_file:
            return
        atts = batch_attempts(self.ledger, self.batch["no"])
        rec = {"phase": phase, "batch": self.batch["no"], "total": len(atts),
               "done": sum(1 for a in atts if a["status"] == "Completed"),
               "failed": sum(1 for a in atts if a["status"] in ("Failed", "Blocked")),
               "unknown": sum(1 for a in atts if a["status"] == "Unknown"),
               "actual_usd_so_far": round(sum(float(a.get("cost") or 0) for a in atts), 6),
               "updated_at": round(time.time(), 1),
               "samples": [{"id": a["sample_id"], "model": a["model"], "status": a["status"],
                            "progress": a.get("progress"), "cost_usd": a.get("cost")} for a in atts]}
        try:
            lg.write_json_atomic(self.progress_file, rec)
        except OSError:
            pass

    def finish(self) -> dict:
        with self.lock:
            atts = batch_attempts(self.ledger, self.batch["no"])
            unresolved = [a for a in atts if a["status"] in ("Pending", "Running", "Submitting")]
            actual = round(sum(float(a.get("cost") or 0) for a in atts), 6)
            self.batch["actual_usd"] = actual
            if not unresolved:
                self.batch["ended_at"] = lg.now()
            finalize_rows(self.ledger, self.batch["no"])
            rows = lg.by_id(self.ledger)
            for a in atts:                                  # a paused batch must not leave rows "Queued"/"Generating"
                row = rows.get(a["row_id"])
                if a["status"] != "Pending" or not row:
                    continue
                busy = any(x["row_id"] == a["row_id"] and x["status"] in ("Submitting", "Running") for x in atts)
                if row["status"] == "Queued":
                    row["status"] = "Ready"
                elif row["status"] == "Generating" and not busy:
                    row["status"] = "Partial"
            self.save()
        if self.record_prices:
            self.remember_prices(atts)
        self.write_progress("done" if not unresolved else "paused")
        result = self.report(atts, unresolved, actual)
        self.write_derived(result)
        return result

    def remember_prices(self, atts: list[dict]) -> None:
        remember_observations(atts, self.log)

    def report(self, atts, unresolved, actual) -> dict:
        by_status: dict[str, int] = {}
        for a in atts:
            by_status[a["status"]] = by_status.get(a["status"], 0) + 1
        by_model: dict[str, float] = {}
        for a in atts:
            by_model[a["model"]] = round(by_model.get(a["model"], 0) + float(a.get("cost") or 0), 6)
        return {
            "batch": self.batch["no"], "fingerprint": self.batch["fingerprint"], "out_dir": str(self.out_dir),
            "estimated_usd_known": self.batch["estimated_usd"], "actual_usd": actual,
            "samples": len(atts), "by_status": by_status, "by_model_usd": by_model,
            "stopped": self.stop_reason, "max_usd": self.max_usd,
            "unfinished": [a["sample_id"] for a in unresolved],
            "unknown": [{"sample": a["sample_id"], "submitted_at": a.get("submitted_at"),
                         "request_hash": a.get("request_hash"), "error": a.get("error")}
                        for a in atts if a["status"] == "Unknown"],
            "failed": [{"sample": a["sample_id"], "status": a["status"], "error": a.get("error")}
                       for a in atts if a["status"] in ("Failed", "Blocked")],
            "images": [a["file"] for a in atts if a.get("file")],
        }

    def write_derived(self, result: dict) -> None:
        """run.json and session.json are exports for humans and for the v0.1 page builder
        (build-exploration-page.py reads `alternatives` and the session's `rounds`); the engine
        never reads them back. A batch is a round."""
        self.out_dir.mkdir(parents=True, exist_ok=True)
        atts = batch_attempts(self.ledger, self.batch["no"])
        try:
            lg.write_json_atomic(self.out_dir / "run.json", {
                **result, "attempts": atts, "round": self.batch["no"],
                "prompt": self.batch.get("prompt"), "intent": self.batch.get("intent"),
                "estimated_usd": self.batch["estimated_usd"],
                "alternatives": self.alternatives(atts)})
            lg.write_json_atomic(self.exp / "session.json", session_export(self.ledger))
        except OSError:
            pass

    def alternatives(self, atts: list[dict]) -> list[dict]:
        out = []
        for i, a in enumerate(atts, start=1):
            entry = self.catalog.get(a["model"], {})
            out.append({
                "label": a["sample_id"].upper(), "index": i, "of": len(atts), "model": a["model"],
                "model_label": entry.get("label") or a["model"], "model_url": entry.get("url") or None,
                "requested_size": a.get("size") or "default",
                "requested_aspect_ratio": self.srows.get(a["row_id"], {}).get("aspect_ratio"),
                "actual_size": a.get("size_actual"), "cost_usd": round(float(a.get("cost") or 0), 6),
                "seconds": a.get("seconds"),
                "status": "COMPLETED" if a["status"] == "Completed" else "FAILED",
                "file": Path(a["file"]).name if a.get("file") else None,
                "note": a.get("error") or a.get("note")})
        return out


def session_export(ledger: dict) -> dict:
    """session.json in the v0.1 shape ({rounds, cumulative_usd}) plus the ledger's batches."""
    rounds = []
    for b in ledger["batches"]:
        cost = round(sum(float(a.get("cost") or 0) for a in batch_attempts(ledger, b["no"])), 6)
        rounds.append({"round": b["no"], "prompt": b.get("prompt"), "intent": b.get("intent"),
                       "out_dir": f"round-{b['no']}", "actual_usd": cost,
                       "estimated_usd": b["estimated_usd"], "created_at": b["started_at"]})
    return {"rounds": rounds, "batches": ledger["batches"],
            "cumulative_usd": round(sum(r["actual_usd"] for r in rounds), 6)}


def remember_observations(atts: list[dict], log=print) -> None:
    """Record what completed samples actually cost (price cache) and promote reference states."""
    p = il.REFS / il.PRICE_OBSERVATIONS_FILE
    try:
        obs = json.loads(p.read_text(encoding="utf-8")) if p.exists() else {}
    except (json.JSONDecodeError, OSError):
        obs = {}
    changed = False
    by_key: dict[str, list[float]] = {}
    for a in atts:
        if a["status"] != "Completed" or a.get("cost") is None:
            continue
        key = (f"{a['model']}|{a.get('size') or 'default'}" + (f"|q={a['quality']}" if a.get("quality") else "")
               + ("|reference" if a.get("mode") == "reference" else ""))
        by_key.setdefault(key, []).append(float(a["cost"]))
    for key, costs in by_key.items():                     # the mean of this batch, not the last image: one model billed
        obs[key] = round(sum(costs) / len(costs), 6)      # $0.030 and $0.055 for the same request shape (2026-10-07)
        changed = True
    if changed:
        try:
            p.write_text(json.dumps(obs, indent=2, sort_keys=True), encoding="utf-8")
        except OSError as e:
            log(f"note: could not write {p}: {e}")
    for model in sorted({a["model"] for a in atts if a["status"] == "Completed" and a.get("mode") == "reference"}):
        cost = next((a["cost"] for a in atts if a["model"] == model and a["status"] == "Completed"
                     and a.get("mode") == "reference" and a.get("cost") is not None), None)
        try:
            if pf.promote_reference(model, cost):
                log(f"note: {model} is now recorded as verified for reference images "
                         f"(a real reference call succeeded).")
        except (OSError, ValueError) as e:
            log(f"note: could not update reference support: {e}")


# --------------------------------------------------------------------------- #
def run_batch(exp_dir, fingerprint: str, gateway=None, concurrency: int = 6, max_usd: float | None = None,
              progress_file=None, again: bool = False, only_rows: list[str] | None = None,
              poll_seconds: float = POLL_SECONDS, poll_timeout: float = POLL_TIMEOUT,
              record_prices: bool = True, log=print) -> dict:
    """Run (or resume) the approved snapshot. Raises RunRefused before any spend if it cannot."""
    exp = Path(exp_dir)
    snap = load_snapshot(exp, fingerprint)
    ledger = lg.load(exp / "ledger.json")
    catalog = il.load_model_catalog()
    for srow in snap["rows"]:
        if srow["model"] not in catalog:
            raise RunRefused(f"{srow['id']}: model {srow['model']} is not in the catalog. Run preflight again.")

    if max_usd is None:
        if snap["unverified_rows"]:
            raise RunRefused(f"{len(snap['unverified_rows'])} row(s) have no verified price, so there is no "
                             f"estimate to derive a spending limit from. Pass --max-usd explicitly.")
        max_usd = round(max(snap["estimated_usd_known"] * 1.25, snap["estimated_usd_known"] + 0.02), 4)

    gw = gateway or Gateway()                       # resolve the token BEFORE anything is written
    lock = lg.Lock(exp, f"run {fingerprint}").acquire()
    try:
        return _run_locked(exp, snap, ledger, catalog, gw, fingerprint, concurrency, max_usd, progress_file, again,
                           only_rows, poll_seconds, poll_timeout, record_prices, log, lock)
    finally:
        lock.release()


def _run_locked(exp, snap, ledger, catalog, gw, fingerprint, concurrency, max_usd, progress_file, again, only_rows,
                poll_seconds, poll_timeout, record_prices, log, lock) -> dict:
    ledger = lg.load(exp / "ledger.json")            # re-read under the lock: nothing wrote since we looked
    batch = find_batch(ledger, fingerprint)
    if batch is None:
        done = last_finished_batch(ledger, fingerprint)
        if done and not again and not batch_spent(ledger, done["no"]):
            log(f"Batch {done['no']} of this snapshot was rejected before anything was billed (for example a wrong "
                f"API key), so it is run again as a new batch.")
            for a in batch_attempts(ledger, done["no"]):
                a["superseded"] = True              # kept in the ledger, but it holds no sample number and is not shown
            done = None
        if done and not again:
            raise RunRefused(f"this snapshot already ran as batch {done['no']} (${done.get('actual_usd', 0):.4f}). "
                             f"Running it again is a new spend: re-run with --again to confirm that.")
        no = max((b["no"] for b in ledger["batches"]), default=0) + 1
        batch = {"no": no, "fingerprint": fingerprint, "approved_at": lg.now(),
                 "models": sorted({r["model"] for r in snap["rows"]}),
                 "estimated_usd": snap["estimated_usd_known"], "max_usd": max_usd,
                 "prompt": batch_prompt(snap), "intent": ledger["experiment"].get("intent"),
                 "started_at": lg.now(), "ended_at": None, "actual_usd": 0.0}
        ledger["batches"].append(batch)
        rows = lg.by_id(ledger)
        for srow in snap["rows"]:
            if only_rows and srow["id"] not in only_rows:
                continue
            used = max((a["sample"] for a in ledger["attempts"]
                        if a["row_id"] == srow["id"] and not a.get("superseded")), default=0)
            first = max(srow.get("first_sample", 1), used + 1)       # sample ids are never reused for a row
            for k in range(first, first + srow["qty"]):
                per = None if srow["est_usd"] is None else round(srow["est_usd"] / srow["qty"], 6)
                ledger["attempts"].append({
                    "batch": no, "row_id": srow["id"], "sample": k, "sample_id": sample_id(srow["id"], k),
                    "params": dict(rows[srow["id"]]["params"]) if srow["id"] in rows else {},
                    "model": srow["model"], "mode": srow["mode"], "size": srow.get("size"), "est_usd": per,
                    "quality": srow.get("quality"),
                    "status": "Pending", "request_id": None, "cost": None, "file": None, "error": None})
            if srow["id"] in rows:
                rows[srow["id"]]["status"] = "Queued"
        lg.save(exp / "ledger.json", ledger)
    else:
        log(f"Resuming batch {batch['no']}: finished samples are skipped, running ones are polled again.")
        for a in batch_attempts(ledger, batch["no"]):           # rejected up front (a wrong key, a model that was down):
            if a["status"] == "Failed" and not a.get("request_id") and not a.get("cost") and                     any(k in str(a.get("error")) for k in ("not charged", "not submitted")):
                a.update(status="Pending", error=None, note="re-submitted after being rejected before billing")
        lg.save(exp / "ledger.json", ledger)

    run = Run(exp, snap, ledger, batch, gw, catalog, concurrency, max_usd, progress_file,
              poll_seconds, poll_timeout, record_prices, log)
    run.heartbeat = lock
    return run.execute()


def format_result(r: dict) -> str:
    L = [f"BATCH {r['batch']} RESULT  (in {r['out_dir']})",
         f"Samples: {r['samples']}   " + "   ".join(f"{k}: {v}" for k, v in sorted(r["by_status"].items())),
         f"Actual cost: ${r['actual_usd']:.4f}   (estimated ${r['estimated_usd_known']:.4f} known)"]
    for m, usd in sorted(r["by_model_usd"].items()):
        L.append(f"  {m}: ${usd:.4f}")
    if r.get("max_usd") is not None:
        over = r["actual_usd"] - r["max_usd"]
        L.append(f"Spending limit: --max-usd ${r['max_usd']:g} limits NEW submissions; it is not a hard cap, because requests already in flight when it is "
                 f"reached can still bill." + (f" Actual cost is ${over:.4f} over it." if over > 1e-9 else ""))
    if r["stopped"]:
        L.append(f"\nSTOPPED: {r['stopped']}")
    if r["unfinished"]:
        L.append(f"\nNot finished: {', '.join(r['unfinished'])}. Run again with the same fingerprint to resume.")
    for u in r["unknown"]:
        L.append(f"\nUNKNOWN {u['sample']}: {u['error']}  (submitted {u['submitted_at']}, request hash {u['request_hash']})")
    for f in r["failed"]:
        L.append(f"  {f['sample']} {f['status']}: {f['error']}")
    return "\n".join(L)
