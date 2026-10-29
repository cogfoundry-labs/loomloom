import assert from 'node:assert/strict';
import test from 'node:test';
import { syncBeta } from '../scripts/sync-beta.mjs';

test('stable promotion updates absent or older beta and verifies read-back', () => {
  for (const beta of [undefined, '0.7.0-beta.10', '0.6.2-beta.3']) {
    const calls = [];
    syncBeta('v0.7.0', (args) => {
      calls.push(args);
      return { latest: '0.7.0', beta: calls.length === 3 ? '0.7.0' : beta };
    });
    assert.equal(calls.length, 3);
    assert.deepEqual(calls[1], ['dist-tag', 'add', '@cogfoundry/loomloom@0.7.0', 'beta']);
  }
});
test('promotion preserves equal or newer beta and skips prereleases', () => {
  for (const beta of ['0.7.0', '0.8.0-beta.1']) {
    let calls = 0;
    syncBeta('v0.7.0', () => { calls++; return { latest: '0.7.0', beta }; });
    assert.equal(calls, 1);
  }
  syncBeta('v0.7.0-beta.1', () => assert.fail('unexpected registry call'));
});
test('promotion fails on wrong latest, invalid beta, failed writes or bad read-back', () => {
  assert.throws(() => syncBeta('v0.7.0', () => ({ latest: '0.6.0' })));
  assert.throws(() => syncBeta('v0.7.0', () => ({ latest: '0.7.0', beta: 'invalid' })));
  assert.throws(() => syncBeta('v0.7.0', (args) => {
    if (args[0] === 'dist-tag') throw new Error('permission denied');
    return { latest: '0.7.0' };
  }), /permission denied/);
  assert.throws(() => syncBeta('v0.7.0', () => ({ latest: '0.7.0', beta: '0.6.0' }), { attempts: 3, delay: () => {} }), /read-back/);
});

test('stale beta read-back is retried without repeating the write', () => {
  let reads = 0;
  let writes = 0;
  let delays = 0;
  syncBeta('v0.7.0', (args) => {
    if (args[0] === 'dist-tag') { writes++; return; }
    reads++;
    return { latest: '0.7.0', beta: reads >= 4 ? '0.7.0' : '0.7.0-beta.2' };
  }, { attempts: 5, delay: () => { delays++; } });
  assert.equal(writes, 1);
  assert.equal(reads, 4);
  assert.equal(delays, 2);
});

test('promotion stops if a newer beta appears during read-back', () => {
  let reads = 0;
  assert.throws(() => syncBeta('v0.7.0', (args) => {
    if (args[0] === 'dist-tag') return;
    reads++;
    return { latest: '0.7.0', beta: reads === 1 ? '0.7.0-beta.2' : '0.8.0-beta.1' };
  }, { delay: () => {} }), /newer version/);
});
