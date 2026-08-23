#!/usr/bin/env python3
"""
build-case-study.py — Gate 4 / Share stage: assembles a Case Study data
model from a real run, then (only after human confirmation) generates the
one paid asset and builds the interactive web case study as a real,
GitHub-Pages-ready folder.

Architecture: Case Study data -> Web renderer (PDF/social renderers can
consume the same case-study-data.json later; this script is the only one
that writes that file, and build_case_study_site() is the only function
that reads it to produce the site folder, so a future renderer is a new
function, not a new data model).

Rev: no more image-generation. The cover used to be a real loomloom
image-generate call grounded in the actual redesign's color tokens -- real,
but still decorative, and it carried this pipeline's single biggest cost/
reliability risk (precheck undershooting the real charge by ~7x, confirmed
twice). It's gone. The hero is now typographic, built directly from data
this stage already computes ($0, no model call) -- see
render-case-study-web.py. The one loomloom call left is the narrative pass,
kept specifically because it's cheap (~$0.01) and fast for what it does:
turning real, verified facts into readable prose, at a scale (3-6 chapters
in one batch) too small to benefit from doing it by hand every run.

Two phases, matching the real gate shape (show cost, then confirm):

  plan     (free, no loomloom install/config required until this point --
           see SKILL.md): diffs before/after via diff-transformations.py,
           selects 3-6 real chapters by significance, gathers evidence via
           package-share.gather_evidence(), reads real validation counts if
           given, gets a real loomloom precheck cost for the one remaining
           paid call, writes case-study-data.json, prints the Gate 4 prompt.

  generate (paid, only after human confirmation): runs the one real
           loomloom call (narrative), fills in case-study-data.json, and
           builds the finished site folder itself --
           build_case_study_site() is called in-process using the before/
           after/logo/title/labels captured by `plan`, so nothing about
           producing the final artifact depends on the calling agent
           separately remembering those paths.

Usage:
    python build-case-study.py plan --output-dir .output/<run> \\
        --before <url-or-file> --after <url-or-file> --winning-direction <name> \\
        --subject "<one real sentence describing the site>" \\
        --status preview|implemented \\
        --before-image <real screenshot path> --after-image <real screenshot path> \\
        --title "<case study title>" \\
        --before-label "<short real characterization, e.g. 'Consumer-SaaS Card Grid'>" \\
        --after-label "<short real characterization, e.g. 'Enterprise Operations Console'>" \\
        [--logo <path>] [--canonical-url <url>] \\
        [--mechanical-report <path>] [--a11y-report <path>]

    python build-case-study.py generate --data .output/<run>/case-study-data.json
"""

import argparse
import json
import subprocess
import sys
from datetime import datetime, timezone
from pathlib import Path

sys.path.insert(0, str(Path(__file__).parent))
import importlib
diff_transformations = importlib.import_module("diff-transformations")
package_share = importlib.import_module("package-share")
render_case_study_web = importlib.import_module("render-case-study-web")

MIN_CHAPTERS = 3
MAX_CHAPTERS = 6


def run_loomloom(args_list):
    # encoding must be explicit: loomloom emits UTF-8 (its own JSON output can
    # carry any real subject/copy text this run passed in, Chinese included),
    # but subprocess's text-mode default falls back to the platform locale
    # encoding when none is given -- GBK on this Windows machine, which cannot
    # decode arbitrary UTF-8 byte sequences. Confirmed the hard way: the first
    # real run against a Chinese-subject site crashed the reader thread with
    # `UnicodeDecodeError: 'gbk' codec can't decode byte ... illegal multibyte
    # sequence`, which silently left result.stdout as None and turned into an
    # unrelated-looking `json.loads(None)` TypeError two frames later.
    result = subprocess.run(
        ["loomloom", *args_list, "-o", "json"], capture_output=True, text=True, encoding="utf-8"
    )
    if result.returncode != 0:
        sys.exit(f"loomloom {' '.join(args_list)} failed:\n{result.stderr or result.stdout}")
    return json.loads(result.stdout)


