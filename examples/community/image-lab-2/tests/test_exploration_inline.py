"""build-exploration-page.py: the one-file copy must stay small enough to publish (a cold end-to-end test made 28 MB from two images)."""
import importlib.util
import tempfile
import unittest
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
try:
    from PIL import Image
except ImportError:                                           # pragma: no cover
    Image = None


def load():
    spec = importlib.util.spec_from_file_location("build_exploration_page", ROOT / "scripts" / "build-exploration-page.py")
    mod = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(mod)
    return mod


@unittest.skipIf(Image is None, "Pillow is not installed")
class InlinePage(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.out = Path(self.tmp.name)
        (self.out / "assets").mkdir()
        import random
        rnd = random.Random(1)
        im = Image.new("RGB", (1024, 1536))
        im.putdata([(rnd.randrange(256), rnd.randrange(256), rnd.randrange(256)) for _ in range(1024 * 1536)])    # noise: a PNG of it is large
        im.save(self.out / "assets" / "a.png")
        self.png_bytes = (self.out / "assets" / "a.png").stat().st_size
        (self.out / "index.html").write_text("<html>" + "".join(f'<img src="assets/a.png">' for _ in range(8)) + "</html>", encoding="utf-8")

    def test_the_inline_copy_uses_recompressed_jpegs_and_is_much_smaller_than_raw_pngs(self):
        mod = load()
        dest = mod._write_inline(self.out)
        text = dest.read_text(encoding="utf-8")
        self.assertIn("data:image/jpeg;base64,", text)
        self.assertNotIn("data:image/png;base64,", text)
        raw_png_page = self.png_bytes * 4 / 3 * 8                                          # the same 8 references as raw base64 PNG
        self.assertLess(dest.stat().st_size, raw_png_page / 3)

    def test_the_uri_is_a_jpeg_with_the_long_side_capped(self):
        import base64
        import io
        mod = load()
        uri = mod._inline_uri(self.out / "assets" / "a.png")
        im = Image.open(io.BytesIO(base64.b64decode(uri.split(",", 1)[1])))
        self.assertEqual(im.format, "JPEG")
        self.assertLessEqual(max(im.size), mod.INLINE_MAX_SIDE)


if __name__ == "__main__":
    unittest.main()
