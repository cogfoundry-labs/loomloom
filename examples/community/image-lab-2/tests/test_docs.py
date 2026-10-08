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
        self.assertLess(len((ROOT / "references" / "quickstart.md").read_text(encoding="utf-8")), 8000)

    def test_skill_md_points_a_first_timer_at_the_quickstart(self):
        text = (ROOT / "SKILL.md").read_text(encoding="utf-8")
        self.assertIn("## Start here", text)
        self.assertIn("references/quickstart.md", text)
        self.assertLess(len(text), 16000)


if __name__ == "__main__":
    unittest.main()
