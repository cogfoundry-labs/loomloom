#!/usr/bin/env python3
"""Image Lab 2 experiment commands (M1; design-v2.md sections 5 and 7). No gateway calls.

  python experiment.py plan      --plan plan.json --out <dir> [--target 30]
        validate the plan, enumerate valid combinations, recommend a pairwise-covering first
        batch, write plan.json + ledger.json + experiment.xlsx (recommended rows ticked).
  python experiment.py preflight --dir <dir>
        read the workbook, validate, compile, price; write snapshots/<fingerprint>.json.
  python experiment.py refresh   --dir <dir>
        read the workbook (read-only), merge user edits, keep experiment.prev.xlsx,
        write a fresh workbook from the ledger.
  python experiment.py retry     --dir <dir> [--include-unknown]
        preflight for rows in Failed/Partial: only the missing samples; writes a snapshot.
  python experiment.py quick     --prompt TEXT --intent LABEL --count N [--models a,b] --out <dir>
        quick mode: an implicit experiment whose only dimension is the model (v0.1 behavior).
        Writes ledger.json + a snapshot; no workbook.
  python experiment.py run       --dir <dir> --confirm <fingerprint> [--max-usd N] [--again]
        generate the approved snapshot (this spends money).
  python experiment.py check     [--plan plan.json]
        no-spend consistency checks of the M1 data files (and a plan, if given).

All of these are also subcommands of image.py.
"""
from __future__ import annotations

import argparse
import difflib
import json
import re
import shutil
import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))

import compile as cp  # noqa: E402
import ledger as lg  # noqa: E402
import matrix as mx  # noqa: E402
import preflight as pf  # noqa: E402
import workbook as wbk  # noqa: E402

WORKBOOK_ROW_CAP = 5000


def row_mode(plan: dict, row: dict) -> str:
    return cp.mode_for_row(plan, row)


def compile_prompts(plan: dict, ledger: dict) -> dict:
    cat = cp.load_catalog()
    out = {}
    for row in ledger["rows"]:
        if row["status"] == "Removed":
            continue
        params = {d: v for d, v in row["params"].items() if v}
        ids, _ = cp.resolve_reference(plan, row)
        role = next((r.get("role") for r in plan.get("references", []) if ids and r["id"] == ids[0]), None)
        out[row["id"]] = cp.compile_prompt(plan, params, row_mode(plan, row), cat, role)["prompt"]
    return out


def check_intent(plan: dict) -> None:
    """An unknown intent label silently fell back to `generic` in the Advisor; refuse it instead."""
    known = [p["intent"] for p in pf.il.load_policy()]
    if plan["intent"] not in known:
        raise mx.PlanError(f"intent {plan['intent']!r} is not one of the known intents: " + "; ".join(known))


def build_experiment(plan_path, out_dir, target: int = 30, dry_run: bool = False) -> dict:
    """Validate a plan, enumerate, recommend a covering batch and (unless `dry_run`) write
    plan.json, ledger.json and experiment.xlsx. A dry run touches no file: it is how a planner
    checks a draft plan and shows the user the size of the experiment before confirming it."""
    plan_path, out = Path(plan_path), Path(out_dir)
    plan = mx.load_plan(plan_path)
    check_intent(plan)
    if dry_run:                                      # nothing is written or sent, so no consent is needed yet
        return preview_plan(plan, target, plan_path.parent)
    if (out / "ledger.json").exists():
        raise mx.PlanError(f"{out} already holds an experiment (ledger.json). `plan` starts a new experiment and "
                           f"would destroy that one's history (attempts, request ids, costs). Use a new --out; to "
                           f"change this one, edit experiment.xlsx and run `preflight`.")
    lg.assert_unlocked(out)
    notice = mx.consent_problem(plan)
    if notice:
        raise mx.PlanError(notice + " Once the user confirms, record it with `image.py acknowledge-person "
                           "--plan <plan.json>`.")
    out.mkdir(parents=True, exist_ok=True)
    if plan_path.resolve() != (out / "plan.json").resolve():
        shutil.copy2(plan_path, out / "plan.json")
    for ref in plan.get("references", []):                 # bring reference files next to the experiment
        dst, src = out / ref["file"], plan_path.parent / ref["file"]
        if not dst.exists() and src.exists():
            dst.parent.mkdir(parents=True, exist_ok=True)
            shutil.copy2(src, dst)

    adv = pf.Advisor(plan["intent"])
    extra = None
    if plan.get("model_strategy", "single") == "spread":
        support = pf.load_reference_support()
        ok = ({m for m, v in support.items() if v.get("state") in pf.REF_OK
               and (v.get("identity") or {}).get("result") != "changed"} if plan.get("references") else None)
        extra = {"model": adv.top_models(2, ok)}
    dims = mx.dimensions(plan, extra)
    result = mx.cover_for_plan(plan, dims, target)
    valid = mx.enumerate_valid(dims, mx.excludes(plan))
    recommended = set(result["rows"])
    write_all = len(valid) <= WORKBOOK_ROW_CAP
    chosen = valid if write_all else result["rows"]

    names = [d for d in dims if d != "model"]
    ledger = lg.new_ledger(plan, names, [])
    for combo in chosen:
        full = dict(zip(dims, combo))
        model = full.pop("model", None)
        lg.add_row(ledger, full, selected=combo in recommended, model=model)
    takes = plan.get("takes", 1)
    if takes > 1:                                          # more images of the ticked rows are more rows (Take 2, 3)
        lg.add_takes(ledger, [r["id"] for r in list(ledger["rows"]) if r["selected"]], takes - 1)
    lg.save(out / "ledger.json", ledger)

    est = pf.run_preflight(out, advisor=adv, write=False)   # estimate only; touches no file
    prompts = compile_prompts(plan, ledger)
    wb = wbk.write_workbook(out / "experiment.xlsx", ledger, plan, prompts=prompts, models=sorted(adv.catalog))
    if Path(wb).resolve() == (out / "experiment.xlsx").resolve():
        ledger["experiment"]["main_workbook_rows"] = [r["id"] for r in ledger["rows"]]
        lg.save(out / "ledger.json", ledger)
    return {"plan": plan, "result": result, "valid": len(valid), "rows_written": len(ledger["rows"]),
            "recommended": len(recommended), "takes": takes, "estimate": est, "workbook": wb, "dims": dims}


