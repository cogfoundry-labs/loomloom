import assert from 'node:assert/strict';
import { createRequire } from 'node:module';
import { spawnSync } from 'node:child_process';
import { pathToFileURL } from 'node:url';
import { npmRoot, npmCommand, npmCommandArgs, releaseVersion } from './lib.mjs';

const require = createRequire(`${npmRoot}/node_modules/npm/package.json`);
const semver = require('semver');

export function syncBeta(tag, npm) {
  const release = releaseVersion(tag);
  if (release.mainDistTag !== 'latest') return;
  const tags = npm(['view', '@cogfoundry/loomloom', 'dist-tags', '--json']);
  assert.equal(tags.latest, release.version, 'latest must match the stable release before beta promotion');
  if (tags.beta) {
    assert.ok(semver.valid(tags.beta), 'invalid beta dist-tag version');
    if (semver.gte(tags.beta, release.version)) return;
  }
  npm(['dist-tag', 'add', `@cogfoundry/loomloom@${release.version}`, 'beta']);
  const after = npm(['view', '@cogfoundry/loomloom', 'dist-tags', '--json']);
  assert.equal(after.beta, release.version, 'beta promotion read-back mismatch');
}

if (process.argv[1] && import.meta.url === pathToFileURL(process.argv[1]).href) {
  syncBeta(process.env.RELEASE_TAG, (args) => {
    const result = spawnSync(npmCommand, [...npmCommandArgs, ...args,
      '--registry', 'https://registry.npmjs.org'], { encoding: 'utf8', timeout: 60000 });
    if (result.error || result.status !== 0) {
      throw new Error(`npm ${args[0]} failed: ${result.error?.message ?? result.stderr}`);
    }
    return args[0] === 'view' ? JSON.parse(result.stdout) : undefined;
  });
}
