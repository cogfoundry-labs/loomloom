# LoomLoom npm release tooling

This directory builds the Beta npm distribution from already-validated GitHub
Release assets. It does not build the Go CLI and it does not install the
general LoomLoom Agent Skill.

## Prepare and verify

Download one published Beta release into `release/`, including
`checksums.txt`, then run:

```sh
npm --prefix npm ci --ignore-scripts
node npm/scripts/prepare-packages.mjs --tag vX.Y.Z-beta.N
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

Do not publish from an implementation branch. After the source PR is merged,
the GitHub Beta Release is published, all local Registry tests pass, and a
maintainer explicitly authorizes npm publication, use the reviewed tarballs:

```sh
node npm/scripts/publish-packages.mjs \
  --publish \
  --confirm-tag vX.Y.Z-beta.N
```

The command publishes all six platform packages before the main package. It
always passes `--access public`; platform packages receive dist-tag
`npm-bootstrap`, while only `@cogfoundry/loomloom` receives `beta`. Existing
name/version pairs are skipped only when both registry integrity values match
the locally packed immutable tarball.

The first version requires a logged-in `@cogfoundry` maintainer with 2FA.
After all seven packages exist, configure the Trusted Publisher for every
package with:

- GitHub organization: `cogfoundry-labs`
- Repository: `loomloom`
- Workflow: `npm-release.yml`
- Allowed action: `npm publish`

Only then may the `publish` action of the workflow be used. The workflow uses
GitHub-hosted runners, Node 24, npm 11.19.1, and OIDC. It never moves `latest`.
