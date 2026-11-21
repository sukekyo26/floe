import { describe, expect, it } from 'vitest';

import { createStore } from '@/lib/store';

describe(createStore, () => {
  it('tells its listeners of a new value, not of the same one, and none once let go', () => {
    expect.assertions(3);
    const store = createStore(1);
    const told: number[] = [];
    const stop = store.subscribe(() => told.push(store.get()));
    store.set(2);
    store.set(2);
    expect(told).toStrictEqual([2]);
    stop();
    store.set(3);
    expect(told).toStrictEqual([2]);
    expect(store.get()).toBe(3);
  });
});