def get_or_create_template(name, spec_path, version_note):
    """Reuse one stable, named template across every Share run instead of
    registering a brand-new one each time. Confirmed real via a live
    `template-spec list` call: prior test runs this session already left
    several orphaned `case-study-cover*`/`case-study-narrative*` templates
    behind on the platform, with no cleanup path (the `-cover` family is now
    dead code below, kept registered on the platform from before this rev
    but never created again). Each run's actual content (subject, winning
    direction, real color tokens, the selected chapters) becomes a new
    immutable *version* under the one reusable template, found by exact name
    match -- not a whole new template every time."""
    listing = run_loomloom(["template-spec", "list"])
    existing = next(
        (t for t in listing.get("items", []) if t.get("name") == name and t.get("status") == "active"),
        None,
    )
    if existing:
        result = run_loomloom(["template-spec", "create-version", existing["templateId"], str(spec_path), "--version-note", version_note])
        return {"templateId": result["templateId"], "versionId": result["versionId"]}
    return run_loomloom(["template-spec", "create", str(spec_path), "--version-note", version_note])


def select_chapters(findings):
    """3-6 of the most significant real differences. Never pads to a
    minimum, never truncates real signal below 6 if that many are genuinely
    significant -- 'meaningful transformations, not transformation count.'"""
    ranked = sorted(findings, key=lambda f: -f["significance"])
    return ranked[:MAX_CHAPTERS] if len(ranked) >= MIN_CHAPTERS else ranked


NARRATIVE_TEMPLATE = {
    "meta": {
        "name": "case-study-narrative",
        "description": "Batch narrative-prose pass: turn one real, terse transformation fact into a polished 2-3 sentence case-study paragraph. Pure text, no images.",
    },
    "steps": [{
        "stepId": "stp_narr01",
        "displayName": "Write chapter narrative",
        "executionUnit": "text-generate",
        "defaultModelRef": {"modelKey": "google/gemini-2.5-flash"},
    }],
    "inputSchema": {"fields": [
        {"key": "chapter_title", "label": "Chapter title", "valueType": "string", "required": True, "order": 1},
        {"key": "facts", "label": "Real facts (before / after / why, combined)", "valueType": "string", "required": True, "order": 2},
        {
            "key": "instruction", "label": "Instruction", "valueType": "string",
            "required": False, "hidden": True, "sourceKind": "default_value",
            "defaultValue": (
                "You are writing one chapter of a real, evidence-based design case study. "
                "You are given real, verified facts about ONE specific transformation: what it "
                "was before, what it is after, and why it changed. Write a 2-3 sentence paragraph "
                "in a confident, editorial, non-hyperbolic voice. Do not invent any fact not given "
                "to you. Do not use marketing superlatives ('stunning', 'revolutionary', "
                "'game-changing'). Do not use em-dashes."
            ),
        },
    ]},
    "paramBindings": [{
        "stepId": "stp_narr01", "paramKey": "prompt", "bindMode": "shared",
        "sources": [
            {"kind": "field_ref", "fieldKey": "instruction"},
            {"kind": "field_ref", "fieldKey": "chapter_title"},
            {"kind": "field_ref", "fieldKey": "facts"},
        ],
    }],
}


