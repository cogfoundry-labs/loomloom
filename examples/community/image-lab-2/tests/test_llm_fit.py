"""Assistant fit, level 1 (scripts/llm_fit.py). Offline: nothing here calls the gateway."""
import json
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path
from unittest import mock

HERE = Path(__file__).resolve().parent
SCRIPTS = HERE.parent / "scripts"
sys.path.insert(0, str(SCRIPTS))
import llm_fit as lf  # noqa: E402

PAYLOAD = {"data": [
    {"api_name": "anthropic/claude-sonnet-5", "company": "Anthropic", "name": "Claude Sonnet 5", "context_window": 1000000, "max_tokens": 64000,
     "architecture": {"input": "text+image", "output": "text"}, "pricing": {"input_price": 2, "output_price": 10, "cached_price": 0.2, "currency": "USD"},
     "support_apis": ["/v1/chat/completions"]},
    {"api_name": "google/gemini-3.1-pro-preview", "company": "Google", "name": "Gemini 3.1 Pro Preview", "context_window": 1048576, "max_tokens": 65535,
     "architecture": {"input": ""}, "pricing": {"input_price": 2, "output_price": 12, "currency": "USD"}, "support_apis": []},
    {"api_name": "openai/gpt-5.6-luna", "company": "OpenAI", "name": "GPT-5.6 Luna", "context_window": 400000, "max_tokens": 128000,
     "architecture": {"input": "text"}, "pricing": {"input_price": 0.2, "output_price": 1.2, "currency": "USD"}, "support_apis": []},
]}


def rec(model, task, score, **kw):
    base = {"model": model, "task": task, "score": score, "n": 3, "repeats": 2, "spread": 0.03, "judge_independent": True,
            "harness": "gateway", "evidence_type": "measured", "date": "2026-10-08"}
    base.update(kw)
    return base


class Models(unittest.TestCase):
    def setUp(self):
        self.models = lf.parse_models(PAYLOAD)

    def test_the_listing_is_reduced_and_an_unlisted_modality_is_not_guessed(self):
        by = {m["api_name"]: m for m in self.models}
        self.assertEqual(by["anthropic/claude-sonnet-5"]["can_read_images"], True)
        self.assertEqual(by["google/gemini-3.1-pro-preview"]["modality"], "unlisted")
        self.assertIsNone(by["google/gemini-3.1-pro-preview"]["can_read_images"])      # unknown, never assumed
        self.assertEqual(by["openai/gpt-5.6-luna"]["can_read_images"], False)

    def test_an_exact_name_matches_in_any_spelling(self):
        for name in ("Claude Sonnet 5", "claude-sonnet-5", "anthropic/claude-sonnet-5", "Anthropic Claude Sonnet 5", "CLAUDE SONNET 5"):
            self.assertEqual(lf.map_assistant(name, self.models)["match"]["api_name"], "anthropic/claude-sonnet-5", name)

    def test_a_near_version_is_a_neighbour_not_a_match(self):
        m = lf.map_assistant("Claude Sonnet 5.5", self.models)
        self.assertIsNone(m["match"])
        self.assertEqual(m["how"], "not on the gateway")
        self.assertEqual([n["name"] for n in m["neighbours"]], ["Claude Sonnet 5"])

    def test_no_name_is_reported_as_not_given(self):
        self.assertEqual(lf.map_assistant(None, self.models)["how"], "not given")
        self.assertEqual(lf.map_assistant("  ", self.models)["how"], "not given")

    def test_the_live_call_falls_back_to_the_snapshot_without_a_token(self):
        def no_token():
            raise SystemExit("no token")
        models, source, _ = lf.load_models(fetch=no_token)
        self.assertIn("snapshot", source)
        self.assertGreater(len(models), 10)

    def test_a_working_fetch_is_used_and_marked_live(self):
        models, source, _ = lf.load_models(fetch=lambda: (200, PAYLOAD, ""))
        self.assertEqual(len(models), 3)
        self.assertIn("live", source)


