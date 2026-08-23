---
name: discover-site
description: Inspect an existing project (local codebase or live URL) for its framework, styling system, routes, and dev command; capture its real logo via scripts/capture-assets.py; and declare which design authority governs this run. First stage of redesign-existing-site. No model call, no question the user has to answer.
---

# discover-site

The only stage in Pipeline 1 with genuinely new instructional content: nothing
external covers "understand this specific codebase's shape." Runs before any
design judgment happens.

## Two modes: local codebase, or live URL

`discover.py` only accepts a local project directory and exits cleanly if
given a URL — it reads `package.json` and config files, which don't exist
for a site you don't have the source of. In practice, most real redesign
targets tested against this pipeline so far (cogfoundry.ai, scu.edu.au,
bunnings.com.au) were live URLs with no local checkout at all, so this mode
is not an edge case: fill in `discover.json` by hand for it, following the
same shape `discover.py` would have produced.

## What to produce

Write `.output/discover.json`:

```json
{
  "framework": "next | vite | astro | remix | plain-html | live-url | ...",
  "styling_system": "tailwind | css-modules | styled-components | vanilla-css | unknown | ...",
  "package_manager": "npm | pnpm | yarn | bun | none",
  "dev_command": "npm run dev",
  "routes": ["/", "/pricing", "/blog/[slug]", "..."],
  "existing_components_dir": "src/components",
  "existing_assets_dir": "public",
  "assets": ".output/assets/manifest.json",
  "design_authority": "leonxlnx-taste-skill"
}
```

## How to fill it in

- **Framework / styling / package manager**: read `package.json` dependencies,
  config files (`next.config.js`, `astro.config.mjs`, `tailwind.config.js`,
  presence of `.css`/`.module.css`/styled-components imports). Don't ask:
  infer from what's actually there. For a live URL with no local checkout,
  set `framework: "live-url"` and `package_manager: "none"`; infer
  `styling_system` from the rendered page instead (computed styles, whether
  utility-class patterns like Tailwind's are visible in class names).
- **Dev command**: the `"dev"` (or `"start"`) script in `package.json`. This is
  what `render-and-screenshot.py` will use to boot the project for every later
  stage's screenshots. Not applicable for a live URL: `render-and-screenshot.py`
  and `mechanical-check.py` both take the URL directly.
- **Routes**: walk the framework's routing convention (file-based routes for
  Next/Astro/Remix; a router config file otherwise) for a local codebase. For
  a live URL, list the real paths actually linked from the primary nav —
  `discover.json`'s own routes list is what Gate 3's "other key pages" option
  later suggests from, so it should reflect real, reachable pages, not a guess.
