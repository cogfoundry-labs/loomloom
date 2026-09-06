#!/usr/bin/env python3
"""Image Lab — the one script behind the skill (v0.1, router API).

Image Lab helps you PICK AN IMAGE, not pick a model. `count` is the number of
image alternatives you want; the Model Advisor decides how that budget is spread
across the best-fit models for the brief.

Subcommands
  resolve --intent LABEL --count N [--models "id,id"] [--explain]
        Check readiness (`loomloom doctor` / `balance`), resolve the token, then
        deterministically score the models and return the ALLOCATION: which
        models, how many each, at what size, and the estimated total. `--models`
        forces a specific model or head-to-head (the user override). `--explain`
        prints the score table on stderr. Never calls the router.

  run --alloc "id:n:WxH,id:n:WxH" --prompt TEXT [--intent LABEL]
      [--out DIR] --confirm
        Submit the approved allocation to the router in parallel, poll each
        branch, redraw the live tree, download each image, print actual cost
        grouped by model, and write <out>/run.json (the record the exploration
        page is built from). Refuses without --confirm. If the first model is
        unavailable it stops before any spend and prints a replacement (exit 3).

Then, on request, `scripts/build-exploration-page.py --from <out>` turns that
run into a shareable static folder. No gate — nothing is spent.

Standard library only: json, urllib, subprocess. No SDK, no framework.
"""
from __future__ import annotations

import argparse
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
from pathlib import Path

REFS = Path(__file__).resolve().parent.parent / "references"
ROUTER = "https://router.cogfoundry.ai/api/v1"
# Cloudflare in front of the router blocks the default Python-urllib User-Agent
# (HTTP 403, "error code: 1010"). Any non-default UA passes.
USER_AGENT = "image-lab/0.1 (+https://github.com/cogfoundry-labs/loomloom)"
DIMENSIONS = ["photorealism", "typography", "composition_control", "speed"]
WEIGHT = {"low": 0, "medium": 1, "high": 2}
COUNTS = (1, 2, 4, 8)            # the alternatives budget — 4 is the default
POLL_SECONDS = 3
POLL_TIMEOUT = 480
TERMINAL = ("COMPLETED", "FAILED")
LABELS = "ABCDEFGH"


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
# router HTTP
# --------------------------------------------------------------------------- #
def _req(method: str, path: str, tok: str, body: dict | None = None) -> tuple[int, dict, str]:
    """Returns (status, json_or_empty, error_str). error_str is '' on success."""
    data = json.dumps(body).encode() if body is not None else None
    req = urllib.request.Request(
        f"{ROUTER}{path}", data=data, method=method,
        headers={"Authorization": f"Bearer {tok}", "Content-Type": "application/json",
                 "User-Agent": USER_AGENT},
    )
    try:
        with urllib.request.urlopen(req, timeout=60) as resp:
            raw = resp.read().decode()
            return resp.status, (json.loads(raw) if raw.strip() else {}), ""
    except urllib.error.HTTPError as e:
        try:
            return e.code, json.loads(e.read().decode() or "{}"), ""
        except (json.JSONDecodeError, OSError):
            return e.code, {}, f"HTTP {e.code}"
    except (urllib.error.URLError, TimeoutError, OSError, json.JSONDecodeError) as e:
        return 0, {}, f"{type(e).__name__}: {e}"


def _download(url: str, dest: Path) -> None:
    req = urllib.request.Request(url, headers={"User-Agent": USER_AGENT})
    with urllib.request.urlopen(req, timeout=60) as resp, open(dest, "wb") as f:
        shutil.copyfileobj(resp, f)
    with open(dest, "rb") as f:
        if f.read(8) != b"\x89PNG\r\n\x1a\n":
            raise OSError("downloaded file is not a PNG (router may have returned an error body)")


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
    """THE ONE PLACE that knows the catalog is a file. Swap this when the router
    ships a model endpoint (see the TODO in router-model-catalog.yaml)."""
    text = (REFS / "router-model-catalog.yaml").read_text(encoding="utf-8")
    m = re.search(r"^models:\s*\n(.*)", text, re.S | re.M)
    if not m:
        die("router-model-catalog.yaml: no `models:` block")
    out: dict = {}
    cur = None
    for raw in m.group(1).splitlines():
        line = raw.split("#", 1)[0].rstrip()
        if not line.strip():
            continue
        if re.match(r"^ {2}\S.*:$", line):
            cur = line.strip()[:-1]
            out[cur] = {}
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
        if line.startswith("    ") and ":" in line:
            k, _, v = s.partition(":")
            k, v = k.strip(), v.strip()
            if k == "pricing":
                out[cur]["pricing"] = []
            elif k == "scores":
                sc = dict(re.findall(r"([a-z_]+)\s*:\s*(-?\d+)", v))
                out[cur]["scores"] = {d: int(sc.get(d, 0)) for d in DIMENSIONS}
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
        entry.setdefault("scores", {d: 0 for d in DIMENSIONS})
        entry.setdefault("usd_per_image", 9.99)
        entry.setdefault("size_min_px", 0)
        if not entry.get("pricing"):
            entry["pricing"] = [(10 ** 12, float(entry["usd_per_image"]))]
        entry["pricing"].sort()
    if not out:
        die("router-model-catalog.yaml: no models parsed")
    return out


