import {
  readPageTheme as readPageThemeOf,
  readThemes as readThemesOf,
  type ThemesRead,
} from '@/features/settings/api/themes';
import { client } from '@/lib/api-client';
import { createStore, type Store } from '@/lib/store';
import { t } from '@/lib/t';
import type { Page } from '@/types/page';

// The settings as opened: which opening this is, what was read, the theme a page shows in without its own, and why they
// could not be read
export interface SettingsState {
  opened: number;
  read: ThemesRead | null;
  own: string;
  failed: string;
}

export interface Settings {
  state: Store<SettingsState>;
  // open reads the themes (and a page's own) anew; an answer of an opening since gone is dropped
  open: (page: Page | null) => Promise<void>;
  // saved is the default theme or the task mode written: the settings read hold it, settings.json readable again
  saved: (change: { default: string } | { taskMode: boolean }) => void;
}

interface Deps {
  readThemes?: () => Promise<ThemesRead>;
  readPageTheme?: (slug: string) => Promise<string>;
}

export function createSettings({
  readThemes = () => readThemesOf(client),
  readPageTheme = (slug) => readPageThemeOf(client, slug),
}: Deps = {}): Settings {
  const state = createStore<SettingsState>({ opened: 0, read: null, own: '', failed: '' });
  return {
    state,
    open: async (page) => {
      const opened = state.get().opened + 1;
      state.set({ opened, read: null, own: '', failed: '' });
      // Asked together: a page's own theme is told on its first card, '' when it cannot be read
      const own = page ? readPageTheme(page.slug) : Promise.resolve('');
      let read: ThemesRead;
      try {
        read = await readThemes();
      } catch (err) {
        if (state.get().opened !== opened) return;
        const why = err instanceof Error ? err.message : String(err);
        state.set({ ...state.get(), failed: t('ui_theme_load_failed', why) });
        return;
      }
      const ownNow = await own;
      if (state.get().opened !== opened) return;
      state.set({ ...state.get(), read, own: ownNow });
    },
    saved: (change) => {
      const now = state.get();
      if (!now.read) return;
      const { settings: _gone, ...read } = now.read;
      state.set({ ...now, read: { ...read, ...change } });
    },
  };
}

export const settings = createSettings();
