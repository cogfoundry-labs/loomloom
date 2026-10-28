# Releasing LoomLoom CLI

This document defines how versions, branches, and tags are cut. The release
workflows enforce it (`scripts/resolve-release-source-branch.sh`).

## Versions

LoomLoom CLI follows [Semantic Versioning](https://semver.org/). While the major
version is 0:

- **Minor** (`0.7.0`) ships new features and may include breaking changes.
- **Patch** (`0.7.1`) ships fixes only.

CLI versions are independent of LoomLoom server versions. Release notes state
which server versions a release was verified against.

## Tags

Only two tag forms are release tags:

| Tag | Channel | npm dist-tag | Homebrew |
| --- | --- | --- | --- |
| `vX.Y.Z-beta.N` | GitHub pre-release | `beta` | no |
| `vX.Y.Z` | GitHub latest release | `latest` | yes |

A stable `vX.Y.Z` must point at the same commit as its highest
`vX.Y.Z-beta.N`: every stable release is a beta that passed verification.
Tags are immutable; a fix means the next beta number, never a moved tag.

Older `-internal.N` and `-rc.N` tags remain installable but are no longer
created.

## Branches

- `main` is the only development branch. Every change lands through a pull
  request.
- **Minor release:** tag `vX.Y.0-beta.N` on `main`. When verification passes,
  tag `vX.Y.0` on the same commit.
- **Patch release:** create `release/X.Y` from `vX.Y.0` the first time a fix is
  needed. Merge the fix into `main` first, then `git cherry-pick -x` it onto
  `release/X.Y`, and tag `vX.Y.Z-beta.N`, then `vX.Y.Z` on the same commit.
  A commit that cannot come from `main` states `Release-only:` and the reason in
  its message.
- `release/X.Y` never merges back into `main`. It is deleted once the next minor
  release ships, after its tags preserve every released commit.

After publishing a stable version, the npm workflow promotes `beta` to that
version unless `beta` already points to an equal or newer version. It verifies
`latest` before promotion and reads `beta` back afterward. A permission or
read-back failure fails the workflow; publication may already have completed,
so repair the tag rather than republishing the immutable version.

Before a stable publication, enable **Allow npm dist-tag** on the Trusted
Publisher for `@cogfoundry/loomloom`, repository `cogfoundry-labs/loomloom`,
workflow `npm-release.yml`, environment `npm-release`. Publishing permission
alone does not grant dist-tag access. See the
[npm Trusted Publisher documentation](https://docs.npmjs.com/trusted-publishers/#managing-dist-tags-with-trusted-publishing).
This permission must be verified separately before release.

## Steps

1. Update `CHANGELOG.md` on the branch you release from.
2. Create an annotated tag and push it:

   ```bash
   git tag -a v0.7.0-beta.1 -m "v0.7.0-beta.1: <summary>"
   git push origin v0.7.0-beta.1
   ```

3. The `release` workflow builds the archives, publishes the GitHub release, and
   mirrors the source branch and tag. The `npm release` workflow publishes to
   npm after the GitHub release is published.
4. Verify `npm view @cogfoundry/loomloom dist-tags` and install the published
   version.
