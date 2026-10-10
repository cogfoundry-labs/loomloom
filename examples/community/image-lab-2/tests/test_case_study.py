"""scripts/case_study.py: a shareable page built from a finished experiment (needs Pillow; skipped without it)."""
import json
import re
import shutil
import sys
import tempfile
import unittest
from pathlib import Path

HERE = Path(__file__).resolve().parent
sys.path.insert(0, str(HERE.parent / "scripts"))
sys.path.insert(0, str(HERE))
import case_study as cs  # noqa: E402
import ledger as lg  # noqa: E402
import review as rv  # noqa: E402
from test_visual_review import PLAN, finish, make_experiment  # noqa: E402

try:
    from PIL import Image
except ImportError:                                   # pragma: no cover
    Image = None

META = {"slug": "demo", "title": "Demo Poster", "summary": "One brief. Two directions. A few images.",
        "groups": {"1 Alpha": {"name": "Alpha", "idea": "The first idea.", "gives_up": "Nothing.", "outcome": "Worked."},
                   "2 Beta": {"name": "Beta", "idea": "The second idea.", "wildcard": True}},
        "batches": {"1": {"title": "First look", "note": "four images"}}, "row_tags": {},
        "caveats": [{"title": "A caveat", "text": "Read this <first> & then use it."}], "trigger": "use Image Lab 2",
        "prompt": "A poster, 4:5 \"vertical\" <b>bold</b> & more"}


def rounds_of(page: str) -> list:
    i = page.index("var ROUNDS = ") + len("var ROUNDS = ")
    j = page.index(";\n  function initRoundNav")
    return json.loads(page[i:j])


