# npm Distribution Beta Design

## Meta

- Date: `2026-09-02`
- Baseline: `origin/main@9f929a61dffe65c712862662353c3155093f416c`
- Branch: `feature/npm-distribution-beta`
- Scope: npm Beta distribution only
- Status: approved for implementation; npm publication remains a separate human gate

## Goal

Publish `@cogfoundry/loomloom@beta` as a small Node.js launcher backed by one
platform-specific package. Installation must not download a Go binary from
GitHub at install time. GitHub Release tags, archives, and `checksums.txt`
remain the build and version source of truth.

The Beta is complete when a clean npm installation and `npx` invocation run
the existing Go CLI on supported platforms, missing or tampered platform
packages fail explicitly, and existing GitHub, Gitee, GitLab, Homebrew, and
installer flows remain unchanged.

## Package Contract

The Beta publishes seven packages at one exact prerelease version:

- `@cogfoundry/loomloom`
- `@cogfoundry/loomloom-linux-x64`
- `@cogfoundry/loomloom-linux-arm64`
- `@cogfoundry/loomloom-darwin-x64`
- `@cogfoundry/loomloom-darwin-arm64`
- `@cogfoundry/loomloom-win32-x64`
- `@cogfoundry/loomloom-win32-arm64`

Only tags matching `vX.Y.Z-beta.N` are accepted. The npm version omits the
leading `v`; the main package uses dist-tag `beta`. Platform packages are
exact-version optional dependencies and use the non-user-facing
`npm-bootstrap` dist-tag.

The generator consumes already-packaged Release archives. It verifies their
existing SHA-256 entries, extracts one binary per platform package, records
the extracted binary SHA-256, and never runs `go build`.

## Runtime Behavior

The main package exposes `loomloom` through its `bin` field. The launcher maps
Node's `process.platform` and `process.arch` to one optional dependency,
resolves its binary, validates the binary against the SHA-256 frozen in the
main package, and then starts it with inherited arguments, environment,
working directory, and stdio.

Unsupported platforms, omitted optional dependencies, missing binaries, and
checksum mismatches exit nonzero with an actionable error. There are no npm
lifecycle download scripts and no implicit Agent Skill directory writes.

## Publication Lifecycle

1. The GitHub Release is built, published, downloaded, and checksum-validated.
2. Seven isolated npm staging directories are generated with strict `files`
   allowlists.
3. `npm pack --json --dry-run` verifies every public file before tarballs are
   produced.
4. All six platform packages are published or proven byte-identical first.
5. The main package is published last with dist-tag `beta`.
6. A fresh official-registry install, `npx`, hash check, and uninstall close
   the npm Beta release.

For the first version, a team maintainer uses 2FA and publishes the reviewed
tarballs manually. After all packages exist, each package is bound to the
same GitHub Actions Trusted Publisher workflow. Later reruns compare npm
tarball integrity and the binary checksum derived from the Release assets;
an existing mismatch blocks because npm versions are immutable.

No workflow in this change moves `latest`. No npm publication is authorized
by merging this implementation branch.

## Skill Boundary

This Beta installs the CLI only. Existing installers continue to install the
general LoomLoom Agent Skill only when an explicit `--skill-dir` is supplied.
The existing `loomloom skill install` command remains dedicated to generated
Market and TemplateSpec skills and is not overloaded.

## Verification

- launcher unit tests: platform mapping, missing dependency, and checksum
  mismatch;
- package generation from six checksum-verified archives;
- exact `npm pack` content allowlists;
- offline Verdaccio publication with platform packages first and main last;
- clean-prefix global install, `loomloom --version`, `loomloom --help`,
  `npm exec`, and `npx`;
- explicit `--omit=optional` failure;
- Windows `.cmd` shim validation in GitHub Actions;
- existing Go and Release script regression tests.
