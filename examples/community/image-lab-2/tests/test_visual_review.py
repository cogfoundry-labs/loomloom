"""scripts/review.py: visual-check verdicts in the ledger, the workbook's Review column, and the approved vision-model review (a fake gateway; nothing is spent)."""
import argparse
import contextlib
import io
import json
import shutil
import sys
import tempfile
import unittest
from pathlib import Path

HERE = Path(__file__).resolve().parent
sys.path.insert(0, str(HERE.parent / "scripts"))
sys.path.insert(0, str(HERE))
import experiment as ex  # noqa: E402
import ledger as lg  # noqa: E402
import review as rv  # noqa: E402
import workbook as wbk  # noqa: E402
from test_preflight import PNG  # noqa: E402

PLAN = {"schema_version": 1, "brief": "A poster for a library", "intent": "poster / flyer", "fixed": {"audience": "children"}, "quality": "medium", "model_strategy": "single",
        "dimensions": {"direction": ["1 Alpha", "2 Beta"], "mood": ["calm", "bold"], "aspect": ["4:5"]},
        "visual_checks": ["the headline is spelled exactly READ THE SUMMER", "books are the heart of the picture"]}
MODELS = [{"api_name": rv.DEFAULT_MODEL, "input_price": 0.75, "output_price": 4.5, "can_read_images": True, "company": "openai", "name": "GPT-5.4 mini"},
          {"api_name": "text/only", "input_price": 1.0, "output_price": 2.0, "can_read_images": False},
          {"api_name": "free/model", "input_price": None, "output_price": None, "can_read_images": True}]


def make_experiment(tmp: Path, plan=None) -> Path:
    src = tmp / "src"
    src.mkdir()
    (src / "plan.json").write_text(json.dumps(plan or PLAN), encoding="utf-8")
    out = tmp / "exp"
    ex.build_experiment(src / "plan.json", out, target=4)
    return out


def finish(exp: Path, row_ids, batch: int = 1, rnd: str = "round-1") -> None:
    """Mark rows Completed, each with one finished image on disk."""
    led = lg.load(exp / "ledger.json")
    (exp / rnd).mkdir(exist_ok=True)
    rows = lg.by_id(led)
    for rid in row_ids:
        (exp / rnd / f"{rid}.png").write_bytes(PNG)
        rows[rid]["status"] = "Completed"
        led["attempts"].append({"batch": batch, "row_id": rid, "sample": 1, "sample_id": rid, "params": rows[rid]["params"], "model": "openai/gpt-image-2.5-sunburst",
                                "mode": "text", "size": "1152x1440", "status": "Completed", "file": f"{rnd}/{rid}.png", "cost": 0.0186, "seconds": 40, "size_actual": "1152x1440"})
    if not any(b["no"] == batch for b in led["batches"]):
        led["batches"].append({"no": batch, "estimated_usd": 0.08, "actual_usd": round(0.0186 * len(row_ids), 4), "ended_at": "2026-10-10T10:00:00"})
    lg.save(exp / "ledger.json", led)


class Base(unittest.TestCase):
    def setUp(self):
        self.tmp = Path(tempfile.mkdtemp())
        self.addCleanup(shutil.rmtree, self.tmp, ignore_errors=True)
        self.exp = make_experiment(self.tmp)
        self.ids = [r["id"] for r in lg.load(self.exp / "ledger.json")["rows"]]
        finish(self.exp, self.ids[:3])

    def row(self, rid):
        return lg.by_id(lg.load(self.exp / "ledger.json"))[rid]


