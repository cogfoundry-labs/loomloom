# Redesign Lab

**What if an AI coding agent could redesign a real website — not from a prompt, but from a workflow?**

Give it an existing site. It analyzes the real thing, builds several genuinely different design directions as real working pages—not mockups, lets you choose, builds the winner, and validates the result with real accessibility and functional checks.

Built as a Claude Code skill on top of a pluggable design-authority contract.

This started as a [loomloom](https://github.com/cogfoundry-labs/loomloom) example, exploring how a reusable AI-workflow component could fit into a larger agent-native workflow. loomloom is completely optional, though. It is used only in the final Share step to turn the verified results into narrative prose for a case study. If you just want to redesign a website, you don't need loomloom at all.


> "Don't choose from mockups. Choose from real working designs."

## See it in action

Two real redesigns, start to finish — the actual comparison widget from each finished case study, dragged end to end so you can watch both the before and after pages animate for real, not a screenshot.

<div align="center">
  <video src="https://github.com/user-attachments/assets/3c2c8fb7-68db-4e67-9514-fb93b91612b3" width="100%" controls></video>
</div>

> **aider.chat** — traded Inter's extra-bold headline for an uppercase Archivo Black display face, tightening letter-spacing for a more assertive visual hierarchy. → [Full case study](https://maxaibuilds.github.io/aider-redesign/)

<div align="center">
  <video src="https://github.com/user-attachments/assets/9c00a67a-a4d4-414f-ab00-efb7f8714600" width="100%" controls></video>
</div>

> **tabbyml.com** — consolidated a ~40-token color inventory down to 8 deliberate values, upgraded headline and nav type to a true cross-platform Geist Mono stack, and squared off previously rounded corners for a sharper, angular finish. → [Full case study](https://maxaibuilds.github.io/tabbyml-redesign/)

Each case study shows the actual process — the real before/after diff, the real accessibility scan, the real cost of the one paid step — not just a final screenshot.

## Why I built this

I'm not a web designer. I built Redesign Lab while trying to develop an example SkillBot for loomloom, and it turned into a question I actually wanted an answer to:

**How far can an AI coding agent get at website design if you give it good design *skills*, instead of throwing lots of *prompts* at it?**

I didn't try to invent a design system myself — I'm not qualified to. Instead I went looking for some of the most popular, open-source AI design skills already on GitHub, and built a workflow that composes them: one skill measures the existing site, another supplies the design rules and direction ideas, another checks the result actually works, another checks it's accessible. Redesign Lab is the orchestration holding those pieces together, plus the human decision points in between.

Because I'm not the person who should be judging whether any of this is *good design*, the whole thing is built to be open: every piece — the design authority, the styling rules, the validation checks — is meant to be swappable. If you already have a skill you trust, or a house design system, you should be able to plug it in instead of the defaults below.

**If you're an experienced web designer or design engineer, I'd genuinely like your read on it** — does this actually work, where does it make bad calls, and what would it take to make this genuinely useful rather than just an interesting experiment? See [Help me test this](#help-me-test-this) below.

## Quickstart

```bash
npx skills add cogfoundry-labs/loomloom --skill redesign-lab -a claude-code -g -y
```

That's it — no `git clone`, no `cd`. This installs the skill globally
(`-g`), the same way every skill this pipeline itself depends on gets
installed, so it's available in any Claude Code session afterward
regardless of which directory you're working in. 

Then, in a Claude Code session, or an agent session, just say:

```
redesign https://aider.chat
```

Or, if you're already on the webpage you want to redesign:

```
redesign this webpage
```

Redesign Lab takes it from there: it analyzes the existing site, explores different design directions, lets you choose, implements the winner, and validates the result.

Nothing else to install first. The design-authority and validation skills the pipeline depends on aren't bundled with Redesign Lab, but you don't need to install them yourself. The first stage, Discover, checks what's already present and automatically installs anything that's missing before continuing.

See [Prerequisites](#prerequisites--the-design-authority-installed-automatically) below for exactly what gets installed and why, or pre-install everything yourself if you'd rather review each dependency before it runs.


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

You can steer the request, too:

```
Redesign https://example.com. Keep the existing information architecture,
but explore 3 substantially different visual directions before
implementing anything.
```

The agent handles discovery, building, and validating. You make the
decisions that actually matter — which direction, which variant, whether
to ship.

## What actually happens

Redesign Lab is deliberately not `prompt → generate website`. It's closer to:

```
Existing website
      │
      ▼
Discover  →  Analyze
      │
      ▼
Direction Slices ──▶ GATE 1 — you pick a direction
      │
      ▼
Explore Variants ──▶ GATE 2 — you pick a variant
      │
      ▼
Implement  →  Validate ──▶ GATE 3 — you approve the build
      │
      ▼
GATE 4 — Share confirmation (the one paid step)
      │
      ▼
Case Study
```

Four gates, not zero. The agent explores and executes; you decide what's
worth pursuing at every point that actually matters. That's the whole
idea — not a smarter autopilot, a workflow with real stopping points built
in.

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

## Built on open-source design skills — real thanks, not name-dropping

Neither of the two case studies above was produced by one big, opinionated
"design AI." Every real design judgment behind them came from someone
else's open-source work — Redesign Lab is the orchestration and the human
decision gates around it, not the thing actually deciding what looks good.
Seven projects, thirteen individual skills between them, grouped by what
each one actually does in the pipeline:

**Design authority** — [Leonxlnx/taste-skill](https://github.com/Leonxlnx/taste-skill),
the single biggest contributor here: its build rules, its pre-flight
check, its redesign audit (`redesign-existing-projects`), and 4 of the 8
direction-variant families (`design-taste-frontend`, `minimalist-ui`,
`industrial-brutalist-ui`, `high-end-visual-design`) are where most of a
run's actual design judgment comes from. This is the default authority —
fully replaceable, see [below](#bring-your-own-design-authority).

**More direction families, so 3 picks don't converge on the same
skeleton** — [educlopez/ui-craft](https://github.com/educlopez/ui-craft)
(`ui-craft`, `ui-craft-editorial`, `ui-craft-dense-dashboard`) and
[mengto/skills](https://github.com/mengto/skills) by MengTo of
Design+Code (`product-proof-saas`, `operational-enterprise-ai`) — both
contributed genuinely **structural**, not just aesthetic, direction
families: real content-organizing decisions, not another color palette on
the same hero-plus-grid layout.

**Understanding the existing site** —
[senlindesign/taste-skill](https://github.com/senlindesign/taste-skill)
measures the real, existing site — screenshots and DOM extraction —
before anything is redesigned, so Analyze starts from evidence, not a
guess.

**Making sure it actually works** —
[anthropics/skills · webapp-testing](https://github.com/anthropics/skills/tree/main/skills/webapp-testing)
renders and screenshots every direction, variant, and final build for
real, so results get inspected rather than assumed.
[snapsynapse/skill-a11y-audit](https://github.com/snapsynapse/skill-a11y-audit)
runs a real axe-core accessibility scan at Validate — both case studies
above finished at **0 violations**.

**The one paid step** —
[cogfoundry-labs/loomloom](https://github.com/cogfoundry-labs/loomloom)
turns the run's verified facts into the case study's narrative prose. The
aider run cost **$0.0089**.

Thank you to every one of these projects and their maintainers — none of
the above would be possible without the work you've already done and
published openly. If you maintain any of them and want something changed
about how you're credited or used here, please open an issue and I'll fix
it.

This is also why Redesign Lab exists as a loomloom example in the first
place: it's a real, working demonstration of loomloom sitting inside a
much larger agent workflow as one small, clearly-scoped execution
component — not the engine driving the whole thing. Most loomloom
examples show a TemplateSpec calling a single execution unit directly;
this one shows the other end of the spectrum, for anyone building a
SkillBot that's more "real product workflow" than "single template."

## Bring your own design authority

This is the part I'd most like people to push on. Redesign Lab separates
the *workflow* (discover, explore, gate, implement, validate) from the
*design authority* (what good design means, which directions to explore,
what to preserve, what to reject). The workflow doesn't know or care which
authority is plugged in:

```
Redesign Lab  +  your design skill  →  your methodology, your redesign workflow
```

You don't have to agree with Leonxlnx/taste-skill's calls. That's rather
the point — write your own design-authority skill, point at your team's
internal system, or find a different one on GitHub, and the same gates,
the same validation, the same case-study output apply to it too. See
`references/design-authority.md` for the exact contract a new authority
needs to fill.

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

## What this is — and isn't

**It is**: an open experiment in agent-native website design, a real
Claude Code skill you can install today, a pluggable design-authority
system, and a working example of loomloom inside a larger agent workflow.

**It isn't**: a claim that AI replaces web designers, one "correct" AI
designer model, a finished design methodology, or a guarantee of good
design. It's stage one of finding out whether this general shape is worth
building further.

## Help me test this

If you're a web designer, design engineer, or you build agent skills
yourself — try it against a site you actually know well, then tell me:

- Where did the workflow make a bad design call?
- Which decisions should the agent own, and which should stop for a human?
- What design skill is missing that you'd want to plug in?
- What would make this genuinely useful to a working designer, not just an
  interesting demo?

And if the whole approach seems wrong to you, I'd like to hear that too —
the goal isn't to defend this specific version, it's to find out what an
actually useful AI-native design workflow looks like. Open an issue, or
find me in [Show and tell](https://github.com/orgs/cogfoundry-labs/discussions/categories/show-and-tell).

## Next step

If enough people find this genuinely useful, the next thing I want to
explore is turning Redesign Lab into a place where designers and
skill-builders can contribute their own design authorities, styling
systems, evaluation checks, and redesign methodologies — so instead of one
AI designer, there's a workflow where different design skills can be
swapped in, compared, and improved on. That only makes sense to build if
this first version actually holds up, which is exactly what I'm trying to
find out.

---

[→ Quickstart](#quickstart) · [→ Aider case study](https://maxaibuilds.github.io/aider-redesign/) · [→ TabbyML case study](https://maxaibuilds.github.io/tabbyml-redesign/) · [→ Technical spec](docs/design-spec.md)
