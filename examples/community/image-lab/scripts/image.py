#!/usr/bin/env python3
"""Image Lab — the one script behind the skill (v0.1, gateway API).

Image Lab helps you PICK AN IMAGE, not pick a model. `count` is the number of
image alternatives you want; the Model Advisor decides how that budget is spread
across the best-fit models for the brief.

`--out` names a creative SESSION, not one run's folder: each `run` writes its
images to `<out>/round-N/`, N auto-detected from `<out>/session.json` (a ledger
appended once per completed round) — so "adjust the prompt, regenerate" can be
repeated against the same `--out` without ever overwriting an earlier round.
This applies from round 1 onward, not just round 2+.

Subcommands
  resolve --intent LABEL --count N [--models "id,id"] [--explain] [--out DIR]
        Check readiness (`loomloom doctor` / `balance`), resolve the token, then
        deterministically score the models and return the ALLOCATION: which
        models, how many each, at what size, and the estimated total. `--models`
        forces a specific model or head-to-head (the user override). `--explain`
        prints the score table on stderr. `--out` (default `./out`) is read-only
        here — resolve peeks at `<out>/session.json` to preview which round this
        would be, the resolved `<out>/round-N/` path, and cumulative session
        spend so far; it never creates or writes anything. Never calls the gateway.

  run --alloc "id:n:SIZE:ASPECT,id:n:SIZE:ASPECT" --prompt TEXT [--intent LABEL]
      [--out DIR] [--progress-file PATH] --confirm FINGERPRINT
        Submit the approved allocation to the gateway in parallel, poll each
        branch, redraw the live tree, download each image, print actual cost
        grouped by model, and write <out>/round-N/run.json (the record the
        exploration page is built from) — then append this round to
        <out>/session.json. --confirm must be the exact `fingerprint` resolve
        printed for THIS --alloc — a sha256 of the alloc string, not a bare
        yes/no flag, so an approval can't silently be replayed against a
        different allocation than the one actually shown to the user (this
        also means a prompt-only edit before a re-generate still needs a fresh
        resolve — the fingerprint binds the model/size allocation, not the
        prompt text, so there is no shortcut that skips re-quoting). Refuses
        without a matching --confirm. If the first model is unavailable it
        stops before any spend and prints a replacement (exit 3).
        --progress-file PATH gets a small JSON snapshot rewritten as each branch
        lands, so the skill can background this call and report live progress.

Then, on request, `scripts/build-exploration-page.py --session <out>` builds
(or rebuilds) the one case-study page for this whole session — a fixed
address that grows to embed each new round, with a widget to switch which
round is shown. No gate — nothing is spent.

Standard library only: json, urllib, subprocess. No SDK, no framework.
"""
from __future__ import annotations

import argparse
import hashlib
import json
import math
import os
import re
import shutil
import struct
import subprocess
import sys
import time
import urllib.error
import urllib.request
from datetime import date
from pathlib import Path

REFS = Path(__file__).resolve().parent.parent / "references"
GATEWAY = "https://router.cogfoundry.ai/api/v1"  # CogFoundry model gateway
# Cloudflare in front of the gateway blocks the default Python-urllib User-Agent
# (HTTP 403, "error code: 1010"). Any non-default UA passes.
USER_AGENT = "image-lab/0.1 (+https://github.com/cogfoundry-labs/loomloom)"
DIMENSIONS = ["overall", "commercial_design", "three_d_modeling", "cartoon",
              "photorealistic", "art", "portraits", "text_rendering"]
WEIGHT = {"low": 0, "medium": 1, "high": 2}
COUNTS = (1, 2, 4, 8)            # the alternatives budget — 4 is the default
POLL_SECONDS = 3
POLL_TIMEOUT = 480
TERMINAL = ("COMPLETED", "FAILED")
LABELS = "ABCDEFGH"
RETRY_SLEEP_SECONDS = 1.5
DEFAULT_PX = 1024 * 1024          # nominal pixel budget for models with no size control
# Tier -> approximate pixel budget. Internal only — never sent to the gateway;
# used solely to pick a cost tier and to size-match against `preferred_sizes`.
TIER_PX = {"0.5K": 512 * 512, "1K": 1024 * 1024, "2K": 2048 * 2048, "4K": 4096 * 4096}
# Aspect-ratio tokens actually offered across the Google Gemini image models'
# schemas (checked on cogfoundry.ai) — only sent to models whose catalog entry
# declares `request.aspect_ratio_param: true`. `size` and `aspect_ratio` are
# independent fields on those models (e.g. Nano Banana Pro takes both a size
# tier AND an aspect ratio at once), so this is computed regardless of
# `size_param`, not treated as a mutually-exclusive size shape.
ASPECT_RATIOS = ["1:1", "4:5", "5:4", "3:4", "4:3", "2:3", "3:2", "9:16", "16:9"]
STALE_AFTER_DAYS = 30  # cmd_check warns when a catalog entry's verified_on is older than this
SESSION_FILE = "session.json"  # ledger of every round in one creative session, see _load_session()
SOFT_ROUND_LIMIT = 5  # a nudge, not a cap — resolve/run keep working past this, they just say so


# --------------------------------------------------------------------------- #
# multi-round sessions — "adjust the prompt, regenerate" support
#
# `--out` names a SESSION, not a single run's folder: each `run` writes its
# images to `<out>/round-N/` (N auto-detected from how many rounds are already
# in `<out>/session.json`), never directly into `<out>` itself — so a second
# round can never silently overwrite the first one's images or run.json. This
# applies from round 1 onward (round 1 also gets its own `round-1/` folder),
# so there's no special-cased "first run is different" behavior to remember.
# --------------------------------------------------------------------------- #
def _load_session(session_root: Path) -> dict:
    """{"rounds": [...], "cumulative_usd": float}. Missing or corrupt
    session.json reads as an empty session (round 1, $0 so far) rather than
    failing — a session ledger is a convenience record, not load-bearing for
    correctness the way --confirm's fingerprint is."""
    p = session_root / SESSION_FILE
    if not p.exists():
        return {"rounds": [], "cumulative_usd": 0.0}
    try:
        data = json.loads(p.read_text(encoding="utf-8"))
    except (json.JSONDecodeError, OSError):
        return {"rounds": [], "cumulative_usd": 0.0}
    data.setdefault("rounds", [])
    data.setdefault("cumulative_usd", 0.0)
    return data


def _next_round(session_root: Path) -> int:
    return len(_load_session(session_root)["rounds"]) + 1


def _round_dir(session_root: Path, round_no: int) -> Path:
    return session_root / f"round-{round_no}"


def _append_round(session_root: Path, round_no: int, prompt: str | None, intent: str | None,
                   actual_usd: float, estimated_usd: float) -> dict:
    """Record a completed round in session.json and return the updated
    session. Called once, after `run` finishes — never by `resolve`, which
    only ever reads the session to preview the next round number."""
    session = _load_session(session_root)
    session["rounds"].append({
        "round": round_no,
        "prompt": prompt,
        "intent": intent,
        "out_dir": f"round-{round_no}",
        "actual_usd": round(actual_usd, 6),
        "estimated_usd": round(estimated_usd, 6),
        "created_at": time.strftime("%Y-%m-%dT%H:%M:%S"),
    })
    session["cumulative_usd"] = round(sum(r["actual_usd"] for r in session["rounds"]), 6)
    session_root.mkdir(parents=True, exist_ok=True)
    (session_root / SESSION_FILE).write_text(json.dumps(session, indent=2), encoding="utf-8")
    return session


def _round_note(round_no: int, cumulative_usd: float) -> str:
    """The soft reminder shown at both QUOTE (cumulative_usd = spend from
    rounds before this one) and RESULTS (cumulative_usd = spend including
    this one, right after it's appended to the session) from round 2 onward
    — visibility into cumulative spend across a whole "adjust and retry"
    session, not a hard stop. Nudges (doesn't block) past SOFT_ROUND_LIMIT."""
    if round_no <= 1:
        return ""
    note = f"this is round {round_no} of this session — ${cumulative_usd:.4f} spent across it so far"
    if round_no > SOFT_ROUND_LIMIT:
        note += f" ({round_no} rounds is a lot of iteration — worth a fresh PLAN instead?)"
    return note


def _fingerprint(s: str) -> str:
    """A short, non-secret content hash — binds an approval to the exact
    plan it approved (see --confirm) instead of being a bare yes/no flag
    that can be replayed against a different plan than the one shown to the
    user."""
    return hashlib.sha256(s.encode("utf-8")).hexdigest()[:12]


def die(msg: str, code: int = 1) -> "NoReturn":  # type: ignore[valid-type]
    print(f"error: {msg}", file=sys.stderr)
    sys.exit(code)


# --------------------------------------------------------------------------- #
# loomloom CLI (readiness + balance only)
# --------------------------------------------------------------------------- #
def _loomloom() -> str:
    exe = shutil.which("loomloom")
    if exe:
        return exe
    for c in (
        Path.home() / "AppData/Local/Programs/loomloom/loomloom.exe",
        Path.home() / "AppData/Local/Programs/loomloom/loomloom",
        Path("/usr/local/bin/loomloom"),
        Path.home() / ".local/bin/loomloom",
    ):
        if c.exists():
            return str(c)
    die("loomloom CLI not found. Install: https://github.com/cogfoundry-labs/loomloom")


def ll(*args: str) -> dict:
    try:
        p = subprocess.run([_loomloom(), *args, "--output", "json"],
                           capture_output=True, text=True, timeout=30)
    except (subprocess.TimeoutExpired, OSError):
        return {}
    try:
        return json.loads(p.stdout.strip() or "{}")
    except json.JSONDecodeError:
        return {}


