"""Offline tests for scripts/matrix.py. Run from image-lab-2/:

    python -m unittest discover -s tests -v

allpairspy must be importable (pip install allpairspy, or put it on PYTHONPATH).
"""
import itertools
import random
import sys
import unittest
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent.parent / "scripts"))
import matrix as mx  # noqa: E402


def make_plan(**over):
    plan = {
        "schema_version": 1, "brief": "b", "intent": "social post",
        "dimensions": {"environment": ["a", "b", "c"], "camera": ["x", "y", "z"]},
        "constraints": [],
    }
    plan.update(over)
    return plan


class PlanValidation(unittest.TestCase):
    def test_valid_plan_passes(self):
        self.assertEqual(mx.validate_plan(make_plan()), [])

    def test_errors_are_reported(self):
        bad = make_plan(dimensions={"env": ["a", "a"]}, constraints=[{"exclude": {"env": "zzz"}}],
                        model_strategy="best", schema_version=2)
        errs = " | ".join(mx.validate_plan(bad))
        for needle in ("schema_version", "duplicate values", "unknown value", "model_strategy"):
            self.assertIn(needle, errs)

    def test_object_values_and_roles(self):
        plan = make_plan(dimensions={"lighting": [{"value": "golden", "fragment": "warm light"}, "soft"]},
                         references=[{"id": "r", "file": "f.png", "role": "robot"}])
        self.assertIn("role must be one of", " ".join(mx.validate_plan(plan)))
        self.assertEqual(mx.dimensions(make_plan(dimensions=plan["dimensions"])),
                         {"lighting": ["golden", "soft"]})


class Covering(unittest.TestCase):
    def test_6x3x3_reaches_the_pairwise_minimum_with_full_coverage(self):
        dims = {"env": [f"e{i}" for i in range(6)], "cam": ["a", "b", "c"], "light": ["x", "y", "z"]}
        res = mx.cover(dims, [], target=0)
        self.assertEqual(res["covering_min"], 18)               # 6 x 3, the lower bound
        self.assertEqual(res["coverage"]["fraction"], 1.0)
        self.assertEqual(len(res["rows"]), 18)

    def test_deterministic(self):
        dims = {"env": [f"e{i}" for i in range(6)], "cam": ["a", "b", "c"], "light": ["x", "y", "z"]}
        excl = [{"env": "e0", "light": "x"}]
        self.assertEqual(mx.cover(dims, excl, target=30)["rows"], mx.cover(dims, excl, target=30)["rows"])

    def test_target_grows_the_batch_but_never_beyond_valid(self):
        dims = {"env": [f"e{i}" for i in range(6)], "cam": ["a", "b", "c"], "light": ["x", "y", "z"]}
        res = mx.cover(dims, [], target=30)
        self.assertEqual(len(res["rows"]), 30)
        self.assertEqual(len(set(res["rows"])), 30)               # no duplicates
        self.assertEqual(len(mx.cover(dims, [], target=999)["rows"]), 54)   # 6*3*3 valid

    def test_floor_is_the_covering_minimum_when_target_is_small(self):
        dims = {"env": [f"e{i}" for i in range(6)], "cam": ["a", "b", "c"], "light": ["x", "y", "z"]}
        self.assertEqual(len(mx.cover(dims, [], target=5)["rows"]), 18)

    def test_rows_respect_exclusions(self):
        dims = {"env": ["a", "b", "c"], "cam": ["x", "y", "z"]}
        excl = [{"env": "a", "cam": "x"}, {"env": "b", "cam": "y"}]
        res = mx.cover(dims, excl, target=30)
        self.assertEqual(res["excluded_count"], 2)
        for env, cam in res["rows"]:
            self.assertNotIn((env, cam), {("a", "x"), ("b", "y")})

    def test_dense_exclusions_are_repaired(self):
        """allpairspy alone leaves valid pairs uncovered here (68 of 716 in the original
        investigation); matrix.cover must cover every coverable pair."""
        rnd = random.Random(7)
        dims = {f"d{d}": [f"d{d}v{v}" for v in range(6)] for d in range(7)}
        names = list(dims)
        forb = set()
        while len(forb) < 40:
            i, j = sorted(rnd.sample(range(7), 2))
            forb.add((names[i], f"{names[i]}v{rnd.randrange(6)}", names[j], f"{names[j]}v{rnd.randrange(6)}"))
        excl = [{a: av, b: bv} for a, av, b, bv in forb]
        raw = mx._allpairs_rows(dims, excl)
        valid = mx.enumerate_valid(dims, excl)
        self.assertGreater(len(mx.coverage(raw, valid)["uncovered"]), 0)    # the library alone is not enough
        res = mx.cover(dims, excl, target=0)
        self.assertEqual(mx.coverage(res["rows"], valid)["uncovered"], [])
        self.assertTrue(all(r in set(valid) for r in res["rows"]))

    def test_impossible_pairs_are_reported_not_dropped(self):
        dims = {"env": ["a", "b"], "cam": ["x", "y"]}
        excl = [{"env": "a", "cam": "x"}]                 # the pair (a, x) can never appear
        res = mx.cover(dims, excl, target=0)
        self.assertEqual(res["pairs"]["impossible"], 1)
        self.assertEqual(res["coverage"]["fraction"], 1.0)

    def test_single_dimension(self):
        res = mx.cover({"model": ["m1", "m2", "m3"]}, [], target=30)
        self.assertEqual(len(res["rows"]), 3)

    def test_enumeration_limit(self):
        dims = {f"d{i}": [str(v) for v in range(10)] for i in range(7)}    # 10^7
        with self.assertRaises(mx.PlanError):
            mx.enumerate_valid(dims, [])

    def test_everything_excluded_is_an_error(self):
        with self.assertRaises(mx.PlanError):
            mx.cover({"a": ["1"], "b": ["2"]}, [{"a": "1"}], target=5)