def model_price(entry: dict, px: int) -> float:
    """Per-image USD for this model at an output size of `px` total pixels."""
    for cap, usd in entry["pricing"]:
        if px <= cap:
            return usd
    return entry["pricing"][-1][1]


# --------------------------------------------------------------------------- #
# deterministic model scoring + allocation
#   1. disqualify a model that is weak (-1) at any HIGH requirement
#   2. rank the rest by SUITABILITY = weighted dot product of the brief's
#      requirement weights and the model's per-dimension scores
#   3. cost is only a tie-break (exact-quality ties, and the "also worth
#      trying" set for the second model)
#   4. allocate the alternatives budget: A gets it all unless a genuinely
#      competitive second model B exists (within QUALITY_TOLERANCE of A) —
#      then split it A / B
# --------------------------------------------------------------------------- #
QUALITY_TOLERANCE = 1


def score_models(requirements: dict, preferred_sizes: list[str], catalog: dict) -> list[dict]:
    """Each model priced at the size IT would actually run — so a model that
    forces an upsize (Seedream) is costed at that larger size, not a notional
    1024x1024."""
    weights = {d: WEIGHT.get(requirements.get(d, "low"), 0) for d in DIMENSIONS}
    rows = []
    for mid, m in catalog.items():
        sc = m["scores"]
        disq = any(weights[d] == 2 and sc.get(d, 0) == -1 for d in DIMENSIONS)
        quality = sum(weights[d] * sc.get(d, 0) for d in DIMENSIONS)
        size = pick_size(preferred_sizes, m["size_min_px"])
        w, h = _wh(size)
        rows.append({
            "model": mid, "label": m["label"], "quality": quality,
            "disqualified": disq, "size": size,
            "usd_per_image": model_price(m, w * h),
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


def _secondary(ranked: list[dict], primary: dict) -> dict | None:
    """B = the best OTHER model that is genuinely competitive for this brief
    (within QUALITY_TOLERANCE of A). None if nothing else clears the bar."""
    pool = [r for r in ranked if not r["disqualified"]] or ranked
    cand = [r for r in pool if r["model"] != primary["model"]
            and r["quality"] >= primary["quality"] - QUALITY_TOLERANCE]
    if not cand:
        return None
    return min(cand, key=lambda r: (-r["quality"], r["usd_per_image"], r["cost_rank"]))


def _split(count: int, k: int) -> list[int]:
    """count alternatives across k models, as even as possible, front-loaded."""
    base, extra = divmod(count, k)
    return [base + (1 if i < extra else 0) for i in range(k)]


def allocate(ranked: list[dict], count: int, forced: list[dict] | None = None) -> list[dict]:
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
        b = _secondary(ranked, a)
        models = [a, b] if b else [a]

    ns = _split(count, len(models))
    alloc = []
    for m, n in zip(models, ns):
        if n <= 0:
            continue
        alloc.append({
            "model": m["model"], "label": m["label"], "size": m["size"],
            "usd_per_image": m["usd_per_image"], "n": n,
            "subtotal_usd": round(m["usd_per_image"] * n, 6),
        })
    return alloc


def pick_size(preferred: list[str], size_min_px: int) -> str:
    for s in preferred:
        w, h = _wh(s)
        if w and h and w * h >= size_min_px:
            return s
    w, h = _wh(preferred[0]) if preferred else (1024, 1024)
    w, h = w or 1024, h or 1024
    if w * h >= size_min_px:
        return f"{w}x{h}"
    ar = w / h
    hh = math.ceil(math.sqrt(size_min_px / ar) / 64) * 64
    ww = math.ceil(hh * ar / 64) * 64
    return f"{ww}x{hh}"


def _wh(s: str) -> tuple[int, int]:
    m = re.match(r"^(\d+)x(\d+)$", s.strip())
    return (int(m.group(1)), int(m.group(2))) if m else (0, 0)


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
    intent = match_intent(intent_label, policy)
    ranked = score_models(intent["requirements"], intent["preferred_sizes"], catalog)
    by_id = {r["model"]: r for r in ranked}

    forced = None
    if forced_ids:
        forced = []
        for mid in forced_ids:
            if mid not in by_id:
                die(f"unknown model {mid}. Known: {', '.join(sorted(by_id))}")
            forced.append(by_id[mid])

    alloc = allocate(ranked, count, forced)
    total = round(sum(x["subtotal_usd"] for x in alloc), 6)
    arg = ",".join(f"{x['model']}:{x['n']}:{x['size']}" for x in alloc)
    chosen = {x["model"] for x in alloc}
    dropped = [m for m in (forced_ids or []) if m not in chosen]
    if dropped:
        print(f"note: count {count} is smaller than the {len(forced_ids)} models you named; "
              f"not using {', '.join(dropped)}", file=sys.stderr)
    return {
        "intent": intent["intent"], "count": count, "allocation": alloc,
        "alloc_arg": arg, "estimated_usd": total, "dropped_models": dropped,
        "why": _why(intent, alloc, ranked, catalog, bool(forced)), "ranked": ranked,
    }


def _why(intent: dict, alloc: list[dict], ranked: list[dict], catalog: dict,
         forced: bool) -> str:
    name = f"'{intent['intent']}'"
    labels = [x["label"] for x in alloc]
    if forced:
        return f"you chose {' + '.join(labels)}"
    req = intent["requirements"]
    blocking = sorted({
        d for d in DIMENSIONS if req.get(d) == "high"
        and any(catalog[r["model"]]["scores"].get(d, 0) == -1 for r in ranked)
    })
    ruled = f"; weak-at-{'/'.join(blocking)} models are ruled out" if blocking else ""
    if all(r["disqualified"] for r in ranked):
        return f"no model fully fits {name}; these are the least-bad options"
    if len(labels) == 1:
        return f"{labels[0]} is the best fit for {name}{ruled}"
    return f"{labels[0]} and {labels[1]} both top the fit for {name}{ruled}"


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
        keys = ["intent", "count", "allocation", "alloc_arg",
                "estimated_usd", "why", "sufficient"]
        if plan.get("dropped_models"):
            keys.append("dropped_models")
        out["plan"] = {k: plan[k] for k in keys}
        _print_quote(plan, bal, file=sys.stderr)
        if a.explain:
            print(f"\n  {'model':<36} suit  $/img    size         ok", file=sys.stderr)
            for r in plan["ranked"]:
                print(f"  {r['model']:<36} {r['quality']:>4}  "
                      f"${r['usd_per_image']:<7.4f} {r['size']:<12} "
                      f"{'-' if r['disqualified'] else 'yes'}", file=sys.stderr)
    print(json.dumps(out, indent=2))


def _print_quote(plan: dict, bal: float | None, file) -> None:
    print(f"\nPLAN  {plan['count']} alternative(s) for '{plan['intent']}'", file=file)
    for x in plan["allocation"]:
        print(f"      {x['label']:<18} x{x['n']}   {x['size']:<11}  ~${x['subtotal_usd']:.4f}",
              file=file)
    print(f"      {plan['why']}", file=file)
    est = plan["estimated_usd"]
    if bal is not None and bal < est:
        print(f"\nQUOTE Estimated total: ~${est:.4f}\n"
              f"      Balance: ${bal:.4f}  —  insufficient. Top up before generating.",
              file=file)
    else:
        tail = "" if bal is not None else "  (balance unread — the gateway will confirm)"
        print(f"\nQUOTE Estimated total: ~${est:.4f}   (actual shown after the run){tail}",
              file=file)


def _parse_alloc(s: str, catalog: dict) -> list[dict]:
    """`id:n:WxH,id:n:WxH` -> [{model,label,size,n}], sizes floor-checked."""
    out = []
    for chunk in s.split(","):
        parts = chunk.strip().split(":")
        if len(parts) != 3:
            die(f"bad --alloc segment {chunk!r}; expected model:n:WxH")
        mid, n_s, size = parts[0].strip(), parts[1].strip(), parts[2].strip()
        if mid not in catalog:
            die(f"unknown model {mid}")
        try:
            n = int(n_s)
        except ValueError:
            die(f"bad count in --alloc segment {chunk!r}")
        if n <= 0:
            continue
        w, h = _wh(size)
        if not (w and h):
            die(f"bad size in --alloc segment {chunk!r}")
        floor = catalog[mid]["size_min_px"]
        if w * h < floor:
            size = pick_size([size], floor)
            print(f"note: {mid} needs >= {floor} px; using {size}", file=sys.stderr)
        out.append({"model": mid, "label": catalog[mid]["label"], "size": size, "n": n})
    if not out:
        die("--alloc has no positive-count segments")
    total = sum(x["n"] for x in out)
    if total not in COUNTS:
        die(f"--alloc sums to {total}; must be one of {', '.join(map(str, COUNTS))}")
    return out


def cmd_run(a) -> None:
    if not a.confirm:
        die("run spends money. Re-invoke with --confirm after the user approves the plan.")
    cat = load_model_catalog()
    alloc = _parse_alloc(a.alloc, cat)
    tok = token()

    out_dir = Path(a.out or "./out").resolve()
    out_dir.mkdir(parents=True, exist_ok=True)

    # flat task list, in allocation order; each carries its own model + size
    tasks: list[dict] = []
    for seg in alloc:
        for _ in range(seg["n"]):
            tasks.append(_new_task(LABELS[len(tasks)], seg["model"], seg["label"], seg["size"]))

    def body_for(t: dict) -> dict:
        return {"model": t["model"], "prompt": a.prompt, "size": t["size"], "watermark": False}

    # first submit — a bad first model 503s here, before any spend
    st, first, err = _req("POST", "/tasks/generations", tok, body_for(tasks[0]))
    if st == 503 or (first.get("error") or {}).get("code") == "service_unavailable":
        _suggest_replacement(tasks[0]["model"], first, a.intent)  # exits 3
    if err:
        die(f"router request failed before any spend: {err}")
    rid0 = (first.get("data") or {}).get("request_id")
    if not rid0:
        die(f"router rejected the request (HTTP {st}): {json.dumps(first)[:300]}")
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

    est_by_model = {seg["model"]: model_price(cat[seg["model"]], _px(seg["size"])) for seg in alloc}
    expected = sum(est_by_model[t["model"]] for t in tasks)
    _poll_all(tasks, tok, out_dir)
    _report(tasks, expected, out_dir, cat, prompt=a.prompt, intent=a.intent)


def _px(size: str) -> int:
    w, h = _wh(size)
    return w * h


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


def _new_task(label: str, model: str, model_label: str, size: str) -> dict:
    return {"label": label, "model": model, "model_label": model_label, "size": size,
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
    alt = _primary(score_models(req, sizes, remaining))
    print(json.dumps({
        "status": "model_unavailable",
        "failed_model": bad_model,
        "message": (resp.get("error") or {}).get("message", "model unavailable"),
        "suggested_model": alt["model"],
        "suggested_label": alt["label"],
        "suggested_usd_per_image": alt["usd_per_image"],
        "suggested_size": alt["size"],
    }, indent=2))
    print(f"\nMODEL UNAVAILABLE\n  {bad_model} could not be generated right now.\n"
          f"  Suggested replacement: {alt['label']}  (~${alt['usd_per_image']}/image)\n"
          f"  Re-resolve, then re-run with the new allocation after the user approves.",
          file=sys.stderr)
    sys.exit(3)


def _poll_all(tasks: list[dict], tok: str, out_dir: Path) -> None:
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
            if d.get("cost"):
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
    """Generation time in seconds. Prefer the router's own start/finish epochs
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
            prompt: str | None = None, intent: str | None = None) -> None:
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
    try:
        (out_dir / "run.json").write_text(json.dumps(result, indent=2), encoding="utf-8")
    except OSError as e:
        print(f"note: could not write {out_dir / 'run.json'}: {e}", file=sys.stderr)
    print(json.dumps(result, indent=2))


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

    x = sub.add_parser("run")
    x.add_argument("--alloc", required=True,
                   help='approved allocation: "id:n:WxH,id:n:WxH" (from resolve\'s alloc_arg)')
    x.add_argument("--prompt", required=True)
    x.add_argument("--intent", default=None,
                   help="used to score a replacement if the first model is unavailable")
    x.add_argument("--out", default=None)
    x.add_argument("--confirm", action="store_true")

    a = ap.parse_args()
    {"resolve": cmd_resolve, "run": cmd_run}[a.cmd](a)


if __name__ == "__main__":
    main()
