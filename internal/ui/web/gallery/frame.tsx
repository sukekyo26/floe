// /a/{slug}/ as the app draws it (App): a page whole, with its bar, tags, tree, notes, page or diff, and the modals they
// open, for real. A fake server answers the pages, the tasks, the themes, the versions, the diff and a folder's files,
// and takes the page's writes in memory; the page in the frame is the gallery's own page (gallery/page.html, sandboxed
// as the server's). ?scene= picks what is open, and ?back= where it was opened from (pages, tasks or task: the rows kept,
// as the list or the task's page keeps them; none, opened by its URL). j and k go to /a/…, the gallery's page alone. The bar's end reads the page's width against the window's (fits)
import './gallery.css';

import { StrictMode, useEffect, useState } from 'react';
import { createRoot } from 'react-dom/client';

import { startFrame } from '@/app/start';
import { App } from '@/app/app';
import { setBoot, type Boot } from '@/config/boot';
import type { Page } from '@/types/page';

const params = new URLSearchParams(window.location.search);
const scene = params.get('scene') ?? 'page';
const ago = (h: number) => new Date(Date.now() - h * 3600000).toISOString();

const page = (slug: string, title: string, over: Partial<Page> = {}): Page => ({
  slug,
  title,
  description: '',
  project: 'shop',
  taskProject: 'shop',
  createdAt: ago(48),
  updatedAt: ago(2),
  template: 'report',
  ...over,
});

const entries: Page[] = [
  page('plan', 'Release plan', { tags: ['plan'] }),
  page('design', 'Checkout design', { version: 3, versionCount: 3, tags: ['api', 'review'], favorite: true }),
  page('notes', 'Floe notes'),
  page('docs', 'Docs', { folder: true, path: '/home/me/docs', template: '' }),
  page('mock', 'Mockup', { path: '/home/me/mock/index.html', template: '', missing: true }),
];
const FILES = {
  '': { dirs: ['guide'], files: [{ file: 'index.html', updatedAt: ago(3) }], more: 0 },
  guide: {
    dirs: [],
    files: ['guide/a.html', 'guide/start.html'].map((file) => ({ file, updatedAt: ago(4) })),
    more: 0,
  },
};
const VERSIONS = { versions: [1, 2, 3].map((n) => ({ n, at: ago(30 - n * 6) })), read: 2 };
const DIFF = {
  key: 'k',
  counts: { changed: 1, added: 1, removed: 0, same: 1 },
  sections: [
    { state: 'same', level: 1, heading: [{ op: '', text: 'Overview' }] },
    { state: 'changed', level: 2, heading: [{ op: '', text: 'Plan' }], place: 0 },
    { state: 'added', level: 2, heading: [{ op: 'ins', text: 'A step added' }], place: 1 },
  ],
  places: [1, 2],
};
const THEMES = [{ name: 'default', about: 'Plain, for any page', dir: '' }];

// What each scene opens: the slug framed, whether the server served it (a folder's file), the version ?v= asked and the
// rest of the URL's query
const SCENES: Record<string, { framed: string; served?: boolean; version?: string; query?: string; words: string }> = {
  page: { framed: 'design', words: 'a page' },
  old: { framed: 'design', version: '2', query: 'v=2', words: 'an older version' },
  diff: { framed: 'design', query: 'diff', words: 'its diff' },
  folder: { framed: 'docs', served: true, query: 'file=guide%2Fstart.html', words: 'a folder’s page' },
  missing: { framed: 'gone', words: 'not found' },
  'link-gone': { framed: 'mock', words: 'a link’s file gone' },
};
const now = SCENES[scene] ?? { framed: 'design', words: 'a page' };

