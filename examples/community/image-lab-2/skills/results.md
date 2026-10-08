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
If the plan has `visual_checks` (`plan.json`), **look at every generated image** and add a
Checks column to the table: for each image, which of the plan's checks it fails ("text
present", "no empty space for the headline", "the label changed"). Image models follow "no
text" and "leave space here" wording only loosely and nothing in the pipeline can see that,
so these checks are the user's only signal; name failing images plainly, and suggest
rewording or a retry for them. Do not claim a check passed unless you looked.

For a quick look at the images grouped by one dimension (for example one grid per creative direction) run
`python scripts/image.py montage --dir ./out/<name> --by direction` (free, needs Pillow); `--blind` writes one shuffled grid with neutral labels and a key file.

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

## Files an experiment folder holds

`plan.json` (the Creative Plan) · `ledger.json` (system of record: rows, batches, every
attempt with request id, cost, file) · `snapshots/` (approved execution snapshots) ·
`experiment.xlsx` (a view; never edit it from code) · `round-N/` (images, `progress.json`,
`run.json`) · `session.json` (derived export). Quick mode writes the same, minus the
plan and the workbook.
