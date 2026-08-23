#!/usr/bin/env python3
"""
package-share.py — local, free. Builds the Share & Reproduce artifact from
whatever the run actually produced in .output/. No new writing at share time;
this only assembles what's already there.

Usage:
    python package-share.py --output-dir .output --prompt "..." --out .output/share-artifact.md
"""

import argparse
import json
import re
from datetime import datetime, timezone
from pathlib import Path

# Real repo per known design authority -- named consistently with how the
# redesign-lab design spec itself cites these (owner/repo form). Extend
# this map, don't guess a URL, when a new authority is registered in
# design-authority.md.
AUTHORITY_REPOS = {
    "leonxlnx-taste-skill": "https://github.com/Leonxlnx/taste-skill",
    "baoyu-design": "https://github.com/JimLiu/baoyu-design",
}

REDESIGN_LAB_REPO = "https://github.com/cogfoundry-labs/loomloom/tree/main/examples/community/redesign-lab"
LOOMLOOM_REPO = "https://github.com/cogfoundry-labs/loomloom"


def read_json_if_exists(path):
    p = Path(path)
    return json.loads(p.read_text(encoding="utf-8")) if p.exists() else None


def gather_evidence(output_dir):
    """Real, evidence-gated facts about what actually ran in this output
    directory -- reused by build-case-study.py's reproducibility section so
    that script and this one never maintain two separate, driftable copies
    of the same gating logic. Returns a dict; callers decide how to render
    it (Markdown here, a case-study section there).

    `tools_used` is a list of {name, repo, role} dicts, not plain strings:
    a case study crediting real open-source tools should let a reader click
    through to each one, and say what it actually did for this specific run
    -- not just name-drop it. Every entry is still evidence-gated: it only
    appears if the real file/directory proving it ran actually exists."""
    out_dir = Path(output_dir)
    discover = read_json_if_exists(out_dir / "discover.json") or {}
    analysis_path = out_dir / "analysis.md"
    analysis_text = analysis_path.read_text(encoding="utf-8") if analysis_path.exists() else None
    validate_path = out_dir / "validate-report.md"
    validate_text = validate_path.read_text(encoding="utf-8") if validate_path.exists() else None
    directions = sorted((out_dir / "directions").glob("*")) if (out_dir / "directions").exists() else []
    variants = sorted((out_dir / "variants").glob("*")) if (out_dir / "variants").exists() else []
    design_authority = discover.get("design_authority")
    if not design_authority and analysis_text:
        # discover.json doesn't exist for a run against a live external site
        # this pipeline doesn't own (no local project to write it into) --
        # but Analyze always states the declared authority in analysis.md's
        # own opening line. Fall back to reading it from there rather than
        # under-crediting the authority just because this run had no
        # discover.json to begin with.
        m = re.search(r"Authority:\s*`([a-zA-Z0-9_-]+)`", analysis_text)
        if m:
            design_authority = m.group(1)

    tools_used = [{
        "name": "redesign-lab",
        "repo": REDESIGN_LAB_REPO,
        "role": "The end-to-end pipeline (Discover, Analyze, Explore, Select, Implement, Validate, Share) that produced this entire redesign and case study.",
    }]
    if analysis_text:
        tools_used.append({
            "name": "senlindesign/taste-skill",
            "repo": "https://github.com/senlindesign/taste-skill",
            "role": "Measured the original page's real design system (colors, type, spacing, layout) via screenshot and DOM extraction -- the raw input to Analyze.",
        })
    if design_authority:
        tools_used.append({
            "name": design_authority,
            "repo": AUTHORITY_REPOS.get(design_authority),
            "role": "Supplied the build rules, direction-variant families, and the mechanical Pre-Flight Check this redesign was built and validated against.",
        })
    if directions or variants:
        tools_used.append({
            "name": "anthropics/skills (webapp-testing)",
            "repo": "https://github.com/anthropics/skills/tree/main/skills/webapp-testing",
            "role": "Rendered and screenshotted every real direction, variant, and the final implementation for functional and responsive checking.",
        })
    if validate_text:
        tools_used.append({
            "name": "snapsynapse/skill-a11y-audit",
            "repo": "https://github.com/snapsynapse/skill-a11y-audit",
            "role": "Ran the real axe-core accessibility scan that found (and confirmed the fixes for) this run's Validate-stage violations.",
        })

    return {
        "discover": discover,
        "analysis_text": analysis_text,
        "validate_text": validate_text,
        "directions": [d.name for d in directions],
        "variants": [v.name for v in variants],
        "design_authority": design_authority,
        "tools_used": tools_used,
    }


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--output-dir", default=".output")
    parser.add_argument("--prompt", required=True, help="the exact prompt the user ran")
    parser.add_argument("--out", default=".output/share-artifact.md")
    args = parser.parse_args()

    evidence = gather_evidence(args.output_dir)
    analysis_text = evidence["analysis_text"]
    validate_text = evidence["validate_text"]
    directions = evidence["directions"]
    variants = evidence["variants"]
    design_authority = evidence["design_authority"]

    lines = [
        "# Redesign Lab: Share & Reproduce",
        "",
        f"_Generated {datetime.now(timezone.utc).strftime('%Y-%m-%d')}_",
        "",
        "## Before → After",
        "",
        "*(attach before.png / after.png here)*",
        "",
        "## Prompt",
        "",
        f"> {args.prompt}",
        "",
        "## Directions explored",
        "",
    ]
    if directions:
        for d in directions:
            lines.append(f"- **{d}**")
    else:
        lines.append("_(no directions/ folder found, run generate-directions.md first)_")

    lines += ["", f"## Variants explored: {len(variants)}", ""]

    lines += ["## Validation", ""]
    if validate_text:
        lines.append(validate_text.strip())
    else:
        lines.append("_(no validate-report.md found, run validate-design.md first)_")
    lines.append("")

    lines += [
        "## Design authority",
        "",
        f"`{design_authority or 'unknown'}`",
        "",
        "## Tools & skills used",
        "",
    ]
    # Built from what actually ran this session, not a fixed claim: an
    # earlier version of this list unconditionally named both of these plus
    # a11y-audit regardless of the real run — confirmed wrong against two
    # real runs this session where neither ever actually happened (analysis
    # was done by hand, Validate was never reached). Note the two lines
    # below name genuinely different, easy-to-confuse tools: `taste`
    # (senlindesign/taste-skill) is the always-invoked measurement pass in
    # self-mode Analyze (extract-design-signal.md step 1) — evidenced by
    # analysis.md existing at all; the design authority is the separate,
    # pluggable judgment layer applied to that measurement (build rules,
    # direction_variants, Pre-Flight Check) — evidenced by discover.json's
    # own design_authority field. Conflating them into one line was an
    # earlier mistake in this file, not a difference-without-a-distinction.
    # "No new writing at share time; this only assembles what's already
    # there" (this file's own docstring) applies to this section too.
    # tools_used always has at least the redesign-lab entry (the
    # pipeline itself always ran) -- the empty-state branch that used to
    # exist here can no longer trigger, so it's removed rather than kept as
    # dead code nothing will ever reach.
    for t in evidence["tools_used"]:
        name = f"[{t['name']}]({t['repo']})" if t.get("repo") else f"`{t['name']}`"
        lines.append(f"- **{name}** — {t['role']}")
    lines.append("")

    lines += [
        "## Reproduce this",
        "",
        "```bash",
        "git clone https://github.com/cogfoundry-labs/loomloom",
        "cd loomloom/examples/community/redesign-lab",
        "```",
        "",
        f"> {args.prompt}",
        "",
        "---",
        "",
        "_Made something cool? Post it in [Show and tell](https://github.com/orgs/cogfoundry-labs/discussions/categories/show-and-tell)._",
    ]

    out_path = Path(args.out)
    out_path.parent.mkdir(parents=True, exist_ok=True)
    out_path.write_text("\n".join(lines), encoding="utf-8")
    print(f"wrote {args.out}")


if __name__ == "__main__":
    main()
