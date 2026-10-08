"""Prompt compiler for Image Lab 2 (design-v2.md sections 6.2 and 8).

plan + a row's parameter values + a mode -> prompt TEXT only. It never builds a gateway
request (that stays in image.py's one request builder). Deterministic: the same inputs
always give the same text.

Modes: "text" (no reference image) and "reference" (a reference image is sent). A value's
wording can differ by mode (`fragment` / `fragment_with_reference`), because M0 showed
camera wording that works without a reference can break product pose with one.
"""
from __future__ import annotations

import json
from pathlib import Path

GENERIC = "{value} as the {dimension}"
DEFAULT_TEXT_SUFFIX = "Realistic, high quality."
DEFAULT_REFERENCE_SUFFIX = "Realistic, high quality."
NEUTRAL_SUFFIX = "High quality."          # used when the plan varies `style`: "Realistic" would fight an illustration
MODES = ("text", "reference")


def load_catalog(path=None) -> dict:
    """Optional controls catalog: {dimension: {value: {fragment, fragment_with_reference,
    reliability}}}. Missing file reads as empty. The plan always wins over the catalog."""
    p = Path(path) if path else Path(__file__).resolve().parent.parent / "references" / "controls-catalog.json"
    if not p.exists():
        return {}
    data = json.loads(p.read_text(encoding="utf-8"))
    return {k: v for k, v in data.items() if isinstance(v, dict)}      # drop schema_version / note


def _entry(plan: dict, dim: str, value: str, catalog: dict) -> dict:
    for v in plan.get("dimensions", {}).get(dim, []):
        if isinstance(v, dict) and v.get("value") == value:
            return v
    return (catalog or {}).get(dim, {}).get(value, {})


def _in_plan(plan: dict, dim: str, value: str) -> bool:
    return any((v["value"] if isinstance(v, dict) else v) == value for v in plan.get("dimensions", {}).get(dim, []))


def fragment(plan: dict, dim: str, value: str, mode: str, catalog: dict | None = None) -> tuple[str, str]:
    """(wording, source). source is "plan", "catalog" or "generic" (a custom value with no
    wording of its own)."""
    if mode not in MODES:
        raise ValueError(f"mode must be one of {MODES}")
    e = _entry(plan, dim, value, catalog or {})
    src = "plan" if _in_plan(plan, dim, value) and isinstance(
        next((v for v in plan["dimensions"][dim] if (v["value"] if isinstance(v, dict) else v) == value), None), dict) else "catalog"
    if mode == "reference" and e.get("fragment_with_reference"):
        return e["fragment_with_reference"], src
    if e.get("fragment"):
        return e["fragment"], src
    return GENERIC.format(value=value, dimension=dim), "generic"


def validation(plan: dict, dim: str, value: str, catalog: dict | None = None) -> str:
    """How far this value's wording was tested: the catalog's `validation` text, or "plan" when
    the plan supplies its own wording (the planner's responsibility), or "unvalidated"."""
    e = _entry(plan, dim, value, catalog or {})
    if isinstance(next((v for v in plan.get("dimensions", {}).get(dim, [])
                        if isinstance(v, dict) and v.get("value") == value), None), dict):
        return "plan"
    return e.get("validation") or "unvalidated"


def reliability(plan: dict, dim: str, value: str, mode: str, catalog: dict | None = None) -> str:
    """"ok" unless the plan or catalog marks this value unreliable in this mode."""
    return (_entry(plan, dim, value, catalog or {}).get("reliability") or {}).get(mode, "ok")


def reference_roles(plan: dict) -> list[str]:
    return [r["role"] for r in plan.get("references", [])]


ROLE_PREFIX = {
    "person": ("Use the reference image only to identify the person: keep their face and likeness "
               "unchanged. Do not copy the pose, framing, clothing or background of the reference image."),
    "style": ("Use the reference image only as a style guide (palette, light, texture and rendering): "
              "do not copy its subject or composition."),
    "general": "Use the reference image as the visual basis: keep its main subject recognizable.",
}


