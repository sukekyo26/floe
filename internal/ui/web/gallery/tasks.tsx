// The task list alone, in a window of its own, as /tasks will be: its columns stand fixed on the window. A fake server
// answers the pages and the tasks, and takes the writes of the screen (a task made, a pin taken off) in memory; the
// browser's storage is the gallery's own origin's. ?data= picks the data (full, empty, failed, pins, nopins), and
// ?project= and ?view= the list as /tasks takes them; ?scene=suggest shows a field's names alone
import './gallery.css';

import { StrictMode, useState } from 'react';
import { createRoot } from 'react-dom/client';

import { useNameSuggest } from '@/components/name-suggest';
import { Toasts } from '@/components/toasts';
import { setBoot, type Boot } from '@/config/boot';
import { ReaderPane } from '@/features/reader/components/reader-pane';
import { TaskListScreen } from '@/features/tasks/components/task-list-screen';
import { pages } from '@/stores/pages';
import type { Page } from '@/types/page';
import type { Tasks } from '@/types/task';
import { nameOffers } from '@/utils/name-offers';

const params = new URLSearchParams(window.location.search);
const data = params.get('data') ?? 'full';
const ago = (h: number) => new Date(Date.now() - h * 3600000).toISOString();
const LONG = 'a-project-of-a-very-long-name';

const page = (slug: string, title: string, project: string, over: Partial<Page> = {}): Page => ({
  slug,
  title,
  description: '',
  project,
  taskProject: project || '/',
  createdAt: ago(48),
  updatedAt: ago(2),
  ...over,
});

// The pages: pins of shop (one unread, one of a long title), pages on tasks, and some of other projects
const PAGES: Page[] = [
  page('pin-a', 'Release checklist', 'shop', { task: { name: '#12', pin: true }, news: 'page' }),
  page('pin-b', 'A design page whose title is long enough to be cut on a narrow screen', 'shop', {
    task: { name: '#12', pin: true },
  }),
  page('pin-c', 'Notes', 'shop', { task: { pin: true } }),
  page('d1', 'Design', 'shop', { task: { name: '#12', category: 'design' }, news: 'version', version: 2 }),
  page('d2', 'Review', 'shop', { task: { name: '#13', category: 'review' } }),
  page('d3', 'Old', 'shop', { task: { name: '#13' }, archived: true }),
  page('f1', 'Floe page', 'floe', { task: { name: 'S1' } }),
  page('l1', 'Long project page', LONG, { task: { name: '#1' } }),
  page('solo-pin', 'Alone', 'solo', { task: { pin: true } }),
  page('free', 'A page of no task', ''),
];
const TASKS: Tasks = {
  shop: {
    '#12': {
      title: 'Checkout redesign',
      state: 'doing',
      at: ago(1),
      urls: ['https://github.com/acme/shop/pull/12', 'https://tracker.example/T-12'],
      categories: ['design'],
      rev: 3,
    },
    '#13': { title: 'Payment review', state: 'doing', at: ago(5), parent: '#12', categories: ['review'], rev: 1 },
    '#14': { title: 'Search box', state: 'todo', at: ago(30), rev: 1 },
    '#10': { title: 'Cart', state: 'done', at: ago(90), rev: 2 },
    '#9': { title: 'Old banner', state: 'archived', at: ago(400), rev: 1 },
  },
  floe: { S1: { title: 'Task screens', state: 'doing', at: ago(3), rev: 1 } },
  [LONG]: { '#1': { title: 'Narrow', state: 'todo', at: ago(10), rev: 1 } },
};

const entries: Page[] =
  data === 'empty'
    ? PAGES.filter((a) => !a.task).concat(page('e2', 'Another page', 'floe'))
    : data === 'pins'
      ? PAGES.filter((a) => a.slug === 'solo-pin')
      : data === 'nopins'
        ? PAGES.map((a) => (a.task?.pin ? { ...a, task: { ...a.task, pin: false } } : a))
        : PAGES;
const tasks: Tasks = data === 'empty' || data === 'pins' ? {} : structuredClone(TASKS);

