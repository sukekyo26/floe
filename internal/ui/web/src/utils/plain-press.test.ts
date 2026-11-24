import { describe, expect, it } from 'vitest';

import { onPlainPress, plainPress } from '@/utils/plain-press';

const press = { button: 0, ctrlKey: false, metaKey: false, shiftKey: false, altKey: false };

describe(plainPress, () => {
  it('takes the main button alone, and leaves another button or a modifier to the link', () => {
    expect.assertions(1);
    expect([
      plainPress(press),
      plainPress({ ...press, button: 1 }),
      plainPress({ ...press, ctrlKey: true }),
      plainPress({ ...press, metaKey: true }),
      plainPress({ ...press, shiftKey: true }),
      plainPress({ ...press, altKey: true }),
    ]).toStrictEqual([true, false, false, false, false, false]);
  });
});

describe(onPlainPress, () => {
  it('acts on a plain press, and not on one with a modifier, which opens a tab of its own', () => {
    expect.assertions(1);
    let acted = 0;
    const click = onPlainPress(() => {
      acted++;
    });
    click(press);
    click({ ...press, ctrlKey: true });
    click({ ...press, button: 1 });
    onPlainPress(undefined)(press);
    expect(acted).toBe(1);
  });
});
