import { renderToString } from 'react-dom/server';
import { describe, expect, it } from 'vitest';

import { testBoot } from '@/components/test-boot';
import type { ThemesRead } from '@/features/settings/api/themes';
import { SettingsDialog } from '@/features/settings/components/settings-dialog';
import type { ThemeWrites } from '@/features/settings/stores/choose-theme';
import type { Settings, SettingsState } from '@/features/settings/stores/settings';
import { createStore } from '@/lib/store';
import type { Page } from '@/types/page';

const read: ThemesRead = {
  themes: [
    { name: 'default', about: 'Plain', dir: '' },
    { name: 'mine', about: '', dir: '/t/mine' },
  ],
  default: 'mine',
  taskMode: true,
};
const draw = (state: Partial<SettingsState>, page: Page | null) => {
  testBoot();
  const settings = {
    state: createStore<SettingsState>({ opened: 1, read, own: '', failed: '', ...state }),
    open: () => Promise.resolve(),
    saved: () => undefined,
  } as Settings;
  return renderToString(
    <SettingsDialog
      target={{ page, back: null }}
      onClosed={() => undefined}
      settings={settings}
      writes={{} as ThemeWrites}
    />,
  );
};
const attrs = (html: string, start: string) => String(new RegExp(`${start}[^>]*>`).exec(html)?.[0]);
// radios are the theme cards' radios, as value: checked
const radios = (html: string) =>
  [...html.matchAll(/<input type="radio"[^>]*>/g)].map(
    (m) => `${String(/value="([^"]*)"/.exec(m[0])?.[1])}:${String(m[0].includes('checked=""'))}`,
  );

describe(SettingsDialog, () => {
  it('opens on its list, with the values read and its part told, the theme chosen checked', () => {
    expect.assertions(4);
    const html = draw({}, null);
    expect(attrs(html, '<dialog')).toMatch(/data-panel="list"[^>]*w-\[min\(40rem/);
    // The head's two marks, by name on the svg itself, the page's hidden
    expect(
      [...html.matchAll(/<svg data-testid="([^"]*)"([^>]*)>/g)].map(
        (m) => `${String(m[1])}:${String(m[2]?.includes('hidden=""'))}`,
      ),
    ).toStrictEqual(['page-icon:true', 'settings-icon:false']);
    expect(html).toMatch(/id="settings-theme-now"[^>]*>mine<.*id="settings-mode-now"[^>]*>ui_mode_on/s);
    expect(radios(html)).toStrictEqual(['default:false', 'mine:true']);
  });

  it('opens a page’s themes alone, wide, the card that takes its theme off first and checked', () => {
    expect.assertions(2);
    const html = draw({ own: 'default' }, { slug: 'a', title: 'A' } as Page);
    expect(attrs(html, '<dialog')).toMatch(/data-panel="theme"[^>]*w-\[min\(64rem/);
    expect(radios(html)).toStrictEqual([':true', 'default:false', 'mine:false']);
  });

  it('draws a page’s themes from its opening: no way back to take the focus', () => {
    expect.assertions(3);
    const html = draw({}, { slug: 'a', title: 'A' } as Page);
    expect(attrs(html, '<button type="button" id="settings-back"')).toContain('hidden=""');
    expect(html).toMatch(/id="settings-title"[^>]*>ui_theme_page:/);
    expect(attrs(html, '<svg data-testid="page-icon"')).not.toContain('hidden=""');
  });
});