def balance_usd() -> float | None:
    """USD available, or None if the balance could not be read."""
    b = ll("balance").get("availableBalance")
    if not isinstance(b, dict):
        return None
    try:
        return float(b.get("amount"))
    except (TypeError, ValueError):
        return None


def readiness() -> None:
    d = ll("doctor")
    if not d:
        die("could not run `loomloom doctor` — is the loomloom CLI installed and on PATH?")
    if not d.get("healthy") or not d.get("token_valid"):
        die("loomloom not ready — run `loomloom doctor`, then `loomloom server use <name>` / `loomloom login`.")


# --------------------------------------------------------------------------- #
# token
# --------------------------------------------------------------------------- #
def token() -> str:
    for env in ("LOOMLOOM_TOKEN_COGFOUNDRY", "LOOMLOOM_TOKEN"):
        v = os.environ.get(env)
        if v and v.strip():
            return v.strip()
    candidates = []
    if os.environ.get("APPDATA"):
        candidates.append(Path(os.environ["APPDATA"]) / "loomloom" / "config.json")
    candidates.append(Path.home() / ".config" / "loomloom" / "config.json")
    for cfg in candidates:
        if not cfg.exists():
            continue
        try:
            data = json.loads(cfg.read_text(encoding="utf-8"))
        except (json.JSONDecodeError, OSError):
            continue
        active = data.get("active_server")
        for prof in data.get("servers", []):
            if prof.get("name") == active and prof.get("token"):
                return str(prof["token"]).strip()
    die("no token. Set LOOMLOOM_TOKEN_COGFOUNDRY to your API key from "
        "https://console.cogfoundry.ai/api-keys (or run `loomloom login`).")


# --------------------------------------------------------------------------- #
# gateway HTTP
# --------------------------------------------------------------------------- #
TRANSIENT_HTTP = (429, 500, 502)        # 503 excluded — it means "model unavailable" to
                                        # callers (see _suggest_replacement) and must
                                        # surface immediately, not be retried away
TRANSIENT_HTTP_GET_ONLY = (504,)       # Gateway Timeout: the proxy gave up waiting on the
                                        # backend, which does NOT mean the backend never
                                        # started — same billing-ambiguity as a network-level
                                        # failure below, so (like those) it's only safe to
                                        # retry on GET. Fixed 2026-09-14 after code review
                                        # caught this being lumped into TRANSIENT_HTTP and
                                        # retried unconditionally on the money-spending POST,
                                        # which could double-submit (and double-bill) a task.


def _req(method: str, path: str, tok: str, body: dict | None = None, retries: int = 1) -> tuple[int, dict, str]:
    """Returns (status, json_or_empty, error_str). error_str is '' on success.

    Retries once (short pause) on a transient condition — but the failure
    categories are NOT equally safe to retry on a money-spending POST:

    - An HTTP 429/500/502 means the gateway itself responded and rejected the
      request before doing any work, so it never created a task — safe to
      retry for any method.
    - An HTTP 504 (Gateway Timeout) or a network-level failure (no response
      received at all — a dropped connection, a timeout while reading) is
      AMBIGUOUS for a POST: in both cases the gateway may have already
      accepted and started billing the task before the response was lost or
      the proxy gave up waiting, and resubmitting would silently create (and
      pay for) a second one, with no idempotency key to prevent it. So both
      of these are only retried for GET (polling), which has no side effects.

    Motivated by observing two CogFoundry image models fail, then succeed
    unmodified on immediate retry, when 9 generations were submitted at once
    (2026-09-13) — that was an HTTP 429/500/502 case, the safe category.
    """
    data = json.dumps(body).encode() if body is not None else None
    req = urllib.request.Request(
        f"{GATEWAY}{path}", data=data, method=method,
        headers={"Authorization": f"Bearer {tok}", "Content-Type": "application/json",
                 "User-Agent": USER_AGENT},
    )
    attempt = 0
    while True:
        try:
            with urllib.request.urlopen(req, timeout=60) as resp:
                raw = resp.read().decode()
                return resp.status, (json.loads(raw) if raw.strip() else {}), ""
        except urllib.error.HTTPError as e:
            try:
                parsed, err = json.loads(e.read().decode() or "{}"), ""
            except (json.JSONDecodeError, OSError):
                parsed, err = {}, f"HTTP {e.code}"
            retryable = e.code in TRANSIENT_HTTP or (method == "GET" and e.code in TRANSIENT_HTTP_GET_ONLY)
            if retryable and attempt < retries:
                attempt += 1
                time.sleep(RETRY_SLEEP_SECONDS)
                continue
            return e.code, parsed, err
        except (urllib.error.URLError, TimeoutError, OSError, json.JSONDecodeError) as e:
            if method == "GET" and attempt < retries:
                attempt += 1
                time.sleep(RETRY_SLEEP_SECONDS)
                continue
            return 0, {}, f"{type(e).__name__}: {e}"


DOWNLOAD_RETRIES = 3  # bounded retry with linear backoff — a signed CDN url is
                       # already paid for by the time we're here, so a dropped
                       # connection mid-download (observed live: "Tunnel
                       # Connection Failed") should not surface as a permanent
                       # failure when a retry would just work.


def _download(url: str, dest: Path) -> None:
    req = urllib.request.Request(url, headers={"User-Agent": USER_AGENT})
    last: Exception | None = None
    for attempt in range(DOWNLOAD_RETRIES + 1):
        try:
            with urllib.request.urlopen(req, timeout=60) as resp, open(dest, "wb") as f:
                shutil.copyfileobj(resp, f)
            with open(dest, "rb") as f:
                if f.read(8) != b"\x89PNG\r\n\x1a\n":
                    raise OSError("downloaded file is not a PNG (gateway may have returned an error body)")
            return
        except (urllib.error.URLError, OSError, TimeoutError) as e:
            last = e
            if attempt < DOWNLOAD_RETRIES:
                time.sleep(RETRY_SLEEP_SECONDS * (attempt + 1))
    raise last


# --------------------------------------------------------------------------- #
# reference files
# --------------------------------------------------------------------------- #
def load_policy() -> list[dict]:
    """Parse the intents list from generation-policy.md (no PyYAML)."""
    text = (REFS / "generation-policy.md").read_text(encoding="utf-8")
    m = re.search(r"##\s*intents\s*\n+```yaml\n(.*?)```", text, re.S | re.I) \
        or re.search(r"```yaml\n(.*?)```", text, re.S)
    if not m:
        die("generation-policy.md: could not find the ```yaml intents block")
    items = _parse_intents(m.group(1))
    if not items:
        die("generation-policy.md: no intents parsed")
    if not any(p["intent"] == "generic" for p in items):
        items.append({"intent": "generic",
                      "requirements": {d: "medium" for d in DIMENSIONS},
                      "preferred_sizes": ["1024x1024"]})
    return items


def _parse_intents(block: str) -> list[dict]:
    items: list[dict] = []
    cur: dict | None = None
    for raw in block.splitlines():
        line = raw.split("#", 1)[0].rstrip()
        if not line.strip():
            continue
        s = line.lstrip()
        if s.startswith("- intent:"):
            cur = {"intent": s.split(":", 1)[1].strip(),
                   "requirements": {}, "preferred_sizes": []}
            items.append(cur)
        elif cur is not None and s.startswith("requirements:") and "{" in s:
            inner = s.split("{", 1)[1].rsplit("}", 1)[0]
            for part in inner.split(","):
                if ":" in part:
                    k, _, v = part.partition(":")
                    cur["requirements"][k.strip()] = v.strip()
        elif cur is not None and s.startswith("preferred_sizes:"):
            cur["preferred_sizes"] = re.findall(r"\d+x\d+", s)
    # fill gaps so a partial "bring your own" entry still works
    for it in items:
        it["requirements"] = {d: it["requirements"].get(d, "low") for d in DIMENSIONS}
        if not it["preferred_sizes"]:
            it["preferred_sizes"] = ["1024x1024"]
    return items