class RecordingTests(Base):
    def test_verdicts_are_stored_on_the_row_and_survive_a_refresh(self):
        a, b = self.ids[:2]
        rv.record(self.exp, {a: {"checks": {"1": "pass", "2": {"verdict": "fail", "reason": "no books"}}, "note": "No books in the picture."},
                             b: {"checks": [{"n": 1, "verdict": "pass"}, {"n": 2, "verdict": "pass"}]}}, source="agent", reviewer="Claude")
        r = self.row(a)["review"]
        self.assertEqual((r["source"], r["reviewer"], r["image"]), ("agent", "Claude", "round-1/%s.png" % a))
        self.assertEqual([(c["n"], c["verdict"]) for c in r["checks"]], [(1, "pass"), (2, "fail")])
        self.assertEqual(r["checks"][1]["reason"], "no books")
        ex.refresh(self.exp)                                                   # a refresh merges workbook edits: the verdicts stay
        self.assertEqual(self.row(a)["review"]["note"], "No books in the picture.")
        self.assertEqual(rv.mark(self.row(a)["review"]), "fail 2")
        self.assertEqual(rv.mark(self.row(b)["review"]), "ok")

    def test_a_check_the_review_leaves_out_is_cant_tell(self):
        rv.record(self.exp, {self.ids[0]: {"checks": {"1": "pass"}}})
        c = self.row(self.ids[0])["review"]["checks"]
        self.assertEqual([x["verdict"] for x in c], ["pass", "cant_tell"])
        self.assertEqual(rv.mark(self.row(self.ids[0])["review"]), "?2")

    def test_a_note_alone_is_allowed(self):
        rv.record(self.exp, {self.ids[0]: {"note": "Nice."}})
        r = self.row(self.ids[0])["review"]
        self.assertEqual((r["checks"], rv.mark(r), rv.caption(r)), ([], "note", "Nice."))

    def test_bad_input_is_refused_with_a_clear_message(self):
        with self.assertRaisesRegex(rv.ReviewError, "check 3 does not exist"):
            rv.record(self.exp, {self.ids[0]: {"checks": {"3": "pass"}}})
        with self.assertRaisesRegex(rv.ReviewError, "pass, fail, cant_tell, na"):
            rv.record(self.exp, {self.ids[0]: {"checks": {"1": "maybe"}}})
        with self.assertRaisesRegex(rv.ReviewError, "source must be"):
            rv.record(self.exp, {self.ids[0]: {"note": "x"}}, source="robot")

    def test_a_row_with_no_image_or_no_such_row_is_skipped_with_a_warning(self):
        res = rv.record(self.exp, {self.ids[3]: {"note": "x"}, "r999": {"note": "y"}})
        self.assertEqual(len(res["warnings"]), 2)
        self.assertIn("no finished image yet", res["warnings"][0])
        self.assertNotIn("review", self.row(self.ids[3]))

    def test_a_verdict_goes_stale_when_a_newer_image_replaces_it(self):
        rid = self.ids[0]
        rv.record(self.exp, {rid: {"checks": {"1": "pass", "2": "pass"}}})
        led = lg.load(self.exp / "ledger.json")
        self.assertFalse(rv.is_stale(led["rows"][0]["review"], self.exp, led, rid))
        (self.exp / "round-2").mkdir()
        (self.exp / "round-2" / f"{rid}.png").write_bytes(PNG)
        led["attempts"].append({"batch": 2, "row_id": rid, "sample": 2, "sample_id": f"{rid}-2", "status": "Completed", "file": f"round-2/{rid}.png"})
        lg.save(self.exp / "ledger.json", led)
        led = lg.load(self.exp / "ledger.json")
        self.assertTrue(rv.is_stale(led["rows"][0]["review"], self.exp, led, rid))
        self.assertTrue(rv.cell_text(led["rows"][0]["review"], True).startswith("(older image) 2/2 pass"))

    def test_the_formats(self):
        rev = rv.normalize({"checks": {"1": "fail", "2": "cant_tell"}, "note": "n"}, 3, source="user", reviewer="me", image="x.png")
        self.assertEqual(rv.cell_text(rev), "0/3 pass; fail: 1; can't tell: 2,3 · n")
        self.assertEqual(rv.mark(rev), "fail 1")
        self.assertEqual(rv.caption({"checks": [{"n": 2, "verdict": "fail", "reason": "typo"}], "note": ""}), "check 2: typo")
        self.assertEqual(rv.caption({"checks": [{"n": 1, "verdict": "pass", "reason": ""}], "note": ""}), "All checks passed.")
        # a check that does not apply to this image is left out of the count and never makes a tile "fail" or "?"
        na = rv.normalize({"checks": {"1": "pass", "2": "na", "3": "pass"}}, 3, source="agent", reviewer="me", image=None)
        self.assertEqual((rv.mark(na), rv.cell_text(na)), ("ok", "2/2 pass"))
        only_na = rv.normalize({"checks": {"1": "na"}}, 1, source="agent", reviewer="me", image=None)
        self.assertEqual((rv.mark(only_na), rv.cell_text(only_na)), ("n/a", "no check applies"))
        self.assertEqual((rv.cell_text(None), rv.mark(None), rv.caption(None)), ("", "", ""))

    def test_the_record_file_is_checked(self):
        bad = self.tmp / "v.json"
        bad.write_text("[1, 2]", encoding="utf-8")
        with self.assertRaises(rv.ReviewError):
            rv.load_record_file(bad)
        with self.assertRaises(rv.ReviewError):
            rv.load_record_file(self.tmp / "missing.json")


