"""scripts/vision_bench.py: the result-review benchmark (labels, parsing and mechanical scoring). Nothing here calls a model."""
import json
import sys
import tempfile
import unittest
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent.parent / "scripts"))
import vision_bench as vb  # noqa: E402

LABELS = {
    "v01": {"defect": "none", "headline": "READ THE SUMMER", "extra_text": False},
    "v02": {"defect": "none", "headline": "READ THE SUMMER", "extra_text": False},
    "v03": {"defect": "typo", "headline": "READ THE SUMER", "extra_text": False},
    "v04": {"defect": "wrong word", "headline": "REED THE SUMMER", "extra_text": False},
    "v05": {"defect": "extra text", "headline": "READ THE SUMMER", "extra_text": True},
    "v06": {"defect": "no text", "headline": None, "extra_text": False},
}


def ans(ok, head):
    return {"requirement_met": ok, "headline_text": head, "other_lettering": None}


class Scoring(unittest.TestCase):
    def test_a_perfect_reviewer_scores_one_and_reads_every_letter(self):
        a = {"v01": ans(True, "READ THE SUMMER"), "v02": ans(True, "READ THE SUMMER"), "v03": ans(False, "READ THE SUMER"),
             "v04": ans(False, "REED THE SUMMER"), "v05": ans(False, "READ THE SUMMER"), "v06": ans(False, None)}
        r = vb.score(a, LABELS)
        self.assertEqual((r["recall"], r["false_alarm"], r["balanced"], r["transcription"]), (1.0, 0.0, 1.0, 1.0))

    def test_an_autocorrecting_reviewer_is_caught(self):
        a = {"v01": ans(True, "READ THE SUMMER"), "v02": ans(True, "READ THE SUMMER"), "v03": ans(True, "READ THE SUMMER"),
             "v04": ans(True, "READ THE SUMMER"), "v05": ans(False, "READ THE SUMMER"), "v06": ans(False, None)}
        r = vb.score(a, LABELS)
        self.assertEqual(r["by_defect"], {"typo": "0/1", "wrong word": "0/1", "extra text": "1/1", "no text": "1/1"})
        self.assertEqual(r["autocorrected"], ["v03", "v04"])
        self.assertAlmostEqual(r["recall"], 0.5)

    def test_false_alarms_on_clean_images_lower_the_balanced_score(self):
        a = {"v01": ans(False, "READ THE SUMMER"), "v02": ans(True, "READ THE SUMMER"), "v03": ans(False, "x"), "v04": ans(False, "x"),
             "v05": ans(False, "x"), "v06": ans(False, None)}
        r = vb.score(a, LABELS)
        self.assertEqual((r["false_alarm"], r["recall"], r["balanced"]), (0.5, 1.0, 0.75))
        self.assertEqual(r["false_alarms"], ["v01"])

    def test_unanswered_images_are_reported_and_left_out_of_both_rates(self):
        r = vb.score({"v01": ans(True, "READ THE SUMMER"), "v03": ans(False, "READ THE SUMER"), "v02": None}, LABELS)
        self.assertEqual(r["answered"], "2/6")
        self.assertEqual((r["recall"], r["false_alarm"]), (1.0, 0.0))

    def test_case_spacing_and_punctuation_do_not_matter_for_the_transcription(self):
        r = vb.score({"v01": ans(True, "Read  the summer!"), "v03": ans(False, "READ THE SUMER.")}, LABELS)
        self.assertEqual(r["transcription"], 1.0)


class Parsing(unittest.TestCase):
    def test_json_is_found_inside_chatter_and_must_have_a_boolean_verdict(self):
        a = vb.parse_answer('Sure! {"headline_text": "READ THE SUMER", "other_lettering": null, "requirement_met": false, "evidence": "one M"} done')
        self.assertEqual((a["requirement_met"], a["headline_text"]), (False, "READ THE SUMER"))
        self.assertIsNone(vb.parse_answer('{"requirement_met": "no"}'))
        self.assertIsNone(vb.parse_answer("no json"))

    def test_a_null_headline_reads_as_none(self):
        self.assertIsNone(vb.parse_answer('{"headline_text": null, "requirement_met": false}')["headline_text"])


class Shipped(unittest.TestCase):
    def test_the_shipped_benchmark_is_balanced_and_every_image_exists(self):
        labels = vb.load_labels()
        self.assertEqual(len(labels), 16)
        self.assertEqual(sum(1 for v in labels.values() if v["defect"] == "none"), 8)
        for name in labels:
            self.assertTrue((vb.BENCH_DIR / (name + ".jpg")).exists(), name)
        self.assertEqual({v["defect"] for v in labels.values()} - {"none"}, {"typo", "wrong word", "extra text", "no text"})

    def test_build_writes_neutral_names_and_labels(self):
        try:
            from PIL import Image
        except ImportError:                                       # pragma: no cover
            self.skipTest("Pillow is not installed")
        with tempfile.TemporaryDirectory() as d:
            src = []
            for i, defect in enumerate(("none", "typo", "no text")):
                Image.new("RGB", (200, 300), "red").save(Path(d) / f"s{i}.png")
                src.append({"path": Path(d) / f"s{i}.png", "defect": defect, "headline": None if defect == "no text" else "X", "look": "l", "source": "t"})
            out = vb.build(src, Path(d) / "out")
            labels = json.loads((out / "labels.json").read_text(encoding="utf-8"))
            self.assertEqual(sorted(labels), ["v01", "v02", "v03"])
            self.assertTrue(all((out / (k + ".jpg")).exists() for k in labels))


if __name__ == "__main__":
    unittest.main()
