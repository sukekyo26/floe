import { describe, expect, it } from 'vitest';

import { setBoot, type Boot } from '@/config/boot';
import { versionLeave, versionView } from '@/features/versions/utils/version-view';
import type { Page } from '@/types/page';

const AT = '2026-11-01T09:00:00Z';
const list = { versions: [1, 2, 3].map((n) => ({ n, at: AT })), read: 2 };
const page = (over: Partial<Page> = {}) => ({ slug: 'a', version: 3, versionCount: 3, ...over }) as Page;
const at = (v = 0, file = '') => ({ slug: 'a', file, v });
const view = (over: Partial<Parameters<typeof versionView>[0]> = {}) => {
  setBoot({ messages: { ui_diff_latest: 'latest', ui_diff_read_version: 'read' } } as unknown as Boot);
  return versionView({ at: at(), page: page(), list, gone: false, diff: false, ...over });
};

describe(versionView, () => {
  it('shows the versions newest first, the latest chosen, with no words', () => {
    expect.assertions(1);
    const { select, note } = view();
    expect({
      ...select,
      options: select?.options.map((o) => `${String(o.n)} ${o.rest.replace(/^\S+ \S+/, 't')}`),
      note,
    }).toStrictEqual({
      hidden: false,
      old: false,
      value: 3,
      numbers: '1,2,3',
      options: ['3 t · latest', '2 t · read', '1 t'],
      note: null,
    });
  });

  it('marks a version other than the latest, with its words', () => {
    expect.assertions(1);
    const { select, note } = view({ at: at(1) });
    expect([select?.old, select?.value, note]).toStrictEqual([true, 1, { kind: 'old', n: 1, at: AT, last: 3 }]);
  });

  it('hides the choice with no versions read, none wanted or one to give way, the words of a version gone shown still', () => {
    expect.assertions(2);
    const hidden = [
      view({ list: null }),
      view({ page: null }),
      view({ at: null }),
      view({ at: at(0, 'x.html') }),
      view({ diff: true }),
      view({ page: page({ versionCount: 1 }) }),
      view({ page: page({ path: '/x' }) }),
      view({ at: at(3) }),
      view({ at: at(5) }),
    ];
    expect(hidden.map((v) => v.select)).toStrictEqual(Array.from(hidden, () => null));
    expect(
      [view({ list: null, gone: true }), view({ page: page({ versionCount: 1 }), gone: true })].map((v) => v.note),
    ).toStrictEqual([{ kind: 'gone' }, { kind: 'gone' }]);
  });

  it('keeps the choice out of sight of a page of one version opened at it, and the words of one gone over those of one old', () => {
    expect.assertions(2);
    expect(
      view({ at: at(1), page: page({ versionCount: 1 }), list: { versions: list.versions.slice(0, 2), read: 0 } })
        .select?.hidden,
    ).toBe(true);
    expect(view({ at: at(1), gone: true }).note).toStrictEqual({ kind: 'gone' });
  });
});

describe(versionLeave, () => {
  it('leaves a version come to be the latest, or gone, and none older or not chosen', () => {
    expect.assertions(1);
    expect([3, 5, 1, 0].map((v) => versionLeave(at(v), list))).toStrictEqual([
      { gone: false },
      { gone: true },
      null,
      null,
    ]);
  });
});
