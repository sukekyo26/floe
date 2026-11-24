import { describe, expect, it } from 'vitest';

import { seenAgain, type SeenKey } from '@/features/tasks/utils/seen-task';

const opened = {};
const key = (change: Partial<SeenKey> = {}): SeenKey => ({
  open: opened,
  project: 'p',
  name: 'a',
  renews: 0,
  ...change,
});

describe(seenAgain, () => {
  it('finds the task again as the modal opens, the project or name changes or a write is refused, and not on a read', () => {
    expect.assertions(2);
    expect(seenAgain(null, key())).toBe(true);
    const changes: Partial<SeenKey>[] = [
      // A read of the list keeps the key as it was (ui_task_children_test.go:594, TestUITaskRev:166)
      {},
      // Opened again over the same page: another open
      { open: {} },
      { project: 'q' },
      { name: 'b' },
      { renews: 1 },
      { name: ' a ' },
    ];
    expect(changes.map((c) => seenAgain(key(), key(c)))).toStrictEqual([false, true, true, true, true, false]);
  });
});
