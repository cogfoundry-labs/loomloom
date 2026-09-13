# The case-study page — reference

`build-exploration-page.py` turns a session's runs into one self-contained,
shareable page. It is **step 7 of RESULTS** — always built/rebuilt, spends
nothing (the images already exist). This file is the detail; `SKILL.md` step 7
is the summary.

## Command

```
python scripts/build-exploration-page.py --session ./out --inline \
  --title "<3-5 word title for THIS round>" \
  --subject "<short noun phrase, e.g. Murree x Toronto>" \
  --invocation "<the exact message the user sent to trigger Image Lab>" \
  [--round N] [--canonical-url <where the folder will live>] [--selected <label>]
```

`--session` is the session root (`image.py run`'s `--out`), not any one
round's own folder — the script reads `<session>/session.json` and defaults
to publishing the **latest** round; pass `--round N` only to explicitly
target an earlier one. There is one page per session, at **one fixed
address**: `<session>/<slug>/`, where `<slug>` is derived from the *first*
round's `--title` and frozen from then on — a later round's `--title` changes
what's displayed on the page, never the URL it lives at.

Writes that static folder — `index.html`, every published round's images in
`assets/`, `case-study-data.json` (the data model a future PDF / social-card
renderer reads). `--inline` also writes `index.inline.html`: one
self-contained file with every round's images embedded. **Publish
`index.inline.html` as an artifact** for the user's live URL. The result's
stderr prints its size; if it is over ~15 MB, recompress the PNGs to JPEG
first.

## Flags

| Flag | Meaning |
|---|---|
| `--session` | the session root (same value as `run`'s `--out`); required |
| `--round` | which round `--title`/`--subject`/etc describe (default: the latest round in `<session>/session.json`) |
| `--title` | a short name for *this round* — becomes the page's headline when this round is the one shown |
| `--subject` | the page composes the tagline *"One brief. N models. M ways to see {subject}."* from the run's real numbers |
| `--invocation` | the user's actual triggering message, **verbatim** — shown as-is in the Prompt block; omitted → `<prompt>\n\nuse Image Lab` |
| `--slug` | only takes effect on the very first round ever published for this session (sets the permanent address); ignored with a note on every later round |
| `--canonical-url` | the URL the folder will live at (GitHub Pages, etc). Makes `og:image` absolute (link unfurls) and points the topbar Share cluster at that URL. Omit for the artifact flow — the artifact URL isn't known until after publish; `location.href` covers it |
| `--summary` | optional; overrides the auto tagline for this round |
| `--selected` | omit on the first build for a round (the user hasn't picked yet). When they name a favourite, re-run **with** `--selected <label>`: its thumbnail gets a ✓ badge and becomes the initial hero for that round — the creator's pick, never "best" |

## The page

redesign-lab's case-study house style — Source Serif 4 body, Arial-Black
uppercase headings, IBM Plex Mono labels, hard edges, loomloom green, 3-state
dark mode. **Every section has the same shape**: a mono eyebrow, an Arial-Black
headline, one lead line, then the body.

- **topbar** — wordmark + a **Share** cluster: `Copy link` + one-click `X` /
  `LinkedIn` (hrefs built from the canonical URL + the OG tags, which describe
  whichever round was the *latest* at build time)
- **hero** — `AI-generated · not a benchmark` badge, the tagline, a **model
  legend** (`GPT Image 2 ×3` chips, each linked to its cogfoundry.ai page), the
  stats line
- **round nav** (once a second round exists) — a strip directly above the
  Gallery section: `← VIEW ROUND N` · `ROUND N / M` · `VIEW ROUND N →`, naming
  the specific adjacent round each side switches to and wrapping at both ends
  (round 1's "prev" is the last round). Lives at the point of use rather than
  in the topbar so it reads as navigation, not a utility action
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
