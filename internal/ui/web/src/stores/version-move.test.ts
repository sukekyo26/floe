import { describe, expect, it } from 'vitest';

import type { Boot } from '@/config/boot';
import type { Kept } from '@/lib/storage';
import { createVersionMove } from '@/stores/version-move';

const memory = (held: Map<string, string>): Kept => ({
  read: (key) => held.get(key) ?? null,
  write: (key, value) => held.set(key, value),
  remove: (key) => held.delete(key),
});

describe(createVersionMove, () => {
  it("carries the focus and a page kept unread under version.move to the frame's page alone, once", () => {
    expect.assertions(3);
    const held = new Map<string, string>();
    const storage = memory(held);
    let framed = 'a';
    const move = createVersionMove({ boot: () => ({ framed }) as Boot, storage });
    move.keep(true, true);
    expect(held.get('version.move')).toBe('{"slug":"a","focus":true,"kept":true}');
    framed = 'b';
    expect(move.take()).toStrictEqual({ focus: false, kept: false });
    move.keep(false, true);
    expect(move.take()).toStrictEqual({ focus: false, kept: true });
  });

  it('leaves what is not JSON as it is', () => {
    expect.assertions(2);
    const held = new Map([['version.move', '{']]);
    const move = createVersionMove({ boot: () => ({ framed: 'a' }) as Boot, storage: memory(held) });
    expect(move.take()).toStrictEqual({ focus: false, kept: false });
    expect(held.get('version.move')).toBe('{');
  });
});
