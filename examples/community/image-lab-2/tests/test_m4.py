"""Offline tests for M4: controls catalog, planner dry run, planner evaluation checker, atomic write."""
import copy
import json
import os
import subprocess
import sys
import unittest
from pathlib import Path
from unittest import mock

HERE = Path(__file__).resolve().parent
SCRIPTS = HERE.parent / "scripts"
sys.path.insert(0, str(SCRIPTS))
sys.path.insert(0, str(HERE))
import compile as cp  # noqa: E402
import experiment as ex  # noqa: E402
import ledger as lg  # noqa: E402
import matrix as mx  # noqa: E402
import planner_eval as pe  # noqa: E402
import preflight as pf_mod  # noqa: E402
from test_preflight import ADVISOR, Base, PLAN  # noqa: E402

GOLDEN = HERE / "planner-eval" / "golden"
BRIEFS = pe.load_briefs()
ENV = {**os.environ, "PYTHONUTF8": "1"}


def golden(bid):
    return json.loads((GOLDEN / f"{bid}.json").read_text(encoding="utf-8"))


class CatalogTests(unittest.TestCase):
    def test_every_value_has_wording_and_says_how_far_it_was_tested(self):
        cat = cp.load_catalog()
        self.assertTrue({"lighting", "camera", "composition", "style", "mood", "environment"} <= set(cat))
        self.assertNotIn("aspect", cat)                      # a size, not wording
        self.assertNotIn("note", cat)                        # metadata keys are filtered out
        for dim, vals in cat.items():
            for val, e in vals.items():
                self.assertTrue(e.get("fragment"), f"{dim}/{val}")
                self.assertTrue(e.get("validation"), f"{dim}/{val} must say how it was tested")

    def test_the_m0_results_are_in_the_catalog(self):
        cam, light = cp.load_catalog()["camera"], cp.load_catalog()["lighting"]
        self.assertTrue(all(light[v]["validation"].startswith("m0") for v in
                            ("soft daylight", "golden hour", "dramatic spotlight")))
        self.assertIn("30 degrees", cam["three-quarter high"]["fragment"])                    # round-1 wording for text
        self.assertIn("35 degrees", cam["three-quarter high"]["fragment_with_reference"])      # stronger with a reference
        self.assertEqual(cam["low angle"]["reliability"], {"reference": "unreliable"})
        self.assertIn("strong warm orange color cast", light["golden hour"]["fragment"])

    def test_plain_values_use_catalog_wording_and_flag_untested_ones(self):
        plan = {"brief": "b", "intent": "generic", "dimensions": {"camera": ["eye-level front", "top-down"]}}
        cat = cp.load_catalog()
        tested = cp.compile_prompt(plan, {"camera": "eye-level front"}, "text", cat)
        self.assertIn("Eye-level front view", tested["prompt"])
        self.assertEqual(tested["unvalidated"], [])
        untested = cp.compile_prompt(plan, {"camera": "top-down"}, "text", cat)
        self.assertEqual(untested["unvalidated"], [("camera", "top-down")])
        self.assertEqual(untested["custom"], [])
        self.assertEqual(cp.compile_prompt(plan, {"camera": "low angle"}, "reference", cat)["unreliable"],
                         [("camera", "low angle")])

    def test_aspect_never_leaks_into_the_prompt(self):
        plan = {"brief": "b", "intent": "generic", "dimensions": {"camera": ["eye-level front"], "aspect": ["16:9"]}}
        out = cp.compile_prompt(plan, {"camera": "eye-level front", "aspect": "16:9"}, "text", cp.load_catalog())
        self.assertNotIn("16:9", out["prompt"])
        self.assertEqual(out["custom"], [])

    def test_plan_wording_beats_the_catalog(self):
        plan = {"brief": "b", "intent": "generic", "dimensions": {"lighting": [
            {"value": "soft daylight", "fragment": "MY OWN WORDING"}]}}
        out = cp.compile_prompt(plan, {"lighting": "soft daylight"}, "text", cp.load_catalog())
        self.assertIn("MY OWN WORDING", out["prompt"])

    def test_controls_command_lists_the_vocabulary(self):
        r = subprocess.run([sys.executable, str(SCRIPTS / "image.py"), "controls", "--dimension", "camera"],
                           capture_output=True, text=True, encoding="utf-8", env=ENV)
        self.assertEqual(r.returncode, 0, r.stderr)
        self.assertIn("eye-level front", r.stdout)
        self.assertIn("UNTESTED", r.stdout)
        self.assertIn("unreliable with a reference", r.stdout)


