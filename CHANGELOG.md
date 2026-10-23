# Changelog

All notable changes to loomloom are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

> loomloom is in beta (0.x). Minor versions may include breaking changes; these
> are called out under **Changed** and in the corresponding
> [release notes](https://github.com/cogfoundry-labs/loomloom/releases).
> Entries below were reconstructed from git history and release tags.

## [Unreleased]

### Added
- `template-spec preview` serves a tiny loopback page; the browser loads an independently published HTTPS viewer. Drafts and session credentials stay local, with no embedded UI package, cloud identity or contract lookup.
- `template-spec view-link` returns a Server-provided URL for one exact saved
  private version without creating a temporary preview. Agent guidance explains
  the separate viewing, validation, saving, and execution steps.
- `listing publish` and `listing payment-settings` accept
  `--subscription-tiers-file` to send initial monthly/yearly prices atomically
  with subscription mode. Strict JSON and money validation preserves explicit
  disabled tiers and zero prices; later repricing continues through the
  revision-checked bundle tier command.
- `template-spec authoring-context` now exposes dynamic Capability Profile
  definitions, operations defaults, default-model availability, and fixed
  input/output ports for text, image, and video authoring.
- The bundled LoomLoom Agent Skill now selects dynamic multimedia Profiles when
  callers need a replaceable model set, while retaining Fixed Model Contracts
  for exact-model or dedicated-interface workflows.

### Changed
- Local draft preview defaults to the ShengSuanYun production viewer. Explicit
  `--viewer-url` overrides `LOOMLOOM_VIEWER_BOOTSTRAP_URL`, which overrides the
  built-in default; cloud Server and API credentials remain independent.
- Agent authoring now explicitly hands off the local draft preview link before
  review and saving. Setup documents the separate viewer URL and same-version
  CLI/Skill Beta installation; preview readiness is not browser rendering.
- `template-spec create` and `create-version` include the exact saved-version
  view link when available. Link lookup failures preserve save success and IDs,
  so callers can retry only `view-link` without duplicating a version.
- `creator earnings` now reads Billing-authoritative posted income, defaults to
  both pay-per-use and released subscription sources, and supports stable page
  tokens without switching to the subscription accrual endpoint.
- Subscription ID filtering remains transaction-specific; use
  `creator transactions --source subscription --subscription-id <id>` instead
  of the removed `creator earnings --subscription-id` combination.
- The text authoring-context table includes dynamic Profile details while
  retaining the existing identity and legacy contract columns.
- `capability resolve` text output no longer appends an empty revision suffix to
  dynamic Profile IDs.

### Security
- Updated npm release tooling to npm 11.21.1 and replaced Verdaccio with a
  loopback-only Registry fixture for offline distribution tests. CI and
  publication use the same pinned npm version and retain the security audit.

## [0.3.3] - 2026-08-26

### Added
- Capability-oriented template discovery: `loomloom capability resolve` surfaces
  the capability inputs a template requires before you run it.

### Changed
- TemplateSpec v1 → v2 upgrades now preserve the original semantics instead of
  silently normalizing them.

### Fixed
- Vision profile artifact ports (e.g. image inputs) are no longer dropped when a
  spec is compiled.
- Stopped emitting model-availability fields that were removed from the catalog.

### Community examples
- redesign-lab: reconstruct the full "before" hero (not just its video) for
  sites whose CSP blocks framing.

## [0.3.2] - 2026-08-25

### Documentation
- Clarified where the loomloom Agent Skill must be installed for each agent
  (the `--skill-dir` root), across the README and installation guide.

## [0.3.1] - 2026-08-25

### Added
- Authoring context now exposes model step types.

### Documentation
- Added a migration guide for v1 media templates.

### Community examples
- redesign-lab: added the tabbyml.com case study; decoupled the compare
  widget's before/after content types and added a video fallback for
  CSP-blocked embeds.

## [0.3.0] - 2026-08-24

### Added
- TemplateSpec v2, now server-authoritative.
- Redesign Lab community example under `examples/community/redesign-lab`.

### Documentation
- RFC-0003: model catalog strategy.
- Complete CLI flag and shell-completion reference.
- README: CI/activity badges, DeepWiki, and a "Hot topics" section.

## [0.2.2] - 2026-08-11

### Changed
- Repository reorganized (phase I) around a single, unified loomloom skill
  distribution path (`skills/loomloom`).

### Documentation
- RFC-0001 (intent-first CLI) and RFC-0002 (command model, output, migration),
  plus a CLI cheat sheet.
- README and reference-doc restructure.

### Fixed
- Hardened the release mirror CI; extended Gitee synchronization timeouts.

## [0.2.1] - 2026-08-06

First public beta release.

### Added
- Multi-platform browser authentication (`loomloom login` / `loomloom logout`).

### Documentation
- Gitee fallback and platform API guidance.

### Fixed
- Uninstall preserves user files.
- Homebrew release downloads are resilient to transient failures.
- Release-tag commits are compared across mirrors before publishing.

## [0.2.0] - 2026-08-03

### Added
- Multiple named server profiles; `loomloom server list` / `use` / `remove`.
- Credentials (browser and token) are bound to the selected server.
- CogFoundry platform setup guidance.

### Fixed
- Safer uninstall path handling; clean uninstall state.
- Gitee prerelease publish and install.
- Skill responses match the user's language.
- Market public input schema is preserved through authoring.

## [0.1.5] - 2026-07-24

Consolidates the 0.1.1–0.1.5 patch series.

### Added
- Browser-based login (`loomloom login` / `loomloom logout`).
- Staged official template execution.
- Skill test-execution mode, gated on a confirmed non-zero precheck.

### Documentation
- Bilingual (English / Simplified Chinese) TemplateSpec handbook.
- Reference-doc restructure; added community health files.

### Fixed
- Numerous Gitee release-mirror reliability fixes (idempotent republishing,
  interrupted-transfer retries, empty-response handling).
- Published release assets are kept immutable.
- Removed the ripgrep release dependency.

## [0.1.0] - 2026-07-09

Initial release.

### Added
- loomloom CLI and the bundled loomloom Agent Skill.
- TemplateSpec authoring and execution.
- Market pricing shown in user-facing money units.

[Unreleased]: https://github.com/cogfoundry-labs/loomloom/compare/v0.3.3...HEAD
[0.3.3]: https://github.com/cogfoundry-labs/loomloom/compare/v0.3.2...v0.3.3
[0.3.2]: https://github.com/cogfoundry-labs/loomloom/compare/v0.3.1...v0.3.2
[0.3.1]: https://github.com/cogfoundry-labs/loomloom/compare/v0.3.0...v0.3.1
[0.3.0]: https://github.com/cogfoundry-labs/loomloom/compare/v0.2.2...v0.3.0
[0.2.2]: https://github.com/cogfoundry-labs/loomloom/compare/v0.2.1...v0.2.2
[0.2.1]: https://github.com/cogfoundry-labs/loomloom/compare/v0.2.0...v0.2.1
[0.2.0]: https://github.com/cogfoundry-labs/loomloom/compare/v0.1.5...v0.2.0
[0.1.5]: https://github.com/cogfoundry-labs/loomloom/compare/v0.1.0...v0.1.5
[0.1.0]: https://github.com/cogfoundry-labs/loomloom/releases/tag/v0.1.0
