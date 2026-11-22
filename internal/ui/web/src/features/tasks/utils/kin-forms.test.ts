import { describe, expect, it } from 'vitest';

import { testBoot } from '@/components/test-boot';
import { askedProjects, KIN_FORMS } from '@/features/tasks/utils/kin-forms';
import type { Page } from '@/types/page';
import type { TaskOf } from '@/types/task';

const there: TaskOf = {
  project: 'p',
  name: '#1',
  title: 'One',
  state: 'doing',
  at: '',
  rev: 3,
  pages: [],
  archived: [],
  children: [],
};
const page = (taskProject: string, project: string): Page => ({
  slug: taskProject + project,
  description: '',
  createdAt: '',
  updatedAt: '',
  taskProject,
  project,
});

describe('the form add', () => {
  it('opens a task there with nothing written, and says so, its title not asked', () => {
    expect.assertions(2);
    testBoot();
    expect(KIN_FORMS.add.there({ project: 'p' }, '#1', there)).toStrictEqual({
      button: 'ui_task_open_button',
      act: 'open',
      note: 'ui_task_there:#1 One',
    });
    expect(KIN_FORMS.add.there({ project: 'p' }, '#2', undefined)).toBeNull();
  });

  it('writes any other as a new task of the project, and offers no task', () => {
    expect.assertions(2);
    expect(KIN_FORMS.add.body({ project: 'p' }, '#2', undefined, 'Two')).toStrictEqual({
      taskProject: 'p',
      task: '#2',
      new: true,
      title: 'Two',
    });
    expect(KIN_FORMS.add.offered({ project: 'p' }, there)).toBe(false);
  });
});

describe(askedProjects, () => {
  it('asks of no project, then the pages’ projects by their colours, the list’s chosen if a page has it', () => {
    expect.assertions(2);
    const entries = [page('b', 'b'), page('a', 'a'), page('a', 'a'), page('/', '')];
    expect(askedProjects(entries, 'b')).toStrictEqual({ names: ['/', 'a', 'b'], chosen: 'b' });
    expect(askedProjects(entries, 'gone').chosen).toBe('/');
  });
});

describe('the forms of a task’s family', () => {
  const page = { ...there, name: '#1', title: 'One' };
  const own: TaskOf = { ...there, name: '#2', title: 'Two', parent: '#1', parentTask: page };
  const elsewhere: TaskOf = { ...there, name: '#3', parent: '#9', parentTask: { ...there, name: '#9' } };
  const parentOf: TaskOf = { ...there, name: '#4', children: [own] };

  it('child: offers what may be a child, takes an own child as no change, says a move', () => {
    expect.assertions(4);
    testBoot();
    expect([own, elsewhere, parentOf, page].map((x) => KIN_FORMS.child.offered(page, x))).toStrictEqual([
      false,
      true,
      false,
      false,
    ]);
    expect(KIN_FORMS.child.there(page, '#2', own)).toStrictEqual({
      button: 'ui_child_take_button',
      act: 'none',
      note: '',
    });
    expect(KIN_FORMS.child.there(page, '#3', elsewhere)?.note).toBe('ui_task_child_move:#3 One');
    // One that may not be a child is not said: pressed, the store refuses it
    expect(KIN_FORMS.child.there(page, '#4', parentOf)?.note).toBe('');
  });

  it('child: writes the child’s row, of its rev, or makes it with its title', () => {
    expect.assertions(2);
    expect(KIN_FORMS.child.body(page, '#3', elsewhere, 'x')).toStrictEqual({
      taskProject: 'p',
      task: '#3',
      new: false,
      rev: 3,
      parent: '#1',
      parentNew: false,
    });
    expect(KIN_FORMS.child.body(page, '#5', undefined, 'Five')).toStrictEqual({
      taskProject: 'p',
      task: '#5',
      new: true,
      title: 'Five',
      parent: '#1',
      parentNew: false,
    });
  });

  it('parent: the one written named again is no change, another is said as a move, any other made with it', () => {
    expect.assertions(4);
    testBoot();
    const child = { ...own, rev: 7 };
    expect(KIN_FORMS.parent.there(child, '#1', page)).toStrictEqual({
      button: 'ui_up_take_button',
      act: 'none',
      note: '',
    });
    expect(KIN_FORMS.parent.there(child, '#4', { ...parentOf, children: [] })?.note).toBe('ui_task_parent_move:#4 One');
    expect(KIN_FORMS.parent.body(child, '#4', parentOf, '')).toStrictEqual({
      taskProject: 'p',
      task: '#2',
      new: false,
      rev: 7,
      parent: '#4',
      parentNew: false,
    });
    expect(KIN_FORMS.parent.body(child, '#8', undefined, 'Eight')).toMatchObject({
      parent: '#8',
      parentNew: true,
      parentTitle: 'Eight',
    });
  });
});
