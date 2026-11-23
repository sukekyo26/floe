import { describe, expect, it, vi } from 'vitest';

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

  it('tells the listeners after one that throws, the new value set, and reports the throw once', () => {
    expect.assertions(3);
    const reported: unknown[] = [];
    vi.stubGlobal('reportError', (e: unknown) => reported.push(e));
    const store = createStore(1);
    const told: number[] = [];
    store.subscribe(() => {
      throw new Error('a listener');
    });
    store.subscribe(() => told.push(store.get()));
    store.set(2);
    expect(told).toStrictEqual([2]);
    expect(store.get()).toBe(2);
    expect(reported).toStrictEqual([new Error('a listener')]);
    vi.unstubAllGlobals();
  });

  it('tells a set inside a listener to the rest as it did, each of the values in turn', () => {
    expect.assertions(1);
    const store = createStore(1);
    const told: number[] = [];
    const stop = store.subscribe(() => {
      stop();
      store.set(3);
    });
    store.subscribe(() => told.push(store.get()));
    store.set(2);
    expect(told).toStrictEqual([3, 3]);
  });
});