class DryRunTests(Base):
    def test_dry_run_writes_nothing_and_reports_the_size(self):
        out = self.tmp / "exp"
        src = self.tmp / "plan.json"
        src.write_text(json.dumps(golden("too-wide")), encoding="utf-8")
        s = ex.build_experiment(src, out, dry_run=True)
        self.assertFalse(out.exists())
        self.assertEqual(s["valid"], 81)
        self.assertEqual(s["recommended"], 30)
        text = ex.format_plan_summary(s)
        self.assertIn("Dry run: nothing was written", text)
        self.assertTrue(any("camera: 'top-down', 'wide shot' use starter wording" in w for w in s["warnings"]))

    def test_dry_run_works_before_consent_but_the_real_build_is_refused(self):
        src = self.tmp / "plan.json"
        src.write_text(json.dumps(golden("person-photo")), encoding="utf-8")
        s = ex.build_experiment(src, self.tmp / "exp", dry_run=True)
        self.assertTrue(any("person is pictured" in w for w in s["warnings"]))
        with self.assertRaises(mx.PlanError):
            ex.build_experiment(src, self.tmp / "exp")

    def test_check_command_validates_a_plan_file(self):
        good = self.tmp / "good.json"
        good.write_text(json.dumps(golden("banner-aspect")), encoding="utf-8")
        bad = self.tmp / "bad.json"
        bad.write_text(json.dumps({"schema_version": 1, "brief": "x"}), encoding="utf-8")
        run = lambda p: subprocess.run([sys.executable, str(SCRIPTS / "image.py"), "check", "--plan", str(p)],
                                       capture_output=True, text=True, encoding="utf-8", env=ENV)
        self.assertEqual(run(good).returncode, 0, run(good).stderr)
        r = run(bad)
        self.assertNotEqual(r.returncode, 0)
        self.assertIn("intent is required", r.stderr)


