# Image Lab — case studies

Curated exploration pages from real Image Lab runs, committed so they can be
hosted as small standalone sites (the same way `redesign-lab/case-studies/`
works). Each folder is self-contained and GitHub-Pages-ready:

```
case-studies/<slug>/
  index.html          the exploration page (styles + gallery/lightbox script inline)
  exploration.json    the data model — a future PDF / social-card renderer reads this
  assets/alternative-01.jpg …   the generated images, JPEG q95, full resolution
```

These are **not** produced by the skill on every run — the skill publishes each
run as a private artifact (see `SKILL.md` step 7). A case study here is a
maintainer picking one run worth showing and committing it.

## Regenerate one

```bash
# 1. render the folder (stdlib only — emits the run's original PNGs)
python ../scripts/build-exploration-page.py --from <the run's --out dir> \
  --out ./<slug> \
  --title "<short title>" --subject "<noun phrase>" \
  --invocation "<the exact message that triggered Image Lab>"

# 2. pack the images for the web before committing
python pack.py ./<slug>
```

`pack.py` swaps every PNG for **progressive JPEG at quality 95** (visually
lossless for line-art + text; full resolution kept, only downscaled past
2048 px on the long edge) and rewrites the references in `index.html` +
`exploration.json`. A full 8-image run lands ~3–4 MB — the same range as
`redesign-lab/case-studies/`, versus ~13 MB as PNG. It needs Pillow and is a
maintainer tool only: nothing under `scripts/` imports it, and the skill runtime
stays standard-library only.

## Deploy one

Any static host. The pages use **relative** asset paths and relative
`og:image`, so they work from any URL without a rebuild. For a nicer link
unfurl, rebuild with `--canonical-url <the deploy URL>` so `og:image` is
absolute.

## Index

| Slug | Brief | Run |
|---|---|---|
| [`zoro-robin-storyboard`](./zoro-robin-storyboard/) | A 30-second One Piece-style storyboard sheet (Zoro × Robin) | `infographic / diagram`, count 8 → GPT Image 2 ×3 + Nano Banana Pro ×3 + Nano Banana 2 ×2, $0.5547 |
