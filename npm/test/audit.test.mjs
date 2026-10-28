import assert from 'node:assert/strict';
import test from 'node:test';
import { findBlockingAdvisories } from '../scripts/audit.mjs';

const accepted = [{ id: 'GHSA-aaaa', node: 'node_modules/npm/node_modules/undici', expires: '2026-11-27' }];
const advisory = (id, severity) => ({ url: `https://github.com/advisories/${id}`, severity, title: id });
const report = (via, nodes = ['node_modules/npm/node_modules/undici']) => ({
  vulnerabilities: { undici: { via, nodes } },
});

test('an accepted advisory at its path passes until it expires', () => {
  const current = report([advisory('GHSA-aaaa', 'high'), advisory('GHSA-mod', 'moderate')]);
  assert.deepEqual(findBlockingAdvisories(current, '2026-11-27', accepted), []);
  assert.equal(findBlockingAdvisories(current, '2026-11-28', accepted).length, 1);
});

test('unaccepted, relocated or critical advisories block', () => {
  assert.equal(findBlockingAdvisories(report([advisory('GHSA-bbbb', 'high')]), '2026-10-28', accepted).length, 1);
  assert.equal(findBlockingAdvisories(report([advisory('GHSA-aaaa', 'critical')], ['node_modules/undici']),
    '2026-10-28', accepted).length, 1);
});

test('transitive entries are reported through the package that carries the advisory', () => {
  const transitive = { vulnerabilities: { npm: { via: ['undici'], nodes: ['node_modules/npm'] } } };
  assert.deepEqual(findBlockingAdvisories(transitive, '2026-10-28', accepted), []);
});
