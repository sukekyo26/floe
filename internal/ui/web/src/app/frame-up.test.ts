import { describe, expect, it } from 'vitest';

import { frameUp } from '@/app/frame-up';
import type { Boot } from '@/config/boot';
import { createOrder } from '@/stores/order';
import { backOf } from '@/utils/back';

describe(frameUp, () => {
  it('goes where the page was opened from, and its press keeps that the rows go on there', () => {
    expect.assertions(2);
    let held = '';
    const order = createOrder({
      boot: () => ({ framed: 'p' }) as Boot,
      storage: {
        read: () => held,
        write: (_, v) => {
          held = v;
        },
        remove: () => undefined,
      },
      here: () => '/?view=fav',
    });
    order.keepRows(['p'], ['p']);
    const up = frameUp(backOf(order.keptBack(), 'http://127.0.0.1:7325'), order);
    up.onPress();
    expect(up.href).toBe('/?view=fav');
    expect(order.keptOrder()).toMatchObject({ pressed: true });
  });
});