class WorkbookColumnTests(Base):
    def test_the_review_column_shows_the_verdicts_and_is_never_read_back(self):
        rv.record(self.exp, {self.ids[0]: {"checks": {"1": "pass", "2": "fail"}, "note": "No books."}})
        import openpyxl
        wb = openpyxl.load_workbook(self.exp / "experiment.xlsx", read_only=True)
        rows = list(wb["Experiment"].iter_rows(values_only=True))
        i = rows[0].index("Review")
        self.assertEqual(rows[0][i - 1], "Cost")
        self.assertEqual(rows[0][i + 1], "Notes")
        self.assertEqual([r[i] for r in rows[1:] if r[0] == self.ids[0]], ["1/2 pass; fail: 2 · No books."])
        sheet = wbk.read_workbook(self.exp / "experiment.xlsx", ["direction", "mood", "aspect"])
        self.assertNotIn("Review", sheet["extra_columns"])                     # system-owned: ignored on read, not carried as an extra column
        wb.close()


class QuoteTests(Base):
    def test_a_quote_counts_the_images_prices_them_and_makes_a_fingerprint(self):
        q = rv.quote(self.exp, MODELS)
        self.assertEqual((q["model"], q["images"]), (rv.DEFAULT_MODEL, 3))
        self.assertTrue(0 < q["expected_usd"] < q["max_usd"])
        self.assertEqual(len(q["checks"]), 2)
        self.assertEqual(q["fingerprint"], rv.quote(self.exp, MODELS)["fingerprint"])
        self.assertIn("never generates or changes an image", rv.format_quote(q))

    def test_the_fingerprint_changes_with_the_checks_the_rows_and_the_image(self):
        base = rv.quote(self.exp, MODELS)["fingerprint"]
        self.assertNotEqual(base, rv.quote(self.exp, MODELS, extra_checks=["nobody looks at the camera"])["fingerprint"])
        self.assertNotEqual(base, rv.quote(self.exp, MODELS, rows=[self.ids[0]])["fingerprint"])
        (self.exp / "round-1" / f"{self.ids[0]}.png").write_bytes(PNG + b"0")
        self.assertNotEqual(base, rv.quote(self.exp, MODELS)["fingerprint"])

    def test_only_unreviewed_skips_rows_with_a_current_review(self):
        rv.record(self.exp, {self.ids[0]: {"checks": {"1": "pass", "2": "pass"}}})
        q = rv.quote(self.exp, MODELS, only_unreviewed=True)
        self.assertEqual((q["images"], q["skipped"]), (2, [self.ids[0]]))

    def test_what_cannot_be_quoted_is_refused(self):
        for kw, msg in (({"model": "nope/x"}, "not in the gateway"), ({"model": "text/only"}, "cannot read images"), ({"model": "free/model"}, "no price"),
                        ({"rows": [self.ids[3]]}, "no finished image")):
            with self.assertRaisesRegex(rv.ReviewError, msg):
                rv.quote(self.exp, MODELS, **kw)
        led = lg.load(self.exp / "ledger.json")
        (self.exp / "plan.json").write_text(json.dumps({k: v for k, v in PLAN.items() if k != "visual_checks"}), encoding="utf-8")
        with self.assertRaisesRegex(rv.ReviewError, "no visual_checks"):
            rv.quote(self.exp, MODELS)
        self.assertEqual(rv.quote(self.exp, MODELS, extra_checks=["x"])["checks"], ["x"])        # a --check stands in for the plan's own
        self.assertEqual(led["rows"][0]["id"], self.ids[0])


