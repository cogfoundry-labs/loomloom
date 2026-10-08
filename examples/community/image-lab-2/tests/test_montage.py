"""scripts/montage.py: labelled grids of finished images (needs Pillow; skipped without it)."""
import json
import sys
import tempfile
import unittest
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent.parent / "scripts"))
import montage as mt  # noqa: E402

try:
    from PIL import Image
except ImportError:                                   # pragma: no cover
    Image = None


@unittest.skipIf(Image is None, "Pillow is not installed")
class MontageTests(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.exp = Path(self.tmp.name)
        (self.exp / "round-1").mkdir()
        rows, attempts = [], []
        for i, (direction, color) in enumerate((("Alpha", "red"), ("Alpha", "blue"), ("Beta", "green")), 1):
            rid = f"r{i:03d}"
            Image.new("RGB", (80, 100), color).save(self.exp / "round-1" / f"{rid}.png")
            params = {"direction": direction, "mood": "calm"}
            rows.append({"id": rid, "params": params, "model": "m", "take": 1, "reference": None, "selected": True, "status": "Completed", "notes": "", "extras": {}})
            attempts.append({"batch": 1, "row_id": rid, "sample": 1, "sample_id": rid, "params": params, "model": "m", "status": "Completed",
                             "file": f"round-1/{rid}.png"})
        attempts.append({"batch": 1, "row_id": "r003", "sample": 2, "sample_id": "r003-2", "params": {}, "model": "m", "status": "Failed", "file": None})
        attempts.append({"batch": 1, "row_id": "r003", "sample": 3, "sample_id": "r003-3", "params": {}, "model": "m", "status": "Completed", "file": "../outside.png"})
        (self.exp / "ledger.json").write_text(json.dumps({
            "schema_version": 1, "experiment": {"brief": "b", "intent": "social post", "created_at": "2026-10-08T00:00:00", "next_row": 4,
                                                "dimensions": ["direction", "mood"], "main_workbook_rows": []},
            "rows": rows, "batches": [], "attempts": attempts}), encoding="utf-8")

    def test_one_grid_per_value_and_only_safe_finished_images(self):
        made = mt.build(self.exp, by="direction", thumb=60, cols=2)
        self.assertEqual(sorted(p.name for p in made), ["direction-1-Alpha.jpg", "direction-2-Beta.jpg"])
        self.assertTrue(all(p.stat().st_size > 0 for p in made))

    def test_blind_mode_writes_one_grid_and_a_key_with_neutral_labels(self):
        grid, key = mt.build(self.exp, blind=True, thumb=60)
        data = json.loads(key.read_text(encoding="utf-8"))
        self.assertEqual(sorted(data), ["S01", "S02", "S03"])
        self.assertEqual({v["id"] for v in data.values()}, {"r001", "r002", "r003"})
        self.assertEqual([k for k in mt.build(self.exp, blind=True, thumb=60)][1].read_text(encoding="utf-8"), key.read_text(encoding="utf-8"))   # fixed order

    def test_an_unknown_dimension_and_an_empty_experiment_are_refused(self):
        with self.assertRaises(mt.MontageError):
            mt.build(self.exp, by="nope")
        empty = Path(self.tmp.name) / "empty"
        empty.mkdir()
        (empty / "ledger.json").write_text(json.dumps({"schema_version": 1, "experiment": {"brief": "b", "intent": "x", "created_at": "t", "next_row": 1,
                                                       "dimensions": ["direction"], "main_workbook_rows": []}, "rows": [], "batches": [], "attempts": []}),
                                           encoding="utf-8")
        with self.assertRaises(mt.MontageError):
            mt.build(empty, by="direction")


if __name__ == "__main__":
    unittest.main()