@unittest.skipIf(Image is None, "Pillow is not installed")
class CaseStudyTests(unittest.TestCase):
    def setUp(self):
        self.tmp = Path(tempfile.mkdtemp())
        self.addCleanup(shutil.rmtree, self.tmp, ignore_errors=True)
        self.exp = make_experiment(self.tmp)
        self.ids = [r["id"] for r in lg.load(self.exp / "ledger.json")["rows"]]
        finish(self.exp, self.ids)
        for rid in self.ids:                                     # real, distinct images so the JPEG step has something to read
            Image.new("RGB", (96, 120), (40 * int(rid[1:]) % 255, 90, 160)).save(self.exp / "round-1" / f"{rid}.png")
        rv.record(self.exp, {self.ids[0]: {"checks": {"1": "pass", "2": {"verdict": "fail", "reason": "no books"}}, "note": "No books anywhere."},
                             self.ids[1]: {"checks": {"1": "pass", "2": "pass"}}}, reviewer="Claude")
        self.meta = self.tmp / "meta.json"
        self.meta.write_text(json.dumps(META), encoding="utf-8")
        self.out = self.tmp / "site"

    def build(self, **kw):
        return cs.build(self.exp, self.out, self.meta, **kw)

    def test_the_folder_has_the_page_the_data_and_one_jpeg_per_image(self):
        r = self.build()
        self.assertEqual((r["images"], r["groups"]), (4, 2))
        self.assertEqual(sorted(p.name for p in self.out.iterdir()), ["assets", "case-study-data.json", "index.html"])
        jpgs = sorted(p.name for p in (self.out / "assets").iterdir())
        self.assertEqual(len(jpgs), 4)
        self.assertTrue(all(n.endswith(".jpg") and n.startswith("d") for n in jpgs))
        with Image.open(self.out / "assets" / jpgs[0]) as im:
            self.assertEqual(im.format, "JPEG")

    def test_every_placeholder_is_filled_and_the_switcher_has_one_round_per_group(self):
        self.build()
        page = (self.out / "index.html").read_text(encoding="utf-8")
        self.assertNotIn("{{", page)
        rounds = rounds_of(page)
        self.assertEqual(len(rounds), 2)
        self.assertIn("DIRECTION 1 / 2", rounds[0]["body"])
        self.assertIn("VIEW DIRECTION 2", rounds[0]["body"])
        self.assertIn('data-round-index="1"', rounds[0]["body"])
        self.assertIn("DIRECTION 2 / 2", rounds[1]["body"])
        self.assertIn("wildcard", rounds[1]["body"])
        self.assertNotIn("wildcard", rounds[0]["body"].replace("wildcard=", ""))
        self.assertTrue(page.count('id="round-body"') == 1)
        self.assertIn(rounds[0]["body"], page)                                     # the first round is what a visitor sees before any script runs

    def test_the_captions_come_from_the_recorded_verdicts(self):
        self.build()
        data = json.loads((self.out / "case-study-data.json").read_text(encoding="utf-8"))
        alts = [a for g in data for a in g["alternatives"]]
        by_row = {a["row"]: a for a in alts}
        self.assertEqual(by_row[self.ids[0]]["note"], "No books anywhere.")
        self.assertEqual(by_row[self.ids[0]]["review"]["checks"][1]["verdict"], "fail")
        self.assertEqual(by_row[self.ids[1]]["note"], "All checks passed.")
        self.assertEqual(by_row[self.ids[2]]["note"], "")                          # an unreviewed image has no caption, not an invented one
        self.assertNotIn("review", by_row[self.ids[2]])
        page = (self.out / "index.html").read_text(encoding="utf-8")
        self.assertIn("No books anywhere.", page)

    def test_the_facts_are_derived_from_the_experiment(self):
        r = self.build()
        data = json.loads((self.out / "case-study-data.json").read_text(encoding="utf-8"))
        self.assertAlmostEqual(r["cost"], 4 * 0.0186, places=4)
        self.assertEqual(data[0]["stats"]["images_total"], 4)
        self.assertEqual(data[0]["stats"]["sizes"], ["1152x1440"])
        self.assertEqual(data[0]["batches"][0]["title"], "First look")
        self.assertEqual(data[0]["batches"][0]["images"], 4)
        page = (self.out / "index.html").read_text(encoding="utf-8")
        self.assertIn("Batch 1 · First look", page)
        self.assertIn("4 images &middot; 2 directions &middot; 1 model &middot; $0.0744 total", page)
        self.assertIn("medium quality", page)

    def test_user_text_is_escaped_in_the_page_and_the_prompt_is_exact_in_the_copy_source(self):
        self.build()
        page = (self.out / "index.html").read_text(encoding="utf-8")
        self.assertNotIn("<b>bold</b>", page)
        self.assertIn("&lt;b&gt;bold&lt;/b&gt;", page)
        self.assertIn("Read this &lt;first&gt; &amp; then use it.", page)
        copy = re.search(r'<span id="copy-src" hidden>(.*?)</span>', page, re.S).group(1)
        import html
        self.assertEqual(html.unescape(copy), META["prompt"])

    def test_a_script_end_tag_in_the_text_cannot_break_out_of_the_page(self):
        meta = dict(META, caveats=[{"title": "x", "text": "evil </script><script>alert(1)</script>"}])
        self.meta.write_text(json.dumps(meta), encoding="utf-8")
        self.build()
        page = (self.out / "index.html").read_text(encoding="utf-8")
        script = page[page.index("var ROUNDS = "):]
        self.assertEqual(len(re.findall(r"(?i)</script", script)), 1)              # only the real end of the script element
        body = rounds_of(page)[0]["body"]                                          # the text is HTML-escaped inside the round too
        self.assertNotIn("<script>alert(1)", body)
        self.assertIn("&lt;script&gt;alert(1)", body)

    def test_a_canonical_url_makes_the_preview_tags_absolute(self):
        self.meta.write_text(json.dumps(dict(META, canonical_url="https://example.github.io/demo/")), encoding="utf-8")
        self.build()
        page = (self.out / "index.html").read_text(encoding="utf-8")
        self.assertIn('<meta property="og:image" content="https://example.github.io/demo/assets/d1-alternative-01.jpg">', page)
        self.assertIn('<link rel="canonical" href="https://example.github.io/demo/">', page)
        self.meta.write_text(json.dumps(META), encoding="utf-8")                  # without a canonical URL the paths stay relative
        self.build(force=True)
        plain = (self.out / "index.html").read_text(encoding="utf-8")
        self.assertIn('<meta property="og:image" content="assets/d1-alternative-01.jpg">', plain)

    def test_an_existing_folder_is_not_replaced_without_force(self):
        self.build()
        with self.assertRaisesRegex(cs.CaseStudyError, "--force"):
            self.build()
        self.build(force=True)

    def test_force_never_deletes_a_folder_that_is_not_a_case_study_or_that_holds_the_experiment(self):
        other = self.tmp / "precious"
        other.mkdir()
        (other / "notes.txt").write_text("keep me", encoding="utf-8")
        with self.assertRaisesRegex(cs.CaseStudyError, "not a case-study folder"):
            cs.build(self.exp, other, self.meta, force=True)
        self.assertEqual((other / "notes.txt").read_text(encoding="utf-8"), "keep me")
        with self.assertRaisesRegex(cs.CaseStudyError, "experiment folder"):
            cs.build(self.exp, self.tmp, self.meta, force=True)                       # the parent of the experiment
        self.assertTrue((self.exp / "ledger.json").exists())

    def test_an_experiment_with_one_group_gets_a_plain_gallery(self):
        self.meta.write_text(json.dumps(dict(META, group_by=None)), encoding="utf-8")
        r = self.build()
        self.assertEqual(r["groups"], 1)
        page = (self.out / "index.html").read_text(encoding="utf-8")
        body = rounds_of(page)[0]["body"]
        self.assertNotIn("round-nav", body)
        self.assertNotIn('class="direction"', body)
        self.assertIn("alternative-01.jpg", body)

    def test_bad_inputs_are_clear_errors(self):
        for bad, msg in (({"slug": "x", "title": "t"}, "needs `summary`"), ({**META, "group_by": "nope"}, "not a dimension")):
            self.meta.write_text(json.dumps(bad), encoding="utf-8")
            with self.assertRaisesRegex(cs.CaseStudyError, msg):
                self.build(force=True)
        with self.assertRaises(cs.CaseStudyError):
            cs.build(self.tmp / "nowhere", self.out, self.meta)

    def test_a_pictured_person_is_never_published(self):
        plan = json.loads((self.exp / "plan.json").read_text(encoding="utf-8"))
        plan["references"] = [{"id": "ref1", "file": "refs/p.png", "role": "person", "contains_person": True}]
        (self.exp / "plan.json").write_text(json.dumps(plan), encoding="utf-8")
        self.meta.write_text(json.dumps(META), encoding="utf-8")
        with self.assertRaisesRegex(cs.CaseStudyError, "person"):
            self.build()

    def test_the_template_matches_the_published_page_structure(self):
        tpl = cs.TEMPLATE.read_text(encoding="utf-8")
        self.assertEqual(sorted(set(re.findall(r"\{\{[A-Z_]+\}\}", tpl))),
                         ["{{CANONICAL}}", "{{DESCRIPTION}}", "{{OG_IMAGE}}", "{{PAGE_TITLE}}", "{{ROUNDS_JSON}}", "{{ROUND_BODY}}", "{{ROUND_HEAD}}", "{{WORDMARK}}"])
        self.assertIn("function renderRound", tpl)
        self.assertIn('id="round-body"', tpl)


if __name__ == "__main__":
    unittest.main()