def load_model_catalog() -> dict:
    """THE ONE PLACE that knows the catalog is a file. Swap this when the gateway
    ships a model endpoint (see the TODO in model-catalog.yaml)."""
    text = (REFS / "model-catalog.yaml").read_text(encoding="utf-8")
    m = re.search(r"^models:\s*\n(.*)", text, re.S | re.M)
    if not m:
        die("model-catalog.yaml: no `models:` block")
    out: dict = {}
    cur = None
    in_request = False
    for raw in m.group(1).splitlines():
        line = raw.split("#", 1)[0].rstrip()
        if not line.strip():
            continue
        if re.match(r"^ {2}\S.*:$", line):
            cur = line.strip()[:-1]
            out[cur] = {}
            in_request = False
            continue
        if not cur:
            continue
        s = line.strip()
        if s.startswith("- {") and isinstance(out[cur].get("pricing"), list):
            px = re.search(r"up_to_px\s*:\s*(\d+)", s)
            usd = re.search(r"usd\s*:\s*([\d.]+)", s)
            if px and usd:
                out[cur]["pricing"].append((int(px.group(1)), float(usd.group(1))))
            continue
        # `request:` sub-fields are indented one level deeper (6 spaces) than
        # this model's own top-level fields (4 spaces) — must be checked first,
        # since a 6-space line also satisfies a 4-space startswith check.
        if in_request and line.startswith("      ") and ":" in line:
            k, _, v = s.partition(":")
            k, v = k.strip(), v.strip()
            if k == "size_values":
                vals = re.findall(r'"([^"]+)"', v)
                # Fail loudly at load time rather than silently degrading to an
                # empty list — an empty size_values here would make pick_size()
                # fall back to floor-based computation, silently reintroducing
                # the "size outside the model's real range" bug this `request:`
                # field exists to prevent.
                if not vals and v.strip() not in ("", "[]"):
                    die(f"model-catalog.yaml: {cur}: size_values must be a quoted "
                        f'list (e.g. ["1024x1024", "2K"]), got: {v}')
                out[cur]["request"]["size_values"] = vals
            elif k in ("watermark_param", "aspect_ratio_param"):
                out[cur]["request"][k] = v.strip().lower() == "true"
            elif v:
                out[cur]["request"][k] = v.strip('"')
            continue
        if line.startswith("    ") and ":" in line:
            k, _, v = s.partition(":")
            k, v = k.strip(), v.strip()
            if k == "request":
                out[cur]["request"] = {}
                in_request = True
                continue
            in_request = False
            if k == "pricing":
                out[cur]["pricing"] = []
            elif k == "usd_per_image":
                try:
                    out[cur][k] = float(v)
                except ValueError:
                    pass
            elif k == "size_min_px":
                try:
                    out[cur][k] = int(v)
                except ValueError:
                    pass
            elif v:
                out[cur][k] = v.strip('"')
    for mid, entry in out.items():
        entry.setdefault("label", mid)
        entry.setdefault("url", "")
        entry.setdefault("size_min_px", 0)
        req = entry.setdefault("request", {})
        req.setdefault("size_param", "wxh")   # unspecified models keep the legacy literal-WxH behavior
        req.setdefault("watermark_param", False)
        req.setdefault("aspect_ratio_param", False)
        if not entry.get("pricing"):
            # A flat usd_per_image normalizes to a single always-applies tier.
            # A model with neither gets an empty tier list — model_price()
            # reports that as "no price known" rather than fabricating one
            # (see _priced(), which is what actually decides what to quote).
            entry["pricing"] = [(10 ** 12, entry["usd_per_image"])] if "usd_per_image" in entry else []
        entry["pricing"].sort()
    if not out:
        die("model-catalog.yaml: no models parsed")
    return out


def load_arena_scores() -> dict:
    """Parse references/arena-scores.yaml: raw Arena.ai Text-to-Image Arena
    Elo + margin per model per category — the ONLY place a quality judgment
    is hand-entered (see that file's header). Returns
    {"snapshot_date": str, "models": {model_id: {category: (elo, margin)}}}.
    compute_arena_scores() turns this into the [-1, 2]-per-dimension shape
    score_models() expects."""
    text = (REFS / "arena-scores.yaml").read_text(encoding="utf-8")
    m = re.search(r'snapshot_date:\s*"([^"]+)"', text)
    snapshot_date = m.group(1) if m else None
    m = re.search(r"^models:\s*\n(.*)", text, re.S | re.M)
    if not m:
        die("arena-scores.yaml: no `models:` block")
    models: dict[str, dict[str, tuple[float, float]]] = {}
    cur = None
    for raw in m.group(1).splitlines():
        line = raw.split("#", 1)[0].rstrip()
        if not line.strip():
            continue
        if re.match(r"^ {2}\S.*:$", line):
            cur = line.strip()[:-1]
            models[cur] = {}
            continue
        if not cur:
            continue
        cm = re.match(r"^\s+([a-z_]+):\s*\{\s*elo:\s*(-?[\d.]+)\s*,\s*margin:\s*(-?[\d.]+)\s*\}", line)
        if cm:
            models[cur][cm.group(1)] = (float(cm.group(2)), float(cm.group(3)))
    if not models:
        die("arena-scores.yaml: no models parsed")
    return {"snapshot_date": snapshot_date, "models": models}


PRICE_OBSERVATIONS_FILE = "price-observations.json"
UNVERIFIED_PRICE_PENALTY = 9.99  # internal ranking / wallet-gate-safety cost
                                  # only — never displayed as if it were a
                                  # real quote (see _priced() / _print_quote())


def load_price_observations() -> dict:
    """Flat {"model_id|size_token": usd} cache of prices CogFoundry has
    actually charged, keyed exactly like _priced() looks them up. Missing or
    corrupt reads as empty — this is a cache, not load-bearing state."""
    p = REFS / PRICE_OBSERVATIONS_FILE
    if not p.exists():
        return {}
    try:
        return json.loads(p.read_text(encoding="utf-8"))
    except (json.JSONDecodeError, OSError):
        return {}


def _record_observed_prices(tasks: list[dict]) -> None:
    """After a real run, remember what each COMPLETED task actually cost —
    the only trustworthy source of CogFoundry's real per-image price (see
    _priced()). FAILED tasks are skipped: whether a failed attempt was billed
    is a different question from what a successful generation costs. Latest
    observation always overwrites — a real repricing should win immediately,
    not get averaged away."""
    p = REFS / PRICE_OBSERVATIONS_FILE
    try:
        obs = json.loads(p.read_text(encoding="utf-8")) if p.exists() else {}
    except (json.JSONDecodeError, OSError):
        obs = {}
    changed = False
    for t in tasks:
        if t["status"] != "COMPLETED":
            continue
        obs[f"{t['model']}|{t['size']}"] = round(t["cost"], 6)
        changed = True
    if not changed:
        return
    try:
        p.write_text(json.dumps(obs, indent=2, sort_keys=True), encoding="utf-8")
    except OSError as e:
        print(f"note: could not write {p}: {e}", file=sys.stderr)


def _priced(mid: str, size_token: str | None, px: int, entry: dict,
            observations: dict) -> tuple[float, str]:
    """(usd_per_image, source) — a real observed CogFoundry charge beats the
    catalog's static estimate, which beats an explicit, honest "unverified"
    state. Never fabricates a number: UNVERIFIED_PRICE_PENALTY is a ranking/
    gate safety value, not a guess at the real price."""
    key = f"{mid}|{size_token or 'default'}"
    if key in observations:
        return observations[key], "observed"
    catalog_price = model_price(entry, px)
    if catalog_price is not None:
        return catalog_price, "catalog"
    return UNVERIFIED_PRICE_PENALTY, "unverified"


def _tier_split(models: list[tuple[str, float, float]]) -> list[list[str]]:
    """models: [(name, elo, margin), ...], margin = arena's reported 95% CI
    half-width. Returns tiers best-to-worst. Two models are "not meaningfully
    different" when their Elo gap doesn't exceed the quadrature combination
    of their margins (`sqrt(marginA**2 + marginB**2)`) — the correct way to
    combine two independent margins of error, NOT a linear sum (linear sum is
    provably more conservative and was found to under-split real data — see
    docs/plans/2026-09-13-image-lab-arena-scoring-implementation.md §1).
    Checks each candidate against EVERY existing tier member, not just the
    previous one, to avoid chaining two distinguishable models together
    through an intermediate one."""
    ordered = sorted(models, key=lambda x: -x[1])
    tiers: list[list[tuple[str, float, float]]] = []
    current: list[tuple[str, float, float]] = []
    for name, elo, margin in ordered:
        fits = all(
            abs(elo - e2) <= math.sqrt(margin ** 2 + m2 ** 2)
            for _, e2, m2 in current
        )
        if current and fits:
            current.append((name, elo, margin))
        else:
            if current:
                tiers.append(current)
            current = [(name, elo, margin)]
    if current:
        tiers.append(current)
    return [[name for name, _, _ in tier] for tier in tiers]


def compute_arena_scores(arena_data: dict) -> dict:
    """{model: {category: {"score": float, "tier_index": int, "num_tiers":
    int}}}. `score` is tier 0 (best) -> 2, worst tier -> -1, intermediate
    tiers evenly interpolated — kept in the same [-1, 2] range the existing
    weighted dot-product expects, purely for compatibility with that
    architecture; the meaning of the values is now entirely Arena-derived,
    not carried over from any prior hand-judged scale. `tier_index`/
    `num_tiers` are threaded through (not just the rescaled score) because
    the provider-diversity guardrail in _secondary()/_tertiary() compares
    tier distance, not raw score."""
    models = arena_data["models"]
    out: dict = {mid: {} for mid in models}
    for cat in DIMENSIONS:
        entries = [(mid, v[cat][0], v[cat][1]) for mid, v in models.items() if cat in v]
        if not entries:
            continue
        tiers = _tier_split(entries)
        n = len(tiers)
        for idx, tier in enumerate(tiers):
            score = 2.0 if n == 1 else 2.0 - (idx / (n - 1)) * 3.0
            for name in tier:
                out[name][cat] = {"score": score, "tier_index": idx, "num_tiers": n}
    return out


def _provider(model_id: str) -> str:
    return model_id.split("/", 1)[0]


def model_price(entry: dict, px: int) -> float | None:
    """Per-image USD for this model at an output size of `px` total pixels,
    or None if the catalog has no price data for it at all — see _priced(),
    which decides what to actually quote in that case."""
    if not entry["pricing"]:
        return None
    for cap, usd in entry["pricing"]:
        if px <= cap:
            return usd
    return entry["pricing"][-1][1]


def _shape(entry: dict) -> dict:
    """Single place that reads a catalog entry's `request` shape. Every
    function that needs to know how to call a model (pick_size, estimate_px,
    _parse_alloc, body_for, cmd_check) goes through here instead of each
    re-deriving `entry.get("request", {})...` independently — a future schema
    change (a renamed field, a new size_param value) then only has to be made
    once instead of in five places that can silently drift out of sync."""
    req = entry.get("request", {})
    return {
        "size_param": req.get("size_param", "wxh"),
        "size_values": req.get("size_values") or [],
        "size_default": req.get("size_default"),
        "watermark_param": req.get("watermark_param", False),
        "aspect_ratio_param": req.get("aspect_ratio_param", False),
    }


