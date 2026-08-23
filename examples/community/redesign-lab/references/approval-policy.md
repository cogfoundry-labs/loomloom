# Approval policy: the three gates

Only these three moments stop for a human. Everything else in the pipeline
just runs.

## Every gate: three outcomes, not two

Every gate below offers **approve, reject-and-retry, and stop** — never
just the first two. This is a standing rule, not a per-gate reminder: any
gate that only offers "pick one of these" with no way to just stop is a
defect in how it was presented, regardless of what stage produced it.

- **Approve** — the domain choice itself (a direction, a variant, a scope).
- **Reject-and-retry** — "none of these, try again" (`on_reject` in
  `../pipelines/redesign-existing-site.yaml`): loops back to redo the
  producing stage, real work, not a no-op. This is never a button — it's
  the human describing what's wrong in ordinary chat. The response is: fix
  the specific thing named, then rebuild that one option and re-open its tab
  (below) rather than redoing the whole set. A blind "regenerate everything"
  button would throw away whatever about the current set was already fine;
  targeted fix-and-re-present doesn't.
- **Stop** — "stop the whole workflow here." Distinct from reject: nothing
  is discarded (everything already produced is real, saved output, not
  provisional), and nothing loops back to retry anything. The pipeline
  simply stops advancing and hands control back. Resuming later is just
  asking again — there's no teardown step, because nothing about stopping
  is destructive.

Concretely: **every choice message ends with an explicit, visible "say stop
to halt" line**, every time, not a side channel the human has to know to
invoke unprompted. A gate that only lists domain options with no stated way
to stop has not implemented this correctly.

## How gates are actually shown: real pages, not descriptions

All gates — and every re-presentation after a fix — use the same concrete
mechanism. This section has been replaced twice now, each time after a real
run surfaced a real problem:

**Rev 1: replaced a click-to-chat widget grid.** An earlier version of this
policy specified `mcp__visualize__show_widget` with `sendPrompt`-wired
buttons. The click-to-chat bridge worked in isolation, but the full card
grid built on top of it confused more than it helped: two similarly-named
buttons per card ("Explore free" / "Explore optimized") with no explanation
of what either meant, and a colorway shown as a 34px swatch too small to
actually judge. The human explicitly said the whole click-based grid was
more complicated than it needed to be, and asked for real pages in real
browser tabs instead — that preference (real pages, not screenshots-in-a-
widget) is the part that stuck.

**Rev 7: replaced the combined-comparison-page-with-iframes mechanism that
rev 1's fix had settled into.** That mechanism (one scrollable HTML page per
gate, every option embedded as an `<iframe srcdoc>`, a sticky "Jump to" bar)
worked, but had two real, confirmed problems: the combined page routinely
exceeded this environment's base64-payload rendering limit once more than a
couple of real, image-heavy options were embedded together, and the "jump to
another option" links built to route around a failed combined page turned
out to be completely inert regardless of how they were built — a
`data:`-rendered local file is blocked from top-level-navigating to another
local file via an in-page click, full stop. Both problems traced to one
thing: the page was opened as a `file://`/`data:` document with every
option's real bytes inlined into it. Rev 7 replaced it with real, separate
browser tabs, one per option — which avoided both problems, for a while.

**Rev 8: replaced tab-per-option with a single comparison page again — not
a reversion, a fix for a bug rev 7's own mechanism turned out to have.**
Confirmed directly, twice, in one session: **this environment's Browser
pane only ever composites one tab live.** Switching tabs in the pane's own
tab bar does not reliably hand compositing to the newly-selected tab, so
every tab but whichever was most recently active silently displays a stale
frame. This is indistinguishable from a genuine rendering bug from the
human's side — it read as "these tabs render wrong" and "these tabs all
look the same," and cost real time chasing viewport and asset causes before
`computer{action:"screenshot"}` against a *fronted, just-selected* tab
returned the exact error that named it: *"the Browser pane is not
displayed, so the page is not compositing frames."* That error on a tab
the human was actively looking at is the tell — multiple tabs cannot both
be true, live render surfaces at once in this environment, no matter how
correctly each is built.

