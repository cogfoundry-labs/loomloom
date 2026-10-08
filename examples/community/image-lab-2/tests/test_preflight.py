"""Offline tests for scripts/preflight.py and scripts/experiment.py (no gateway calls)."""
import base64
import copy
import json
import shutil
import sys
import tempfile
import unittest
from pathlib import Path

HERE = Path(__file__).resolve().parent
sys.path.insert(0, str(HERE.parent / "scripts"))
import experiment as ex  # noqa: E402
import ledger as lg  # noqa: E402
import preflight as pf  # noqa: E402
import workbook as wbk  # noqa: E402

PNG = base64.b64decode(
    "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg==")
PLAN = json.loads((HERE / "fixtures" / "plan-mighty.json").read_text(encoding="utf-8"))
NB = "google/gemini-2.5-flash-image"
SB = "openai/gpt-image-2.5-sunburst"
SEED = "bytedance/doubao-seedream-5-0-pro"

pf.il.load_price_observations = lambda: {}   # tests must not depend on a local observed-price cache
pf.REF_SUPPORT_FILE = HERE / "fixtures" / "reference-support.json"   # ...nor on the live reference-support data
ADVISOR = pf.Advisor(PLAN["intent"])        # real catalog + Arena data, built once


def make_experiment(tmp: Path, plan: dict, target=30, with_ref=True) -> Path:
    src = tmp / "src"
    (src / "refs").mkdir(parents=True)
    if with_ref:
        (src / "refs" / "mighty-product.png").write_bytes(PNG)
    (src / "plan.json").write_text(json.dumps(plan), encoding="utf-8")
    out = tmp / "exp"
    ex.build_experiment(src / "plan.json", out, target=target)
    return out


class Base(unittest.TestCase):
    def setUp(self):
        self.tmp = Path(tempfile.mkdtemp())

    def tearDown(self):
        shutil.rmtree(self.tmp, ignore_errors=True)

    def build(self, plan=None, **kw):
        self.exp = make_experiment(self.tmp, plan or PLAN, **kw)
        return self.exp

    def ledger(self):
        return lg.load(self.exp / "ledger.json")

    def edit_ledger(self, fn):
        led = self.ledger()
        fn(led)
        lg.save(self.exp / "ledger.json", led)
        (self.exp / "experiment.xlsx").unlink()          # ledger is the source in these tests

    def run_pf(self, **kw):
        return pf.run_preflight(self.exp, advisor=ADVISOR, **kw)


class Plan(Base):
    def test_plan_writes_ledger_workbook_and_ticks_the_recommended_rows(self):
        self.build()
        led = self.ledger()
        self.assertEqual(len(led["rows"]), 9)
        self.assertTrue(all(r["selected"] for r in led["rows"]))
        self.assertTrue((self.exp / "experiment.xlsx").exists())
        self.assertTrue((self.exp / "refs" / "mighty-product.png").exists())   # copied next to the experiment
        self.assertEqual({r["status"] for r in led["rows"]}, {"Draft"})

    def test_target_below_the_valid_count_ticks_only_the_batch(self):
        plan = copy.deepcopy(PLAN)
        plan["dimensions"]["mood"] = ["calm", "bold", "warm"]
        self.build(plan, target=12)
        led = self.ledger()
        self.assertEqual(len(led["rows"]), 27)                     # all valid combinations are in the workbook
        self.assertEqual(sum(r["selected"] for r in led["rows"]), 12)

    def test_spread_makes_model_an_explicit_dimension(self):
        plan = copy.deepcopy(PLAN)
        plan["model_strategy"] = "spread"
        self.build(plan, with_ref=False)
        led = self.ledger()
        self.assertEqual(led["experiment"]["dimensions"], ["camera", "lighting"])
        models = {r["model"] for r in led["rows"]}
        self.assertEqual(len(models), 2)
        for m in models:
            cams = {r["params"]["camera"] for r in led["rows"] if r["model"] == m}
            self.assertEqual(cams, {"eye-level front", "three-quarter high", "low angle"})   # each model covered