def il_shape_q(entry: dict) -> list:
    return pf.il._shape(entry)["quality_values"]


def preview_plan(plan: dict, target: int = 30, plan_dir=None) -> dict:
    """The numbers a user needs to confirm a plan, with no file written and no gateway call."""
    adv = pf.Advisor(plan["intent"])
    extra = {"model": adv.top_models(2)} if plan.get("model_strategy", "single") == "spread" else None
    dims = mx.dimensions(plan, extra)
    result = mx.cover_for_plan(plan, dims, target)
    valid = mx.enumerate_valid(dims, mx.excludes(plan))
    mode = "reference" if plan.get("references") else "text"
    support = pf.load_reference_support()
    allowed = ({m for m, v in support.items() if v.get("state") in pf.REF_OK
                and (v.get("identity") or {}).get("result") != "changed"} if mode == "reference" else None)
    strategy = plan.get("model_strategy", "single")
    cat = cp.load_catalog()
    ids = [r["id"] for r in plan.get("references", [])]
    role0 = (plan.get("references") or [{}])[0].get("role") if ids else None
    longest = {d: max(vals, key=lambda v: len(cp.fragment(plan, d, v["value"] if isinstance(v, dict) else v, mode, cat)[0]))
               for d, vals in plan["dimensions"].items() if d != "aspect"}
    longest = {d: (v["value"] if isinstance(v, dict) else v) for d, v in longest.items()}
    worst_chars = len(cp.compile_prompt(plan, longest, mode, cat, role0)["prompt"])   # the longest prompt any row can have
    best = adv.best(allowed, prompt_chars=worst_chars)
    if strategy.startswith("fixed:"):                    # the plan names the model: price that one, not the Advisor's pick
        fixed_id = strategy[6:]
        if fixed_id not in adv.catalog:
            raise mx.PlanError(f"model_strategy names {fixed_id!r}, which is not in the model catalog "
                               f"(known: {', '.join(sorted(adv.catalog))})")
        best = next((r for r in adv.ranked if r["model"] == fixed_id), None) or best
        if best and best["model"] != fixed_id:
            best = {**best, "model": fixed_id}
    takes = plan.get("takes", 1)
    n_images = len(result["rows"]) * takes
    known, unpriced, each, basis = 0.0, 0, None, None
    if best:
        size = adv.size_for(best["model"], None)
        quality = plan.get("quality") if plan.get("quality") not in (None, "auto") else None
        usd, src = adv.price(best["model"], size, mode, support, quality)
        if quality and best["model"] in adv.catalog and quality not in il_shape_q(adv.catalog[best["model"]]):
            raise mx.PlanError(f"{best['model']} has no quality setting {quality!r}")
        if usd is None:
            unpriced = n_images
        else:
            each, basis = usd, f"{mode} call, {src} price"
        if "model" in dims:                              # spread: every model prices its own rows
            per_model: dict[str, int] = {}
            for combo in result["rows"]:
                m = dict(zip(dims, combo))["model"]
                per_model[m] = per_model.get(m, 0) + takes
            known, unpriced = 0.0, 0
            for m, n_rows in per_model.items():
                u, _ = adv.price(m, adv.size_for(m, None), mode, support, plan.get("quality"))
                if u is None:
                    unpriced += n_rows
                else:
                    known += u * n_rows
            known = round(known, 4)
            basis = f"{mode} calls, per model (spread)"
        elif usd is not None:
            known = round(usd * n_images, 4)
    # plan-level problems a planner should fix or disclose before the user confirms, grouped per dimension
    warns = []
    by = plan.get("batch_by")
    if by and not all(isinstance(v, dict) and isinstance(v.get("traits"), dict) for v in plan["dimensions"].get(by, [])):
        warns.append(f"the {by} dimension has no `traits` on every value, so nothing checks that its values look different "
                     f"(see skills/direction.md); a plan whose directions share a layout renders as the same picture")
    limited = sorted(m for m in adv.catalog if adv.prompt_limit(m) and worst_chars > adv.prompt_limit(m))
    if limited:
        warns.append(f"the longest prompt is about {worst_chars:,} characters; {', '.join(limited)} accept at most "
                     f"{adv.prompt_limit(limited[0]):,} and would be refused, so they are not auto-picked")
    if best and adv.prompt_limit(best["model"]) and worst_chars > adv.prompt_limit(best["model"]):
        warns.append(f"{best['model']} accepts at most {adv.prompt_limit(best['model']):,} characters but the prompt is "
                     f"about {worst_chars:,}: the rows would be refused at preflight")
    if mx.consent_problem(plan):
        warns.append("a person is pictured: show the person notice with the confirmation; `plan --out` stays "
                     "refused until the user confirms and `acknowledge-person` is run")
    for d, vals in plan["dimensions"].items():
        if d == "aspect":
            continue
        close = [] if d in cat else difflib.get_close_matches(d, list(cat), n=1, cutoff=0.6)
        if close:
            warns.append(f'{d}: not a dimension of the controls catalog, so its values get no starter wording; did you mean '
                         f'{close[0]!r}? (`controls` lists the catalog). If {d!r} is your own dimension, give each value a `fragment`')
        groups: dict[str, list[str]] = {}
        for v in vals:
            name = v["value"] if isinstance(v, dict) else v
            text, src = cp.fragment(plan, d, name, mode, cat)
            if src == "generic":
                groups.setdefault("has no wording (give it a fragment)", []).append(name)
            elif cp.validation(plan, d, name, cat).startswith("unvalidated"):
                groups.setdefault("uses starter wording that has not been tested", []).append(name)
            elif cp.validation(plan, d, name, cat) == "plan":
                groups.setdefault("uses wording written in the plan (untested unless you tested it)", []).append(name)
            if cp.reliability(plan, d, name, mode, cat) != "ok":
                groups.setdefault(f"is unreliable in {mode} mode", []).append(name)
        for what, names in groups.items():
            if len(names) > 1:
                what = what.replace("uses ", "use ").replace("has ", "have ").replace("is ", "are ", 1)
                warns.append(f'{d}: {", ".join(repr(n) for n in names)} {what}')
            else:
                warns.append(f'{d} {names[0]!r} {what}')
    for d, vals in plan["dimensions"].items():
        for v in vals:
            if isinstance(v, dict) and v.get("fragment") and v["value"] in cat.get(d, {}):
                warns.append(f'{d} {v["value"]!r} has the same name as a catalog value, so your wording replaces the '
                             f'catalog wording (which is {cat[d][v["value"]].get("validation", "unvalidated")}); '
                             f'rename it if you did not mean to override it')
    ref_ids = [r["id"] for r in plan.get("references", [])]
    for key, text in (plan.get("fixed") or {}).items():
        hit = [i for i in ref_ids if re.search(rf"\b{re.escape(i)}\b", str(text))]
        if hit:
            warns.append(f'fixed "{key}" mentions {hit[0]}: Fixed text goes into the prompt as written and the model '
                         f'only sees "the reference image", so say "the product in the reference image" instead')
    if len(valid) > 150:
        warns.append(f"{len(valid)} valid combinations is wide; consider fewer dimensions or values "
                     f"(the first batch is still about {len(result['rows'])} rows)")
    plan_dir = Path(plan_dir) if plan_dir else None
    if plan_dir:
        for ref in plan.get("references", []):
            if not (plan_dir / ref["file"]).exists():
                warns.append(f"reference file {ref['file']} was not found next to the plan; `plan --out` copies it, "
                             f"so put it there first")
    wording, sizes = [], []
    for d, vals in plan["dimensions"].items():
        if d == "aspect":
            continue
        for v in vals:
            name = v["value"] if isinstance(v, dict) else v
            wording.append((d, name, cp.fragment(plan, d, name, mode, cat)[0]))
    if best:
        ratios = [v["value"] if isinstance(v, dict) else v for v in plan["dimensions"].get("aspect", [])] or [None]
        for ratio in ratios:
            sz = adv.size_for(best["model"], ratio)
            got = sz.get("aspect_ratio")
            if not got and sz["token"] and "x" in str(sz["token"]):
                w, h = pf.il._wh(sz["token"])
                got = pf.ratio_text(w, h) if w and h else None
            sizes.append((ratio, sz["token"] or "default", got))
    sample = None
    if result["rows"]:
        row = dict(zip(dims, result["rows"][0]))
        row.pop("model", None)
        ids = [r["id"] for r in plan.get("references", [])]
        role = (plan.get("references") or [{}])[0].get("role") if ids else None
        sample = cp.compile_prompt(plan, {d: v for d, v in row.items() if v}, mode, cat, role)["prompt"]
    return {"plan": plan, "result": result, "valid": len(valid), "rows_written": 0,
            "recommended": len(result["rows"]), "takes": takes, "dims": dims, "dry_run": True, "workbook": None,
            "estimate": {"known_usd": known, "unverified_rows": list(range(unpriced))},
            "model": best["model"] if best else None, "warnings": warns,
            "price_each": each, "price_basis": basis, "sample_prompt": sample, "mode": mode,
            "wording": wording, "sizes": sizes, "visual_checks": plan.get("visual_checks", []),
            "traits": [(d, mx.value_name(v), dict(v["traits"])) for d, vals in plan["dimensions"].items() for v in vals
                       if isinstance(v, dict) and isinstance(v.get("traits"), dict)],
            "flags": [(d, mx.value_name(v), v.get("wildcard") is True, list(v.get("relaxes") or []))
                      for d, vals in plan["dimensions"].items() for v in vals if isinstance(v, dict)
                      and (v.get("wildcard") is True or v.get("relaxes"))],
            "quality": plan.get("quality") if plan.get("quality") not in (None, "auto") else None}


