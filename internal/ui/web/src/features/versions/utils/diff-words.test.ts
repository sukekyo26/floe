import { describe, expect, it } from 'vitest';

import { setBoot, type Boot } from '@/config/boot';
import { countParts, levelOf, sectionName, type Section } from '@/features/versions/utils/diff-words';

const section = (state: Section['state'], heading: [string, string][], level = 2): Section => ({
  state,
  level,
  heading: heading.map(([op, text]) => ({ op, text })),
});

describe(countParts, () => {
  it('leaves out none added or removed, and says all the same in one sentence', () => {
    expect.assertions(2);
    expect(
      countParts({ changed: 1, added: 0, removed: 2, same: 0 })?.map((x) => `${x.cls}${String(x.n)}`),
    ).toStrictEqual(['c1', 'r2', 's0']);
    expect(countParts({ changed: 0, added: 0, removed: 0, same: 5 })).toBeNull();
  });
});

describe(sectionName, () => {
  it('names a removed section by its deleted words, any other without what was deleted, none the top', () => {
    expect.assertions(1);
    setBoot({ messages: { ui_diff_top: 'top' } } as unknown as Boot);
    const words: [string, string][] = [
      ['', 'A '],
      ['del', 'old'],
      ['ins', 'new'],
    ];
    expect([
      sectionName(section('changed', words)),
      sectionName(section('removed', [['del', 'Gone']])),
      sectionName(section('same', [])),
    ]).toStrictEqual(['A new', 'Gone', 'top']);
  });
});

describe(levelOf, () => {
  it('keeps a level within the three of the contents', () => {
    expect.assertions(1);
    expect([0, 1, 3, 5].map((level) => levelOf(section('same', [], level)))).toStrictEqual([1, 1, 3, 3]);
  });
});
