import { describe, expect, it } from 'vitest';

import { setBoot, type Boot } from '@/config/boot';
import { verMonth, verRest } from '@/features/versions/utils/version-words';

const at = (month: number, day: number, h: number, m: number) => new Date(2026, month - 1, day, h, m).toISOString();

describe(verRest, () => {
  it('tells the time, and the latest or the version read before', () => {
    expect.assertions(2);
    setBoot({ messages: { ui_diff_latest: 'latest', ui_diff_read_version: 'read' } } as unknown as Boot);
    expect(verMonth(at(3, 4, 5, 6))).toBe('03/04 05:06');
    const list = { versions: [1, 2, 3].map((n) => ({ n, at: at(11, n, 9, 0) })), read: 2 };
    expect(list.versions.map((v) => verRest(v, list))).toStrictEqual([
      '11/01 09:00',
      '11/02 09:00 · read',
      '11/03 09:00 · latest',
    ]);
  });
});
