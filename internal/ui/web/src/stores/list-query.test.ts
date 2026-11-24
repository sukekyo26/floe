import { describe, expect, it, vi } from 'vitest';

import type { Boot } from '@/config/boot';
import type { Kept } from '@/lib/storage';
import { createListQuery } from '@/stores/list-query';

const STATES = [
  { name: 'todo', new: true, first: false },
  { name: 'doing', new: false, first: true },
];

function memory(): Kept & { held: Map<string, string>; used: () => number } {
  const held = new Map<string, string>();
  let used = 0;
  return {
    held,
    used: () => used,
    read: (key) => {
      used++;
      return held.get(key) ?? null;
    },
    write: (key, value) => {
      used++;
      held.set(key, value);
    },
    remove: (key) => {
      used++;
      held.delete(key);
    },
  };
}

function made(screen: Boot['screen'], search: string) {
  const storage = memory();
  const urls: string[] = [];
  const query = createListQuery({
    boot: () => ({ screen, states: STATES }) as unknown as Boot,
    search: () => search,
    storage,
    replace: (url) => urls.push(url),
    href: () => `http://x/${screen === 'tasks' ? 'tasks' : ''}${search}`,
    folders: { takeOpen: () => undefined },
  });
  return { query, storage, urls };
}

describe(createListQuery, () => {
  it("reads the views of the list's screen, and of the task screens their states, leaving their picks out", () => {
    expect.assertions(2);
    expect(made('list', '').query.readState('?view=fav&kind=folder&q=a&open=x')).toStrictEqual({
      view: 'fav',
      kind: 'folder',
      template: '',
      project: '',
      tag: '',
      q: 'a',
      open: ['x'],
    });
    expect(made('tasks', '').query.readState('?view=fav&kind=folder&q=a&project=p')).toMatchObject({
      view: 'doing',
      kind: '',
      q: '',
      project: 'p',
    });
  });

  it('writes the URL with its defaults left out and keeps it for the tab under pages.list or pages.tasks', () => {
    expect.assertions(3);
    const list = made('list', '?view=new&q=+a+&open=x');
    list.query.sync();
    expect(list.urls).toStrictEqual(['http://x/?view=new&q=a']);
    expect(list.storage.held.get('pages.list')).toBe('?view=new&q=a');
    const tasks = made('tasks', '?view=doing&project=p');
    tasks.query.sync();
    expect(tasks.storage.held.get('pages.tasks')).toBe('?project=p');
  });

  it('keeps the task gone back from under pages.tasks.from once, and reads no storage as it is made', () => {
    expect.assertions(3);
    const { query, storage } = made('tasks', '');
    expect(storage.used()).toBe(0);
    query.keepFrom('["p","x"]');
    expect(query.takeFrom()).toBe('["p","x"]');
    expect(query.takeFrom()).toBeNull();
  });

  it('lets the seen pages of the view "new" go with a new view', () => {
    expect.assertions(2);
    const { query } = made('list', '?view=new');
    query.see(['a']);
    query.filter({ kind: 'page' });
    expect([...query.query.get().seen]).toStrictEqual(['a']);
    query.filter({ view: 'new' });
    expect(query.query.get().seen.size).toBe(0);
  });
});

describe('see', () => {
  it('keeps every page given as seen in one write, and writes nothing of those seen already', () => {
    expect.assertions(3);
    const { query } = made('list', '?view=new');
    const told = vi.fn<() => void>();
    query.query.subscribe(told);
    query.see(['a', 'b']);
    expect([...query.query.get().seen]).toStrictEqual(['a', 'b']);
    query.see(['b', 'a']);
    expect(told).toHaveBeenCalledTimes(1);
    query.see(['c', 'a']);
    expect([...query.query.get().seen]).toStrictEqual(['a', 'b', 'c']);
  });
});
