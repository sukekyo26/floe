import { describe, expect, it } from 'vitest';

import { pageMessage, type MessageFrom } from '@/utils/page-message';

const here = 'http://127.0.0.1:7325';
const from = (change: Partial<MessageFrom> = {}): MessageFrom => ({
  frame: 'beside',
  fromShown: true,
  origin: 'null',
  givenPath: '/a/p/',
  focused: true,
  keys: new Set(['/', '?', 'a']),
  here,
  ...change,
});
const served = { pagePath: '/a/p/', pageServed: { n: 2, at: 't' } };

describe(pageMessage, () => {
  it('takes no message but of the iframe shown, of origin null', () => {
    expect.assertions(2);
    expect(pageMessage(served, from({ fromShown: false }))).toStrictEqual([]);
    expect(pageMessage(served, from({ origin: here }))).toStrictEqual([]);
  });

  it('passes a key of the table and of PAGE_KEYS while the iframe has the focus, whatever path the page is at', () => {
    expect.assertions(1);
    expect([
      pageMessage({ pageKey: '/', pagePath: '/elsewhere' }, from()),
      pageMessage({ pageKey: 'x' }, from()),
      // In the table but no key a page may pass (archive)
      pageMessage({ pageKey: 'a' }, from()),
      pageMessage({ pageKey: '/' }, from({ focused: false })),
      pageMessage({ pageKey: 1 }, from()),
    ]).toStrictEqual([[{ kind: 'key', key: '/' }], [], [], [], []]);
  });

  it('moves to the list or a page whole of floe alone, and throws a message of no URL away whole', () => {
    expect.assertions(2);
    const open = (to: string) => pageMessage({ pageOpen: to }, from());
    expect([
      open('/'),
      open('/a/q/'),
      open('/tasks'),
      open('/a/q/?raw'),
      open('https://example.com/a/q/'),
    ]).toStrictEqual([[{ kind: 'open', href: `${here}/` }], [{ kind: 'open', href: `${here}/a/q/` }], [], [], []]);
    expect(pageMessage({ pageOpen: 'http://[', pageKey: '/', ...served }, from())).toStrictEqual([]);
  });

  it('reads the rest only from the path the iframe was given', () => {
    expect.assertions(2);
    expect(pageMessage({ ...served, pagePath: '/a/other/' }, from())).toStrictEqual([]);
    expect(pageMessage(served, from())).toStrictEqual([{ kind: 'served', mark: { n: 2, at: 't' } }]);
  });

  it('reads a diff changed again and nothing more of it, and a hash of the page whole alone', () => {
    expect.assertions(3);
    const changed = { ...served, pageChanged: true, pageHash: '#x' };
    expect(pageMessage(changed, from({ frame: 'diff' }))).toStrictEqual([{ kind: 'changed' }]);
    expect([
      pageMessage({ pagePath: '/a/p/', pageHash: '#x' }, from({ frame: 'whole' })),
      pageMessage({ pagePath: '/a/p/', pageHash: '' }, from({ frame: 'whole' })),
      pageMessage({ pagePath: '/a/p/', pageHash: 'x' }, from({ frame: 'whole' })),
      pageMessage({ pagePath: '/a/p/', pageHash: '#x' }, from()),
    ]).toStrictEqual([[{ kind: 'hash', hash: '#x' }], [{ kind: 'hash', hash: '' }], [], []]);
    expect(pageMessage(changed, from({ frame: 'whole' }))).toStrictEqual([
      { kind: 'hash', hash: '#x' },
      { kind: 'served', mark: { n: 2, at: 't' } },
    ]);
  });

  it('reads a version served of a whole number and a time', () => {
    expect.assertions(1);
    const read = (pageServed: unknown) => pageMessage({ pagePath: '/a/p/', pageServed }, from());
    expect([read({ n: 2.5, at: 't' }), read({ n: 2, at: 3 }), read(null)]).toStrictEqual([[], [], []]);
  });
});
