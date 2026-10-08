"""Offline tests: retry, quick mode, the merged image.py CLI, and no loomloom CLI dependency."""
import json
import subprocess
import sys
import unittest
from pathlib import Path

HERE = Path(__file__).resolve().parent
SCRIPTS = HERE.parent / "scripts"
sys.path.insert(0, str(SCRIPTS))
sys.path.insert(0, str(HERE))
import experiment as ex  # noqa: E402
import generate as gen  # noqa: E402
import ledger as lg  # noqa: E402
import preflight as pf  # noqa: E402
import test_generate as tg  # noqa: E402
from test_generate import FakeGateway, quiet  # noqa: E402
from test_preflight import ADVISOR, Base  # noqa: E402


class RetryTests(Base):
    prepare = tg.GenerateTests.prepare
    run_gen = tg.GenerateTests.run_gen

    def retry_pf(self, **kw):
        return pf.run_preflight(self.exp, advisor=ADVISOR, retry=True, **kw)

    def test_retry_picks_only_failed_rows_one_image_each(self):
        fp = self.prepare(3)
        self.run_gen(fp, FakeGateway(fail_reasons={"req-1": "x", "req-2": "x"}), concurrency=1)
        statuses = {r["id"]: r["status"] for r in self.ledger()["rows"] if r["selected"]}
        self.assertEqual(sorted(statuses.values()), ["Completed", "Failed", "Failed"])
        rep = self.retry_pf()
        self.assertEqual(rep["ready"], 2)
        by_id = {r["id"]: r for r in rep["snapshot"]["rows"]}
        self.assertEqual(sorted(by_id), sorted(i for i, s in statuses.items() if s == "Failed"))
        for r in by_id.values():
            self.assertEqual(r["qty"], 1)
            self.assertEqual(r["first_sample"], 2)                   # the failed attempt used sample 1; ids are never reused
        self.assertIn("RETRY", rep["text"])
        # preflight did not flip statuses to Ready
        self.assertEqual({r["id"]: r["status"] for r in self.ledger()["rows"] if r["selected"]}, statuses)

    def test_retry_run_numbers_the_new_attempt_after_the_old_one_and_completes_the_row(self):
        fp = self.prepare(1)
        self.run_gen(fp, FakeGateway(fail_reasons={"req-1": "boom"}), concurrency=1)
        self.assertEqual(self.ledger()["rows"][0]["status"], "Failed")
        rep = self.retry_pf()
        self.run_gen(rep["fingerprint"], FakeGateway())
        led = self.ledger()
        self.assertEqual(sorted(a["sample_id"] for a in led["attempts"]), ["r001", "r001-2"])
        self.assertEqual(led["rows"][0]["status"], "Completed")
        self.assertEqual(len(led["batches"]), 2)
        self.assertTrue((self.exp / "round-2" / "r001-2.png").exists())

    def test_unknown_rows_are_not_retried_unless_asked(self):
        fp = self.prepare(2)
        self.run_gen(fp, FakeGateway(lambda n, b: (0, {}, "TimeoutError") if n == 1 else None), concurrency=1)
        rep = self.retry_pf()
        self.assertEqual(rep["ready"], 0)
        self.assertTrue(any("Unknown" in w for w in rep["warnings"]))
        rep2 = self.retry_pf(include_unknown=True)
        self.assertEqual(rep2["ready"], 1)

    def test_nothing_to_retry(self):
        fp = self.prepare(2)
        self.run_gen(fp, FakeGateway())
        self.assertEqual(self.retry_pf()["ready"], 0)