# --------------------------------------------------------------------------- #
# deterministic model scoring + allocation
#   1. disqualify a model that is in the worst tier at any HIGH requirement,
#      or that has no arena data at all for some dimension
#   2. rank the rest by SUITABILITY = weighted dot product of the brief's
#      requirement weights and the model's per-dimension scores
#   3. cost is only a tie-break (exact-suitability ties)
#   4. allocate the alternatives budget (count >= 2): A = best fit, B = the
#      next-best surviving model ("also worth trying"). A takes the whole budget
#      only when it is the sole surviving model, or the user forced one model.
#   5. count == 8 ("deep exploration") widens to a THIRD surviving model C when
#      one exists: A x3 + B x3 + C x2. `count` still means "alternatives", not
#      "top-N models" — 1 / 2 / 4 are unchanged.
# --------------------------------------------------------------------------- #


DOMINANT_TIER_SLACK = 1  # a diverse pick must be within this many CONFIRMED
                          # tiers (see compute_arena_scores) of the best
                          # available candidate on the intent's dominant
                          # dimension, or diversity is abandoned in favor of
                          # raw quality. "Diversity among credible
                          # alternatives, not at any cost." A raw point-value
                          # threshold was rejected: a suitability point means
                          # a different amount in every category depending on
                          # how many tiers it resolved into, so it has no
                          # principled unit; confirmed-tier distance does.
                          #
                          # Known limitation: this is a raw tier-COUNT slack,
                          # not normalized by how many tiers that dimension
                          # split into. A dimension with only 2 tiers total
                          # treats slack=1 as "any tier is close enough" (no
                          # real guardrail), while a dimension with 6 tiers
                          # treats it as a strict 1/5 of the range. Left as-is
                          # deliberately — normalizing this would change which
                          # model B/C get picked across the whole catalog, and
                          # that needs its own validation pass against the
                          # diversity-guardrail test cases, not a drive-by
                          # change bundled with an unrelated review.


def score_models(requirements: dict, preferred_sizes: list[str], catalog: dict,
                  arena_scores: dict, observations: dict | None = None) -> list[dict]:
    """Each model priced at the size IT would actually run — so a model that
    forces an upsize (Seedream) is costed at that larger size, not a notional
    1024x1024. Quality comes from `arena_scores` (compute_arena_scores()'s
    output) rather than a hand-typed `scores:` block — see
    references/arena-scores.yaml. `usd_per_image` prefers a real observed
    CogFoundry price over the catalog's static one — see _priced()."""
    observations = observations if observations is not None else {}
    weights = {d: WEIGHT.get(requirements.get(d, "low"), 0) for d in DIMENSIONS}
    rows = []
    for mid, m in catalog.items():
        sc = arena_scores.get(mid, {})
        missing = [d for d in DIMENSIONS if d not in sc]
        if missing:
            # sc.get(d, {}).get("score", 0.0) below would otherwise stand in a
            # neutral 0.0 for a dimension with NO arena data at all — 0.0 is
            # also a real mid-tier score, so a missing dimension would be
            # silently indistinguishable from a genuinely middling one.
            # Disqualify instead of guessing.
            print(f"warning: {mid} has no arena score for {', '.join(missing)} "
                  "— disqualifying until arena-scores.yaml is updated", file=sys.stderr)
        score = {d: sc.get(d, {}).get("score", 0.0) for d in DIMENSIONS}
        tier_index = {d: sc.get(d, {}).get("tier_index", 0) for d in DIMENSIONS}
        num_tiers = {d: sc.get(d, {}).get("num_tiers", 1) for d in DIMENSIONS}
        # Disqualify on tier position, not on the derived score hitting -1 —
        # that was an incidental property of the score formula (worst tier of
        # a multi-tier split happens to rescale to exactly -1.0), not the
        # actual rule. A single-tier category (num_tiers == 1, no Arena
        # disagreement) must never disqualify: tier_index 0 there means "the
        # only tier", not "the worst tier".
        disq = bool(missing) or any(
            weights[d] == 2 and num_tiers[d] > 1 and tier_index[d] == num_tiers[d] - 1
            for d in DIMENSIONS
        )
        quality = sum(weights[d] * score[d] for d in DIMENSIONS)
        sized = pick_size(preferred_sizes, m)
        usd_per_image, price_source = _priced(mid, sized["token"], sized["px"], m, observations)
        rows.append({
            "model": mid, "label": m["label"], "quality": quality,
            "tier_index": tier_index, "score_by_dim": score,
            "disqualified": disq, "size": sized["token"], "aspect_ratio": sized["aspect_ratio"],
            "usd_per_image": usd_per_image, "price_source": price_source,
            "catalog_usd_per_image": model_price(m, sized["px"]),
        })
    for i, r in enumerate(sorted(rows, key=lambda r: r["usd_per_image"])):
        r["cost_rank"] = i
    rows.sort(key=lambda r: (r["disqualified"], -r["quality"], r["cost_rank"]))
    return rows


def _primary(ranked: list[dict]) -> dict:
    """A = highest suitability; exact-quality ties broken by lower cost."""
    pool = [r for r in ranked if not r["disqualified"]] or ranked
    top_q = pool[0]["quality"]
    return min((r for r in pool if r["quality"] == top_q),
               key=lambda r: (r["usd_per_image"], r["cost_rank"]))


def _dominant_dimension(requirements: dict) -> str:
    """The intent's single highest-weighted requirement (ties broken
    alphabetically for determinism). Used only by the diversity guardrail
    below — never changes the suitability sum itself."""
    return max(sorted(requirements), key=lambda d: WEIGHT.get(requirements.get(d, "low"), 0))


def _tier_gap(a: dict, b: dict, dim: str) -> int:
    return abs(a["tier_index"][dim] - b["tier_index"][dim])


def _secondary(ranked: list[dict], primary: dict, requirements: dict) -> dict | None:
    """B = the next-best surviving model — the "also worth trying" pick. None
    only when A is the sole survivor (every other model disqualified).
    Thin wrapper over _tertiary() with a single model already taken (A) —
    see that docstring for the provider-diversity guardrail."""
    return _tertiary(ranked, requirements, primary)


def _tertiary(ranked: list[dict], requirements: dict, *taken: dict) -> dict | None:
    """The next-best surviving model not already taken — used for both B
    (one model taken: A) and C (two taken: A and B, only to widen count == 8).

    Prefers a different-provider candidate over one from a provider already
    taken (the catalog's 3 OpenAI skus otherwise crowd out every other lab),
    but only when that candidate is within DOMINANT_TIER_SLACK confirmed
    tiers, on the intent's dominant dimension, of the best available
    candidate — "diversity among credible alternatives, not diversity at any
    cost." A diverse candidate several tiers behind falls back to the raw
    best."""
    pool = [r for r in ranked if not r["disqualified"]] or ranked
    seen = {t["model"] for t in taken}
    seen_providers = {_provider(t["model"]) for t in taken}
    cand = [r for r in pool if r["model"] not in seen]
    if not cand:
        return None
    best_overall = min(cand, key=lambda r: (-r["quality"], r["usd_per_image"], r["cost_rank"]))
    dim = _dominant_dimension(requirements)
    other = [r for r in cand if _provider(r["model"]) not in seen_providers]
    if other:
        best_other = min(other, key=lambda r: (-r["quality"], r["usd_per_image"], r["cost_rank"]))
        if _tier_gap(best_other, best_overall, dim) <= DOMINANT_TIER_SLACK:
            return best_other
    return best_overall


def _split(count: int, k: int) -> list[int]:
    """count alternatives across k models, as even as possible, front-loaded.
    8 across 3 -> [3, 3, 2]; 4 across 2 -> [2, 2]; 4 across 3 -> [2, 1, 1]."""
    base, extra = divmod(count, k)
    return [base + (1 if i < extra else 0) for i in range(k)]


def allocate(ranked: list[dict], count: int, requirements: dict,
             forced: list[dict] | None = None) -> list[dict]:
    """Return [{model,label,size,usd_per_image,n,subtotal_usd}], summing to count.

    forced = the user's explicit model list (override); otherwise the Advisor
    picks A (+ maybe B).
    """
    if forced:
        models = forced[:count] if count < len(forced) else forced
    elif count == 1:
        models = [_primary(ranked)]
    else:
        a = _primary(ranked)
        b = _secondary(ranked, a, requirements)
        if not b:
            models = [a]
        elif count == 8 and (c := _tertiary(ranked, requirements, a, b)):
            models = [a, b, c]           # deep exploration -> widen to a 3rd model
        else:
            models = [a, b]

    ns = _split(count, len(models))
    alloc = []
    for m, n in zip(models, ns):
        if n <= 0:
            continue
        alloc.append({
            "model": m["model"], "label": m["label"], "size": m["size"] or "default",
            "aspect_ratio": m.get("aspect_ratio") or "-",
            "usd_per_image": m["usd_per_image"], "n": n,
            "subtotal_usd": round(m["usd_per_image"] * n, 6),
            "price_source": m.get("price_source", "catalog"),
            "catalog_usd_per_image": m.get("catalog_usd_per_image"),
        })
    return alloc


def _wh(s: str) -> tuple[int, int]:
    m = re.match(r"^(\d+)x(\d+)$", s.strip())
    return (int(m.group(1)), int(m.group(2))) if m else (0, 0)