class PersonTests(Base):
    def test_a_pictured_person_without_consent_is_never_sent_to_a_model_provider(self):
        plan = json.loads((self.exp / "plan.json").read_text(encoding="utf-8"))
        plan["references"] = [{"id": "ref1", "file": "refs/p.png", "role": "person", "contains_person": True}]
        (self.exp / "plan.json").write_text(json.dumps(plan), encoding="utf-8")
        with self.assertRaisesRegex(rv.ReviewError, "acknowledge-person"):
            rv.quote(self.exp, MODELS)


class FakeVision:
    """A stand-in for the gateway's chat endpoint: one call per image, a canned answer per row."""

    def __init__(self, fail_for=(), cant_parse=False):
        self.calls = []
        self.fail_for = set(fail_for)
        self.cant_parse = cant_parse

    def __call__(self, body):
        text = body["messages"][0]["content"][0]["text"]
        self.calls.append(body)
        if len(self.calls) in self.fail_for:
            return 0, {}, "ConnectionResetError: gone"
        if self.cant_parse:
            return 200, {"choices": [{"message": {"content": "I cannot do that"}}], "usage": {"prompt_tokens": 2000, "completion_tokens": 50}}, ""
        answer = {"text_seen": "READ THE SUMMER", "checks": [{"n": 1, "verdict": "pass", "reason": "spelled right"}, {"n": 2, "verdict": "fail", "reason": "no books"}],
                  "note": "No books anywhere."}
        return 200, {"choices": [{"message": {"content": "```json\n" + json.dumps(answer) + "\n```"}}], "usage": {"prompt_tokens": 2000, "completion_tokens": 100}}, ""