def cmd_plan(args):
    out_dir = Path(args.output_dir)
    cs_dir = out_dir / "case-study"
    cs_dir.mkdir(parents=True, exist_ok=True)

    # 0. Freshness caveat: a target that's a live URL gets re-fetched right
    # now, not read from whatever was captured at Gate 2 time. For a local
    # file target (the common case: a real variant this pipeline rendered,
    # or a local project Implement wrote into) the file on disk is the same
    # file Gate 2 showed, so this doesn't apply. For a live external site
    # (the larstornoe.com "preview" case), the page could have changed since
    # approval -- surfaced honestly here and in the rendered case study,
    # rather than silently assumed to still match what the human saw.
    def is_live_url(target):
        return target.startswith("http://") or target.startswith("https://")

    live_targets = [label for label, t in [("before", args.before), ("after", args.after)] if is_live_url(t)]
    captured_at = datetime.now(timezone.utc).isoformat()
    diff_capture_note = None
    if live_targets:
        diff_capture_note = (
            f"The {' and '.join(live_targets)} target(s) were fetched live at {captured_at} "
            "(Share time), not re-read from a saved snapshot of what Gate 2 actually showed. "
            "If a live external site changed since that approval, these facts may not exactly "
            "match what the human saw and chose."
        )
        print(f"NOTE: {diff_capture_note}", file=sys.stderr)

    # 1. Real diff -- $0, no loomloom.
    from playwright.sync_api import sync_playwright
    with sync_playwright() as p:
        browser = p.chromium.launch()
        results = {}
        for label, target in [("before", args.before), ("after", args.after)]:
            page = browser.new_page(viewport={"width": 1280, "height": 800})
            page.goto(diff_transformations.to_target(target), wait_until="networkidle")
            results[label] = diff_transformations.extract_facts(page)
            page.close()
        browser.close()
    findings = diff_transformations.score_and_diff(results["before"], results["after"])
    chapters = select_chapters(findings)
    print(f"{len(findings)} real difference(s) found; {len(chapters)} selected as chapters.", file=sys.stderr)

    # 2. Real evidence -- $0, no loomloom.
    evidence = package_share.gather_evidence(args.output_dir)

    # 3. Real root colors of the winning direction -- used as real swatches
    # in the typographic hero, not fed to an image-generation prompt anymore.
    root_colors = results["after"]["root_colors"]

    # 4. Real validation counts, if the caller has them -- $0, just reading
    # JSON already produced by mechanical-check.py / a11y-audit's scan.js.
    # Optional: an older run or one that skipped a check simply omits that
    # metric from the rendered Validation section rather than guessing.
    validation = {}
    if args.mechanical_report:
        mech = json.loads(Path(args.mechanical_report).read_text(encoding="utf-8"))
        validation["mechanical_passed"] = mech.get("passed")
        validation["mechanical_total"] = mech.get("checks_run")
    if args.a11y_report:
        a11y = json.loads(Path(args.a11y_report).read_text(encoding="utf-8"))
        results_list = a11y.get("results", [])
        violations = sum(len(r.get("axe", {}).get("violations", [])) for r in results_list)
        validation["a11y_violations"] = violations

    # 5. Build and register the one remaining TemplateSpec (narrative) --
    # reusing an existing template by name where one exists -- then precheck
    # for real cost. No cover/image-generate template anymore: it was the
    # single biggest cost/reliability risk in this whole pipeline (~7x
    # precheck-undershoot, confirmed twice) for a result that was still only
    # decorative, never a real artifact. The typographic hero this rev
    # replaces it with uses data already computed above, $0, no model call.
    cs_dir.mkdir(exist_ok=True)
    narrative_spec_path = cs_dir / "narrative-template.json"
    narrative_spec_path.write_text(json.dumps(NARRATIVE_TEMPLATE, indent=2), encoding="utf-8")
    narrative_created = get_or_create_template("case-study-narrative", narrative_spec_path, "case study narrative")

    narrative_rows_path = cs_dir / "_narrative_rows.jsonl"
    with narrative_rows_path.open("w", encoding="utf-8") as f:
        for ch in chapters:
            facts = f"Before: {ch['before_fact']} After: {ch['after_fact']}"
            f.write(json.dumps({"chapter_title": ch["category"].replace("-", " ").title(), "facts": facts}) + "\n")
    narrative_input = run_loomloom(["orchestration-input", "upload", str(narrative_rows_path)])

    narrative_precheck = run_loomloom([
        "template-spec", "precheck", narrative_created["templateId"],
        "--version-id", narrative_created["versionId"], "--input-file-id", narrative_input["inputFileId"],
    ])
    narrative_est = float(narrative_precheck["precheck"]["estimatedTotalCost"]["amount"])

    data = {
        "status": args.status,
        "winning_direction": args.winning_direction,
        "subject": args.subject,
        "before_target": args.before,
        "after_target": args.after,
        "diff_capture_note": diff_capture_note,
        "diff_captured_at": captured_at,
        "root_colors": root_colors,
        "chapters": chapters,
        "evidence": evidence,
        "validation": validation,
        # Captured now, at plan time, so `generate` can render the finished
        # case study itself with no implicit hand-off step -- the calling
        # agent doesn't need to separately remember these paths later.
        "render_inputs": {
            "before_image": args.before_image,
            "after_image": args.after_image,
            "logo_image": args.logo,
            "title": args.title,
            "before_label": args.before_label,
            "after_label": args.after_label,
            "canonical_url": args.canonical_url,
        },
        "loomloom": {
            "narrative_template_id": narrative_created["templateId"],
            "narrative_version_id": narrative_created["versionId"],
            "narrative_input_file_id": narrative_input["inputFileId"],
            "narrative_precheck_usd": narrative_est,
        },
        "narrative_results": None,
    }
    data_path = out_dir / "case-study-data.json"
    data_path.write_text(json.dumps(data, indent=2), encoding="utf-8")

    print(f"\nwrote {data_path}")
    print("\n--- Gate 4: Share confirmation ---")
    print("Share")
    print("Generates:")
    print("  - typographic hero from real color tokens (free, no model call)")
    print(f"  - {len(chapters)} narrative chapter(s) -- the one real loomloom call left in Share")
    print("  - interactive before/after presentation (reused, free)")
    print("  - reproducibility information (reused, free)")
    print(f"\nEstimated cost: ~${narrative_est:.4f} (narrative only -- no image-generation anymore)")
    if diff_capture_note:
        print(f"Note: {diff_capture_note}")
    print(f"\n[ Generate Share Artifact ]  ->  python build-case-study.py generate --data {data_path}")
    print("[ Skip ]  ->  stop here, nothing spent")


