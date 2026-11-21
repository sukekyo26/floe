import { describe, expect, it, vi } from 'vitest';

import { setBoot, type Boot } from '@/config/boot';
import { createClient } from '@/lib/api-client';
import type { Kept } from '@/lib/storage';
import { createListQuery } from '@/stores/list-query';
import { createNotice } from '@/stores/notice';
import { createFolders } from '@/stores/open-folders';
import type { Page } from '@/types/page';

vi.stubGlobal('window', globalThis);

const memory = (): Kept => {
  const held = new Map<string, string>();
  return {
    read: (key) => held.get(key) ?? null,
    write: (key, value) => held.set(key, value),
    remove: (key) => held.delete(key),
  };
};

const folder: Page = { slug: 'a', folder: true, description: '', createdAt: '', updatedAt: '', taskProject: '' };

describe(createListQuery, () => {
  it('opens the folder of open= though the URL is written without it before the list is read', async () => {
    expect.assertions(2);
    setBoot({ screen: 'list', states: [], messages: {} } as unknown as Boot);
    vi.stubGlobal('fetch', () => Promise.resolve(new Response('{"dirs":[],"files":[],"more":0}')));
    const url = { search: '?open=a' };
    const folders = createFolders({
      client: createClient(),
      notice: createNotice(),
      storage: memory(),
      search: () => url.search,
    });
    const query = createListQuery({
      boot: () => ({ screen: 'list', states: [] }) as unknown as Boot,
      search: () => url.search,
      storage: memory(),
      replace: (to) => {
        url.search = new URL(to).search;
      },
      href: () => `http://x/${url.search}`,
      folders,
    });
    query.sync();
    expect(url.search).toBe('');
    const take = await folders.readOpen([folder], AbortSignal.timeout(1000));
    take?.();
    expect([...folders.list.get().opened]).toStrictEqual(['a']);
  });
});