class Coverage(unittest.TestCase):
    def test_coverage_of_a_subset(self):
        dims = {"a": ["1", "2"], "b": ["x", "y"]}
        valid = mx.enumerate_valid(dims, [])
        cov = mx.coverage([("1", "x"), ("2", "y")], valid)
        self.assertEqual((cov["covered"], cov["coverable"]), (2, 4))
        self.assertLess(cov["fraction"], 1.0)


class SpareRowBalanceTests(unittest.TestCase):
    """The spare rows after the covering minimum must spread the values evenly (found on a real plan:
    3 colors x 3 poses x 4 framings x 3 emphases at 30 rows gave one color in 16 rows and the others 7)."""

    def dims(self):
        return {"color": ["a", "b", "c"], "pose": ["p", "q", "r"], "framing": ["f1", "f2", "f3", "f4"],
                "emphasis": ["x", "y", "z"]}

    def test_every_value_is_used_about_equally_often(self):
        import collections
        d = self.dims()
        res = mx.cover(d, [], target=30)
        self.assertEqual(len(res["rows"]), 30)
        self.assertEqual(res["coverage"]["covered"], res["pairs"]["coverable"])           # still every pair
        for i, name in enumerate(d):
            counts = collections.Counter(r[i] for r in res["rows"]).values()
            self.assertLessEqual(max(counts) - min(counts), 1, (name, counts))

    def test_balance_survives_exclusions_and_is_deterministic(self):
        import collections
        d = self.dims()
        excl = [{"color": "a", "pose": "p"}, {"framing": "f4", "emphasis": "z"}]
        r1, r2 = mx.cover(d, excl, target=24), mx.cover(d, excl, target=24)
        self.assertEqual(r1["rows"], r2["rows"])
        self.assertEqual(r1["coverage"]["covered"], r1["pairs"]["coverable"])
        for i, name in enumerate(d):
            counts = collections.Counter(r[i] for r in r1["rows"])
            self.assertEqual(set(counts), set(d[name]), name)                              # every value appears
            self.assertLessEqual(max(counts.values()) - min(counts.values()), 4, (name, counts))