def cmd_generate(args):
    data_path = Path(args.data)
    data = json.loads(data_path.read_text(encoding="utf-8"))
    out_dir = data_path.parent
    cs_dir = out_dir / "case-study"
    ll = data["loomloom"]

    def save():
        data_path.write_text(json.dumps(data, indent=2), encoding="utf-8")

    # Idempotency key tied to the actual content being submitted
    # (narrative_input_file_id), not just the output directory name. Must
    # still be stable across a retry after a crash -- same input file id,
    # same key -- so the platform's own idempotency (confirmed real:
    # "Idempotency key; auto-generated if omitted" per `template-spec run
    # --help`) can catch a resubmission of money already spent. But it must
    # also change whenever `plan` is re-run with genuinely different content
    # (different chapters/facts -> a new input file uploaded to loomloom):
    # confirmed the hard way when a key derived only from `out_dir.name`
    # collided across two real `plan` runs for the same site with different
    # payloads, and the platform correctly rejected the mismatch with
    # "idempotency_key already exists with different request payload" rather
    # than silently reusing the first run's stale result.
    narrative_request_id = f"case-study-narrative-{out_dir.name}-{ll['narrative_input_file_id'][:12]}"

    # Narrative: the one real paid call left in Share. Skipped entirely if a
    # prior invocation already completed it -- never re-charge a step that's
    # already on disk as done.
    if data.get("narrative_results"):
        print("narrative already generated in a prior run, skipping", file=sys.stderr)
    else:
        run_id2 = run_loomloom([
            "template-spec", "run", ll["narrative_template_id"],
            "--version-id", ll["narrative_version_id"], "--input-file-id", ll["narrative_input_file_id"],
            "--client-request-id", narrative_request_id,
        ])["runId"]
        watch_result2 = run_loomloom(["run", "watch", run_id2])
        actual_cost2 = watch_result2.get("actualCost") or {}
        narrative_result = run_loomloom(["run", "result-rows", run_id2])
        # Match back by rowIndex, not list position: the same pattern already
        # validated in scripts/score.py. A batched call reordering or
        # dropping rows must not silently mis-assign one chapter's narrative
        # to a different chapter, or drop one with no error.
        narratives_by_idx = {}
        for row in narrative_result["rows"]:
            idx = row["rowIndex"]
            text = next((a["inlineText"] for a in row.get("artifacts", []) if a.get("portName") == "output"), "")
            narratives_by_idx[idx] = text
        missing = [i for i in range(len(data["chapters"])) if i not in narratives_by_idx]
        if missing:
            sys.exit(
                f"narrative pass returned {len(narratives_by_idx)}/{len(data['chapters'])} row(s); "
                f"missing chapter index(es) {missing}. Refusing to silently mis-assign narratives."
            )
        for idx, ch in enumerate(data["chapters"]):
            ch["narrative"] = narratives_by_idx[idx]
        data["narrative_results"] = {"run_id": run_id2}
        data["loomloom"]["narrative_actual_usd"] = actual_cost2.get("amount")
        # Credit loomloom in the rendered "How This Was Built" section only
        # now that its one real call in this run has actually succeeded --
        # the same evidence-gating rule every other tool credit follows
        # (package_share.gather_evidence() runs at `plan` time, before this
        # call exists, so it can't gate on it itself).
        data["evidence"]["tools_used"].append({
            "name": "cogfoundry-labs/loomloom",
            "repo": "https://github.com/cogfoundry-labs/loomloom",
            "role": f"Generated the {len(data['chapters'])} narrative paragraphs in the Design Transformations section above from this run's real, verified facts.",
        })
        save()
        print(f"narrative: precheck estimated ${ll['narrative_precheck_usd']:.4f}, "
              f"actual charge {actual_cost2.get('amount', '?')} {actual_cost2.get('currency', '')}")

    save()
    print(f"updated {data_path} with real loomloom results")

    # Build the finished GitHub-Pages-ready site now, in-process --
    # render_inputs was captured at plan time specifically so this doesn't
    # depend on the calling agent separately remembering the before/after/
    # logo/title/label paths.
    render_inputs = data["render_inputs"]
    site_dir = cs_dir / "site"
    result = render_case_study_web.build_case_study_site(
        data,
        before_image=render_inputs["before_image"],
        after_image=render_inputs["after_image"],
        logo=render_inputs["logo_image"],
        title=render_inputs["title"],
        before_label=render_inputs["before_label"],
        after_label=render_inputs["after_label"],
        out_dir=site_dir,
        canonical_url=render_inputs.get("canonical_url"),
    )
    print(f"wrote {result['out_dir']}/ ({result['file_count']} files, {result['total_bytes']/1024/1024:.2f}MB total)")
    print(f"  index.html: {result['index_html_bytes']/1024:.1f}KB")