def estimate_px(entry: dict, token: str | None) -> int:
    """Approximate total pixels for `token` under this model's size shape —
    used only to pick a cost tier via model_price(); never sent to the
    gateway, and never load-bearing for correctness the way the token itself
    is."""
    shape = _shape(entry)["size_param"]
    if shape == "tier":
        return TIER_PX.get(token, TIER_PX["1K"])
    if shape == "wxh":
        w, h = _wh(token or "")
        if w and h:
            return w * h
    return DEFAULT_PX


def _ar_diff(size_str: str, target_ar: float) -> float:
    w, h = _wh(size_str)
    if not (w and h):
        return float("inf")
    return abs((w / h) - target_ar)


def _nearest_aspect_ratio(w: int, h: int) -> str:
    target = w / h

    def _val(token: str) -> float:
        num, den = token.split(":")
        return int(num) / int(den)

    return min(ASPECT_RATIOS, key=lambda t: abs(_val(t) - target))


def pick_size(preferred: list[str], entry: dict) -> dict:
    """Return {"token": <size to send, or None>, "aspect_ratio": <ratio to
    send, or None>, "px": <for cost lookup>} for this model, given the
    intent's ordered `preferred_sizes` (literal WxH).

    `size` and `aspect_ratio` are independent fields on some models (e.g.
    Nano Banana Pro takes a size tier AND an aspect ratio at once) — so
    aspect_ratio is computed here regardless of `size_param`, not treated as
    a mutually-exclusive size shape. This is the ONE public entry point;
    `_pick_size_token()` below only computes the size half."""
    result = _pick_size_token(preferred, entry)
    if _shape(entry)["aspect_ratio_param"] and preferred:
        w, h = _wh(preferred[0])
        if w and h:
            result["aspect_ratio"] = _nearest_aspect_ratio(w, h)
    result.setdefault("aspect_ratio", None)
    return result


def _pick_size_token(preferred: list[str], entry: dict) -> dict:
    """The size half of pick_size() — dispatches on this model's own
    `request.size_param` instead of assuming every model takes literal WxH —
    that assumption was wrong for several models already in the catalog (see
    model-catalog.yaml's `request:` docs and docs/plans/2026-09-13-image-lab-
    lessons-from-model-image-arena.md)."""
    sh = _shape(entry)
    shape, values, default = sh["size_param"], sh["size_values"], sh["size_default"]

    if shape in ("aspect_ratio", "none"):
        # No absolute-size control on this model — nothing to send; `px` is a
        # nominal estimate purely for pricing (these models are flat-priced).
        return {"token": None, "px": estimate_px(entry, preferred[0] if preferred else None)}

    if shape == "tier":
        if not values:
            # Fail loudly here rather than guessing from an internal generic
            # tier vocabulary — a wrong guess would only surface later as a
            # confusing "bad size" error (or, worse, a live gateway 400).
            die(f"model-catalog.yaml: {entry.get('label', '?')} has "
                f"size_param 'tier' but no size_values")
        w, h = _wh(preferred[0]) if preferred else (0, 0)
        want = (w * h) if (w and h) else DEFAULT_PX
        token = next((t for t in values if TIER_PX.get(t, 0) >= want), values[-1])
        return {"token": token, "px": TIER_PX.get(token, DEFAULT_PX)}

    if shape == "wxh" and values:
        for s in preferred:
            if s in values:
                return {"token": s, "px": estimate_px(entry, s)}
        target = preferred[0] if preferred else (default or values[0])
        tw, th = _wh(target)
        if tw and th:
            best = min(values, key=lambda v: _ar_diff(v, tw / th))
            return {"token": best, "px": estimate_px(entry, best)}
        chosen = default or values[0]
        return {"token": chosen, "px": estimate_px(entry, chosen)}

    if shape != "wxh":
        # Everything above handles a known size_param; anything else is a
        # typo or a new value model-catalog.yaml hasn't been taught yet —
        # fail loudly here rather than silently falling into the legacy
        # continuous-wxh branch below, which was written only for genuine
        # enum-less wxh models (e.g. openai/gpt-image-2) and would otherwise
        # quietly mis-size an unrelated model.
        die(f"model-catalog.yaml: {entry.get('label', '?')} has unrecognized "
            f"size_param {shape!r}")

    # Legacy continuous `wxh` (no enumerated size_values) — e.g. openai/gpt-image-2,
    # which genuinely accepts a broad constrained range rather than a small enum.
    floor = entry.get("size_min_px", 0)
    for s in preferred:
        w, h = _wh(s)
        if w and h and w * h >= floor:
            return {"token": s, "px": w * h}
    w, h = _wh(preferred[0]) if preferred else (1024, 1024)
    w, h = w or 1024, h or 1024
    if w * h >= floor:
        return {"token": f"{w}x{h}", "px": w * h}
    ar = w / h
    hh = math.ceil(math.sqrt(floor / ar) / 64) * 64
    ww = math.ceil(hh * ar / 64) * 64
    return {"token": f"{ww}x{hh}", "px": ww * hh}


def match_intent(label: str, policy: list[dict]) -> dict:
    label = (label or "generic").strip().lower()
    for p in policy:
        if p["intent"].lower() == label:
            return p
    for p in policy:
        head = p["intent"].lower().split("/")[0].strip()
        if label and (label in p["intent"].lower() or (head and head in label)):
            return p
    return next((p for p in policy if p["intent"] == "generic"), policy[0])


def plan_for(intent_label: str, count: int, forced_ids: list[str] | None = None) -> dict:
    policy = load_policy()
    catalog = load_model_catalog()
    arena_scores = compute_arena_scores(load_arena_scores())
    observations = load_price_observations()
    intent = match_intent(intent_label, policy)
    ranked = score_models(intent["requirements"], intent["preferred_sizes"], catalog, arena_scores,
                           observations)
    by_id = {r["model"]: r for r in ranked}

    forced = None
    if forced_ids:
        forced = []
        for mid in forced_ids:
            if mid not in by_id:
                die(f"unknown model {mid}. Known: {', '.join(sorted(by_id))}")
            forced.append(by_id[mid])

    alloc = allocate(ranked, count, intent["requirements"], forced)
    total = round(sum(x["subtotal_usd"] for x in alloc), 6)
    arg = ",".join(f"{x['model']}:{x['n']}:{x['size']}:{x['aspect_ratio']}" for x in alloc)
    chosen = {x["model"] for x in alloc}
    dropped = [m for m in (forced_ids or []) if m not in chosen]
    if dropped:
        print(f"note: count {count} is smaller than the {len(forced_ids)} models you named; "
              f"not using {', '.join(dropped)}", file=sys.stderr)
    return {
        "intent": intent["intent"], "count": count, "allocation": alloc,
        "alloc_arg": arg, "fingerprint": _fingerprint(arg), "estimated_usd": total,
        "dropped_models": dropped,
        "why": _why(intent, alloc, ranked, bool(forced)), "ranked": ranked,
    }


def _why(intent: dict, alloc: list[dict], ranked: list[dict], forced: bool) -> str:
    name = f"'{intent['intent']}'"
    labels = [x["label"] for x in alloc]
    if forced:
        return f"you chose {' + '.join(labels)}"
    req = intent["requirements"]
    blocking = sorted({
        d for d in DIMENSIONS if req.get(d) == "high"
        and any(r["score_by_dim"].get(d, 0) == -1 for r in ranked)
    })
    ruled = f"; weak-at-{'/'.join(blocking)} models are ruled out" if blocking else ""
    if all(r["disqualified"] for r in ranked):
        return f"no model fully fits {name}; these are the least-bad options"
    if len(labels) == 1:
        return f"{labels[0]} is the best fit for {name}{ruled}"
    if len(labels) >= 3:
        return (f"{labels[0]} leads the fit for {name}{ruled}; "
                f"{labels[1]} and {labels[2]} widen a deep-exploration run")
    q = {r["model"]: r["quality"] for r in ranked}
    if q.get(alloc[1]["model"]) == q.get(alloc[0]["model"]):
        return f"{labels[0]} and {labels[1]} both top the fit for {name}{ruled}"
    return f"{labels[0]} is the best fit for {name}{ruled}; {labels[1]} is the runner-up"


# --------------------------------------------------------------------------- #
# commands
# --------------------------------------------------------------------------- #
def _parse_models_arg(s: str | None) -> list[str] | None:
    if not s:
        return None
    ids = [x.strip() for x in s.split(",") if x.strip()]
    return ids or None


def cmd_resolve(a) -> None:
    if a.count not in COUNTS:
        die(f"count must be one of {', '.join(map(str, COUNTS))}")
    readiness()
    token()  # dies with the fix instruction if not resolvable
    bal = balance_usd()
    out = {
        "ready": True,
        "balance_usd": bal,
        "balance_read": bal is not None,
        "models": sorted(load_model_catalog()),
        "intents": [p["intent"] for p in load_policy()],
    }
    if a.intent or a.models:
        plan = plan_for(a.intent, a.count, _parse_models_arg(a.models))
        est = plan["estimated_usd"]
        plan["sufficient"] = bal is None or bal >= est
        session_root = Path(a.out or "./out").resolve()
        plan["round"] = _next_round(session_root)
        plan["session_cumulative_usd"] = _load_session(session_root)["cumulative_usd"]
        plan["out_dir"] = str(_round_dir(session_root, plan["round"]))
        keys = ["intent", "count", "allocation", "alloc_arg", "fingerprint",
                "estimated_usd", "why", "sufficient", "out_dir", "round",
                "session_cumulative_usd"]
        if plan.get("dropped_models"):
            keys.append("dropped_models")
        out["plan"] = {k: plan[k] for k in keys}
        _print_quote(plan, bal, file=sys.stderr)
        if a.explain:
            print(f"\n  {'model':<36} suit    $/img    size              ok", file=sys.stderr)
            for r in plan["ranked"]:
                disp = r["size"] or "default"
                if r.get("aspect_ratio"):
                    disp = f"{disp}·{r['aspect_ratio']}"
                price = "⚠ unverified " if r.get("price_source") == "unverified" \
                    else f"${r['usd_per_image']:<7.4f}"
                print(f"  {r['model']:<36} {r['quality']:>6.2f}  "
                      f"{price} {disp:<17} "
                      f"{'-' if r['disqualified'] else 'yes'}", file=sys.stderr)
    print(json.dumps(out, indent=2))