class Advice(unittest.TestCase):
    def setUp(self):
        self.models = lf.parse_models(PAYLOAD)

    def test_with_no_evidence_nothing_is_recommended(self):
        rep = lf.advise("Claude Sonnet 5.5", self.models, [], "yes")
        self.assertTrue(all("no recommendation yet" in r["recommendation"] for r in rep["rows"]))
        text = lf.format_report(rep, "the saved snapshot", "2026-10-08")
        self.assertIn("none yet", text)
        self.assertIn("not on the gateway", text)
        self.assertIn("Advice only", text)

    def test_a_host_that_cannot_read_images_is_told_so_for_the_result_review(self):
        rep = lf.advise("Claude Sonnet 5", self.models, [], "no")
        review = next(r for r in rep["rows"] if r["step"] == "Result review")
        self.assertIn("cannot read image files", review["recommendation"])
        self.assertNotIn("cannot read", next(r for r in rep["rows"] if r["step"] == "Plan writing")["recommendation"])

    def test_models_measured_at_the_hosts_level_are_reported_as_ties_with_the_cheapest_first(self):
        recs = [rec("anthropic/claude-sonnet-5", "vision", 1.0), rec("m/cheap", "vision", 1.0, cost_per_use=0.0007),
                rec("m/pricey", "vision", 1.0, cost_per_use=0.0023), rec("m/worse", "vision", 0.6)]
        rep = lf.advise("Claude Sonnet 5", self.models, recs, "yes")
        row = next(r for r in rep["rows"] if r["step"] == "Result review")
        self.assertEqual([t["model"] for t in row["ties"]], ["m/cheap", "m/pricey"])          # the lower-scoring model is not a tie
        self.assertEqual((row["recommendation"], row["alt"]), ("keep", None))
        text = lf.format_report(rep)
        self.assertIn("2 measured at your level; cheapest m/cheap (about $0.0007 per use)", text)

    def test_measured_but_all_lower_says_none_better(self):
        recs = [rec("anthropic/claude-sonnet-5", "plan", 0.95), rec("m/worse", "plan", 0.5)]
        text = lf.format_report(lf.advise("Claude Sonnet 5", self.models, recs, "yes"))
        self.assertIn("1 measured, none better", text)

    def test_levels_are_words_and_a_good_host_is_told_to_keep(self):
        recs = [rec("anthropic/claude-sonnet-5", "plan", 0.95)]
        row = next(r for r in lf.advise("Claude Sonnet 5", self.models, recs)["rows"] if r["step"] == "Plan writing")
        self.assertEqual((row["host_level"], row["recommendation"]), ("Strong", "keep"))
        self.assertEqual(lf.level(0.7), "Good")
        self.assertEqual(lf.level(0.3), "Limited")
        self.assertEqual(lf.level(None), "Not measured")

    def test_a_clearly_better_alternative_measured_the_same_way_is_suggested(self):
        recs = [rec("anthropic/claude-sonnet-5", "vision", 0.55), rec("google/gemini-3.1-pro-preview", "vision", 0.9, cost_per_use=0.16)]
        row = next(r for r in lf.advise("Claude Sonnet 5", self.models, recs)["rows"] if r["step"] == "Result review")
        self.assertEqual((row["tier"], row["recommendation"]), ("strong", "suggested"))
        self.assertEqual(row["alt"]["model"], "google/gemini-3.1-pro-preview")

    def test_results_measured_in_different_harnesses_never_count_as_strong(self):
        recs = [rec("Claude Sonnet 5.5", "vision", 0.5, harness="agent"), rec("google/gemini-3.1-pro-preview", "vision", 0.95)]
        row = next(r for r in lf.advise("Claude Sonnet 5.5", self.models, recs)["rows"] if r["step"] == "Result review")
        self.assertEqual((row["tier"], row["recommendation"]), ("suggestive", "optional"))

    def test_small_samples_and_differences_inside_the_noise_are_not_enough(self):
        small = [rec("anthropic/claude-sonnet-5", "direction", 0.6, n=1, repeats=1), rec("openai/gpt-5.6-luna", "direction", 0.9, n=1, repeats=1)]
        row = next(r for r in lf.advise("Claude Sonnet 5", self.models, small)["rows"] if r["step"] == "Creative Direction")
        self.assertEqual(row["tier"], "suggestive")                     # better, but one brief and one run
        noisy = [rec("anthropic/claude-sonnet-5", "direction", 0.80, spread=0.1), rec("openai/gpt-5.6-luna", "direction", 0.85, spread=0.1)]
        row = next(r for r in lf.advise("Claude Sonnet 5", self.models, noisy)["rows"] if r["step"] == "Creative Direction")
        self.assertEqual(row["tier"], "insufficient")                   # 0.05 inside a 0.1 spread
        self.assertEqual(row["recommendation"], "keep")                 # the host is Strong: nothing to change

    def test_an_unmeasured_judge_or_a_non_measured_record_is_ignored(self):
        recs = [rec("google/gemini-3.1-pro-preview", "vision", 0.99, evidence_type="reported"), {"model": "x", "task": "vision"}]
        self.assertEqual(lf._agg(recs), {})

    def test_a_damaged_record_line_is_skipped(self):
        with tempfile.TemporaryDirectory() as d:
            p = Path(d) / "records.jsonl"
            p.write_text(json.dumps(rec("a", "plan", 0.9)) + "\nnot json\n\n" + json.dumps(rec("b", "plan", 0.8)) + "\n", encoding="utf-8")
            self.assertEqual(len(lf.load_records(p)), 2)
            self.assertEqual(lf.load_records(Path(d) / "missing.jsonl"), [])


