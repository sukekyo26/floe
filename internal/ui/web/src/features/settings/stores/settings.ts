import { readThemes as readThemesOf, type ThemesRead } from '@/features/settings/api/themes';
import { client } from '@/lib/api-client';
import { createStore, type Store } from '@/lib/store';
import { t } from '@/lib/t';

// The settings as opened: which opening this is, what was read, and why they could not be read
export interface SettingsState {
  opened: number;
  read: ThemesRead | null;
  failed: string;
}

export interface Settings {
  state: Store<SettingsState>;
  // open reads the themes anew; an answer of an opening since gone is dropped
  open: () => Promise<void>;
  // saved is the default theme or the task mode written: the settings read hold it, settings.json readable again
  saved: (change: { default: string } | { taskMode: boolean }) => void;
}

interface Deps {
  readThemes?: () => Promise<ThemesRead>;
}

export function createSettings({ readThemes = () => readThemesOf(client) }: Deps = {}): Settings {
  const state = createStore<SettingsState>({ opened: 0, read: null, failed: '' });
  return {
    state,
    open: async () => {
      const opened = state.get().opened + 1;
      state.set({ opened, read: null, failed: '' });
      let read: ThemesRead;
      try {
        read = await readThemes();
      } catch (err) {
        if (state.get().opened !== opened) return;
        const why = err instanceof Error ? err.message : String(err);
        state.set({ ...state.get(), failed: t('ui_theme_load_failed', why) });
        return;
      }
      if (state.get().opened !== opened) return;
      state.set({ ...state.get(), read });
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
