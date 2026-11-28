import type { Mistake, ThemesRead } from '@/features/settings/api/themes';
import type { MessageKey } from '@/types/messages.gen';
import type { Page } from '@/types/page';

// The part of the settings shown: the list, the default theme, the task mode; a page's themes are theme alone
export type Panel = 'list' | 'theme' | 'mode';

const TITLES: Record<Panel, MessageKey> = { list: 'ui_settings', theme: 'ui_settings_theme', mode: 'ui_settings_mode' };

export interface SettingsView {
  title: MessageKey;
  back: boolean;
  list: boolean;
  // broken is the line said over the parts: why the settings could not be read, or that settings.json cannot be
  broken: string | null;
  themes: boolean;
  mode: boolean;
  // close is the close button's words: the list saves nothing, a part with its button that saves cancels
  close: MessageKey;
  // themeNow and modeNow are the list's values: '' while read, — when they cannot be
  themeNow: string;
  modeNow: MessageKey | '—' | '';
  // modeSaves tells the task mode can be saved: not before it is read, as saving would write what it does not know
  modeSaves: boolean;
}

// settingsView is what the settings show of panel
export function settingsView(
  panel: Panel,
  page: boolean,
  read: ThemesRead | null,
  failed: string,
  brokenWords: string,
): SettingsView {
  const broken = failed ? (panel === 'theme' ? null : failed) : panel === 'list' && read?.settings ? brokenWords : null;
  const unread = failed !== '' || read?.settings !== undefined;
  return {
    title: page ? 'ui_theme_page' : TITLES[panel],
    back: !page && panel !== 'list',
    list: panel === 'list',
    broken,
    themes: panel === 'theme',
    mode: panel === 'mode',
    close: panel === 'list' ? 'ui_close' : 'ui_cancel',
    themeNow: read === null ? (failed ? '—' : '') : unread ? '—' : read.default || 'default',
    modeNow: read === null ? (failed ? '—' : '') : unread ? '—' : read.taskMode ? 'ui_mode_on' : 'ui_mode_off',
    modeSaves: read !== null,
  };
}

// A card of a theme: the one that takes a page's theme off has no name, and its about in the sample's place
export interface Card {
  name: string;
  about: string;
  dir: string;
  now: boolean;
}

// The line over the cards: what the default or the page's theme is now, or why there are no cards
export type Now =
  | { kind: 'reading' }
  | { kind: 'failed'; why: string }
  | { kind: 'ownBroken' }
  | { kind: 'settingsBroken' }
  | { kind: 'now'; name: string }
  | { kind: 'gone'; name: string }
  | { kind: 'pageMore' }
  | { kind: 'pickedGone'; name: string };

export interface Cards {
  now: Now;
  cards: Card[];
  // checked is the card chosen as they are drawn; null for none
  checked: string | null;
  // mistakes are those that stop every drawing, in the cards' place
  mistakes: string[];
}

// toldAt is a mistake at its place as the CLI words it
export const toldAt = (m: Mistake): string =>
  (m.file ? `${[m.file, m.line, m.line && m.column].filter(Boolean).join(':')}: ` : '') + m.text;

// themeCards are the cards of the default's choice or of a page's: unset is the first card's about, of a page's
export function themeCards(read: ThemesRead | null, failed: string, pageTheme: string | null): Cards {
  if (failed) return { now: { kind: 'failed', why: failed }, cards: [], checked: null, mistakes: [] };
  if (!read) return { now: { kind: 'reading' }, cards: [], checked: null, mistakes: [] };
  if (read.drawing) {
    const mistakes = [...(read.drawing.mistakes ?? []).map(toldAt), read.drawing.failure ?? ''].filter(Boolean);
    return { now: { kind: 'ownBroken' }, cards: [], checked: null, mistakes };
  }
  const cards = (now: (name: string) => boolean) =>
    read.themes.map((th) => ({ name: th.name, about: th.about, dir: th.dir, now: now(th.name) }));
  if (pageTheme !== null) {
    const there = read.themes.some((th) => th.name === pageTheme);
    return {
      now: there ? { kind: 'pageMore' } : { kind: 'pickedGone', name: pageTheme },
      cards: cards(() => false),
      checked: there ? pageTheme : null,
      mistakes: [],
    };
  }
  const shown = read.default || 'default';
  const there = read.themes.some((th) => th.name === shown);
  const now: Now =
    read.settings !== undefined
      ? { kind: 'settingsBroken' }
      : there
        ? { kind: 'now', name: shown }
        : { kind: 'gone', name: shown };
  const drawn = cards((name) => name === shown && there && read.settings === undefined);
  // The default as it is, or the first card where it is none of them
  return { now, cards: drawn, checked: drawn.find((c) => c.now)?.name ?? drawn[0]?.name ?? null, mistakes: [] };
}

// pageShown is the page whose themes the settings draw: the one opened for, from the first draw of the opening (the
// modal shows on it at once, so a way back drawn for a moment would take the focus), kept while it closes (target null)
export const pageShown = (shown: Page | null, target: { page: Page | null } | null): Page | null =>
  target ? target.page : shown;
