import { describe, expect, it } from 'vitest';

import { setBoot, type Boot } from '@/config/boot';
import { createNotice } from '@/stores/notice';
import type { Pages } from '@/stores/pages';
import { buttonOf, createToasts } from '@/stores/toasts';
import { createUndoToast } from '@/stores/undo-toast';

function made(ok: boolean, dropped = false) {
  setBoot({ messages: { ui_undo: 'Undo', ui_undone: 'Undone', ui_save_failed: 'failed: %s' } } as unknown as Boot);
  const said: string[] = [];
  const toasts = createToasts();
  const notice = createNotice();
  const pages = {
    load: () => {
      said.push('load');
      return Promise.resolve();
    },
  } as unknown as Pages;
  const undoToast = createUndoToast({ toasts, notice, pages });
  undoToast(
    'taken off',
    {
      undo: () => Promise.resolve(dropped ? undefined : { ok, text: 'stale' }),
      done: () => said.push('done'),
      away: () => said.push('away'),
    },
    { focus: true },
  );
  const button = () => buttonOf(toasts.shown.get()[0]);
  return { toasts, notice, said, button };
}

describe(createUndoToast, () => {
  it('takes the change back, says so, reads the list and then gives the focus back, its toast gone', async () => {
    expect.assertions(4);
    const { toasts, said, button } = made(true);
    expect(button()).toMatchObject({ undo: true, holds: true });
    expect(toasts.shown.get()[0]?.focus).toBe(true);
    await button()?.onPress();
    expect(toasts.shown.get().map((x) => x.text)).toStrictEqual(['Undone']);
    expect(said).toStrictEqual(['load', 'done']);
  });

  it('does nothing for a press dropped, its toast there still', async () => {
    expect.assertions(2);
    const { toasts, said, button } = made(true, true);
    await button()?.onPress();
    expect(toasts.shown.get().map((x) => x.text)).toStrictEqual(['taken off']);
    expect(said).toStrictEqual([]);
  });

  it('says a refusal on the error line, its toast gone all the same, and leaves for away on Escape', async () => {
    expect.assertions(4);
    const refused = made(false);
    await refused.button()?.onPress();
    expect(refused.toasts.shown.get()).toStrictEqual([]);
    expect(refused.notice.said.get().message).toBe('failed: stale');
    expect(refused.said).toStrictEqual([]);
    const left = made(true);
    left.button()?.onEscape?.();
    expect([left.toasts.shown.get().length, left.said]).toStrictEqual([0, ['away']]);
  });
});