class OnlyInTests(unittest.TestCase):
    """A value scoped to some values of another dimension, instead of hand-written exclude pairs."""
    def plan(self, scope):
        plan = make_plan()
        plan["dimensions"] = {"direction": ["Process", "Ritual", "Graphic"],
                              "layout": [{"value": "hands close", "only_in": scope}, "centred", {"value": "flat grid", "only_in": {"direction": ["Graphic"]}}]}
        return plan

    def test_a_scoped_value_is_combined_only_with_its_directions(self):
        plan = self.plan({"direction": ["Process"]})
        self.assertEqual(mx.validate_plan(plan), [])
        dims = {d: [mx.value_name(v) for v in vs] for d, vs in plan["dimensions"].items()}
        valid = set(mx.enumerate_valid(dims, mx.excludes(plan)))
        self.assertIn(("Process", "hands close"), valid)
        self.assertNotIn(("Ritual", "hands close"), valid)
        self.assertNotIn(("Process", "flat grid"), valid)
        self.assertEqual(len(valid), 5)                      # 9 minus 2 ("hands close") minus 2 ("flat grid")

    def test_only_in_must_name_real_values_of_another_dimension(self):
        for scope, word in (({"direction": ["Nope"]}, "not a value"), ({"colour": ["x"]}, "not another dimension"),
                            ({"layout": ["centred"]}, "not another dimension"), ({"direction": []}, "non-empty"), ([], "must be")):
            errs = " ".join(mx.validate_plan(self.plan(scope)))
            self.assertIn(word, errs, scope)


class ExamplePlanTests(unittest.TestCase):
    def test_the_shipped_direction_plan_is_valid_and_has_no_hand_written_excludes(self):
        plan = mx.load_plan(Path(__file__).resolve().parent.parent / "references" / "examples" / "direction-plan.json")
        self.assertEqual(plan.get("constraints", []), [])
        dims = {d: [mx.value_name(v) for v in vs] for d, vs in plan["dimensions"].items()}
        self.assertEqual(len(mx.enumerate_valid(dims, mx.excludes(plan))), 72)

    def test_a_misspelled_catalog_dimension_gets_a_did_you_mean(self):
        import experiment as ex
        plan = make_plan(dimensions={"lightning": ["a", "b"], "camera": ["x", "y"]})
        self.assertTrue(any("did you mean 'lighting'" in w for w in ex.preview_plan(plan)["warnings"]))


