import { describe, expect, it } from 'vitest';

import { setBoot, type Boot } from '@/config/boot';
import { createApplyTags } from '@/features/tags/stores/apply-tags';
import { createStore } from '@/lib/store';
import type { ActOn } from '@/stores/act-on';
import type { PageWrites } from '@/stores/page-writes';
import type { Pages } from '@/stores/pages';
import { createToasts } from '@/stores/toasts';
import type { Page } from '@/types/page';

const page = (slug: string, tags: string[]): Page => ({
  slug,
  title: slug,
  description: '',
  createdAt: '',
  updatedAt: '',
  taskProject: '',
  tags,
});
const words = { done: () => 'done', failed: () => 'failed' };

// made makes the apply over fakes that log, in order, what they were asked; held tells whether a hold is on already
function made({ held = false, throws = false } = {}) {
  setBoot({ messages: new Proxy({}, { get: (_, key) => String(key) }) } as unknown as Boot);
  const log: string[] = [];
  const toasts = createToasts();
  const state = createStore({ entries: [page('a', ['x']), page('b', [])] });
  const pages = {
    state,
    load: () => {
      log.push('load');
      return Promise.resolve();
    },
  } as unknown as Pages;
  const writes = {
    patchPage: (slug: string, body: unknown) => {
      log.push(`patch ${slug} ${JSON.stringify(body)}`);
      return Promise.resolve('');
    },
  } as unknown as PageWrites;
  const send: ActOn = async (act, _words, slugs) => {
    log.push(`send ${slugs.join(',')}`);
    if (throws) throw new Error('lost');
    await Promise.all(slugs.map(act));
  };
  const apply = createApplyTags({ pages, writes, toasts });
  const call = (add: string[], remove: string[]) =>
    apply({
      slugs: ['a', 'b', 'gone'],
      add,
      remove,
      words,
      send,
      after: (next) => log.push(`after ${[...next].join(',')}`),
      whileHeld: async (write) => {
        if (held) return false;
        await write();
        return true;
      },
      close: () => log.push('close'),
    });
  return { call, log, toasts };
}

describe(createApplyTags, () => {
  it('reads the list, tidies the opener, sends the pages it changes, and closes', async () => {
    expect.assertions(1);
    const { call, log } = made();
    await call(['x'], []);
    expect(log).toStrictEqual(['load', 'after b', 'send b', 'patch b {"tags":{"add":["x"],"remove":[]}}', 'close']);
  });

  it('says none changes, and closes, when no page is changed', async () => {
    expect.assertions(2);
    const { call, log, toasts } = made();
    await call([], ['y']);
    expect(log).toStrictEqual(['load', 'after ', 'close']);
    expect(toasts.shown.get().map((x) => x.text)).toStrictEqual(['ui_tag_bulk_none']);
  });

  it('closes when the send throws (TestUITagLine 230)', async () => {
    expect.assertions(2);
    const { call, log } = made({ throws: true });
    await expect(call(['x'], [])).rejects.toThrow('lost');
    expect(log.at(-1)).toBe('close');
  });

  it('leaves the modal, the list and the opener as they are for a second press while held', async () => {
    expect.assertions(1);
    const { call, log } = made({ held: true });
    await call(['x'], []);
    expect(log).toStrictEqual([]);
  });
});