A single page sidesteps this by construction: there is only ever one tab,
so there is nothing for the pane to get wrong. This is not the same
mechanism rev 7 removed — the reason rev 7 removed it doesn't apply here.
Rev 7's combined page inlined every option's actual bytes into one
`file://`/`data:` document (the base64-payload and dead-link problems both
came from that). This one is served over a **real local HTTP server**, and
each option is a real `<iframe src="http://...">` pointing at its own real
file — the parent page's payload is a handful of URL strings, not the
options' own bytes, so rev 7's blocker genuinely doesn't apply.

**The mechanism now:**

1. **Build each option's real HTML file exactly as before** (per
   `generate-directions.md`/`explore-variants.md`) — this part is
   unchanged. Run `mechanical-check.py` against it, same as always.
2. **Serve the run's output directory over a real local HTTP server**
   (`python -m http.server <port>` from the run's working directory, or
   equivalent) before showing anything. This must happen before the first
   option is presented — file:// breaks the iframe `src` resolution below
   and reintroduces the exact problem this mechanism exists to avoid.
3. **Generate the comparison page with `../scripts/build-compare-page.py`**
   — pass `--option "label=src=score"` once per option built so far (real
   `/`-relative URLs served by the HTTP server, never `file://`, never
   inlined `srcdoc`), and `--out` a path inside the served directory.
   Re-run it (adding one more `--option`) each time a new option finishes
   building — this is what makes the reveal progressive, replacing "open a
   new tab" from the rev-7 mechanism.
4. **Open (or force-reload) the one comparison-page tab.** First option:
   open it. Every option after that: the same tab, force-reloaded against
   the regenerated page, so the human always has exactly one tab to look at
   and it always has every option built so far. Tell the human it's ready
   in the *chat* message — "`current-fixed` is up: [1-3 real notable
   facts] — mechanical-check: 11/11" — same as the old per-tab
   announcement, just pointing at a button instead of a new tab.
5. **The decision itself is an `AskUserQuestion` call, made once every
   option in the current pass is built.** Options: each built option by
   name (mechanical score + real notable facts as the description), plus
   explicit flexibility options in the same call — "ask for N more" and
   "let me pick/describe one not yet shown" — and always a "stop here"
   option. `AskUserQuestion` is used only for the decision, after the
   comparison page is already open with every real option in it, with
   clear option labels matching the page's own button labels. Never let it
   substitute for actually opening the comparison page first.
6. **The "stop here" option is always present** in the same `AskUserQuestion`
   call as the domain choices — see "Every gate: three outcomes" above.
7. **Go straight from building/regenerating the page to telling the human
   it's ready — no placeholder tool call in between.**

**The comparison page's own chrome must be visually unmistakable from the
options it's showing, in both directions.** Confirmed directly: a first
version used a neutral dark header (`#141416`) that was close enough to a
dark redesign's own near-black background (`#0A0A0A`) that a human couldn't
tell, from a screenshot, where the tool's own UI ended and the real page
began. `build-compare-page.py`'s header uses a saturated amber
(`#F5B400`) with an explicit "COMPARISON TOOL — NOT PART OF THE DESIGN"
badge — a color and a label no real redesign is likely to produce for its
own background, so the chrome reads as chrome regardless of whether the
option currently shown is light or dark, loud or restrained. Don't soften
this into the redesign's own palette to "match the vibe"; the entire point
is that it doesn't match.

**Constraints confirmed the hard way, not guesses:**

- **Always verify the comparison page actually loaded — never trust the
  tool's own "opened"/"navigated" message alone.** A direct DOM check
  (`document.title`, or `document.querySelector('iframe.is-active')`'s
  `contentDocument.title` from inside the active frame) is the only
  reliable signal that the right option is actually showing, not just that
  *a* page loaded.
