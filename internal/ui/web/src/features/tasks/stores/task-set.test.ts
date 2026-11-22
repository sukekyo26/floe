import { describe, expect, it } from 'vitest';

import { createTaskSetActs } from '@/features/tasks/stores/task-set';
import type { Page } from '@/types/page';

const page: Page = { slug: 'a', description: '', createdAt: '', updatedAt: '', taskProject: 'p', task: { name: 'S1' } };
const prepare = () => ({ done: () => undefined, away: () => undefined });

describe(createTaskSetActs, () => {
  it('tells a way off that could not be sent as a reason, for the modal to say, and passes the rest on', async () => {
    expect.assertions(2);
    const down = createTaskSetActs({
      actions: {
        takeOff: () => Promise.reject(new Error('Failed to fetch')),
      },
    });
    await expect(down.takeOff(page, prepare)).resolves.toStrictEqual({ reason: 'Failed to fetch' });
    const up = createTaskSetActs({ actions: { takeOff: () => Promise.resolve({ reason: '', toast: 7 }) } });
    await expect(up.takeOff(page, prepare)).resolves.toStrictEqual({ reason: '', toast: 7 });
  });
});
