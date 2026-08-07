# RFC-0001: Intent-first CLI

- **Status:** Draft
- **Author:** Max
- **Created:** 2026-08-07

## Abstract

This RFC proposes redesigning the [loomloom](../../README.md) CLI so that commands express
**what the user wants to do**, not which internal resources they are talking to.

The current CLI maps closely to backend concepts. That worked for early
development, but it does not scale well for a public product used by developers
and AI agents.

## Motivation

The current [CLI](../reference/cli.md) works, but it has clear problems:

- Commands are organized around internal resources (`template`, `template-spec`,
  `listing`, etc.) instead of user goals.
- The same job — run work, publish work, check status — is split across multiple
  command groups with different names and flags.
- Users see internal details they should not have to care about
  (`client-request-id`, unit conversion, official vs. private vs. market
  distinctions).
- Naming is inconsistent (`validate-file`, `precheck-file`, `submit-file`,
  `result-workbook`, `create-version`…).
- Adding features keeps expanding the top-level surface.

These issues make the CLI harder to learn, harder for AI agents to use correctly,
and harder to maintain as the project grows. Fixing them one by one will not be
enough — we need a cleaner starting point.

## Goals

The CLI should:

- Let users say what they want (`run`, `build`, `publish`, `install`, `search`…)
  instead of which backend object to touch.
- Work well for both humans and AI agents.
- Keep the common path short and obvious.
- Hide internal details by default.
- Stay small at the top level even as features are added.

## Non-goals

This RFC does not change:

- Backend APIs
- Runtime or execution engine
- Marketplace rules
- Auth or server management (except how they appear in the CLI)

Only the command-line interface is in scope.

## Design principles

- **Intent first** — Primary commands are the actions a user wants to perform.
  Resource types are secondary and inferred when possible.
- **Agent-friendly** — Stable behavior, predictable output, clear errors, and
  safe defaults for non-interactive use.
- **Progressive disclosure** — Most users need only a few high-level commands.
  Advanced options stay available but out of the way.
- **Convention over configuration** — Infer input type, resource scope, and
  context where it is safe to do so; provide flags to override.
- **Good for humans and machines** — Human-readable and machine-readable output
  are both first-class.

## Success criteria

- Common tasks need fewer commands and less documentation.
- AI coding assistants produce correct loomloom commands more often.
- The top-level command set stays small and stable.
- Internal concepts stay hidden unless the user asks for them.
- New features mostly extend existing commands instead of adding new top-level
  groups.

## Open questions

- Pure intent-first, or keep a few explicit resource namespaces for power users?
- Which old commands (if any) keep their current names for compatibility?
- How do we migrate existing users and agents?
- Do we provide aliases for old command names, and for how long?
- How do future features fit the new model without recreating the current
  fragmentation?

## Next steps

This RFC only sets the direction. Follow-up RFCs will cover:

- Command hierarchy
- Naming
- Output format
- Agent guidelines
- Migration plan

Comments on the principles and open questions are welcome before those are
written.
