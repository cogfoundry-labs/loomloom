"""Experiment ledger for Image Lab 2 (design-v2.md section 6.4): the system of record.

One JSON file: experiment metadata, rows, batches, attempts. Written atomically (temp
file + rename). The workbook is only a view; user-owned cells are merged into the ledger
by `merge_workbook`, system-owned cells are never read back.

Row statuses (design 10.2): Draft, Ready, Queued, Generating, Completed, Partial, Failed,
Blocked, Unknown, Removed. Selection (`selected`) is a separate property.
"""
from __future__ import annotations

import contextlib
import json
import os
import threading
import time
from pathlib import Path

SCHEMA_VERSION = 1
STATUSES = ("Draft", "Ready", "Queued", "Generating", "Completed", "Partial", "Failed",
            "Blocked", "Unknown", "Removed")
USER_FIELDS = ("selected", "model", "take", "reference", "notes")


def now() -> str:
    return time.strftime("%Y-%m-%dT%H:%M:%S")


# --------------------------------------------------------------------------- #
# io
# --------------------------------------------------------------------------- #
def write_json_atomic(path, data) -> None:
    """Write via a temp file and rename. On Windows the rename can fail for a moment when another
    process (a virus scanner, the indexer, a reader) has the destination open, so retry briefly;
    the temp name is unique per process and thread so two writers never share it."""
    path = Path(path)
    path.parent.mkdir(parents=True, exist_ok=True)
    tmp = path.with_name(f"{path.name}.{os.getpid()}.{threading.get_ident()}.tmp")
    tmp.write_text(json.dumps(data, indent=2, ensure_ascii=False), encoding="utf-8")
    try:
        for attempt in range(20):
            try:
                os.replace(tmp, path)
                return
            except PermissionError:
                if attempt == 19:
                    raise
                time.sleep(0.05 * (attempt + 1))
    finally:
        if tmp.exists():
            try:
                tmp.unlink()
            except OSError:
                pass


def save(path, ledger: dict) -> None:
    write_json_atomic(path, ledger)


def load(path) -> dict:
    ledger = json.loads(Path(path).read_text(encoding="utf-8"))
    if ledger.get("schema_version") != SCHEMA_VERSION:
        raise ValueError(f"ledger schema_version {ledger.get('schema_version')!r} is not supported")
    migrate_qty(ledger)
    return ledger


def migrate_qty(ledger: dict) -> None:
    """Older ledgers gave a row a Qty (several images per row). A row is now exactly one image and further
    images of the same configuration are further rows with the next Take. A legacy row keeps all its attempts;
    if it has none yet and asked for N images, N-1 extra take rows are added so the request is not lost."""
    if any("qty" in r for r in ledger["rows"]):
        # an older workbook does not contain the take rows added below: remember which rows it can know about, so
        # the first merge does not mark the new ones Removed
        ledger["experiment"].setdefault("main_workbook_rows", [r["id"] for r in ledger["rows"]])
    for row in list(ledger["rows"]):
        if "qty" not in row:
            continue
        qty = row.pop("qty")
        row.setdefault("take", 1)
        started = any(a["row_id"] == row["id"] for a in ledger["attempts"])
        if isinstance(qty, int) and not isinstance(qty, bool) and qty > 1 and not started and row["status"] != "Removed":
            rest = {k: v for k, v in row.items() if k not in ("id", "params", "status", "take", "notes", "extras", "selected")}
            for k in range(2, qty + 1):
                add_row(ledger, row["params"], selected=row.get("selected", True), take=k, **rest)


# --------------------------------------------------------------------------- #
# one writer at a time
#
# `run` keeps the whole ledger in memory and rewrites the file on every update, so any other
# command that loads, changes and saves the same file (preflight, refresh, recover, quick) would
# be overwritten, or would overwrite a request id the run had just recorded. A lock file with a
# heartbeat keeps them apart. A lock whose heartbeat is older than LOCK_STALE_SECONDS is a crash
# and is taken over.
# --------------------------------------------------------------------------- #
LOCK_NAME = "ledger.lock"
LOCK_STALE_SECONDS = 300


class LedgerBusy(Exception):
    """Another command is writing this experiment's ledger."""


def _lock_info(path: Path) -> dict | None:
    try:
        return json.loads(path.read_text(encoding="utf-8"))
    except (OSError, ValueError):
        return None


def _fresh(info: dict | None) -> bool:
    return bool(info) and (time.time() - float(info.get("beat", 0))) < LOCK_STALE_SECONDS


def _busy_message(info: dict, path: Path) -> str:
    return (f"{info.get('who', 'another command')} is writing this experiment (started {info.get('started')}, "
            f"pid {info.get('pid')}). Wait for it to finish. If it crashed, delete {path} "
            f"(a lock with no heartbeat for {LOCK_STALE_SECONDS // 60} minutes is taken over automatically).")