def _print_quote(plan: dict, bal: float | None, file) -> None:
    round_tag = f"  (round {plan['round']})" if plan.get("round", 1) > 1 else ""
    print(f"\nPLAN  {plan['count']} alternative(s) for '{plan['intent']}'{round_tag}", file=file)
    for x in plan["allocation"]:
        disp = x["size"] if x.get("aspect_ratio", "-") == "-" else f"{x['size']} · {x['aspect_ratio']}"
        if x.get("price_source") == "unverified":
            # Never print UNVERIFIED_PRICE_PENALTY as if it were a real
            # quote — no catalog entry and no observed run means the price
            # is genuinely unknown, so say that instead of guessing.
            print(f"      {x['label']:<18} x{x['n']}   {disp:<16}  ⚠ price unverified", file=file)
            continue
        # Show $/img and the row total side by side — a bare subtotal here
        # previously read as "the price", which misled a user into thinking
        # $0.0207 was per-image when it was actually 3 x $0.0069.
        print(f"      {x['label']:<18} x{x['n']}   {disp:<16}  "
              f"${x['usd_per_image']:.4f}/img x {x['n']} = ~${x['subtotal_usd']:.4f}",
              file=file)
        cat_price = x.get("catalog_usd_per_image")
        if x.get("price_source") == "observed" and cat_price is not None \
                and round(cat_price, 4) != round(x["usd_per_image"], 4):
            print(f"                         observed price; catalog: ${cat_price:.4f}", file=file)
    print(f"      {plan['why']}", file=file)
    print(f"      output: {plan['out_dir']}  (created by `run`, not by this preview)", file=file)
    note = _round_note(plan.get("round", 1), plan.get("session_cumulative_usd", 0.0))
    if note:
        print(f"      {note}", file=file)
    est = plan["estimated_usd"]
    if bal is not None and bal < est:
        print(f"\nQUOTE Estimated total: ~${est:.4f}\n"
              f"      Balance: ${bal:.4f}  —  insufficient. Top up before generating.",
              file=file)
    else:
        tail = "" if bal is not None else "  (balance unread — the gateway will confirm)"
        print(f"\nQUOTE Estimated total: ~${est:.4f}   (actual shown after the run){tail}",
              file=file)
    print(f"      On approval, run with: --confirm {plan['fingerprint']}", file=file)


def _parse_alloc(s: str, catalog: dict) -> list[dict]:
    """`id:n:SIZE:ASPECT,...` -> [{model,label,size,aspect_ratio,n}]. SIZE/ASPECT
    are whatever `resolve`'s alloc_arg put there for that model (a literal WxH,
    a tier like "2K", or "default"/"-" for a field a model doesn't support) —
    validated against that model's own `request` shape, not assumed to be WxH."""
    out = []
    for chunk in s.split(","):
        # maxsplit=3: ASPECT itself contains a colon (e.g. "2:3"), so a plain
        # split(":") would over-split it — bound the split to exactly 4 parts,
        # with the 4th absorbing any colons of its own.
        parts = chunk.strip().split(":", 3)
        if len(parts) != 4:
            die(f"bad --alloc segment {chunk!r}; expected model:n:SIZE:ASPECT")
        mid, n_s, size, aspect = (p.strip() for p in parts)
        if mid not in catalog:
            die(f"unknown model {mid}")
        try:
            n = int(n_s)
        except ValueError:
            die(f"bad count in --alloc segment {chunk!r}")
        if n <= 0:
            continue
        entry = catalog[mid]
        sh = _shape(entry)
        shape, values = sh["size_param"], sh["size_values"]
        if shape in ("aspect_ratio", "none"):
            # Non-fatal (an incoming size here is meaningless, not malformed)
            # but still visible — silently swallowing it would hide a real
            # upstream bug in whatever built this --alloc string.
            if size != "default":
                print(f"note: {mid} has no size control; ignoring size {size!r}", file=sys.stderr)
            size = "default"
        elif values:
            default = sh["size_default"]
            if default is not None and default not in values:
                # size_default is trusted below as a known-good fallback —
                # if model-catalog.yaml's own default isn't a member of its
                # own size_values, that trust is misplaced and would send an
                # unvalidated size straight to the gateway. Catch it here,
                # at the source, rather than as a confusing live 400 later.
                die(f"model-catalog.yaml: {mid} has size_default {default!r} "
                    f"not present in its own size_values {values}")
            if size not in values and size != default:
                fallback = default or values[0]
                print(f"note: {mid} does not support size {size!r}; using {fallback}", file=sys.stderr)
                size = fallback
        else:
            w, h = _wh(size)
            if not (w and h):
                die(f"bad size in --alloc segment {chunk!r}")
            floor = entry.get("size_min_px", 0)
            if w * h < floor:
                fixed = pick_size([size], entry)["token"]
                print(f"note: {mid} needs >= {floor} px; using {fixed}", file=sys.stderr)
                size = fixed
        if not sh["aspect_ratio_param"] and aspect != "-":
            print(f"note: {mid} has no aspect-ratio control; ignoring aspect_ratio {aspect!r}",
                  file=sys.stderr)
            aspect = "-"
        out.append({"model": mid, "label": entry["label"], "size": size,
                     "aspect_ratio": aspect, "n": n})
    if not out:
        die("--alloc has no positive-count segments")
    total = sum(x["n"] for x in out)
    if total not in COUNTS:
        die(f"--alloc sums to {total}; must be one of {', '.join(map(str, COUNTS))}")
    return out


def cmd_run(a) -> None:
    if not a.confirm:
        die("run spends money. Re-invoke with --confirm <fingerprint> after the user approves the plan.")
    expected = _fingerprint(a.alloc)
    if a.confirm != expected:
        die(f"--confirm {a.confirm!r} does not match this --alloc (expected {expected!r}). "
            f"This --alloc is not the one the user approved — re-run resolve and use its "
            f"current plan.fingerprint, don't reuse an old --confirm value.")
    cat = load_model_catalog()
    alloc = _parse_alloc(a.alloc, cat)
    tok = token()

    session_root = Path(a.out or "./out").resolve()
    round_no = _next_round(session_root)
    out_dir = _round_dir(session_root, round_no)
    out_dir.mkdir(parents=True, exist_ok=True)

    # flat task list, in allocation order; each carries its own model + size
    tasks: list[dict] = []
    for seg in alloc:
        for _ in range(seg["n"]):
            tasks.append(_new_task(LABELS[len(tasks)], seg["model"], seg["label"],
                                    seg["size"], seg.get("aspect_ratio")))

    def body_for(t: dict) -> dict:
        """Build this branch's request body from its model's own `request`
        shape — never assume every model takes literal `size` WxH, an
        `aspect_ratio`, or tolerates `watermark` (some 400 on an undeclared
        parameter). `size` and `aspect_ratio` are independent fields, sent
        together when the model's schema declares both."""
        body = {"model": t["model"], "prompt": a.prompt}
        sh = _shape(cat.get(t["model"], {}))
        if sh["size_param"] in ("wxh", "tier") and t["size"] not in (None, "default"):
            body["size"] = t["size"]
        if sh["aspect_ratio_param"] and t.get("aspect_ratio") not in (None, "-"):
            body["aspect_ratio"] = t["aspect_ratio"]
        if sh["watermark_param"]:
            body["watermark"] = False
        return body

    # first submit — a bad first model 503s here, before any spend
    st, first, err = _req("POST", "/tasks/generations", tok, body_for(tasks[0]))
    if st == 503 or (first.get("error") or {}).get("code") == "service_unavailable":
        _suggest_replacement(tasks[0]["model"], first, a.intent)  # exits 3
    if err:
        die(f"gateway request failed before any spend: {err}")
    rid0 = (first.get("data") or {}).get("request_id")
    if not rid0:
        die(f"gateway rejected the request (HTTP {st}): {json.dumps(first)[:300]}")
    tasks[0]["rid"] = rid0
    tasks[0]["status"] = "SUBMITTING"
    tasks[0]["submitted_at"] = time.time()

    for t in tasks[1:]:
        _, r, e = _req("POST", "/tasks/generations", tok, body_for(t))
        rid = (r.get("data") or {}).get("request_id")
        t["submitted_at"] = time.time()
        if rid:
            t["rid"], t["status"] = rid, "SUBMITTING"
        else:  # a later model failing to submit does not sink the run
            t["status"] = "FAILED"
            t["done_at"] = t["submitted_at"]
            t["fail_reason"] = e or f"submit rejected: {json.dumps(r)[:120]}"

    prog = Path(a.progress_file).resolve() if a.progress_file else None
    _write_progress(prog, tasks, "submitted")

    # Priced per-task from each task's OWN resolved size, not aggregated by
    # model id — a model id keyed dict would collapse two tasks of the same
    # model at different sizes (possible via a hand-crafted --alloc) onto one
    # price and silently misprice the other. Same observed->catalog->
    # unverified priority as resolve's own quote (_priced()), so the
    # "estimated" figure RESULTS compares against agrees with what was quoted.
    observations = load_price_observations()
    expected = sum(
        _priced(t["model"], t["size"], estimate_px(cat[t["model"]], t["size"]), cat[t["model"]],
                observations)[0]
        for t in tasks
    )
    _poll_all(tasks, tok, out_dir, prog)
    _write_progress(prog, tasks, "done")
    _record_observed_prices(tasks)
    result = _report(tasks, expected, out_dir, cat, prompt=a.prompt, intent=a.intent)
    session = _append_round(session_root, round_no, a.prompt, a.intent,
                             result["actual_usd"], result["estimated_usd"])
    result["round"] = round_no
    result["session_root"] = str(session_root)
    result["session_cumulative_usd"] = session["cumulative_usd"]
    _write_run_json(out_dir, result)
    note = _round_note(round_no, session["cumulative_usd"])
    if note:
        print(f"\n  {note}", file=sys.stderr)
    print(json.dumps(result, indent=2))


