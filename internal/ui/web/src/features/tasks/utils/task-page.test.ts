import { describe, expect, it } from 'vitest';

import { familyOf, familySaid, pageTask, taskGroups, taskListHref } from '@/features/tasks/utils/task-page';
import type { Page } from '@/types/page';
import type { TaskOf } from '@/types/task';

const page = (slug: string, over: Partial<Page> = {}): Page => ({
  slug,
  description: '',
  createdAt: '',
  updatedAt: '',
  taskProject: 'p',
  project: 'p',
  ...over,
});
const task = (name: string, over: Partial<TaskOf> = {}): TaskOf => ({
  project: 'p',
  name,
  state: 'doing',
  at: '2026-01-01T00:00:00Z',
  rev: 1,
  pages: [],
  archived: [],
  children: [],
  ...over,
});
const STATES = ['doing', 'todo', 'done', 'archived'];

describe(pageTask, () => {
  const last = task('#1', { rev: 3, pages: [page('old')] });

  it('draws the task read, and says one read and not there missing', () => {
    expect.assertions(2);
    const found = task('#1', { pages: [page('a')] });
    expect(pageTask({ found, rows: undefined, failed: false, last })).toStrictEqual({
      task: found,
      none: false,
      missing: false,
    });
    expect(pageTask({ found: undefined, rows: undefined, failed: false, last })).toStrictEqual({
      task: null,
      none: false,
      missing: true,
    });
  });

  it('draws the task as last read while unreadable, with its pages now and what the screen wrote since', () => {
    expect.assertions(3);
    expect(pageTask({ found: undefined, rows: undefined, failed: true, last })).toStrictEqual({
      task: { ...last, pages: [], archived: [] },
      none: true,
      missing: false,
    });
    // A write while tasks.json is unreadable: its answer's row is the store's, and the next is of its rev
    const now = pageTask({
      found: task('#1', { state: 'done', rev: 4, pages: [page('a')] }),
      rows: { '#1': { state: 'done', at: '', rev: 4 } },
      failed: true,
      last,
    });
    expect([now.task?.state, now.task?.rev, now.task?.pages.map((a) => a.slug)]).toStrictEqual(['done', 4, ['a']]);
    // Never read: nothing drawn, nor said missing
    expect(pageTask({ found: undefined, rows: undefined, failed: true, last: null })).toStrictEqual({
      task: null,
      none: false,
      missing: false,
    });
  });
});

describe('pageTask, a parent and a child taken off while unreadable', () => {
  it('keeps the parent as last read with nothing written since', () => {
    expect.assertions(1);
    const up = task('#9');
    const last = task('#1', { parent: '#9', parentTask: up });
    expect(pageTask({ found: undefined, rows: undefined, failed: true, last }).task?.parentTask).toBe(up);
  });

  it('draws no parent once its own row as answered names none, and no child whose row names another', () => {
    expect.assertions(1);
    const up = task('#9');
    const last = task('#1', { parent: '#9', parentTask: up, rev: 3, children: [task('#2'), task('#3')] });
    // The answer leaves out a parent taken off (omitempty)
    const now = pageTask({
      found: undefined,
      rows: { '#1': { state: 'doing', at: '', rev: 4 }, '#2': { state: 'doing', at: '', rev: 2 } },
      failed: true,
      last,
    }).task;
    expect([now?.parent, now?.parentTask, now?.rev, now?.children.map((x) => x.name)]).toStrictEqual([
      undefined,
      undefined,
      4,
      ['#3'],
    ]);
  });
});

describe(taskGroups, () => {
  it('puts no category first, with what is on no category of the task, then each category in order, empty too, archived last', () => {
    expect.assertions(1);
    const on = (slug: string, category: string, createdAt: string, archived = false) =>
      page(slug, { createdAt, archived, task: { name: '#1', category } });
    const groups = taskGroups(
      task('#1', {
        categories: ['b', 'a', 'empty'],
        pages: [on('a2', 'a', '2'), on('a1', 'a', '1'), on('x', 'gone', '1'), on('b1', 'b', '1'), on('n', '', '3')],
        archived: [on('a0', 'a', '0', true)],
      }),
    );
    expect(groups.map((g) => ({ ...g, entries: g.entries.map((a) => a.slug) }))).toStrictEqual([
      { category: '', entries: ['x', 'n'] },
      { category: 'b', entries: ['b1'] },
      { category: 'a', entries: ['a1', 'a2', 'a0'] },
      { category: 'empty', entries: [] },
    ]);
  });

  it('heads no category only where it has pages, a category with none too', () => {
    expect.assertions(2);
    expect(taskGroups(task('#1', { pages: [], archived: [] }))).toStrictEqual([]);
    expect(taskGroups(task('#1', { categories: ['plan'], pages: [], archived: [] }))).toStrictEqual([
      { category: 'plan', entries: [] },
    ]);
  });
});

describe(familyOf, () => {
  it('offers the parent while there are no children, a child while there is no parent, children by state then newest', () => {
    expect.assertions(3);
    const kids = [
      task('#3', { state: 'todo', at: '2026-01-03T00:00:00Z' }),
      task('#2', { state: 'doing', at: '2026-01-01T00:00:00Z' }),
      task('#4', { state: 'doing', at: '2026-01-02T00:00:00Z' }),
    ];
    const up = familyOf(task('#1', { children: kids }), STATES, 'en');
    expect([up.ups, up.adds, up.kids.map((x) => x.name), up.shown]).toStrictEqual([
      false,
      true,
      ['#4', '#2', '#3'],
      true,
    ]);
    const child = familyOf(task('#5', { parent: '#1', parentTask: task('#1') }), STATES, 'en');
    expect([child.ups, child.adds, child.kidsShown, child.up?.name]).toStrictEqual([true, false, false, '#1']);
    // A parent written by hand that makes no family is changed, not added to
    expect(familyOf(task('#6', { parent: 'nobody' }), STATES, 'en').adds).toBe(false);
  });
});

describe(familySaid, () => {
  it('changes with anything the card says, and stays with nothing changed', () => {
    expect.assertions(1);
    const kid = task('#2', { parent: '#1', title: 'two' });
    const of = (x: TaskOf) => familySaid(x, familyOf(x, STATES, 'en'));
    const base = task('#1', { children: [kid] });
    const said = of(base);
    expect([
      of(task('#1', { children: [task('#2', { parent: '#1', title: 'two' })] })) === said,
      of({ ...base, rev: 2 }) === said,
      of({ ...base, name: '#9' }) === said,
      of(task('#1', { children: [{ ...kid, title: 'renamed' }] })) === said,
      of(task('#1', { children: [{ ...kid, state: 'done' }] })) === said,
      of(task('#1', { children: [{ ...kid, rev: 5 }] })) === said,
      of(task('#1', { children: [{ ...kid, pages: [page('u', { news: 'version' })] }] })) === said,
      of(task('#7', { parent: '#1', parentTask: task('#1', { title: 'up' }) })) ===
        of(task('#7', { parent: '#1', parentTask: task('#1', { title: 'up, renamed' }) })),
    ]).toStrictEqual([true, false, false, false, false, false, false, false]);
  });
});

describe(taskListHref, () => {
  it('names the view of the task unless it is the first, and the project alone with no task read', () => {
    expect.assertions(1);
    expect([
      taskListHref('p', 'doing', 'doing'),
      taskListHref('p q', 'done', 'doing'),
      taskListHref('p', undefined, 'doing'),
    ]).toStrictEqual(['/tasks?project=p', '/tasks?project=p+q&view=done', '/tasks?project=p']);
  });
});