def _lock_live(path: Path):
    """(is the lock held, its info). A lock file that exists but cannot be read (it is being rewritten, or is empty) counts as
    held while the file itself is recent: failing open here let a second command take over a running run's lock."""
    if not path.exists():
        return False, None
    info = _lock_info(path)
    if info is not None:
        return _fresh(info), info
    try:
        recent = (time.time() - path.stat().st_mtime) < LOCK_STALE_SECONDS
    except OSError:
        return False, None
    return recent, {"who": "another command", "started": "unknown", "pid": "unknown"}


def assert_unlocked(exp_dir) -> None:
    """Raise LedgerBusy while another command holds the experiment."""
    path = Path(exp_dir) / LOCK_NAME
    live, info = _lock_live(path)
    if live:
        raise LedgerBusy(_busy_message(info, path))


class Lock:
    """Held by `run` for its whole duration. Acquire raises LedgerBusy if another run holds it."""

    def __init__(self, exp_dir, who: str = "a run"):
        self.path = Path(exp_dir) / LOCK_NAME
        self.who = who
        self.started = now()

    def _write(self, flags) -> None:
        fd = os.open(str(self.path), flags)
        with os.fdopen(fd, "w", encoding="utf-8") as f:
            json.dump({"pid": os.getpid(), "who": self.who, "started": self.started, "beat": time.time()}, f)

    def acquire(self) -> "Lock":
        self.path.parent.mkdir(parents=True, exist_ok=True)
        for attempt in range(2):
            try:
                self._write(os.O_CREAT | os.O_EXCL | os.O_WRONLY)
                return self
            except FileExistsError:
                live, info = _lock_live(self.path)
                if live:
                    raise LedgerBusy(_busy_message(info, self.path))
                try:                                   # stale: a crashed run; take it over
                    self.path.unlink()
                except OSError:
                    pass
        raise LedgerBusy(f"could not take {self.path}")

    def touch(self) -> None:
        """Heartbeat: written to a temp file and renamed, so a reader never sees an empty lock."""
        tmp = self.path.with_name(f"{self.path.name}.{os.getpid()}.hb")
        try:
            with open(tmp, "w", encoding="utf-8") as f:
                json.dump({"pid": os.getpid(), "who": self.who, "started": self.started, "beat": time.time()}, f)
            os.replace(tmp, self.path)
        except OSError:
            try:
                tmp.unlink()
            except OSError:
                pass

    def release(self) -> None:
        try:
            self.path.unlink()
        except OSError:
            pass

    def __enter__(self):
        return self.acquire()

    def __exit__(self, *exc):
        self.release()


_HELD: set = set()


@contextlib.contextmanager
def writer_lock(exp_dir, who: str):
    """Hold the experiment's lock for a command that loads, changes and saves the ledger (preflight, refresh, recover,
    quick, add-takes), so it cannot run underneath a `run` or another writer. Re-entrant inside one process."""
    key = str(Path(exp_dir).resolve())
    if key in _HELD:
        yield
        return
    lock = Lock(exp_dir, who).acquire()
    _HELD.add(key)
    try:
        yield
    finally:
        _HELD.discard(key)
        lock.release()


# --------------------------------------------------------------------------- #
# construction
# --------------------------------------------------------------------------- #
def new_ledger(plan: dict, names: list[str], rows: list[tuple], selected: bool = True) -> dict:
    ledger = {
        "schema_version": SCHEMA_VERSION,
        "experiment": {"brief": plan["brief"], "intent": plan["intent"], "created_at": now(),
                       "next_row": 1, "dimensions": list(names)},
        "rows": [], "batches": [], "attempts": [],
    }
    for combo in rows:
        add_row(ledger, dict(zip(names, combo)), selected=selected)
    return ledger


def next_id(ledger: dict) -> str:
    exp = ledger["experiment"]
    rid = f"r{exp['next_row']:03d}"
    exp["next_row"] += 1                      # IDs are never reused, even after Removed
    return rid


def add_row(ledger: dict, params: dict, selected: bool = True, **fields) -> dict:
    row = {"id": next_id(ledger), "params": dict(params), "model": None, "take": 1, "reference": None,
           "selected": selected, "status": "Draft", "notes": "", "extras": {}}
    row.update(fields)
    ledger["rows"].append(row)
    return row


def by_id(ledger: dict) -> dict:
    return {r["id"]: r for r in ledger["rows"]}


def row_cost(ledger: dict, row_id: str) -> float:
    return round(sum(float(a.get("cost") or 0) for a in ledger["attempts"] if a.get("row_id") == row_id), 6)


