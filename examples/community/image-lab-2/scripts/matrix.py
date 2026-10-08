"""Experiment matrix for Image Lab 2 (design-v2.md sections 6.2 and 7).

Pure functions plus `load_plan`: plan -> valid combinations -> pairwise-covering
first batch. allpairspy builds the covering rows; THIS module verifies the coverage
and repairs it, because allpairspy with an exclusion filter can silently leave valid
pairs uncovered (see tests/test_matrix.py, `test_dense_exclusions_are_repaired`).

A "row" here is a tuple of values, one per dimension, in `names` order.
"""
from __future__ import annotations

import collections
import itertools
import json
import re
import datetime
from pathlib import Path

SCHEMA_VERSION = 1
MAX_COMBINATIONS = 1_000_000          # exhaustive enumeration limit (design 7.1)
FILL_CANDIDATE_CAP = 20_000           # farthest-point fill looks at most this many candidates
ROLES = ("product", "person", "style", "general")
# workbook column names and the model choice: a dimension called any of these would collide
RESERVED_DIMENSIONS = {"id", "selected", "model", "qty", "take", "reference", "prompt", "status", "images",
                       "image", "file", "cost", "notes"}


class PlanError(ValueError):
    pass


# --------------------------------------------------------------------------- #
# plan
# --------------------------------------------------------------------------- #
def load_plan(path) -> dict:
    try:
        plan = json.loads(Path(path).read_text(encoding="utf-8"))
    except FileNotFoundError:
        raise PlanError(f"plan file not found: {path}")
    except (OSError, ValueError) as e:
        raise PlanError(f"cannot read {path} as JSON: {e}")
    errors = validate_plan(plan)
    if errors:
        raise PlanError("plan.json is invalid:\n  - " + "\n  - ".join(errors))
    return plan


def value_name(v) -> str:
    return v["value"] if isinstance(v, dict) else str(v)


def dimensions(plan: dict, extra: dict | None = None) -> dict[str, list[str]]:
    """{dimension: [value names]} in plan order; `extra` appends dimensions (for example
    the `model` dimension under model_strategy `spread`)."""
    dims = {d: [value_name(v) for v in vals] for d, vals in plan["dimensions"].items()}
    if extra:
        dims.update(extra)
    return dims


TRAIT_KEYS = ("ground", "medium", "layout", "type", "density", "palette")
MIN_TRAIT_DIFFERENCES = 3        # any two directions must differ in at least this many of the six traits
MAX_SHARED_PALETTE = 2           # at most this many directions may share one palette


def _norm_trait(text) -> str:
    return " ".join(str(text).lower().replace("-", " ").split())


_TRAIT_STOP = frozenset("a an the and or of with on in at to for by from as is it its that this".split())


def _trait_words(text) -> set:
    """Meaningful words (any script). Size and number words such as large, small, one and two are kept: they are often
    the whole difference between two looks."""
    return {w for w in re.findall(r"\w+", str(text).lower().replace("-", " ")) if w not in _TRAIT_STOP}


def traits_alike(a, b) -> bool:
    """Two trait descriptions count as the same look when more than half of their meaningful words overlap, so rewording
    "off-white paper" as "warm off-white paper stock" does not make a duplicate look different. Descriptions with no
    meaningful words are alike only when they are the same text. This is a heuristic: it cannot tell "left" from "right"
    in an otherwise identical phrase, which is why the generated images, not this check, are the real test."""
    wa, wb = _trait_words(a), _trait_words(b)
    if not (wa | wb):
        return _norm_trait(a) == _norm_trait(b)
    return len(wa & wb) / len(wa | wb) > 0.5


