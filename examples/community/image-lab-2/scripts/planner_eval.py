#!/usr/bin/env python3
"""Planner evaluation checker (design-v2.md section 14, "Planner evaluation set").

The Planner is an agent following skills/plan.md, so it cannot be unit-tested. What can be
checked mechanically is whether the plan it produced obeys the rules in that skill. This script
holds the evaluation briefs' expectations and checks a produced plan.json against them:

    python scripts/planner_eval.py --list
    python scripts/planner_eval.py --brief product-photo --plan path/to/plan.json
    python scripts/planner_eval.py --all path/to/dir      # dir holds <brief id>.json per brief
                                                           # (the route-to-quick brief holds {"route": "quick"})

It never calls the gateway or spends anything. A pass means the plan is structurally sound; it
does not say the dimensions are the *best* ones, so a person still reviews the briefs' plans
(that review is the evaluation; this keeps it honest and repeatable).
"""
from __future__ import annotations

import argparse
import json
import re
import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))

import compile as cp  # noqa: E402
import matrix as mx  # noqa: E402

BRIEFS_FILE = Path(__file__).resolve().parent.parent / "tests" / "planner-eval" / "briefs.json"
RATIO = re.compile(r"^\d+(\.\d+)?:\d+(\.\d+)?$")
INTENTS = ("launch / announcement image", "profile / avatar", "social post", "blog hero / article cover",
           "poster / flyer", "infographic / diagram", "illustration / concept art",
           "3D render / isometric illustration", "product / e-commerce shot", "generic")
MAX_DIMS, MAX_VALUES, MIN_VALUES, MAX_VALID = 4, 4, 2, 150


ROUTES_FILE = Path(__file__).resolve().parent.parent / "tests" / "planner-eval" / "route-cases.json"
ROUTES = ("quick", "variation", "direction", "ask")


def load_route_cases(path=None) -> dict:
    data = json.loads(Path(path or ROUTES_FILE).read_text(encoding="utf-8"))
    return {c["id"]: c for c in data["cases"]}


def routing_text() -> str:
    """The shipped routing guidance exactly as an agent reads it: SKILL.md step 0 and the quickstart's section 1."""
    root = Path(__file__).resolve().parent.parent
    skill = (root / "SKILL.md").read_text(encoding="utf-8")
    quick = (root / "references" / "quickstart.md").read_text(encoding="utf-8")
    a = skill[skill.index("**0. Route.**"):skill.index("**1. Brief.**")]
    b = quick[quick.index("## 1. Pick the route"):quick.index("## 2. The loop")]
    return a.strip() + "\n\n" + b.strip()


def check_route(case_id: str, answer: str, cases: dict | None = None) -> str | None:
    """None when `answer` is the expected route for the case, else why not."""
    cases = cases if cases is not None else load_route_cases()
    got = re.sub(r"[^a-z]", "", str(answer).lower())
    if got not in ROUTES:
        return f"{case_id}: {answer!r} is not one of {', '.join(ROUTES)}"
    want = cases[case_id]["route"]
    return None if got == want else f"{case_id}: routed to {got}; expected {want} ({cases[case_id]['why']})"


def load_briefs(path=None) -> dict:
    data = json.loads(Path(path or BRIEFS_FILE).read_text(encoding="utf-8"))
    return {b["id"]: b for b in data["briefs"]}


def _name(v) -> str:
    return v["value"] if isinstance(v, dict) else v


