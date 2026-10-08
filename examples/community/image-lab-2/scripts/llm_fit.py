"""Assistant fit, level 1 (docs/proposal-llm-fit-advisor.md): advise only.

Image Lab controls the image model but not the assistant (the LLM the user's coding agent runs on) that does
the creative, planning and review work. This module compares the user's assistant with the text models the
gateway lists, step by step, from evidence we measured ourselves, and says keep, optional or suggested. It
never changes the host's model and never sends the brief anywhere: level 1 reads the gateway's public model
list (free) and a local evidence file. With no measured evidence it says so; that is the honest first release.

Facts (price, context window, APIs, modality) come from the gateway listing. Quality comes only from evaluation
records in references/llm-fit/records.jsonl; a listing never says how good a model is, and for 34 of 49 models it
does not even say whether it can read images (so modality is "unlisted" until measured).

Paid probes (the pricing check of the proposal's P0 gate) are separate and gated by a quote and a confirmation:
`probe_quote` / `run_probes`.
"""
from __future__ import annotations

import hashlib
import json
import re
import time
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
FIT_DIR = ROOT / "references" / "llm-fit"
RECORDS_FILE = FIT_DIR / "records.jsonl"
SNAPSHOT_FILE = FIT_DIR / "models-snapshot.json"
PROBES_FILE = FIT_DIR / "probes.jsonl"

# The three workflow steps where the assistant's quality matters most (proposal section 1), and the task whose
# evaluation records measure each.
STEPS = ("Creative Direction", "Plan writing", "Result review")
TASK_OF_STEP = {"Creative Direction": "direction", "Plan writing": "plan", "Result review": "vision"}
STEP_NEEDS_IMAGES = {"Result review"}

STRONG_LEVEL, GOOD_LEVEL = 0.8, 0.6          # internal: a score becomes the words Strong / Good / Limited; users never see the numbers
MIN_BRIEFS, MIN_REPEATS = 3, 2               # what a "strong" comparison needs at least


# --------------------------------------------------------------------------- #
# the gateway's model list (facts)
# --------------------------------------------------------------------------- #
def norm(text) -> str:
    return re.sub(r"[^a-z0-9]", "", str(text).lower())


def parse_models(payload: dict) -> list[dict]:
    """The listing, reduced to what advice needs. `modality` is 'unlisted' when the gateway does not say."""
    out = []
    for m in (payload or {}).get("data", []):
        arch = m.get("architecture") or {}
        inp = (arch.get("input") or "").strip()
        pr = m.get("pricing") or {}
        out.append({
            "api_name": m.get("api_name", ""), "company": m.get("company", ""), "name": m.get("name", ""),
            "context_window": m.get("context_window"), "max_tokens": m.get("max_tokens"),
            "input_price": pr.get("input_price"), "output_price": pr.get("output_price"),
            "cached_price": pr.get("cached_price"), "currency": pr.get("currency"),
            "modality": inp or "unlisted", "can_read_images": ("image" in inp) if inp else None,
            "apis": m.get("support_apis") or [],
        })
    return out


def load_models(offline: bool = False, fetch=None):
    """(models, source, fetched_at). Live from the gateway unless offline or the call fails; then the saved snapshot."""
    if not offline:
        try:
            if fetch is None:
                import image as il
                tok = il.token()
                status, js, err = il._req("GET", "/models", tok, None, retries=0)
            else:
                status, js, err = fetch()
            if status == 200 and js.get("data"):
                return parse_models(js), "the gateway, live", time.strftime("%Y-%m-%d %H:%M")
        except SystemExit:
            pass                                    # no token: fall back to the snapshot
        except Exception:
            pass
    if SNAPSHOT_FILE.exists():
        snap = json.loads(SNAPSHOT_FILE.read_text(encoding="utf-8"))
        return snap["models"], "the saved snapshot", snap.get("fetched_at", "unknown date")
    return [], "nothing (no gateway access and no snapshot)", ""


def save_snapshot(models: list[dict], path: Path = SNAPSHOT_FILE) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(json.dumps({"fetched_at": time.strftime("%Y-%m-%d"), "note": "facts from the gateway's /models listing; "
                                "quality is NOT in it. Prices are USD per million tokens (the gateway does not state the unit; confirmed against billed usage on 2026-10-08).",
                                "models": models}, indent=1, ensure_ascii=False), encoding="utf-8")


def map_assistant(name: str | None, models: list[dict]) -> dict:
    """Match the self-reported assistant to a gateway model. Only an exact match (ignoring case, spaces and punctuation, on the
    api name, the display name or either with the company) counts; a version that is merely close is reported as a
    neighbour, never guessed (Claude Sonnet 5.5 is not Claude Sonnet 5)."""
    if not name or not str(name).strip():
        return {"match": None, "how": "not given", "neighbours": []}
    key = norm(name)
    for m in models:
        api_tail = m["api_name"].split("/", 1)[-1]
        forms = {norm(m["api_name"]), norm(api_tail), norm(m["name"]), norm(m["company"] + m["name"]), norm(m["company"] + " " + api_tail)}
        if key in forms:
            return {"match": m, "how": "exact", "neighbours": []}
    near = [m for m in models if key and (key.startswith(norm(m["name"])) or norm(m["name"]).startswith(key)
                                            or key.startswith(norm(m["api_name"].split("/", 1)[-1])))]
    return {"match": None, "how": "not on the gateway", "neighbours": near[:3]}


# --------------------------------------------------------------------------- #
# evidence (quality)
# --------------------------------------------------------------------------- #
def load_records(path: Path = RECORDS_FILE) -> list[dict]:
    if not Path(path).exists():
        return []
    out = []
    for line in Path(path).read_text(encoding="utf-8").splitlines():
        line = line.strip()
        if line:
            try:
                out.append(json.loads(line))
            except ValueError:
                continue                               # a damaged line is ignored, not fatal
    return out


def _agg(records: list[dict]) -> dict:
    """One summary per (model, task): the mean score over its measured records, with the sample behind it."""
    groups: dict = {}
    for r in records:
        if r.get("evidence_type") != "measured" or not isinstance(r.get("score"), (int, float)):
            continue
        groups.setdefault((norm(r["model"]), r["task"]), []).append(r)
    out = {}
    for key, rs in groups.items():
        scores = [float(r["score"]) for r in rs]
        out[key] = {
            "model": rs[0]["model"], "task": key[1], "score": sum(scores) / len(scores),
            "n": max(int(r.get("n") or 0) for r in rs), "repeats": max(int(r.get("repeats") or 1) for r in rs),
            "spread": max(float(r.get("spread") or 0) for r in rs),
            "independent": all(bool(r.get("judge_independent", True)) for r in rs),
            "harness": rs[0].get("harness", "gateway"), "cost_per_use": rs[0].get("cost_per_use"), "date": max(str(r.get("date", "")) for r in rs),
        }
    return out


def level(score: float | None) -> str:
    if score is None:
        return "Not measured"
    return "Strong" if score >= STRONG_LEVEL else "Good" if score >= GOOD_LEVEL else "Limited"


def comparison_tier(host: dict | None, alt: dict) -> str:
    """strong / suggestive / insufficient, for an alternative against the user's assistant."""
    if not host or alt["score"] <= host["score"]:
        return "insufficient"
    diff = alt["score"] - host["score"]
    spread = max(host["spread"], alt["spread"])
    enough = min(host["n"], alt["n"]) >= MIN_BRIEFS and min(host["repeats"], alt["repeats"]) >= MIN_REPEATS
    same_way = host["harness"] == alt["harness"]
    if enough and host["independent"] and alt["independent"] and same_way and spread > 0 and diff > 2 * spread:
        return "strong"
    if diff > spread:
        return "suggestive"
    return "insufficient"


