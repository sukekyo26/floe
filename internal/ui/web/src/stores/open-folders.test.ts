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
  const notice = createNotice();
  const folders = createFolders({
    client: createClient(),
    notice,
    storage,
    search: () => options.search ?? '',
  });
  return { folders, asked, storage, notice };
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

  it('opens a directory of the tree and reads it, and closes what is under one closed, the page own one still read', async () => {
    expect.assertions(4);
    const { folders } = made();
    (await folders.readSide('f', 'a/b.html', signal()))?.();
    const own = folders.side.get().fileListing;
    await folders.toggleSide('f', 'a/b.html', 'a/c');
    expect([...folders.side.get().listings.keys()].toSorted()).toStrictEqual(['', 'a', 'a/c']);
    await folders.toggleSide('f', 'a/b.html', 'a');
    expect([...folders.side.get().opened]).toStrictEqual(['']);
    expect([...folders.side.get().listings.keys()].toSorted()).toStrictEqual(['', 'a']);
    expect(folders.side.get().fileListing).toBe(own);
  });

  it('lets a read of the tree begun before a toggle take nothing, and takes no answer for one closed meanwhile', async () => {
    expect.assertions(2);
    const { folders } = made();
    const read = folders.readSide('f', 'b.html', signal());
    const opening = folders.toggleSide('f', 'b.html', 'x');
    await expect(read).resolves.toBeNull();
    await folders.toggleSide('f', 'b.html', 'x');
    await opening;
    expect(folders.side.get().listings.has('x')).toBe(false);
  });

  it('keeps a directory of the tree open when gone or unread, saying why it was not read', async () => {
    expect.assertions(3);
    const { folders, notice } = made(['f:gone']);
    await folders.toggleSide('f', 'b.html', 'gone');
    expect([folders.side.get().opened.has('gone'), folders.side.get().listings.has('gone')]).toStrictEqual([
      true,
      false,
    ]);
    vi.stubGlobal('fetch', () => Promise.reject(new Error('down')));
    await folders.toggleSide('f', 'b.html', 'x');
    expect(folders.side.get().opened.has('x')).toBe(true);
    expect(notice.said.get()).toStrictEqual({ message: 'failed: down', from: 'load' });
  });

  it('takes the page own directory opened again as what j and k go through, none when it cannot be read', async () => {
    expect.assertions(2);
    const { folders } = made();
    (await folders.readSide('f', 'a/b.html', signal()))?.();
    await folders.toggleSide('f', 'a/b.html', 'a');
    vi.stubGlobal('fetch', () =>
      Promise.resolve(new Response('{"dirs":[],"files":[{"file":"a/c.html","updatedAt":""}],"more":0}')),
    );
    await folders.toggleSide('f', 'a/b.html', 'a');
    expect(folders.side.get().fileListing?.files.map((p) => p.file)).toStrictEqual(['a/c.html']);
    await folders.toggleSide('f', 'a/b.html', 'a');
    vi.stubGlobal('fetch', () => Promise.resolve(new Response('no', { status: 500 })));
    await folders.toggleSide('f', 'a/b.html', 'a');
    expect(folders.side.get().fileListing).toBeUndefined();
  });
});