class Preflight(Base):
    def test_ready_rows_snapshot_and_fingerprint(self):
        self.build()
        rep = self.run_pf()
        self.assertEqual((rep["ready"], rep["selected"], len(rep["issues"])), (9, 9, 0))
        fp = rep["fingerprint"]
        snap = json.loads((self.exp / "snapshots" / f"{fp}.json").read_text(encoding="utf-8"))
        self.assertEqual(snap["fingerprint"], fp)
        row = snap["rows"][0]
        self.assertEqual(row["mode"], "reference")
        self.assertEqual(row["references"][0]["sha256"], pf.sha256_file(self.exp / "refs" / "mighty-product.png"))
        self.assertEqual(pf.snapshot_fingerprint(snap), fp)       # the fingerprint covers the whole snapshot body
        self.assertEqual(self.run_pf()["fingerprint"], fp)          # deterministic

    def test_reference_rows_prefer_the_verified_model_and_it_is_priced(self):
        self.build()
        rep = self.run_pf()
        self.assertEqual(rep["models"], [NB])
        self.assertAlmostEqual(rep["known_usd"], 9 * 0.039, places=6)
        self.assertEqual(rep["unverified_rows"], [])

    def test_documented_reference_model_is_allowed_but_unverified_and_unpriced(self):
        self.build()
        self.edit_ledger(lambda led: [r.update(model=SB) for r in led["rows"]])
        rep = self.run_pf()
        self.assertEqual(rep["ready"], 9)
        self.assertEqual(len(rep["unverified_rows"]), 9)
        self.assertEqual(rep["known_usd"], 0)                       # never a made-up number
        self.assertTrue(any("documented" in w for w in rep["warnings"]))
        self.assertIn("price unverified", rep["text"])

    def test_text_mode_prices_from_the_catalog_one_image_per_row(self):
        text_plan = copy.deepcopy(PLAN)
        text_plan["references"] = []
        self.build(text_plan, with_ref=False)
        self.edit_ledger(lambda led: [r.update(model=SB) for r in led["rows"]])
        rep = self.run_pf()
        r1 = next(r for r in rep["snapshot"]["rows"] if r["id"] == "r001")
        self.assertEqual((r1["mode"], r1["model"], r1["qty"]), ("text", SB, 1))
        self.assertAlmostEqual(r1["est_usd"], 0.0069, places=5)
        self.assertEqual(rep["images"], rep["ready"])                  # images = ticked rows

    def test_invalid_rows_become_draft_and_are_not_in_the_snapshot(self):
        self.build()

        def bad(led):
            led["rows"][0]["take"] = 0
            led["rows"][1]["params"]["lighting"] = ""
            led["rows"][2]["reference"] = "nope"
        self.edit_ledger(bad)
        rep = self.run_pf()
        reasons = {i["id"]: " ".join(i["problems"]) for i in rep["issues"]}
        self.assertIn("Take must be a whole number from 1", reasons["r001"])
        self.assertIn("missing value for lighting", reasons["r002"])
        self.assertIn("unknown reference", reasons["r003"])
        self.assertEqual(rep["ready"], 6)
        ids = {r["id"] for r in rep["snapshot"]["rows"]}
        self.assertTrue({"r001", "r002", "r003"}.isdisjoint(ids))
        self.assertEqual({r["id"]: r["status"] for r in self.ledger()["rows"]}["r001"], "Draft")

    def test_missing_or_unreadable_reference_file(self):
        self.build()
        (self.exp / "refs" / "mighty-product.png").write_bytes(b"not an image")
        self.assertIn("not a PNG, JPEG or WebP image", self.run_pf()["issues"][0]["problems"][0])
        (self.exp / "refs" / "mighty-product.png").unlink()
        self.assertIn("not found", self.run_pf()["issues"][0]["problems"][0])

    def test_unselected_removed_and_unknown_rows_are_not_included(self):
        self.build()

        def mark(led):
            led["rows"][0]["selected"] = False
            led["rows"][1]["status"] = "Removed"
            led["rows"][2]["status"] = "Unknown"
        self.edit_ledger(mark)
        rep = self.run_pf()
        self.assertEqual(rep["ready"], 6)
        self.assertTrue(any("r003" in w and "never retried automatically" in w for w in rep["warnings"]))

    def test_unreliable_value_is_reported_in_reference_mode_only(self):
        self.build()
        self.assertTrue(any('"low angle" is unreliable' in i for i in self.run_pf()["info"]))
        text_plan = copy.deepcopy(PLAN)
        text_plan["references"] = []
        shutil.rmtree(self.tmp / "exp")
        shutil.rmtree(self.tmp / "src")
        self.exp = make_experiment(self.tmp, text_plan, with_ref=False)
        self.assertFalse(any("unreliable" in i for i in self.run_pf()["info"]))

    def test_custom_value_warning_and_generic_wording_in_the_snapshot(self):
        self.build()

        def custom(led):
            led["rows"][0]["params"]["lighting"] = "rainy night"
        self.edit_ledger(custom)
        rep = self.run_pf()
        self.assertTrue(any('custom value "rainy night"' in w for w in rep["warnings"]))
        r1 = next(r for r in rep["snapshot"]["rows"] if r["id"] == "r001")
        self.assertIn("rainy night as the lighting", r1["prompt"])

    def test_changing_a_value_changes_the_fingerprint(self):
        self.build()
        before = self.run_pf()["fingerprint"]

        def change(led):
            led["rows"][0]["params"]["lighting"] = "golden hour"
            led["rows"][0]["params"]["camera"] = "eye-level front"
        self.edit_ledger(change)
        self.assertNotEqual(self.run_pf()["fingerprint"], before)

    def test_aspect_ratio_that_the_model_cannot_honor_is_reported(self):
        plan = copy.deepcopy(PLAN)
        plan["references"] = []
        plan["dimensions"]["aspect"] = ["4:5"]
        self.build(plan, with_ref=False, target=9)
        self.edit_ledger(lambda led: [r.update(model=SEED) for r in led["rows"]])
        rep = self.run_pf()
        self.assertTrue(any("requested 4:5 ->" in i for i in rep["info"]))
        row = rep["snapshot"]["rows"][0]
        self.assertTrue(row["size"])                                  # a real size token was chosen

    def test_unknown_model_override_is_an_issue(self):
        self.build()
        self.edit_ledger(lambda led: led["rows"][0].update(model="nope/model"))
        self.assertIn("not in the catalog", self.run_pf()["issues"][0]["problems"][0])

    def test_estimate_only_mode_touches_no_file(self):
        self.build()
        before = sorted(p.name for p in self.exp.rglob("*") if p.is_file())
        rep = pf.run_preflight(self.exp, advisor=ADVISOR, write=False)
        self.assertEqual(sorted(p.name for p in self.exp.rglob("*") if p.is_file()), before)
        self.assertEqual(rep["ready"], 9)


