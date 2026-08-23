# Redesign Lab

An agent-native workflow that turns an existing website into several real,
working design directions, lets a human choose, then builds the winner.
Built as a Claude Code skill on top of a pluggable design-authority contract,
with [loomloom](https://github.com/cogfoundry-labs/loomloom) doing exactly
one job in the whole pipeline: writing the narrative prose for the final
case study.

> "Don't choose from mockups. Choose from real working designs."

## Why this is useful as a loomloom example

Most loomloom examples show a TemplateSpec calling a single execution unit
directly. This one shows the other end of the spectrum: a full multi-stage
agent skill that treats loomloom as one small, clearly-scoped component
inside a much larger, mostly-free, mostly-local pipeline — Discover, Analyze,
generate design directions, gate on human choice, implement, validate, gate
again, and only then spend real money on one batched `text-generate` call.
If you're building a SkillBot that's more "real product workflow" than
"single template," this is the shape to copy.

## Quickstart

```bash
npx skills add cogfoundry-labs/loomloom --skill redesign-lab -a claude-code -g -y
```

That's it — no `git clone`, no `cd`. This installs the skill globally
(`-g`), the same way every skill this pipeline itself depends on gets
installed, so it's available in any Claude Code session afterward
regardless of which directory you're working in. Then, in an agent
session:

```
Redesign my website. Give me a fixed-baseline option plus a couple of
substantially different design directions before implementing anything.
```

**Nothing else to install first.** The design-authority skills this
pipeline depends on beyond itself (Analyze, Direction Slices, Validate)
aren't bundled with it, but you don't need to install them yourself either
— the first stage, Discover, checks what's already present and installs
whatever's missing automatically (globally, non-interactively, one real
security scan printed per package as it happens) before continuing. See
"Prerequisites" below for exactly what gets installed and why, or to
pre-install everything yourself ahead of time if you'd rather review each
one before it runs.

Want to read or modify the source instead of just using it? Clone the full
`loomloom` repo — `redesign-lab` lives inside it as a community example,
not as its own standalone repo, so there's no lighter way to get the
source itself:

```bash
git clone https://github.com/cogfoundry-labs/loomloom
cd loomloom/examples/community/redesign-lab
```

Nothing before the final Share stage requires loomloom to be installed at
all — Discover through Gate 3 (three human decision points: pick a
direction, pick a variant, approve the build) run entirely on your own
coding agent and local scripts, for $0. loomloom is only needed if you go
on to generate the interactive case study at the end, and even then it's
one real API call, with a real cost estimate shown before anything is
spent.

## Prerequisites — the design authority (installed automatically)

Every stage after Discover — Analyze, Direction Slices, Validate — reads
its rules from the active design authority (`references/design-authority.md`)
plus a couple of infrastructure skills (`taste` for Analyze,
`webapp-testing`/`a11y-audit` for Validate), and none of that is bundled in
this repo: they're separate skill packages this pipeline reads at runtime,
the same way it reads loomloom at Share. **`skills/discover-site.md` installs
whichever of these aren't already present, automatically, the first time you
run this pipeline** — the list below is exactly what it runs, shown here so
you know what's happening to your machine, and so you can run it yourself
ahead of time if you'd rather review each install before it happens (via
the [skills CLI](https://skills.sh)):

```bash
# Default design authority (Leonxlnx/taste-skill): build rules, 4 of 8
# direction variants, and the redesign audit used at Analyze/Implement
npx skills add https://github.com/Leonxlnx/taste-skill --skill "design-taste-frontend" -a claude-code -g -y
npx skills add https://github.com/Leonxlnx/taste-skill --skill "minimalist-ui" -a claude-code -g -y
npx skills add https://github.com/Leonxlnx/taste-skill --skill "industrial-brutalist-ui" -a claude-code -g -y
npx skills add https://github.com/Leonxlnx/taste-skill --skill "high-end-visual-design" -a claude-code -g -y
npx skills add https://github.com/Leonxlnx/taste-skill --skill "redesign-existing-projects" -a claude-code -g -y

# The other 4 of 8 direction variants (structural/IA families)
npx skills add educlopez/ui-craft --skill "ui-craft" -a claude-code -g -y
npx skills add educlopez/ui-craft --skill "ui-craft-editorial" -a claude-code -g -y
npx skills add educlopez/ui-craft --skill "ui-craft-dense-dashboard" -a claude-code -g -y
npx skills add mengto/skills --skill "product-proof-saas" -a claude-code -g -y
npx skills add mengto/skills --skill "operational-enterprise-ai" -a claude-code -g -y

# Analyze stage: taste measurement
npx skills add https://github.com/senlindesign/taste-skill -a claude-code -g -y
# Playwright MCP (below) is what taste's own SKILL.md asks for, but it's not
# actually required: extract-design-signal.md falls back to whatever
# browser-automation tool this session already has (confirmed working with
# Claude Code's built-in Browser tool). Only run this if Analyze specifically
# says it has no browser tool available at all:
# claude mcp add playwright -s user -- npx -y @playwright/mcp@latest --isolated
# (restart Claude Code after that line so the new MCP tools load)

# Validate stage
npx skills add https://github.com/anthropics/skills --skill "webapp-testing" -a claude-code -g -y
npx skills add https://github.com/snapsynapse/skill-a11y-audit -a claude-code -g -y
```

Notes:
- `-g` installs globally so the authority is available in any project;
  drop it to install into just the current project instead.
- Each `npx skills add` prints a security scan (Gen/Socket/Snyk) before
  installing — read it, don't just trust the badge. At the time of writing,
  everything above came back Safe/Low Risk except `taste-skill`'s `taste`
  package (Snyk: High Risk) and `skill-a11y-audit` (Gen/Snyk: Med Risk); both
  were inspected directly before use here (`taste`'s only script is a
  client-side DOM-measurement snippet with no network calls beyond the page
  being analyzed) — do the same rather than skipping the check.
- `discover-site.md` checks these are actually present before Analyze runs
  and installs whichever are missing itself, automatically, right at
  Discover — the commands above are exactly what it runs. It only stops and
  asks you if something goes genuinely wrong: an install fails outright, or
  a package reports High Risk that isn't the one already-vetted exception
  (`taste`) documented above.

## What's in this folder

| Path | What it is |
|---|---|
| `SKILL.md` | Skill entry point — triggers, capabilities, top-level flow |
| `pipelines/redesign-existing-site.yaml` | The pipeline definition |
| `skills/*.md` | Seven stage-level skill files (Discover, Analyze, Direction Slices, Explore Variants, Implement, Validate, Share) |
| `scripts/*.py` | Local, free, no-model-call tooling — screenshot capture, mechanical scoring, a real computed-style before/after diff, and the one script that calls loomloom |
| `references/*.md` | The design-authority contract, style genome, preservation contract, gate/approval policy, and schemas |
| `test-fixtures/hello-world-site/` | A small synthetic site for exercising Implement → Validate → Share without needing a live external target |

See `docs/design-spec.md` for the full technical specification.

## Where loomloom fits

```
Discover → Analyze → Direction Slices → GATE 1 → Explore Variants → GATE 2
  → Implement → Validate → GATE 3 → GATE 4 (Share, paid) → Case Study
```

Every stage above is local and free except one call inside Share: a batched
`text-generate` pass that turns a mechanically-computed before/after diff
into narrative prose for the case study's chapters. That's it — loomloom
never renders a design, never scores a variant, never makes a gate decision.