def _traits_errors(dim: str, vals) -> list[str]:
    """Directions carry `traits` (ground, medium, layout, type, density, palette): what the picture looks like at
    thumbnail size. When every value of a dimension has them, any two must differ in at least three, and no palette
    may be shared by more than two. This is the mechanical guard against directions that sound different but render alike."""
    if not isinstance(vals, list) or len(vals) < 2:
        return []
    objs = [v for v in vals if isinstance(v, dict) and "traits" in v]
    if not objs:
        return []
    errs: list[str] = []
    if len(objs) != len(vals):
        errs.append(f"dimension {dim!r}: either every value has `traits` or none does")
        return errs
    for v in objs:
        t = v["traits"]
        if not isinstance(t, dict) or set(t) != set(TRAIT_KEYS) or any(not isinstance(x, str) or not x.strip() for x in t.values()):
            errs.append(f"dimension {dim!r} value {v.get('value')!r}: traits must be an object with exactly these non-empty text keys: {', '.join(TRAIT_KEYS)}")
    if errs:
        return errs
    for a in range(len(objs)):
        for b in range(a + 1, len(objs)):
            diff = [k for k in TRAIT_KEYS if not traits_alike(objs[a]["traits"][k], objs[b]["traits"][k])]
            if len(diff) < MIN_TRAIT_DIFFERENCES:
                errs.append(f"dimension {dim!r}: {objs[a].get('value')!r} and {objs[b].get('value')!r} differ in only {len(diff)} of the six traits "
                            f"({', '.join(diff) or 'none'}); they need at least {MIN_TRAIT_DIFFERENCES}, or they will look alike")
    # palette: from four directions up, ignore the words nearly all of them share (a brief's fixed foundation such as
    # "black and off-white"): what tells palettes apart is the rest (the accent). With fewer directions nothing is ignored.
    counts = collections.Counter(w for v in objs for w in _trait_words(v["traits"]["palette"]))
    common = {w for w, c in counts.items() if c > len(objs) * 2 / 3} if len(objs) >= 4 else set()

    def pal(v):
        return _trait_words(v["traits"]["palette"]) - common

    def same_pal(v, w):
        a, b = pal(v), pal(w)
        if not (a | b):
            return True                      # nothing left to tell them apart after dropping the shared foundation
        return len(a & b) / len(a | b) > 0.5

    seen_groups = set()
    for v in objs:
        group = frozenset(str(w.get("value")) for w in objs if same_pal(v, w))
        if len(group) > MAX_SHARED_PALETTE and group not in seen_groups:
            seen_groups.add(group)
            errs.append(f"dimension {dim!r}: {len(group)} directions share the palette {v['traits']['palette']!r} "
                        f"({', '.join(sorted(group))}); at most {MAX_SHARED_PALETTE} may")
    return errs


def _safe_name(v):
    """A value's name, or None for a malformed value (validate_plan reports those separately)."""
    if isinstance(v, str):
        return v
    if isinstance(v, dict) and isinstance(v.get("value"), str):
        return v["value"]
    return None