def check_plan(plan: dict, expect: dict, catalog: dict | None = None) -> list[str]:
    """Failures (empty list = pass) of `plan` against one brief's expectations."""
    cat = catalog if catalog is not None else cp.load_catalog()
    fails: list[str] = []
    route = expect.get("route", "plan")
    if route == "quick":
        if isinstance(plan, dict) and plan.get("route") == "quick":
            return []
        return ["this brief should go to quick mode (no plan.json): a single prompt, no reference, no "
                "controlled variation, at most 8 images"]
    if isinstance(plan, dict) and plan.get("route") == "quick":
        return ["this brief needs an experiment plan, not quick mode"]

    errs = mx.validate_plan(plan)
    if errs:
        return [f"plan.json is invalid: {e}" for e in errs]

    if plan["intent"] not in INTENTS:
        fails.append(f"intent {plan['intent']!r} is not one of the known intents")
    if expect.get("intents") and plan["intent"] not in expect["intents"]:
        fails.append(f"intent {plan['intent']!r}; expected one of {expect['intents']}")

    dims = plan["dimensions"]
    n_var = len([d for d, v in dims.items() if not (d == "aspect" and len(v) == 1)])   # a fixed shape is not a variable
    lo, hi = expect.get("min_dims", 2), expect.get("max_dims", MAX_DIMS)
    if not lo <= n_var <= hi:
        fails.append(f"{n_var} dimensions; this brief expects {lo} to {hi} (the skill allows at most {MAX_DIMS})")
    for d, vals in dims.items():
        if len(vals) > MAX_VALUES:
            fails.append(f"dimension {d!r} has {len(vals)} values; at most {MAX_VALUES}")
        if len(vals) < MIN_VALUES and d != "aspect":
            fails.append(f"dimension {d!r} has {len(vals)} value; a variable dimension needs at least {MIN_VALUES}")
    want_any = expect.get("require_dimensions_any")
    if want_any and not any(d in dims for d in want_any):
        fails.append(f"none of the expected dimensions {want_any} is present")
    for d in expect.get("forbid_dimensions", []):
        if d in dims:
            fails.append(f"dimension {d!r} should not be offered for this brief")

    if not (plan.get("fixed") or {}) and expect.get("must_fix", True):
        fails.append("`fixed` is empty: state what must not change (for a product or person reference, its identity)")

    # references and consent
    refs = plan.get("references", [])
    for key, text in (plan.get("fixed") or {}).items():
        for r in refs:
            if re.search(rf"\b{re.escape(str(r.get('id')))}\b", str(text)):
                fails.append(f'fixed "{key}" mentions {r.get("id")}: Fixed text goes into the prompt as written and the '
                             f'model only sees "the reference image"')
    want_refs = expect.get("references", [])
    if len(refs) != len(want_refs):
        fails.append(f"{len(refs)} reference(s); expected {len(want_refs)}")
    for got, want in zip(refs, want_refs):
        if got.get("role") != want["role"]:
            fails.append(f"reference {got.get('id')!r} has role {got.get('role')!r}; expected {want['role']!r}")
        if bool(got.get("contains_person")) != bool(want.get("contains_person")):
            fails.append(f"reference {got.get('id')!r}: contains_person should be {bool(want.get('contains_person'))}")
    if expect.get("consent_must_be_null") and plan.get("consent_acknowledged"):
        fails.append("consent_acknowledged is set; the planner must never acknowledge a person notice itself "
                     "(only the user's yes, recorded by `acknowledge-person`)")

    # aspect
    if "aspect" in dims:
        bad = [v for v in dims["aspect"] if not RATIO.match(_name(v))]
        if bad:
            fails.append(f"aspect values must be ratios like 16:9 (got {bad})")
        if any(isinstance(v, dict) and v.get("fragment") for v in dims["aspect"]):
            fails.append("aspect is a size, not wording: do not give it a fragment")
    elif expect.get("require_aspect"):
        fails.append("this brief needs a particular shape: offer an `aspect` dimension")

    # wording: every non-aspect value must have wording from the plan or the catalog
    mode = "reference" if refs else "text"
    for d, vals in dims.items():
        if d == "aspect":
            continue
        for v in vals:
            text, src = cp.fragment(plan, d, _name(v), mode, cat)
            if src == "generic":
                fails.append(f'{d} "{_name(v)}" has no wording (not in the catalog and no fragment in the plan)')
            if refs and d == "camera":
                entry = v if isinstance(v, dict) else cat.get(d, {}).get(_name(v), {})
                rel = (entry.get("reliability") or {}).get("reference")
                if isinstance(v, dict) and v.get("fragment") and not v.get("fragment_with_reference") and not rel:
                    fails.append(f'camera "{_name(v)}" has its own wording but no fragment_with_reference: with a '
                                 f"photo the model copies the reference pose, so camera wording needs a milder "
                                 f"reference variant (or mark it unreliable)")
                if cp.reliability(plan, d, _name(v), "reference", cat) == "unreliable" and expect.get("forbid_unreliable"):
                    fails.append(f'camera "{_name(v)}" is unreliable with a reference; do not offer it')

    if expect.get("require_visual_checks") and not plan.get("visual_checks"):
        fails.append("`visual_checks` is empty: list the requirements models follow only loosely (no text, headline "
                     "space, identity) so they are checked by eye after the first batch")

    # size of the experiment
    try:
        names = mx.dimensions(plan)
        valid = mx.enumerate_valid(names, mx.excludes(plan))
        if not valid:
            fails.append("no valid combination: the exclude rules remove everything")
        elif len(valid) > MAX_VALID:
            fails.append(f"{len(valid)} valid combinations; the plan is too wide (at most about {MAX_VALID})")
    except mx.PlanError as e:
        fails.append(str(e))
    return fails