class EditedWorkbook(Base):
    def test_user_edits_in_excel_drive_the_snapshot(self):
        self.build()
        import openpyxl
        path = self.exp / "experiment.xlsx"
        book = openpyxl.load_workbook(path)
        ws = book["Experiment"]
        ws["B2"] = False                      # untick r001
        ws["F3"] = 2                          # r002 Take 2
        ws["C4"] = "low angle"                # r003 camera
        book.save(path)
        rep = pf.run_preflight(self.exp, advisor=ADVISOR)
        self.assertEqual(rep["ready"], 8)
        rows = {r["id"]: r for r in rep["snapshot"]["rows"]}
        self.assertNotIn("r001", rows)
        self.assertEqual(rep["snapshot"]["rows"][0]["qty"], 1)         # one image per row, whatever Take says
        self.assertEqual(self.ledger()["rows"][1]["take"], 2)
        self.assertIn("Low camera position at counter level", rows["r003"]["prompt"])   # reference wording
        self.assertEqual(rep["images"], 8)                                              # one image per ticked row
        self.assertTrue(rep["workbook_saved"])


class Check(unittest.TestCase):
    def test_data_files_are_consistent(self):
        self.assertEqual(ex.check(HERE / "fixtures" / "plan-mighty.json"), [])

    def test_bad_plan_is_reported(self):
        tmp = Path(tempfile.mkdtemp())
        try:
            (tmp / "p.json").write_text(json.dumps({"schema_version": 1}), encoding="utf-8")
            self.assertTrue(any("plan.json is invalid" in p for p in ex.check(tmp / "p.json")))
        finally:
            shutil.rmtree(tmp, ignore_errors=True)


class Grouping(unittest.TestCase):
    def test_group_messages(self):
        msgs = ["r001: x", "r002: x", "plain", "r003: y"] + [f"r{n:03d}: z" for n in range(10, 18)]
        out = pf.group_messages(msgs)
        self.assertIn("plain", out)
        self.assertIn("x: r001, r002", out)
        self.assertIn("y: r003", out)
        self.assertTrue(any(o.startswith("z: r010, r011, r012, r013, r014, ... (8 rows)") for o in out))


class Refresh(Base):
    def test_refresh_merges_edits_regenerates_and_keeps_prev(self):
        self.build()
        import openpyxl
        path = self.exp / "experiment.xlsx"
        book = openpyxl.load_workbook(path)
        book["Experiment"]["F2"] = 3
        book.save(path)
        res = ex.refresh(self.exp)
        self.assertTrue((self.exp / "experiment.prev.xlsx").exists())
        again = wbk.read_workbook(self.exp / "experiment.xlsx", ["camera", "lighting"])
        self.assertEqual(again["rows"][0]["take"], 3)
        self.assertEqual(res["warnings"], [])
        self.assertEqual(self.ledger()["rows"][0]["take"], 3)


if __name__ == "__main__":
    unittest.main()