def validate_plan(plan: dict) -> list[str]:
    errs: list[str] = []
    if not isinstance(plan, dict):
        return ["plan must be a JSON object"]
    if plan.get("schema_version") != SCHEMA_VERSION:
        errs.append(f"schema_version must be {SCHEMA_VERSION}")
    for key in ("brief", "intent"):
        if not isinstance(plan.get(key), str) or not plan[key].strip():
            errs.append(f"{key} is required (non-empty text)")
    dims = plan.get("dimensions")
    if not isinstance(dims, dict) or not dims:
        errs.append("dimensions must be a non-empty object")
        dims = {}
    for d, vals in dims.items():
        if not isinstance(vals, list) or not vals:
            errs.append(f"dimension {d!r} needs a non-empty list of values")
            continue
        names = []
        for v in vals:
            if isinstance(v, dict):
                if not isinstance(v.get("value"), str) or not v["value"].strip():
                    errs.append(f"dimension {d!r}: an object value needs a text `value`")
                    continue
            elif not isinstance(v, str) or not v.strip():
                errs.append(f"dimension {d!r}: values must be text or {{value, fragment}} objects")
                continue
            names.append(value_name(v))
        if len(set(names)) != len(names):
            errs.append(f"dimension {d!r} has duplicate values")
        for v in vals:
            if not isinstance(v, dict):
                continue
            rl = v.get("relaxes")
            if rl is not None and (not isinstance(rl, list) or not rl or any(not isinstance(x, str) or not x.strip() for x in rl)):
                errs.append(f"dimension {d!r} value {v.get('value')!r}: relaxes must be a non-empty list of the brief's rules it deliberately leaves")
            if v.get("wildcard") is not None and not isinstance(v["wildcard"], bool):
                errs.append(f"dimension {d!r} value {v.get('value')!r}: wildcard must be true or false")
            if rl and v.get("wildcard") is not True:
                errs.append(f"dimension {d!r} value {v.get('value')!r}: a value that relaxes a rule must also be marked wildcard: true")
    for d, vals in dims.items():
        errs.extend(_traits_errors(d, vals))
        for v in (vals if isinstance(vals, list) else []):
            scope = v.get("only_in") if isinstance(v, dict) else None
            if scope is None:
                continue
            label = f"dimension {d!r} value {v.get('value')!r}: only_in"
            if not isinstance(scope, dict) or not scope:
                errs.append(f"{label} must be {{dimension: [values]}}")
                continue
            for other, keep in scope.items():
                names_o = [_safe_name(x) for x in dims[other]] if other in dims and isinstance(dims[other], list) else None
                if other == d or names_o is None:
                    errs.append(f"{label} names {other!r}, which is not another dimension of this plan")
                elif not isinstance(keep, list) or not keep:
                    errs.append(f"{label}.{other} must be a non-empty list of values of {other!r}")
                else:
                    errs.extend(f"{label}.{other} names {k!r}, which is not a value of {other!r}" for k in keep if k not in names_o)
    flagged = [(d, v["value"]) for d, vals in dims.items() if isinstance(vals, list)
               for v in vals if isinstance(v, dict) and isinstance(v.get("value"), str) and (v.get("relaxes") or v.get("wildcard") is True)]
    if len(flagged) > 1:
        errs.append("at most one value (the wildcard) may relax a rule of the brief; flagged: "
                    + ", ".join(f"{d}={n}" for d, n in flagged))
    cons = plan.get("constraints", [])
    if not isinstance(cons, list):
        errs.append("constraints must be a list")
        cons = []
    for d in dims:
        if str(d).strip().lower() in RESERVED_DIMENSIONS:
            errs.append(f"dimension name {d!r} is reserved (it is a workbook column or the model choice); "
                        f"rename it, for example 'model look' or 'outfit'")
    for key in ("fixed", "prompt"):
        v = plan.get(key)
        if v is not None and (not isinstance(v, dict) or any(not isinstance(x, str) for x in v.values())):
            errs.append(f"{key} must be an object whose values are text")
    for c in cons:
        ex = c.get("exclude") if isinstance(c, dict) else None
        if not isinstance(ex, dict) or not ex:
            errs.append("each constraint must be {\"exclude\": {dimension: value, ...}}")
            continue
        for d, val in ex.items():
            if d not in dims:
                errs.append(f"constraint refers to unknown dimension {d!r}")
            elif val not in [_safe_name(v) for v in (dims[d] if isinstance(dims[d], list) else [])]:
                errs.append(f"constraint refers to unknown value {val!r} of {d!r}")
    q = plan.get("quality")
    if q is not None and (not isinstance(q, str) or not q.strip()):
        errs.append("quality must be a non-empty text such as medium or high (see the model's quality values)")
    by = plan.get("batch_by")
    if by is not None:
        names = [d for d in (plan.get("dimensions") or {}) if isinstance(plan.get("dimensions"), dict)]
        if (not isinstance(by, str) or by not in names or not isinstance(plan["dimensions"][by], list)
                or len(plan["dimensions"][by]) < 2):
            errs.append("batch_by must be the name of a dimension with at least two values")
    tk = plan.get("takes", 1)
    if isinstance(tk, bool) or not isinstance(tk, int) or not 1 <= tk <= 3:
        errs.append("takes must be a whole number from 1 to 3 (images per ticked row of the first batch; default 1)")
    vc = plan.get("visual_checks", [])
    if not isinstance(vc, list) or len(vc) > 6 or any(not isinstance(x, str) or not x.strip() for x in vc):
        errs.append("visual_checks must be a list of at most 6 non-empty sentences")
    strat = plan.get("model_strategy", "single")
    if not (strat in ("single", "spread") or (isinstance(strat, str) and strat.startswith("fixed:") and len(strat) > 6)):
        errs.append("model_strategy must be single, spread or fixed:<model id>")
    seen = set()
    refs = plan.get("references", [])
    if not isinstance(refs, list):
        errs.append("references must be a list")
        refs = []
    for ref in refs:
        if not isinstance(ref, dict):
            errs.append("each reference must be an object with id, file and role")
            continue
        rid = ref.get("id")
        if not isinstance(rid, str) or not rid.strip():
            errs.append("each reference needs a text id (for example ref1)")
        if rid in seen:
            errs.append(f"duplicate reference id {rid!r}")
        seen.add(rid)
        if ref.get("role") not in ROLES:
            errs.append(f"reference {rid!r}: role must be one of {', '.join(ROLES)}")
        f = ref.get("file")
        if not isinstance(f, str) or not f.strip():
            errs.append(f"reference {rid!r}: file is required")
        elif Path(f).is_absolute() or ".." in Path(f).parts or re.match(r"^[A-Za-z]:", f):
            errs.append(f"reference {rid!r}: file must be a relative path inside the plan's folder (got {f!r})")
    return errs