def advise(host_name: str | None, models: list[dict], records: list[dict], can_read_images: str = "unknown") -> dict:
    """The report data: host mapping, host capability, and one row per step."""
    mapped = map_assistant(host_name, models)
    host_key = norm(mapped["match"]["api_name"]) if mapped["match"] else norm(host_name or "")
    host_keys = {host_key} | ({norm(mapped["match"]["name"])} if mapped["match"] else set())
    agg = _agg(records)
    rows = []
    for step in STEPS:
        task = TASK_OF_STEP[step]
        host = next((v for (m, t), v in agg.items() if t == task and m in host_keys), None)
        alts = [v for (m, t), v in agg.items() if t == task and m not in host_keys]
        better = sorted((a for a in alts if (not host or a["score"] > host["score"])), key=lambda a: -a["score"])
        alt = better[0] if better else None
        # models measured at the same level as the host (or higher is "better" above): the cheapest of them is worth knowing about
        ties = sorted((a for a in alts if host and a["score"] <= host["score"] and a["score"] >= host["score"] - max(host["spread"], a["spread"])),
                      key=lambda a: (a.get("cost_per_use") is None, a.get("cost_per_use") or 0)) if host else []
        tier = comparison_tier(host, alt) if alt else "insufficient"
        host_level = level(host["score"] if host else None)
        if step in STEP_NEEDS_IMAGES and can_read_images == "no":
            rec = "suggested: your assistant cannot read image files, so have the images checked another way (your own eyes, or a second opinion)"
        elif tier == "strong":
            rec = "suggested"
        elif tier == "suggestive":
            rec = "optional"
        elif host and host_level in ("Strong", "Good"):
            rec = "keep"
        else:
            rec = "no recommendation yet (insufficient evidence)"
        rows.append({"step": step, "host_level": host_level, "alt": alt, "tier": tier, "recommendation": rec,
                     "measured_host": bool(host), "measured_any": bool(alts) or bool(host),
                     "ties": [{"model": t["model"], "cost_per_use": t.get("cost_per_use")} for t in ties], "n_alts": len(alts)})
    return {"host_name": host_name, "mapped": mapped, "can_read_images": can_read_images, "rows": rows,
            "measured_records": len(records)}


def format_report(rep: dict, source: str = "", fetched: str = "") -> str:
    L = []
    host = rep["host_name"] or "(not given)"
    evid = f"measured, our tests ({rep['measured_records']} records)" if rep["measured_records"] else "none yet: no measured evaluation has been run"
    L.append(f"ASSISTANT FIT   yours (self-reported): {host}     evidence: {evid}")
    m = rep["mapped"]
    if m["match"]:
        mm = m["match"]
        L.append(f"On the gateway as {mm['name']} ({mm['api_name']}): context {mm['context_window']:,} tokens, "
                 f"price {mm['input_price']} in / {mm['output_price']} out ({mm['currency']} per million tokens), images: {mm['modality']}.")
    elif m["how"] == "not given":
        L.append("Your assistant was not named; pass --model \"<name>\" (the agent knows its own model).")
    else:
        near = ", ".join(n["name"] for n in m["neighbours"])
        L.append(f"{host} is not on the gateway" + (f" (closest listed: {near}; a near version is not treated as the same model)" if near else "")
                 + ". It can be measured in your session but not through the gateway.")
    L.append("")
    L.append(f"{'Step':<20} {'Your assistant':<16} {'Best measured alternative':<70} Recommendation")
    for r in rep["rows"]:
        if r["alt"]:
            a = r["alt"]
            alt = f"{a['model']} ({r['tier']} evidence" + (f", about ${a['cost_per_use']} per use" if a.get("cost_per_use") else "") + ")"
        elif r.get("ties"):
            t = r["ties"][0]
            alt = (f"{len(r['ties'])} measured at your level; cheapest {t['model']}"
                   + (f" (about ${t['cost_per_use']} per use)" if t.get("cost_per_use") else ""))
        elif r.get("n_alts"):
            alt = f"{r['n_alts']} measured, none better"
        else:
            alt = "none measured yet"
        L.append(f"{r['step']:<20} {r['host_level']:<16} {alt:<70} {r['recommendation']}")
    L.append("")
    cap = {"yes": "yes", "no": "NO", "unknown": "unknown (pass --can-read-images yes|no)"}[rep["can_read_images"]]
    L.append(f"Host check: can read image files: {cap}. The result review needs it.")
    if not rep["measured_records"]:
        L.append("No recommendation is made without measured evidence. Keep your assistant unless it cannot read images.")
    L.append(f"Model facts from {source or 'unknown'}" + (f" ({fetched})" if fetched else "") + "; prices are USD per million tokens (confirmed against billed usage, 2026-10-08).")
    L.append("Advice only: nothing was sent anywhere and nothing was changed.")
    return "\n".join(L)


# --------------------------------------------------------------------------- #
# the pricing probes (P0): two tiny gateway calls, gated by a quote and a confirmation
# --------------------------------------------------------------------------- #
PROBES = (
    {"id": "text", "model": "openai/gpt-5.6-luna", "max_tokens": 16, "prompt": "Reply with the single word: ok", "image": None},
    {"id": "vision", "model": "google/gemini-3-flash", "max_tokens": 16, "prompt": "Reply with one word naming the main color.", "image": "tiny"},
)


def _est_max_usd(probe: dict, models: list[dict]) -> float:
    m = next((x for x in models if x["api_name"] == probe["model"]), None)
    if not m or m.get("input_price") is None:
        return 0.0
    in_tokens = 60 + (4000 if probe["image"] else 0)             # a generous ceiling for the prompt plus an image
    return round((in_tokens * m["input_price"] + probe["max_tokens"] * (m.get("output_price") or 0)) / 1e6, 6)


def probe_quote(models: list[dict]) -> dict:
    calls = [{"id": p["id"], "model": p["model"], "max_tokens": p["max_tokens"], "est_usd_max": _est_max_usd(p, models)} for p in PROBES]
    total = round(sum(c["est_usd_max"] for c in calls), 6)
    fp = hashlib.sha256(json.dumps(calls, sort_keys=True).encode()).hexdigest()[:12]
    unpriced = [c["model"] for c in calls if not c["est_usd_max"]]
    return {"calls": calls, "total_est_max_usd": total, "fingerprint": fp, "unpriced": unpriced,
            "basis": "listed prices, assuming USD per million tokens (unverified: confirming the unit is what the probe is for)"}


def _tiny_image_data_uri() -> str:
    import base64
    import io
    from PIL import Image
    buf = io.BytesIO()
    Image.new("RGB", (64, 80), (200, 40, 40)).save(buf, "JPEG", quality=70)
    return "data:image/jpeg;base64," + base64.b64encode(buf.getvalue()).decode()


def run_probes(quote: dict, confirm: str, models: list[dict], post=None, now=time.time) -> list[dict]:
    """Run the two probes only when `confirm` equals the quote's fingerprint. Records the raw usage the gateway reports (token
    counts and any cost field), so the next quote can be made from measured numbers."""
    if confirm != quote["fingerprint"]:
        raise PermissionError("the confirmation does not match the quote: nothing was sent")
    if post is None:
        import image as il
        tok = il.token()

        def post(body):
            return il._req("POST", "/chat/completions", tok, body, retries=0)
    results = []
    for p in PROBES:
        content = [{"type": "text", "text": p["prompt"]}]
        if p["image"]:
            content.append({"type": "image_url", "image_url": {"url": _tiny_image_data_uri()}})
        body = {"model": p["model"], "max_tokens": p["max_tokens"], "messages": [{"role": "user", "content": content if p["image"] else p["prompt"]}]}
        t0 = now()
        status, js, err = post(body)
        rec = {"id": p["id"], "model": p["model"], "status": status, "error": err, "seconds": round(now() - t0, 2),
               "usage": (js or {}).get("usage"), "reply": (((js or {}).get("choices") or [{}])[0].get("message") or {}).get("content"),
               "extra": {k: v for k, v in (js or {}).items() if k not in ("choices", "usage", "id", "model", "object", "created")}}
        results.append(rec)
        PROBES_FILE.parent.mkdir(parents=True, exist_ok=True)
        with PROBES_FILE.open("a", encoding="utf-8") as f:
            f.write(json.dumps({"at": time.strftime("%Y-%m-%dT%H:%M:%S"), **rec}, ensure_ascii=False) + "\n")
    return results


