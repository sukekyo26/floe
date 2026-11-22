import { describe, expect, it } from 'vitest';

import { testBoot } from '@/components/test-boot';
import { getBoot, setBoot } from '@/config/boot';
import { createKinActs } from '@/features/tasks/stores/kin-acts';
import type { TasksAnswer, TasksChange } from '@/lib/tasks';
import type { Kept } from '@/lib/storage';
import { createListQuery } from '@/stores/list-query';
import type { TaskWrites } from '@/stores/tasks';
import { createToasts } from '@/stores/toasts';
import type { TaskOf } from '@/types/task';

const STATES = [
  { name: 'todo', new: true, first: false },
  { name: 'doing', new: false, first: true },
];
const none: Kept = { read: () => null, write: () => undefined, remove: () => undefined };

function made() {
  testBoot();
  setBoot({ ...getBoot(), screen: 'tasks', states: STATES });
  const log: string[] = [];
  const query = createListQuery({
    boot: getBoot,
    search: () => '?project=a&view=doing',
    storage: none,
    replace: () => undefined,
    href: () => 'http://x/tasks',
    folders: { takeOpen: () => undefined },
  });
  const writes = {
    writeTasks: (body: TasksChange) => {
      log.push(`write ${JSON.stringify(body)}`);
      return Promise.resolve({ ok: true, status: 200 } as TasksAnswer);
    },
  } as unknown as TaskWrites;
  const toasts = createToasts();
  const acts = createKinActs({
    writes,
    toasts,
    query,
    kept: { kept: () => null, keep: (name) => log.push(`keep ${name}`) },
    boot: getBoot,
    page: { parentBack: (said, back) => log.push(`undo ${said} ${JSON.stringify(back)}`) },
  });
  return { acts, log, query, toasts };
}

describe(createKinActs, () => {
  it('writes a task made as new, with its title, in the project it is made in', async () => {
    expect.assertions(1);
    const { acts, log } = made();
    await acts.write('add', { project: 'b' }, '#9', undefined, 'Nine');
    expect(log).toStrictEqual(['write {"taskProject":"b","task":"#9","new":true,"title":"Nine"}']);
  });

  it('tells it made by a link to it, keeps its project and shows the new state, its row the one to go to', () => {
    expect.assertions(4);
    const { acts, log, query, toasts } = made();
    acts.done('add', { project: 'b' }, '#9', undefined, 1);
    expect(toasts.shown.get()[0]?.button).toStrictEqual({
      label: 'ui_task_made:#9',
      href: '/tasks?project=b&task=%239',
    });
    expect(log).toStrictEqual(['keep b']);
    expect(query.query.get()).toMatchObject({ project: 'b', view: 'todo' });
    expect(acts.back('add', { project: 'b' }, '#9')).toStrictEqual({ row: 'task:["b","#9"]' });
  });

  it('tells a child made or taken by a link to it, the children’s button the one to go to', () => {
    expect.assertions(3);
    const { acts, toasts } = made();
    acts.done('child', { project: 'b', name: '#1' }, '#9', undefined, 1);
    acts.done('child', { project: 'b', name: '#1' }, '#8', { name: '#8' } as TaskOf, 4);
    expect(toasts.shown.get().map((x) => x.button)).toStrictEqual([
      { label: 'ui_task_child_made:#9', href: '/tasks?project=b&task=%239' },
      { label: 'ui_task_child_took:#8', href: '/tasks?project=b&task=%238' },
    ]);
    expect(acts.back('child', { project: 'b' }, '#9')).toStrictEqual({ part: 'kids' });
    expect(acts.back('parent', { project: 'b' }, '#9')).toStrictEqual({ part: 'up' });
  });

  it('tells a parent put by a link; changed, with the way back of the rev answered while the one before makes a family', () => {
    expect.assertions(2);
    const { acts, log, toasts } = made();
    acts.done('parent', { project: 'b', name: '#1' }, '#9', undefined, 2);
    const up = { name: '#5' } as TaskOf;
    acts.done('parent', { project: 'b', name: '#1', parent: '#5', parentTask: up }, '#9', undefined, 3);
    acts.done('parent', { project: 'b', name: '#1', parent: 'hand' }, '#9', undefined, 4);
    expect(toasts.shown.get().map((x) => [x.text, x.button])).toStrictEqual([
      ['', { label: 'ui_task_parent_put:#9', href: '/tasks?project=b&task=%239' }],
      ['ui_task_parent_moved:hand', null],
    ]);
    expect(log).toStrictEqual([
      'undo ui_task_parent_moved:#5 {"project":"b","name":"#1","parent":"#5","rev":3,"up":true}',
    ]);
  });
});