PERSON_NOTICE = ("This uses a photo of a person. Confirm you have the right to use it: it is your own photo, or you have the permission of the perso"
                 "n pictured. The photo is sent to CogFoundry and the image model's provider for processing, and the results stay local (they are not published).")


def uses_person(plan: dict) -> bool:
    return any(r.get("contains_person") or r.get("role") == "person" for r in plan.get("references", []))


def consent_ok(plan: dict) -> bool:
    """consent_acknowledged must be the ISO timestamp `acknowledge-person` writes, not any text."""
    c = plan.get("consent_acknowledged")
    if not isinstance(c, str):
        return False
    try:
        datetime.datetime.fromisoformat(c.strip())
    except ValueError:
        return False
    return True


def consent_problem(plan: dict) -> str | None:
    """None when the plan needs no consent or has it; otherwise the notice to show the user."""
    return PERSON_NOTICE if uses_person(plan) and not consent_ok(plan) else None


def excludes(plan: dict) -> list[dict]:
    """The plan's `constraints`, plus the exclusions its values' `only_in` imply: a value with
    `only_in: {"direction": ["2 Process"]}` is never combined with any other value of `direction`."""
    out = [c["exclude"] for c in plan.get("constraints", [])]
    dims = plan.get("dimensions") or {}
    for d, vals in dims.items():
        for v in vals:
            scope = v.get("only_in") if isinstance(v, dict) else None
            if not isinstance(scope, dict):
                continue
            for other, keep in scope.items():
                if other in dims and other != d and isinstance(keep, list):
                    out.extend({d: _safe_name(v), other: _safe_name(w)} for w in dims[other] if _safe_name(w) not in keep)
    return out


# --------------------------------------------------------------------------- #
# enumeration
# --------------------------------------------------------------------------- #
def violates(assignment: dict, excl: list[dict]) -> bool:
    """True when `assignment` (dimension -> value; possibly partial) contains every pair of
    some exclude rule."""
    return any(all(assignment.get(d) == v for d, v in rule.items()) for rule in excl)


def enumerate_valid(dims: dict[str, list[str]], excl: list[dict]) -> list[tuple]:
    names = list(dims)
    total = 1
    for v in dims.values():
        total *= len(v)
    if total > MAX_COMBINATIONS:
        raise PlanError(f"{total:,} combinations is over the {MAX_COMBINATIONS:,} limit; "
                        f"reduce dimensions or values")
    out = []
    for combo in itertools.product(*dims.values()):
        if not violates(dict(zip(names, combo)), excl):
            out.append(combo)
    return out


def pairs_of(row: tuple) -> set:
    return {(i, row[i], j, row[j]) for i, j in itertools.combinations(range(len(row)), 2)}


def all_pairs(dims: dict[str, list[str]]) -> int:
    sizes = [len(v) for v in dims.values()]
    return sum(a * b for a, b in itertools.combinations(sizes, 2))


def coverage(rows: list[tuple], valid: list[tuple]) -> dict:
    """Pairwise coverage of `rows` against the pairs that SOME valid combination contains."""
    coverable = set()
    for r in valid:
        coverable |= pairs_of(r)
    covered = set()
    for r in rows:
        covered |= pairs_of(r)
    covered &= coverable
    n = len(coverable)
    return {"coverable": n, "covered": len(covered),
            "fraction": 1.0 if n == 0 else len(covered) / n,
            "uncovered": sorted(coverable - covered)}