class Probes(unittest.TestCase):
    def setUp(self):
        self.models = lf.parse_models({"data": [
            {"api_name": "openai/gpt-5.6-luna", "company": "OpenAI", "name": "GPT-5.6 Luna", "architecture": {"input": "text"},
             "pricing": {"input_price": 0.2, "output_price": 1.2, "currency": "USD"}},
            {"api_name": "google/gemini-3-flash", "company": "Google", "name": "Gemini 3 Flash", "architecture": {"input": "text+image"},
             "pricing": {"input_price": 0.5, "output_price": 3, "currency": "USD"}}]})

    def test_the_quote_is_deterministic_small_and_names_what_is_sent(self):
        q1, q2 = lf.probe_quote(self.models), lf.probe_quote(self.models)
        self.assertEqual(q1["fingerprint"], q2["fingerprint"])
        self.assertLess(q1["total_est_max_usd"], 0.01)
        self.assertEqual([c["id"] for c in q1["calls"]], ["text", "vision"])
        self.assertEqual(q1["unpriced"], [])

    def test_without_the_matching_confirmation_nothing_is_sent(self):
        sent = []
        with self.assertRaises(PermissionError):
            lf.run_probes(lf.probe_quote(self.models), "wrong", self.models, post=lambda b: sent.append(b))
        self.assertEqual(sent, [])

    def test_with_the_confirmation_the_usage_is_recorded(self):
        quote = lf.probe_quote(self.models)
        seen = []

        def post(body):
            seen.append(body)
            return 200, {"choices": [{"message": {"content": "ok"}}], "usage": {"prompt_tokens": 12, "completion_tokens": 1}, "cost": 0.000003}, ""

        with tempfile.TemporaryDirectory() as d:
            with mock.patch.object(lf, "PROBES_FILE", Path(d) / "probes.jsonl"):
                res = lf.run_probes(quote, quote["fingerprint"], self.models, post=post)
                lines = (Path(d) / "probes.jsonl").read_text(encoding="utf-8").splitlines()
        self.assertEqual(len(seen), 2)
        self.assertEqual([r["usage"]["prompt_tokens"] for r in res], [12, 12])
        self.assertEqual(res[0]["extra"], {"cost": 0.000003})                       # any cost field the gateway reports is kept
        self.assertEqual(len(lines), 2)
        content = seen[1]["messages"][0]["content"]
        self.assertTrue(any(part.get("type") == "image_url" for part in content))   # the second probe carries one tiny image
        self.assertEqual(seen[0]["max_tokens"], 16)


