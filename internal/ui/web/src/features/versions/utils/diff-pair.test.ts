import { describe, expect, it } from 'vitest';

import { setBoot, type Boot } from '@/config/boot';
import { firstPair, fromFor, pairOptions } from '@/features/versions/utils/diff-pair';

const listOf = (read: number, ...ns: number[]) => ({
  versions: ns.map((n) => ({ n, at: '2026-11-01T09:00:00Z' })),
  read,
});

describe(fromFor, () => {
  it('compares with the version read before if older, else the one just before, and none before the first', () => {
    expect.assertions(1);
    const list = listOf(2, 1, 2, 3, 4);
    expect([fromFor(4, list), fromFor(2, list), fromFor(3, listOf(0, 1, 2, 3)), fromFor(1, list)]).toStrictEqual([
      2,
      1,
      2,
      undefined,
    ]);
  });
});

describe(firstPair, () => {
  it('takes the pair asked for, the default for what is left out', () => {
    expect.assertions(1);
    const list = listOf(2, 1, 2, 3, 4);
    expect(['?diff&from=1&to=3', '?diff', '?diff&to=3', '?diff&from=3'].map((q) => firstPair(q, list))).toStrictEqual([
      { pair: { from: 1, to: 3 }, bad: false },
      { pair: { from: 2, to: 4 }, bad: false },
      { pair: { from: 2, to: 3 }, bad: false },
      { pair: { from: 3, to: 4 }, bad: false },
    ]);
  });

  it('gives a pair that cannot be compared way to the first, saying so', () => {
    expect.assertions(1);
    const list = listOf(0, 1, 2, 3);
    const first = { pair: { from: 2, to: 3 }, bad: true };
    // Unreadable, not older, a version gone, to alone with none before it, from alone not older than the latest
    expect(
      ['?from=x&to=3', '?from=3&to=2', '?from=2&to=2', '?from=9&to=3', '?to=1', '?from=3', '?from=01&to=3'].map((q) =>
        firstPair(q, list),
      ),
    ).toStrictEqual(Array.from({ length: 7 }, () => first));
  });
});

describe(pairOptions, () => {
  it('lists the versions newest first, none chosen that would not be older or later than the other', () => {
    expect.assertions(2);
    setBoot({ messages: { ui_diff_latest: 'latest', ui_diff_read_version: 'read' } } as unknown as Boot);
    const list = listOf(1, 1, 2, 3);
    const from = pairOptions('from', { from: 1, to: 2 }, list);
    expect(from.map((o) => [o.n, o.off])).toStrictEqual([
      [3, true],
      [2, true],
      [1, false],
    ]);
    expect([
      from[0]?.text.replace(/\d\d\/\d\d \d\d:\d\d/, 't'),
      pairOptions('to', { from: 1, to: 2 }, list).map((o) => o.off),
    ]).toStrictEqual(['v3 · t · latest', [false, false, true]]);
  });
});