class CoverByTests(unittest.TestCase):
    """A hierarchy dimension (campaign territories) whose values exclude different values of the others."""
    DIMS = {"territory": ["A", "B", "C"], "shot": ["w", "x", "y", "z"], "color": ["red", "blue", "mono"]}
    EXCL = [{"territory": "A", "color": "mono"}, {"territory": "B", "color": "mono"},       # the exclude dicts mx.excludes(plan) returns
            {"territory": "C", "shot": "z"}, {"territory": "C", "shot": "y"}]

    def test_the_plain_design_is_unbalanced_and_the_split_one_is_not(self):
        plain = mx.cover(self.DIMS, self.EXCL, target=6)
        by = mx.cover_by(self.DIMS, self.EXCL, "territory", target=9)
        names = list(self.DIMS)
        counts = [sum(1 for r in by["rows"] if r[names.index("territory")] == t) for t in "ABC"]
        self.assertEqual(counts, [3, 3, 3])
        self.assertEqual(by["by"], "territory")
        self.assertEqual(set(by["strata"]), {"A", "B", "C"})
        self.assertTrue(plain["rows"])                               # the plain design still works without batch_by (it is not
                                                                     # claimed unbalanced here: the measured 8 / 16 / 7 case needed the full plan)

    def test_every_row_is_valid_and_a_share_below_the_minimum_reports_it(self):
        by = mx.cover_by(self.DIMS, self.EXCL, "territory", target=6)       # 2 each: less than any stratum needs
        valid = set(mx.enumerate_valid(self.DIMS, self.EXCL))
        self.assertTrue(set(by["rows"]) <= valid)
        self.assertEqual(len(by["rows"]), 6)
        self.assertTrue(any(not x["complete"] for x in by["strata"].values()))

    def test_a_large_share_covers_every_pair_inside_each_stratum(self):
        by = mx.cover_by(self.DIMS, self.EXCL, "territory", target=60)
        self.assertTrue(all(x["complete"] for x in by["strata"].values()))
        self.assertEqual(by["coverage"]["covered"], by["pairs"]["coverable"])

    def test_batch_by_must_name_a_dimension_with_two_values(self):
        with self.assertRaises(mx.PlanError):
            mx.cover_by(self.DIMS, self.EXCL, "nope", target=9)
        plan = make_plan(batch_by="camera")
        self.assertEqual(mx.validate_plan(plan), [])
        plan["batch_by"] = "missing"
        self.assertTrue(any("batch_by" in e for e in mx.validate_plan(plan)))
        plan["batch_by"] = 3
        self.assertTrue(any("batch_by" in e for e in mx.validate_plan(plan)))

    def test_cover_for_plan_uses_batch_by(self):
        plan = make_plan(batch_by="camera")
        res = mx.cover_for_plan(plan, mx.dimensions(plan), 9)
        self.assertEqual(res["by"], "camera")
        self.assertNotIn("by", mx.cover_for_plan(make_plan(), mx.dimensions(make_plan()), 9))


class WildcardFlagTests(unittest.TestCase):
    def plan_with(self, *flags):
        plan = make_plan()
        plan["dimensions"]["environment"] = [
            {"value": "a", "fragment": "A"}, {"value": "b", "fragment": "B", **flags[0]} if flags else {"value": "b", "fragment": "B"}, "c"]
        return plan

    def test_one_wildcard_that_relaxes_a_rule_is_valid(self):
        plan = self.plan_with({"wildcard": True, "relaxes": ["the off-white foundation"]})
        self.assertEqual(mx.validate_plan(plan), [])

    def test_relaxes_without_wildcard_is_refused(self):
        plan = self.plan_with({"relaxes": ["a rule"]})
        self.assertTrue(any("must also be marked wildcard" in e for e in mx.validate_plan(plan)))

    def test_more_than_one_flagged_value_is_refused(self):
        plan = self.plan_with({"wildcard": True, "relaxes": ["x"]})
        plan["dimensions"]["camera"] = [{"value": "x", "fragment": "X", "wildcard": True}, "y", "z"]
        self.assertTrue(any("at most one value" in e for e in mx.validate_plan(plan)))

    def test_bad_flag_types_are_refused(self):
        for flags in ({"relaxes": "off-white"}, {"relaxes": []}, {"wildcard": "yes"}):
            plan = self.plan_with(flags)
            self.assertTrue(mx.validate_plan(plan), flags)

    def test_a_batch_by_dimension_without_traits_gets_a_dry_run_warning(self):
        import experiment as ex
        plan = make_plan(batch_by="camera")
        self.assertTrue(any("no `traits`" in w for w in ex.preview_plan(plan)["warnings"]))

    def test_the_dry_run_prints_the_wildcard_and_the_prompt_ignores_the_flags(self):
        import compile as cp
        import experiment as ex
        plan = self.plan_with({"wildcard": True, "relaxes": ["the off-white foundation"]})
        summary = ex.preview_plan(plan)
        self.assertEqual(summary["flags"], [("environment", "b", True, ["the off-white foundation"])])
        self.assertIn("relaxes: the off-white foundation", ex.format_plan_summary(summary))
        text = cp.compile_prompt(plan, {"environment": "b", "camera": "x"}, "text")["prompt"]
        self.assertNotIn("wildcard", text)
        self.assertNotIn("off-white", text)