const json = (body: unknown, status = 200) => new Response(JSON.stringify(body), { status });

// The fake server: what the screen reads and writes
window.fetch = (input: string | URL | Request, init?: RequestInit) => {
  const url = new URL(input instanceof Request ? input.url : input, window.location.href);
  const method = init?.method ?? 'GET';
  const answer = (res: Response) =>
    new Promise<Response>((done) => {
      setTimeout(() => {
        done(res);
      }, 120);
    });
  if (url.pathname === '/api/pages') return answer(json(entries));
  if (url.pathname === '/api/tasks' && method === 'GET')
    return answer(data === 'failed' ? new Response('tasks.json: unknown key "colour"', { status: 500 }) : json(tasks));
  if (url.pathname === '/api/tasks' && method === 'PATCH') {
    const body = JSON.parse(typeof init?.body === 'string' ? init.body : '{}') as {
      taskProject?: string;
      task?: string;
      title?: string;
      slugs?: string[];
      pin?: boolean;
    };
    if (body.slugs) {
      for (const a of entries) if (body.slugs.includes(a.slug)) a.task = { ...a.task, pin: Boolean(body.pin) };
      return answer(json({ tasks: [], pages: [] }));
    }
    const project = body.taskProject ?? '';
    const name = body.task ?? '';
    const row = { title: body.title ?? '', state: 'todo', at: new Date().toISOString(), rev: 1 };
    tasks[project] = { ...tasks[project], [name]: row };
    return answer(json({ tasks: [{ taskProject: project, task: name, row }], pages: [] }));
  }
  return answer(new Response('', { status: 404 }));
};

const SCENES = [
  ['?data=full&project=shop', 'shop'],
  ['?data=full&project=shop&view=todo', 'shop todo'],
  ['?data=full&project=solo-gone', 'a project gone'],
  ['?data=full&project=' + LONG, 'long project'],
  ['?data=nopins&project=shop', 'no pins'],
  ['?data=pins&project=solo', 'pins alone'],
  ['?data=empty', 'nothing'],
  ['?data=failed', 'unreadable'],
  ['?scene=suggest', 'names'],
] as const;

// The scenes, over all: a gallery's, not the screen's
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

// A field's names alone, as the modal that sets a task will offer them (P6c): ↓ opens them, Enter puts one in
function Suggest() {
  const [value, setValue] = useState('');
  const names = ['#12', '#120', '#13', 'S1', 'review'];
  const suggest = useNameSuggest({
    id: 'g-task',
    offers: (written, typed) =>
      nameOffers(names, written, typed, {
        detail: (name) => ({ name, label: `${name} A title`, note: '(doing)' }),
        fresh: (name) => ({ name, label: `New task ${name}` }),
      }),
    put: setValue,
  });
  return (
    <div className="grid max-w-sm gap-2 p-8">
      <label className="grid gap-1.5 text-small font-semibold">
        A task
        <input
          id="g-task"
          className="h-9 rounded-small border border-line bg-panel px-3 font-normal"
          value={value}
          {...suggest.field}
          onChange={(e) => {
            setValue(e.currentTarget.value);
          }}
        />
      </label>
      {suggest.box}
    </div>
  );
}

const boot = import.meta.glob<Boot>('./boot.json', { eager: true, import: 'default' })['./boot.json'];
const el = document.getElementById('root');
if (el !== null && boot) {
  setBoot({ ...boot, screen: 'tasks' });
  if (params.get('scene') !== 'suggest') {
    void pages.load();
    pages.watch();
  }
  createRoot(el).render(
    <StrictMode>
      <Scenes />
      {params.get('scene') === 'suggest' ? (
        <Suggest />
      ) : (
        <div className="flex h-dvh flex-col">
          <TaskListScreen reader={<ReaderPane at={null} page={null} keys={new Set()} onKey={() => undefined} />} />
        </div>
      )}
      <Toasts />
    </StrictMode>,
  );
} else if (el !== null) {
  el.textContent = 'gallery/boot.json is not there: start the gallery with just web-gallery';
}
