import { describe, expect, it } from 'vitest';

import type { Kept } from '@/lib/storage';
import { createKeptSide } from '@/stores/kept-side';

// A storage of the values given, which records what is written
function storage(values: Record<string, string> = {}) {
  const reads: string[] = [];
  const kept: Kept = {
    read: (key) => {
      reads.push(key);
      return values[key] ?? null;
    },
    write: (key, value) => {
      values[key] = value;
    },
    remove: () => undefined,
  };
  return { kept, values, reads };
}

describe(createKeptSide, () => {
  it('reads what the browser holds at the first call alone, the default for what it cannot read', () => {
    expect.assertions(3);
    const s = storage({ a: '{"width":300,"open":false}', b: '{"width":"wide","open":1}', c: 'not JSON' });
    const a = createKeptSide('a', 240, { storage: s.kept });
    expect(s.reads).toStrictEqual([]);
    expect(a.side().get()).toStrictEqual({ width: 300, open: false });
    expect(['b', 'c', 'd'].map((key) => createKeptSide(key, 240, { storage: s.kept }).side().get())).toStrictEqual(
      Array.from({ length: 3 }, () => ({ width: 240, open: true })),
    );
  });

  it('keeps the width and whether it shows under its own key, as today', () => {
    expect.assertions(1);
    const s = storage();
    const toc = createKeptSide('diff.toc', 240, { storage: s.kept });
    const tree = createKeptSide('frame.side', 280, { storage: s.kept });
    toc.setWidth(320);
    toc.setOpen(false);
    toc.keep();
    tree.keep();
    expect(s.values).toStrictEqual({
      'diff.toc': '{"width":320,"open":false}',
      'frame.side': '{"width":280,"open":true}',
    });
  });
});
