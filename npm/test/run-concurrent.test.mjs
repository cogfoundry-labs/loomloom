import assert from 'node:assert/strict';
import test from 'node:test';
import { runConcurrent } from '../scripts/run-concurrent.mjs';

test('all independent uploads finish within the concurrency limit', async () => {
  let active = 0;
  let maximum = 0;
  const completed = [];
  await runConcurrent([0, 1, 2, 3, 4, 5], 3, async (item) => {
    maximum = Math.max(maximum, ++active);
    await new Promise((resolve) => setTimeout(resolve, 5));
    completed.push(item);
    active--;
  });
  assert.equal(maximum, 3);
  assert.deepEqual(completed.sort(), [0, 1, 2, 3, 4, 5]);
});

test('uploads run with a bounded limit and wait for all started work after a failure', async () => {
  let active = 0;
  let maximum = 0;
  const started = [];
  await assert.rejects(runConcurrent([0, 1, 2, 3], 2, async (item) => {
    started.push(item);
    active++;
    maximum = Math.max(maximum, active);
    await new Promise((resolve) => setTimeout(resolve, item === 0 ? 5 : 20));
    active--;
    if (item === 0) throw new Error('upload failed');
  }), /upload failed/);
  assert.equal(maximum, 2);
  assert.equal(active, 0);
  assert.deepEqual(started, [0, 1]);
});
