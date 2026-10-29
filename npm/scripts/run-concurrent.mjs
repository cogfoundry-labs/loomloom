// Wait for every started upload before reporting failure; never start more after one fails.
export async function runConcurrent(items, limit, operation) {
  let next = 0;
  let failure;
  const workers = Array.from({ length: Math.min(limit, items.length) }, async () => {
    while (!failure && next < items.length) {
      const item = items[next++];
      try {
        await operation(item);
      } catch (error) {
        failure ??= error;
      }
    }
  });
  await Promise.all(workers);
  if (failure) throw failure;
}