def format_plan_summary(s: dict) -> str:
    r, est = s["result"], s["estimate"]
    cost = f"${est['known_usd']:.4f} known"
    if est["unverified_rows"]:
        cost += f" · {len(est['unverified_rows'])} rows price unverified"
    dims = s["dims"]
    covers = " × ".join(f"{len(v)} {d}" for d, v in dims.items())
    lines = [
        f"Valid combinations:  {r['valid_count']:,}   (excluded by rules: {r['excluded_count']:,})",
        (f"Recommended batch:   {s['recommended']} rows, one image each = {s['recommended']} images"
         if s.get("takes", 1) == 1 else
         f"Recommended batch:   {s['recommended']} configurations × {s['takes']} takes = {s['recommended'] * s['takes']} images "
         f"(one row per image)"),
        f"Estimated cost:      {cost}",
        f"Covers:              {covers}",
        (f"Why {s['recommended']}:".ljust(21) + f"every pair of values appears at least once ({r['covering_min']} needed); "
         f"the rest is spare") if not r.get("strata") else
        (f"Why {s['recommended']}:".ljust(21) + f"split evenly across {r['by']} ("
         + ", ".join(f"{v}: {x['rows']}" for v, x in r["strata"].items()) + "), each with its own covering design; "
         + ("; ".join(f"{v} has NO valid combination and gets no images" for v, x in r["strata"].items() if x.get("empty")) + "; "
            if any(x.get("empty") for x in r["strata"].values()) else "")
         + ("every other value's own pairs are covered" if all(x["complete"] or x.get("empty") for x in r["strata"].values()) else
            "a value whose share is below its covering minimum (" + ", ".join(
                f"{v} needs {x['covering_min']}" for v, x in r["strata"].items() if not x["complete"])
            + ") shows only the pairs that fit")),
        f"Pairs:               {r['coverage']['covered']} of {r['pairs']['coverable']} coverable covered"
        + (f"; {r['pairs']['impossible']} pairs can never occur (excluded by rules)" if r["pairs"]["impossible"] else ""),
        (f"Workbook:            {s['workbook']}  ({s['rows_written']} rows, {s['recommended'] * s.get('takes', 1)} ticked)"
         if not s.get("dry_run") else f"Dry run: nothing was written. Model for the estimate: {s.get('model')}"),
    ]
    if s.get("dry_run"):
        n_un = len(est["unverified_rows"])
        if s.get("price_each") is not None:
            lines.append(f"Price basis:         ${s['price_each']:.4f} per image ({s['price_basis']}); "
                         f"{n_un} row(s) with no verified price")
        else:
            why = ("no observed price for this model, size and quality yet (the first image sets it; pass --max-usd)"
                   if s.get("quality") else "no verified price for this model and mode")
            lines.append(f"Price basis:         {why}; all {n_un} row(s) unverified")
        if s.get("quality"):
            lines.append(f"Quality:             {s['quality']} (sent to every request)")
        for ratio, token, got in s.get("sizes", []):
            ask = f"{ratio} requested -> " if ratio else "default size -> "
            if token == "default" and not got:
                shown = "the model's default (this model has no size or ratio control, so the shape cannot be requested)"
            else:
                shown = f"{token} ({got or 'unknown ratio'})"
            note = "" if not ratio or ratio == got or token == "default" else "  (not exact on this model: the ratio differs)"
            lines.append(f"Size on {s.get('model')}: {ask}{shown}{note}")
        if s.get("wording"):
            lines.append(f"Wording used ({s['mode']} mode):")
            for d, name, text in s["wording"]:
                lines.append(f"  {d} / {name}: {text}")
        if s.get("sample_prompt"):
            lines.append(f"Sample prompt (first row, {s['mode']} mode):\n  {s['sample_prompt']}")
        for d, name, tr in s.get("traits", []):
            lines.append(f"Look ({d}):".ljust(21) + f"{name[:30]}: " + " | ".join(tr[k] for k in mx.TRAIT_KEYS))
        for d, name, wild, relaxes in s.get("flags", []):
            lines.append(f"Wildcard:            {d} = {name}" + (f" (relaxes: {'; '.join(relaxes)}, needs the user's OK)" if relaxes else ""))
        if s.get("visual_checks"):
            lines.append("Check by eye on the first batch: " + "; ".join(s["visual_checks"]))
    for w in s.get("warnings", []):
        lines.append(f"Check:               {w}")
    return "\n".join(lines)


