# Image Lab 2: case studies

Case studies from real Image Lab 2 runs. Each folder is a self-contained static page, in the same design as the
[Image Lab 1 case studies](../../image-lab/case-studies/): relative asset paths, no build step to view it.

```
case-studies/<slug>/
  index.html             the page (styles, gallery, lightbox and direction switcher inline)
  case-study-data.json   the data model: one object per direction, with every image, its settings and its verdict
  assets/d<N>-alternative-<NN>.jpg   the generated images, JPEG q95, full resolution (progressive)
```

Differences from the Image Lab 1 page: the switcher steps through the **creative directions** of one run instead of rounds,
each direction has a short description block (what it gives up, how it turned out), and each image has a one-line verdict
from checking it at full size against the plan's visual checks. The page also carries a "What to know" caveats section.

These are not produced by the skill on every run; a case study here is a maintainer picking one run worth showing. Build one from a finished experiment with
`python scripts/image.py case-study --dir <experiment> --out case-studies/<slug> --meta <meta.json>` (see [`references/case-study/README.md`](../references/case-study/README.md)); the meta, prompt and
verdict files of each study sit beside its folder (for example `woolworths-editorial-catalogue.meta.json`, `.prompt.txt`, `.verdicts.json`).
Image Lab 2 is not published yet, so nothing links to these pages.

## Index

| Slug | Brief | Run |
|---|---|---|
| [`woolworths-editorial-catalogue`](./woolworths-editorial-catalogue/) | A Woolworths editorial catalogue cover (4:5): the trolley treated like a luxury car | Creative Direction, 5 directions (one wildcard), 35 images on GPT Image 2.5 Sunburst, $0.6518 |
| [`adidas-adizero-creative-directions`](./adidas-adizero-creative-directions/) | An adidas Adizero running campaign poster (4:5): a youth-oriented digital collage, headline RUN YOUR FAST. | Creative Direction, 5 directions (one wildcard), 30 images on GPT Image 2.5 Sunburst, $0.4424 |
