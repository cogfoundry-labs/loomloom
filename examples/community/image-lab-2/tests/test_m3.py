"""Offline tests for M3: reference handling, per-model reference states, person notice."""
import copy
import json
import os
import shutil
import subprocess
import sys
import unittest
from pathlib import Path

HERE = Path(__file__).resolve().parent
SCRIPTS = HERE.parent / "scripts"
sys.path.insert(0, str(SCRIPTS))
sys.path.insert(0, str(HERE))
import compile as cp  # noqa: E402
import experiment as ex  # noqa: E402
import generate as gen  # noqa: E402
import ledger as lg  # noqa: E402
import matrix as mx  # noqa: E402
import preflight as pf  # noqa: E402
from test_generate import FakeGateway, quiet  # noqa: E402
from test_preflight import ADVISOR, Base, NB, PLAN, PNG, SEED, make_experiment  # noqa: E402

PERSON_PLAN = copy.deepcopy(PLAN)
PERSON_PLAN["references"] = [{"id": "ref1", "file": "refs/mighty-product.png", "role": "person",
                              "contains_person": True}]


def ledger_select(exp, n):
    led = lg.load(exp / "ledger.json")
    for i, r in enumerate(led["rows"]):
        r["selected"] = i < n
    lg.save(exp / "ledger.json", led)
    if (exp / "experiment.xlsx").exists():
        (exp / "experiment.xlsx").unlink()


class PersonNoticeTests(Base):
    def make_plan_dir(self, plan):
        src = self.tmp / "src"
        (src / "refs").mkdir(parents=True)
        (src / "refs" / "mighty-product.png").write_bytes(PNG)
        (src / "plan.json").write_text(json.dumps(plan), encoding="utf-8")
        return src

    def acknowledge(self, src, exp=None):
        args = [sys.executable, str(SCRIPTS / "image.py"), "acknowledge-person", "--plan", str(src / "plan.json")]
        if exp:
            args += ["--dir", str(exp)]
        return subprocess.run(args, capture_output=True, text=True, encoding="utf-8",
                              env={**os.environ, "PYTHONUTF8": "1"})

    def test_plan_with_a_person_is_refused_until_acknowledged(self):
        src = self.make_plan_dir(PERSON_PLAN)
        with self.assertRaises(mx.PlanError) as cm:
            ex.build_experiment(src / "plan.json", self.tmp / "exp")
        self.assertIn("photo of a person", str(cm.exception))
        self.assertIn("provider", str(cm.exception))
        self.assertFalse((self.tmp / "exp" / "experiment.xlsx").exists())
        r = self.acknowledge(src)
        self.assertEqual(r.returncode, 0, r.stderr)
        plan = json.loads((src / "plan.json").read_text(encoding="utf-8"))
        self.assertTrue(plan["consent_acknowledged"])
        ex.build_experiment(src / "plan.json", self.tmp / "exp")
        self.assertTrue((self.tmp / "exp" / "experiment.xlsx").exists())

    def test_acknowledge_is_refused_when_nobody_is_pictured(self):
        src = self.make_plan_dir(PLAN)
        r = self.acknowledge(src)
        self.assertEqual(r.returncode, 2)
        self.assertIsNone(json.loads((src / "plan.json").read_text(encoding="utf-8")).get("consent_acknowledged"))

    def test_preflight_and_run_refuse_if_consent_is_missing(self):
        src = self.make_plan_dir(PERSON_PLAN)
        self.acknowledge(src)
        exp = self.tmp / "exp"
        ex.build_experiment(src / "plan.json", exp)
        ledger_select(exp, 2)
        rep = pf.run_preflight(exp, advisor=ADVISOR)
        self.assertEqual(rep["ready"], 2)
        fp = rep["fingerprint"]
        plan = json.loads((exp / "plan.json").read_text(encoding="utf-8"))
        plan["consent_acknowledged"] = None                       # consent withdrawn / removed after approval
        (exp / "plan.json").write_text(json.dumps(plan), encoding="utf-8")
        gw = FakeGateway()
        with self.assertRaises(gen.RunRefused) as cm:
            gen.run_batch(exp, fp, gateway=gw, log=quiet, record_prices=False)
        self.assertIn("person", str(cm.exception))
        self.assertEqual(gw.n, 0)
        rep2 = pf.run_preflight(exp, advisor=ADVISOR)
        self.assertEqual(rep2["ready"], 0)
        self.assertTrue(any("consent" in p for i in rep2["issues"] for p in i["problems"]))

    def test_person_experiment_page_is_never_built(self):
        src = self.make_plan_dir(PERSON_PLAN)
        self.acknowledge(src)
        exp = self.tmp / "exp"
        ex.build_experiment(src / "plan.json", exp)
        ledger_select(exp, 1)
        fp = pf.run_preflight(exp, advisor=ADVISOR)["fingerprint"]
        gen.run_batch(exp, fp, gateway=FakeGateway(), poll_seconds=0.01, record_prices=False, log=quiet)
        r = subprocess.run([sys.executable, str(SCRIPTS / "build-exploration-page.py"), "--session", str(exp),
                            "--title", "t", "--subject", "s", "--invocation", "i"],
                           capture_output=True, text=True, encoding="utf-8", env={**os.environ, "PYTHONUTF8": "1"})
        self.assertNotEqual(r.returncode, 0)
        self.assertIn("photo of a person", r.stderr)
        self.assertEqual(list(exp.glob("*/index.html")), [])

    def test_person_snapshot_marks_the_reference_and_prompt_keeps_likeness(self):
        src = self.make_plan_dir(PERSON_PLAN)
        self.acknowledge(src)
        exp = self.tmp / "exp"
        ex.build_experiment(src / "plan.json", exp)
        ledger_select(exp, 1)
        snap = pf.run_preflight(exp, advisor=ADVISOR)["snapshot"]
        self.assertTrue(snap["rows"][0]["references"][0]["person"])