class QuickTests(Base):
    def test_quick_writes_a_runnable_snapshot_without_a_plan_or_workbook(self):
        out = self.tmp / "quick"
        q = ex.build_quick("a red apple on a white table", "e-commerce product photo", 2, out)
        self.assertEqual(q["images"], 2)
        self.assertFalse((out / "plan.json").exists())
        self.assertFalse((out / "experiment.xlsx").exists())
        gw = FakeGateway()
        res = gen.run_batch(out, q["fingerprint"], gateway=gw, poll_seconds=0.01, record_prices=False, log=quiet)
        self.assertEqual(res["by_status"], {"Completed": 2})
        self.assertEqual({b["prompt"] for b in gw.bodies}, {"a red apple on a white table"})   # verbatim
        self.assertNotIn("image", gw.bodies[0])                                               # no reference
        self.assertTrue((out / "round-1" / "r001.png").exists())

    def test_quick_sample_ids_have_no_eight_branch_limit(self):
        out = self.tmp / "quick8"
        q = ex.build_quick("a cat", "e-commerce product photo", 8, out, models=["openai/gpt-image-2.5-sunburst"])
        res = gen.run_batch(out, q["fingerprint"], gateway=FakeGateway(cost=0.0065), poll_seconds=0.01,
                            record_prices=False, log=quiet)
        self.assertEqual(res["samples"], 8)
        self.assertEqual(res["by_status"], {"Completed": 8})
        self.assertIn("r008", [a["sample_id"] for a in lg.load(out / "ledger.json")["attempts"]])
        self.assertEqual([r["take"] for r in lg.load(out / "ledger.json")["rows"]], list(range(1, 9)))   # 8 images = 8 rows

    def test_quick_rejects_a_bad_count(self):
        with self.assertRaises(ex.mx.PlanError):
            ex.build_quick("x", "e-commerce product photo", 3, self.tmp / "q")

    def test_second_quick_round_adds_rows_to_the_same_ledger(self):
        out = self.tmp / "quick2"
        a = ex.build_quick("a cat", "e-commerce product photo", 1, out)
        gen.run_batch(out, a["fingerprint"], gateway=FakeGateway(), poll_seconds=0.01, record_prices=False, log=quiet)
        b = ex.build_quick("a dog", "e-commerce product photo", 1, out)
        self.assertNotEqual(a["fingerprint"], b["fingerprint"])
        gen.run_batch(out, b["fingerprint"], gateway=FakeGateway(), poll_seconds=0.01, record_prices=False, log=quiet)
        led = lg.load(out / "ledger.json")
        self.assertEqual([r["id"] for r in led["rows"]], ["r001", "r002"])
        self.assertEqual(len(led["batches"]), 2)


class QuickFollowUpTests(Base):
    def run_quick(self, count=2, **kw):
        out = self.tmp / "q"
        q = ex.build_quick("a green pear on a wooden table", "e-commerce product photo", count, out,
                           models=["openai/gpt-image-2.5-sunburst"])
        res = gen.run_batch(out, q["fingerprint"], poll_seconds=0.01, record_prices=False, log=quiet, **kw)
        return out, q, res

    def test_retry_works_in_a_quick_folder_and_keeps_the_verbatim_prompt(self):
        out, q, _ = self.run_quick(2, gateway=FakeGateway(fail_reasons={"req-2": "boom"}))
        self.assertEqual([r["status"] for r in lg.load(out / "ledger.json")["rows"]], ["Completed", "Failed"])
        rep = pf.run_preflight(out, advisor=ADVISOR, retry=True)               # no plan.json here
        self.assertEqual(rep["ready"], 1)
        snap_row = rep["snapshot"]["rows"][0]
        self.assertEqual(snap_row["prompt"], "a green pear on a wooden table")
        self.assertEqual(snap_row["id"], "r002")
        self.assertEqual(snap_row["first_sample"], 2)
        self.assertFalse(any("experiment.xlsx" in w for w in rep["warnings"]))
        gw = FakeGateway()
        gen.run_batch(out, rep["fingerprint"], gateway=gw, poll_seconds=0.01, record_prices=False, log=quiet)
        self.assertEqual(gw.bodies[0]["prompt"], "a green pear on a wooden table")
        self.assertEqual({r["status"] for r in lg.load(out / "ledger.json")["rows"]}, {"Completed"})

    def test_a_paused_batch_does_not_leave_rows_generating(self):
        out, q, res = self.run_quick(2, gateway=FakeGateway(cost=0.0065), concurrency=1, max_usd=0.008)
        self.assertIn("budget guard", res["stopped"])
        led = lg.load(out / "ledger.json")
        self.assertEqual([r["status"] for r in led["rows"]], ["Completed", "Ready"])   # one done, one held back and not started: not "Generating"
        res2 = gen.run_batch(out, q["fingerprint"], gateway=FakeGateway(cost=0.0065), poll_seconds=0.01,
                             record_prices=False, log=quiet)
        self.assertEqual(res2["by_status"], {"Completed": 2})
        self.assertEqual({r["status"] for r in lg.load(out / "ledger.json")["rows"]}, {"Completed"})