class Pilot(unittest.TestCase):
    def setUp(self):
        self.models = lf.parse_models({"data": [
            {"api_name": n, "company": "x", "name": n, "architecture": {"input": "text"}, "pricing": {"input_price": 10, "output_price": 50, "currency": "USD"}}
            for n in lf.PILOT_MODELS]})

    def test_the_quote_has_an_expected_figure_and_a_ceiling_from_the_output_cap(self):
        q = lf.pilot_quote(self.models, "x" * 3000)
        self.assertEqual(len(q["calls"]), 2)
        self.assertGreater(q["max_usd"], q["expected_usd"])
        self.assertAlmostEqual(q["calls"][0]["est_usd_max"], (1000 * 10 + lf.PILOT_MAX_TOKENS * 50) / 1e6, places=4)
        self.assertNotEqual(q["fingerprint"], lf.pilot_quote(self.models, "y" * 3000)["fingerprint"])    # the prompt is part of what is approved

    def test_nothing_is_sent_without_the_matching_confirmation_or_when_the_prompt_changed(self):
        sent = []
        q = lf.pilot_quote(self.models, "hello")
        with self.assertRaises(PermissionError):
            lf.run_pilot(q, "nope", self.models, post=lambda b: sent.append(b), prompt="hello")
        with self.assertRaises(PermissionError):
            lf.run_pilot(q, q["fingerprint"], self.models, post=lambda b: sent.append(b), prompt="hello, changed")
        self.assertEqual(sent, [])

    def test_a_confirmed_run_records_usage_the_implied_cost_and_the_plan_checks(self):
        import matrix as mx
        plan = mx.json.loads(json.dumps({"schema_version": 1, "brief": "b", "intent": "social post",
                                         "dimensions": {"direction": ["a", "b", "c"], "camera": ["x", "y"]}}))
        reply = "Sheet.\n```json\n" + json.dumps(plan) + "\n```"
        q = lf.pilot_quote(self.models, "hello")
        seen = []

        def post(body):
            seen.append(body)
            return 200, {"choices": [{"message": {"content": reply}, "finish_reason": "stop"}], "usage": {"prompt_tokens": 1000, "completion_tokens": 2000}}, ""

        with tempfile.TemporaryDirectory() as d:
            with mock.patch.object(lf, "PILOT_FILE", Path(d) / "pilot.jsonl"):
                res = lf.run_pilot(q, q["fingerprint"], self.models, post=post, prompt="hello")
                self.assertEqual(len((Path(d) / "pilot.jsonl").read_text(encoding="utf-8").splitlines()), 2)
                self.assertTrue((Path(d) / "pilot").is_dir())
        self.assertEqual([b["model"] for b in seen], list(lf.PILOT_MODELS))
        self.assertTrue(all(b["max_tokens"] == lf.PILOT_MAX_TOKENS for b in seen))
        self.assertAlmostEqual(res[0]["implied_usd"], (1000 * 10 + 2000 * 50) / 1e6, places=5)
        self.assertEqual((res[0]["checks"]["valid"], res[0]["checks"]["directions"]), (True, 3))

    def test_a_reply_without_a_plan_is_reported_not_crashed_on(self):
        self.assertEqual(lf.pilot_checks("no plan here")["valid"], False)
        self.assertEqual(lf.pilot_checks("```json\n{not json}\n```")["valid"], False)


