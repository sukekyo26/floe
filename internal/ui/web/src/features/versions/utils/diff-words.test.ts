import { describe, expect, it } from 'vitest';

import { setBoot, type Boot } from '@/config/boot';
import {
  countParts,
  frameSrc,
  levelOf,
  sectionName,
  type DiffAnswer,
  type Section,
} from '@/features/versions/utils/diff-words';

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

describe(frameSrc, () => {
  it('loads the later version marked, at a place or its top, and nothing before an answer or of no page', () => {
    expect.assertions(1);
    const answer = { from: { n: 1 }, to: { n: 3 }, key: 'k 1' } as DiffAnswer;
    expect([
      frameSrc('a', answer, -1),
      frameSrc('a', answer, 2),
      frameSrc('a', null, 2),
      frameSrc('', answer, 0),
    ]).toStrictEqual(['/a/a/?raw&v=3&from=1&key=k%201', '/a/a/?raw&v=3&from=1&key=k%201#floe-diff.2', null, null]);
  });
});