class TraitsTests(unittest.TestCase):
    def tr(self, **over):
        base = {"ground": "off-white paper", "medium": "collage", "layout": "runner centre, headline left", "type": "giant stacked headline",
                "density": "layered", "palette": "cobalt, black, off-white"}
        base.update(over)
        return base

    def plan(self, *traits):
        plan = make_plan()
        plan["dimensions"]["environment"] = [{"value": n, "fragment": n, "traits": t} for n, t in zip("abc", traits)]
        return plan

    def test_distinct_directions_pass(self):
        plan = self.plan(self.tr(),
                         self.tr(ground="black night", medium="photograph", layout="low angle, runner right", palette="yellow, black"),
                         self.tr(ground="pale grey studio", medium="photograph", layout="wide negative space", type="small label", density="minimal", palette="red, grey"))
        self.assertEqual(mx.validate_plan(plan), [])

    def test_two_directions_that_differ_in_two_traits_are_refused(self):
        plan = self.plan(self.tr(), self.tr(ground="concrete wall", palette="blue, concrete"),
                         self.tr(ground="black night", medium="photograph", layout="low angle", type="small", density="minimal", palette="yellow"))
        errs = mx.validate_plan(plan)
        self.assertTrue(any("differ in only 2 of the six traits" in e for e in errs), errs)

    def test_rewording_does_not_make_a_duplicate_look_different(self):
        plan = self.plan(self.tr(), self.tr(ground="warm off-white paper stock", medium="paper collage", layout="runner centre with headline left",
                                            type="giant stacked headline in black", density="layered with marks", palette="cobalt, black and off-white"),
                         self.tr(ground="black night", medium="photograph", layout="low angle", type="small label", density="minimal", palette="yellow"))
        errs = mx.validate_plan(plan)
        self.assertTrue(any("differ in only" in e for e in errs), errs)

    def test_a_palette_shared_by_three_directions_is_refused(self):
        a = self.tr(); b = self.tr(ground="black", medium="photo", layout="low", type="small"); c = self.tr(ground="grey", medium="print", layout="wide", density="minimal")
        self.assertTrue(any("share the palette" in e for e in mx.validate_plan(self.plan(a, b, c))))

    def test_traits_must_be_complete_and_on_every_value(self):
        plan = self.plan(self.tr(), self.tr(ground="x", medium="y", layout="z"), self.tr(ground="p", medium="q", layout="r"))
        del plan["dimensions"]["environment"][1]["traits"]["palette"]
        self.assertTrue(any("exactly these non-empty text keys" in e for e in mx.validate_plan(plan)))
        plan = self.plan(self.tr(), self.tr(ground="x", medium="y", layout="z"), self.tr(ground="p", medium="q", layout="r"))
        del plan["dimensions"]["environment"][2]["traits"]
        self.assertTrue(any("every value has `traits` or none does" in e for e in mx.validate_plan(plan)))

    def test_plans_without_traits_are_unchanged_and_the_dry_run_prints_them(self):
        import experiment as ex
        self.assertEqual(mx.validate_plan(make_plan()), [])
        plan = self.plan(self.tr(), self.tr(ground="black night", medium="photograph", layout="low angle", palette="yellow"),
                         self.tr(ground="grey studio", medium="photograph", layout="wide", type="small", density="minimal", palette="red"))
        text = ex.format_plan_summary(ex.preview_plan(plan))
        self.assertIn("Look (environment):", text)
        self.assertIn("black night | photograph", text)


