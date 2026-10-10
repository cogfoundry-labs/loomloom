# Case study meta file

`python scripts/image.py case-study --dir <experiment> --out <folder> --meta <meta.json> [--force]` builds a static page from a finished experiment
(free, needs Pillow). The page's CSS and script are in [`template.html`](template.html); everything that can be derived comes from the experiment:
the images (JPEG q95, full resolution, at most 2048 px on the long side), their cost, time and size, the batches, the model, the plan's brief and the
verdicts recorded by `image.py review` (the caption under each image). You write the text that cannot be derived.

```json
{
  "slug": "woolworths-editorial-catalogue",
  "title": "Woolworths Editorial Catalogue",
  "summary": "One brief. Five creative directions. Thirty-five images from one model: what each direction gave, and what it cost.",
  "page_title": "optional: the browser title, default the title",
  "eyebrow": "optional HTML: the line above the title",
  "wordmark": "Image Lab 2",
  "group_by": "direction",
  "prompt_file": "woolworths-editorial-catalogue.prompt.txt",
  "trigger": "use Image Lab 2, and explore creative directions",
  "generated_on": "2026-10-10",
  "canonical_url": "https://example.github.io/<repo>/",
  "project_url": "https://github.com/cogfoundry-labs/loomloom/tree/main/examples/community/image-lab-2",
  "groups": {
    "1 Your brief": {"name": "Your brief", "idea": "...", "gives_up": "...", "outcome": "...", "wildcard": false}
  },
  "row_tags": {"r272": "new line"},
  "batches": {"1": {"title": "Calibration", "note": "5 planned, one per direction; 3 finished."}},
  "caveats": [{"title": "The logo is the model's approximation.", "text": "..."}],
  "how": ["optional: your own three reuse steps (HTML)"]
}
```

| Field | Needed | What it does |
|---|---|---|
| `slug`, `title`, `summary` | yes | the folder's identity, the heading and the one-line summary (also the link-preview text) |
| `group_by` | no | the dimension the switcher steps through (default `direction` when the experiment has one). One group, or `null`, gives a plain gallery |
| `groups` | no | per value of `group_by`: `name`, `idea` (what the picture is), `gives_up`, `outcome` (how it turned out, in your words, from the verdicts), `wildcard` (adds the wildcard badge) |
| `prompt` or `prompt_file` | no | the exact brief shown with a Copy button (default: the plan's `brief`, which is usually a summary, so give the user's real prompt). `prompt_file` is relative to the meta file |
| `canonical_url` | no | the deploy address: makes `og:image` and the canonical link absolute, so link previews find the picture and the Share buttons use the stable URL |
| `row_tags` | no | a short tag on a thumbnail's caption (`r272 · new line`) |
| `batches` | no | a title and note per batch number; the numbers and costs come from the ledger |
| `caveats` | no | the "What to know" list: say what a reader must know before using the images |

Rules: the page never states a verdict that is not in the ledger (an image with no review has no caption), and the builder refuses an experiment with a pictured person.
Text in the meta file is HTML-escaped on the page except `eyebrow` and `how`, which are HTML you write yourself.
Look at the page before it goes anywhere. A worked example is `case-studies/woolworths-editorial-catalogue.meta.json` (with its prompt file and the recorded verdicts in `...verdicts.json`): built from the experiment those verdicts were recorded on, it reproduces the published page byte for byte.