def _write_progress(path: Path | None, tasks: list[dict], phase: str) -> None:
    """Overwrite `path` with a small JSON snapshot the agent polls while `run`
    works in the background. Best-effort — a write error never sinks the run."""
    if not path:
        return
    rec = {
        "phase": phase,                                  # submitted | generating | done
        "done": sum(t["status"] == "COMPLETED" for t in tasks),
        "failed": sum(t["status"] == "FAILED" for t in tasks),
        "total": len(tasks),
        "actual_usd_so_far": round(sum(t["cost"] for t in tasks), 6),
        "updated_at": round(time.time(), 1),
        "branches": [
            {"label": t["label"], "model_label": t["model_label"],
             "status": t["status"], "progress": t.get("progress"),
             "seconds": _elapsed(t), "cost_usd": round(t["cost"], 6)}
            for t in tasks
        ],
    }
    try:
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text(json.dumps(rec, indent=2), encoding="utf-8")
    except OSError:
        pass


def _png_size(path: str) -> str | None:
    """Actual WxH from a PNG's IHDR chunk (stdlib only)."""
    try:
        with open(path, "rb") as f:
            head = f.read(24)
    except OSError:
        return None
    if head[:8] != b"\x89PNG\r\n\x1a\n" or len(head) < 24:
        return None
    w, h = struct.unpack(">II", head[16:24])
    return f"{w}x{h}"


def _new_task(label: str, model: str, model_label: str, size: str, aspect_ratio: str | None) -> dict:
    return {"label": label, "model": model, "model_label": model_label, "size": size,
            "aspect_ratio": aspect_ratio,
            "rid": None, "status": "PENDING", "progress": "0%",
            "cost": 0.0, "url": None, "file": None,
            "submitted_at": None, "done_at": None,
            "start_time": None, "finish_time": None}


def _suggest_replacement(bad_model: str, resp: dict, intent_label: str | None) -> "NoReturn":  # type: ignore[valid-type]
    cat = load_model_catalog()
    remaining = {k: v for k, v in cat.items() if k != bad_model}
    if not remaining:
        die("the only image model in the catalog is unavailable.", code=3)
    policy = load_policy()
    intent = match_intent(intent_label, policy) if intent_label else None
    req = intent["requirements"] if intent else {d: "medium" for d in DIMENSIONS}
    sizes = intent["preferred_sizes"] if intent else ["1024x1024"]
    arena_scores = compute_arena_scores(load_arena_scores())
    alt = _primary(score_models(req, sizes, remaining, arena_scores, load_price_observations()))
    print(json.dumps({
        "status": "model_unavailable",
        "failed_model": bad_model,
        "message": (resp.get("error") or {}).get("message", "model unavailable"),
        "suggested_model": alt["model"],
        "suggested_label": alt["label"],
        "suggested_usd_per_image": alt["usd_per_image"],
        "suggested_size": alt["size"] or "default",
    }, indent=2))
    print(f"\nMODEL UNAVAILABLE\n  {bad_model} could not be generated right now.\n"
          f"  Suggested replacement: {alt['label']}  (~${alt['usd_per_image']}/image)\n"
          f"  Re-resolve, then re-run with the new allocation after the user approves.",
          file=sys.stderr)
    sys.exit(3)


def _poll_all(tasks: list[dict], tok: str, out_dir: Path, progress_file: Path | None = None) -> None:
    deadline = time.time() + POLL_TIMEOUT
    last = ""
    net_fails = 0
    while time.time() < deadline:
        for t in tasks:
            if t["status"] in TERMINAL or not t["rid"]:
                continue
            _, r, err = _req("GET", f"/tasks/generations/{t['rid']}", tok)
            if err:                       # transient — retry next cycle
                net_fails += 1
                continue
            d = r.get("data") or {}
            was_terminal = t["status"] in TERMINAL
            t["status"] = d.get("status", t["status"])
            t["progress"] = d.get("progress", t["progress"])
            if d.get("cost") is not None:
                # was `if d.get("cost"):` — treated a real, confirmed $0.00
                # (e.g. a launch-preview model) as "no update" and could leave
                # a stale nonzero interim value in place at COMPLETED time.
                t["cost"] = float(d["cost"])
            for k in ("start_time", "finish_time"):
                if isinstance(d.get(k), (int, float)):
                    t[k] = d[k]
            if not was_terminal and t["status"] in TERMINAL and not t["done_at"]:
                t["done_at"] = time.time()
            urls = (d.get("data") or {}).get("image_urls") or []
            if t["status"] == "COMPLETED" and urls and not t["file"]:
                t["url"] = urls[0]
                dest = out_dir / f"variant-{t['label'].lower()}.png"
                try:
                    _download(urls[0], dest)
                    t["file"] = str(dest)
                except (urllib.error.URLError, OSError, TimeoutError) as e:
                    t["file_error"] = str(e)
            if t["status"] == "FAILED":
                t["fail_reason"] = d.get("fail_reason", "")
        frame = _tree(tasks)
        if frame != last:
            print(frame, file=sys.stderr)
            _write_progress(progress_file, tasks, "generating")
            last = frame
        if all(t["status"] in TERMINAL for t in tasks):
            return
        time.sleep(POLL_SECONDS)
    unfinished = [t["label"] for t in tasks if t["status"] not in TERMINAL]
    print(f"timed out after {POLL_TIMEOUT}s; branches still running: {', '.join(unfinished)}"
          + (f" ({net_fails} transient poll errors)" if net_fails else ""), file=sys.stderr)


def _short(model: str) -> str:
    return model.split("/", 1)[-1]


def _tree(tasks: list[dict]) -> str:
    done = sum(t["status"] == "COMPLETED" for t in tasks)
    lines = [f"GENERATE  {done}/{len(tasks)} done"]
    for t in tasks:
        extra = f"  {t['progress']}" if t["status"] in ("IN_PROGRESS", "PROCESSING") else ""
        lines.append(f"  {t['label']}  {_short(t['model']):<26} {t['status']}{extra}")
    return "\n".join(lines)


def _elapsed(t: dict) -> float | None:
    """Generation time in seconds. Prefer the gateway's own start/finish epochs
    (the true model time); fall back to local wall-clock from submit to the
    branch reaching a terminal state (includes queue wait, ~POLL_SECONDS
    granularity)."""
    s, f = t.get("start_time"), t.get("finish_time")
    if isinstance(s, (int, float)) and isinstance(f, (int, float)) and 0 < f - s < 3600:
        return round(f - s, 1)
    if t.get("submitted_at") and t.get("done_at"):
        return round(t["done_at"] - t["submitted_at"], 1)
    return None


def _report(tasks: list[dict], estimate: float, out_dir: Path, catalog: dict | None = None,
            prompt: str | None = None, intent: str | None = None) -> dict:
    """Prints the human-readable RESULTS block to stderr and returns the run
    record — cmd_run adds session/round bookkeeping (round, session_root,
    session_cumulative_usd) to it and is the one that writes <out_dir>/run.json,
    so the persisted file and the documented record are never out of sync
    (this function used to write the file itself before those fields were
    added, which meant run.json on disk permanently lacked them)."""
    catalog = catalog or {}
    actual = sum(t["cost"] for t in tasks)
    order: list[str] = []
    for t in tasks:
        if t["model"] not in order:
            order.append(t["model"])

    def name(t: dict) -> str | None:                 # basename — the images all live in out_dir
        return Path(t["file"]).name if t.get("file") else None

    print(f"\nRESULTS  (in {out_dir})", file=sys.stderr)
    for model in order:
        grp = [t for t in tasks if t["model"] == model]
        sub = sum(t["cost"] for t in grp)
        print(f"  {grp[0]['model_label']}   (${sub:.4f})", file=sys.stderr)
        for t in grp:
            secs = _elapsed(t)
            took = f"{secs:>5.0f}s " if secs is not None else "   -- "
            if t.get("file"):
                mark = name(t)
            elif t["status"] == "FAILED":
                mark = f"FAILED — {t.get('fail_reason', '')}"
            else:
                mark = t.get("file_error") or f"NO IMAGE (status {t['status']})"
            print(f"    {t['label']}  {took} {mark}", file=sys.stderr)
    print(f"\n  Actual total: ${actual:.4f}   (estimated ~${estimate:.4f})", file=sys.stderr)

    n = len(tasks)
    alternatives = [
        {"label": t["label"], "index": i + 1, "of": n,
         "model": t["model"], "model_label": t["model_label"],
         "model_url": catalog.get(t["model"], {}).get("url") or None,
         "requested_size": t["size"],
         "requested_aspect_ratio": t.get("aspect_ratio") if t.get("aspect_ratio") not in (None, "-") else None,
         "actual_size": _png_size(t["file"]) if t.get("file") else None,
         "cost_usd": round(t["cost"], 6),
         "seconds": _elapsed(t),
         "status": t["status"],
         "file": name(t),
         "note": t.get("fail_reason") or t.get("file_error") or None}
        for i, t in enumerate(tasks)
    ]

    def entry(t: dict) -> dict:
        return {"label": t["label"], "model": t["model"], "cost_usd": round(t["cost"], 6),
                "file": name(t)}

    result = {
        "actual_usd": round(actual, 6),
        "estimated_usd": round(estimate, 6),
        "out_dir": str(out_dir),
        "prompt": prompt,
        "intent": intent,
        "alternatives": alternatives,
        "by_model": [
            {"model": m, "label": next(t["model_label"] for t in tasks if t["model"] == m),
             "actual_usd": round(sum(t["cost"] for t in tasks if t["model"] == m), 6),
             "images": [name(t) for t in tasks if t["model"] == m and t.get("file")]}
            for m in order
        ],
        "images": [name(t) for t in tasks if t.get("file")],
        "failed": [entry(t) for t in tasks if t["status"] == "FAILED"],
        "incomplete": [entry(t) for t in tasks
                       if t["status"] != "FAILED" and not t.get("file")],
    }
    return result