def main():
    parser = argparse.ArgumentParser()
    sub = parser.add_subparsers(dest="cmd", required=True)

    p_plan = sub.add_parser("plan")
    p_plan.add_argument("--output-dir", required=True)
    p_plan.add_argument("--before", required=True)
    p_plan.add_argument("--after", required=True)
    p_plan.add_argument("--winning-direction", required=True)
    p_plan.add_argument("--subject", required=True)
    p_plan.add_argument("--status", choices=["implemented", "preview"], required=True)
    p_plan.add_argument("--before-image", required=True, help="real screenshot path for the interactive before/after comparison")
    p_plan.add_argument("--after-image", required=True, help="real screenshot path for the interactive before/after comparison")
    p_plan.add_argument("--logo", default=None, help="optional real logo path")
    p_plan.add_argument("--title", required=True, help="case study title")
    p_plan.add_argument("--before-label", required=True, help="short, real characterization of the original design (e.g. 'Consumer-SaaS Card Grid')")
    p_plan.add_argument("--after-label", required=True, help="short, real characterization of the redesign (e.g. 'Enterprise Operations Console')")
    p_plan.add_argument("--canonical-url", default=None, help="the real URL this will be published at, if known (e.g. a GitHub Pages URL)")
    p_plan.add_argument("--mechanical-report", default=None, help="path to the chosen page's real mechanical-check.py JSON report")
    p_plan.add_argument("--a11y-report", default=None, help="path to the chosen page's real a11y-audit scan JSON")
    p_plan.set_defaults(func=cmd_plan)

    p_gen = sub.add_parser("generate")
    p_gen.add_argument("--data", required=True)
    p_gen.set_defaults(func=cmd_generate)

    args = parser.parse_args()
    args.func(args)


if __name__ == "__main__":
    main()