class ReferenceHandlingTests(Base):
    TWO_REFS = copy.deepcopy(PLAN)
    TWO_REFS["references"] = [
        {"id": "ref1", "file": "refs/mighty-product.png", "role": "product", "contains_person": False},
        {"id": "ref2", "file": "refs/mighty-product.png", "role": "style", "contains_person": False}]
    TWO_REFS.pop("prompt")                                         # use the default, role-specific wording

    def test_role_specific_wording(self):
        p = {"brief": "b", "references": [], "dimensions": {}}
        self.assertIn("identify the product", cp.default_reference_prefix(p, "product"))
        self.assertIn("likeness", cp.default_reference_prefix(p, "person"))
        self.assertIn("style guide", cp.default_reference_prefix(p, "style"))
        self.assertIn("main subject", cp.default_reference_prefix(p, "general"))

    def test_reference_choice_per_row(self):
        two = self.TWO_REFS
        self.assertEqual(cp.resolve_reference(PLAN, {"reference": None}), (["ref1"], None))
        self.assertEqual(cp.resolve_reference(PLAN, {"reference": "none"}), ([], None))
        self.assertEqual(cp.resolve_reference(two, {"reference": "ref2"}), (["ref2"], None))
        ids, problem = cp.resolve_reference(two, {"reference": None})
        self.assertEqual(ids, [])
        self.assertIn("choose a reference", problem)
        self.assertIn("one reference per row", cp.resolve_reference(two, {"reference": "ref1, ref2"})[1])

    def test_none_makes_a_text_only_row_and_ambiguous_rows_are_issues(self):
        exp = make_experiment(self.tmp, self.TWO_REFS)
        led = lg.load(exp / "ledger.json")
        for i, r in enumerate(led["rows"]):
            r["selected"] = i < 3
            led["rows"][0]["reference"] = "none"
        led["rows"][1]["reference"] = "ref2"
        led["rows"][2]["reference"] = None                         # ambiguous with two references
        lg.save(exp / "ledger.json", led)
        (exp / "experiment.xlsx").unlink()
        rep = pf.run_preflight(exp, advisor=ADVISOR)
        modes = {r["id"]: r["mode"] for r in rep["snapshot"]["rows"]}
        self.assertEqual(modes[led["rows"][0]["id"]], "text")
        self.assertEqual(modes[led["rows"][1]["id"]], "reference")
        self.assertEqual([i["id"] for i in rep["issues"]], [led["rows"][2]["id"]])
        self.assertIn("style guide", rep["prompts"][led["rows"][1]["id"]])

    def test_duplicate_rows_warn_and_the_arena_proxy_is_disclosed(self):
        exp = make_experiment(self.tmp, PLAN)
        led = lg.load(exp / "ledger.json")
        led["rows"][1]["params"] = dict(led["rows"][0]["params"])
        for i, r in enumerate(led["rows"]):
            r["selected"] = i < 2
        lg.save(exp / "ledger.json", led)
        (exp / "experiment.xlsx").unlink()
        rep = pf.run_preflight(exp, advisor=ADVISOR)
        self.assertTrue(any("same values, model, reference and Take" in w for w in rep["warnings"]))
        self.assertTrue(any("proxy" in i for i in rep["info"]))

    def test_array_models_get_the_image_as_a_list(self):
        srow = {"id": "r001", "model": SEED, "prompt": "p", "size": None, "aspect_ratio": None,
                "references": [{"id": "ref1", "file": "refs/x.png"}]}
        d = self.tmp
        (d / "refs").mkdir()
        (d / "refs" / "x.png").write_bytes(PNG)
        body = gen.build_request(srow, pf.il.load_model_catalog(), d)
        self.assertIsInstance(body["image"], list)
        self.assertTrue(body["image"][0].startswith("data:image/png;base64,"))
        body2 = gen.build_request({**srow, "model": NB}, pf.il.load_model_catalog(), d)
        self.assertIsInstance(body2["image"], str)


