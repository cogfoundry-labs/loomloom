import assert from 'node:assert/strict';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import { createRequire } from 'node:module';
import { spawnSync } from 'node:child_process';
import { pathToFileURL } from 'node:url';
import { npmRoot, npmCommand, npmCommandArgs, releaseVersion } from './lib.mjs';

const require = createRequire(`${npmRoot}/node_modules/npm/package.json`);
const semver = require('semver');

export function syncBeta(tag, npm, { attempts = 12, delay = () => Atomics.wait(new Int32Array(new SharedArrayBuffer(4)), 0, 0, 5000) } = {}) {
  const release = releaseVersion(tag);
  if (release.mainDistTag !== 'latest') return;
  const tags = npm(['view', '@cogfoundry/loomloom', 'dist-tags', '--json']);
  assert.equal(tags.latest, release.version, 'latest must match the stable release before beta promotion');
  if (tags.beta) {
    assert.ok(semver.valid(tags.beta), 'invalid beta dist-tag version');
    if (semver.gte(tags.beta, release.version)) return;
  }
  npm(['dist-tag', 'add', `@cogfoundry/loomloom@${release.version}`, 'beta']);
  for (let attempt = 0; attempt < attempts; attempt++) {
    const after = npm(['view', '@cogfoundry/loomloom', 'dist-tags', '--json']);
    if (after.beta === release.version) return;
    if (after.beta && semver.valid(after.beta) && semver.gt(after.beta, release.version)) {
      throw new Error('beta changed to a newer version during promotion');
    }
    if (attempt + 1 < attempts) delay();
  }
  throw new Error('beta promotion read-back mismatch');
}

if (process.argv[1] && import.meta.url === pathToFileURL(process.argv[1]).href) {
  syncBeta(process.env.RELEASE_TAG, (args) => {
    const cache = args[0] === 'view' ? fs.mkdtempSync(path.join(os.tmpdir(), 'loomloom-npm-tags-')) : null;
    try {
      const result = spawnSync(npmCommand, [...npmCommandArgs, ...args,
        '--registry', 'https://registry.npmjs.org', ...(cache ? ['--cache', cache, '--prefer-online'] : [])], { encoding: 'utf8', timeout: 60000 });
      if (result.error || result.status !== 0) {
        throw new Error(`npm ${args[0]} failed: ${result.error?.message ?? result.stderr}`);
      }
      return args[0] === 'view' ? JSON.parse(result.stdout) : undefined;
    } finally {
      if (cache) fs.rmSync(cache, { recursive: true, force: true });
    }
  });
}