def build_quick(prompt: str, intent: str | None, count: int, out_dir, models: list[str] | None = None) -> dict:
    """Quick mode (design 5, 7): the model is the only dimension. v0.1's allocation decides which
    models and how many samples each; the prompt is used verbatim. Writes ledger.json and a
    snapshot (no plan.json, no workbook); `run` then works exactly as for an experiment."""
    if count not in pf.il.COUNTS:
        raise mx.PlanError(f"count must be one of {', '.join(map(str, pf.il.COUNTS))}")
    out = Path(out_dir)
    out.mkdir(parents=True, exist_ok=True)
    with lg.writer_lock(out, "quick"):
        return _build_quick(prompt, intent, count, out, models)


def _build_quick(prompt, intent, count, out, models) -> dict:
    plan = pf.il.plan_for(intent, count, models)
    ledger_path = out / "ledger.json"
    ledger = lg.load(ledger_path) if ledger_path.exists() else lg.new_ledger(
        {"brief": prompt, "intent": plan["intent"]}, [], [])
    snapshot_rows, known, unverified = [], 0.0, []
    for seg in plan["allocation"]:
        priced = seg.get("price_source") != "unverified"
        per_image = round(seg["subtotal_usd"] / seg["n"], 6) if priced else None
        size = None if seg["size"] in (None, "default") else seg["size"]
        for take in range(1, seg["n"] + 1):                 # a row is one image: n images are n rows, Take 1..n
            row = lg.add_row(ledger, {}, selected=True, model=seg["model"], take=take, prompt=prompt)
            row["status"] = "Ready"
            known += per_image or 0.0
            if not priced:
                unverified.append(row["id"])
            snapshot_rows.append({
                "id": row["id"], "model": seg["model"], "prompt": prompt, "size": size,
                "aspect_ratio": None if seg["aspect_ratio"] in (None, "-") else seg["aspect_ratio"],
                "qty": 1, "mode": "text", "references": [], "est_usd": per_image})
    snap = {"created_at": lg.now(), "estimated_usd_known": round(known, 6),
            "unverified_rows": unverified, "rows": snapshot_rows}
    fp = snap["fingerprint"] = pf.snapshot_fingerprint(snap)
    lg.write_json_atomic(out / "snapshots" / f"{fp}.json", snap)
    lg.save(ledger_path, ledger)
    return {"fingerprint": fp, "rows": snapshot_rows, "known_usd": round(known, 6), "unverified_rows": unverified,
            "images": sum(r["qty"] for r in snapshot_rows), "intent": plan["intent"], "why": plan["why"]}


