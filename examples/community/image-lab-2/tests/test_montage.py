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
        self.assertEqual(sorted(p.name for p in made), ["direction-Alpha.jpg", "direction-Beta.jpg"])
        self.assertTrue(all(p.stat().st_size > 0 for p in made))

    def set_directions(self, mapping):
        led = json.loads((self.exp / "ledger.json").read_text(encoding="utf-8"))
        for row in led["rows"]:
            row["params"]["direction"] = mapping[row["id"]]
        (self.exp / "ledger.json").write_text(json.dumps(led), encoding="utf-8")

    def test_groups_come_in_natural_order_and_files_are_named_by_the_value(self):
        self.set_directions({"r001": "10 Ten", "r002": "2 Two", "r003": "2 Two"})
        made = mt.build(self.exp, by="direction", thumb=60, cols=2)
        self.assertEqual([p.name for p in made], ["direction-2-Two.jpg", "direction-10-Ten.jpg"])        # 2 before 10, and the name carries the direction's own number

    def test_two_values_that_would_share_a_file_name_still_get_two_files(self):
        self.set_directions({"r001": "A b", "r002": "A-b", "r003": "A-b"})
        made = mt.build(self.exp, by="direction", thumb=60, cols=2)
        self.assertEqual(len({p.name for p in made}), 2)

    def test_readable_makes_bigger_tiles(self):
        with Image.open(mt.build(self.exp, by="direction", thumb=60, cols=6, out_dir=self.exp / "m1")[0]) as small,                 Image.open(mt.build(self.exp, by="direction", readable=True, out_dir=self.exp / "m2")[0]) as big:
            self.assertGreater(big.width, small.width * 4)
        self.assertEqual(mt.READABLE, {"cols": 3, "thumb": 576})

    def test_a_tile_shows_its_verdict_when_the_row_has_one(self):
        from unittest import mock
        led = json.loads((self.exp / "ledger.json").read_text(encoding="utf-8"))
        led["rows"][0]["review"] = {"checks": [{"n": 1, "verdict": "fail", "reason": "x"}], "note": ""}
        (self.exp / "ledger.json").write_text(json.dumps(led), encoding="utf-8")
        seen = []
        real = mt._grid
        with mock.patch.object(mt, "_grid", lambda items, labels, *a, **k: (seen.append(labels), real(items, labels, *a, **k))[1]):
            mt.build(self.exp, by="direction", thumb=60, cols=2)
        self.assertIn(["r001 fail 1", "r002"], seen)

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