- **Real logo and real hero visual — always run this here, both modes,
  unconditionally.** Run `scripts/capture-assets.py <url> --out-dir
  .output/assets` against the live site (or the local dev server once it's
  running) and record its output path in `discover.json`'s `assets` field.
  This captures two things every time, neither one opt-in: the real logo
  (home-link `<img>`/`<svg>`, filtered to plausible logo dimensions so it
  doesn't grab a cart/account icon instead), and whatever real visual sits
  behind the hero heading — a photo, a CSS `background-image`, or a
  `<video>`'s poster/first frame — anchored to the real `<h1>`, not to any
  label someone has to remember to pass. That second capture used to be
  opt-in (via `--match`), and a real hero photo was missed on a real site
  for exactly that reason: nobody passed the hero's own heading as a label.
  It's unconditional now, the same way the logo always was. Real *content*
  photos beyond the hero (product/feature images) are captured later, once
  `extract-design-signal.md` knows what real copy to match them against —
  see that skill's own step. None of this existed before 2026-08: every
  direction slice and variant built before that had zero `<img>` tags
  anywhere, including the logo, because nothing upstream ever captured one.
  If `capture-assets.py` finds nothing plausible, say so plainly rather than
  silently leaving `assets` empty: a later stage falling back to text should
  be a visible, logged decision, not an accident.
- **`design_authority`**: read `../references/design-authority.md`. Default is
  `leonxlnx-taste-skill`: use it unless the user has explicitly named a
  different registered authority (currently only `baoyu-design`, and only if
  the project has actually installed it). This field is set **once** and never
  re-asked for the rest of the run: every later Redesign Lab skill reads it from
  here rather than re-resolving it.
- **Verify the chosen authority's skills — plus `taste`, `webapp-testing`,
  and `a11y-audit` — are actually installed, right here, before writing
  `discover.json` at all, and install whichever are missing automatically.**
  `design-authority.md` names which skills each capability maps to, but
  naming them doesn't install them — they're separate skill packages this
  pipeline reads at runtime, same as loomloom at Share, and a fresh clone of
  this repo doesn't carry them. For `leonxlnx-taste-skill`, the full list is
  `design-taste-frontend`, `minimalist-ui`, `industrial-brutalist-ui`,
  `high-end-visual-design`, `redesign-existing-projects`, `ui-craft`,
  `ui-craft-editorial`, `ui-craft-dense-dashboard`, `product-proof-saas`,
  `operational-enterprise-ai`, `taste` (senlindesign, for Analyze),
  `webapp-testing`, and `a11y-audit` (both for Validate) — check the Skill
  tool's own list, not just whether a file happens to exist on disk.

  **A first-time user should never have to manually run install commands
  before this works.** For every one missing, run its exact `npx skills
  add ...` command from this repo's `README.md#prerequisites` right now,
  the same way this was actually done and verified working end-to-end
  (2026-08): globally (`-g`), non-interactively (`-y`), one Bash call per
  skill. Each install prints a real Gen/Socket/Snyk security scan before
  installing — read it as you go. **One expected exception**: `taste`
  (senlindesign/taste-skill, required for Analyze) reports Snyk **High
  Risk** every time, already investigated directly (its only script is a
  client-side DOM-measurement snippet — colors, fonts, spacing read via
  `getComputedStyle()` — with no network calls beyond the page being
  analyzed, no exfiltration, nothing suspicious); install it anyway without
  stopping, same as every Low/Med-risk package here. If any *other* package
  — one not already named and cleared in `README.md#prerequisites` — comes
  back High Risk, that's genuinely unexpected: stop and tell the user
  plainly what was flagged and why, rather than installing it anyway.
  Everything else proceeds without asking, since that's the entire point of
  "just works" for someone who's never touched this pipeline before. After
  installing, verify each one is now a real, invokable skill (not just a
  file on disk) before continuing — an install can print success and still
  not be picked up by the current session in rare cases, and that's worth
  catching here rather than failing confusingly three stages later. Only if
  an install genuinely fails (network error, registry down) should this
  stage stop and tell the user what failed and why, since that's a real
  blocker no amount of automation can route around.

  Confirmed the hard way, twice: first, an agent read `design-authority.md`'s
  old "already installed" line, took it at face value, and only discovered
  the gap deep inside `extract-design-signal.md` instead of here, where it
  could have been caught for free. Second, even after that got fixed to
  "stop and tell the user to install these by hand," a first-time user still
  had to leave the conversation, run a dozen shell commands themselves, and
  come back — real friction for exactly the person this pipeline is
  supposed to be a low-friction on-ramp for. Auto-installing (with the one
  High-Risk exception above) is the actual fix, not just documenting the
  problem more clearly.

  Same treatment for `baoyu-design` if selected: its own documented gaps
  (`direction_variants`, `redesign_audit`, `preflight_check` all null) still
  apply even once installed, and discover-site.md is where that gets said,
  not discovered later.

## What NOT to do here

- Don't run any design analysis: that's `extract-design-signal.md`, next.
- Don't ask the user "what framework are you using" if `package.json` already
  answers it. Only ask when genuinely ambiguous (e.g. two competing config
  files present).
- Don't make a loomloom call. This stage is 100% local and free.
- Don't skip the asset-capture step because the target "looks like" it has no
  real logo worth capturing — run it and let it report nothing found, rather
  than assuming.
