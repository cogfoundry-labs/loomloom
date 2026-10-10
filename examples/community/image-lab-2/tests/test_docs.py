"""The docs an agent reads first must point at files that exist (links and paths), and the quickstart's examples must be valid."""
import re
import sys
import unittest
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
sys.path.insert(0, str(ROOT / "scripts"))
import matrix as mx  # noqa: E402

DOCS = ["README.md", "SKILL.md", "references/quickstart.md", "skills/plan.md", "skills/direction.md", "skills/quick.md", "skills/results.md",
        "references/plan-schema.md"]
def flat(rel: str) -> str:
    """A doc's text with every run of whitespace collapsed, so a test checks the words and not where a line happens to wrap."""
    return re.sub(r"\s+", " ", (ROOT / rel).read_text(encoding="utf-8"))


PATH = re.compile(r"`((?:references|skills|scripts|docs|tests)/[A-Za-z0-9_./\-]+\.[A-Za-z0-9]+)`")
LINK = re.compile(r"\]\(([^)\s#]+)(?:#[^)]*)?\)")


class DocLinks(unittest.TestCase):
    def test_every_relative_markdown_link_resolves(self):
        bad = []
        for rel in DOCS:
            f = ROOT / rel
            for m in LINK.finditer(f.read_text(encoding="utf-8")):
                target = m.group(1)
                if target.startswith(("http://", "https://", "mailto:")):
                    continue
                if not (f.parent / target).resolve().exists():
                    bad.append((rel, target))
        self.assertEqual(bad, [])

    def test_every_path_named_in_backticks_exists(self):
        bad = []
        for rel in DOCS:
            for m in PATH.finditer((ROOT / rel).read_text(encoding="utf-8")):
                p = m.group(1)
                if "<" in p or "*" in p:
                    continue
                if not (ROOT / p).exists() and not (ROOT / rel).parent.joinpath(p).exists():
                    bad.append((rel, p))
        self.assertEqual(bad, [])


class Quickstart(unittest.TestCase):
    def test_it_names_both_examples_and_they_are_valid_plans(self):
        text = (ROOT / "references" / "quickstart.md").read_text(encoding="utf-8")
        for name in ("variation-plan.json", "direction-plan.json"):
            self.assertIn(name, text)
            self.assertEqual(mx.validate_plan(mx.load_plan(ROOT / "references" / "examples" / name)), [])

    def test_it_stays_short_enough_to_read_in_one_go(self):
        self.assertLess(len((ROOT / "references" / "quickstart.md").read_text(encoding="utf-8")), 9300)

    def test_skill_md_points_a_first_timer_at_the_quickstart(self):
        text = (ROOT / "SKILL.md").read_text(encoding="utf-8")
        self.assertIn("## Start here", text)
        self.assertIn("references/quickstart.md", text)
        self.assertLess(len(text), 15500)


class Routing(unittest.TestCase):
    def test_the_route_cases_cover_every_route_and_name_their_reason(self):
        import planner_eval as pe
        cases = pe.load_route_cases()
        self.assertGreaterEqual(len(cases), 10)
        self.assertEqual({c["route"] for c in cases.values()}, set(pe.ROUTES))
        self.assertTrue(all(c["why"] and c["request"] for c in cases.values()))

    def test_check_route_accepts_the_expected_route_in_any_casing_and_names_the_miss(self):
        import planner_eval as pe
        self.assertIsNone(pe.check_route("plain-prompt", " Quick. "))
        self.assertIn("expected quick", pe.check_route("plain-prompt", "variation"))
        self.assertIn("not one of", pe.check_route("plain-prompt", "dunno"))

    def test_the_shipped_routing_text_names_three_modes_and_the_decision_order(self):
        import planner_eval as pe
        text = pe.routing_text()
        for needle in ("**Quick**", "**Variation**", "**Creative Direction**", "first match wins", "ask once", "intent and constraints"):
            self.assertIn(needle, text)
        self.assertLess(text.index("explore creative directions"), text.index("control or compare variables"))   # exploration is checked first
        self.assertLess(text.index("ask once"), text.index("Anything else"))                                    # the question precedes the Quick fallback

    def test_quick_mode_says_what_quick_gives_at_approval(self):
        text = (ROOT / "skills" / "quick.md").read_text(encoding="utf-8")
        self.assertIn("not by composition or look", text)


class ApprovalSemantics(unittest.TestCase):
    """Each paid batch is approved once and authorizes exactly one snapshot; the docs must say so in the same words everywhere."""
    def test_the_principle_the_scope_message_and_the_two_yeses_are_in_the_skill_and_the_quickstart(self):
        skill, quick = flat("SKILL.md"), flat("references/quickstart.md")
        for needle in ("each approval authorizes exactly one execution snapshot", "A calibration batch is approved on its own", "plan confirmation", "spend approval",
                       "It does not approve anything else"):
            self.assertIn(needle, skill, needle)
        for needle in ("plan confirmation", "spend approval", "A **calibration**", "An approved snapshot never changes"):
            self.assertIn(needle, quick, needle)

    def test_the_fingerprint_coverage_the_single_spend_gate_and_the_limit_caveat_are_stated(self):
        skill, quick = flat("SKILL.md"), flat("references/quickstart.md")
        for needle in ("What the fingerprint covers", "the compiled prompt, model, size, quantity", "It does not cover file times", "The one spend gate",
                       "`retry`, `add-takes` and `recover` never spend by", "**by content**", "**not a hard cap**"):
            self.assertIn(needle, skill, needle)
        for needle in ("**only** command that spends", "**not a hard cap**", "by content, not file time"):
            self.assertIn(needle, quick, needle)

    def test_the_editing_boundary_and_the_two_part_results_report_are_written_down(self):
        skill, results, quick = flat("SKILL.md"), flat("skills/results.md"), flat("references/quickstart.md")
        self.assertIn("after the plan confirmation the user may keep editing the workbook", skill)
        self.assertIn("even dropping a single image, never alters the approved snapshot", skill)
        for needle in ("Report in two parts, never mixed", "Execution result", "**Visual checks**", "pass**, **fail** or **can't tell**",
                       "not an independent verification", "subjective", "Never say a check passed unless you looked"):
            self.assertIn(needle, results, needle)
        self.assertIn("Subjective qualities", quick)

    def test_the_end_to_end_findings_are_documented(self):
        skill, results, quick = flat("SKILL.md"), flat("skills/results.md"), flat("references/quickstart.md")
        self.assertIn("./out/<name>/plan.json", skill)                                   # where to change wording after the build
        self.assertIn("To change **wording** after the build", quick)
        self.assertIn("a thumbnail grid hides errors", results)
        self.assertIn("not a thumbnail", quick)
        import workbook
        self.assertTrue(any("Ticking starts nothing" in line for line in workbook.READ_ME))

    def test_no_doc_still_says_the_whole_experiment_is_approved_once(self):
        for rel in ("SKILL.md", "README.md", "references/quickstart.md"):
            text = (ROOT / rel).read_text(encoding="utf-8")
            self.assertNotIn("Approve once", text, rel)
            self.assertNotIn("The one confirmation", text, rel)
            self.assertNotIn("your one approval", text, rel)
            self.assertNotIn("cannot stop tasks already", text, rel)


if __name__ == "__main__":
    unittest.main()