JPEG = bytes.fromhex("ffd8ffc0001108001000200301220002110103110 1ffd9".replace(" ", ""))   # a 32x16 JPEG header


class IdentityTests(Base):
    def support(self):
        sup = json.loads(pf.REF_SUPPORT_FILE.read_text(encoding="utf-8"))["models"]
        sup[NB]["identity"] = {"result": "changed", "note": "turned the bottle into a tube"}
        sup["openai/gpt-image-2.5-sunburst"]["state"] = "verified"
        sup["openai/gpt-image-2.5-sunburst"]["identity"] = {"result": "kept"}
        return sup

    def test_models_that_altered_the_product_are_not_auto_picked_but_can_be_named(self):
        exp = make_experiment(self.tmp, PLAN)
        ledger_select(exp, 2)
        led = lg.load(exp / "ledger.json")
        led["rows"][1]["model"] = NB                                  # the user names it explicitly
        lg.save(exp / "ledger.json", led)
        rep = pf.run_preflight(exp, advisor=ADVISOR, ref_support=self.support())
        by_id = {r["id"]: r["model"] for r in rep["snapshot"]["rows"]}
        self.assertEqual(by_id[led["rows"][0]["id"]], "openai/gpt-image-2.5-sunburst")     # auto-pick avoids NB
        self.assertEqual(by_id[led["rows"][1]["id"]], NB)                                   # but an explicit choice stands
        self.assertTrue(any("altered the product" in w for w in rep["warnings"]))

    def test_jpeg_downloads_keep_their_real_extension_and_size(self):
        import image as il
        self.assertEqual(il._image_kind(JPEG[:12]), "jpg")
        f = self.tmp / "x.jpg"
        f.write_bytes(JPEG)
        self.assertEqual(il._png_size(str(f)), "32x16")
        self.assertIsNone(il._image_kind(b"<html>error"))

    def test_recover_refetches_billed_images_without_a_generation_call(self):
        out = self.tmp / "q"
        q = ex.build_quick("a cat", "e-commerce product photo", 1, out)

        class BrokenDownload(FakeGateway):
            def download(self, url, dest):
                raise OSError("tunnel connection failed")

        gen.run_batch(out, q["fingerprint"], gateway=BrokenDownload(), poll_seconds=0.01, record_prices=False, log=quiet)
        led = lg.load(out / "ledger.json")
        self.assertEqual(led["attempts"][0]["status"], "Failed")
        self.assertTrue(led["attempts"][0]["url"])
        self.assertEqual(led["rows"][0]["status"], "Failed")

        class Jpeg(FakeGateway):
            def download(self, url, dest):
                Path(dest).write_bytes(JPEG)

        gw = Jpeg()
        res = gen.recover_downloads(out, gateway=gw, log=quiet, record_prices=False)
        self.assertEqual(res["recovered"], ["r001"])
        self.assertEqual(gw.n, 0)                                      # no new submission, no new spend
        led = lg.load(out / "ledger.json")
        self.assertEqual(led["attempts"][0]["status"], "Completed")
        self.assertTrue(led["attempts"][0]["file"].endswith(".jpg"))
        self.assertEqual(led["attempts"][0]["size_actual"], "32x16")
        self.assertEqual(led["rows"][0]["status"], "Completed")
        self.assertTrue((out / led["attempts"][0]["file"]).exists())