const json = (body: unknown, status = 200) => new Response(JSON.stringify(body), { status });
window.fetch = (input: string | URL | Request, init?: RequestInit) => {
  const url = new URL(input instanceof Request ? input.url : input, window.location.href);
  const method = init?.method ?? 'GET';
  const answer = (res: Response) =>
    new Promise<Response>((done) => {
      setTimeout(() => {
        done(res);
      }, 120);
    });
  if (url.pathname === '/api/pages' && method === 'GET') return answer(json(entries));
  if (url.pathname === '/api/tasks')
    return answer(json({ shop: { '#128': { title: 'Checkout', state: 'doing', at: ago(3), rev: 1 } } }));
  if (url.pathname === '/api/themes') return answer(json({ themes: THEMES, default: '', taskMode: false }));
  if (url.pathname.endsWith('/versions')) return answer(json(VERSIONS));
  if (url.pathname.endsWith('/diff'))
    return answer(
      json({
        ...DIFF,
        from: { n: Number(url.searchParams.get('from')) || 2 },
        to: { n: Number(url.searchParams.get('to')) || 3 },
      }),
    );
  if (url.pathname.endsWith('/files')) {
    const dir = (url.searchParams.get('dir') ?? '') as keyof typeof FILES;
    return answer(dir in FILES ? json(FILES[dir]) : new Response('', { status: 404 }));
  }
  if (url.pathname === '/api/settings' || url.pathname.endsWith('/read')) return answer(new Response(''));
  if (url.pathname.endsWith('/theme')) return answer(new Response(''));
  const own = /^\/api\/pages\/([^/]+)$/.exec(url.pathname);
  const at = entries.findIndex((x) => x.slug === decodeURIComponent(own?.[1] ?? ''));
  if (own && method === 'PATCH' && at >= 0) {
    Object.assign(entries[at] ?? {}, JSON.parse(typeof init?.body === 'string' ? init.body : '{}'));
    return answer(json(entries[at]));
  }
  return answer(new Response('', { status: 404 }));
};

// Where the page was opened from, kept as the screen that opened it keeps its rows
const BACKS: Record<string, string> = {
  pages: '/?view=fav',
  tasks: '/tasks?project=shop&view=doing',
  task: '/tasks?project=shop&task=%23128',
};
const back = BACKS[params.get('back') ?? ''];
if (back) {
  const rows = ['plan', 'design', 'notes', now.framed];
  sessionStorage.setItem('pages.order', JSON.stringify({ order: rows, shown: rows, back }));
} else sessionStorage.removeItem('pages.order');

// Fits tells whether the screen is as wide as the window, nothing running off its right (the page's width / the
// window's), read again as the window changes and after the list's read
function Fits() {
  const read = () => `${String(document.documentElement.scrollWidth)} / ${String(window.innerWidth)}`;
  const [width, setWidth] = useState('');
  useEffect(() => {
    const again = () => {
      setWidth(read());
    };
    const late = setTimeout(again, 1000);
    window.addEventListener('resize', again);
    return () => {
      clearTimeout(late);
      window.removeEventListener('resize', again);
    };
  }, []);
  return (
    <output id="fits" className="rounded-small bg-on-ink px-1.5 text-ink">
      {width}
    </output>
  );
}

function Scenes() {
  const link = (s: string, b: string) =>
    `?scene=${s}${b ? `&back=${b}` : ''}${SCENES[s]?.query ? `&${SCENES[s].query}` : ''}`;
  return (
    <nav className="fixed bottom-1 left-1 z-50 flex max-w-[70vw] flex-wrap gap-1 rounded-small bg-ink/80 p-1 text-label print:hidden">
      {Object.entries(SCENES).map(([s, { words }]) => (
        <a key={s} href={link(s, params.get('back') ?? '')} className="rounded-small px-1.5 text-on-ink">
          {words}
        </a>
      ))}
      {['', 'pages', 'tasks', 'task'].map((b) => (
        <a key={b} href={link(scene, b)} className="rounded-small px-1.5 text-on-ink underline">
          {b ? `from ${b}` : 'by its URL'}
        </a>
      ))}
      <Fits />
    </nav>
  );
}

const boot = import.meta.glob<Boot>('./boot.json', { eager: true, import: 'default' })['./boot.json'];
const el = document.getElementById('root');
if (el !== null && boot) {
  setBoot({ ...boot, screen: 'frame', framed: now.framed, served: now.served ?? false, version: now.version ?? '' });
  startFrame();
  createRoot(el).render(
    <StrictMode>
      <Scenes />
      <App search={window.location.search} />
    </StrictMode>,
  );
} else if (el !== null) {
  el.textContent = 'gallery/boot.json is not there: start the gallery with just web-gallery';
}