class Streaming(unittest.TestCase):
    def serve(self, events):
        import http.server
        import threading

        class H(http.server.BaseHTTPRequestHandler):
            def do_POST(self):
                self.rfile.read(int(self.headers.get("Content-Length", 0)))
                self.send_response(200)
                self.send_header("Content-Type", "text/event-stream")
                self.end_headers()
                for e in events:
                    self.wfile.write(("data: " + (e if isinstance(e, str) else json.dumps(e)) + "\n\n").encode())
                    self.wfile.flush()

            def log_message(self, *a):
                pass

        srv = http.server.HTTPServer(("127.0.0.1", 0), H)
        threading.Thread(target=srv.serve_forever, daemon=True).start()
        self.addCleanup(srv.server_close)
        self.addCleanup(srv.shutdown)
        return f"http://127.0.0.1:{srv.server_address[1]}/chat/completions"

    def call(self, events):
        import urllib.request
        return lf.stream_chat("t", {"model": "m", "messages": []}, url=self.serve(events),
                              opener=urllib.request.build_opener(urllib.request.ProxyHandler({})))

    def test_events_are_folded_into_one_reply_with_usage_and_finish_reason(self):
        status, js, err = self.call([
            {"choices": [{"delta": {"content": "Hel"}}]}, {"choices": [{"delta": {"content": "lo"}, "finish_reason": "stop"}]},
            {"choices": [], "usage": {"prompt_tokens": 5, "completion_tokens": 2}, "provider": "X"}, "[DONE]"])
        self.assertEqual((status, err), (200, ""))
        self.assertEqual(js["choices"][0]["message"]["content"], "Hello")
        self.assertEqual(js["choices"][0]["finish_reason"], "stop")
        self.assertEqual(js["usage"], {"prompt_tokens": 5, "completion_tokens": 2})
        self.assertEqual(js["provider"], "X")

    def test_a_stream_that_ends_without_done_still_returns_what_arrived(self):
        status, js, err = self.call([{"choices": [{"delta": {"content": "part"}}]}])
        self.assertEqual(js["choices"][0]["message"]["content"], "part")

    def test_the_streaming_check_is_gated_by_its_quote(self):
        models = lf.parse_models({"data": [{"api_name": lf.STREAM_CHECK["model"], "company": "x", "name": "n", "architecture": {"input": "text"},
                                            "pricing": {"input_price": 0.2, "output_price": 1.2, "currency": "USD"}}]})
        q = lf.stream_check_quote(models)
        self.assertLess(q["max_usd"], 0.01)
        sent = []
        with self.assertRaises(PermissionError):
            lf.run_stream_check(q, "nope", models, post=lambda b: sent.append(b))
        self.assertEqual(sent, [])
        with tempfile.TemporaryDirectory() as d, mock.patch.object(lf, "PILOT_FILE", Path(d) / "pilot.jsonl"):
            rec = lf.run_stream_check(q, q["fingerprint"], models, post=lambda b: (200, {"choices": [{"message": {"content": "x" * 10}, "finish_reason": "stop"}], "usage": {"prompt_tokens": 9}}, ""))
        self.assertEqual((rec["status"], rec["reply_chars"]), (200, 10))


class DirectionScoring(unittest.TestCase):
    def good_reply(self):
        plan = json.loads((SCRIPTS.parent / "references" / "examples" / "direction-plan.json").read_text(encoding="utf-8"))
        return "Said / Inferred / Open gaps\n```json\n" + json.dumps(plan) + "\n```"

    def test_the_shipped_example_scores_high_and_names_its_misses(self):
        r = lf.direction_checks(self.good_reply())
        failed = [k for k, v in r["checks"].items() if not v]
        self.assertEqual(failed, ["keep out in every direction"])           # the example words keep-outs only in some directions
        self.assertGreaterEqual(r["score"], 0.9)

    def test_a_reply_without_a_plan_scores_only_for_what_it_has(self):
        r = lf.direction_checks("Said, inferred and open gaps, but no plan.")
        self.assertEqual(r["checks"]["sheet sections"], True)
        self.assertEqual(r["checks"]["plan block"], False)
        self.assertLess(r["score"], 0.2)

    def test_an_invalid_plan_fails_validity_and_the_dry_run_without_crashing(self):
        r = lf.direction_checks('```json\n{"schema_version": 1, "brief": "b", "intent": "nope", "dimensions": {"direction": ["a"]}}\n```')
        self.assertEqual((r["checks"]["plan block"], r["checks"]["plan valid"], r["checks"]["3-10 directions"]), (True, True, False))
        self.assertIn("dry run", r["checks"])

    def test_a_record_can_be_appended_and_read_back(self):
        with tempfile.TemporaryDirectory() as d:
            p = Path(d) / "records.jsonl"
            lf.append_record(rec("anthropic/claude-sonnet-5", "direction", 0.9), p)
            self.assertEqual(len(lf.load_records(p)), 1)


