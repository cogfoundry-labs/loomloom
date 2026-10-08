import copy
import json
import sys
import unittest
from pathlib import Path

HERE = Path(__file__).resolve().parent
sys.path.insert(0, str(HERE.parent / "scripts"))
import compile as cp  # noqa: E402
import matrix as mx  # noqa: E402

PLAN = json.loads((HERE / "fixtures" / "plan-mighty.json").read_text(encoding="utf-8"))


class Compile(unittest.TestCase):
    def test_fixture_is_a_valid_plan(self):
        self.assertEqual(mx.validate_plan(PLAN), [])

    def test_text_prompt_has_prefix_fragments_in_plan_order_and_suffix(self):
        out = cp.compile_prompt(PLAN, {"lighting": "golden hour", "camera": "eye-level front"}, "text")
        p = out["prompt"]
        self.assertTrue(p.startswith(PLAN["prompt"]["text_prefix"].rstrip(".")))
        self.assertLess(p.index("Eye-level front view"), p.index("golden-hour"))   # camera before lighting
        self.assertTrue(p.endswith("no text overlays."))
        self.assertEqual((out["custom"], out["unreliable"]), ([], []))

    def test_reference_mode_uses_the_reference_wording(self):
        text = cp.compile_prompt(PLAN, {"camera": "low angle", "lighting": "soft daylight"}, "text")["prompt"]
        ref = cp.compile_prompt(PLAN, {"camera": "low angle", "lighting": "soft daylight"}, "reference")
        self.assertIn("Extreme low angle", text)
        self.assertNotIn("Extreme low angle", ref["prompt"])
        self.assertIn("Low camera position at counter level", ref["prompt"])
        self.assertIn("do not copy the tilt", ref["prompt"])
        self.assertEqual(ref["unreliable"], [("camera", "low angle")])      # flagged in reference mode only
        self.assertEqual(cp.compile_prompt(PLAN, {"camera": "low angle"}, "text")["unreliable"], [])

    def test_value_without_a_reference_variant_falls_back_to_its_fragment(self):
        ref = cp.compile_prompt(PLAN, {"lighting": "golden hour"}, "reference")["prompt"]
        self.assertIn("golden-hour", ref)

    def test_custom_value_gets_generic_wording_and_is_flagged(self):
        out = cp.compile_prompt(PLAN, {"lighting": "rainy night"}, "text")
        self.assertIn("rainy night as the lighting", out["prompt"])
        self.assertEqual(out["custom"], [("lighting", "rainy night")])

    def test_defaults_when_the_plan_has_no_prompt_section(self):
        plan = copy.deepcopy(PLAN)
        del plan["prompt"]
        t = cp.compile_prompt(plan, {"camera": "low angle"}, "text")["prompt"]
        r = cp.compile_prompt(plan, {"camera": "low angle"}, "reference")["prompt"]
        self.assertTrue(t.startswith("the cream refill bottle"))                # from plan["fixed"]
        self.assertTrue(r.startswith("Use the reference image only to identify the product"))
        self.assertTrue(t.endswith("Realistic, high quality."))

    def test_deterministic_and_row_order_independent(self):
        a = cp.compile_prompt(PLAN, {"camera": "low angle", "lighting": "golden hour"}, "text")
        b = cp.compile_prompt(PLAN, {"lighting": "golden hour", "camera": "low angle"}, "text")
        self.assertEqual(a, b)

    def test_catalog_supplies_wording_but_the_plan_wins(self):
        plan = copy.deepcopy(PLAN)
        plan["dimensions"]["mood"] = ["calm", {"value": "bold", "fragment": "A bold mood"}]
        cat = {"mood": {"calm": {"fragment": "A calm, quiet mood"}, "bold": {"fragment": "catalog bold"}}}
        self.assertIn("A calm, quiet mood", cp.compile_prompt(plan, {"mood": "calm"}, "text", cat)["prompt"])
        self.assertIn("A bold mood", cp.compile_prompt(plan, {"mood": "bold"}, "text", cat)["prompt"])

    def test_mode_for_row(self):
        self.assertEqual(cp.mode_for_row(PLAN, {}), "reference")        # the plan has a reference
        no_ref = copy.deepcopy(PLAN)
        no_ref["references"] = []
        self.assertEqual(cp.mode_for_row(no_ref, {}), "text")
        self.assertEqual(cp.mode_for_row(no_ref, {"reference": "ref1"}), "reference")

    def test_bad_mode_is_rejected(self):
        with self.assertRaises(ValueError):
            cp.fragment(PLAN, "camera", "low angle", "video")


if __name__ == "__main__":
    unittest.main()
