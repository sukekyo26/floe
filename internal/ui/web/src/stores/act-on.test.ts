import { describe, expect, it } from 'vitest';

import { createActOn } from '@/stores/act-on';
import { createNotice } from '@/stores/notice';
import type { Pages } from '@/stores/pages';
import { createToasts } from '@/stores/toasts';

const words = {
  done: (n: number) => `done ${String(n)}`,
  failed: (n: number, why: string) => `failed ${String(n)}: ${why}`,
};

function made() {
  const log: string[] = [];
  const toasts = createToasts();
  const notice = createNotice();
  notice.say('before');
  const pages = {
    load: () => {
      log.push(`load after ${String(toasts.shown.get().length)} toasts`);
      return Promise.resolve();
    },
  } as unknown as Pages;
  return { act: createActOn({ toasts, notice, pages }), log, toasts, notice };
}

describe(createActOn, () => {
  it('says how many were written and the first refusal, then reads the list', async () => {
    expect.assertions(3);
    const { act, log, toasts, notice } = made();
    const answers: Record<string, string> = { a: '', b: 'no', c: '' };
    await act(
      (slug) => Promise.resolve(String(answers[slug])),
      words,
      ['a', 'b', 'c'],
      (reasons) => {
        log.push(`settled ${reasons.join(',')}`);
      },
    );
    expect(toasts.shown.get().map((x) => x.text)).toStrictEqual(['done 2']);
    expect(notice.said.get().message).toBe('failed 1: no');
    expect(log).toStrictEqual(['settled ,no,', 'load after 1 toasts']);
  });

  it('takes the error line back when all are written, and says nothing done when none is', async () => {
    expect.assertions(3);
    const ok = made();
    await ok.act(() => Promise.resolve(''), words, ['a']);
    expect(ok.notice.said.get().message).toBe('');
    const refused = made();
    await refused.act(() => Promise.resolve('no'), words, ['a']);
    expect(refused.toasts.shown.get()).toStrictEqual([]);
    expect(refused.notice.said.get().message).toBe('failed 1: no');
  });
});
