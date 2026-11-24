import { describe, expect, it } from 'vitest';

import { testBoot } from '@/components/test-boot';
import { movesFrom, setBody, setSaid } from '@/features/tasks/utils/task-set';
import type { TaskOf } from '@/types/task';

const task: TaskOf = {
  rev: 4,
  state: 'doing',
  at: '',
  project: 'p',
  name: 'S1',
  pages: [],
  archived: [],
  children: [],
};

describe(setBody, () => {
  it('sends a task there by the rev shown, and a new one as new with its title', () => {
    expect.assertions(2);
    expect(setBody(task, 'x')).toStrictEqual({ new: false, rev: 4 });
    expect(setBody(undefined, ' A title ')).toStrictEqual({ new: true, title: 'A title' });
  });
});

describe(setSaid, () => {
  it('says saved where the page stays on the task it opened on, else links to the task with where it came from', () => {
    expect.assertions(3);
    testBoot();
    const page = { taskProject: 'p' };
    expect(setSaid(page, 'S1 ', 'S1', { pages: [] }, 'doing')).toStrictEqual({ text: 'ui_saved:undefined' });
    expect(setSaid(page, 'S2', 'S1', { pages: [] }, 'doing')).toStrictEqual({
      link: 'ui_task_put:S2',
      href: '/tasks?project=p&task=S2',
    });
    // Moved off a task the write found it on, though it opened on the same name
    const off = { slug: 'a', description: '', createdAt: '', updatedAt: '', taskProject: 'p', offTask: 'S9' };
    expect(setSaid(page, 'S1', 'S1', { pages: [off] }, 'doing')).toStrictEqual({
      link: 'ui_task_put_moved:S9',
      href: '/tasks?project=p&task=S1',
    });
  });
});

describe(movesFrom, () => {
  it('is the task the page would leave, none when on none or on the one named', () => {
    expect.assertions(3);
    expect(movesFrom({}, 'S1')).toBe('');
    expect(movesFrom({ task: { name: 'S1' } }, ' S1')).toBe('');
    expect(movesFrom({ task: { name: 'S2' } }, 'S1')).toBe('S2');
  });
});
