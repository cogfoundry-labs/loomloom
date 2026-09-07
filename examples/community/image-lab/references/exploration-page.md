# The exploration page — reference

`build-exploration-page.py` turns one finished run into a self-contained,
shareable page. It is **step 7 of RESULTS** — always built, spends nothing (the
images already exist). This file is the detail; `SKILL.md` step 7 is the summary.

## Command

```
python scripts/build-exploration-page.py --from ./out --inline \
  --title "<3-5 word title you propose>" \
  --subject "<short noun phrase, e.g. Murree x Toronto>" \
  --invocation "<the exact message the user sent to trigger Image Lab>" \
  [--canonical-url <where the folder will live>] [--selected <label>]
```

Writes a static folder `./out/<slug>/` — `index.html`, the images in `assets/`,
`exploration.json` (the data model a future PDF / social-card renderer reads).
`--inline` also writes `./out/<slug>/index.inline.html`: one self-contained file.
**Publish `index.inline.html` as an artifact** for the user's live URL. The
result's stderr prints its size; if it is over ~15 MB, recompress the PNGs to
JPEG first.

## Flags

| Flag | Meaning |
|---|---|
| `--title` | a short name for the exploration |
| `--subject` | the page composes the tagline *"One brief. N models. M ways to see {subject}."* from the run's real numbers |
| `--invocation` | the user's actual triggering message, **verbatim** — shown as-is in the Prompt block; omitted → `<prompt>\n\nuse Image Lab` |
| `--canonical-url` | the URL the folder will live at (GitHub Pages, etc). Makes `og:image` absolute (link unfurls) and points the topbar Share cluster at that URL. Omit for the artifact flow — the artifact URL isn't known until after publish; `location.href` covers it |
| `--summary` | optional; overrides the auto tagline |
| `--selected` | omit on the first build (the user hasn't picked yet). When they name a favourite, re-run **with** `--selected <label>`: its thumbnail gets a ✓ badge and becomes the initial hero — the creator's pick, never "best" |

## The page

redesign-lab's case-study house style — Source Serif 4 body, Arial-Black
uppercase headings, IBM Plex Mono labels, hard edges, loomloom green, 3-state
dark mode. **Every section has the same shape**: a mono eyebrow, an Arial-Black
headline, one lead line, then the body.

- **topbar** — wordmark + a **Share** cluster: `Copy link` + one-click `X` /
  `LinkedIn` (hrefs built from the canonical URL + the OG tags)
- **hero** — `AI-generated · not a benchmark` badge, the tagline, a **model
  legend** (`GPT Image 2 ×3` chips, each linked to its cogfoundry.ai page), the
  stats line
- **Gallery / Pick your image** — framed hero + letter-badged thumbnails (`A ·
  GPT Image 2` — candidates, not steps) + live meta + `N / 8` counter; click a
  thumbnail to swap, click the hero for a minimal full-screen lightbox (`← →`,
  `Esc`, `Copy link`, `Open raw ↗`). Each alternative has a **deep link** — the
  page reads `…/index.html#E` on load and selects alternative E, updates the
  hash as you browse; a "Copy link to alternative N" button copies it
- **Details / The run** — a hairline facts grid (intent · models · candidates ·
  size · actual cost · date)
- **Reuse / The prompt** — the exact run prompt in italic + a "→ use Image Lab"
  trigger + Copy button + a 3-step **"how to use this"** (copy → paste into an
  agent with the skill → run as-is or swap details to make it yours)
- **Provenance / How this was made** — every model and tool, linked, with the
  run's real cost
- **Roadmap / From image exploration to AI work** — the v0.4 loomloom workflow
- **Your turn / Bring your own prompt** — a CTA with the `npx skills add …` line
- Open Graph / Twitter-card meta so a pasted link unfurls with the image

With JS off the hero is still a plain link to the full-resolution file. Give the
user the folder path (GitHub-Pages-ready) and publish `index.inline.html` as an
artifact so they have a live URL to pick from.

## Curated case studies

A run worth keeping goes in `case-studies/<slug>/` (committed, web-weight JPEG
assets, optionally a Remotion showcase video). See `case-studies/README.md`.
