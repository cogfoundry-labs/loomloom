# LoomLoom npm release tooling

This directory builds the stable and Beta npm distributions from already-validated GitHub
Release assets. It does not build the Go CLI and it does not install the
general LoomLoom Agent Skill during npm lifecycle scripts.

The published main package provides an explicit setup command for CLI plus one
named Agent Skill:

```sh
npx @cogfoundry/loomloom@<version> install --agent codex --yes
```

It installs the same-version global CLI, then uses the bundled general
`loomloom` Skill from that npm package for the named Agent. The Skill is copied
to the Agent's selected root, so it does not depend on a GitHub checkout after
installation. Ordinary `npm install -g @cogfoundry/loomloom` remains CLI-only:
it does not write to any Agent Skill directory.

For a Beta, replace `<version>` with the exact requested Beta version (for
example `0.6.2-beta.3`). `loomloom update --agent codex` follows stable `latest`;
it does not select a Beta. Verify the installed Skill separately from
`loomloom --version`. Local preview uses the ShengSuanYun production viewer by
default; see [workflow preview](../docs/guides/workflow-preview.md) for overrides.

## Prepare and verify

Download one published stable or Beta release into `release/`, including
`checksums.txt`, then run:

```sh
npm --prefix npm ci --ignore-scripts
node npm/scripts/prepare-packages.mjs --tag vX.Y.Z
node npm/scripts/verify-packages.mjs
node npm/scripts/pack-packages.mjs
node npm/scripts/test-registry.mjs
```

Generated public package roots live under `dist/npm/`; publishable tarballs
and their frozen integrity manifest live under `dist/npm-tarballs/`. Both are
ignored build outputs.

Offline Registry tests use a loopback-only HTTP fixture in a Worker, accepting
only the seven packages and immutable tarballs in the publication manifest.
It has no upstream Registry, persistent storage, or Verdaccio dependency. These
tests exercise the real npm client and publication script, including read-back,
repeat publication, install, exec, npx, and uninstall; they do not validate
production OIDC or npm Registry policy. CI retains `npm audit --audit-level=high`
against the public Registry for the tooling dependencies.
Test commands use a temporary working directory, npm configuration and cache,
which are removed along with the fixture when the test finishes.

The current npm 11.21.1 tarball still bundles `ip-address@10.7.2`, affected by
[GHSA-rxp7-6hxx-x8q5](https://github.com/advisories/GHSA-rxp7-6hxx-x8q5).
The checked npm 12.2.1 tarball also contains that version, and npm overrides do
not replace bundled files. This remaining moderate advisory is reported by
`npm audit`; it does not trip the existing high-severity gate. Update the pinned
npm client when an upstream release includes the fixed dependency; do not hide
the report or edit installed bundle contents to make the audit pass.
The fixed dependency version is 10.7.4. The affected dependency is part of the
private release tooling, not a dependency shipped in the LoomLoom npm packages
or Go executable. Passing the CI gate does not mean the tooling is advisory-free.

To inspect the official-registry repair plan without writing anything:

```sh
node npm/scripts/publish-packages.mjs
```

## First publication gate

Do not publish from an implementation branch. Pushing a protected
`vX.Y.Z` or `vX.Y.Z-beta.N` tag is the npm publication authorization: the
`npm-release.yml` workflow waits for the matching GitHub Release, validates its
assets, runs the local Registry tests, and publishes the reviewed tarballs.

`workflow_dispatch` remains available for a prepare-only diagnostic run or an
idempotent repair of an already-published stable or Beta release:

```sh
node npm/scripts/publish-packages.mjs \
  --publish \
  --confirm-tag vX.Y.Z
```

The command publishes all six platform packages before the main package. It
always passes `--access public`; platform packages receive dist-tag
`npm-bootstrap`, while `@cogfoundry/loomloom` receives `latest` for a stable
release and `beta` for a Beta release. Existing
name/version pairs are skipped only when both registry integrity values match
the locally packed immutable tarball. After a new publish, it reads Registry
metadata through a fresh npm cache until the expected integrity appears; tune
the bounded wait with `--readback-attempts` and `--readback-interval-ms`.

The first version requires a logged-in `@cogfoundry` maintainer with 2FA.
After all seven packages exist, configure the Trusted Publisher for every
package with:

- GitHub organization: `cogfoundry-labs`
- Repository: `loomloom`
- Workflow: `npm-release.yml`
- Environment: `npm-release`
- Allowed action: `npm publish`

Only then may the `publish` action of the workflow be used. The workflow uses
GitHub-hosted runners, Node 24, npm 11.21.1, and OIDC. The npm Registry may
automatically create `latest` for a brand-new package even when publication
uses another dist-tag; stable releases deliberately take ownership of `latest`.
