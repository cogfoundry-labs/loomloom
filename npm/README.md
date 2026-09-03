# LoomLoom npm release tooling

This directory builds the stable and Beta npm distributions from already-validated GitHub
Release assets. It does not build the Go CLI and it does not install the
general LoomLoom Agent Skill.

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
GitHub-hosted runners, Node 24, npm 11.19.1, and OIDC. The npm Registry may
automatically create `latest` for a brand-new package even when publication
uses another dist-tag; stable releases deliberately take ownership of `latest`.
