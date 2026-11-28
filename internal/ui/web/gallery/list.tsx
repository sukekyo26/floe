// / as the app draws it (App): the bar with the list's search, the shelf, the list under its head and the page beside
// it, and the modals they open, for real. A fake server answers the pages, the tasks, the themes and a folder's files,
// and takes the writes of the list (a page changed, removed or added as a link) in memory; the browser's storage is the
// gallery's own origin's, and the list's state is written into this page's URL. ?data= picks the data (full, empty,
// archived: every page archived, failed: the pages unreadable, throw: two reads the screen cannot draw)
import './gallery.css';

import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';

import { App } from '@/app/app';
import { setBoot, type Boot } from '@/config/boot';
import type { Page } from '@/types/page';

const params = new URLSearchParams(window.location.search);
const data = params.get('data') ?? 'full';
const ago = (h: number) => new Date(Date.now() - h * 3600000).toISOString();

const page = (slug: string, title: string, project: string, over: Partial<Page> = {}): Page => ({
  slug,
  title,
  description: '',
  project,
  taskProject: project || '/',
  createdAt: ago(48),
  updatedAt: ago(2),
  template: 'report',
  ...over,
});

// The pages: of three projects and none, unread and of two versions, starred, tagged, archived, a link and a folder
const PAGES: Page[] = [
  page('plan', 'Release plan', 'shop', { news: 'page', tags: ['plan'], updatedAt: ago(0.2) }),
  page('design', 'Checkout design', 'shop', { news: 'version', version: 2, versionCount: 2, favorite: true }),
  page('review', 'A review whose title is long enough to be cut on a narrow screen', 'shop', {
    tags: ['review', 'api'],
    template: 'design',
    updatedAt: ago(30),
  }),
  page('notes', 'Floe notes', 'floe', { updatedAt: ago(50), task: { name: 'S1' } }),
  page('free', 'A page of no project', '', { updatedAt: ago(80) }),
  page('mock', 'Mockup', 'shop', { path: '/home/me/mock/index.html', template: '', updatedAt: ago(5) }),
  page('docs', 'Docs', 'floe', { folder: true, path: '/home/me/docs', template: '', updatedAt: ago(26) }),
  page('old', 'Old copy', 'shop', { archived: true, updatedAt: ago(200) }),
];
const entries: Page[] =
  data === 'empty' ? [] : data === 'archived' ? PAGES.map((a) => ({ ...a, archived: true })) : structuredClone(PAGES);
const FILES = {
  '': { entries: [{ name: 'guide', dir: true }, { name: 'index.html' }, { name: 'setup.html' }] },
  guide: { entries: [{ name: 'start.html' }] },
};

let pagesRead = 0;
const THEMES = [
  { name: 'default', about: 'Plain, for any page', dir: '' },
  { name: 'paper', about: 'Warm paper, serif headings', dir: '' },
];
const json = (body: unknown, status = 200) => new Response(JSON.stringify(body), { status });
const bodyOf = (init?: RequestInit): Record<string, unknown> =>
  JSON.parse(typeof init?.body === 'string' ? init.body : '{}') as Record<string, unknown>;

window.fetch = (input: string | URL | Request, init?: RequestInit) => {
  const url = new URL(input instanceof Request ? input.url : input, window.location.href);
  const method = init?.method ?? 'GET';
  const answer = (res: Response) =>
    new Promise<Response>((done) => {
      setTimeout(() => {
        done(res);
      }, 120);
    });
  if (url.pathname === '/api/pages' && method === 'GET') {
    if (data === 'failed') return answer(new Response('the store is not there', { status: 500 }));
    return answer(json(data === 'throw' && ++pagesRead <= 2 ? [...entries, null] : entries));
  }
  if (url.pathname === '/api/pages' && method === 'POST') {
    const { path } = bodyOf(init) as { path?: string };
    const name = (path ?? 'link').split('/').filter(Boolean).at(-1) ?? 'link';
    const slug = name.replace(/\.html$/, '').replaceAll(/[^a-z0-9-]/g, '-');
    if (entries.some((a) => a.slug === slug)) return answer(new Response(`${slug} is there already`, { status: 409 }));
    const added = page(slug, name, 'shop', { path: path ?? '', template: '', updatedAt: ago(0) });
    entries.push(added);
    return answer(json(added));
  }
  if (url.pathname === '/api/tasks')
    return answer(json({ floe: { S1: { title: 'Task screens', state: 'doing', at: ago(3), rev: 1 } } }));
  if (url.pathname === '/api/themes') return answer(json({ themes: THEMES, default: '', taskMode: false }));
  if (url.pathname === '/api/settings' || url.pathname.endsWith('/read')) return answer(new Response(''));
  if (url.pathname.endsWith('/theme')) return answer(new Response(''));
  if (url.pathname.endsWith('/files')) {
    const dir = (url.searchParams.get('dir') ?? '') as keyof typeof FILES;
    return answer(dir in FILES ? json(FILES[dir]) : new Response('', { status: 404 }));
  }
  const own = /^\/api\/pages\/([^/]+)$/.exec(url.pathname);
  const at = entries.findIndex((x) => x.slug === decodeURIComponent(own?.[1] ?? ''));
  if (own && method === 'PATCH' && at >= 0) {
    Object.assign(entries[at] ?? {}, bodyOf(init));
    return answer(json(entries[at]));
  }
  if (own && method === 'DELETE' && at >= 0) {
    entries.splice(at, 1);
    return answer(new Response(''));
  }
  return answer(new Response('', { status: 404 }));
};

const SCENES = [
  ['?data=full', 'full'],
  ['?data=full&view=new', 'new'],
  ['?data=full&project=shop&tag=api', 'narrowed'],
  ['?data=full&q=notes', 'searched'],
  ['?data=full&open=docs', 'a folder open'],
  ['?data=empty', 'empty'],
  ['?data=archived', 'all archived'],
  ['?data=failed', 'unreadable'],
  ['?data=throw', 'a draw that throws'],
] as const;

function Scenes() {
  return (
    <nav className="fixed bottom-1 left-1 z-50 flex max-w-[60vw] flex-wrap gap-1 rounded-small bg-ink/80 p-1 text-label">
      {SCENES.map(([search, words]) => (
        <a key={search} href={search} className="rounded-small px-1.5 text-on-ink">
          {words}
        </a>
      ))}
    </nav>
  );
}

const boot = import.meta.glob<Boot>('./boot.json', { eager: true, import: 'default' })['./boot.json'];
const el = document.getElementById('root');
if (el !== null && boot) {
  setBoot({ ...boot, screen: 'list' });
  createRoot(el).render(
    <StrictMode>
      <Scenes />
      <App search={window.location.search} />
    </StrictMode>,
  );
} else if (el !== null) {
  el.textContent = 'gallery/boot.json is not there: start the gallery with just web-gallery';
}
