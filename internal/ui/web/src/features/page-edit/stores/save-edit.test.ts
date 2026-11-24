import { describe, expect, it } from 'vitest';

import { setBoot, type Boot } from '@/config/boot';
import { createSaveEdit, type EditValues } from '@/features/page-edit/stores/save-edit';
import { createStore } from '@/lib/store';
import type { OpenAt, OpenPage } from '@/stores/open-page';
import type { Order } from '@/stores/order';
import type { PageWrites } from '@/stores/page-writes';
import type { Pages } from '@/stores/pages';
import { createToasts } from '@/stores/toasts';
import type { Page } from '@/types/page';

const page: Page = {
  slug: 'a',
  title: 'A',
  description: '',
  createdAt: '',
  updatedAt: '',
  taskProject: 'floe',
  project: 'floe',
};
const values = (change: Partial<EditValues>): EditValues => ({
  title: 'A',
  slug: 'a',
  project: 'floe',
  description: '',
  ...change,
});

interface Options {
  refuse?: string;
  whole?: OpenAt | null;
  picked?: OpenAt | null;
  held?: boolean;
  offTask?: string;
}

// made makes the save over fakes that log, in order, what they were asked
function made({ refuse = '', whole = null, picked = null, held = false, offTask = '' }: Options = {}) {
  setBoot({ messages: { ui_saved: 'saved', ui_saved_off_task: 'saved, off %s' } } as unknown as Boot);
  const log: string[] = [];
  const toasts = createToasts();
  const writes = {
    writePage: (slug: string, init: RequestInit, options?: { answer?: (got: unknown) => void }) => {
      log.push(`patch ${slug} ${init.body as string}`);
      if (!refuse) options?.answer?.(offTask ? { offTask } : {});
      return Promise.resolve(refuse);
    },
  } as unknown as PageWrites;
  const pages = {
    load: () => {
      log.push(`load after ${String(toasts.shown.get().length)} toasts`);
      return Promise.resolve();
    },
  } as unknown as Pages;
  const openPage = { frameAt: () => whole, picked: createStore(picked) } as unknown as OpenPage;
  const order = { renameKept: (was: string, now: string) => log.push(`rename ${was} ${now}`) } as unknown as Order;
  const save = createSaveEdit({ writes, pages, toasts, openPage, order, go: (href) => log.push(`go ${href}`) });
  const whileHeld = async (write: () => Promise<void>) => {
    if (held) return false;
    await write();
    return true;
  };
  return { save: (v: EditValues) => save(page, v, whileHeld), log, toasts, openPage };
}

describe(createSaveEdit, () => {
  it('sends the fields changed, trimmed, says so and reads the list after', async () => {
    expect.assertions(3);
    const { save, log, toasts } = made();
    const got = await save(values({ title: ' B ', description: '  ' }));
    expect(log).toStrictEqual(['patch a {"title":"B"}', 'load after 1 toasts']);
    expect(toasts.shown.get().map((x) => x.text)).toStrictEqual(['saved']);
    expect(got).toStrictEqual({ ran: true, reason: '', field: '', slug: 'a', away: false });
  });

  it('says a page taken off its task by its new project', async () => {
    expect.assertions(1);
    const { save, toasts } = made({ offTask: '#1' });
    await save(values({ project: 'other' }));
    expect(toasts.shown.get().map((x) => x.text)).toStrictEqual(['saved, off #1']);
  });

  it('goes to the new name of the page open whole, keeping its rows, and reads nothing', async () => {
    expect.assertions(2);
    const { save, log } = made({ whole: { slug: 'a', file: '', v: 2 } });
    const got = await save(values({ slug: 'b' }));
    expect(log).toStrictEqual(['patch a {"slug":"b"}', 'rename a b', 'go /a/b/?v=2']);
    expect(got).toMatchObject({ slug: 'b', away: true });
  });

  it('keeps the page open beside the list under its new name, and leaves the rows kept alone', async () => {
    expect.assertions(3);
    const { save, log, openPage } = made({ picked: { slug: 'a', file: '', v: 0 } });
    const got = await save(values({ slug: 'b' }));
    expect(openPage.picked.get()).toStrictEqual({ slug: 'b', file: '', v: 0 });
    expect(log).toStrictEqual(['patch a {"slug":"b"}', 'load after 1 toasts']);
    expect(got).toMatchObject({ slug: 'b', away: false });
  });

  it('renames a row not open, moving nothing', async () => {
    expect.assertions(2);
    const { save, log, openPage } = made({ picked: { slug: 'z', file: '', v: 0 } });
    await save(values({ slug: 'b' }));
    expect(openPage.picked.get()?.slug).toBe('z');
    expect(log).toStrictEqual(['patch a {"slug":"b"}', 'load after 1 toasts']);
  });

  it('tells a refusal, with the slug sent before the project, then the project, then none', async () => {
    expect.assertions(4);
    const both = await made({ refuse: 'no' }).save(values({ slug: 'b', project: 'p' }));
    expect(both).toStrictEqual({ ran: true, reason: 'no', field: 'slug', slug: 'a', away: false });
    expect((await made({ refuse: 'no' }).save(values({ project: 'p' }))).field).toBe('project');
    expect((await made({ refuse: 'no' }).save(values({ title: 'B' }))).field).toBe('');
    const { save, log } = made({ refuse: 'no' });
    await save(values({ title: 'B' }));
    expect(log).toStrictEqual(['patch a {"title":"B"}']);
  });

  it('writes nothing for a second press while held', async () => {
    expect.assertions(2);
    const { save, log } = made({ held: true });
    const got = await save(values({ title: 'B' }));
    expect(log).toStrictEqual([]);
    expect(got.ran).toBe(false);
  });
});