class ReviewFixTests(unittest.TestCase):
    """Regressions for the review of the matrix and checks code (2026-10-08)."""

    def tr(self, **over):
        base = {"ground": "off-white paper", "medium": "collage", "layout": "runner centre, headline left", "type": "giant stacked headline",
                "density": "layered", "palette": "cobalt, black, off-white"}
        base.update(over)
        return base

    def plan(self, *traits):
        plan = make_plan()
        plan["dimensions"]["environment"] = [{"value": n, "fragment": n, "traits": t} for n, t in zip("abcdefg", traits)]
        return plan

    def distinct(self, i, palette):
        # six directions' looks differ in every trait except the palette we test
        return {"ground": f"ground{i} alpha{i}", "medium": f"medium{i} beta{i}", "layout": f"layout{i} gamma{i}", "type": f"type{i} delta{i}",
                "density": f"density{i} eps{i}", "palette": palette}

    def test_three_different_palettes_that_overlap_pairwise_are_accepted(self):
        for pals in (("red blue", "red green", "blue green"), ("cream black", "cream coral", "black coral")):
            plan = self.plan(*[self.distinct(i, p) for i, p in enumerate(pals)])
            self.assertEqual(mx.validate_plan(plan), [], pals)

    def test_a_chain_of_similar_palettes_is_still_caught_when_three_are_alike(self):
        plan = self.plan(*[self.distinct(i, p) for i, p in enumerate(("teal navy", "teal navy", "teal navy", "ruby gold"))])
        self.assertTrue(any("share the palette" in e for e in mx.validate_plan(plan)))

    def test_the_shared_foundation_words_are_ignored_from_four_directions_up(self):
        pals = ("black white cobalt", "black white yellow", "black white red", "black white green")
        plan = self.plan(*[self.distinct(i, p) for i, p in enumerate(pals)])
        self.assertEqual(mx.validate_plan(plan), [])

    def test_size_and_number_words_count_and_other_scripts_are_compared(self):
        self.assertFalse(mx.traits_alike("large serif headline", "small serif headline"))
        self.assertFalse(mx.traits_alike("one column", "two column"))
        self.assertFalse(mx.traits_alike("日本語 ポスター", "中文 海报"))
        self.assertTrue(mx.traits_alike("the", "the"))
        self.assertFalse(mx.traits_alike("the", "a"))
        self.assertTrue(mx.traits_alike("off-white paper", "warm off-white paper stock"))      # rewording is not a difference

    def test_malformed_plans_return_errors_instead_of_crashing(self):
        plan = make_plan()
        plan["dimensions"] = {"a": 5}
        plan["batch_by"] = "a"
        self.assertTrue(mx.validate_plan(plan))
        plan = make_plan()
        plan["dimensions"]["camera"] = [{"fragment": "no value key"}, "y", "z"]
        plan["constraints"] = [{"exclude": {"camera": "y"}}]
        self.assertTrue(mx.validate_plan(plan))                      # reported, not a KeyError

    def test_a_value_with_no_valid_rows_is_reported_and_its_share_goes_to_the_others(self):
        dims = {"t": ["A", "B", "C"], "x": ["1", "2"], "y": ["p", "q"]}
        excl = [{"t": "B", "x": "1"}, {"t": "B", "x": "2"}]
        res = mx.cover_by(dims, excl, "t", target=8)
        self.assertEqual(res["strata"]["B"]["rows"], 0)
        self.assertTrue(res["strata"]["B"]["empty"])
        self.assertFalse(res["strata"]["B"]["complete"])
        self.assertEqual(len(res["rows"]), 8)                         # B's share went to A and C: 4 + 4

    def test_a_zero_target_gives_each_value_its_own_covering_design(self):
        dims = {"t": ["A", "B"], "x": ["1", "2", "3"], "y": ["p", "q", "r"]}
        res = mx.cover_by(dims, [], "t", target=0)
        self.assertGreaterEqual(len(res["rows"]), 18)                  # at least the two 3x3 designs
        self.assertTrue(all(x["complete"] for x in res["strata"].values()))

    def test_the_share_rounds_up_so_the_batch_can_exceed_the_target(self):
        dims = {"t": ["A", "B", "C", "D"], "x": [str(i) for i in range(1, 9)], "y": ["p", "q", "r", "s"]}
        self.assertEqual(len(mx.cover_by(dims, [], "t", target=30)["rows"]), 32)            # 4 x ceil(30 / 4)


if __name__ == "__main__":
    unittest.main()