def format_quick(q: dict) -> str:
    L = [f"IMAGE LAB QUICK PLAN  ({q['intent']})", q["why"]]
    for r in q["rows"]:
        price = "price unverified" if r["est_usd"] is None else f"~${r['est_usd']:.4f}"
        L.append(f"  {r['id']}  {r['model']:<36} {price}")
    cost = f"${q['known_usd']:.4f} known"
    if q["unverified_rows"]:
        cost += f"  +  {len(q['unverified_rows'])} row(s) price unverified (not in the total)"
    L.append(f"Images: {q['images']}   Estimated cost: {cost}")
    L.append("Balance: unread (the gateway will confirm)")
    L.append(f"Fingerprint: {q['fingerprint']}")
    return "\n".join(L)


def refresh(exp_dir, record_models: bool = False, after_merge=None) -> dict:
    """Merge the user's edits, then rewrite the workbook. record_models=True (after a run, or on request for an
    older experiment) fills each generated row's blank Model cell with the model that made its images; it happens
    after the merge so a workbook written before the run cannot blank it again. `after_merge(ledger)` may change
    the ledger once the user's edits are in, before it is saved and the workbook is written (add-takes uses it)."""
    exp = Path(exp_dir)
    with lg.writer_lock(exp, "refresh"):
        return _refresh(exp, record_models, after_merge)


def _refresh(exp, record_models, after_merge) -> dict:
    plan = mx.load_plan(exp / "plan.json")
    ledger = lg.load(exp / "ledger.json")
    names = ledger["experiment"]["dimensions"]
    path = exp / "experiment.xlsx"
    warnings = []
    if path.exists():
        sheet = wbk.read_workbook(path, names)                 # read-only; if this fails nothing is written
        warnings = lg.merge_workbook(ledger, sheet["rows"])
        warnings += [f"the workbook has no {d!r} column (renamed or deleted?): the ledger's values for it were kept"
                     for d in sheet.get("missing_columns", [])]
        wbk.backup_previous(path)
    if record_models:                                  # True: every generated row; a set of ids: only those rows
        only = None if record_models is True else set(record_models)
        for row in ledger["rows"]:
            if row["status"] != "Removed" and (only is None or row["id"] in only):
                lg.record_used_model(ledger, row)
    if after_merge:
        after_merge(ledger)
    lg.save(exp / "ledger.json", ledger)
    adv_models = sorted(pf.il.load_model_catalog())
    links = {}
    for a in ledger["attempts"]:
        if a["status"] == "Completed" and a.get("file"):
            links.setdefault(a["row_id"], []).append("external:" + a["file"])
    written = wbk.write_workbook(path, ledger, plan, prompts=compile_prompts(plan, ledger), models=adv_models,
                                 image_links=links)
    fell_back = Path(written).resolve() != path.resolve()
    if not fell_back:                                  # the main workbook now holds exactly these rows
        ledger["experiment"]["main_workbook_rows"] = [r["id"] for r in ledger["rows"] if r["status"] != "Removed"]
        lg.save(exp / "ledger.json", ledger)
    return {"warnings": warnings, "workbook": written, "fell_back": fell_back}


