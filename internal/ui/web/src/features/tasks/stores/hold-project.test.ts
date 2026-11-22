import { describe, expect, it } from 'vitest';

import type { Boot } from '@/config/boot';
import { holdProject } from '@/features/tasks/stores/hold-project';
import type { ProjectKept } from '@/features/tasks/stores/project-kept';
import type { Kept } from '@/lib/storage';
import { createStore } from '@/lib/store';
import { createListQuery } from '@/stores/list-query';
import type { Pages, PagesState } from '@/stores/pages';
import type { Tasks } from '@/types/task';

const STATES = [
  { name: 'todo', new: true, first: false },
  { name: 'doing', new: false, first: true },
];
const boot = () => ({ screen: 'tasks', lang: 'en', states: STATES }) as unknown as Boot;
const none: Kept = { read: () => null, write: () => undefined, remove: () => undefined };

function made(search: string, kept: string | null = null) {
  const urls: string[] = [];
  const query = createListQuery({
    boot,
    search: () => search,
    storage: none,
    replace: (url) => urls.push(url),
    href: () => `http://x/tasks${search}`,
    folders: { takeOpen: () => undefined },
  });
  const state = createStore<PagesState>({
    read: false,
    entries: [],
    tasks: { error: '', tasks: {}, raw: '' },
    readWhy: '',
    drawWhy: '',
  });
  const keeps: string[] = [];
  const projectKept: ProjectKept = { kept: () => kept, keep: (name) => keeps.push(name) };
  const read = (tasks: Tasks, error = '') => {
    state.set({ ...state.get(), read: true, tasks: { error, tasks, raw: JSON.stringify(tasks) + error } });
  };
  const stop = holdProject({ query, pages: { state } as unknown as Pages, kept: projectKept, boot });
  return { query, urls, keeps, read, stop };
}

const row = (state: string, at: string) => ({ state, at });

describe(holdProject, () => {
  it('writes nothing before the tasks are read, nor while they cannot be', () => {
    expect.assertions(2);
    const { urls, keeps, read } = made('', 'p');
    read({}, 'broken');
    expect(urls).toStrictEqual([]);
    expect(keeps).toStrictEqual([]);
  });

  it('puts in the project it opens with, once, keeps it once, and writes the URL each time', () => {
    expect.assertions(3);
    const { query, urls, keeps, read } = made('');
    read({ a: { '#1': row('doing', '2026-11-01T00:00:00Z') } });
    // A task come into progress later elsewhere leaves the list where it is
    read({ a: { '#1': row('doing', '2026-11-01T00:00:00Z') }, b: { '#2': row('doing', '2026-11-05T00:00:00Z') } });
    expect(query.query.get().project).toBe('a');
    expect(keeps).toStrictEqual(['a']);
    expect(urls).toStrictEqual(['http://x/tasks?project=a', 'http://x/tasks?project=a']);
  });

  it('leaves the project empty with nothing there, and puts it in once a task comes', () => {
    expect.assertions(2);
    const { query, read } = made('');
    read({});
    expect(query.query.get().project).toBe('');
    read({ b: { '#2': row('todo', '2026-11-05T00:00:00Z') } });
    expect(query.query.get().project).toBe('b');
  });

  it('keeps the project asked for in the URL and writes its URL, but never puts in another', () => {
    expect.assertions(3);
    const { query, urls, keeps, read } = made('?project=b', 'a');
    read({ a: { '#1': row('doing', '2026-11-01T00:00:00Z') }, b: { '#2': row('todo', '2026-11-05T00:00:00Z') } });
    expect(query.query.get().project).toBe('b');
    expect(keeps).toStrictEqual(['b']);
    expect(urls).toStrictEqual(['http://x/tasks?project=b']);
  });

  it('keeps no project gone, asked for in the URL', () => {
    expect.assertions(2);
    const { query, keeps, read } = made('?project=gone', 'a');
    read({ a: { '#1': row('doing', '2026-11-01T00:00:00Z') } });
    expect(query.query.get().project).toBe('gone');
    expect(keeps).toStrictEqual([]);
  });

  it('writes the URL as the view changes once read, and not while unreadable', () => {
    expect.assertions(1);
    const { query, urls, read } = made('?project=a');
    read({}, 'broken');
    query.filter({ project: 'b' });
    read({ a: { '#1': row('doing', '2026-11-01T00:00:00Z') } });
    query.filter({ view: 'todo' });
    expect(urls).toStrictEqual(['http://x/tasks?project=b', 'http://x/tasks?view=todo&project=b']);
  });
});
