import { describe, expect, it } from 'vitest';

import type { Boot } from '@/config/boot';
import type { Kept } from '@/lib/storage';
import type { ListQuery } from '@/stores/list-query';
import { createOrder } from '@/stores/order';

const memory = (): Kept & { held: Map<string, string> } => {
  const held = new Map<string, string>();
  return {
    held,
    read: (key) => held.get(key) ?? null,
    write: (key, value) => held.set(key, value),
    remove: (key) => held.delete(key),
  };
};

describe(createOrder, () => {
  it('keeps the rows under pages.order with where they came from, and goes back there from a page of them', () => {
    expect.assertions(3);
    const storage = memory();
    const order = createOrder({
      boot: () => ({ framed: 'b' }) as Boot,
      storage,
      listQuery: { savedList: () => '?view=fav' } as ListQuery,
      here: () => '/?q=x',
    });
    order.keepRows(['a', 'b'], ['a', 'b', 'd']);
    expect(storage.held.get('pages.order')).toBe('{"order":["a","b"],"shown":["a","b","d"],"back":"/?q=x"}');
    expect(order.keptBack()).toBe('/?q=x');
    order.forgetOrder();
    expect(order.keptBack()).toBe('/?view=fav');
  });

  it('takes no kept rows of another shape', () => {
    expect.assertions(1);
    const storage = memory();
    storage.held.set('pages.order', '{"order":"a"}');
    expect(createOrder({ storage }).keptOrder()).toBeNull();
  });
});