- **Sweep and close any stray `file://` tab before presenting anything.**
  The `Write` tool's own preview hook auto-opens a `file://` tab for every
  HTML file the instant it's written — before any templating/asset
  substitution happens, so that tab can show broken placeholders
  (`__LOGO_B64__` literally in the `src`, zero viewport) that were never
  real to begin with. Confirmed directly: these phantom tabs interleaved
  with real tabs, shared the option's own `<title>`, and were
  indistinguishable by name — one was even the *active* tab after a
  rebuild. Call `tabs_context` and close every `file://`-origin tab before
  navigating or screenshotting anything; only `http://`-origin tabs are
  ever real.
- **Source images must match their actual rendered width, not a guess** —
  applies to the fallback low-res preview above, not the real deliverable
  file (which keeps its real assets at whatever size the real design
  calls for). Measure each `<img>`'s real `getBoundingClientRect().width`
  before picking a preview resolution — sizing below that guarantees
  visible upscale blur no matter how high the JPEG quality is set,
  confirmed directly on a real run where a hero photo rendered at 1040px
  from a 340px source.
- **Never batch `tabs_create` calls in parallel.** Creating several browser
  tabs in one parallel tool-call batch desyncs the tabId the tool reports
  from the tab that actually receives the next `navigate` call, and can
  silently exhaust a real tab-count cap before every intended page loads.
  Create and navigate one tab at a time. If a `navigate` call ever fails
  with a vague "couldn't open" error, check for a tab-count/dead-tab
  problem before suspecting the file itself.
- **`navigate` with `force: true` can close and recreate the tab under a
  new id**, rather than reloading in place. Always re-check
  `tabs_context`/the actual DOM after a forced reload — don't assume the
  tabId you passed is still where the content landed.
- **The Browser pane must actually be displayed to composite frames.** A
  screenshot request against a backgrounded/hidden pane fails outright;
  this isn't fixable from the agent side — ask the human to bring the pane
  back into view.
- **Every tab shown to the human MUST have an explicit viewport set via
  `resize_window` with real `width`/`height`, or it renders wrong in the
  human's pane.** This is the single highest-value entry in this section:
  it cost most of a real run to find, and it was misdiagnosed twice before
  being measured. A tab left at "native" (no explicit emulation) lays its
  page out at a width that does not match the pane widget, leaving a stale
  framebuffer strip beside the real content. To the human that reads as
  *"two tabs rendered on top of each other"* — a completely misleading
  symptom that invites the wrong fix. Confirmed, in one session:
  - Three gate tabs held three different viewports (1239x1257, 1280x720,
    800x455) because `resize_window` **applies per-tab and persists**, so
    earlier measurement/responsive calls stranded whichever tab was active
    at the time. The 800x455 tab laid out a 785px-wide body inside a
    ~1239px pane; that 785px column plus dead space to its right was the
    "overlapping tabs" the human actually saw.
  - Setting a tab to **the exact numbers it already reported** still fixed
    its rendering. That is the proof it's *emulation being active* that
    matters, not the dimensions — so "it already reports the right size"
    is never a reason to skip this.
  - **`resize_window` with `preset: "desktop"` is a silent no-op**: it
    replies `"Viewport reset to native size (desktop)"` and changes
    nothing (verified by reading `innerWidth` back immediately after).
    Never use a preset to restore a viewing tab; only explicit
    `width`/`height` takes effect.

  So, concretely, before every gate's `AskUserQuestion`: read one tab's
  `window.innerWidth`/`innerHeight` via `javascript_tool` (don't hardcode
  — the pane size is per-machine and per-session), then `resize_window`
  **every** tab being presented to exactly those numbers, then read
  `innerWidth` back per tab to confirm it took. Do all measurement and
  responsive-width checking on a **separate throwaway tab** the human is
  never shown, so gate tabs never drift in the first place.
