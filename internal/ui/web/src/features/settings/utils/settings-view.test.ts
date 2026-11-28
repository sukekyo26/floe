import { describe, expect, it } from 'vitest';

import { pageThemeWords, type ThemesRead } from '@/features/settings/api/themes';
import { pageShown, settingsView, themeCards } from '@/features/settings/utils/settings-view';
import type { Page } from '@/types/page';

const read = (change: Partial<ThemesRead> = {}): ThemesRead => ({
  themes: [
    { name: 'default', about: 'Plain', dir: '' },
    { name: 'mine', about: '', dir: '/t/mine' },
  ],
  default: '',
  taskMode: true,
  ...change,
});

describe(settingsView, () => {
  it('shows a part, its title, its way back and its close words, the page’s themes with no way to the list', () => {
    expect.assertions(3);
    const pick = (v: ReturnType<typeof settingsView>) => [v.title, v.back, v.list, v.themes, v.mode, v.close];
    expect(pick(settingsView('list', false, read(), '', 'broken'))).toStrictEqual([
      'ui_settings',
      false,
      true,
      false,
      false,
      'ui_close',
    ]);
    expect(pick(settingsView('mode', false, read(), '', 'broken'))).toStrictEqual([
      'ui_settings_mode',
      true,
      false,
      false,
      true,
      'ui_cancel',
    ]);
    expect(pick(settingsView('theme', true, read(), '', 'broken'))).toStrictEqual([
      'ui_theme_page',
      false,
      false,
      true,
      false,
      'ui_cancel',
    ]);
  });

  it('tells the values as read, — where they cannot be, and why, on the list and the mode but not the themes', () => {
    expect.assertions(4);
    const values = (v: ReturnType<typeof settingsView>) => [v.themeNow, v.modeNow, v.broken, v.modeSaves];
    expect(values(settingsView('list', false, null, '', 'broken'))).toStrictEqual(['', '', null, false]);
    expect(values(settingsView('list', false, read({ default: 'mine' }), '', 'broken'))).toStrictEqual([
      'mine',
      'ui_mode_on',
      null,
      true,
    ]);
    expect(values(settingsView('list', false, read({ settings: { errors: [] } }), '', 'broken'))).toStrictEqual([
      '—',
      '—',
      'broken',
      true,
    ]);
    expect([
      settingsView('mode', false, null, 'lost', 'broken').broken,
      settingsView('theme', false, null, 'lost', 'broken').broken,
    ]).toStrictEqual(['lost', null]);
  });
});

describe(themeCards, () => {
  it('chooses the default, or the first card when the default is none, and tells a default gone', () => {
    expect.assertions(3);
    const now = themeCards(read({ default: 'mine' }), '', null);
    expect([now.now, now.checked, now.cards.map((c) => c.now)]).toStrictEqual([
      { kind: 'now', name: 'mine' },
      'mine',
      [false, true],
    ]);
    expect(themeCards(read(), '', null).checked).toBe('default');
    const gone = themeCards(read({ default: 'lost' }), '', null);
    expect([gone.now, gone.checked, gone.cards.some((c) => c.now)]).toStrictEqual([
      { kind: 'gone', name: 'lost' },
      'default',
      false,
    ]);
  });

  it('says settings.json unreadable with no card now, and one’s own things broken with their mistakes and no cards', () => {
    expect.assertions(2);
    const settings = themeCards(read({ settings: { errors: [] } }), '', null);
    expect([settings.now, settings.cards.some((c) => c.now)]).toStrictEqual([{ kind: 'settingsBroken' }, false]);
    const broken = themeCards(
      read({
        drawing: {
          mistakes: [{ file: 'p.html', line: 3, column: 2, text: 'bad' }, { text: 'odd' }],
          failure: 'stopped',
        },
      }),
      '',
      null,
    );
    expect([broken.now, broken.cards, broken.mistakes]).toStrictEqual([
      { kind: 'ownBroken' },
      [],
      ['p.html:3:2: bad', 'odd', 'stopped'],
    ]);
  });

  it('checks the page’s theme among the themes alone, or tells it gone', () => {
    expect.assertions(2);
    const mine = themeCards(read(), '', 'mine');
    expect([mine.cards.map((c) => c.name), mine.checked, mine.now]).toStrictEqual([
      ['default', 'mine'],
      'mine',
      { kind: 'pageMore' },
    ]);
    const gone = themeCards(read(), '', 'lost');
    expect([gone.now, gone.checked]).toStrictEqual([{ kind: 'pickedGone', name: 'lost' }, null]);
  });
});

describe(pageShown, () => {
  it('draws the page opened for from its opening, and keeps it while the modal closes', () => {
    expect.assertions(3);
    const a = { slug: 'a', title: 'A' } as Page;
    const opened = pageShown(null, { page: a });
    expect(opened).toBe(a);
    expect(pageShown(opened, null)).toBe(a);
    expect(pageShown(opened, { page: null })).toBeNull();
  });
});

describe(pageThemeWords, () => {
  it('says a page changed meanwhile and a page not drawn in the theme, and leaves any other answer to the server', () => {
    expect.assertions(1);
    expect([pageThemeWords(409), pageThemeWords(400), pageThemeWords(500), pageThemeWords(200)]).toStrictEqual([
      'ui_theme_page_changed',
      'ui_theme_page_undrawn',
      null,
      null,
    ]);
  });
});