class PromotionTests(Base):
    def setUp(self):
        super().setUp()
        self.support = self.tmp / "reference-support.json"
        shutil.copy(pf.REF_SUPPORT_FILE, self.support)

    def test_promote_marks_a_documented_model_verified_with_its_price(self):
        self.assertTrue(pf.promote_reference("openai/gpt-image-2.5-sunburst", 0.0071, path=self.support))
        e = json.loads(self.support.read_text(encoding="utf-8"))["models"]["openai/gpt-image-2.5-sunburst"]
        self.assertEqual((e["state"], e["usd_per_image_reference"]), ("verified", 0.0071))
        self.assertFalse(pf.promote_reference("openai/gpt-image-2.5-sunburst", 0.0071, path=self.support))   # no change
        self.assertFalse(pf.promote_reference("no/such-model", 0.1, path=self.support))

    def test_a_real_reference_run_promotes_the_model(self):
        plan = copy.deepcopy(PLAN)
        exp = make_experiment(self.tmp, plan)
        ledger_select(exp, 1)
        led = lg.load(exp / "ledger.json")
        led["rows"][0]["model"] = "openai/gpt-image-2.5-sunburst"
        lg.save(exp / "ledger.json", led)
        old_file, old_obs = pf.REF_SUPPORT_FILE, gen.il.PRICE_OBSERVATIONS_FILE
        pf.REF_SUPPORT_FILE, gen.il.PRICE_OBSERVATIONS_FILE = self.support, str(self.tmp / "price-observations.json")
        try:
            rep = pf.run_preflight(exp, advisor=ADVISOR)
            self.assertTrue(any("not yet verified" in w for w in rep["warnings"]))
            logs = []
            gen.run_batch(exp, rep["fingerprint"], gateway=FakeGateway(cost=0.0071), poll_seconds=0.01,
                          max_usd=0.5, log=logs.append)
        finally:
            pf.REF_SUPPORT_FILE, gen.il.PRICE_OBSERVATIONS_FILE = old_file, old_obs
        e = json.loads(self.support.read_text(encoding="utf-8"))["models"]["openai/gpt-image-2.5-sunburst"]
        self.assertEqual(e["state"], "verified")
        self.assertEqual(e["usd_per_image_reference"], 0.0071)
        self.assertTrue(any("now recorded as verified" in m for m in logs))
        obs = json.loads((self.tmp / "price-observations.json").read_text(encoding="utf-8"))
        self.assertTrue(any(k.startswith("openai/gpt-image-2.5-sunburst|") and k.endswith("|reference") for k in obs), obs)


if __name__ == "__main__":
    unittest.main()