def add_takes(exp_dir, row_ids: list[str], count: int = 1) -> dict:
    """More images of the same values: `count` new ticked rows per given row, with the next Take numbers. Free:
    nothing is generated until preflight and run. The workbook is merged first so edits in it are not lost."""
    exp = Path(exp_dir)
    if not 1 <= count <= 3:
        raise mx.PlanError("count must be 1 to 3")
    if not (exp / "plan.json").exists():
        raise mx.PlanError("add-takes applies to experiments (plan.json and a workbook); a quick folder has neither: "
                           "run `quick` again with the number of images you want")
    lg.assert_unlocked(exp)
    made: list[dict] = []

    def add(ledger):
        try:
            made.extend(lg.add_takes(ledger, row_ids, count))
        except KeyError as e:
            raise mx.PlanError(str(e).strip("'\""))

    rf = refresh(exp, after_merge=add)                       # the user's workbook edits are merged first, then rows added
    return {"added": [(r["id"], r["take"]) for r in made], "workbook": rf["workbook"], "fell_back": rf["fell_back"]}


def check(plan_path=None) -> list[str]:
    """No-spend consistency checks for the M1 data files. Returns problems (empty = ok)."""
    problems = []
    catalog = pf.il.load_model_catalog()
    support = pf.load_reference_support()
    for mid in catalog:
        if mid not in support:
            problems.append(f"reference-support.json has no entry for catalog model {mid}")
    for mid, v in support.items():
        if mid not in catalog:
            problems.append(f"reference-support.json lists {mid}, which is not in model-catalog.yaml")
        if v.get("state") not in ("verified", "documented", "unknown", "unsupported"):
            problems.append(f"{mid}: reference state {v.get('state')!r} is not one of verified/documented/unknown/unsupported")
        if v.get("state") in ("verified", "documented") and not v.get("max_images"):
            problems.append(f"{mid}: a reference-capable model needs max_images")
    cat = cp.load_catalog()
    for dim, vals in cat.items():
        for val, e in vals.items():
            if not (e.get("fragment") or e.get("fragment_with_reference")):
                problems.append(f"controls-catalog.json: {dim}/{val} has no wording")
    if plan_path:
        try:
            mx.load_plan(plan_path)
        except mx.PlanError as e:
            problems.append(str(e))
    return problems


def register(sub, with_run: bool = True) -> None:
    """Add the experiment subcommands to an argparse subparsers object (used by image.py too)."""
    p = sub.add_parser("plan", help="validate a plan, recommend a covering batch, write the workbook")
    p.add_argument("--plan", required=True)
    p.add_argument("--out", required=True)
    p.add_argument("--target", type=int, default=30)
    p.add_argument("--dry-run", action="store_true",
                   help="validate and show the size of the experiment; write nothing")
    q = sub.add_parser("preflight", help="read the workbook, validate, compile, price; write the snapshot")
    q.add_argument("--dir", required=True)
    q.add_argument("--only", default=None, help="comma-separated row ids: price only these of the ticked rows")
    q.add_argument("--one-per", default=None, metavar="DIMENSION",
                   help="price one ticked row per value of this dimension (a calibration: for example --one-per direction)")
    la = sub.add_parser("llm-advice", help="Assistant fit: is your assistant (the LLM your coding agent runs on) well suited for "
                                            "Image Lab's steps? Advice only, from measured evidence; free")
    la.add_argument("--model", help="the assistant's own model name, as the agent knows it")
    la.add_argument("--can-read-images", choices=("yes", "no", "unknown"), default="unknown",
                    help="whether the host can read image files (the result review needs it)")
    la.add_argument("--offline", action="store_true", help="use the saved model snapshot instead of asking the gateway")
    la.add_argument("--json", action="store_true")
    la.add_argument("--probe-pricing", action="store_true",
                    help="the pricing probe: two tiny PAID gateway calls; prints a quote first and needs --confirm <fingerprint>")
    la.add_argument("--eval", action="store_true", help="the worker batch: Creative Direction on Luna, Gemini 3 Flash and GPT-6 Astra (3 briefs x 2 runs); quote first, needs --confirm <fingerprint>")
    la.add_argument("--limit", type=int, default=None, help="with --judge: judge only the first N sheets (a cheap trial)")
    la.add_argument("--judge-model", default=None, help="with --judge: the judge's gateway model id (default x-ai/grok-4.6)")
    la.add_argument("--judge", action="store_true", help="the blind judge on the saved Direction replies (gateway, quoted first); needs --confirm <fingerprint>")
    la.add_argument("--score", nargs="+", metavar="FILE", help="free: run the mechanical Creative Direction checks on saved replies")
    la.add_argument("--stream-check", action="store_true",
                    help="one long streamed reply from the cheap model (a few thousandths of a dollar); needs --confirm <fingerprint>")
    la.add_argument("--long", action="store_true", help="with --stream-check: a reply of about 5000 words (past the gateway's 60 second cut), at most about $0.011")
    la.add_argument("--check-model", default=None, help="with --stream-check: a tiny streamed check of this one model id (max 64 tokens)")
    la.add_argument("--pilot", action="store_true",
                    help="the cost pilot: one real Creative Direction call on each premium gateway model; prints a quote first and needs --confirm <fingerprint>")
    la.add_argument("--confirm", default=None, help="the fingerprint the probe or pilot quote printed")
    t = sub.add_parser("retry", help="preflight for Failed/Partial rows (missing samples only)")
    t.add_argument("--dir", required=True)
    t.add_argument("--include-unknown", action="store_true",
                   help="also include rows whose outcome was Unknown (only after checking the usage log)")
    k = sub.add_parser("quick", help="quick mode: model is the only dimension, the prompt is used verbatim")
    k.add_argument("--prompt", required=True)
    k.add_argument("--intent", default=None)
    k.add_argument("--count", type=int, default=4)
    k.add_argument("--models", default=None, help="comma-separated model ids to force")
    k.add_argument("--out", required=True)
    ak = sub.add_parser("acknowledge-person",
                        help="record that the user confirmed permission to use a person's photo (writes "
                             "consent_acknowledged into the plan)")
    ak.add_argument("--plan", required=True, help="the plan.json (and, with --dir, that experiment's copy too)")
    ak.add_argument("--dir", default=None)
    sh = sub.add_parser("sheet", help="build the contact-sheet page for an experiment (free)")
    import sheet as _sheet
    _sheet.add_args(sh)
    mo = sub.add_parser("montage", help="labelled image grids of finished images, one per value of a dimension (free, needs Pillow)")
    import montage as _montage
    _montage.add_args(mo)
    cc = sub.add_parser("controls", help="list the controls vocabulary (dimensions, values, wording, how far tested)")
    cc.add_argument("--dimension", default=None)
    cc.add_argument("--json", action="store_true")
    rc = sub.add_parser("recover", help="re-download images that were generated and billed but failed to download "
                                          "(free; uses the URL kept in the ledger)")
    rc.add_argument("--dir", required=True)
    r = sub.add_parser("refresh", help="merge workbook edits into the ledger and regenerate the workbook")
    r.add_argument("--dir", required=True)
    r.add_argument("--record-models", action="store_true",
                   help="fill blank Model cells of generated rows with the model that made their images")
    at = sub.add_parser("add-takes", help="add more images of the same values: new ticked rows with the next Take numbers (free)")
    at.add_argument("--dir", required=True)
    at.add_argument("--rows", required=True, help="comma-separated row ids, for example r001,r005")
    at.add_argument("--count", type=int, default=1, help="new takes per row, 1 to 3 (default 1)")
    if with_run:
        g = sub.add_parser("run", help="generate the approved snapshot (spends money)")
        add_run_args(g)