def run_all(plan_dir: Path, briefs: dict) -> dict:
    out = {}
    for bid, b in briefs.items():
        p = Path(plan_dir) / f"{bid}.json"
        if not p.exists():
            out[bid] = [f"no plan file {p.name}"]
            continue
        out[bid] = check_plan(json.loads(p.read_text(encoding="utf-8")), b["expect"])
    return out


def main(argv=None) -> int:
    ap = argparse.ArgumentParser(prog="planner_eval.py", description=__doc__,
                                 formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("--list", action="store_true")
    ap.add_argument("--brief")
    ap.add_argument("--plan")
    ap.add_argument("--all")
    ap.add_argument("--briefs", default=None, help="a different briefs.json")
    ap.add_argument("--routes", default=None, metavar="FILE", help="score a {case id: route} JSON file against tests/planner-eval/route-cases.json")
    ap.add_argument("--routing-text", action="store_true", help="print the shipped routing guidance an agent reads")
    a = ap.parse_args(argv)
    if hasattr(sys.stdout, "reconfigure"):
        sys.stdout.reconfigure(encoding="utf-8", errors="replace")
    if a.routing_text:
        print(routing_text())
        return 0
    if a.routes:
        cases = load_route_cases()
        answers = json.loads(Path(a.routes).read_text(encoding="utf-8"))
        bad = 0
        for cid in cases:
            f = check_route(cid, answers.get(cid, ""), cases)
            print(f"{'PASS' if f is None else 'FAIL'}  {cid}" + (f"  - {f}" if f else ""))
            bad += f is not None
        return 1 if bad else 0
    briefs = load_briefs(a.briefs)
    if a.list:
        for bid, b in briefs.items():
            print(f"{bid}\n  {b['brief']}\n  tests: {b['tests']}")
        return 0
    if a.all:
        res = run_all(Path(a.all), briefs)
    elif a.brief and a.plan:
        res = {a.brief: check_plan(json.loads(Path(a.plan).read_text(encoding="utf-8")), briefs[a.brief]["expect"])}
    else:
        ap.error("use --list, --brief ID --plan FILE, or --all DIR")
    bad = 0
    for bid, fails in res.items():
        print(f"{'PASS' if not fails else 'FAIL'}  {bid}")
        for f in fails:
            print(f"      - {f}")
        bad += bool(fails)
    return 1 if bad else 0


if __name__ == "__main__":
    sys.exit(main())
