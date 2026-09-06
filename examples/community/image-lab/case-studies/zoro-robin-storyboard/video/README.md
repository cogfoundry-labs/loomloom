# zoro-robin-storyboard — showcase video

A ~14 s, 9:16 [Remotion](https://remotion.dev) composition that turns this case
study into a shareable clip. Five screens:

1. **ONE BRIEF. EIGHT CHOICES.** — the hero sheet (C)
2. **3 MODELS** — all 8 generations, a 2×4 grid
3. **EXPLORE.** — one slow scroll down the alternatives
4. **PICK YOUR IMAGE.** — the seven others recede, C becomes dominant
5. **$0.5547** — 8 generations · 3 models · IMAGE LAB

Not a model benchmark — Image Lab helps you pick an *image*, not a model. There
is no "winner" badge anywhere in the video.

## Run it

```bash
npm i
npx remotion studio        # http://localhost:3000/ImageLabshowcase — edit live
npx remotion render ImageLabShowcase out/zoro-robin-showcase.mp4 --crf=28
```

`remotion.config.ts` points the public dir at `../assets`, so the video reads the
case study's real JPEGs directly — no duplicate copy in this folder.

## Where to change things

| File | What |
|---|---|
| `src/config.ts` | every timing (frames), size, text string; the hero index; the shot list |
| `src/theme.ts` | palette (from the case-study page) + the two fonts (Archivo Black, IBM Plex Mono) |
| `src/grid.ts` | the 2×4 grid math, shared by Screen 2 and Screen 4 |
| `src/ImageLabShowcase.tsx` | scene order + the `fadeUp` crossfade |
| `src/scenes/Screen1..5.tsx` | one file per screen |

The committed `../assets/zoro-robin-showcase.mp4` is a render of this project at
`--crf=28` (~5–6 MB). Re-render and replace it after changes.
