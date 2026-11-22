import { describe, expect, it } from 'vitest';

import {
  listState,
  openingProject,
  openProject,
  projectPress,
  projectChips,
  shownTasks,
  stateCounts,
  taskProjects,
  urlEnd,
  type Opening,
} from '@/features/tasks/utils/task-list';
import type { Page } from '@/types/page';
import type { TaskOf } from '@/types/task';

const task = (project: string, name: string, state: string, at: string): TaskOf => ({
  project,
  name,
  state,
  at,
  rev: 1,
  pages: [],
  archived: [],
  children: [],
});
const page = (slug: string, taskProject: string, over: Partial<Page> = {}): Page => ({
  slug,
  description: '',
  createdAt: '',
  updatedAt: '',
  project: taskProject,
  taskProject,
  ...over,
});

const TASKS = [
  task('a', '#1', 'doing', '2026-11-01T00:00:00Z'),
  task('b', '#2', 'doing', '2026-11-03T00:00:00Z'),
  task('b', '#3', 'todo', '2026-11-02T00:00:00Z'),
];
const PIN = page('p', 'c', { task: { pin: true } });
const opening = (over: Partial<Opening> = {}): Opening => ({
  tasks: TASKS,
  entries: [PIN],
  kept: null,
  failed: false,
  first: 'doing',
  lang: 'en',
  ...over,
});

describe(openingProject, () => {
  it('opens with the project kept while there is one, else where a task in progress was written last', () => {
    expect.assertions(3);
    expect(openingProject(opening({ kept: 'c' }))).toBe('c');
    expect(openingProject(opening({ kept: 'gone' }))).toBe('b');
    expect(openingProject(opening({ tasks: [...TASKS, task('a', '#4', 'doing', '2026-11-04T00:00:00Z')] }))).toBe('a');
  });

  it('opens with the first project of the shelf, then none, with no task in progress', () => {
    expect.assertions(2);
    expect(openingProject(opening({ tasks: [] }))).toBe('c');
    expect(openingProject(opening({ tasks: [], entries: [] }))).toBe('');
  });

  it('opens unreadable with what was kept alone, which may be gone', () => {
    expect.assertions(2);
    expect(openingProject(opening({ failed: true, kept: 'gone' }))).toBe('gone');
    expect(openingProject(opening({ failed: true }))).toBe('');
  });

  it('shows the project asked for, there or not', () => {
    expect.assertions(1);
    expect(openProject('gone', opening({ kept: 'c' }))).toBe('gone');
  });
});

describe(taskProjects, () => {
  it('has the tasks’ projects and the pinned pages’, not those archived', () => {
    expect.assertions(1);
    const archived = page('q', 'd', { task: { pin: true }, archived: true });
    expect([...taskProjects(TASKS, [PIN, archived])].sort()).toStrictEqual(['a', 'b', 'c']);
  });
});

describe(shownTasks, () => {
  it('shows the project’s tasks of the view, latest first, then by name', () => {
    expect.assertions(2);
    const same = task('b', '#0', 'doing', '2026-11-03T00:00:00Z');
    expect(shownTasks([...TASKS, same], 'b', 'doing', 'en').map((x) => x.name)).toStrictEqual(['#0', '#2']);
    expect(
      stateCounts(TASKS, 'b', [
        { name: 'doing', new: false, first: true },
        { name: 'todo', new: true, first: false },
      ]),
    ).toStrictEqual({ doing: 1, todo: 1 });
  });
});

describe(projectChips, () => {
  it('has every project there, the one shown though gone after them, and no project last, with the view’s tasks', () => {
    expect.assertions(2);
    // A page of no project: its project unwritten, its task's project '/'
    const none = page('n', '/', { project: '', task: { pin: true } });
    expect(projectChips(TASKS, [PIN, none], 'gone', 'doing')).toStrictEqual([
      { name: 'c', n: 0 },
      { name: 'a', n: 1 },
      { name: 'b', n: 1 },
      { name: 'gone', n: 0 },
      { name: '/', n: 0 },
    ]);
    expect(projectChips(TASKS, [], '/', 'todo').map((x) => x.name)).toStrictEqual(['a', 'b', '/']);
  });
});

describe(listState, () => {
  it('says nothing at all, none of the view, a project gone, and hides the way to add one but where it adds', () => {
    expect.assertions(4);
    expect(listState([], [], '', 'doing', false, 0)).toStrictEqual({
      nothing: true,
      none: false,
      gone: true,
      addHidden: true,
    });
    expect(listState(TASKS, [], 'a', 'todo', false, 0)).toStrictEqual({
      nothing: false,
      none: true,
      gone: false,
      addHidden: false,
    });
    expect(listState(TASKS, [], 'gone', 'todo', false, 0)).toMatchObject({ none: true, gone: true, addHidden: true });
    expect(listState([], [PIN], 'c', 'archived', true, 0)).toStrictEqual({
      nothing: false,
      none: false,
      gone: false,
      addHidden: true,
    });
  });
});

describe('listState of a project of pins alone', () => {
  it('has something to list, read: no words of nothing anywhere, and the way to add', () => {
    expect.assertions(1);
    expect(listState([], [PIN], 'c', 'doing', false, 0)).toStrictEqual({
      nothing: false,
      none: true,
      gone: false,
      addHidden: false,
    });
  });
});

describe(projectPress, () => {
  it('keeps the project pressed once read, not while unreadable, and shows it unless shown', () => {
    expect.assertions(3);
    expect(projectPress('b', 'a', false)).toStrictEqual({ keep: true, show: true });
    expect(projectPress('b', 'a', true)).toStrictEqual({ keep: false, show: true });
    expect(projectPress('a', 'a', false)).toStrictEqual({ keep: true, show: false });
  });
});

describe(urlEnd, () => {
  it('tells a URL by its last two parts of path, or its host, or itself', () => {
    expect.assertions(1);
    expect(['https://x.org/a/b/c', 'https://x.org/', 'not a url'].map(urlEnd)).toStrictEqual([
      'b/c',
      'x.org',
      'not a url',
    ]);
  });
});
