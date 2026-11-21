import { describe, expect, it } from 'vitest';

import type { Client } from '@/lib/api-client';
import { createPageNote } from '@/stores/page-note';

// A server answering /versions with read and /diff with the counts of changes, or failing it
function client(read: number | undefined, counts = { changed: 1, added: 1, removed: 0 }, diffOk = true): Client {
  return {
    api: (url: string) =>
      Promise.resolve(
        url.endsWith('/versions')
          ? new Response(JSON.stringify({ read }))
          : new Response(JSON.stringify({ counts }), { status: diffOk ? 200 : 500 }),
      ),
  } as unknown as Client;
}

describe(createPageNote, () => {
  it('says how many places changed since the version read before', async () => {
    expect.assertions(2);
    const n = createPageNote({ client: client(2) });
    await n.show('a', 4);
    expect(n.note.get()).toStrictEqual({ slug: 'a', read: 2, changed: 2 });
    n.clear();
    expect([n.note.get(), n.to()]).toStrictEqual([null, null]);
  });

  it('says nothing with no version read before, one not older, no change, or a diff refused', async () => {
    expect.assertions(1);
    const said = async (c: Client) => {
      const n = createPageNote({ client: c });
      await n.show('a', 4);
      return n.note.get();
    };
    await expect(
      Promise.all([
        said(client(undefined)),
        said(client(4)),
        said(client(2, { changed: 0, added: 0, removed: 0 })),
        said(client(2, undefined, false)),
      ]),
    ).resolves.toStrictEqual([null, null, null, null]);
  });

  it('drops an answer come after the words were cleared', async () => {
    expect.assertions(1);
    const n = createPageNote({ client: client(2) });
    const asked = n.show('a', 4);
    n.clear();
    await asked;
    expect(n.note.get()).toBeNull();
  });
});