def add_run_args_extra(g) -> None:
    """Arguments only the Image Lab 2 `run` needs (image.py's v0.1 `run` already has the others)."""
    g.add_argument("--dir", help="the experiment folder")
    g.add_argument("--max-usd", type=float, default=None,
                   help="stop submitting new samples past this (default: 1.25x the known estimate)")
    g.add_argument("--concurrency", type=int, default=6)
    g.add_argument("--again", action="store_true", help="run a snapshot that already ran (a new spend)")


def add_run_args(g) -> None:
    g.add_argument("--confirm", help="the fingerprint preflight printed (your approval)")
    g.add_argument("--progress-file", default=None)
    add_run_args_extra(g)


def after_run(d: Path) -> dict:
    """The free views rebuilt after a run: the contact sheet and the regenerated workbook (experiments
    only; a quick-mode folder has neither a plan nor a workbook). A view failing never fails the run."""
    out = {"sheet": None, "workbook": None}
    if not (d / "plan.json").exists():
        return out
    try:
        import sheet as _sheet
        out["sheet"] = _sheet.build_sheet(d)
        print(f"contact sheet: {out['sheet']}")
    except Exception as e:
        print(f"(contact sheet not built: {e})", file=sys.stderr)
    try:
        led = lg.load(d / "ledger.json")
        last = max((b["no"] for b in led["batches"]), default=None)
        just_ran = {a["row_id"] for a in led["attempts"] if a.get("batch") == last}
        rf = refresh(d, record_models=just_ran)
    except Exception as e:                           # noqa: BLE001  the run itself already succeeded
        print(f"(workbook not regenerated: {type(e).__name__}: {e}. The results are in ledger.json; run "
              f"`image.py refresh --dir {d}` once it is fixed.)", file=sys.stderr)
        return out
    out["workbook"] = rf["workbook"]
    if rf["fell_back"]:
        print(f"\nexperiment.xlsx is open elsewhere; the updated workbook is {rf['workbook']}")
    return out


def cmd_run_batch(a) -> None:
    import generate as gen
    if not a.dir or not a.confirm:
        print("run needs --dir and --confirm <fingerprint> (the fingerprint preflight printed).", file=sys.stderr)
        sys.exit(2)
    try:
        res = gen.run_batch(a.dir, a.confirm, concurrency=a.concurrency, max_usd=a.max_usd,
                            progress_file=a.progress_file, again=a.again,
                            log=lambda m: print(m, file=sys.stderr, flush=True))
    except gen.RunRefused as e:
        print(f"REFUSED (nothing was spent): {e}", file=sys.stderr)
        sys.exit(2)
    print(gen.format_result(res))
    after_run(Path(a.dir))
    sys.exit(0 if not (res["failed"] or res["unknown"] or res["unfinished"]) else 1)


def handle(a) -> None:
    try:
        _handle(a)
    except lg.LedgerBusy as e:
        print(f"BUSY (nothing was changed): {e}", file=sys.stderr)
        sys.exit(2)