class EvaluationFindingsTests(Base):
    """Fixes for problems the independent planner agents reported (2026-10-07)."""

    def run_cli(self, *args):
        return subprocess.run([sys.executable, str(SCRIPTS / "image.py"), *args], capture_output=True,
                              text=True, encoding="utf-8", env=ENV)

    def test_check_plan_says_plainly_whether_the_plan_is_valid(self):
        good = self.tmp / "g.json"
        good.write_text(json.dumps(golden("too-wide")), encoding="utf-8")
        r = self.run_cli("check", "--plan", str(good))
        self.assertEqual(r.returncode, 0, r.stderr)
        self.assertIn("PLAN OK", r.stderr)
        self.assertIn("81 valid combination", r.stderr)
        bad = self.tmp / "b.json"
        bad.write_text(json.dumps({"schema_version": 1, "brief": "x"}), encoding="utf-8")
        r = self.run_cli("check", "--plan", str(bad))
        self.assertEqual(r.returncode, 1)
        self.assertIn("PLAN INVALID", r.stderr)

    def test_a_missing_plan_file_is_a_clean_error_not_a_traceback(self):
        r = self.run_cli("check", "--plan", str(self.tmp / "nope.json"))
        self.assertEqual(r.returncode, 1)
        self.assertIn("plan file not found", r.stderr)
        self.assertNotIn("Traceback", r.stderr)
        r = self.run_cli("plan", "--plan", str(self.tmp / "nope.json"), "--out", str(self.tmp / "o"), "--dry-run")
        self.assertNotIn("Traceback", r.stderr)

    def test_the_dry_run_shows_price_basis_unverified_count_sample_prompt_and_missing_reference(self):
        src = self.tmp / "plan.json"
        src.write_text(json.dumps(golden("product-photo")), encoding="utf-8")
        s = ex.build_experiment(src, self.tmp / "o", dry_run=True)
        text = ex.format_plan_summary(s)                  # the test fixture has no reference price: say so, not "$0"
        self.assertIn("no verified price for this model and mode; all 12 row(s) unverified", text)
        support = json.loads(pf_mod.REF_SUPPORT_FILE.read_text(encoding="utf-8"))["models"]
        for m in support:
            support[m]["usd_per_image_reference"] = 0.0225
        with mock.patch.object(pf_mod, "load_reference_support", lambda path=None: support):
            priced = ex.format_plan_summary(ex.build_experiment(src, self.tmp / "o", dry_run=True))
        self.assertIn("$0.0225 per image (reference call, measured price); 0 row(s) with no verified price", priced)
        self.assertIn("Sample prompt (first row, reference mode)", text)
        self.assertTrue(any("refs/product.png was not found" in w for w in s["warnings"]))
        (self.tmp / "refs").mkdir()
        (self.tmp / "refs" / "product.png").write_bytes(b"x")
        s2 = ex.build_experiment(src, self.tmp / "o", dry_run=True)
        self.assertFalse(any("not found" in w for w in s2["warnings"]))

    def test_fixed_items_reach_the_reference_prompt(self):
        plan = golden("product-photo")
        out = cp.compile_prompt(plan, {"camera": "eye-level front", "lighting": "soft daylight"}, "reference",
                                cp.load_catalog(), "product")
        self.assertIn("Keep this the same in every image", out["prompt"])
        self.assertIn("unchanged in shape, color, branding and proportions", out["prompt"])
        self.assertIn(plan["fixed"]["product identity"].split(",")[0], out["prompt"])
        explicit = copy.deepcopy(plan)
        explicit["prompt"] = {"reference_prefix": "MY PREFIX"}
        out2 = cp.compile_prompt(explicit, {"camera": "eye-level front"}, "reference", cp.load_catalog(), "product")
        self.assertTrue(out2["prompt"].startswith("MY PREFIX"))
        self.assertNotIn("Keep this the same", out2["prompt"])           # an explicit prefix is used as written

    def test_a_style_dimension_gets_the_neutral_closing_sentence(self):
        styled = {"brief": "b", "intent": "generic", "dimensions": {"style": ["flat illustration", "photorealistic"]}}
        out = cp.compile_prompt(styled, {"style": "flat illustration"}, "text", cp.load_catalog())
        self.assertTrue(out["prompt"].endswith("High quality."))
        self.assertNotIn("Realistic", out["prompt"])
        plain = {"brief": "b", "intent": "generic", "dimensions": {"camera": ["eye-level front"]}}
        self.assertTrue(cp.compile_prompt(plain, {"camera": "eye-level front"}, "text", cp.load_catalog())
                        ["prompt"].endswith("Realistic, high quality."))
        explicit = dict(styled, prompt={"text_suffix": "Finished poster."})
        self.assertTrue(cp.compile_prompt(explicit, {"style": "flat illustration"}, "text", cp.load_catalog())
                        ["prompt"].endswith("Finished poster."))

    def test_catalog_marks_product_only_wording_and_has_portrait_entries(self):
        cat = cp.load_catalog()
        for dim, val in (("lighting", "dramatic spotlight"), ("camera", "low angle"),
                         ("composition", "flat lay"), ("style", "photorealistic")):
            self.assertEqual(cat[dim][val].get("scope"), "product", f"{dim}/{val}")
        self.assertEqual(cat["camera"]["head-and-shoulders"]["scope"], "portrait")
        self.assertIn("natural portrait", cat["style"])
        r = self.run_cli("controls", "--dimension", "lighting")
        self.assertIn("written for a product on a surface", r.stdout)

    def test_fixed_text_that_names_a_reference_id_is_flagged_by_the_dry_run_and_the_checker(self):
        plan = copy.deepcopy(golden("product-photo"))
        plan["fixed"]["product identity"] = "the bottle in ref1, unchanged"
        src = self.tmp / "plan.json"
        src.write_text(json.dumps(plan), encoding="utf-8")
        s = ex.build_experiment(src, self.tmp / "o", dry_run=True)
        self.assertTrue(any("mentions ref1" in w for w in s["warnings"]))
        self.assertTrue(any("mentions ref1" in f for f in pe.check_plan(plan, BRIEFS["product-photo"]["expect"])))
        self.assertFalse(any("mentions" in w for w in ex.build_experiment(
            self.write_golden("product-photo"), self.tmp / "o", dry_run=True)["warnings"]))

    def write_golden(self, bid):
        p = self.tmp / f"{bid}.json"
        p.write_text(json.dumps(golden(bid)), encoding="utf-8")
        return p

    def test_the_dry_run_prints_every_wording_the_size_and_flags_own_wording(self):
        s = ex.build_experiment(self.write_golden("banner-aspect"), self.tmp / "o", dry_run=True)
        text = ex.format_plan_summary(s)
        self.assertIn("Wording used (text mode):", text)
        self.assertIn("style / editorial:", text)
        self.assertIn("Size on ", text)
        self.assertIn("16:9 requested ->", text)
        plan = copy.deepcopy(golden("banner-aspect"))
        plan["dimensions"]["composition"][0] = {"value": "headline left", "fragment": "Empty space on the left"}
        src = self.tmp / "p2.json"
        src.write_text(json.dumps(plan), encoding="utf-8")
        s2 = ex.build_experiment(src, self.tmp / "o", dry_run=True)
        self.assertTrue(any("headline left" in w and "written in the plan" in w for w in s2["warnings"]))

    def test_check_plan_recognizes_the_quick_route_marker(self):
        m = self.tmp / "quick.json"
        m.write_text(json.dumps({"route": "quick"}), encoding="utf-8")
        r = self.run_cli("check", "--plan", str(m))
        self.assertEqual(r.returncode, 0)
        self.assertIn("ROUTE quick", r.stderr)

    def test_the_person_notice_reads_correctly_for_your_own_photo_too(self):
        n = mx.PERSON_NOTICE
        self.assertIn("your own photo", n)
        self.assertIn("permission of the person pictured", n)
        self.assertIn("sent to CogFoundry and the image model's provider", n)
        self.assertIn("stay local", n)

    def test_environment_wording_names_things_that_must_be_visible(self):
        env = cp.load_catalog()["environment"]
        for v in ("studio backdrop", "home interior", "outdoors", "office", "city street"):
            self.assertIn(v, env)
        self.assertIn("clearly visible", env["outdoors"]["fragment"])
        self.assertIn("sofa", env["home interior"]["fragment"])
        self.assertTrue(all(e["validation"] == "unvalidated" for e in env.values()))

    def test_a_plan_value_named_like_a_catalog_value_is_reported_as_an_override(self):
        plan = copy.deepcopy(golden("banner-aspect"))
        plan["dimensions"]["style"][0] = {"value": "editorial", "fragment": "My own editorial wording"}
        src = self.tmp / "p.json"
        src.write_text(json.dumps(plan), encoding="utf-8")
        s = ex.build_experiment(src, self.tmp / "o", dry_run=True)
        self.assertTrue(any("same name as a catalog value" in w and "editorial" in w for w in s["warnings"]))
        plain = ex.build_experiment(self.write_golden("banner-aspect"), self.tmp / "o", dry_run=True)
        self.assertFalse(any("same name as a catalog value" in w for w in plain["warnings"]))

    def test_visual_checks_are_validated_printed_and_required_by_the_checker(self):
        bad = copy.deepcopy(golden("banner-aspect"))
        bad["visual_checks"] = ["ok", ""]
        self.assertTrue(any("visual_checks" in e for e in mx.validate_plan(bad)))
        bad["visual_checks"] = ["x"] * 7
        self.assertTrue(any("visual_checks" in e for e in mx.validate_plan(bad)))
        text = ex.format_plan_summary(ex.build_experiment(self.write_golden("banner-aspect"), self.tmp / "o",
                                                          dry_run=True))
        self.assertIn("Check by eye on the first batch:", text)
        self.assertIn("no text, letters, numbers or logos", text)
        none = copy.deepcopy(golden("banner-aspect"))
        none.pop("visual_checks")
        f = pe.check_plan(none, BRIEFS["banner-aspect"]["expect"])
        self.assertTrue(any("visual_checks" in x for x in f))
        self.assertEqual(pe.check_plan(golden("banner-aspect"), BRIEFS["banner-aspect"]["expect"]), [])

    def test_visual_checks_reach_the_workbook_read_me(self):
        import workbook as wbk
        lines = wbk.plan_summary(golden("banner-aspect"), list(golden("banner-aspect")["dimensions"]))
        self.assertTrue(any(l.startswith("Check by eye on the first batch:") for l in lines))

    def test_a_single_fixed_aspect_does_not_use_one_of_the_four_dimension_slots(self):
        plan = copy.deepcopy(golden("too-wide"))                                   # already 4 variable dimensions
        plan["dimensions"]["aspect"] = ["4:5"]
        ex_expect = dict(BRIEFS["too-wide"]["expect"], max_dims=4)
        self.assertEqual(pe.check_plan(plan, ex_expect), [])
        plan["dimensions"]["aspect"] = ["4:5", "1:1"]                              # two shapes: that is a variable
        self.assertTrue(any("5 dimensions" in f for f in pe.check_plan(plan, ex_expect)))

    def test_the_too_wide_rule_is_about_valid_combinations_not_the_capped_batch(self):
        wide = copy.deepcopy(golden("too-wide"))
        wide["dimensions"]["mood"] = ["warm", "calm", "energetic", "playful"]
        wide["dimensions"]["camera"] += ["close-up"]
        wide["dimensions"]["lighting"] += ["studio softbox"]
        wide["dimensions"]["style"] += ["minimalist"]
        f = pe.check_plan(wide, dict(BRIEFS["too-wide"]["expect"], max_dims=4))
        self.assertTrue(any("too wide" in x for x in f), f)


