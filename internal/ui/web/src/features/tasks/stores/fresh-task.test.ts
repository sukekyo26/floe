import { describe, expect, it } from 'vitest';

import type { Boot } from '@/config/boot';
import { createFreshTask } from '@/features/tasks/stores/fresh-task';
import type { Kept } from '@/lib/storage';
import { createListQuery } from '@/stores/list-query';

const STATES = [
  { name: 'todo', new: true, first: false },
  { name: 'doing', new: false, first: true },
];
const none: Kept = { read: () => null, write: () => undefined, remove: () => undefined };

function made() {
  const query = createListQuery({
    boot: () => ({ screen: 'tasks', states: STATES }) as unknown as Boot,
    search: () => '?project=a',
    storage: none,
    replace: () => undefined,
    href: () => 'http://x/tasks',
    folders: { takeOpen: () => undefined },
  });
  const fresh = createFreshTask(query);
  fresh.follow();
  return { query, fresh };
}

describe(createFreshTask, () => {
  it('marks the row made where the list is as it is marked, after the made task’s view is shown', () => {
    expect.assertions(2);
    const { query, fresh } = made();
    // As done shows the new task's state, then the modal marks its row
    query.filter({ project: 'a', view: 'todo' });
    fresh.mark('task:["a","#1"]');
    expect(fresh.key.get()).toBe('task:["a","#1"]');
    query.filter({ project: 'a' });
    expect(fresh.key.get()).toBe('task:["a","#1"]');
  });

  it('lets the mark go as the list shows another view or project', () => {
    expect.assertions(2);
    const { query, fresh } = made();
    fresh.mark('k');
    query.filter({ view: 'todo' });
    expect(fresh.key.get()).toBeNull();
    fresh.mark('k');
    query.filter({ project: 'b' });
    expect(fresh.key.get()).toBeNull();
  });
});