# --------------------------------------------------------------------------- #
# the pilot (P0b): one real Direction call on each premium model, to read the real billed tokens
# --------------------------------------------------------------------------- #
PILOT_MODELS = ("anthropic/claude-fable-5", "openai/gpt-6-astra")
PILOT_MAX_TOKENS = 8000
PILOT_EXPECTED_OUT = 3500
PILOT_FILE = FIT_DIR / "pilot.jsonl"
PILOT_BRIEF = FIT_DIR / "briefs" / "skincare-serum.md"
PILOT_INSTRUCTIONS = (
    "You are the planning assistant of Image Lab 2. Follow the skill below exactly. You have no tools here and cannot run commands, so skip any "
    "step that needs one. Reply with (1) the Direction Sheet exactly as Step F describes, and (2) the complete plan.json in ONE ```json block.")


def stream_chat(tok: str, body: dict, timeout: int = 300, opener=None, url: str | None = None) -> tuple[int, dict, str]:
    """POST /chat/completions with `stream: true` and fold the server-sent events into the same shape a plain call returns:
    {choices: [{message: {content}, finish_reason}], usage}. A long reply then does not hit the gateway's 60 second cut for a
    reply that has not started. Never retried: a failure after the request left may still have been billed."""
    import urllib.error
    import urllib.request
    import image as il
    req = urllib.request.Request(
        url or f"{il.GATEWAY}/chat/completions", method="POST",
        data=json.dumps({**body, "stream": True, "stream_options": {"include_usage": True}}).encode(),
        headers={"Authorization": f"Bearer {tok}", "Content-Type": "application/json", "User-Agent": il.USER_AGENT, "Accept": "text/event-stream"})
    parts: list[str] = []
    usage: dict = {}
    finish = None
    extra: dict = {}
    notes: list[str] = []
    try:
        with (opener or il._OPENER).open(req, timeout=timeout) as resp:
            status = resp.status
            for raw in resp:
                line = raw.decode("utf-8", "replace").strip()
                if not line.startswith("data:"):
                    if line and len(notes) < 5:
                        notes.append(line[:200])                  # an "event: error" line or a comment: kept to diagnose an empty reply
                    continue
                data = line[5:].strip()
                if data == "[DONE]":
                    break
                try:
                    ev = json.loads(data)
                except ValueError:
                    if len(notes) < 5:
                        notes.append(data[:200])
                    continue
                if ev.get("error") and len(notes) < 5:
                    notes.append(json.dumps(ev["error"])[:200])
                if ev.get("usage"):
                    usage = ev["usage"]
                for k, v in ev.items():
                    if k not in ("choices", "usage", "id", "model", "object", "created", "system_fingerprint"):
                        extra[k] = v
                for ch in ev.get("choices") or []:
                    parts.append(((ch.get("delta") or {}).get("content")) or "")
                    finish = ch.get("finish_reason") or finish
    except urllib.error.HTTPError as e:
        return e.code, {}, f"HTTP {e.code}"
    except Exception as e:                      # noqa: BLE001  a dropped stream: report what arrived, never retry
        err = f"{type(e).__name__}: {e}"
        if parts or usage:
            return 200, {"choices": [{"message": {"content": "".join(parts)}, "finish_reason": finish or "interrupted"}], "usage": usage, **extra}, err
        return 0, {}, err
    if notes:
        extra["stream_notes"] = notes
    return status, {"choices": [{"message": {"content": "".join(parts)}, "finish_reason": finish}], "usage": usage, **extra}, ""


def pilot_prompt(root: Path = ROOT) -> str:
    skill = (root / "skills" / "direction.md").read_text(encoding="utf-8")
    schema = (root / "references" / "plan-schema.md").read_text(encoding="utf-8")
    brief = PILOT_BRIEF.read_text(encoding="utf-8")
    return f"{PILOT_INSTRUCTIONS}\n\n=== SKILL ===\n{skill}\n\n=== PLAN SCHEMA ===\n{schema}\n\n=== THE USER'S BRIEF ===\n{brief}"


