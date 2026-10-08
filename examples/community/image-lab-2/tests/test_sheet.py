"""Offline tests for scripts/sheet.py (the experiment contact sheet)."""
import copy
import json
import os
import re
import subprocess
import sys
import unittest
from pathlib import Path

HERE = Path(__file__).resolve().parent
SCRIPTS = HERE.parent / "scripts"
sys.path.insert(0, str(SCRIPTS))
sys.path.insert(0, str(HERE))
import generate as gen  # noqa: E402
import ledger as lg  # noqa: E402
import preflight as pf  # noqa: E402
import sheet as sh  # noqa: E402
from test_generate import FakeGateway, quiet  # noqa: E402
from test_m3 import PERSON_PLAN, ledger_select  # noqa: E402
from test_preflight import ADVISOR, Base, PLAN, make_experiment  # noqa: E402

ENV = {**os.environ, "PYTHONUTF8": "1"}


def payload(html: str) -> dict:
    m = re.search(r"const DATA = (.*?);\nconst \$ =", html, re.S)
    return json.loads(m.group(1))


class SheetTests(Base):
    def run_experiment(self, plan=None, n=4, gateway=None):
        exp = make_experiment(self.tmp, plan or PLAN)
        ledger_select(exp, n)
        fp = pf.run_preflight(exp, advisor=ADVISOR)["fingerprint"]
        gen.run_batch(exp, fp, gateway=gateway or FakeGateway(), poll_seconds=0.01, record_prices=False, log=quiet)
        return exp

    def test_the_sheet_holds_every_sample_with_values_cost_model_and_prompt(self):
        exp = self.run_experiment()
        html = sh.build_sheet(exp).read_text(encoding="utf-8")
        d = payload(html)
        self.assertEqual(d["dims"], ["camera", "lighting"])
        self.assertEqual(d["stats"]["images"], 4)
        self.assertEqual(len(d["rows"]), len([r for r in lg.load(exp / "ledger.json")["rows"] if r["status"] != "Removed"]))
        first = next(r for r in d["rows"] if r["samples"])
        smp = first["samples"][0]
        self.assertTrue(smp["file"].startswith("round-1/"))
        self.assertIn("Use the reference image", smp["prompt"])
        self.assertAlmostEqual(d["stats"]["cost"], 4 * 0.039, places=4)
        self.assertEqual(d["dim_values"]["camera"][:3], ["eye-level front", "three-quarter high", "low angle"])
        self.assertFalse(d["person"])

    def test_default_axes_and_explicit_axes(self):
        exp = self.run_experiment()
        d = payload(sh.build_sheet(exp).read_text(encoding="utf-8"))
        self.assertEqual((d["default_rows"], d["default_cols"]), ("camera", "lighting"))
        d = payload(sh.build_sheet(exp, "lighting", "none", out=self.tmp / "x.html").read_text(encoding="utf-8"))
        self.assertEqual((d["default_rows"], d["default_cols"]), ("lighting", "none"))
        with self.assertRaises(sh.SheetError):
            sh.build_sheet(exp, "nope", None)

    def test_hostile_cell_text_cannot_become_html(self):
        exp = self.run_experiment(n=1)
        led = lg.load(exp / "ledger.json")
        evil = '</script><img src=x onerror=alert(1)>'
        led["rows"][0]["notes"] = evil
        led["rows"][0]["params"]["camera"] = evil
        led["attempts"][0]["error"] = evil
        lg.save(exp / "ledger.json", led)
        html = sh.build_sheet(exp).read_text(encoding="utf-8")
        self.assertNotIn(evil, html)
        self.assertNotIn("</script><img", html)
        self.assertEqual(html.count("</script>"), 1)                       # only the real closing tag
        self.assertEqual(payload(html)["rows"][0]["notes"], evil)           # the data itself round-trips intact

    def test_failed_and_unknown_samples_become_placeholders_with_the_reason(self):
        gw = FakeGateway(lambda n, b: (0, {}, "TimeoutError") if n == 2 else None, fail_reasons={"req-1": "boom"})
        exp = self.run_experiment(n=3, gateway=gw)
        d = payload(sh.build_sheet(exp).read_text(encoding="utf-8"))
        statuses = sorted(s["status"] for r in d["rows"] for s in r["samples"])
        self.assertEqual(statuses, ["Completed", "Failed", "Unknown"])
        self.assertEqual(d["stats"]["problems"], 2)
        bad = [s for r in d["rows"] for s in r["samples"] if s["status"] != "Completed"]
        self.assertTrue(all(s["error"] for s in bad))

    def test_person_experiments_get_a_local_only_banner_and_no_inline_sheet(self):
        person = copy.deepcopy(PERSON_PLAN)
        person["consent_acknowledged"] = "2026-10-07T10:00:00"
        exp = self.run_experiment(person, n=1)
        self.assertTrue(payload(sh.build_sheet(exp).read_text(encoding="utf-8"))["person"])
        with self.assertRaises(sh.SheetError) as cm:
            sh.build_sheet(exp, inline=True)
        self.assertIn("stay local", str(cm.exception))

    def test_inline_embeds_the_images_for_a_normal_experiment(self):
        exp = self.run_experiment(n=2)
        d = payload(sh.build_sheet(exp, inline=True, out=self.tmp / "inline.html").read_text(encoding="utf-8"))
        files = [s["file"] for r in d["rows"] for s in r["samples"]]
        self.assertTrue(all(f.startswith("data:image/png;base64,") for f in files))

    def test_jpeg_images_from_recovered_runs_are_embedded_with_their_type(self):
        exp = self.run_experiment(n=1)
        led = lg.load(exp / "ledger.json")
        a = led["attempts"][0]
        old = exp / a["file"]
        new = old.with_suffix(".jpg")
        old.replace(new)
        a["file"] = a["file"].replace(".png", ".jpg")
        lg.save(exp / "ledger.json", led)
        d = payload(sh.build_sheet(exp, inline=True, out=self.tmp / "j.html").read_text(encoding="utf-8"))
        self.assertTrue(d["rows"][0]["samples"][0]["file"].startswith("data:image/jpeg;base64,"))

    def test_selected_ids_are_highlighted_and_the_cli_builds_the_page(self):
        exp = self.run_experiment(n=1)
        r = subprocess.run([sys.executable, str(SCRIPTS / "image.py"), "sheet", "--dir", str(exp),
                            "--selected", "r001-1, r002-1"], capture_output=True, text=True, encoding="utf-8", env=ENV)
        self.assertEqual(r.returncode, 0, r.stderr)
        self.assertIn("contact sheet:", r.stdout)
        d = payload((exp / "contact-sheet.html").read_text(encoding="utf-8"))
        self.assertEqual(d["selected"], ["r001-1", "r002-1"])
        bad = subprocess.run([sys.executable, str(SCRIPTS / "image.py"), "sheet", "--dir", str(exp), "--rows", "zzz"],
                             capture_output=True, text=True, encoding="utf-8", env=ENV)
        self.assertEqual(bad.returncode, 2)
        self.assertNotIn("Traceback", bad.stderr)

    def test_after_a_run_the_sheet_and_the_workbook_are_rebuilt_for_experiments_only(self):
        import experiment as ex
        exp = self.run_experiment(n=2)
        self.assertFalse((exp / "contact-sheet.html").exists())             # run_batch alone is only the engine
        out = ex.after_run(exp)
        self.assertEqual(out["sheet"], exp / "contact-sheet.html")
        self.assertTrue((exp / "contact-sheet.html").exists())
        self.assertEqual(payload((exp / "contact-sheet.html").read_text(encoding="utf-8"))["stats"]["images"], 2)
        q = self.tmp / "quick"
        qq = ex.build_quick("a cat", "e-commerce product photo", 1, q)
        gen.run_batch(q, qq["fingerprint"], gateway=FakeGateway(), poll_seconds=0.01, record_prices=False, log=quiet)
        self.assertEqual(ex.after_run(q), {"sheet": None, "workbook": None})
        self.assertFalse((q / "contact-sheet.html").exists())

    def test_quick_mode_folders_have_no_experiment_sheet_but_can_still_be_built(self):
        import experiment as ex
        out = self.tmp / "q"
        q = ex.build_quick("a cat", "e-commerce product photo", 2, out)
        gen.run_batch(out, q["fingerprint"], gateway=FakeGateway(), poll_seconds=0.01, record_prices=False, log=quiet)
        d = payload(sh.build_sheet(out).read_text(encoding="utf-8"))          # no dimensions: a flat grid by model
        self.assertEqual(d["dims"], [])
        self.assertEqual(d["default_rows"], "model")
        self.assertEqual(d["stats"]["images"], 2)


if __name__ == "__main__":
    unittest.main()
