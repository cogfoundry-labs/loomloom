---
name: build-case-study
description: Gate 4 / Share stage. Assembles a real, interactive Case Study from a completed redesign run as a GitHub-Pages-ready folder — reusing every already-produced artifact, spending real money only on one batched narrative pass. Rev: image-generation removed (cost/reliability risk, still only decorative); loomloom install/config is deferred until this stage is actually confirmed, not required upfront. Validated end to end against a real live-site run (larstornoe.com, "Golden Case Study"; shengsuanyun.com/loomloom, this rev) before being generalized into these scripts.
---

# build-case-study

Share's real implementation, not a description of one. The Golden Case
Study run this was generalized from is documented in
`../references/model-policy.md` (the real loomloom costs).

## Prerequisite: loomloom isn't required until this exact moment

Nothing before Gate 4 touches loomloom — Gates 1-3 are entirely free and
don't need it installed at all (rev 6/7). The first real loomloom
touchpoint anywhere in this pipeline is `plan`'s template registration and
precheck, right here. So the *installation and API-key configuration* of
loomloom itself is deferred to this exact moment too, not bundled into
setting up Redesign Lab: **the first action once a human says "do the
case study now" is confirming loomloom is actually installed and
configured**, before running `plan`.

1. Check `loomloom` is on PATH (a version/doctor call) and that an API key
   resolves for it.
2. If either is missing, stop and walk the human through it right there:
   the real install command, and where to get a key
   (`console.<provider>.com/user/keys`-style — the human gets and manages
   their own key in their own environment; never type a key into a field
   yourself). Confirm once done, then proceed to `plan`.
3. If someone never reaches Gate 4, they never needed loomloom at all —
   that's the point of deferring this, not a fallback path.

## If this run built more than one page, ask which one first

