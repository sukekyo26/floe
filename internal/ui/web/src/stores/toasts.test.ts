import { describe, expect, it, vi } from 'vitest';

import { createToasts, pressToastButton } from '@/stores/toasts';

const texts = (t: ReturnType<typeof createToasts>) => t.shown.get().map((x) => x.text);

describe(createToasts, () => {
  it('shows a toast for 3 s, one of its own time for that, and a page’s in the place of the page’s before', () => {
    expect.assertions(4);
    vi.useFakeTimers();
    const t = createToasts();
    t.toast('plain');
    t.toast('own', { ms: 8000, page: true });
    t.toast('next page', { ms: 5000, page: true });
    expect(texts(t)).toStrictEqual(['plain', 'next page']);
    vi.advanceTimersByTime(3000);
    expect(texts(t)).toStrictEqual(['next page']);
    expect(t.shown.get()[0]).toMatchObject({ timed: true, page: true });
    vi.advanceTimersByTime(2000);
    expect(texts(t)).toStrictEqual([]);
    vi.useRealTimers();
  });

  it('holds a toast while held, and gives it its whole time again once let go', () => {
    expect.assertions(3);
    vi.useFakeTimers();
    const t = createToasts();
    const id = t.toast('held', {
      ms: 8000,
      button: { label: 'undo', undo: true, holds: true, onPress: () => undefined },
    });
    vi.advanceTimersByTime(7000);
    t.hold(id);
    vi.advanceTimersByTime(60000);
    expect(texts(t)).toStrictEqual(['held']);
    t.again(id);
    vi.advanceTimersByTime(7999);
    expect(texts(t)).toStrictEqual(['held']);
    vi.advanceTimersByTime(1);
    expect(texts(t)).toStrictEqual([]);
    vi.useRealTimers();
  });

  it('takes the focus a toast held away with it, and marks only the toast told to take the focus', () => {
    expect.assertions(3);
    const t = createToasts();
    const button = { label: 'undo', undo: true, holds: false, onPress: () => undefined };
    const first = t.toast('first', { ms: 8000, button, focus: true });
    t.toast('second', { ms: 8000, button });
    expect(t.shown.get().map((x) => x.focus)).toStrictEqual([true, false]);
    t.focused.set(first);
    t.remove(first);
    expect(t.focused.get()).toBeNull();
    // @ts-expect-error a toast with a button is given its time
    t.toast('no time', { button });
    expect(texts(t)).toStrictEqual(['second', 'no time']);
  });
});

describe(pressToastButton, () => {
  it('holds a button that holds until its press is done, and no other', async () => {
    expect.assertions(3);
    const held: boolean[] = [];
    const hold = (on: boolean) => held.push(on);
    const { promise: going, resolve: done } = Promise.withResolvers<undefined>();
    pressToastButton({ label: 'Undo', undo: true, holds: true, onPress: () => going }, hold);
    expect(held).toStrictEqual([true]);
    done(undefined);
    await going;
    await Promise.resolve();
    expect(held).toStrictEqual([true, false]);
    pressToastButton({ label: 'Restore', undo: true, holds: false, onPress: () => Promise.resolve() }, hold);
    expect(held).toStrictEqual([true, false]);
  });
});
