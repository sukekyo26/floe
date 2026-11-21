import { describe, expect, it } from 'vitest';

import type { ThemesRead } from '@/features/settings/api/themes';
import { settingsView, themeCards } from '@/features/settings/utils/settings-view';

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
    expect(values(settingsView('list', false, read({ settings: 'bad json' }), '', 'broken'))).toStrictEqual([
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
    const now = themeCards(read({ default: 'mine' }), '', null, '');
    expect([now.now, now.checked, now.cards.map((c) => c.now)]).toStrictEqual([
      { kind: 'now', name: 'mine' },
      'mine',
      [false, true],
    ]);
    expect(themeCards(read(), '', null, '').checked).toBe('default');
    const gone = themeCards(read({ default: 'lost' }), '', null, '');
    expect([gone.now, gone.checked, gone.cards.some((c) => c.now)]).toStrictEqual([
      { kind: 'gone', name: 'lost' },
      'default',
      false,
    ]);
  });

  it('says settings.json unreadable with no card now, and one’s own things broken with their mistakes and no cards', () => {
    expect.assertions(2);
    const settings = themeCards(read({ settings: 'bad' }), '', null, '');
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
      '',
    );
    expect([broken.now, broken.cards, broken.mistakes]).toStrictEqual([
      { kind: 'ownBroken' },
      [],
      ['p.html:3:2: bad', 'odd', 'stopped'],
    ]);
  });

  it('puts the card that takes a page’s theme off first, checking the page’s theme or telling it gone', () => {
    expect.assertions(3);
    const none = themeCards(read(), '', '', 'shows in default');
    expect([none.cards[0], none.checked, none.now]).toStrictEqual([
      { name: '', about: 'shows in default', dir: '', now: false },
      '',
      { kind: 'pageMore' },
    ]);
    expect(themeCards(read(), '', 'mine', '').checked).toBe('mine');
    const gone = themeCards(read(), '', 'lost', '');
    expect([gone.now, gone.checked]).toStrictEqual([{ kind: 'pickedGone', name: 'lost' }, null]);
  });
});
