import type { Client } from '@/lib/api-client';
import { t } from '@/lib/t';
import type { MessageKey } from '@/types/messages.gen';
import { PAGES_API } from '@/utils/page-urls';

export interface Theme {
  name: string;
  about: string;
  // dir is where one's own theme is, '' for a built-in one
  dir: string;
}

// A mistake that stops every drawing: one's own parts, themes or templates
export interface Mistake {
  file?: string;
  line?: number;
  column?: number;
  text: string;
}

// ThemesRead is GET /api/themes: the themes, the default chosen and the task mode; settings is why settings.json could
// not be read, drawing what keeps every page from being drawn
export interface ThemesRead {
  themes: Theme[];
  default: string;
  taskMode: boolean;
  // settings is the report of why settings.json could not be read (the server's usererr.Report)
  settings?: object;
  drawing?: { mistakes?: Mistake[]; failure?: string };
}

const json = (body: unknown): RequestInit => ({
  method: 'PUT',
  headers: { 'content-type': 'application/json' },
  body: JSON.stringify(body),
});
// pageThemeWords are the words of a page's theme refused: changed meanwhile (409), or not drawn in it, its look kept
// (400: the theme, one's own things or the data); null for any other answer, said as the server words it
export const pageThemeWords = (status: number): MessageKey | null =>
  status === 409 ? 'ui_theme_page_changed' : status === 400 ? 'ui_theme_page_undrawn' : null;

const reason = async (res: Response) => (res.ok ? '' : t('ui_http_error', await res.text(), res.status));

// readThemes reads the themes, throwing what keeps them from being read
export async function readThemes(client: Client): Promise<ThemesRead> {
  const res = await client.api('/api/themes');
  if (!res.ok) throw new Error(t('ui_http_error', await res.text(), res.status));
  return (await res.json()) as ThemesRead;
}

// readPageTheme is the theme a page shows in without one of its own; '' when it cannot be read
export async function readPageTheme(client: Client, slug: string): Promise<string> {
  try {
    const res = await client.api(`${PAGES_API}/${encodeURIComponent(slug)}/theme`);
    if (!res.ok) return '';
    const got = (await res.json()) as { own?: string };
    return got.own ?? '';
  } catch {
    return '';
  }
}

// putSettings writes the default theme or the task mode alone: '' or the reason
export async function putSettings(client: Client, body: { theme: string } | { taskMode: boolean }): Promise<string> {
  try {
    return await reason(await client.api('/api/settings', json(body)));
  } catch (err) {
    return err instanceof Error ? err.message : String(err);
  }
}

// putPageTheme writes a page's theme ('' takes it off): '' or the reason, a page changed meanwhile (409) or not drawn
// in it (400) said as such, as the server's words are of the command line
export async function putPageTheme(client: Client, slug: string, theme: string): Promise<string> {
  try {
    const res = await client.api(`${PAGES_API}/${encodeURIComponent(slug)}/theme`, json({ theme }));
    const words = pageThemeWords(res.status);
    return words ? t(words) : await reason(res);
  } catch (err) {
    return err instanceof Error ? err.message : String(err);
  }
}