`implement-design.md`'s wider-scope pass (Gate 3's "other key pages"
choice) can leave a run with several real built pages. A Case Study
features exactly **one** before/after comparison — WP1's own spec treats
it as *the* singular visual centerpiece, not a gallery — so ask which page
before calling `plan`, the same `AskUserQuestion` shape as every other
choice in this pipeline: each built page as an option (name it by what it
actually shows, e.g. "Home page — the SLA dashboard and featured-product
panel" / "/loomloom — the real comparison table and 3-step workflow"), no
"show me more" needed since the pool is just whatever this run actually
built. Whichever page is chosen decides `--before`/`--after`,
`--before-image`/`--after-image`, and `--mechanical-report`/`--a11y-report`
for `plan` below — all four must point at the *same* page's real artifacts,
not mixed across pages.

## Two phases, matching Gate 4's own shape

`scripts/build-case-study.py plan` then, only after human approval,
`scripts/build-case-study.py generate`. See
`../references/approval-policy.md`'s "Gate 4: Share confirmation" for the
human-facing side of this. This file covers the judgment calls inside
`plan` that aren't mechanical.

`plan` takes `--before-image`/`--after-image` (the real screenshots for the
interactive comparison), `--before-label`/`--after-label` (see below),
`--title`, an optional `--logo`, and optional `--mechanical-report`/
`--a11y-report` (the chosen page's real JSON reports — enables the
Validation section's real pass/violation counts; omitted gracefully if not
given), and writes them into `case-study-data.json` alongside everything
else it computes. `generate` reads them back and builds the finished site
folder itself at the end of its own run — there is no separate manual "now
run `render-case-study-web.py`" step, because that used to depend on
whoever called `generate` still remembering those paths from earlier in
the conversation. The one paid step is also skipped if a prior invocation
of the same `case-study-data.json` already completed it, so retrying after
a crash never re-charges it.

**`--before-label`/`--after-label` are new, real, human/agent-judged
sentences** (same bar as `--subject`/`--title`): a short, honest
characterization of the visual-language shift — e.g. "Consumer-SaaS Card
Grid" → "Enterprise Operations Console" — that becomes the typographic
hero's headline. Ground it in the real direction chosen at Gate 1, not a
generic "before/after" label.

**`--output-dir` must be the real run directory, not a fresh folder for the
case study's own output.** It's the same directory holding `discover.json`,
`analysis.md`, `directions/`, `variants/`, `validate-report.md` — `plan`
both reads it (`package_share.gather_evidence()` scans exactly these paths
to decide which real tool/skill credits belong in the finished case study)
and writes to it (`case-study/` is created as a subfolder of it, not
somewhere separate). Confirmed the hard way: pointing this at a fresh,
otherwise-empty folder doesn't error — `gather_evidence()` just finds none
of the files it's looking for, and the finished case study silently credits
only `redesign-lab` and `cogfoundry-labs/loomloom`, missing every other real
dependency the run actually used (the design authority, `taste`,
`webapp-testing`, `a11y-audit`). Nothing fails loudly; the case study just
under-credits real open-source work by default. (`gather_evidence()` has no
code path that credits a specific Gate-1 direction-variant skill by name,
e.g. `industrial-brutalist-ui` — only the base design authority declared at
Discover — so a correct `--output-dir` doesn't change that particular gap;
it's not a real credit category this function produces at all, not
something a wrong path merely fails to find.) If this happens after `generate`
already ran (the narrative already paid for), don't re-run `plan` from
scratch — that mints a new `case-study-data.json` and re-triggers the paid
call. Instead, call `package_share.gather_evidence()` directly against the
correct directory, patch the result into the existing `case-study-data.json`'s
`evidence` key (preserving the `cogfoundry-labs/loomloom` entry `generate`
already appended), and re-run `generate` alone — it detects the narrative is
already done and only re-renders, for free.

## No image-generation anymore

The cover used to be a real `image-generate` call grounded in the actual
redesign's color tokens — real, but still a decorative abstraction, and it
carried this pipeline's single biggest cost/reliability risk: `precheck`
undershot the real charge by ~7x, confirmed on two separate real runs
(larstornoe.com, then again on shengsuanyun.com). It's gone. The hero is
now typographic (`render-case-study-web.py`'s `render_case_study_html`):
site name, before-label ↓ after-label, real color swatches read directly
from `root_colors` — the same tokens the old cover prompt used, applied
directly instead of asked of a model. Zero cost, zero model call, and
arguably more "real artifact" than the generated illustration ever was,
since it's the actual tokens rather than a picture of them.

**The one loomloom call left is the narrative pass**, kept deliberately —
not an oversight, not the next thing to cut. It's genuinely cheap (~$0.01)
and fast for what it does: turning 3-6 real, verified facts into readable
prose in one batched call. That's specifically why it's worth keeping
loomloom in the picture for Share at all, even though nothing else in this
pipeline needs it: a single narrative pass at this scale doesn't need
loomloom's parallel-throughput advantage (that's the aesthetic-scoring
pass's old justification, itself removed in rev 6) — it's just cheap and
fast on its own terms.

## Templates are reused across runs, not recreated

`get_or_create_template()` looks up an existing private template by name
(`case-study-narrative`) via `template-spec list` before registering a new
one, and calls `template-spec create-version` on it if found. This isn't
hypothetical tidiness: a live `template-spec list` call during an earlier
review turned up several orphaned `case-study-cover-*`/
`case-study-narrative-*` templates left behind by ad hoc test runs — the
`-cover` family is now dead weight on the platform (never created again,
never cleaned up either, out of scope for this rev). Every real run's
actual content becomes a new version under the one stable narrative
template, not a fresh template every time.

## Selecting chapters: significance, not a target count

`scripts/diff-transformations.py` does the real, $0, no-model-call
comparison (root color tokens, headline typography, nav typography,
structural framing/borders, corner treatment, content hierarchy, layout
density) and returns every real difference found, ranked by significance.
`select_chapters()` then takes 3-6 of the most significant — never padded
to a minimum, never truncated below 6 if that many are genuinely
significant. If a redesign only produced two real, meaningful differences,
the case study gets two chapters, not two real ones plus manufactured
filler.

**This is a real comparison, not a heuristic guess dressed up as one** —
every fact in a finding is read from `getComputedStyle()` on the actual
rendered page, the same standard `mechanical-check.py` already holds itself
to. If `diff-transformations.py` ever needs a new axis (a real redesign
pattern this file's fixed list doesn't cover yet), add a new computed-style
comparison there, not a text-based guess here.

**One real heuristic gap, worth knowing about**: `content-hierarchy`'s
"featured item" detection is purely image-size-based (a real `<img>`
meaningfully larger than the median image width). A redesign that creates
emphasis through borders/typography/layout instead of a large image (a
real, legitimate technique — confirmed on the shengsuanyun.com run) reads
to this heuristic as "featured item removed," and an AI-generated
narrative from that raw fact alone can spin a plausible but backwards
story ("the redesign eliminated emphasis"). Check this chapter's generated
narrative against what the redesign actually did before shipping; correct
it by hand if needed (`render-case-study-web.py`'s render step is a free,
local re-run — no need to re-spend the narrative call to fix a wording
issue, edit `case-study-data.json`'s `chapters[].narrative` directly and
re-run the render step alone).

## `implemented` vs `preview`: a first-class status, not a caveat in prose

Set by the caller of `build-case-study.py plan` via `--status`, based on
one real fact: did Implement actually write files into a project this
pipeline could deploy, or was the redesign target a live external site
Implement was never going to touch? `render-case-study-web.py`'s
`STATUS_COPY` table is the single place this status becomes user-facing
language (badge, before/after caption) — never restate the distinction ad
hoc elsewhere in generated copy, or a future run can drift into implying a
preview shipped.

## Reproducibility limit: a live "before"/"after" target is re-fetched, not replayed

`diff-transformations.py` visits the real, live before/after pages at
`plan` time to compute its facts. For a local file target (a variant this
pipeline rendered, or a project Implement wrote into) that's the exact same
file Gate 2 showed. For a live external URL — the case for a `preview`-status
run against a site this pipeline doesn't own — the page could have changed
since Gate 2 was approved, so the diff might not exactly match what the
human actually looked at. `plan` detects this (any `http://`/`https://`
target) and surfaces it as an explicit `diff_capture_note`, printed at plan
time and carried into the finished case study's Reproduce section — an
honest, visible limitation rather than a silent assumption that a live
page held still.

## A real GitHub-Pages-ready folder, not one inlined file

`render-case-study-web.py`'s `build_case_study_site()` writes a real,
separate-file site (`index.html`, `styles.css`, `script.js`,
`assets/{before,after,logo,favicon}.*`) instead of one base64-inlined HTML
blob. This is a deliberate rev, not a style preference:
`implement-design.md` already draws the line between "a standalone preview
with no real project/server" (base64-inline — Direction Slices, Explore
Variants, an Implement pass against a site this pipeline doesn't own) and
"a real deploy target" (real asset files, referenced normally). A GitHub
Pages repo is the latter. Confirmed the cost of getting this wrong twice
in one session: a 2.48MB and a 3.45MB single-file case study both exceeded
this environment's Browser-pane rendering ceiling and had to be sent as
files instead of shown. Real separate files also make the folder
genuinely "easy to inspect, clone, and modify" — WP1's own bar for a
GitHub Pages reference implementation — where one giant inlined string
never was.

**Actually creating a GitHub repo, enabling Pages, or pushing commits is a
separate, explicit-permission action** — this stage only produces the
ready-to-deploy folder locally. Never push/publish it without the human
asking for that specific action.

## The compare widget is a fixed-size frame with real scroll inside it

`.compare` used to size itself from the before-image's natural aspect
ratio (`width:100%;height:auto`), which reads fine when the two real
screenshots are close in length but breaks down otherwise: confirmed for
real on the shengsuanyun.com/loomloom run, where the redesign compressed
an 8340px-tall real page into a 2932px-tall one (a real, legitimate
content-density change) — the widget ballooned to ~5500px tall to fit the
before-image, while the after-image (much shorter) only filled the first
~1950px of that height before running out of real content. A human
looking at it reported "I can only see the top part of that page," which
was accurate.

The first fix (`object-fit:cover` cropped to each image's own top) solved
the ballooning but traded away real content — the user asked, correctly,
for both properties at once: a fixed, video-frame-like outer size (16:9,
so someone could screen-record the interaction and it would read like a
normal demo video) **and** a real vertical scrollbar inside it so every
part of both real pages stays reachable. The two images no longer crop:
each renders at its own full real length (`width:100%;height:auto`,
no `object-fit`), and `.compare-inner`'s height is set via JS to
`Math.max(beforeHeight, afterHeight)` so scrolling reaches the end of
whichever real page is longer — the shorter one simply runs out of real
content first, shown honestly rather than stretched or padded to match.

**The real trap in this fix**: `position:absolute` does *not* exempt an
element from scrolling with its own scrolling ancestor — that's
`position:fixed`/`sticky` behavior, not `absolute`. The first attempt kept
the drag handle as a child of the scrolling `.compare` element itself,
assuming its absolute positioning would keep it visually anchored; testing
found it scrolled away with the content instead, confirmed directly by
comparing the handle's `getBoundingClientRect().top` before and after a
programmatic scroll. Fixed by splitting `.compare` (the scrolling element)
from a new outer `.compare-frame` (the non-scrolling 16:9 box) — the
handle is a sibling of `.compare` inside `.compare-frame`, not a
descendant of it, so `.compare`'s internal scroll never touches the
handle's own containing block at all.

Two more real, confirmed details this rework needed:

- **`touch-action` had to change from `none` to `pan-y`** on `.compare` (and
  back to `none` on just the handle) so native vertical touch-scrolling
  works on the frame while the handle still fully owns horizontal drag
  gestures — `touch-action:none` on the whole widget, correct when the
  only interaction was horizontal drag, would have blocked the new
  vertical scroll gesture entirely.
- **Dragging now starts only from the handle itself**, not anywhere in the
  widget. The old `widget.addEventListener('pointerdown', ...)` (drag from
  anywhere) and the click-anywhere-to-jump shortcut both fought the new
  scroll gesture — a vertical scroll attempt starting anywhere but the
  handle would have also jumped the horizontal reveal. Removed the
  click-to-jump shortcut entirely rather than trying to disambiguate it
  from a scroll gesture after the fact.
- **A real axe-core finding, not anticipated going in**: a scrollable
  region needs to be keyboard-focusable in its own right
  (`scrollable-region-focusable`) — confirmed via a real scan, not assumed.
  `.compare` now has `tabindex="0"` and a real `aria-label` so a
  keyboard-only user can Tab to the frame and use arrow/Page keys to
  scroll it, separately from tabbing to the handle to adjust the
  horizontal reveal.

## A "broken" before/after image can be a capture bug, not a widget bug

After all of the compare-widget rework above, a human still reported the
"before" side rendering incorrectly — "this problem was here and never
been fixed." Every prior fix had targeted the widget's CSS/JS, all of it
correct and confirmed working; the real defect was one layer upstream, in
the screenshot itself. Pixel-sampling `before.png` row by row (mean +
variance per sampled row) found ~80% of the image — everything below the
first fold — was flat, zero-variance near-white: not real page content
that happened to be light-colored, but a genuinely blank capture.

Root cause: `render-and-screenshot.py` (used for the before/after
screenshots specifically, a separate script from `capture-assets.py`)
called `page.goto(..., wait_until="networkidle")` immediately followed by
`page.screenshot(full_page=True)`, with no scrolling in between.
Playwright's full-page screenshot resizes the viewport to the full
document height and captures in one shot — it does not fire `scroll`
events along the way. ShengSuanYun's real page uses scroll-triggered
reveal animations (common on marketing pages), so every section below the
first viewport was still sitting at its pre-animation `opacity:0` state
when the screenshot fired. `capture-assets.py` already had this exact
problem solved for lazy-loaded `<img src>` (`trigger_lazy_load()`, with a
comment explaining why, confirmed against a real bunnings.com.au bug) —
that fix had just never been ported to the sibling script that actually
produces the case study's before/after images.

The fix, `prepare_for_full_page_capture()` in `render-and-screenshot.py`:
inject CSS forcing all transition/animation durations and delays to `0s`
(so anything triggered resolves to its end state instantly, not mid-fade),
then scroll through the full page in steps before taking the full-page
screenshot. Confirmed against the real target: re-capturing dropped the
image from 8340px (mostly blank) to 4427px of entirely real content, no
blank rows anywhere in the same row-variance scan.

**The general lesson**: when a human reports a rendering defect a second
or third time after a fix that was individually verified and correct,
don't re-verify the same layer harder — check the layer underneath it.
Pixel-sampling the actual image asset (mean + variance per row, not just
eyeballing a screenshot) settled in one call what three rounds of widget
CSS/JS changes couldn't.

## Links resolve correctly on GitHub Pages, not just in local preview

The "Copy link"/"Copy the code" buttons resolve their URL at *click time*
from `<link rel="canonical">` if `--canonical-url` was given, otherwise
from the page's own real `location` — never a build-time constant. This
matters specifically because it was tested wrong the naive way first: a
URL baked in at build time would show whatever this environment's local
preview server happened to be running on (`http://127.0.0.1:8941/...`),
not the real GitHub Pages URL once actually deployed. Resolving from
`location`/`canonical` at the moment someone clicks means the exact same
built files produce a correct link in local preview *and* once deployed,
with no re-render needed in between.

## Chapters and the compare widget are individually embeddable, cross-origin

Each chapter and the before/after compare widget has a **"Copy the
code"** button, not "Copy link" — it copies a real `<iframe>` snippet
pointing at a small standalone fragment page (`embed/compare.html`,
`embed/ch-01.html`, ...), not a same-page anchor. Anchors don't help
someone who wants to show one chapter on their own blog; an iframe does.
Each fragment reuses the real `../styles.css`/`../script.js` rather than
duplicating the compare-widget's CSS/JS a second time, so it renders
identically to the source page regardless of the host site's own styles —
confirmed for real by embedding both fragment types on a mock page served
from a different origin/port and verifying the drag interaction still
works cross-origin (Pointer Events aren't blocked by `iframe` boundaries
the way DOM access to `contentDocument` correctly is). Each fragment is
also its own complete, valid document — real `<main>` landmark, a real
`<h1>` (visually hidden for the compare widget, since the widget speaks
for itself; visible for a chapter, since its title *is* the fragment's
real heading) — confirmed by a real axe-core scan against the fragment
files directly, not just the parent page.

## Every credited tool/repo says what it actually did for this run

`package_share.gather_evidence()`'s `tools_used` is a list of
`{name, repo, role}` dicts, not one-line strings — a case study crediting
open-source work should let a reader click through and see *why* it's
credited, not just that it's named. Every entry is still evidence-gated
(only appears if the real file/directory proving it ran exists), now
including `redesign-lab` itself (always, since the pipeline produced
everything else) and `cogfoundry-labs/loomloom` (appended in
`cmd_generate`, after the one real loomloom call actually succeeds —
`gather_evidence()` runs at `plan` time, before that call exists, so it
can't credit it itself). The design-authority credit also has a fallback:
`discover.json` doesn't exist for a run against a live external site (no
local project to write it into), but `analysis.md` always states the
declared authority in its own opening line — read from there when
`discover.json` is absent, rather than under-crediting the authority.

## Naming conventions in rendered copy

Always **`loomloom`**, lowercase, in every piece of copy this pipeline
writes about itself (the "How loomloom Did It" section, the tool credit,
the final CTA) — never `LoomLoom` or `LOOMLOOM`. This is this pipeline's
own stylized name for itself, same as the real `cogfoundry-labs/loomloom`
repo path is already all-lowercase. **This does not apply to a redesign
target's own real product names** that happen to share the string — if
the site being redesigned has its own real product genuinely branded
"LoomLoom" (as shengsuanyun.com's own workflow product is, coincidentally,
in the run this rev was validated against), that's the subject's own real
proper noun and must be reproduced as the subject actually brands it, not
forced to match this pipeline's own lowercase convention. The two are
unrelated by name coincidence only; don't conflate them into one casing
rule.

The pipeline's own display name is **Redesign Lab** (the `# Redesign Lab`
heading and `name: redesign-lab` in `../SKILL.md`) — every "Made with",
"Case Study ·", and "Try Redesign Lab" line in the rendered copy uses this
name, and so does every internal identifier (script comments, request-id
prefixes, credited repo names): no `redesign-factory`/`Redesign Factory`
anywhere. The real GitHub location is
`cogfoundry-labs/loomloom/tree/main/examples/community/redesign-lab` — this
pipeline lives as a community example inside the loomloom repo itself, not
as a separate standalone repo. `REDESIGN_LAB_REPO` in `package-share.py`
and the credit/reproduce links in `render-case-study-web.py` all point
there. If it's ever split out into its own repo, update that one constant
and the handful of literal links alongside it — don't let them drift.

## What's deliberately out of scope for this pass

- **Per-chapter social-card image generation** — the chapter data model
  (`category`, `significance`, `before_fact`, `after_fact`, `narrative`) is
  already shaped so a future social-card renderer can consume it directly;
  building that renderer is a separate, later capability, not part of this
  one.
- **PDF/ebook export** — same reasoning: `case-study-data.json` is the one
  artifact every renderer reads from; `render-case-study-web.py` is the
  first sibling, not the only one that will ever exist.
- **Automatic subject/label generation** — `--subject`, `--before-label`,
  `--after-label` are required, human/agent-supplied real sentences, not
  inferred automatically.
- **A reusable, multi-case-study template/CMS** (WP2-WP5 in the case-study
  design spec) — this pass makes one case study excellent; generalizing
  what works is a later, separate step.

## Handoff

Once `generate` completes (it builds the site folder itself, as its last
step), open the real, rendered `index.html` in the Browser pane — same
"show, don't just describe" standard as every gate before it
(`approval-policy.md`'s Gate 3 section). If this run's scope actually
produced a real implemented site (status `implemented`), open that too.