class WorkerBatch(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        d = Path(self.tmp.name)
        self.briefs = {}
        for b in ("adidas", "skincare", "childrens"):
            (d / (b + ".txt")).write_text("a brief about " + b, encoding="utf-8")
            self.briefs[b] = d / (b + ".txt")
        self.replies = d / "replies"
        self.models = lf.parse_models({"data": [
            {"api_name": n, "company": "x", "name": n, "architecture": {"input": "text"}, "pricing": {"input_price": pin, "output_price": pout, "currency": "USD"}}
            for n, pin, pout in (("openai/gpt-5.6-luna", 0.2, 1.2), ("google/gemini-3-flash", 0.5, 3), ("openai/gpt-6-astra", 10, 50))]})
        for patch in (mock.patch.object(lf, "EVAL_BRIEFS", self.briefs), mock.patch.object(lf, "EVALS_FILE", d / "evals.jsonl")):
            patch.start()
            self.addCleanup(patch.stop)

    def test_the_quote_covers_every_model_brief_and_run_and_skips_replies_already_on_disk(self):
        q = lf.eval_quote(self.models, self.replies)
        self.assertEqual(len(q["calls"]), 18)
        lf.reply_path("openai/gpt-6-astra", "skincare", 1, self.replies).parent.mkdir(parents=True)
        lf.reply_path("openai/gpt-6-astra", "skincare", 1, self.replies).write_text("x", encoding="utf-8")
        q2 = lf.eval_quote(self.models, self.replies)
        self.assertEqual(len(q2["calls"]), 17)
        self.assertNotEqual(q["fingerprint"], q2["fingerprint"])
        self.assertGreater(q2["max_usd"], q2["expected_usd"])

    def test_nothing_is_sent_without_the_matching_confirmation(self):
        sent = []
        q = lf.eval_quote(self.models, self.replies)
        with self.assertRaises(PermissionError):
            lf.run_eval(q, "nope", self.models, post=lambda b: sent.append(b), replies_dir=self.replies)
        self.assertEqual(sent, [])

    def test_a_confirmed_run_saves_scores_and_records_every_reply_and_stops_at_the_budget(self):
        q = lf.eval_quote(self.models, self.replies)
        sent = []

        def post(body):
            sent.append(body["model"])
            return 200, {"choices": [{"message": {"content": "no plan"}, "finish_reason": "stop"}], "usage": {"prompt_tokens": 100, "completion_tokens": 50}}, ""

        res = lf.run_eval(q, q["fingerprint"], self.models, post=post, replies_dir=self.replies, workers=1, live_spend=lambda: None)
        self.assertEqual(len(sent), 18)
        self.assertTrue(lf.reply_path("openai/gpt-5.6-luna", "adidas", 1, self.replies).exists())
        self.assertTrue(all(r["score"] is not None for r in res))
        # a budget below the first call's ceiling sends nothing
        sent.clear()
        shutil_rmtree = __import__("shutil").rmtree
        shutil_rmtree(self.replies)
        res2 = lf.run_eval(lf.eval_quote(self.models, self.replies), lf.eval_quote(self.models, self.replies)["fingerprint"], self.models,
                           post=post, replies_dir=self.replies, workers=1, budget=0.0001, live_spend=lambda: None)
        self.assertEqual(sent, [])
        self.assertTrue(all(r["status"] == "skipped" for r in res2))


class Judge(unittest.TestCase):
    SHEET = "Direction 1 keeps the brief. Direction 3 only changes the color of the same poster."

    def scores(self, **over):
        d = {"faithfulness": 4, "distinctness": 2, "specificity": 4, "levers": 3, "honesty": 5, "overall": 3,
             "weakest": "distinctness", "quote": "Direction 3 only changes the color of the same poster."}
        d.update(over)
        return json.dumps(d)

    def test_a_valid_judgment_with_a_real_quote_is_accepted(self):
        r = lf.parse_judgment("Here you go: " + self.scores(), self.SHEET)
        self.assertEqual((r["ok"], r["quote_ok"]), (True, True))
        self.assertEqual(r["scores"]["distinctness"], 2)

    def test_an_invented_quote_is_flagged_but_the_scores_are_kept(self):
        r = lf.parse_judgment(self.scores(quote="The directions are wonderfully varied and bold."), self.SHEET)
        self.assertEqual((r["ok"], r["quote_ok"]), (True, False))

    def test_scores_out_of_range_or_missing_are_refused(self):
        for bad in (self.scores(overall=6), self.scores(honesty=0), self.scores(levers="3"), self.scores(faithfulness=True), "no json at all"):
            self.assertFalse(lf.parse_judgment(bad, self.SHEET)["ok"], bad)

    def test_a_summary_averages_per_brief_and_reports_the_spread(self):
        def row(model, brief, v, q=True):
            return {"ok": True, "model": model, "brief": brief, "scores": {k: v for k in lf.JUDGE_KEYS}, "quote_ok": q}
        rows = [row("m", "a", 5), row("m", "a", 3), row("m", "b", 4), row("m", "b", 4), {"ok": False, "model": "m", "brief": "b"}]
        s = lf.summarize_judgments(rows)["m"]
        self.assertEqual((s["n"], s["repeats"]), (2, 2))
        self.assertAlmostEqual(s["score"], 0.8)                    # mean of 5,3,4,4 over 5
        self.assertAlmostEqual(s["spread"], 0.0)                   # both briefs average 4
        self.assertEqual(s["quote_ok"], "4/4")

    def test_labels_are_neutral_and_the_order_is_fixed(self):
        with tempfile.TemporaryDirectory() as d:
            for name in ("openai__gpt-6-astra__skincare__r1.md", "claude-sonnet-5.5__adidas__r2.md", "google__gemini-3-flash__childrens__r1.md"):
                (Path(d) / name).write_text("x", encoding="utf-8")
            a, b = lf.judge_items(Path(d)), lf.judge_items(Path(d))
            self.assertEqual([i["path"].name for i in a], [i["path"].name for i in b])
            self.assertEqual([i["label"] for i in a], ["S01", "S02", "S03"])
            self.assertEqual({i["model"] for i in a}, {"openai/gpt-6-astra", "claude-sonnet-5.5", "google/gemini-3-flash"})

    def test_nothing_is_sent_without_the_matching_confirmation(self):
        models = lf.parse_models({"data": [{"api_name": lf.JUDGE_MODEL, "company": "x", "name": "j", "architecture": {"input": "text"},
                                            "pricing": {"input_price": 2, "output_price": 6, "currency": "USD"}}]})
        sent = []
        with tempfile.TemporaryDirectory() as d:
            (Path(d) / "openai__gpt-6-astra__skincare__r1.md").write_text("a reply", encoding="utf-8")
            with mock.patch.object(lf, "EVAL_BRIEFS", {"skincare": Path(d) / "openai__gpt-6-astra__skincare__r1.md"}):
                q = lf.judge_quote(models, Path(d))
                self.assertLess(q["max_usd"], 0.1)
                with self.assertRaises(PermissionError):
                    lf.run_judge(q, "nope", models, post=lambda b: sent.append(b), replies_dir=Path(d))
        self.assertEqual(sent, [])


class VisionRunner(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        d = Path(self.tmp.name)
        self.bench = d / "bench"
        self.bench.mkdir()
        labels = {"v01": {"defect": "none", "headline": "READ THE SUMMER", "extra_text": False},
                  "v02": {"defect": "typo", "headline": "READ THE SUMER", "extra_text": False}}
        (self.bench / "labels.json").write_text(json.dumps(labels), encoding="utf-8")
        for n in labels:
            (self.bench / (n + ".jpg")).write_bytes(b"\xff\xd8fake")
        self.models = lf.parse_models({"data": [
            {"api_name": n, "company": "x", "name": n, "architecture": {"input": "text+image"}, "pricing": {"input_price": pin, "output_price": pout, "currency": "USD"}}
            for n, pin, pout in (("m/cheap", 0.5, 3), ("m/pricey", 2.5, 15))]})
        self.names = ("m/cheap", "m/pricey")
        self.evals = mock.patch.object(lf, "VISION_FILE", d / "vision-evals.jsonl")
        self.evals.start()
        self.addCleanup(self.evals.stop)

    def test_the_quote_prices_every_call_and_changes_with_the_runs(self):
        q = lf.vision_quote(self.models, self.names, runs=2, bench=self.bench)
        self.assertEqual([c["calls"] for c in q["calls"]], [4, 4])               # 2 images x 2 runs each
        self.assertGreater(q["max_usd"], q["expected_usd"])
        self.assertNotEqual(q["fingerprint"], lf.vision_quote(self.models, self.names, runs=3, bench=self.bench)["fingerprint"])

    def test_nothing_is_sent_without_the_matching_confirmation(self):
        sent = []
        q = lf.vision_quote(self.models, self.names, bench=self.bench)
        with self.assertRaises(PermissionError):
            lf.run_vision(q, "nope", self.models, self.names, post=lambda b: sent.append(b), bench=self.bench, out_dir=Path(self.tmp.name) / "o")
        self.assertEqual(sent, [])

    def test_a_confirmed_run_sends_one_image_per_call_scores_and_saves_the_answers(self):
        q = lf.vision_quote(self.models, self.names, bench=self.bench)
        seen = []

        def post(body):
            seen.append(body)
            img = [p for p in body["messages"][0]["content"] if p.get("type") == "image_url"]
            self.assertEqual(len(img), 1)                                          # individual images, never a montage
            good = '{"headline_text": "READ THE SUMER", "other_lettering": null, "requirement_met": false, "evidence": "e"}'
            return 200, {"choices": [{"message": {"content": good}}], "usage": {"prompt_tokens": 1300, "completion_tokens": 100}}, ""

        out = Path(self.tmp.name) / "o"
        res = lf.run_vision(q, q["fingerprint"], self.models, self.names, post=post, bench=self.bench, out_dir=out, workers=1)
        self.assertEqual(len(seen), 8)
        sc = res["scores"]["m/cheap"][0]
        self.assertEqual(sc["recall"], 1.0)                                          # the typo image is flagged
        self.assertEqual(sc["false_alarm"], 1.0)                                     # and so is the clean one (the fake always says "not met")
        self.assertTrue((out / "m__cheap__r1.json").exists())
        self.assertEqual(len((Path(self.tmp.name) / "vision-evals.jsonl").read_text(encoding="utf-8").splitlines()), 8)

    def test_a_failed_or_unparseable_answer_is_counted_as_unanswered_and_never_retried(self):
        q = lf.vision_quote(self.models, ("m/cheap",), runs=1, bench=self.bench)
        calls = []

        def post(body):
            calls.append(1)
            return (504, {}, "HTTP 504") if len(calls) == 1 else (200, {"choices": [{"message": {"content": "I cannot tell"}}], "usage": {}}, "")

        res = lf.run_vision(q, q["fingerprint"], self.models, ("m/cheap",), post=post, bench=self.bench, out_dir=Path(self.tmp.name) / "o2", workers=1)
        self.assertEqual(len(calls), 2)
        self.assertEqual(res["scores"]["m/cheap"][0]["answered"], "0/2")

    def test_the_budget_stops_calls_before_they_are_sent(self):
        q = lf.vision_quote(self.models, self.names, bench=self.bench)
        sent = []
        res = lf.run_vision(q, q["fingerprint"], self.models, self.names, post=lambda b: sent.append(b) or (200, {}, ""), bench=self.bench,
                            out_dir=Path(self.tmp.name) / "o3", workers=1, budget=0.0001)
        self.assertEqual(sent, [])
        self.assertTrue(all(r["status"] == "skipped" for r in res["results"]))


class Cli(unittest.TestCase):
    def test_llm_advice_runs_offline_and_says_there_is_no_evidence(self):
        r = subprocess.run([sys.executable, str(SCRIPTS / "image.py"), "llm-advice", "--offline", "--model", "Claude Sonnet 5.5",
                            "--can-read-images", "yes"], capture_output=True, text=True, encoding="utf-8",
                           env={**__import__("os").environ, "PYTHONUTF8": "1"})
        self.assertEqual(r.returncode, 0, r.stderr)
        self.assertIn("ASSISTANT FIT", r.stdout)
        self.assertIn("no recommendation yet", r.stdout)

    def test_the_probe_without_confirm_only_prints_the_quote(self):
        r = subprocess.run([sys.executable, str(SCRIPTS / "image.py"), "llm-advice", "--offline", "--probe-pricing"],
                           capture_output=True, text=True, encoding="utf-8", env={**__import__("os").environ, "PYTHONUTF8": "1"})
        self.assertEqual(r.returncode, 0, r.stderr)
        self.assertIn("Fingerprint:", r.stdout)
        self.assertIn("--confirm", r.stdout)


if __name__ == "__main__":
    unittest.main()
