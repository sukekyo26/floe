import { describe, expect, it, vi } from 'vitest';

import { setBoot, type Boot } from '@/config/boot';
import { createClient } from '@/lib/api-client';
import type { Kept } from '@/lib/storage';
import { createNotice } from '@/stores/notice';
import { createFolders } from '@/stores/open-folders';
import type { Page } from '@/types/page';

vi.stubGlobal('window', globalThis);

const LISTING = '{"dirs":[],"files":[],"more":0}';
const folder = (slug: string, more: Partial<Page> = {}): Page => ({
  slug,
  folder: true,
  description: '',
  createdAt: '',
  updatedAt: '',
  taskProject: '',
  ...more,
});

function memory(start: Record<string, string> = {}): Kept & { held: Map<string, string> } {
  const held = new Map(Object.entries(start));
  return {
    held,
    read: (key) => held.get(key) ?? null,
    write: (key, value) => held.set(key, value),
    remove: (key) => held.delete(key),
  };
}

// A server of directories: those named in gone are not there (404)
function made(gone: string[] = [], options: { search?: string; storage?: Kept } = {}) {
  setBoot({ messages: { ui_load_failed: 'failed: %s', ui_http_error: '%s (%d)' } } as unknown as Boot);
  const asked: string[] = [];
  vi.stubGlobal('fetch', (url: string) => {
    asked.push(url);
    const at = new URL(url, 'http://x');
    const key = `${at.pathname.split('/')[3] ?? ''}:${at.searchParams.get('dir') ?? ''}`;
    return Promise.resolve(gone.includes(key) ? new Response('', { status: 404 }) : new Response(LISTING));
  });
  const storage = options.storage ?? memory();
  const folders = createFolders({
    client: createClient(),
    notice: createNotice(),
    storage,
    search: () => options.search ?? '',
  });
  return { folders, asked, storage };
}

const signal = () => AbortSignal.timeout(1000);

describe(createFolders, () => {
  it('takes open= and list.open once, reads only folders still there under directories open, and closes one gone', async () => {
    expect.assertions(3);
    const storage = memory({ 'list.open': '["a:x","gone","b:y/z"]' });
    const { folders, asked } = made(['a:x'], { search: '?open=a', storage });
    const take = await folders.readOpen([folder('a'), folder('b')], signal());
    take?.();
    expect(asked.toSorted()).toStrictEqual(['/api/pages/a/files?dir=', '/api/pages/a/files?dir=x']);
    expect([...folders.list.get().opened].toSorted()).toStrictEqual(['a', 'b:y/z']);
    expect(storage.held.get('list.open')).toBe('["a","b:y/z"]');
  });

  it('keeps a directory of the tree open when gone, reads the page own one, and takes only the last read', async () => {
    expect.assertions(3);
    const { folders, asked } = made(['f:a']);
    const [first, second] = await Promise.all([
      folders.readSide('f', 'a/b.html', signal()),
      folders.readSide('f', 'a/b.html', signal()),
    ]);
    expect(first).toBeNull();
    second?.();
    expect(asked.slice(0, 2)).toStrictEqual(['/api/pages/f/files?dir=', '/api/pages/f/files?dir=a']);
    expect([...folders.side.get().opened].toSorted()).toStrictEqual(['', 'a']);
  });

  it('opens a directory and reads it, closes what is under one closed, and closes one gone', async () => {
    expect.assertions(3);
    const { folders } = made(['a:gone']);
    await folders.toggle('a', '');
    await folders.toggle('a', 'x');
    expect([...folders.list.get().opened]).toStrictEqual(['a', 'a:x']);
    await folders.toggle('a', '');
    expect([...folders.list.get().opened]).toStrictEqual([]);
    await folders.toggle('a', 'gone');
    expect([...folders.list.get().opened]).toStrictEqual([]);
  });

  it('lets a read of all open begun before a toggle take nothing', async () => {
    expect.assertions(1);
    const { folders } = made();
    const read = folders.readOpen([folder('a')], signal());
    await folders.toggle('a', '');
    await expect(read).resolves.toBeNull();
  });
});
