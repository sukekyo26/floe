import { describe, expect, it } from 'vitest';

import { setBoot, type Boot } from '@/config/boot';
import type { ThemesRead } from '@/features/settings/api/themes';
import { createThemeWrites } from '@/features/settings/stores/choose-theme';
import { createSettings } from '@/features/settings/stores/settings';
import type { Pages } from '@/stores/pages';
import { createToasts } from '@/stores/toasts';
import type { Page } from '@/types/page';

const messages = new Proxy({}, { get: (_, key) => `${String(key)}:%s` });
const read: ThemesRead = { themes: [], default: 'mine', taskMode: false, settings: { errors: [] } };
const page = { slug: 'a', title: 'A' } as Page;

describe(createSettings, () => {
  it('drops a refusal of an opening gone since', async () => {
    expect.assertions(1);
    setBoot({ messages } as unknown as Boot);
    const { promise: late, reject } = Promise.withResolvers<ThemesRead>();
    const answers = (function* answer() {
      yield late;
      for (;;) yield Promise.resolve(read);
    })();
    const s = createSettings({ readThemes: () => answers.next().value, readPageTheme: () => Promise.resolve('') });
    const opening = s.open(null);
    await s.open(null);
    reject(new Error('lost'));
    await opening;
    expect([s.state.get().failed, s.state.get().read]).toStrictEqual(['', read]);
  });

  it('drops the answer of an opening gone since, and says a read that failed', async () => {
    expect.assertions(3);
    setBoot({ messages } as unknown as Boot);
    const { promise: first, resolve: answer } = Promise.withResolvers<ThemesRead>();
    // The first opening's answer comes late, the second's is a refusal, any after are read
    const answers = (function* answer() {
      yield first;
      yield Promise.reject(new Error('lost'));
      for (;;) yield Promise.resolve(read);
    })();
    const s = createSettings({
      readThemes: () => answers.next().value,
      readPageTheme: () => Promise.resolve('own'),
    });
    const opening = s.open(page);
    await s.open(null);
    answer(read);
    await opening;
    expect([s.state.get().opened, s.state.get().read, s.state.get().failed]).toStrictEqual([
      2,
      null,
      'ui_theme_load_failed:lost',
    ]);
    await s.open(page);
    expect([s.state.get().read, s.state.get().own]).toStrictEqual([read, 'own']);
    s.saved({ taskMode: true });
    // Written, settings.json is readable again
    expect(s.state.get().read).toStrictEqual({ themes: [], default: 'mine', taskMode: true });
  });
});

describe(createThemeWrites, () => {
  // made makes the writes over fakes that log, in order, what they were asked
  function made(why = '', held = false) {
    setBoot({ messages } as unknown as Boot);
    const log: string[] = [];
    const toasts = createToasts();
    const writes = createThemeWrites({
      putSettings: (body) => {
        log.push(`put settings ${JSON.stringify(body)}`);
        return Promise.resolve(why);
      },
      putPageTheme: (slug, theme) => {
        log.push(`put ${slug} ${theme}`);
        return Promise.resolve(why);
      },
      toasts,
      pages: {
        load: () => {
          log.push(`load after ${String(toasts.shown.get().length)} toasts`);
          return Promise.resolve();
        },
      } as unknown as Pages,
    });
    const whileHeld = async (write: () => Promise<void>) => {
      if (held) return false;
      log.push('hold');
      await write();
      log.push('let go');
      return true;
    };
    const saved = () => log.push('saved');
    return { writes, log, toasts, whileHeld, saved };
  }
  const texts = (toasts: ReturnType<typeof createToasts>) => toasts.shown.get().map((x) => x.text);

  it('says a page’s theme and reads the list, both held, before the modal closes (S427)', async () => {
    expect.assertions(2);
    const { writes, log, toasts, whileHeld, saved } = made();
    const got = await writes.chooseTheme({ page, theme: 'mine', whileHeld, saved });
    expect(log).toStrictEqual(['hold', 'put a mine', 'load after 1 toasts', 'let go']);
    expect([texts(toasts), got]).toStrictEqual([['ui_theme_page_set:mine'], { ran: true, why: '' }]);
  });

  it('tells the default written to the settings and says it, reading nothing; a refusal is said and saved nothing', async () => {
    expect.assertions(3);
    const ok = made();
    await ok.writes.chooseTheme({ page: null, theme: 'mine', whileHeld: ok.whileHeld, saved: ok.saved });
    expect([ok.log, texts(ok.toasts)]).toStrictEqual([
      ['hold', 'put settings {"theme":"mine"}', 'let go', 'saved'],
      ['ui_theme_set:mine'],
    ]);
    const refused = made('no');
    await refused.writes.chooseTheme({ page, theme: '', whileHeld: refused.whileHeld, saved: refused.saved });
    expect(texts(refused.toasts)).toStrictEqual(['ui_theme_page_failed:no']);
    const mode = made('no');
    await mode.writes.saveTaskMode({ on: true, whileHeld: mode.whileHeld, saved: mode.saved });
    expect([mode.log.includes('saved'), texts(mode.toasts)]).toStrictEqual([false, ['ui_task_mode_failed:no']]);
  });

  it('writes, tells and says nothing for a second press while held', async () => {
    expect.assertions(2);
    const { writes, log, toasts, whileHeld, saved } = made('', true);
    const got = await writes.chooseTheme({ page, theme: 'mine', whileHeld, saved });
    expect([log, got.ran]).toStrictEqual([[], false]);
    await writes.chooseTheme({ page: null, theme: 'mine', whileHeld, saved });
    await writes.saveTaskMode({ on: true, whileHeld, saved });
    expect([log, texts(toasts)]).toStrictEqual([[], []]);
  });
});