def pilot_quote(models: list[dict], prompt: str | None = None, names: tuple | None = None) -> dict:
    prompt = prompt if prompt is not None else pilot_prompt()
    in_tokens = -(-len(prompt) // 3)                        # a generous 3 characters per token
    calls = []
    for name in (names or PILOT_MODELS):
        m = next((x for x in models if x["api_name"] == name), None)
        pin, pout = (m or {}).get("input_price"), (m or {}).get("output_price")
        if pin is None or pout is None:
            calls.append({"model": name, "est_usd_expected": 0.0, "est_usd_max": 0.0, "unpriced": True})
            continue
        calls.append({"model": name, "in_tokens_est": in_tokens, "max_tokens": PILOT_MAX_TOKENS,
                      "est_usd_expected": round((in_tokens * pin + PILOT_EXPECTED_OUT * pout) / 1e6, 4),
                      "est_usd_max": round((in_tokens * pin + PILOT_MAX_TOKENS * pout) / 1e6, 4), "unpriced": False})
    fp = hashlib.sha256((json.dumps(calls, sort_keys=True) + hashlib.sha256(prompt.encode()).hexdigest()).encode()).hexdigest()[:12]
    return {"calls": calls, "expected_usd": round(sum(c["est_usd_expected"] for c in calls), 4),
            "max_usd": round(sum(c["est_usd_max"] for c in calls), 4), "fingerprint": fp,
            "unpriced": [c["model"] for c in calls if c["unpriced"]], "prompt_chars": len(prompt)}


def _json_block(text: str):
    import re
    blocks = re.findall(r"```json\s*(.*?)```", text or "", re.S)
    if not blocks:
        return None, "no ```json block in the reply"
    try:
        return json.loads(blocks[-1]), ""
    except ValueError as e:
        return None, f"the json block does not parse: {e}"


def pilot_checks(reply: str) -> dict:
    """The mechanical (free) checks of a Direction reply: a parseable plan that passes `check`, and its direction count."""
    import matrix as mx
    plan, err = _json_block(reply)
    if plan is None:
        return {"valid": False, "errors": [err], "directions": 0}
    errs = mx.validate_plan(plan)
    dirs = plan.get("dimensions", {}).get("direction", []) if isinstance(plan.get("dimensions"), dict) else []
    return {"valid": not errs, "errors": errs[:6], "directions": len(dirs) if isinstance(dirs, list) else 0}


def run_pilot(quote: dict, confirm: str, models: list[dict], post=None, now=time.time, prompt: str | None = None,
              names: tuple | None = None) -> list[dict]:
    """Send the pilot only when `confirm` equals the quote's fingerprint (and the quote still matches the prompt and prices); one
    call per model, recorded with its real usage."""
    prompt = prompt if prompt is not None else pilot_prompt()
    if confirm != quote["fingerprint"] or quote["fingerprint"] != pilot_quote(models, prompt, names)["fingerprint"]:
        raise PermissionError("the confirmation does not match the quote: nothing was sent")
    if post is None:
        import image as il
        tok = il.token()

        def post(body):
            return stream_chat(tok, body)
    results = []
    for name in (names or PILOT_MODELS):
        m = next((x for x in models if x["api_name"] == name), {})
        t0 = now()
        status, js, err = post({"model": name, "max_tokens": PILOT_MAX_TOKENS, "messages": [{"role": "user", "content": prompt}]})
        choice = ((js or {}).get("choices") or [{}])[0]
        reply = (choice.get("message") or {}).get("content") or ""
        usage = (js or {}).get("usage") or {}
        pin, pout = m.get("input_price"), m.get("output_price")
        implied = None
        if pin is not None and pout is not None and usage:
            implied = round(((usage.get("prompt_tokens") or 0) * pin + (usage.get("completion_tokens") or 0) * pout) / 1e6, 5)
        rec = {"model": name, "status": status, "error": err, "seconds": round(now() - t0, 1), "usage": usage,
               "finish_reason": choice.get("finish_reason"), "implied_usd": implied, "reply_chars": len(reply),
               "checks": pilot_checks(reply),
               "extra": {k: v for k, v in (js or {}).items() if k not in ("choices", "usage", "id", "model", "object", "created")}}
        results.append(rec)
        (PILOT_FILE.parent / "pilot").mkdir(parents=True, exist_ok=True)
        if reply:
            (PILOT_FILE.parent / "pilot" / (name.replace("/", "__") + ".md")).write_text(reply, encoding="utf-8")
        with PILOT_FILE.open("a", encoding="utf-8") as f:
            f.write(json.dumps({"at": time.strftime("%Y-%m-%dT%H:%M:%S"), **rec}, ensure_ascii=False) + "\n")
    return results


# --------------------------------------------------------------------------- #
# the streaming check: a long reply from the cheap model, to prove streaming gets past the gateway's 60 second cut
# --------------------------------------------------------------------------- #
STREAM_CHECK = {"model": "openai/gpt-5.6-luna", "max_tokens": 3000,
                "prompt": "Write about 1500 words on the history of the lighthouse, in plain paragraphs."}
STREAM_CHECK_LONG = {"model": "openai/gpt-5.6-luna", "max_tokens": 9000,
                     "prompt": "Write about 5000 words on the history of the lighthouse, in plain paragraphs, with no headings and no lists."}


def stream_check_quote(models: list[dict], spec: dict | None = None) -> dict:
    spec = spec or STREAM_CHECK
    m = next((x for x in models if x["api_name"] == spec["model"]), None) or {}
    pin, pout = m.get("input_price"), m.get("output_price")
    usd = None if pin is None or pout is None else round((60 * pin + spec["max_tokens"] * pout) / 1e6, 5)
    fp = hashlib.sha256(json.dumps({**spec, "usd": usd}, sort_keys=True).encode()).hexdigest()[:12]
    return {"model": spec["model"], "max_tokens": spec["max_tokens"], "max_usd": usd, "fingerprint": fp, "prompt": spec["prompt"]}


def run_stream_check(quote: dict, confirm: str, models: list[dict], post=None, now=time.time, spec: dict | None = None) -> dict:
    if confirm != quote["fingerprint"] or quote["fingerprint"] != stream_check_quote(models, spec)["fingerprint"]:
        raise PermissionError("the confirmation does not match the quote: nothing was sent")
    if post is None:
        import image as il
        tok = il.token()

        def post(body):
            return stream_chat(tok, body)
    t0 = now()
    spec = spec or STREAM_CHECK
    status, js, err = post({"model": spec["model"], "max_tokens": spec["max_tokens"],
                            "messages": [{"role": "user", "content": spec["prompt"]}]})
    choice = ((js or {}).get("choices") or [{}])[0]
    reply = (choice.get("message") or {}).get("content") or ""
    rec = {"model": spec["model"], "status": status, "error": err, "seconds": round(now() - t0, 1), "usage": (js or {}).get("usage") or {},
           "finish_reason": choice.get("finish_reason"), "reply_chars": len(reply), "reply": reply[:200] if len(reply) < 200 else None,
           "extra": {k: v for k, v in (js or {}).items() if k not in ("choices", "usage")}}
    PILOT_FILE.parent.mkdir(parents=True, exist_ok=True)
    with PILOT_FILE.open("a", encoding="utf-8") as f:
        f.write(json.dumps({"at": time.strftime("%Y-%m-%dT%H:%M:%S"), "kind": "stream-check", **rec}, ensure_ascii=False) + "\n")
    return rec


def run_stream_check_command(models: list[dict], confirm: str | None, long: bool = False, model: str | None = None) -> str:
    spec = STREAM_CHECK_LONG if long else STREAM_CHECK
    if model:
        spec = {"model": model, "max_tokens": 64, "prompt": "Reply with the single word: ok"}      # a tiny check of one named model
    q = stream_check_quote(models, spec)
    lines = [f"STREAMING CHECK QUOTE (one long reply from {q['model']}, streamed; max_tokens {q['max_tokens']}): at most about ${q['max_usd']}"]
    if not confirm:
        lines.append(f"Fingerprint: {q['fingerprint']}   (run again with --confirm {q['fingerprint']} after you approve the cost)")
        return "\n".join(lines)
    r = run_stream_check(q, confirm, models, spec=spec)
    lines.append(f"  status {r['status']}, {r['seconds']}s, finish {r['finish_reason']}, {r['reply_chars']:,} characters, usage {json.dumps(r['usage'])}"
                 + (f", error {r['error']}" if r["error"] else "") + f", reply {r.get('reply')!r}, extra {json.dumps(r.get('extra'))[:300]}")
    return "\n".join(lines)


def run_pilot_command(models: list[dict], confirm: str | None, only: str | None = None) -> str:
    names = (only,) if only else None
    quote = pilot_quote(models, names=names)
    lines = ["PILOT QUOTE (P0b: one real Creative Direction call on each premium model through the gateway)"]
    for c in quote["calls"]:
        if c["unpriced"]:
            lines.append(f"  {c['model']}: not priced in the listing")
            continue
        lines.append(f"  {c['model']:<26} input about {c['in_tokens_est']:,} tokens; output capped at {c['max_tokens']:,}; "
                     f"expected about ${c['est_usd_expected']:.2f}, at most ${c['est_usd_max']:.2f}")
    lines.append(f"Total: expected about ${quote['expected_usd']:.2f}, ceiling ${quote['max_usd']:.2f} (listed prices, assuming USD per million "
                 f"tokens; reasoning tokens, if any, are billed as output and count against the cap).")
    lines.append(f"It sends the Creative Direction skill, the plan schema and one invented brief ({PILOT_BRIEF.relative_to(ROOT)}), "
                 f"{quote['prompt_chars']:,} characters, to each model's provider; none of your files or briefs.")
    if not confirm:
        lines.append(f"Fingerprint: {quote['fingerprint']}   (run again with --confirm {quote['fingerprint']} after you approve the cost)")
        return "\n".join(lines)
    for r in run_pilot(quote, confirm, models, names=names):
        ck = r["checks"]
        lines.append(f"  {r['model']}: status {r['status']}, {r['seconds']}s, finish {r['finish_reason']}, usage {json.dumps(r['usage'])}, "
                     f"implied ${r['implied_usd']}; plan valid {ck['valid']} ({ck['directions']} directions) {ck['errors'][:2]}")
    lines.append(f"Recorded in {PILOT_FILE.relative_to(ROOT)}; replies in references/llm-fit/pilot/.")
    return "\n".join(lines)


# --------------------------------------------------------------------------- #
# scoring a Direction reply with free, mechanical checks (no judge, no spend)
# --------------------------------------------------------------------------- #
DIRECTION_CHECKS = ("plan block", "plan valid", "3-10 directions", "direction first, batch_by direction", "3+ variable dimensions",
                    "traits on every direction", "copy closing", "keep out in every direction", "dry run", "sheet sections")


def direction_checks(reply: str) -> dict:
    """The mechanical checks of a Creative Direction reply. `score` is the share that pass (0 to 1). Whether the directions are
    also *good* is the judged rubric's job, not this one's."""
    import experiment as ex
    import matrix as mx
    plan, err = _json_block(reply)
    res = {name: False for name in DIRECTION_CHECKS}
    notes: list[str] = []
    if plan is not None and isinstance(plan, dict):
        res["plan block"] = True
        errs = mx.validate_plan(plan)
        res["plan valid"] = not errs
        notes += errs[:4]
        dims = plan.get("dimensions") if isinstance(plan.get("dimensions"), dict) else {}
        dirs = dims.get("direction") if isinstance(dims.get("direction"), list) else []
        res["3-10 directions"] = 3 <= len(dirs) <= 10
        res["direction first, batch_by direction"] = bool(dims) and next(iter(dims)) == "direction" and plan.get("batch_by") == "direction"
        variable = [d for d, v in dims.items() if d not in ("direction", "aspect") and isinstance(v, list) and len(v) >= 2]
        res["3+ variable dimensions"] = len(variable) >= 3
        res["traits on every direction"] = bool(dirs) and all(isinstance(v, dict) and isinstance(v.get("traits"), dict) for v in dirs)
        suffix = str((plan.get("prompt") or {}).get("text_suffix", "")).lower()
        res["copy closing"] = "any other" in suffix or "no other" in suffix
        # a keep-out is a "Keep out:" sentence or a closing list of "no ..." exclusions; the skill asks for the sentence, not for its label
        res["keep out in every direction"] = bool(dirs) and all(
            isinstance(v, dict) and re.search(r"keep out|\bno\b|\bnever\b|\bwithout\b|\bnothing\b", str(v.get("fragment", ""))[-260:], re.I) for v in dirs)
        try:
            ex.preview_plan(plan, 30)
            res["dry run"] = True
        except Exception as e:                  # noqa: BLE001  any refusal is a failed check, not a crash
            notes.append(f"dry run: {str(e)[:120]}")
    else:
        notes.append(err)
    low = (reply or "").lower()
    res["sheet sections"] = all(w in low for w in ("said", "inferred", "open gaps"))
    return {"checks": res, "score": round(sum(res.values()) / len(res), 3), "notes": notes}


def append_record(rec: dict, path: Path | None = None) -> None:
    path = path or RECORDS_FILE
    path.parent.mkdir(parents=True, exist_ok=True)
    with path.open("a", encoding="utf-8") as f:
        f.write(json.dumps(rec, ensure_ascii=False) + "\n")


def score_command(files: list[str]) -> str:
    lines = []
    for f in files:
        r = direction_checks(Path(f).read_text(encoding="utf-8"))
        failed = [k for k, v in r["checks"].items() if not v]
        lines.append(f"{Path(f).name}: {r['score']:.2f}  failed: {', '.join(failed) or 'none'}  {r['notes'][:2] or ''}")
    return "\n".join(lines)


# --------------------------------------------------------------------------- #
# the worker batch (P1b): Creative Direction on the cheap pair and Astra, 3 briefs x 2 runs, gated by one quote
# --------------------------------------------------------------------------- #
EVAL_MODELS = ("openai/gpt-5.6-luna", "google/gemini-3-flash", "openai/gpt-6-astra")
EVAL_MAX_TOKENS = 10000
EVAL_EXPECTED_OUT = 7000                       # measured: Astra 7,233 output tokens on one Direction call
EVAL_RUNS = 2
EVAL_BRIEFS = {"adidas": ROOT / "out" / "llm-fit" / "briefs" / "adidas-collage.txt",
               "skincare": FIT_DIR / "briefs" / "skincare-serum.md",
               "childrens": FIT_DIR / "briefs" / "childrens-book.md"}
REPLIES_DIR = ROOT / "out" / "llm-fit" / "replies"
EVALS_FILE = FIT_DIR / "evals.jsonl"


def reply_path(model: str, brief: str, run: int, root: Path | None = None) -> Path:
    return (root or REPLIES_DIR) / f"{model.replace('/', '__')}__{brief}__r{run}.md"


def eval_prompt(brief: str, root: Path = ROOT) -> str:
    skill = (root / "skills" / "direction.md").read_text(encoding="utf-8")
    schema = (root / "references" / "plan-schema.md").read_text(encoding="utf-8")
    text = EVAL_BRIEFS[brief].read_text(encoding="utf-8")
    return f"{PILOT_INSTRUCTIONS}\n\n=== SKILL ===\n{skill}\n\n=== PLAN SCHEMA ===\n{schema}\n\n=== THE USER'S BRIEF ===\n{text}"


def eval_quote(models: list[dict], replies_dir: Path | None = None) -> dict:
    """Every (model, brief, run) whose reply file does not exist yet, priced from the listing and the measured token sizes."""
    calls, prompts = [], {b: eval_prompt(b) for b in EVAL_BRIEFS}
    for name in EVAL_MODELS:
        m = next((x for x in models if x["api_name"] == name), None) or {}
        pin, pout = m.get("input_price"), m.get("output_price")
        for brief, prompt in prompts.items():
            for run in range(1, EVAL_RUNS + 1):
                if reply_path(name, brief, run, replies_dir).exists():
                    continue
                if pin is None or pout is None:
                    calls.append({"model": name, "brief": brief, "run": run, "expected": 0.0, "max": 0.0, "unpriced": True})
                    continue
                in_tok = len(prompt) / 4.2                                  # measured: 4.28 characters per token
                calls.append({"model": name, "brief": brief, "run": run, "unpriced": False,
                              "expected": round((in_tok * pin + EVAL_EXPECTED_OUT * pout) / 1e6, 5),
                              "max": round((len(prompt) / 3.0 * pin + EVAL_MAX_TOKENS * pout) / 1e6, 5)})
    fp = hashlib.sha256((json.dumps(calls, sort_keys=True) + "".join(hashlib.sha256(p.encode()).hexdigest() for p in prompts.values())).encode()).hexdigest()[:12]
    by_model: dict = {}
    for c in calls:
        d = by_model.setdefault(c["model"], {"calls": 0, "expected": 0.0, "max": 0.0})
        d["calls"] += 1
        d["expected"] += c["expected"]
        d["max"] += c["max"]
    return {"calls": calls, "by_model": by_model, "expected_usd": round(sum(c["expected"] for c in calls), 3),
            "max_usd": round(sum(c["max"] for c in calls), 3), "fingerprint": fp, "unpriced": sorted({c["model"] for c in calls if c["unpriced"]})}


def _live_spend() -> float | None:
    """The gateway's own running total of unsettled usage (GET /balance, locked_balance_cny), or None."""
    try:
        import image as il
        st, js, _ = il._req("GET", "/balance", il.token(), None, retries=0)
        return float(js["data"]["locked_balance_cny"]) if st == 200 else None
    except Exception:                               # noqa: BLE001  advisory only
        return None


def run_eval(quote: dict, confirm: str, models: list[dict], post=None, replies_dir: Path | None = None, workers: int = 3,
             budget: float | None = None, live_spend=_live_spend) -> list[dict]:
    """Run the quoted calls, a few at a time, never past `budget` (default: the quote's ceiling). Replies are saved and mechanically
    scored; usage is recorded. A failed call is recorded and never retried."""
    from concurrent.futures import ThreadPoolExecutor
    import threading
    if confirm != quote["fingerprint"] or quote["fingerprint"] != eval_quote(models, replies_dir)["fingerprint"]:
        raise PermissionError("the confirmation does not match the quote: nothing was sent")
    if post is None:
        import image as il
        tok = il.token()

        def post(body):
            return stream_chat(tok, body)
    budget = quote["max_usd"] if budget is None else budget
    lock = threading.Lock()
    committed = [0.0]
    results: list[dict] = []
    prompts = {b: eval_prompt(b) for b in EVAL_BRIEFS}
    price = {m["api_name"]: (m.get("input_price"), m.get("output_price")) for m in models}

    def one(c: dict) -> dict:
        with lock:                                  # reserve the call's ceiling before sending; skip it if the budget would be passed
            if committed[0] + c["max"] > budget:
                return {**{k: c[k] for k in ("model", "brief", "run")}, "status": "skipped", "error": "budget", "checks": None}
            committed[0] += c["max"]
        t0 = time.time()
        status, js, err = post({"model": c["model"], "max_tokens": EVAL_MAX_TOKENS, "messages": [{"role": "user", "content": prompts[c["brief"]]}]})
        choice = ((js or {}).get("choices") or [{}])[0]
        reply = (choice.get("message") or {}).get("content") or ""
        usage = (js or {}).get("usage") or {}
        pin, pout = price.get(c["model"], (None, None))
        implied = round(((usage.get("prompt_tokens") or 0) * pin + (usage.get("completion_tokens") or 0) * pout) / 1e6, 5) if pin is not None and usage else None
        with lock:
            committed[0] += (implied if implied is not None else 0.0) - c["max"]      # replace the reservation with the real figure
        path = reply_path(c["model"], c["brief"], c["run"], replies_dir)
        if reply:
            path.parent.mkdir(parents=True, exist_ok=True)
            path.write_text(reply, encoding="utf-8")
        ck = direction_checks(reply) if reply else None
        rec = {"model": c["model"], "brief": c["brief"], "run": c["run"], "status": status, "error": err, "seconds": round(time.time() - t0, 1),
               "usage": usage, "finish_reason": choice.get("finish_reason"), "implied_usd": implied, "reply_chars": len(reply),
               "score": ck["score"] if ck else None, "failed": [k for k, v in ck["checks"].items() if not v] if ck else None}
        with lock:
            EVALS_FILE.parent.mkdir(parents=True, exist_ok=True)
            with EVALS_FILE.open("a", encoding="utf-8") as f:
                f.write(json.dumps({"at": time.strftime("%Y-%m-%dT%H:%M:%S"), **rec}, ensure_ascii=False) + "\n")
        return rec

    # cheap models first, so a surprise shows up before the expensive ones are sent
    ordered = sorted(quote["calls"], key=lambda c: (EVAL_MODELS.index(c["model"]), c["brief"], c["run"]))
    before = live_spend()
    with ThreadPoolExecutor(max_workers=workers) as ex_:
        results = list(ex_.map(one, [c for c in ordered if not c["unpriced"]]))
    after = live_spend()
    if before is not None and after is not None:
        results.append({"model": "(all)", "brief": "-", "run": 0, "status": "balance", "error": "", "implied_usd": None,
                        "settled_delta_usd": round(after - before, 4)})
    return results


def run_eval_command(models: list[dict], confirm: str | None) -> str:
    q = eval_quote(models)
    lines = ["WORKER BATCH QUOTE (Creative Direction on the cheap pair and GPT-6 Astra; 3 briefs x 2 runs; streamed; output capped at "
             f"{EVAL_MAX_TOKENS:,} tokens per call; calls already on disk are skipped)"]
    for name, d in q["by_model"].items():
        lines.append(f"  {name:<24} {d['calls']} call(s): expected about ${d['expected']:.2f}, at most ${d['max']:.2f}")
    lines.append(f"Total: expected about ${q['expected_usd']:.2f}, ceiling ${q['max_usd']:.2f} (listed prices, USD per million tokens; the ceiling assumes "
                 f"3 characters per token in and every call hitting the cap; the run also stops before any call that could pass the ceiling).")
    lines.append("It sends the Creative Direction skill, the plan schema and one invented or supplied brief per call to each model's provider; "
                 "no files from your project.")
    if q["unpriced"]:
        lines.append(f"Not priced in the listing (skipped): {', '.join(q['unpriced'])}")
    if not confirm:
        lines.append(f"Fingerprint: {q['fingerprint']}   (run again with --confirm {q['fingerprint']} after you approve the cost)")
        return "\n".join(lines)
    for r in run_eval(q, confirm, models):
        if r["status"] == "balance":
            lines.append(f"Gateway running total moved by ${r['settled_delta_usd']} during the batch (settles with a lag; read it again later).")
            continue
        lines.append(f"  {r['model']:<24} {r['brief']:<9} r{r['run']}: status {r['status']}, {r.get('seconds', '-')}s, finish {r.get('finish_reason')}, "
                     f"implied ${r.get('implied_usd')}, mechanical score {r.get('score')} {r.get('failed') or ''}")
    lines.append(f"Recorded in {EVALS_FILE.relative_to(ROOT)}; replies in out/llm-fit/replies/.")
    return "\n".join(lines)


# --------------------------------------------------------------------------- #
# the blind judge (P1b): one gateway call per sheet, from a family none of the workers belong to
# --------------------------------------------------------------------------- #
JUDGE_MODEL = "x-ai/grok-4.6"
JUDGE_MAX_TOKENS = 1500
JUDGE_EXPECTED_OUT = 3500        # measured (qwen3.7-flash): 3,400 output tokens, 3,320 of them reasoning, past the 1,500 cap
JUDGE_CEILING_OUT = 6000
JUDGE_RUBRIC = FIT_DIR / "judge-rubric.md"
JUDGES_FILE = FIT_DIR / "judgments.jsonl"
JUDGE_KEYS = ("faithfulness", "distinctness", "specificity", "levers", "honesty", "overall")


def judge_prompt(brief_text: str, reply: str, rubric: str | None = None) -> str:
    rubric = rubric if rubric is not None else JUDGE_RUBRIC.read_text(encoding="utf-8")
    return f"{rubric}\n\n=== THE BRIEF ===\n{brief_text}\n\n=== THE REPLY TO SCORE ===\n{reply}"


def judge_items(replies_dir: Path | None = None, seed: int = 20261008, limit: int | None = None) -> list[dict]:
    """The sheets to judge, in a fixed shuffled order with neutral labels (S01...), so the judge's order says nothing about the model."""
    import random
    items = []
    for p in sorted((replies_dir or REPLIES_DIR).glob("*__r*.md")):
        parts = p.stem.rsplit("__r", 1)
        run = int(parts[1])
        head = parts[0].split("__")
        brief = head[-1]
        model = "/".join(head[:-1]) if len(head) == 3 else head[0]
        items.append({"path": p, "model": model, "brief": brief, "run": run})
    random.Random(seed).shuffle(items)
    for i, it in enumerate(items, 1):
        it["label"] = f"S{i:02d}"
    return items[:limit] if limit else items


def judge_quote(models: list[dict], replies_dir: Path | None = None, limit: int | None = None) -> dict:
    m = next((x for x in models if x["api_name"] == JUDGE_MODEL), None) or {}
    pin, pout = m.get("input_price"), m.get("output_price")
    items = judge_items(replies_dir, limit=limit)
    rubric = JUDGE_RUBRIC.read_text(encoding="utf-8")
    total_e = total_m = 0.0
    for it in items:
        chars = len(rubric) + len(EVAL_BRIEFS[it["brief"]].read_text(encoding="utf-8")) + len(it["path"].read_text(encoding="utf-8"))
        if pin is None or pout is None:
            continue
        total_e += (chars / 4.2 * pin + JUDGE_EXPECTED_OUT * pout) / 1e6
        total_m += (chars / 3.0 * pin + JUDGE_CEILING_OUT * pout) / 1e6
    fp = hashlib.sha256((JUDGE_MODEL + "".join(f"{i['label']}{i['path'].name}{i['path'].stat().st_size}" for i in items) + rubric).encode()).hexdigest()[:12]
    return {"model": JUDGE_MODEL, "sheets": len(items), "expected_usd": round(total_e, 3), "max_usd": round(total_m, 3),
            "fingerprint": fp, "unpriced": pin is None or pout is None}


def parse_judgment(text: str, reply: str) -> dict:
    """The judge's scores, checked: all six whole numbers 1 to 5, and its quote really appears in the sheet (a quote that does not
    is flagged, because a judge that invents its evidence cannot be trusted on the numbers either)."""
    import re
    m = re.search(r"\{.*\}", text or "", re.S)
    try:
        js = json.loads(m.group(0)) if m else None
    except ValueError:
        js = None
    if not isinstance(js, dict) or any(not isinstance(js.get(k), int) or isinstance(js.get(k), bool) or not 1 <= js[k] <= 5 for k in JUDGE_KEYS):
        return {"ok": False, "scores": None, "quote_ok": False, "note": "no valid scores"}
    norm_q = re.sub(r"\s+", " ", str(js.get("quote", ""))).strip().lower().strip("\"'")
    norm_r = re.sub(r"\s+", " ", reply).lower()
    quote_ok = len(norm_q) >= 12 and norm_q in norm_r
    return {"ok": True, "scores": {k: js[k] for k in JUDGE_KEYS}, "quote_ok": quote_ok, "weakest": js.get("weakest"), "quote": js.get("quote")}


def run_judge(quote: dict, confirm: str, models: list[dict], post=None, replies_dir: Path | None = None, workers: int = 3,
              limit: int | None = None) -> list[dict]:
    from concurrent.futures import ThreadPoolExecutor
    import threading
    if confirm != quote["fingerprint"] or quote["fingerprint"] != judge_quote(models, replies_dir, limit)["fingerprint"]:
        raise PermissionError("the confirmation does not match the quote: nothing was sent")
    if post is None:
        import image as il
        tok = il.token()

        def post(body):
            return stream_chat(tok, body)
    lock = threading.Lock()

    def one(it: dict) -> dict:
        reply = it["path"].read_text(encoding="utf-8")
        prompt = judge_prompt(EVAL_BRIEFS[it["brief"]].read_text(encoding="utf-8"), reply)
        t0 = time.time()
        status, js, err = post({"model": JUDGE_MODEL, "max_tokens": JUDGE_MAX_TOKENS, "messages": [{"role": "user", "content": prompt}]})
        text = (((js or {}).get("choices") or [{}])[0].get("message") or {}).get("content") or ""
        pj = parse_judgment(text, reply)
        rec = {"label": it["label"], "model": it["model"], "brief": it["brief"], "run": it["run"], "status": status, "error": err, "seconds": round(time.time() - t0, 1),
               "usage": (js or {}).get("usage") or {}, **pj}
        with lock:
            JUDGES_FILE.parent.mkdir(parents=True, exist_ok=True)
            with JUDGES_FILE.open("a", encoding="utf-8") as f:
                f.write(json.dumps({"at": time.strftime("%Y-%m-%dT%H:%M:%S"), **rec}, ensure_ascii=False) + "\n")
        return rec

    with ThreadPoolExecutor(max_workers=workers) as ex_:
        return list(ex_.map(one, judge_items(replies_dir, limit=limit)))


def summarize_judgments(rows: list[dict]) -> dict:
    """Per model: mean of the six scores (0 to 1), per-brief means, spread across briefs, and how many judgments had a supported quote."""
    by: dict = {}
    for r in rows:
        if not r.get("ok"):
            continue
        by.setdefault(r["model"], {}).setdefault(r["brief"], []).append(sum(r["scores"].values()) / (5 * len(r["scores"])))
    out = {}
    for model, briefs in by.items():
        means = {b: sum(v) / len(v) for b, v in briefs.items()}
        allv = [x for v in briefs.values() for x in v]
        out[model] = {"score": round(sum(allv) / len(allv), 3), "n": len(briefs), "repeats": min(len(v) for v in briefs.values()),
                      "spread": round(max(means.values()) - min(means.values()), 3), "per_brief": {b: round(v, 3) for b, v in means.items()},
                      "quote_ok": f"{sum(1 for r in rows if r.get('ok') and r['model'] == model and r.get('quote_ok'))}/{sum(1 for r in rows if r.get('ok') and r['model'] == model)}"}
    return out


def run_judge_command(models: list[dict], confirm: str | None, limit: int | None = None) -> str:
    q = judge_quote(models, limit=limit)
    lines = [f"JUDGE QUOTE ({q['model']}, a family none of the workers belong to; {q['sheets']} sheets, one call each, blind labels S01 to S{q['sheets']:02d}; "
             f"requested cap {JUDGE_MAX_TOKENS:,} tokens, but reasoning tokens are not held to it; the ceiling assumes {JUDGE_CEILING_OUT:,}): expected about ${q['expected_usd']:.2f}, at most ${q['max_usd']:.2f}"]
    if q["unpriced"]:
        lines.append(f"{q['model']} is not priced in the listing: nothing can be quoted")
        return "\n".join(lines)
    lines.append("It sends the rubric, the brief and one saved reply per call to the judge's provider; the judge is not told which assistant wrote it.")
    if not confirm:
        lines.append(f"Fingerprint: {q['fingerprint']}   (run again with --confirm {q['fingerprint']} after you approve the cost)")
        return "\n".join(lines)
    rows = run_judge(q, confirm, models, limit=limit)
    for r in rows:
        lines.append(f"  {r['label']}: status {r['status']}, {r.get('seconds')}s, usage {json.dumps(r.get('usage'))}, ok {r.get('ok')}, quote supported {r.get('quote_ok')}")
    for model, d in sorted(summarize_judgments(rows).items(), key=lambda kv: -kv[1]["score"]):
        lines.append(f"  {model:<28} score {d['score']:.2f}  per brief {d['per_brief']}  spread {d['spread']:.2f}  supported quotes {d['quote_ok']}")
    bad = [r for r in rows if not r.get("ok")]
    if bad:
        lines.append(f"  {len(bad)} judgment(s) could not be read: {', '.join(r['label'] for r in bad)}")
    lines.append(f"Recorded in {JUDGES_FILE.relative_to(ROOT)} (labels S01.. map to the files in out/llm-fit/replies/ by that file).")
    return "\n".join(lines)


# --------------------------------------------------------------------------- #
# the result-review evaluation (P1b, vision): each model reviews the 16 benchmark images one at a time; scored mechanically
# --------------------------------------------------------------------------- #
VISION_MODELS = ("google/gemini-3-flash", "openai/gpt-5.4-mini", "anthropic/claude-haiku-4.5:thinking", "openai/gpt-5.4")
VISION_RUNS = 2
VISION_IMAGE_TOKENS = 1300                      # expected input tokens for one 768 px image plus the prompt (a probe image measured ~1,000)
VISION_EXPECTED_OUT = 400
VISION_CEILING_IN = 2200
VISION_CEILING_OUT = 3000                       # reasoning tokens are not held to max_tokens, so the ceiling is generous
VISION_MAX_TOKENS = 1500
VISION_OUT_DIR = ROOT / "out" / "llm-fit" / "vision"
VISION_FILE = FIT_DIR / "vision-evals.jsonl"


def vision_quote(models: list[dict], names: tuple | None = None, runs: int = VISION_RUNS, bench=None, budget: float | None = None) -> dict:
    import vision_bench as vb
    n_images = len(vb.load_labels(bench))
    calls = []
    for name in (names or VISION_MODELS):
        m = next((x for x in models if x["api_name"] == name), None) or {}
        pin, pout = m.get("input_price"), m.get("output_price")
        if pin is None or pout is None:
            calls.append({"model": name, "calls": 0, "expected": 0.0, "max": 0.0, "unpriced": True})
            continue
        k = n_images * runs
        calls.append({"model": name, "calls": k, "unpriced": False,
                      "expected": round(k * (VISION_IMAGE_TOKENS * pin + VISION_EXPECTED_OUT * pout) / 1e6, 4),
                      "max": round(k * (VISION_CEILING_IN * pin + VISION_CEILING_OUT * pout) / 1e6, 4)})
    fp = hashlib.sha256((json.dumps(calls, sort_keys=True) + str(runs) + str(budget) + vb.PROMPT).encode()).hexdigest()[:12]
    return {"calls": calls, "runs": runs, "budget": budget, "images": n_images, "expected_usd": round(sum(c["expected"] for c in calls), 3),
            "max_usd": round(sum(c["max"] for c in calls), 3), "fingerprint": fp, "unpriced": [c["model"] for c in calls if c["unpriced"]]}


def run_vision(quote: dict, confirm: str, models: list[dict], names: tuple | None = None, post=None, bench=None, out_dir: Path | None = None,
               workers: int = 4, budget: float | None = None) -> dict:
    """One call per (model, run, image), a few at a time; never past `budget`. Answers are saved per model and run, usage is recorded,
    and everything is scored mechanically against the labels. A failed call is recorded and never retried."""
    from concurrent.futures import ThreadPoolExecutor
    import threading
    import vision_bench as vb
    if confirm != quote["fingerprint"] or quote["fingerprint"] != vision_quote(models, names, quote["runs"], bench, quote.get("budget"))["fingerprint"]:
        raise PermissionError("the confirmation does not match the quote: nothing was sent")
    if post is None:
        import image as il
        tok = il.token()

        def post(body):
            return stream_chat(tok, body)
    labels = vb.load_labels(bench)
    bench_dir = Path(bench or vb.BENCH_DIR)
    out = Path(out_dir or VISION_OUT_DIR)
    out.mkdir(parents=True, exist_ok=True)
    budget = (quote.get("budget") or quote["max_usd"]) if budget is None else budget
    per_call_max = {c["model"]: (c["max"] / c["calls"] if c["calls"] else 0.0) for c in quote["calls"]}
    price = {m["api_name"]: (m.get("input_price"), m.get("output_price")) for m in models}
    lock = threading.Lock()
    committed = [0.0]
    jobs = [(c["model"], run, name) for c in quote["calls"] if not c["unpriced"] for run in range(1, quote["runs"] + 1) for name in sorted(labels)]

    def one(job):
        model, run, name = job
        with lock:
            if committed[0] + per_call_max[model] > budget:
                return {"model": model, "run": run, "image": name, "status": "skipped", "error": "budget", "answer": None, "usage": {}, "implied_usd": None}
            committed[0] += per_call_max[model]
        content = [{"type": "text", "text": vb.PROMPT}, {"type": "image_url", "image_url": {"url": vb.image_data_uri(bench_dir / f"{name}.jpg")}}]
        t0 = time.time()
        status, js, err = post({"model": model, "max_tokens": VISION_MAX_TOKENS, "messages": [{"role": "user", "content": content}]})
        text = (((js or {}).get("choices") or [{}])[0].get("message") or {}).get("content") or ""
        usage = (js or {}).get("usage") or {}
        pin, pout = price.get(model, (None, None))
        implied = round(((usage.get("prompt_tokens") or 0) * pin + (usage.get("completion_tokens") or 0) * pout) / 1e6, 6) if pin is not None and usage else None
        with lock:
            committed[0] += (implied if implied is not None else 0.0) - per_call_max[model]
        return {"model": model, "run": run, "image": name, "status": status, "error": err, "seconds": round(time.time() - t0, 1),
                "answer": vb.parse_answer(text), "raw": text[:300] if not vb.parse_answer(text) else None, "usage": usage, "implied_usd": implied}

    before = _live_spend()
    with ThreadPoolExecutor(max_workers=workers) as ex_:
        results = list(ex_.map(one, jobs))
    after = _live_spend()
    scores: dict = {}
    for model in {j[0] for j in jobs}:
        for run in range(1, quote["runs"] + 1):
            rs = [r for r in results if r["model"] == model and r["run"] == run]
            answers = {r["image"]: r["answer"] for r in rs}
            sc = vb.score(answers, labels)
            cost = round(sum(r["implied_usd"] or 0 for r in rs), 4)
            scores.setdefault(model, []).append({**sc, "run": run, "cost_usd": cost, "skipped": sum(1 for r in rs if r["status"] == "skipped")})
            (out / f"{model.replace('/', '__')}__r{run}.json").write_text(
                json.dumps({r["image"]: r["answer"] for r in rs}, indent=1, ensure_ascii=False), encoding="utf-8")
    VISION_FILE.parent.mkdir(parents=True, exist_ok=True)
    with VISION_FILE.open("a", encoding="utf-8") as f:
        for r in results:
            f.write(json.dumps({"at": time.strftime("%Y-%m-%dT%H:%M:%S"), **{k: v for k, v in r.items() if k != "answer"}, "ok": bool(r["answer"])}, ensure_ascii=False) + "\n")
    return {"scores": scores, "results": results, "settled_delta_usd": None if before is None or after is None else round(after - before, 4)}


def run_vision_command(models: list[dict], confirm: str | None, names: tuple | None = None, budget: float | None = None) -> str:
    q = vision_quote(models, names, budget=budget)
    lines = [f"VISION REVIEW QUOTE (result review: each model looks at {q['images']} benchmark images one at a time, {q['runs']} runs each; "
             f"images at 768 px; one call per image, streamed)"]
    for c in q["calls"]:
        lines.append(f"  {c['model']:<38} {c['calls']} call(s): expected about ${c['expected']:.2f}, at most ${c['max']:.2f}" if not c["unpriced"]
                     else f"  {c['model']}: not priced in the listing (skipped)")
    lines.append(f"Total: expected about ${q['expected_usd']:.2f}, ceiling ${q['max_usd']:.2f} (listed prices, USD per million tokens; the ceiling assumes 2,200 tokens in "
                 f"and 3,000 out per call; reasoning tokens are not held to the cap).")
    lines.append(f"Spending cap for this run: ${q['budget'] if q['budget'] else q['max_usd']:.2f}: the run reserves each call's ceiling before sending it and "
                 f"skips any call that could pass the cap (cheapest models go first, so the most expensive one is skipped first).")
    lines.append("It sends one benchmark image (an invented children's-library poster) and the review instruction per call to each model's provider; nothing from your project.")
    if not confirm:
        lines.append(f"Fingerprint: {q['fingerprint']}   (run again with --confirm {q['fingerprint']} after you approve the cost)")
        return "\n".join(lines)
    res = run_vision(q, confirm, models, names)
    for model, runs in res["scores"].items():
        for sc in runs:
            f = lambda x: "n/a" if x is None else f"{x:.2f}"          # noqa: E731
            lines.append(f"  {model:<38} run {sc['run']}: balanced {f(sc['balanced'])}  recall {f(sc['recall'])}  false alarms {f(sc['false_alarm'])}  "
                         f"transcription {f(sc['transcription'])}  answered {sc['answered']}  cost ${sc['cost_usd']:.3f}")
    if res["settled_delta_usd"] is not None:
        lines.append(f"Gateway running total moved by ${res['settled_delta_usd']} during the run (it settles with a lag).")
    lines.append(f"Answers in {VISION_OUT_DIR.relative_to(ROOT)}; per-call usage in {VISION_FILE.relative_to(ROOT)}.")
    return "\n".join(lines)


# --------------------------------------------------------------------------- #
# the result-review benchmark (free scoring; the gateway runner is separate)
# --------------------------------------------------------------------------- #
def score_vision_command(files: list[str]) -> str:
    """Score saved reviewer answers ({image name: answer}) against the benchmark labels."""
    import vision_bench as vb
    labels = vb.load_labels()
    lines = []
    for f in files:
        raw = json.loads(Path(f).read_text(encoding="utf-8"))
        answers = {k: (vb.parse_answer(json.dumps(v)) if not isinstance(v, str) else vb.parse_answer(v)) for k, v in raw.items()}
        r = vb.score(answers, labels)
        fmt = lambda x: "n/a" if x is None else f"{x:.2f}"          # noqa: E731
        lines.append(f"{Path(f).name}: balanced {fmt(r['balanced'])}  recall {fmt(r['recall'])}  false alarms {fmt(r['false_alarm'])}  "
                     f"transcription {fmt(r['transcription'])}  answered {r['answered']}")
        lines.append(f"    by defect {r['by_defect']}  missed {r['missed']}  false alarms {r['false_alarms']}  autocorrected {r['autocorrected']}")
    return "\n".join(lines)


# --------------------------------------------------------------------------- #
def run(args) -> str:
    """The `llm-advice` command."""
    models, source, fetched = load_models(offline=getattr(args, "offline", False))
    if getattr(args, "probe_pricing", False):
        quote = probe_quote(models)
        lines = ["PRICING PROBE QUOTE (the P0 gate of the Assistant fit proposal; two tiny paid calls through the gateway)"]
        for c in quote["calls"]:
            lines.append(f"  {c['id']:<7} {c['model']:<26} max_tokens {c['max_tokens']:<3} at most about ${c['est_usd_max']:.6f}")
        lines.append(f"Estimated ceiling: ${quote['total_est_max_usd']:.6f}  ({quote['basis']})")
        if quote["unpriced"]:
            lines.append(f"Not priced in the listing: {', '.join(quote['unpriced'])}")
        lines.append("It sends two fixed one-line prompts (and one 64x80 image) to the model providers; none of your files or briefs.")
        confirm = getattr(args, "confirm", None)
        if not confirm:
            lines.append(f"Fingerprint: {quote['fingerprint']}   (run again with --confirm {quote['fingerprint']} after you approve the cost)")
            return "\n".join(lines)
        res = run_probes(quote, confirm, models)
        for r in res:
            lines.append(f"  {r['id']:<7} status {r['status']}  {r['seconds']}s  usage {json.dumps(r['usage'])}  extra {json.dumps(r['extra'])[:120]}")
        lines.append(f"Recorded in {PROBES_FILE.relative_to(ROOT)}. Read the usage to confirm the price unit and how an image is charged.")
        return "\n".join(lines)
    if getattr(args, "vision_eval", False):
        names = tuple(x.strip() for x in args.vision_models.split(",") if x.strip()) if getattr(args, "vision_models", None) else None
        return run_vision_command(models, getattr(args, "confirm", None), names, getattr(args, "budget", None))
    if getattr(args, "score_vision", None):
        return score_vision_command(args.score_vision)
    if getattr(args, "judge_model", None):
        globals()["JUDGE_MODEL"] = args.judge_model
    if getattr(args, "judge", False):
        return run_judge_command(models, getattr(args, "confirm", None), getattr(args, "limit", None))
    if getattr(args, "eval", False):
        return run_eval_command(models, getattr(args, "confirm", None))
    if getattr(args, "score", None):
        return score_command(args.score)
    if getattr(args, "stream_check", False):
        return run_stream_check_command(models, getattr(args, "confirm", None), getattr(args, "long", False), getattr(args, "check_model", None))
    if getattr(args, "pilot", False):
        return run_pilot_command(models, getattr(args, "confirm", None), getattr(args, "check_model", None))
    rep = advise(getattr(args, "model", None), models, load_records(), getattr(args, "can_read_images", "unknown"))
    if getattr(args, "json", False):
        return json.dumps(rep, indent=1, ensure_ascii=False, default=str)
    return format_report(rep, source, fetched)