def _write_run_json(out_dir: Path, result: dict) -> None:
    """Persists the final run record (after cmd_run has added its round/session
    fields) — kept separate from _report() so the file on disk always matches
    what run prints, instead of being written mid-build and missing fields
    added afterward."""
    try:
        (out_dir / "run.json").write_text(json.dumps(result, indent=2), encoding="utf-8")
    except OSError as e:
        print(f"note: could not write {out_dir / 'run.json'}: {e}", file=sys.stderr)


# --------------------------------------------------------------------------- #
# catalog drift check (no spend, no network) — P1 in
# docs/plans/2026-09-13-image-lab-lessons-from-model-image-arena.md
# --------------------------------------------------------------------------- #
def cmd_check(_a) -> None:
    """Sanity-check model-catalog.yaml against generation-policy.md: for every
    (model, intent) pair, verify the size pick_size() would actually compute
    is one of that model's own declared-good values, and that size_default is
    itself consistent with size_values. Also warns (non-fatal) when a model's
    `verified_on` date is older than STALE_AFTER_DAYS, or missing.

    IMPORTANT — this validates the catalog's INTERNAL self-consistency only.
    It makes no network call and has no independent ground truth to check
    against, so it CANNOT detect CogFoundry silently changing a model's real
    accepted values on their end — exactly what happened to Seedream 5.0 Pro
    between 2026-09-06 and 2026-09-13, the bug this catalog's `request:`
    fields exist to fix. A clean run here means the catalog agrees with
    itself, not that it still matches the live API. The `verified_on`/staleness
    warning is a bound on how old that risk might be, not a live check —
    re-verify against a real call after any long gap since a model was last
    checked on cogfoundry.ai.
    """
    catalog = load_model_catalog()
    policy = load_policy()
    arena_raw = load_arena_scores()
    observations = load_price_observations()
    problems: list[str] = []
    warnings: list[str] = []

    # Arena data completeness: every catalog model needs a score for every
    # dimension, or score_models() silently falls back to a neutral 0 for
    # the missing cell (see score_models()'s arena_scores.get(mid, {})) —
    # better to surface that loudly here than let it quietly skew a ranking.
    for mid in sorted(catalog):
        have = set(arena_raw["models"].get(mid, {}))
        missing = [d for d in DIMENSIONS if d not in have]
        if missing:
            problems.append(f"{mid}: arena-scores.yaml is missing {', '.join(missing)}")

    # Staleness of the ONE arena snapshot (all cells share one pull date,
    # unlike model-catalog.yaml's per-model verified_on for request shape).
    snapshot_date = arena_raw.get("snapshot_date")
    if not snapshot_date:
        warnings.append("arena-scores.yaml: no snapshot_date — unknown how stale the quality scores are")
    else:
        try:
            age_days = (date.today() - date.fromisoformat(snapshot_date)).days
        except ValueError:
            problems.append(f"arena-scores.yaml: snapshot_date {snapshot_date!r} is not a valid ISO date (YYYY-MM-DD)")
        else:
            if age_days > STALE_AFTER_DAYS:
                warnings.append(
                    f"arena-scores.yaml: snapshot_date {snapshot_date} is {age_days} days old "
                    f"(> {STALE_AFTER_DAYS}) — re-pull the Arena.ai leaderboards")

    for mid, entry in sorted(catalog.items()):
        # An unpriced model isn't a broken config — resolve/run will honestly
        # quote it as "unverified" (see _priced()) and the first real run
        # teaches Image Lab the price. Loud enough to notice, not a hard fail.
        if not entry["pricing"] and not any(k.startswith(f"{mid}|") for k in observations):
            warnings.append(f"{mid}: no catalog price and no observed price yet — "
                             f"will quote as unverified until run once")

        sh = _shape(entry)
        shape, values, default = sh["size_param"], sh["size_values"], sh["size_default"]

        if shape not in ("wxh", "tier", "aspect_ratio", "none"):
            problems.append(f"{mid}: unknown request.size_param {shape!r}")
            continue  # unrecognized shape — pick_size() below can't handle it safely
        if default and values and default not in values:
            problems.append(f"{mid}: size_default {default!r} is not in its own size_values {values}")
        if shape == "tier" and not values:
            problems.append(f"{mid}: size_param 'tier' needs a size_values list")
            continue  # pick_size() would die() on this model — already reported above

        for intent in policy:
            sized = pick_size(intent["preferred_sizes"], entry)
            token = sized["token"]
            if shape in ("wxh", "tier") and values and token not in values:
                problems.append(
                    f"{mid} / {intent['intent']!r}: computed size {token!r} is not "
                    f"one of this model's own size_values {values}")

        # Staleness: internal consistency (above) can't detect the vendor changing a
        # model's real behavior. A verified_on date at least bounds how old our last
        # look was, so a check can flag "go re-verify this" instead of staying silent
        # forever between real API checks.
        verified_on = entry.get("verified_on")
        if not verified_on:
            warnings.append(f"{mid}: no verified_on date — unknown how stale this entry is")
        else:
            try:
                age_days = (date.today() - date.fromisoformat(verified_on)).days
            except ValueError:
                problems.append(f"{mid}: verified_on {verified_on!r} is not a valid ISO date (YYYY-MM-DD)")
            else:
                if age_days > STALE_AFTER_DAYS:
                    warnings.append(
                        f"{mid}: verified_on {verified_on} is {age_days} days old "
                        f"(> {STALE_AFTER_DAYS}) — re-check its request shape and pricing "
                        f"against cogfoundry.ai")

    print(f"checked {len(catalog)} model(s) x {len(policy)} intent(s)", file=sys.stderr)
    for w in warnings:
        print(f"WARN: {w}", file=sys.stderr)
    if problems:
        for p in problems:
            print(f"FAIL: {p}", file=sys.stderr)
        die(f"{len(problems)} catalog problem(s) found — fix model-catalog.yaml before generating")
    print("OK — the catalog is internally self-consistent (this does not confirm it still "
          "matches the live API — see this command's docstring).", file=sys.stderr)
    print(json.dumps({
        "ok": True, "models_checked": len(catalog), "intents_checked": len(policy),
        "warnings": warnings,
    }, indent=2))


# --------------------------------------------------------------------------- #
def main() -> None:
    ap = argparse.ArgumentParser(prog="image.py")
    sub = ap.add_subparsers(dest="cmd", required=True)

    r = sub.add_parser("resolve")
    r.add_argument("--intent", default=None)
    r.add_argument("--count", type=int, default=4)
    r.add_argument("--models", default=None,
                   help="comma-separated model ids to force (user override of the Advisor)")
    r.add_argument("--explain", action="store_true")
    r.add_argument("--out", default=None,
                   help="the SESSION folder (default ./out) — `run` writes each round to "
                        "<out>/round-N/, auto-numbered from <out>/session.json, never directly "
                        "into <out> itself. Shown in the quote as a preview only (round number + "
                        "resolved path + cumulative session spend); resolve never creates or "
                        "writes anything. Pass the same value you intend to give `run`.")

    x = sub.add_parser("run")
    x.add_argument("--alloc", required=True,
                   help='approved allocation: "id:n:SIZE:ASPECT,..." (from resolve\'s alloc_arg; '
                        'SIZE is a literal WxH, a tier like "2K", or "default"; '
                        'ASPECT is a ratio like "2:3" or "-")')
    x.add_argument("--prompt", required=True)
    x.add_argument("--intent", default=None,
                   help="used to score a replacement if the first model is unavailable")
    x.add_argument("--out", default=None,
                   help="the SESSION folder (default ./out) — images/run.json for THIS round land "
                        "in <out>/round-N/, N auto-detected from <out>/session.json (one JSON "
                        "record appended per completed round; never overwrites an earlier round). "
                        "Pass the same --out across an 'adjust the prompt, regenerate' loop; a "
                        "brand-new prompt/subject deserves a fresh --out instead.")
    x.add_argument("--progress-file", default=None,
                   help="poll target: a JSON snapshot rewritten as each branch lands "
                        "(so the skill can run this in the background and report progress)")
    x.add_argument("--confirm", default=None,
                   help="the exact plan.fingerprint resolve printed for THIS --alloc — not a "
                        "bare flag, so the approval can't be replayed against a changed plan")

    sub.add_parser("check", help="no-spend sanity check of model-catalog.yaml vs. generation-policy.md")

    a = ap.parse_args()
    {"resolve": cmd_resolve, "run": cmd_run, "check": cmd_check}[a.cmd](a)


if __name__ == "__main__":
    main()