class RunTests(Base):
    def test_the_prompt_carries_the_brief_the_choices_and_every_check(self):
        text = rv.review_prompt("A poster", {"direction": "1 Alpha", "mood": ""}, ["one", "two"])
        self.assertIn("A poster", text)
        self.assertIn("direction: 1 Alpha", text)
        self.assertNotIn("mood", text)
        self.assertIn("1. one 2. two", text)
        self.assertIn("cant_tell", text)
        self.assertIn("na when the requirement does not apply to THIS image", text)

    def test_an_approved_run_records_model_verdicts_costs_them_and_keeps_an_audit_file(self):
        fake = FakeVision()
        q = rv.quote(self.exp, MODELS)
        res = rv.run(q, q["fingerprint"], self.exp, MODELS, post=fake)
        self.assertEqual(len(fake.calls), 3)
        self.assertEqual((res["recorded"], res["failed"]), (3, []))
        self.assertAlmostEqual(res["cost_usd"], 3 * (2000 * 0.75 + 100 * 4.5) / 1e6, delta=0.0001)
        r = self.row(self.ids[0])["review"]
        self.assertEqual((r["source"], r["reviewer"]), ("model", rv.DEFAULT_MODEL))
        self.assertEqual(r["text_seen"], "READ THE SUMMER")
        self.assertEqual(rv.mark(r), "fail 2")
        audit = json.loads((self.exp / "reviews" / f"review-{q['fingerprint']}.json").read_text(encoding="utf-8"))
        self.assertEqual(len(audit["results"]), 3)
        content = fake.calls[0]["messages"][0]["content"]
        self.assertTrue(content[1]["image_url"]["url"].startswith("data:image/"))
        self.assertEqual(fake.calls[0]["model"], rv.DEFAULT_MODEL)

    def test_a_wrong_confirmation_or_a_changed_image_sends_nothing(self):
        fake = FakeVision()
        q = rv.quote(self.exp, MODELS)
        with self.assertRaises(PermissionError):
            rv.run(q, "not-the-fingerprint", self.exp, MODELS, post=fake)
        (self.exp / "round-1" / f"{self.ids[1]}.png").write_bytes(PNG + b"1")
        with self.assertRaises(PermissionError):
            rv.run(q, q["fingerprint"], self.exp, MODELS, post=fake)
        self.assertEqual(fake.calls, [])
        self.assertNotIn("review", self.row(self.ids[0]))

    def test_a_failed_or_unreadable_call_is_reported_and_never_retried(self):
        fake = FakeVision(fail_for={2})
        q = rv.quote(self.exp, MODELS)
        res = rv.run(q, q["fingerprint"], self.exp, MODELS, post=fake, workers=1)
        self.assertEqual(len(fake.calls), 3)                                # one call per image, no retry
        self.assertEqual((res["recorded"], len(res["failed"])), (2, 1))
        garbled = FakeVision(cant_parse=True)
        q2 = rv.quote(self.exp, MODELS, rows=[self.ids[0]])
        res2 = rv.run(q2, q2["fingerprint"], self.exp, MODELS, post=garbled)
        self.assertEqual((res2["recorded"], res2["failed"]), (0, [self.ids[0]]))

    def test_the_budget_stops_calls_before_they_are_sent(self):
        fake = FakeVision()
        q = rv.quote(self.exp, MODELS, budget=0.0001)
        res = rv.run(q, q["fingerprint"], self.exp, MODELS, post=fake)
        self.assertEqual(fake.calls, [])
        self.assertEqual(len(res["skipped"]), 3)


class CommandTests(Base):
    def run_cmd(self, **kw):
        a = argparse.Namespace(dir=str(self.exp), record=None, by=None, source="agent", rows=None, only_unreviewed=False, model=None, check=None, max_usd=None, confirm=None)
        for k, v in kw.items():
            setattr(a, k, v)
        out, err = io.StringIO(), io.StringIO()
        with contextlib.redirect_stdout(out), contextlib.redirect_stderr(err):
            code = rv.run_command(a, MODELS)
        return code, out.getvalue(), err.getvalue()

    def test_without_confirm_it_only_prints_a_quote(self):
        code, out, _ = self.run_cmd()
        self.assertEqual(code, 0)
        self.assertIn("REVIEW QUOTE", out)
        self.assertIn("--confirm", out)
        self.assertFalse((self.exp / "reviews").exists())

    def test_record_from_a_file(self):
        f = self.tmp / "v.json"
        f.write_text(json.dumps({self.ids[0]: {"checks": {"1": "pass", "2": "pass"}, "note": "Good."}}), encoding="utf-8")
        code, out, _ = self.run_cmd(record=str(f), by="Ana", source="user")
        self.assertEqual(code, 0)
        self.assertEqual(self.row(self.ids[0])["review"]["reviewer"], "Ana")
        self.assertIn("Review column updated", out)

    def test_an_error_is_a_clean_message_not_a_traceback(self):
        code, _, err = self.run_cmd(model="nope/x")
        self.assertEqual(code, 2)
        self.assertIn("error:", err)


if __name__ == "__main__":
    unittest.main()
