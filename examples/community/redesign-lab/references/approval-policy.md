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
couple of real, image-heavy options were embedded together (see "Building
the comparison page" below for the confirmed numbers), and the "jump to
another option" links built to route around a failed combined page turned
out to be completely inert regardless of how they were built — a
`data:`-rendered local file is blocked from top-level-navigating to another
local file via an in-page click, full stop. Both problems shared one root
cause: cramming every option into a single document. Rev 7 also brought the
default option count for Gates 1 and 2 down to 3-4 (see
`../skills/generate-directions.md` and `../skills/explore-variants.md`),
which made a different mechanism practical.

**The mechanism now:**

1. **Real, separate browser tabs, one per option, opened progressively as
   each is built** — not a combined page. Build option 1, open its tab, tell
   the human it's ready with a short real-facts summary, then build option
   2 while they look at what's already open. This sidesteps both rev-7
   problems at the root: no combined payload to exceed a limit, and no
   in-page navigation needed, since the human just switches tabs in the
   browser's own tab bar. It also means the human sees the first real result
   as soon as it exists, not after every option in the batch is done.

   **Open the real deliverable file directly — no wrapper, no meta-header
   added to the page.** The tab shows exactly the real slice/variant,
   unwrapped, with a real distinguishing `<title>` so the browser's own tab
   bar tells options apart. The substantive information (which
   family/direction/variant it is, 1-3 concrete facts about what's actually
   different about this option, a one-line mechanical-check summary like
   "11/11" or "10/11 — nav height is intentional, see below") goes in the
   *chat message* announcing that tab, not printed into the page itself —
   with exactly one option per tab there's no multi-option page to lose
   track of while scrolling, which is the only reason the old
   combined-page mechanism needed an in-page header at all.
2. **The decision itself is an `AskUserQuestion` call, made once every
   option in the current pass is open.** Options: each built option by name
   (mechanical score + real notable facts as the option's description),
   plus explicit flexibility options in the same call — "ask for N more"
   and "let me pick/describe one not yet shown" — and always a "stop here"
   option. This replaced a plain numbered-list-in-chat mechanism: that was
   the right call in rev 1 specifically *because* the widget being replaced
   conflated "look at the real work" and "make the decision" into one
   bespoke UI. `AskUserQuestion` doesn't repeat that mistake — it's used
   only for the decision, after the real pages are already open in their
   own tabs, with clear option labels rather than a hand-built card grid.
   Never let it substitute for actually opening the real tabs first.
3. **The "stop here" option is always present** in the same `AskUserQuestion`
   call as the domain choices — see "Every gate: three outcomes" above. If
   a gate's choice mechanism doesn't include it as a real, selectable
   option, that's a defect in how the gate was built, not an acceptable
   shortcut.
4. **Go straight from building/opening a tab to telling the human it's
   ready — no placeholder tool call in between.** A no-op bash command, an
   empty checkpoint marker, or any other tool call whose only purpose is to
   mark "I'm about to say something" is visible noise to the human watching
   tool calls scroll by, and adds nothing a plain chat message doesn't
   already say.

**Opening each option's tab — constraints confirmed the hard way, not
guesses:**

- **Always verify the tab actually loaded — never trust the tool's own
  "opened" message alone.** A direct DOM check (`document.title`, or an
  element count specific to that slice/variant) is the only reliable
  signal; a tool that reports success can still leave stale content on
  screen, or spawn a fresh tab id instead of loading into the one requested
  — check `tabs_context`/the DOM after every open, not just the tool's own
  message.
- **Prefer opening the real deliverable file directly, full quality, no
  wrapper.** With one option per tab (not eight embedded in one page), the
  base64-payload math that broke the old combined page is far less likely
  to bite — a single slice's own file is a small fraction of what a
  combined page used to carry. If a real deliverable file ever does fail to
  open (same DOM-check rule above catches this), the confirmed fallback is
  a disposable, deliberately low-resolution copy of its embedded images
  built *only* for viewing (e.g. capped at ~300-500px width, JPEG quality
  ~40-45) — never degrade the actual deliverable file itself, only a
  throwaway preview copy of it.
  **Why this matters at all, confirmed rev 6 (a real 6-live-site run,
  northsydbo-h):** a 4.15MB combined comparison page (8 iframes,
  full-quality embedded photos) failed to open, while a same-size plain-text
  file (0.47MB, no images) opened fine well inside that failing range. This
  preview tool renders a local file as a `data:` URL, and base64's own
  `+`/`/`/`=` characters balloon under the percent-encoding that requires —
  a base64-heavy file is effectively much "larger" to this tool than its raw
  byte count suggests. That's the failure mode per-tab opening avoids by
  construction, not something this rule is guessing will also apply here.
- **Cross-tab/cross-page links inside a rendered page are inert — don't
  build them, and don't rely on them.** A same-tab `<a href="...">` to
  another local file does not work from inside a `data:`-rendered page, in
  either relative or absolute `file://` form — confirmed by clicking it and
  observing the DOM was unchanged (`document.title` still the old page). A
  `data:` document is blocked from top-level-navigating to a `file://` URL
  via an in-page click, full stop, regardless of the href's form. This is
  exactly why the tab-per-option mechanism above has the agent open each
  tab itself, one at a time, rather than building any kind of "switch to
  the next option" control into the rendered pages.
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

## Gate 1: direction choice

Shown after Direction Slices produces its real, rendered multi-section
slices — 3 by default (a `current-fixed` baseline plus 2 exploratory
directions, one carefully-chosen colorway each), 4 if the human opted into
a 3rd exploratory direction beforehand (see `../skills/generate-directions.md`).
Each slice is hero plus at least 2 more real content sections, always
screenshotted full-page. Built and shown one tab at a time, progressively,
per "How gates are actually shown" above — never a combined comparison page.

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

Build and open each slice's tab as it's ready (current-fixed first, then
each exploratory direction), telling the human plainly each time — don't
wait until all are done. Once every slice in this pass is open, present the
decision as an `AskUserQuestion` call:

- One option per built slice, its 1-3 real notable facts (including any
  defects fixed, for `current-fixed`) and mechanical-check score as the
  option's description.
- **"Show me more directions"** — agent picks 1-2 more per the same
  genome-distance method, built and opened the same progressive way.
- **"Let me pick a direction myself"** — human names a `direction_variants`
  skill not yet shown.
- **"Stop here"** — nothing already built is lost.

If the human likes a shown direction but wants to see it in a different
colorway before deciding, that's also always available — build the
requested colorway (`generate-directions.md`'s "Colorways on request"),
open it in its own tab the same way, and re-present the same
`AskUserQuestion` shape with the new option added.

## Gate 2: variant choice

Shown after Explore Variants produces its real structural variants of the
Gate-1-chosen direction+colorway — 3 by default (the Gate-1 slice itself,
reused for free, plus 2 new structural compositions), 4 if the human opted
into a 4th (see `../skills/explore-variants.md`). Same progressive,
one-tab-per-variant mechanism as Gate 1 — build variant 2, open it, tell the
human, build variant 3 while they look.

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

Once every variant in this pass is open, present the decision as an
`AskUserQuestion` call:

- One option per built variant (mechanical score + real composition notes
  as the description).
- **"Show me more variants"** — agent picks 1-2 more structural
  compositions, built and opened the same progressive way.
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
there's no per-option tab mechanism here — just a plain-text question, same
"say stop" line as every other gate:

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
