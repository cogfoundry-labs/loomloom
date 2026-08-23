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
git clone https://github.com/cogfoundry-labs/loomloom
cd loomloom/examples/community/redesign-lab
```

Install this as a Claude Code skill (or open the folder directly in a
project that has one), then in an agent session:

```
Redesign my website. Give me a fixed-baseline option plus a couple of
substantially different design directions before implementing anything.
```

Nothing before the final Share stage requires loomloom to be installed at
all — Discover through Gate 3 (three human decision points: pick a
direction, pick a variant, approve the build) run entirely on your own
coding agent and local scripts, for $0. loomloom is only needed if you go
on to generate the interactive case study at the end, and even then it's
one real API call, with a real cost estimate shown before anything is
spent.

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