class PageCompatTests(Base):
    def test_quick_run_feeds_the_v01_exploration_page_builder(self):
        out = self.tmp / "q"
        q = ex.build_quick("a red apple on a white table", "e-commerce product photo", 2, out)
        gen.run_batch(out, q["fingerprint"], gateway=FakeGateway(), poll_seconds=0.01, record_prices=False, log=quiet)
        run = json.loads((out / "round-1" / "run.json").read_text(encoding="utf-8"))
        self.assertEqual(run["prompt"], "a red apple on a white table")
        self.assertEqual([a["status"] for a in run["alternatives"]], ["COMPLETED", "COMPLETED"])
        session = json.loads((out / "session.json").read_text(encoding="utf-8"))
        self.assertEqual(session["rounds"][0]["round"], 1)
        env = {**__import__("os").environ, "PYTHONUTF8": "1"}
        r = subprocess.run([sys.executable, str(SCRIPTS / "build-exploration-page.py"), "--session", str(out),
                            "--title", "Red apple", "--subject", "a red apple", "--invocation", "test",
                            "--selected", "R001"],
                           capture_output=True, text=True, encoding="utf-8", env=env)
        self.assertEqual(r.returncode, 0, r.stderr)
        pages = list(out.glob("*/index.html"))
        self.assertEqual(len(pages), 1)
        self.assertIn("a red apple on a white table", pages[0].read_text(encoding="utf-8"))

    def test_v01_resolve_sees_the_rounds_the_new_run_made(self):
        out = self.tmp / "q2"
        q = ex.build_quick("a cat", "e-commerce product photo", 1, out)
        gen.run_batch(out, q["fingerprint"], gateway=FakeGateway(), poll_seconds=0.01, record_prices=False, log=quiet)
        import image as il
        self.assertEqual(il._next_round(out), 2)


class CliTests(unittest.TestCase):
    def test_image_py_exposes_the_new_subcommands_and_checks_cleanly(self):
        env = {**__import__("os").environ, "PYTHONUTF8": "1"}
        h = subprocess.run([sys.executable, str(SCRIPTS / "image.py"), "--help"], capture_output=True, text=True, encoding="utf-8", env=env)
        for cmd in ("plan", "preflight", "retry", "quick", "refresh", "run", "resolve", "check"):
            self.assertIn(cmd, h.stdout)
        c = subprocess.run([sys.executable, str(SCRIPTS / "image.py"), "check"], capture_output=True, text=True, encoding="utf-8", env=env)
        self.assertEqual(c.returncode, 0, c.stderr)
        self.assertIn("Image Lab 2 data files", c.stderr)

    def test_run_without_dir_or_confirm_refuses_and_spends_nothing(self):
        env = {**__import__("os").environ, "PYTHONUTF8": "1"}
        c = subprocess.run([sys.executable, str(SCRIPTS / "image.py"), "run"], capture_output=True, text=True, encoding="utf-8", env=env)
        self.assertEqual(c.returncode, 2)
        self.assertIn("--confirm", c.stderr)

    def test_no_loomloom_cli_is_invoked_by_the_runtime(self):
        for name in ("image.py", "generate.py", "preflight.py", "experiment.py"):
            src = (SCRIPTS / name).read_text(encoding="utf-8")
            self.assertNotIn("subprocess", src, name)
            self.assertNotIn("shutil.which", src, name)


if __name__ == "__main__":
    unittest.main()