def default_reference_prefix(plan: dict, role: str | None = None) -> str:
    """The sentence stating how the reference relates to the output (design 8). `role` is the
    role of the reference this row uses; without it, the plan's roles are combined."""
    if role in ROLE_PREFIX:
        return ROLE_PREFIX[role]
    roles = [role] if role else (sorted(set(reference_roles(plan))) or ["product"])
    return (f"Use the reference image only to identify the {' and '.join(roles)}: keep it unchanged in "
            f"shape, color, branding and proportions. Do not copy the tilt, rotation, pose or "
            f"viewpoint of the reference image.")


def default_text_prefix(plan: dict) -> str:
    fixed = [str(v).strip() for v in (plan.get("fixed") or {}).values() if str(v).strip()]
    return ". ".join(s.rstrip(". ") for s in fixed) + "." if fixed else plan["brief"].rstrip(". ") + "."


def resolve_reference(plan: dict, row: dict) -> tuple[list[str], str | None]:
    """(reference ids this row uses, a problem or None). The Reference cell names one id, or
    `none` for a text-only row; blank means the plan's reference when it has exactly one. One
    reference per row for now (the verified request form)."""
    plan_ids = [r["id"] for r in plan.get("references", [])]
    cell = str(row.get("reference") or "").strip()
    if cell.lower() == "none":
        return [], None
    if cell:
        ids = [x.strip() for x in cell.split(",") if x.strip()]
        if len(ids) > 1:
            return ids, f"one reference per row is supported (got {', '.join(ids)})"
        return ids, None
    if len(plan_ids) <= 1:
        return plan_ids, None
    return [], f"choose a reference for this row ({', '.join(plan_ids)}) or enter none"


def with_fixed(prefix: str, plan: dict) -> str:
    """Append the plan's Fixed items to the default reference sentence, so what the planner said
    must not change reaches the model (text mode already builds its prefix from `fixed`)."""
    fixed = [str(v).strip().rstrip(". ") for v in (plan.get("fixed") or {}).values() if str(v).strip()]
    if not fixed:
        return prefix
    return prefix.rstrip(". ") + ". Keep this the same in every image: " + "; ".join(fixed)


def mode_for_row(plan: dict, row: dict) -> str:
    """A row runs in reference mode when it uses a reference (see resolve_reference)."""
    ids, _ = resolve_reference(plan, row)
    return "reference" if ids else "text"


def compile_prompt(plan: dict, params: dict, mode: str, catalog: dict | None = None,
                   ref_role: str | None = None) -> dict:
    """{prompt, custom: [(dim, value)], unreliable: [(dim, value)]}"""
    cfg = plan.get("prompt") or {}
    prefix = (cfg.get("reference_prefix") or with_fixed(default_reference_prefix(plan, ref_role), plan)) \
        if mode == "reference" else (cfg.get("text_prefix") or default_text_prefix(plan))
    varies_style = "style" in plan.get("dimensions", {})
    suffix = (cfg.get("reference_suffix") or (NEUTRAL_SUFFIX if varies_style else DEFAULT_REFERENCE_SUFFIX)) \
        if mode == "reference" \
        else (cfg.get("text_suffix") or (NEUTRAL_SUFFIX if varies_style else DEFAULT_TEXT_SUFFIX))
    parts, custom, unreliable, unvalidated, no_wording = [], [], [], [], []
    for dim in plan["dimensions"]:                 # plan order, not the row's order
        if dim not in params or dim == "aspect":   # aspect is a size (chosen per model), not wording
            continue
        value = params[dim]
        text, source = fragment(plan, dim, value, mode, catalog)
        if not _in_plan(plan, dim, value):             # typed in the workbook; not one of the confirmed values
            custom.append((dim, value))
        if source == "generic":
            no_wording.append((dim, value))
        if reliability(plan, dim, value, mode, catalog) != "ok":
            unreliable.append((dim, value))
        if source == "catalog" and validation(plan, dim, value, catalog).startswith("unvalidated"):
            unvalidated.append((dim, value))
        parts.append(text.rstrip(". "))
    sentences = [prefix.rstrip(". ")] + parts + [suffix.rstrip(". ")]
    return {"prompt": ". ".join(s for s in sentences if s) + ".", "custom": custom, "no_wording": no_wording, "unreliable": unreliable,
            "unvalidated": unvalidated}