# --------------------------------------------------------------------------- #
# merge user edits from the workbook
# --------------------------------------------------------------------------- #
def merge_workbook(ledger: dict, sheet_rows: list[dict]) -> list[str]:
    """Merge the user-owned cells of `sheet_rows` (from workbook.read_workbook) into the
    ledger and return warnings. Rules (design 6.3):

    - rows are matched by ID; IDs are read-only;
    - a row with no ID gets a new ID; a duplicate ID gets a new ID (warning); an ID the
      ledger does not know is treated as a new row (warning) and the original row, if it
      is no longer present, becomes Removed (its history is kept);
    - ledger rows absent from the workbook become Removed;
    - extra columns are carried as values in `extras`.
    """
    warnings: list[str] = []
    known = by_id(ledger)
    seen: set[str] = set()            # ledger rows matched by ID in this workbook
    fresh: set[str] = set()           # rows created during this merge
    for sr in sheet_rows:
        rid = (sr.get("id") or "").strip() or None
        label = f"row {sr.get('line', '?')}"
        if rid is None:
            row = add_row(ledger, sr["params"])
            fresh.add(row["id"])
            warnings.append(f"{label}: no ID, assigned {row['id']}")
        elif rid in seen:
            row = add_row(ledger, sr["params"])
            fresh.add(row["id"])
            warnings.append(f"{label}: duplicate ID {rid}, assigned {row['id']}")
        elif rid not in known:
            row = add_row(ledger, sr["params"])
            fresh.add(row["id"])
            warnings.append(f'{label}: ID "{rid}" not recognized (IDs are read-only), '
                            f"treated as a new row {row['id']}")
        else:
            row = known[rid]
            seen.add(rid)
            if row["status"] == "Removed":                  # the user restored a removed row
                row["status"] = "Draft"
            row["params"] = {**row["params"], **sr["params"]}         # a column the workbook no longer has keeps its value
        lq = sr.get("legacy_qty")
        if isinstance(lq, (int, float)) and not isinstance(lq, bool) and lq > 1:
            warnings.append(f"{row['id']}: Qty {lq:g} is no longer used (a row is one image). For more images of the "
                            f"same values run `image.py add-takes --dir <experiment> --rows {row['id']}`")
        for f in USER_FIELDS:
            if f not in sr:
                continue
            v = sr[f]
            if v is None and f in ("model", "reference"):
                row[f] = None                               # a blank cell means "per the plan / the strategy"
            elif v is None and f == "take":
                row[f] = 1                                  # a blank Take is the default
            elif v is not None:
                row[f] = v
        row["extras"] = dict(sr.get("extras") or {})
    in_file = ledger["experiment"].get("main_workbook_rows")
    in_file = set(in_file) if in_file is not None else None       # None: an older ledger, every row was in the file
    for row in ledger["rows"]:
        if row["id"] in seen or row["id"] in fresh or row["status"] == "Removed":
            continue
        if in_file is not None and row["id"] not in in_file:
            continue                      # added after the main workbook was last written (it fell back to a side copy)
        row["status"] = "Removed"
    return warnings


def record_used_model(ledger: dict, row: dict) -> None:
    """Show which model made a row's images: fill the Model cell with the model of its latest completed
    sample, unless the user already chose one. Done when a run finishes (never on a plain refresh), so
    clearing the cell afterwards stays cleared until the row is generated again. The filled value then
    acts as the row's choice for further samples, which keeps a row comparable across batches."""
    if row.get("model"):
        return
    done = [a for a in ledger["attempts"] if a["row_id"] == row["id"] and a["status"] == "Completed" and a.get("model")]
    if done:
        row["model"] = done[-1]["model"]


def same_image_key(row: dict) -> str:
    """Rows with the same key are takes of one configuration (same values, reference and verbatim prompt)."""
    return json.dumps([row["params"], row.get("reference"), row.get("prompt"), row.get("model")], sort_keys=True, ensure_ascii=False)


def add_takes(ledger: dict, row_ids: list[str], count: int) -> list[dict]:
    """Add `count` new rows for each given row: the same values, the next Take numbers, ticked. Returns them."""
    made = []
    rows = by_id(ledger)
    for rid in row_ids:
        src = rows.get(rid)
        if src is None or src["status"] == "Removed":
            raise KeyError(f"no such row: {rid}")
        key = same_image_key(src)
        top = max((r["take"] for r in ledger["rows"] if r["status"] != "Removed" and same_image_key(r) == key
                   and isinstance(r.get("take"), int) and not isinstance(r.get("take"), bool)), default=1)
        rest = {k: v for k, v in src.items() if k in ("model", "reference", "prompt") and v}
        for n in range(1, count + 1):
            made.append(add_row(ledger, src["params"], selected=True, take=top + n, **rest))
    return made