- **The agent's own screenshot succeeding is NOT evidence the human's pane
  is correct — the two render paths genuinely diverge.** In the same run,
  `computer{action:"screenshot"}` returned correct, fully-rendered images
  of tabs that were simultaneously displaying broken layout in the human's
  actual pane. Every agent-side check (console clean, network 200s,
  repeated screenshots, explicit `tabId`) passed while the human was
  looking at a visibly broken page. **The only reliable way to know what
  the human sees is to ask them to screenshot their own pane and send
  it** — do that early when a rendering complaint comes in, rather than
  burning turns on agent-side checks that cannot observe the problem.
  Corollary: always send `SendUserFile` screenshots alongside opening
  tabs, every time, not as a fallback after the pane visibly fails — they
  come from `render-and-screenshot.py` at fixed widths and are immune to
  all pane viewport state, which is what makes them the one reliably
  verifiable channel in this environment.

## Gate 1: direction choice

Shown after Direction Slices produces its real, rendered multi-section
slices — 3 by default (a `current-fixed` baseline plus 2 exploratory
directions, one carefully-chosen colorway each), 4 if the human opted into
a 3rd exploratory direction beforehand (see `../skills/generate-directions.md`).
Each slice is hero plus at least 2 more real content sections, always
screenshotted full-page. Built progressively, shown via the single
comparison page (`../scripts/build-compare-page.py`), per "How gates are
actually shown" above — regenerate and reload the same page each time a new
slice finishes, never a fresh tab per slice.

**Narrate which 2 (or 3) exploratory directions were picked and why**, in
plain language, before building anything — the genome-distance reasoning and
the ivory-serif exclusion rule (`../references/style-genome.md`) are how the
choice gets made, but a human reading only "here are 2 directions" has no
way to tell that from an arbitrary pick. State the reasoning, e.g. "I picked
X and Y because they maximize style distance from your current site and
from each other." Always frame the total explicitly as **"3 total: your
current design with its real defects fixed, plus the 2 I picked"** — never
let an earlier mention of "2 picks" and a later slice count read as two
different numbers to reconcile. In the same message, ask the rule-6
question from `generate-directions.md`: offer a 4th exploratory direction,
with a plain, honest time-cost reminder. Also say what "direction slice"
means in plain language the first time it comes up in a session (e.g.
"candidate design" or "one full-page rendered mockup of a design
direction") — don't assume the term is self-explanatory.

Build each slice, regenerate the comparison page with one more `--option`
as soon as it's ready (current-fixed first, then each exploratory
direction), telling the human plainly each time — don't wait until all are
done. Once every slice in this pass is in the page, present the decision as
an `AskUserQuestion` call:

- One option per built slice, its 1-3 real notable facts (including any
  defects fixed, for `current-fixed`) and mechanical-check score as the
  option's description.
- **"Show me more directions"** — agent picks 1-2 more per the same
  genome-distance method, built and added to the same comparison page.
- **"Let me pick a direction myself"** — human names a `direction_variants`
  skill not yet shown.
- **"Stop here"** — nothing already built is lost.

