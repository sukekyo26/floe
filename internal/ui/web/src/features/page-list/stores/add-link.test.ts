import { describe, expect, it } from 'vitest';

import { testBoot } from '@/components/test-boot';
import { createAddLink } from '@/features/page-list/stores/add-link';
import { createStore } from '@/lib/store';
import type { Client } from '@/lib/api-client';
import type { ListQuery } from '@/stores/list-query';
import type { Folders } from '@/stores/open-folders';
import type { Pages } from '@/stores/pages';
import type { Toasts } from '@/stores/toasts';

const made = (answer: Response, opened: string[] = []) => {
  testBoot();
  const did: string[] = [];
  const sent: unknown[] = [];
  const add = createAddLink({
    client: {
      api: (_url: string, init?: RequestInit) => {
        sent.push(JSON.parse(init?.body as string));
        return Promise.resolve(answer);
      },
    } as unknown as Client,
    query: {
      setWords: (q: string) => did.push(`words ${q}`),
      filter: (change: object) => did.push(`filter ${JSON.stringify(change)}`),
    } as unknown as ListQuery,
    folders: {
      list: createStore({ opened: new Set(opened), listings: new Map() }),
      toggle: (slug: string) => {
        did.push(`open ${slug}`);
        return Promise.resolve();
      },
    } as unknown as Folders,
    pages: {
      load: () => {
        did.push('load');
        return Promise.resolve();
      },
    } as unknown as Pages,
    toasts: { toast: (words: string) => did.push(`toast ${words}`) } as unknown as Toasts,
  });
  const held = async (write: () => Promise<void>) => {
    did.push('hold');
    await write();
    did.push('let go');
    return true;
  };
  const close = () => did.push('close');
  return { add, did, sent, held, close };
};
const json = (body: unknown) => new Response(JSON.stringify(body), { status: 200 });

describe(createAddLink, () => {
  it('sends the fields typed, cut, and not those left blank', async () => {
    expect.assertions(1);
    const { add, sent, held, close } = made(json({ slug: 'x' }));
    add.values.set({ path: ' /a/b.html ', slug: '', description: 'it ' });
    await add.send(held, close);
    expect(sent).toStrictEqual([{ path: '/a/b.html', description: 'it' }]);
  });

  it('holds the modal for the write alone, then closes it, shows the list whole with the folder open, and reads it', async () => {
    expect.assertions(3);
    const { add, did, held, close } = made(json({ slug: 'f', folder: true }));
    add.values.set({ path: '/a', slug: '', description: '' });
    await expect(add.send(held, close)).resolves.toStrictEqual({ added: true, reason: '' });
    expect(add.values.get()).toStrictEqual({ path: '', slug: '', description: '' });
    expect(did).toStrictEqual([
      'hold',
      'let go',
      'close',
      'toast ui_folder_added:f',
      'words ',
      'open f',
      'filter {"view":"all","kind":"","template":"","project":"","tag":""}',
      'load',
    ]);
  });

  it('opens no folder open already, and keeps the fields typed as a refusal is said', async () => {
    expect.assertions(3);
    const open = made(json({ slug: 'f', folder: true }), ['f']);
    await open.add.send(open.held, open.close);
    expect(open.did).not.toContain('open f');
    const refused = made(new Response('taken', { status: 409 }));
    refused.add.values.set({ path: '/a', slug: 'x', description: '' });
    await expect(refused.add.send(refused.held, refused.close)).resolves.toStrictEqual({
      added: false,
      reason: 'ui_http_error:taken',
    });
    expect(refused.add.values.get()).toStrictEqual({ path: '/a', slug: 'x', description: '' });
  });

  it('tells a write that failed with no words not added, as one answered with nothing', async () => {
    expect.assertions(1);
    const { add, held, close } = made(json(null));
    await expect(add.send(held, close)).resolves.toStrictEqual({ added: false, reason: '' });
  });
});
