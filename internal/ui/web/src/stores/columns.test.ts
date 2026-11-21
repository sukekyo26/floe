import { describe, expect, it } from 'vitest';

import type { Boot } from '@/config/boot';
import type { Kept } from '@/lib/storage';
import { createColumns } from '@/stores/columns';
import { createOpenPage } from '@/stores/open-page';

const memory = (start: Record<string, string>): Kept & { held: Map<string, string> } => {
  const held = new Map(Object.entries(start));
  return {
    held,
    read: (key) => held.get(key) ?? null,
    write: (key, value) => held.set(key, value),
    remove: (key) => held.delete(key),
  };
};

function made(screen: Boot['screen'], search: string, kept: string) {
  const boot = () => ({ screen }) as unknown as Boot;
  const openPage = createOpenPage({ boot, search: () => search });
  const storage = memory({ 'list.columns': kept });
  return { columns: createColumns({ boot, search: () => search, storage, openPage }), storage, openPage };
}

describe(createColumns, () => {
  it('reads list.columns, keeping the defaults for what it does not hold, and writes it back as it is', () => {
    expect.assertions(2);
    const { columns, storage } = made('list', '', '{"shelf":14,"list":"wide","open":false}');
    expect(columns.columns.get()).toStrictEqual({ shelf: 14, list: null, open: false, listOpen: true });
    columns.set({ listOpen: false });
    columns.keep();
    expect(storage.held.get('list.columns')).toBe('{"shelf":14,"list":null,"open":false,"listOpen":false}');
  });

  it('shows the list on the task list, and on a task page until a page is picked', () => {
    expect.assertions(3);
    expect(made('tasks', '', '{"listOpen":false}').columns.listOpen()).toBe(true);
    const page = made('tasks', '?task=x', '{"listOpen":false}');
    expect(page.columns.listOpen()).toBe(true);
    page.openPage.picked.set({ slug: 'a', file: '', v: 0 });
    expect(page.columns.listOpen()).toBe(false);
  });
});