If the human likes a shown direction but wants to see it in a different
colorway before deciding, that's also always available — build the
requested colorway (`generate-directions.md`'s "Colorways on request"),
regenerate the comparison page with it added, and re-present the same
`AskUserQuestion` shape with the new option added.

## Gate 2: variant choice

Shown after Explore Variants produces its real structural variants of the
Gate-1-chosen direction+colorway — 3 by default (the Gate-1 slice itself,
reused for free, plus 2 new structural compositions), 4 if the human opted
into a 4th (see `../skills/explore-variants.md`). Same progressive-build,
single-comparison-page mechanism as Gate 1 — build variant 2, regenerate
the page, tell the human, build variant 3 while they look.

**Mechanical score and the aesthetic note are reference information for the
human's decision, not a filter the agent uses to decide what gets shown.**
With only 3-4 variants built by default there is no curation step at all —
every variant built gets its own tab and its own place in the
`AskUserQuestion` options, in build order, each with its real mechanical
score and a real note on what its composition actually trades off (a
variant that trades a mechanical nav-height pass for a persistent sidebar
nav is a real, explainable choice to describe accurately, not a reason to
rank it last or leave it out). See `evaluation-rubric.md` for the mechanical-
vs-aesthetic distinction this relies on: mechanical findings can eliminate
before a variant is even shown; aesthetic notes never do, they're advisory
information for the human, same as here.

Once every variant in this pass is in the comparison page, present the
decision as an `AskUserQuestion` call:

- One option per built variant (mechanical score + real composition notes
  as the description).
- **"Show me more variants"** — agent picks 1-2 more structural
  compositions, built and added to the same comparison page.
- **"Let me describe a structural idea myself"** — human names the
  composition axis they want tried (e.g. "put the events section first").
- **"Stop here"** — nothing already built is lost.

## Gate 3: final implementation & scope

Shown after Validate passes (all four pieces: webapp-testing, a11y-audit,
Pre-Flight Check, preservation contract) for the page Implement actually
built on its first pass — commonly the home page, but whichever page the
request was actually about (see `../skills/implement-design.md`, "Scope").
This gate approves both the result and how far it should reach. Not a
design choice between options (the design was already chosen at Gate 2), so
there's no per-option tab mechanism here — but it's still presented via
`AskUserQuestion`, same as every other gate (SKILL.md's standing rule 1:
"'Stop here' is always one of the options in the same `AskUserQuestion`
call" applies to Gate 3 as much as Gate 1/2 — a plain-text question typed
into chat is not an acceptable substitute, even for a non-design decision):

```
Validate passed on <the built page>. Build this into the real project, or explore more?

If building, how much of the site should get this treatment?
  "this page only"     ship what's already validated, nothing else
  "other key pages"    pick a curated subset (pricing, docs, about, ...)
                       beyond this page; suggested from discover.json's
                       nav-linked routes, but you confirm the exact list
  "all pages"          every route discover.json listed
Or say "stop" to halt the workflow — nothing already built is lost.
```

If Validate fails any piece, this gate is never reached: the pipeline
returns to Implement instead (see `../pipelines/redesign-existing-site.yaml`).

Picking "Other key pages" or "All pages" sends Implement back for those
routes specifically, reusing the already-approved direction, variant, and
colorway: no new design decisions, no new gate for that second pass (see
"What never gets a gate" below). It hands off to Validate again for the
newly-added routes only, since different content can surface a real,
page-specific issue the first page didn't have.

## Gate 4: Share confirmation

Shown after Gate 3 (and any wider-scope Implement/Validate re-entry) is
done. Share is the one deliberately paid stage in an otherwise-free
pipeline — see `../skills/build-case-study.md` for the judgment calls
involved and `scripts/build-case-study.py`'s own docstring for the
mechanism. Rev: image-generation is gone (it was the pipeline's single
biggest cost/reliability risk for a result that was still only
decorative); the one real loomloom call left is a cheap, fast narrative
pass, kept deliberately for that reason.

**Step 0, before anything else: confirm loomloom is actually usable.**
Nothing before this point in the whole pipeline needs loomloom installed
or configured — that's deferred on purpose, not an oversight, so someone
who never reaches Gate 4 never had to set it up. The moment a human says
"do the case study," check `loomloom` is on PATH and a real API key
resolves. If not, walk them through installing and configuring it right
here (the human gets and manages their own key; never enter one into a
field yourself) before running `plan`.

**Step 0.5, only if this run built more than one page:** ask which single
page the case study should feature, via `AskUserQuestion` — one option per
page this run actually built, named by what it shows. A Case Study has one
before/after centerpiece, not a gallery. See `build-case-study.md`.

Two phases, not one prompt:

1. **`plan` (always free, always safe to run).** Diffs the real before/after
   pages via `scripts/diff-transformations.py` (a real, deterministic,
   $0 comparison — no model call, same guarantee `mechanical-check.py`
   makes), selects 3-6 of the most significant real differences as
   chapters (never padded to a minimum, never truncated below 6 if that
   many are genuinely significant — "meaningful transformations, not
   transformation count"), gathers real evidence via
   `scripts/package-share.py`'s `gather_evidence()`, reads real
   mechanical/a11y pass counts if given, and gets a real
   `template-spec precheck` cost for the one remaining paid call. Also
   captures the before/after screenshot paths, logo path, title, and
   before/after labels that the render step will need, into
   `case-study-data.json` — so nothing about producing the final artifact
   later depends on the calling agent remembering them. Produces the gate
   message below. Nothing has been spent yet. If either before/after target
   is a live URL rather than a local file, this phase also surfaces a
   freshness caveat: that page is being re-fetched right now, not read from
   whatever Gate 2 actually showed, so the diff may not exactly match what
   was approved if the live page changed since then — this is printed at
   plan time and carried into the finished case study's Reproduce section,
   not silently assumed away.
2. **`generate` (paid, only after explicit approval).** Runs the one real
   loomloom call — a batched `text-generate` narrative pass over the
   already-selected chapters, matched back to their chapters by `rowIndex`
   rather than list position — then builds the finished GitHub-Pages-ready
   site folder itself, in-process, using the paths `plan` already captured.
   One command, no separate manual render step. Skipped entirely if a prior
   invocation already completed it (so a retry after a crash never
   re-charges), and uses a stable `--client-request-id` so the platform's
   own idempotency is a safety net too. The real charged cost (not just the
   precheck estimate) is captured from `run watch`'s result and printed and
   recorded once the run completes.

This confirmation is presented via `AskUserQuestion` — same standing rule
as every other gate (SKILL.md rule 1), not the plain bracketed text
`plan`'s own stderr output prints. That printed text is a log line for
whoever's reading the terminal, not the gate itself; the actual approval
step is always the structured widget, with "Skip" as an explicit option
alongside "Generate," never a side channel implied by the printed text
alone. Confirmed as a real gap: an earlier run treated the script's own
`[ Generate Share Artifact ] / [ Skip ]` stdout as if printing it were the
gate, without a following `AskUserQuestion` call — inconsistent with every
other gate in the same run, and exactly what this line exists to prevent
now. The content of the question should still surface everything `plan`
computed:

```
Share
Generates:
  - typographic hero from real color tokens (free, no model call)
  - <N> narrative chapter(s) — the one real loomloom call in Share
  - interactive before/after presentation (reused, free)
  - reproducibility information (reused, free)

Estimated cost: ~$X.XX (narrative only — no image-generation anymore)

[ Generate Share Artifact ]  or  say "skip" to halt — nothing already
built is lost, and nothing above has been spent yet.
```

**Only pay for what's actually new.** Real screenshots, mechanical-check
scores, `analysis.md`, and every other already-produced artifact are reused
directly, never regenerated. The cost estimate reflects only the narrative
pass — nothing else about Share should ever appear in that number.

**The `implemented` vs `preview` status is a first-class field, not
explanatory text the human has to notice.** When the real site could
actually be modified/deployed (a local project Implement wrote real files
into), the case study's status is `implemented` and its language says so
plainly. When it couldn't (a live external site Implement was never going
to touch — the redesign target itself, not a local checkout), status is
`preview`: the case study must say "validated redesign preview," carry a
visible preview badge, and never imply the redesign is live. This isn't a
one-off judgment call per run — `render-case-study-web.py`'s `STATUS_COPY`
table is what actually decides the wording, so the same real distinction
can't quietly disappear from a future run's copy.

## What never gets a gate

Discover, Analyze, and Direction Slices are local/agent work with no
loomloom spend: nothing to approve before Gate 1. Implement and Validate
run automatically between Gate 2 and Gate 3: the human already approved
"build this" at Gate 2's variant choice; Gate 3 is approving the *result*,
not re-approving the decision to build. The same logic covers a wider-scope
Implement/Validate pass after Gate 3 picks "other key pages" or "all pages":
that choice is itself the approval, so the expanded build doesn't stop for
an additional gate before Share.