def _handle(a) -> None:
    if hasattr(sys.stdout, "reconfigure"):
        sys.stdout.reconfigure(encoding="utf-8", errors="replace")      # Windows consoles default to cp1252
    if a.cmd == "check":
        probs = check(getattr(a, "plan", None))
        print("OK: M1 data files are consistent" if not probs else "\n".join(probs))
        sys.exit(1 if probs else 0)
    elif a.cmd == "run":
        cmd_run_batch(a)
    elif a.cmd == "montage":
        import montage as _montage
        sys.exit(_montage.run(a))
    elif a.cmd == "sheet":
        import sheet as _sheet
        sys.exit(_sheet.run(a))
    elif a.cmd == "controls":
        cat = cp.load_catalog()
        if a.dimension:
            cat = {a.dimension: cat.get(a.dimension, {})}
        if a.json:
            print(json.dumps(cat, indent=2, ensure_ascii=False))
        else:
            for dim, vals in cat.items():
                print(f"{dim}")
                for val, e in vals.items():
                    flag = "tested" if str(e.get("validation", "")).startswith("m0") else "UNTESTED"
                    rel = "  [unreliable with a reference]" if (e.get("reliability") or {}).get("reference") else ""
                    per = "  [separate reference wording]" if e.get("fragment_with_reference") else ""
                    scope = {"product": "  [written for a product on a surface: not for people or whole rooms]",
                             "portrait": "  [written for portraits]"}.get(e.get("scope"), "")
                    print(f"  {val:<22} {flag}{rel}{per}{scope}")
                    print(f"      {e.get('fragment', '')}")
    elif a.cmd == "llm-advice":
        import llm_fit
        try:
            print(llm_fit.run(a))
        except PermissionError as e:
            print(f"llm-advice: {e}", file=sys.stderr)
            sys.exit(2)
    elif a.cmd == "add-takes":
        try:
            res = add_takes(a.dir, [x.strip() for x in a.rows.split(",") if x.strip()], a.count)
        except (mx.PlanError, lg.LedgerBusy) as e:
            print(f"add-takes: {e}", file=sys.stderr)
            sys.exit(2)
        print("added " + ", ".join(f"{rid} (Take {t})" for rid, t in res["added"]))
        print(f"workbook: {res['workbook']}")
        if res["fell_back"]:
            print("NOTE: experiment.xlsx is open elsewhere, so the updated workbook was written as a side copy "
                  f"({res['workbook']}). Close Excel and run `refresh` so experiment.xlsx shows the new rows.")
        print("These rows are ticked. Run `preflight` to price them; nothing is generated until you approve.")
    elif a.cmd == "recover":
        import generate as gen
        res = gen.recover_downloads(a.dir, log=lambda m: print(m))
        if (Path(a.dir) / "plan.json").exists() and res["recovered"]:
            refresh(a.dir, record_models={x.split("-")[0] for x in res["recovered"]})
        print(f"recovered {len(res['recovered'])}, still failing {len(res['failed'])}")
        sys.exit(0 if not res["failed"] else 1)
    elif a.cmd == "acknowledge-person":
        targets = [Path(a.plan)] + ([Path(a.dir) / "plan.json"] if a.dir and (Path(a.dir) / "plan.json").exists() else [])
        for t in targets:
            plan = json.loads(t.read_text(encoding="utf-8"))
            if not mx.uses_person(plan):
                print(f"{t}: no reference shows a person; nothing to acknowledge.", file=sys.stderr)
                sys.exit(2)
            plan["consent_acknowledged"] = lg.now()
            lg.write_json_atomic(t, plan)
            print(f"{t}: consent_acknowledged = {plan['consent_acknowledged']}")
    elif a.cmd == "plan":
        try:
            print(format_plan_summary(build_experiment(a.plan, a.out, a.target, dry_run=a.dry_run)))
        except mx.PlanError as e:
            print(f"error: {e}", file=sys.stderr)
            sys.exit(2)
    elif a.cmd == "quick":
        models = [m.strip() for m in a.models.split(",") if m.strip()] if a.models else None
        try:
            print(format_quick(build_quick(a.prompt, a.intent, a.count, a.out, models)))
        except mx.PlanError as e:
            print(f"error: {e}", file=sys.stderr)
            sys.exit(2)
    elif a.cmd in ("preflight", "retry"):
        only = [x.strip() for x in (getattr(a, "only", None) or "").split(",") if x.strip()]
        rep = pf.run_preflight(a.dir, retry=(a.cmd == "retry"), include_unknown=getattr(a, "include_unknown", False),
                               only=only, one_per=getattr(a, "one_per", None))
        print(rep["text"])
        sys.exit(0 if rep["ready"] else 2)
    else:
        if not (Path(a.dir) / "plan.json").exists():
            print("refresh applies to experiments (plan.json + workbook); a quick-mode folder has neither.", file=sys.stderr)
            sys.exit(2)
        res = refresh(a.dir, record_models=a.record_models)
        for w in res["warnings"]:
            print("  " + w)
        if res["fell_back"]:
            print("experiment.xlsx could not be written (is it open in Excel?). Wrote a side copy instead:")
        print(f"workbook: {res['workbook']}")


def main(argv=None):
    ap = argparse.ArgumentParser(prog="experiment.py", description=__doc__,
                                 formatter_class=argparse.RawDescriptionHelpFormatter)
    sub = ap.add_subparsers(dest="cmd", required=True)
    register(sub)
    c = sub.add_parser("check")
    c.add_argument("--plan")
    handle(ap.parse_args(argv))


if __name__ == "__main__":
    main()
