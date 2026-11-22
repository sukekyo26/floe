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
