# After approval: generate, results, retry (Image Lab 2, experiment mode)

Steps 6 to 8 of experiment mode, and what an experiment folder holds. Read `../SKILL.md` first for the rules that always apply. Steps 0 to 5 (route, brief, plan, build, preflight, approve) are in `../SKILL.md`.

**6. Generate** (background, ~30 s updates as in quick mode):

```
python scripts/image.py run --dir ./out/<name> --confirm <fingerprint> \
  --progress-file ./out/<name>/progress.json
```

**7. Results.** `run` also builds `contact-sheet.html` in the experiment folder (free; rebuild any
time with `python scripts/image.py sheet --dir ./out/<name> [--rows camera --cols lighting]
[--selected r003-1]`): every image in a pivot of two dimensions so the effect of each control can be
compared at a glance, with cost, model, status and the exact prompt one click away. Tell the user
to open it. `--inline` embeds the images into one shareable file, and is refused for a person
experiment (those stay local). Images are `round-N/<row>-<k>.<ext>` (read the `file` field in `ledger.json`; Seedream models give `.jpg`). Send them captioned with the row's
dimension values (read `ledger.json`: `rows[].params`), give a table and the real cost.
`run` regenerates the workbook with Status, Images (a link) and Cost.
**Report in two parts, never mixed.**
1. **Execution result** (facts from `run` and the ledger): how many images were generated, which failed or were `Blocked`, which are `Unknown`, and the real cost against the quote.
2. **Visual checks** (only if the plan has `visual_checks`): look at **every** generated image. For each image and each check give **pass**, **fail** or **can't tell**, with a
   reason of a few words ("reads SUMER, one M"). For a text check write what you read, letter by letter, **on the full-size image or a crop of the text** (a thumbnail grid hides errors such as a lowercase i). Say plainly that this is **your reading of the image (a model's
   review), not an independent verification**; the user's own look is the check that counts. Image models follow "no text" and "leave space here" wording only loosely and
   nothing in the pipeline can see that, so name failing images and suggest rewording or a retry. Never say a check passed unless you looked at that image.
   A **subjective** requirement (feels premium, distinctive, warm) is not pass or fail: give one line on what you see and let the user judge.
   **Record your readings** in the ledger (free): `python scripts/image.py review --dir ./out/<name> --record verdicts.json --by "<your model>"`, where the file is
   `{"r016": {"checks": {"1": "pass", "2": {"verdict": "fail", "reason": "reads SUMER"}}, "note": "one line"}}`. Verdicts are `pass`, `fail`, `cant_tell` and `na` (the check does not apply to
   this image: it names another direction or compares several images). They show in the workbook's Review column, label the montage tiles and become the captions of a case study. A check you
   leave out is recorded as can't tell.
   **Or have a vision model read the images** (about $0.002 per image on the default model, so a few cents for a batch; it never generates or changes an image):
   `python scripts/image.py review --dir ./out/<name> [--rows r001,r002] [--only-unreviewed]` prints a quote with a fingerprint; after the user approves exactly that
   quote, run it again with `--confirm <fingerprint>`. It is a separate approval from the generation. A call that fails is reported and never retried. Its verdicts are marked
   `source: model`: say so. Treat it as a first pass, not a verification: on a live test it read a garbled small-type line as correct and failed a check that did not apply to the image, so
   look at every failure and every doubtful image yourself, and phrase each plan check so it can be judged from one image.

For a quick look at the images grouped by one dimension (for example one grid per creative direction) run
`python scripts/image.py montage --dir ./out/<name> --by direction` (free, needs Pillow). Tiles show a recorded verdict next to the row id (`r054 fail 1`). The default tiles are too small
to read small type: add `--readable` (3 across, 576 px) for that. `--blind` writes one shuffled grid with neutral labels and a key file.

**8. Retry and next batch.**
- `python scripts/image.py retry --dir ./out/<name>` preflights the `Failed`/`Partial`
  rows for their **missing samples only**, with its own fingerprint and approval, then
  `run --confirm` as above. It never asks again for a sample that is Completed, still in
  flight, `Blocked` (the model refused: offer to change the wording instead), `Unknown`
  (unless `--include-unknown`), or billed but not downloaded (use `recover`, free). A row the
  user edited after it ran is a new configuration: its earlier images are kept as they were.
- For a new batch the user edits the workbook (tick rows, change values, add rows) and you
  preflight again. A row that finished stays ticked unless they untick it.
- If a result says "image generated but download failed", the image was billed but not saved:
  run `python scripts/image.py recover --dir ./out/<name>` (free, it re-fetches from the
  recorded URL; do this soon, the URLs expire). Never resubmit to fix that.
- `python scripts/image.py refresh --dir ./out/<name>` re-merges their edits and
  regenerates the workbook (useful after they edited a saved copy).

**9. Share it (optional).** `python scripts/image.py case-study --dir ./out/<name> --out ./case-studies/<slug> --meta <meta.json>` (free, needs Pillow) builds a static page in the
Image Lab case-study design from the finished experiment: every image with its cost and time, the batches, and the recorded verdicts as captions, with a switcher through the
directions. You write only the meta file (title, summary, a short description of each direction, caveats); `references/case-study/README.md` says how. It refuses an experiment with a
pictured person. Look at the page before it goes anywhere; publishing it is the user's call.

## Files an experiment folder holds

`plan.json` (the Creative Plan) · `ledger.json` (system of record: rows, batches, every
attempt with request id, cost, file) · `snapshots/` (approved execution snapshots) ·
`experiment.xlsx` (a view; never edit it from code) · `round-N/` (images, `progress.json`,
`run.json`) · `reviews/` (the audit file of each model review: answers, usage, cost) · `session.json` (derived export). Quick mode writes the same, minus the
plan and the workbook.
