# Quick mode (Image Lab 2)

One finished prompt, spread across the 2 to 3 best-fit image models. Read `../SKILL.md` first for the rules that always apply (one approval, honest price, spend limits).

```
PLAN -> QUOTE -> APPROVE -> GENERATE -> RESULTS (+ gallery page) -> adjust?
```

**1. Capture** the final prompt (from the conversation or a file). No prompt: ask.
If they want to start from a photo, use experiment mode instead.

**2. Classify** the intent, your one judgement call; the user can correct it. One of:
`launch / announcement image` · `profile / avatar` · `social post` ·
`blog hero / article cover` · `poster / flyer` · `infographic / diagram` ·
`illustration / concept art` · `3D render / isometric illustration` ·
`product / e-commerce shot` · `generic` (only if nothing fits).

Map count: "just one" 1 · "a couple / two" 2 · nothing said **4** · "lots / deep dive" 8.
Only 1, 2, 4, 8 exist (5 becomes 4; 6+ becomes 8).

**3. Quote** (writes `ledger.json` and a snapshot, no workbook, spends nothing):

```
python scripts/image.py quick --prompt "<final prompt>" --intent "<intent>" --count <N> --out ./out
```

Add `--models "<id>[,<id>]"` only if the user named models; the Advisor never replaces a
model they asked for. Reuse the same `--out` for a further round of the same subject; a
different subject gets a fresh `--out`. It prints each row (`r001`...), model, samples,
price, the total, and a fingerprint. For the score table use
`python scripts/image.py resolve --intent "<intent>" --count <N> --explain`.

**4. Approve:** `AskUserQuestion` with Generate / Adjust / Stop, naming the real mix and
total ("Generate 4: Sunburst x2 + Nano Banana 2 x2, about $0.14?"). Adjust goes back to 3.
Say what Quick gives, in one line: the options differ by **model and random seed, not by composition or look**, because the prompt is used verbatim. If the user wanted
deliberate differences, offer Variation ("vary the lighting and camera") or Creative Direction ("explore different ideas") instead of generating.

**5. Generate**, in the background, with a progress file:

```
python scripts/image.py run --dir ./out --confirm <fingerprint> --progress-file ./out/progress.json
```

Every ~30 s read the progress file and post a short update (a plain text tree):
`phase` (`generating` -> `done` or `paused`), `done`, `failed`, `unknown`, `total`,
`actual_usd_so_far`, and `samples[]` with `id`, `model`, `status`, `progress`, `cost_usd`.

**6. Results.** `run` prints the result and writes `./out/round-N/run.json`; images are
`./out/round-N/<id>.<ext>` (`r001-1`, `r001-2`, ...; the extension is usually `.png`, but Seedream models
return `.jpg`, so take the file name from `run.json` or the ledger's `file` field, never assume `.png`). Send each image with
`SendUserFile`, captioned with id and model; show a table (id, model, time, cost,
size) and `Actual total vs estimated`. Name any failed, blocked or unknown sample plainly.
Exit code 0 = all done, 1 = something failed/unknown/unfinished, 2 = refused or busy (nothing
spent). If it says samples are unfinished, run the same command again: it resumes
polling and never resubmits.

**7. Gallery page** (free; always build it):

```
python scripts/build-exploration-page.py --session ./out --inline \
  --title "<3-5 words>" --subject "<short noun phrase>" \
  --invocation "<the user's message>" [--selected R001-2]
```

Labels on the page are the uppercase sample ids (`R001-1`). The one-file copy carries recompressed JPEGs (long side 1280; the full-size PNGs stay in `assets/`) and prints a warning if it is still over the 16 MB an artifact may weigh; then publish the folder instead. Publish `index.inline.html`
as an artifact. Full flags: `references/exploration-page.md`.

**8. Adjust?** Pick a favourite / adjust the prompt for another round / stop. Another
round re-enters step 3 in full (new quote, new approval).