class PlannerEvalTests(unittest.TestCase):
    def test_every_golden_plan_passes_its_brief(self):
        for bid, b in BRIEFS.items():
            self.assertEqual(pe.check_plan(golden(bid), b["expect"]), [], bid)

    def test_the_golden_plans_are_real_plans(self):
        for bid in BRIEFS:
            if BRIEFS[bid]["expect"].get("route") != "quick":
                self.assertEqual(mx.validate_plan(golden(bid)), [], bid)

    def fails(self, bid, mutate):
        plan = copy.deepcopy(golden(bid))
        mutate(plan)
        return pe.check_plan(plan, BRIEFS[bid]["expect"])

    def test_a_planner_that_acknowledges_consent_itself_fails(self):
        f = self.fails("person-photo", lambda p: p.update(consent_acknowledged="2026-10-07T10:00:00"))
        self.assertTrue(any("never acknowledge" in x for x in f))

    def test_wrong_reference_role_or_person_flag_fails(self):
        f = self.fails("person-photo", lambda p: p["references"][0].update(role="general", contains_person=False))
        self.assertTrue(any("role" in x for x in f) and any("contains_person" in x for x in f))

    def test_too_many_dimensions_or_values_fails(self):
        def widen(p):
            p["dimensions"]["composition"] = ["centered", "rule of thirds", "negative space"]
        self.assertTrue(any("dimensions" in x for x in self.fails("too-wide", widen)))
        f = self.fails("product-photo", lambda p: p["dimensions"].update(
            lighting=["soft daylight", "golden hour", "dramatic spotlight", "studio softbox", "rim light"]))
        self.assertTrue(any("5 values" in x for x in f))

    def test_a_value_with_no_wording_fails(self):
        f = self.fails("product-photo", lambda p: p["dimensions"]["lighting"].append("candlelit"))
        self.assertTrue(any("no wording" in x for x in f))

    def test_camera_wording_without_a_reference_variant_fails_when_a_photo_is_used(self):
        def own(p):
            p["dimensions"]["camera"] = ["eye-level front", {"value": "bird's eye", "fragment": "Shot from far above"}]
        f = self.fails("product-photo", own)
        self.assertTrue(any("fragment_with_reference" in x for x in f))

    def test_offering_an_unreliable_camera_value_with_a_reference_fails(self):
        f = self.fails("product-photo", lambda p: p["dimensions"]["camera"].append("low angle"))
        self.assertTrue(any("unreliable" in x for x in f))

    def test_missing_aspect_or_wording_on_aspect_fails(self):
        f = self.fails("banner-aspect", lambda p: p["dimensions"].pop("aspect"))
        self.assertTrue(any("aspect" in x for x in f))
        f = self.fails("banner-aspect", lambda p: p["dimensions"].update(aspect=["wide", "16:9"]))
        self.assertTrue(any("ratios" in x for x in f))
        f = self.fails("banner-aspect", lambda p: p["dimensions"].update(aspect=[{"value": "16:9", "fragment": "wide"}, "3:2"]))
        self.assertTrue(any("not wording" in x for x in f))

    def test_empty_fixed_fails_and_routing_is_checked_both_ways(self):
        self.assertTrue(any("`fixed` is empty" in x for x in self.fails("product-photo", lambda p: p.update(fixed={}))))
        self.assertTrue(pe.check_plan(golden("product-photo"), BRIEFS["prompt-only"]["expect"]))      # a plan where quick is right
        self.assertTrue(pe.check_plan({"route": "quick"}, BRIEFS["product-photo"]["expect"]))         # quick where a plan is needed

    def test_cli_runs_the_whole_set(self):
        r = subprocess.run([sys.executable, str(SCRIPTS / "planner_eval.py"), "--all", str(GOLDEN)],
                           capture_output=True, text=True, encoding="utf-8", env=ENV)
        self.assertEqual(r.returncode, 0, r.stdout + r.stderr)
        self.assertEqual(r.stdout.count("PASS"), len(BRIEFS))


class AtomicWriteTests(unittest.TestCase):
    def test_a_transient_permission_error_on_rename_is_retried(self):
        import tempfile
        d = Path(tempfile.mkdtemp())
        real = os.replace
        calls = {"n": 0}

        def flaky(src, dst):
            calls["n"] += 1
            if calls["n"] <= 3:
                raise PermissionError("[WinError 5] Access is denied")
            return real(src, dst)

        with mock.patch.object(os, "replace", flaky):
            lg.write_json_atomic(d / "x.json", {"a": 1})
        self.assertEqual(json.loads((d / "x.json").read_text(encoding="utf-8")), {"a": 1})
        self.assertGreaterEqual(calls["n"], 4)
        self.assertEqual([p.name for p in d.iterdir()], ["x.json"])        # no temp file left behind


if __name__ == "__main__":
    unittest.main()
