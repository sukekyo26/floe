import { describe, expect, it } from 'vitest';

import { kept, readJSON } from '@/lib/storage';

// A storage whose every call throws, as a blocked or full one does
const refusing = () =>
  ({
    getItem: () => {
      throw new DOMException('blocked', 'SecurityError');
    },
    setItem: () => {
      throw new DOMException('full', 'QuotaExceededError');
    },
    removeItem: () => {
      throw new DOMException('blocked', 'SecurityError');
    },
  }) as unknown as Storage;

describe(kept, () => {
  it('goes on without a storage that throws, keeping nothing', () => {
    expect.assertions(2);
    const store = kept(refusing);
    store.write('k', 'v');
    store.remove('k');
    expect(store.read('k')).toBeNull();
    expect(readJSON(store, 'k')).toBeNull();
  });

  it('reads no storage until it is used', () => {
    expect.assertions(1);
    let reached = 0;
    kept(() => {
      reached++;
      return refusing();
    });
    expect(reached).toBe(0);
  });
});
