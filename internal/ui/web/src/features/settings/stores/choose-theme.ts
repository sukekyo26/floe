import { putPageTheme as putPageThemeOf, putSettings as putSettingsOf } from '@/features/settings/api/themes';
import { client } from '@/lib/api-client';
import { t } from '@/lib/t';
import { pages as pagePages, type Pages } from '@/stores/pages';
import { toasts as pageToasts, type Toasts } from '@/stores/toasts';
import type { Page } from '@/types/page';

type WhileHeld = (write: () => Promise<void>) => Promise<boolean>;

// What came of a write: ran is false for a second press while held, which leaves the modal as it is
export interface Written {
  ran: boolean;
  why: string;
}

export interface ThemeWrites {
  // chooseTheme writes the default theme, or a page's (page): a page's is said and the list read, held, before the
  // caller closes the modal; the default's is told to the settings (saved) and said, the modal left open on its list
  chooseTheme: (call: {
    page: Page | null;
    theme: string;
    whileHeld: WhileHeld;
    saved: () => void;
  }) => Promise<Written>;
  // saveTaskMode writes the task mode alone, the default theme as it is
  saveTaskMode: (call: { on: boolean; whileHeld: WhileHeld; saved: () => void }) => Promise<Written>;
}

interface Deps {
  putSettings?: (body: { theme: string } | { taskMode: boolean }) => Promise<string>;
  putPageTheme?: (slug: string, theme: string) => Promise<string>;
  toasts?: Toasts;
  pages?: Pages;
}

// A refusal is said in a toast, in the modal (useMovedOverlay), which stays open to try again
export function createThemeWrites({
  putSettings = (body) => putSettingsOf(client, body),
  putPageTheme = (slug, theme) => putPageThemeOf(client, slug, theme),
  toasts = pageToasts,
  pages = pagePages,
}: Deps = {}): ThemeWrites {
  return {
    chooseTheme: async ({ page, theme, whileHeld, saved }) => {
      let why = '';
      const ran = await whileHeld(async () => {
        why = page ? await putPageTheme(page.slug, theme) : await putSettings({ theme });
        if (why || !page) return;
        toasts.toast(theme ? t('ui_theme_page_set', theme) : t('ui_theme_page_unset'));
        await pages.load();
      });
      if (!ran) return { ran, why };
      if (why) toasts.toast(t(page ? 'ui_theme_page_failed' : 'ui_theme_set_failed', why));
      else if (!page) {
        saved();
        toasts.toast(t('ui_theme_set', theme));
      }
      return { ran, why };
    },
    saveTaskMode: async ({ on, whileHeld, saved }) => {
      let why = '';
      const ran = await whileHeld(async () => {
        why = await putSettings({ taskMode: on });
      });
      if (!ran) return { ran, why };
      if (why) toasts.toast(t('ui_task_mode_failed', why));
      else {
        saved();
        toasts.toast(t(on ? 'ui_task_mode_set_on' : 'ui_task_mode_set_off'));
      }
      return { ran, why };
    },
  };
}

export const themeWrites = createThemeWrites();
