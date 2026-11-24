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

  it('keeps a page renamed whole of the rows that opened it, and the row it was at', () => {
    expect.assertions(1);
    const storage = memory();
    const order = createOrder({ storage });
    order.writeKept({ order: ['a', 'b'], shown: ['b', 'c'], back: '/', at: { slug: 'b', file: '' } });
    order.renameKept('b', 'b2');
    expect(order.keptOrder()).toStrictEqual({
      order: ['a', 'b2'],
      shown: ['b2', 'c'],
      back: '/',
      at: { slug: 'b2', file: '' },
    });
  });

  it('takes the row come back to by the back button or the way back, once, and forgets the order either way', () => {
    expect.assertions(4);
    const at = { slug: 'a', file: '', v: 2 };
    const made = (here: string) => {
      const storage = memory();
      const order = createOrder({ boot: () => ({ framed: '' }) as Boot, storage, here: () => here });
      return { storage, order };
    };
    const pressed = made('/?q=x');
    pressed.order.writeKept({ order: ['a'], shown: ['a'], back: '/?q=x', at, pressed: true });
    pressed.order.comeBack(false);
    expect([pressed.order.takeCameFrom(), pressed.order.takeCameFrom()]).toStrictEqual([at, null]);
    expect(pressed.storage.held.has('pages.order')).toBe(false);
    // Neither pressed nor the way back: the logo, or the same list by its URL
    const typed = made('/?q=x');
    typed.order.writeKept({ order: ['a'], shown: ['a'], back: '/?q=x', at });
    typed.order.comeBack(false);
    expect(typed.order.takeCameFrom()).toBeNull();
    // Another list than the one that opened it
    const other = made('/?view=fav');
    other.order.writeKept({ order: ['a'], shown: ['a'], back: '/?q=x', at, pressed: true });
    other.order.comeBack(true);
    expect(other.order.takeCameFrom()).toBeNull();
  });

  it('takes the row come back to by the browser’s way back without a press, and makes it anew of what was kept', () => {
    expect.assertions(4);
    const made = () => {
      const storage = memory();
      return { storage, order: createOrder({ boot: () => ({ framed: '' }) as Boot, storage, here: () => '/' }) };
    };
    const back = made();
    back.order.writeKept({ order: [], shown: [], back: '/', at: { slug: 'a', file: 'x.html', v: 2 } });
    back.order.comeBack(true);
    expect(back.order.takeCameFrom()).toStrictEqual({ slug: 'a', file: 'x.html', v: 2 });
    // A version or a file not kept as such: the latest, and the entry itself
    const odd = made();
    odd.order.writeKept({ order: [], shown: [], back: '/', at: { slug: 'a', v: 'x' }, pressed: true });
    odd.order.comeBack(false);
    expect(odd.order.takeCameFrom()).toStrictEqual({ slug: 'a', file: '', v: 0 });
    // Words that are no JSON, and a kept order with no row: nothing to go back to
    const broken = made();
    broken.storage.write('pages.order', '{');
    broken.order.comeBack(true);
    expect(broken.order.takeCameFrom()).toBeNull();
    const none = made();
    none.order.writeKept({ order: [], shown: [], back: '/', pressed: true });
    none.order.comeBack(true);
    expect(none.order.takeCameFrom()).toBeNull();
  });

  it('keeps where the frame is for the rows that opened it, and none for a frame not of them', () => {
    expect.assertions(2);
    const at = { slug: 'b', file: '', v: 2 };
    const of = (framed: string) => {
      const order = createOrder({ boot: () => ({ framed }) as Boot, storage: memory(), here: () => '/' });
      order.keepRows(['a', 'b'], ['a', 'b']);
      order.keepAt(at);
      return order.keptOrder();
    };
    expect(of('b')).toMatchObject({ at });
    expect(of('z')).toMatchObject({ at: null });
  });
});