# --------------------------------------------------------------------------- #
# covering design
# --------------------------------------------------------------------------- #
def _allpairs_rows(dims: dict[str, list[str]], excl: list[dict]) -> list[tuple]:
    try:
        from allpairspy import AllPairs      # imported late so the rest works without it
    except ImportError:
        raise PlanError("the allpairspy package is required to build a covering batch: "
                        "pip install -r requirements.txt")
    names = list(dims)

    def ok(partial):                          # called with a prefix of the dimensions
        return not violates(dict(zip(names, partial)), excl)

    rows = [tuple(r) for r in AllPairs(list(dims.values()), filter_func=ok)]
    seen, out = set(), []
    for r in rows:
        if r not in seen:
            seen.add(r)
            out.append(r)
    return out


def _repair(rows: list[tuple], valid: list[tuple]) -> list[tuple]:
    """Add, for every coverable pair that `rows` misses, a valid combination containing it."""
    cov = coverage(rows, valid)
    missing = set(cov["uncovered"])
    if not missing:
        return rows
    first_for: dict = {}
    for r in valid:                           # one scan: first valid combination per missing pair
        for p in pairs_of(r) & missing:
            first_for.setdefault(p, r)
        if len(first_for) == len(missing):
            break
    rows = list(rows)
    have = set(rows)
    covered_now = set()
    for r in rows:
        covered_now |= pairs_of(r)
    for p in sorted(missing):
        if p in covered_now:
            continue
        r = first_for[p]
        if r not in have:
            rows.append(r)
            have.add(r)
            covered_now |= pairs_of(r)
    return rows


def _fill(rows: list[tuple], valid: list[tuple], target: int) -> list[tuple]:
    """Grow `rows` to `target` with spare valid combinations. The covering rows already show every
    pair; the spare rows are chosen so that every value of every dimension is used about equally
    often (lowest total value frequency first), then farthest (Hamming distance) from the rows
    already chosen; ties are broken by position in `valid`, so the result is deterministic.
    Choosing by distance alone favoured whichever block of `valid` came first (a plan with 3 colors
    ended with one color in over half the rows)."""
    rows = list(rows)
    have = set(rows)
    pool = [r for r in valid if r not in have]
    if len(pool) > FILL_CANDIDATE_CAP:
        step = len(pool) / FILL_CANDIDATE_CAP
        pool = [pool[int(i * step)] for i in range(FILL_CANDIDATE_CAP)]
    width = len(pool[0]) if pool else 0
    freq = [dict() for _ in range(width)]
    for r in rows:
        for k, v in enumerate(r):
            freq[k][v] = freq[k].get(v, 0) + 1

    def dist(a, b):
        return sum(x != y for x, y in zip(a, b))

    mind = [min((dist(c, r) for r in rows), default=len(c)) for c in pool]
    while len(rows) < target and pool:
        best = min(range(len(pool)),
                   key=lambda i: (sum(freq[k].get(v, 0) for k, v in enumerate(pool[i])), -mind[i], i))
        pick = pool.pop(best)
        mind.pop(best)
        rows.append(pick)
        for k, v in enumerate(pick):
            freq[k][v] = freq[k].get(v, 0) + 1
        mind = [min(m, dist(c, pick)) for m, c in zip(mind, pool)]
    return rows


def cover(dims: dict[str, list[str]], excl: list[dict], target: int = 30) -> dict:
    """Recommended first batch.

    Returns {names, rows, covering_min, valid_count, excluded_count, pairs, coverage}.
    `rows` is at least the covering minimum, at most the number of valid combinations,
    and grows toward `target` with spare rows.
    """
    names = list(dims)
    valid = enumerate_valid(dims, excl)
    if not valid:
        raise PlanError("every combination is excluded by the constraints")
    product = 1
    for v in dims.values():
        product *= len(v)

    if len(names) == 1:
        base = list(valid)
    else:
        base = _repair(_allpairs_rows(dims, excl), valid)
    pos = {r: i for i, r in enumerate(valid)}
    rows = sorted(set(base), key=pos.get)
    covering_min = len(rows)
    want = min(max(target, covering_min), len(valid))
    if len(rows) < want:
        rows = _fill(rows, valid, want)
    rows = sorted(rows, key=pos.get)
    cov = coverage(rows, valid)
    return {
        "names": names, "rows": rows, "covering_min": covering_min,
        "valid_count": len(valid), "excluded_count": product - len(valid),
        "pairs": {"all": all_pairs(dims), "coverable": cov["coverable"],
                  "impossible": all_pairs(dims) - cov["coverable"]},
        "coverage": {"covered": cov["covered"], "fraction": cov["fraction"]},
    }


def _greedy_pairs(rows: list[tuple], limit: int) -> list[tuple]:
    """The `limit` rows that together cover the most pairs (each pick adds the most new pairs; ties keep the
    earlier row), used when a stratum's own covering design is larger than its share of the batch."""
    chosen, seen, pool = [], set(), list(rows)
    while pool and len(chosen) < limit:
        best = max(range(len(pool)), key=lambda i: (len(pairs_of(pool[i]) - seen), -i))
        pick = pool.pop(best)
        chosen.append(pick)
        seen |= pairs_of(pick)
    return chosen


def cover_by(dims: dict[str, list[str]], excl: list[dict], by: str, target: int = 30) -> dict:
    """First batch split evenly across the values of dimension `by` (the plan's `batch_by`).

    A plain covering design over a plan whose first dimension is a hierarchy (three campaign territories
    that exclude different values of everything else) came out 8 / 16 / 7 across the territories, because
    the territory with the most valid values needs the most rows to show every pair. Here each value of
    `by` gets its own covering design and an equal share of the batch (`target` divided by the number of
    values, rounded up). If a share is smaller than that value's covering minimum it keeps the rows that
    cover the most pairs, and the result says how many pairs that leaves out.
    Returns the same keys as `cover` plus `strata` {value: {rows, covering_min, complete}}."""
    if by not in dims or len(dims[by]) < 2:
        raise PlanError(f"batch_by {by!r} must name a dimension with at least two values")
    valid = enumerate_valid(dims, excl)
    if not valid:
        raise PlanError("every combination is excluded by the constraints")
    names = list(dims)
    by_pos = names.index(by)
    present = {r[by_pos] for r in valid}
    nonempty = [v for v in dims[by] if v in present]
    # a target of zero or less means "each value's own covering design"; a value with no valid rows gives its share to the others
    share = (10 ** 9) if target <= 0 else max(1, -(-target // max(1, len(nonempty))))
    rows, strata, minimum = [], {}, 0
    for v in dims[by]:
        if v not in present:
            strata[v] = {"rows": 0, "covering_min": 0, "complete": False, "empty": True}
            continue
        sub = {**dims, by: [v]}
        part = cover(sub, excl, target=min(share, 10 ** 6))
        got = part["rows"]
        if len(got) > share:
            got = _greedy_pairs(got, share)
        rows += got
        minimum += part["covering_min"]
        strata[v] = {"rows": len(got), "covering_min": part["covering_min"], "complete": len(got) >= part["covering_min"]}
    pos = {r: i for i, r in enumerate(valid)}
    rows = sorted(set(rows), key=pos.get)
    cov = coverage(rows, valid)
    product = 1
    for vals in dims.values():
        product *= len(vals)
    return {
        "names": names, "rows": rows, "covering_min": minimum, "by": by, "strata": strata,
        "valid_count": len(valid), "excluded_count": product - len(valid),
        "pairs": {"all": all_pairs(dims), "coverable": cov["coverable"], "impossible": all_pairs(dims) - cov["coverable"]},
        "coverage": {"covered": cov["covered"], "fraction": cov["fraction"]},
    }


def cover_for_plan(plan: dict, dims: dict[str, list[str]], target: int = 30) -> dict:
    """`cover`, or `cover_by` when the plan sets `batch_by`."""
    by = plan.get("batch_by")
    if by:
        return cover_by(dims, excludes(plan), by, target)
    return cover(dims, excludes(plan), target=target)
