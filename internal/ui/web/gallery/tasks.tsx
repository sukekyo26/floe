// /tasks as the app draws it (App): the bar, the task list or a task's page, and the modals they open, for real. A fake
// server answers the pages, the tasks and the themes, and takes the writes of the screen (a task made or changed, a pin
// taken off, a task removed) in memory; the browser's storage is the gallery's own origin's. ?data= picks the data
// (full, empty, failed, flaky: the tasks unreadable after the first read, pins, nopins, throw: two reads the screen
// cannot draw), and ?project=, ?view= and ?task= the screen as /tasks takes them; ?scene=suggest shows a field's names
// alone, ?scene=not-link a URL of ftp typed in the task edit's add modal; ?stale=1 refuses the first write of a task's
// rev as written meanwhile (409), as another window would
import './gallery.css';

import { StrictMode, useState } from 'react';
import { createRoot } from 'react-dom/client';

import { App } from '@/app/app';
import { useNameSuggest } from '@/components/name-suggest';
import { Toasts } from '@/components/toasts';
import { setBoot, type Boot } from '@/config/boot';
import type { Page } from '@/types/page';
import type { TaskRow, Tasks } from '@/types/task';
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
  page('b1', 'Build notes', 'shop', { task: { name: '#12', category: 'build' }, tags: ['plan', 'api'] }),
  page('b0', 'Build, first try', 'shop', { task: { name: '#12', category: 'build' }, archived: true }),
  page('fold', 'Mockups', 'shop', { task: { name: '#12' }, folder: true, path: '/home/me/mockups' }),
  page('d2', 'Review', 'shop', { task: { name: '#13', category: 'review' } }),
  page('d3', 'Old', 'shop', { task: { name: '#13' }, archived: true }),
  page('f1', 'Floe page', 'floe', { task: { name: 'S1' } }),
  page('l1', 'Long project page', LONG, { task: { name: '#1' } }),
  page('solo-pin', 'Alone', 'solo', { task: { pin: true } }),
  page('free', 'A page of no task', ''),
  page('p18', 'Plan', 'shop', { task: { name: '#18', category: 'plan' } }),
  page('n19a', 'Notes', 'shop', { task: { name: '#19' } }),
  page('n19b', 'Sketch', 'shop', { task: { name: '#19' } }),
  page('o22', 'Old copy', 'shop', { task: { name: '#22' }, archived: true }),
];
const TASKS: Tasks = {
  shop: {
    '#12': {
      title: 'Checkout redesign',
      state: 'doing',
      at: ago(1),
      urls: ['https://github.com/acme/shop/pull/12', 'https://tracker.example/T-12'],
      categories: ['design', 'build', 'later'],
      rev: 3,
    },
    '#13': { title: 'Payment review', state: 'doing', at: ago(5), parent: '#12', categories: ['review'], rev: 1 },
    '#14': { title: 'Search box', state: 'todo', at: ago(30), rev: 1 },
    '#10': { title: 'Cart', state: 'done', at: ago(90), rev: 2 },
    '#11': {
      title: 'Copy for the payment page, a title long enough to be cut',
      state: 'todo',
      at: ago(8),
      parent: '#12',
      rev: 1,
    },
    '#15': { title: 'An idea with no page yet', state: 'todo', at: ago(20), rev: 1 },
    '#16': { title: 'A parent of no page', state: 'doing', at: ago(6), rev: 1 },
    '#17': { title: 'Its child', state: 'done', at: ago(7), parent: '#16', rev: 1 },
    '#9': { title: 'Old banner', state: 'archived', at: ago(400), rev: 1 },
    '#22': { title: 'Old copy, archived', state: 'archived', at: ago(300), rev: 1 },
    '#18': { title: 'All in categories', state: 'doing', at: ago(2), categories: ['plan', 'test'], rev: 1 },
    '#19': { title: 'No categories', state: 'doing', at: ago(2), rev: 1 },
    '#20': { title: 'Categories, no page', state: 'todo', at: ago(2), categories: ['plan'], rev: 1 },
    '#21': {
      title: 'Ten URLs',
      state: 'doing',
      at: ago(2),
      urls: Array.from({ length: 10 }, (_, i) => `https://example.com/${String(i + 1)}`),
      rev: 1,
    },
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

let tasksRead = 0;
let pagesRead = 0;
const THEMES = [
  { name: 'default', about: 'Plain, for any page', dir: '' },
  { name: 'paper', about: 'Warm paper, serif headings', dir: '' },
];
// ?stale=1: the first write of a task's rev is refused, as written meanwhile
let stale = params.get('stale') === '1';
const refuseOnce = () => {
  const now = stale;
  stale = false;
  return now;
};
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
  // ?data=throw: the first two reads hold an entry no screen can draw, as a broken answer would; the next draws again
  if (url.pathname === '/api/pages')
    return answer(json(data === 'throw' && ++pagesRead <= 2 ? [...entries, null] : entries));
  // The settings' themes and a page's own, written in nothing
  if (url.pathname === '/api/themes') return answer(json({ themes: THEMES, default: '', taskMode: false }));
  if (url.pathname === '/api/settings' || url.pathname.endsWith('/read')) return answer(new Response(''));
  if (url.pathname.endsWith('/theme')) return answer(method === 'GET' ? json({ own: '' }) : new Response(''));
  // A page's star, archive and the rest of its own fields, as PATCH /api/pages/{slug} writes them
  const own = /^\/api\/pages\/([^/]+)$/.exec(url.pathname);
  if (own && method === 'PATCH') {
    const a = entries.find((x) => x.slug === decodeURIComponent(own[1] ?? ''));
    if (a) Object.assign(a, JSON.parse(typeof init?.body === 'string' ? init.body : '{}') as Partial<Page>);
    return answer(a ? json(a) : new Response('', { status: 404 }));
  }
  if (url.pathname === '/api/tasks' && method === 'GET') {
    const down = data === 'failed' || (data === 'flaky' && ++tasksRead > 1);
    return answer(down ? new Response('tasks.json: unknown key "colour"', { status: 500 }) : json(tasks));
  }
  if (url.pathname === '/api/tasks' && method === 'DELETE') {
    const project = url.searchParams.get('taskProject') ?? '';
    const name = url.searchParams.get('task') ?? '';
    const { [name]: _gone, ...rest } = tasks[project] ?? {};
    tasks[project] = rest;
    return answer(new Response(''));
  }
  if (url.pathname === '/api/tasks' && method === 'PATCH') {
    const body = JSON.parse(typeof init?.body === 'string' ? init.body : '{}') as {
      taskProject?: string;
      task?: string;
      new?: boolean;
      rev?: number;
      title?: string;
      state?: string;
      parent?: string;
      parentNew?: boolean;
      parentTitle?: string;
      slugs?: string[];
      pin?: boolean;
      category?: string;
      urls?: { add?: string[]; remove?: string[]; order?: string[] };
      categories?: string[];
    };
    if (body.slugs && body.task === undefined) {
      for (const a of entries) if (body.slugs.includes(a.slug)) a.task = { ...a.task, pin: Boolean(body.pin) };
      return answer(json({ tasks: [], pages: [] }));
    }
    const put = body.slugs ? entries.filter((a) => body.slugs?.includes(a.slug)) : [];
    const project = body.slugs ? (put[0]?.taskProject ?? '') : (body.taskProject ?? '');
    const name = (body.task ?? '').trim();
    const rows = { ...tasks[project] };
    const was = rows[name];
    // Off its task: the page alone
    if (body.slugs && !name) {
      for (const a of put) a.task = { ...(a.task?.pin ? { pin: true } : {}) };
      return answer(json({ tasks: [], pages: put }));
    }
    if (body.new && was) return answer(new Response(`task ${name} is there already`, { status: 409 }));
    if (!body.new && (was?.rev !== body.rev || refuseOnce())) {
      return answer(
        new Response(was ? `task ${name} has changed since it was read` : `task ${name} is no longer there`, {
          status: 409,
        }),
      );
    }
    const at = new Date().toISOString();
    const written: { taskProject: string; task: string; row: TaskRow }[] = [];
    if (body.parentNew && body.parent) {
      const parent: TaskRow = { title: body.parentTitle ?? '', state: 'todo', at, rev: 1 };
      rows[body.parent] = parent;
      written.push({ taskProject: project, task: body.parent, row: parent });
    }
    const row: TaskRow = { state: 'todo', ...was, at, rev: (was?.rev ?? 0) + 1 };
    if (body.title !== undefined) row.title = body.title;
    if (body.state !== undefined) row.state = body.state;
    if (body.parent !== undefined) {
      if (body.parent) row.parent = body.parent;
      else delete row.parent;
    }
    if (body.urls) {
      const urls = (row.urls ?? []).filter((u) => !body.urls?.remove?.includes(u)).concat(body.urls.add ?? []);
      const order = body.urls.order ?? [];
      row.urls = [...order.filter((u) => urls.includes(u)), ...urls.filter((u) => !order.includes(u))];
    }
    if (body.categories) row.categories = body.categories;
    // A page put on a category of the task: the category added where it is not there, the page off the task it was on
    const offs = put.map((a) => {
      const off =
        a.task?.name && a.task.name !== name ? { offTask: a.task.name, offCategory: a.task.category ?? '' } : {};
      a.task = { ...(a.task?.pin ? { pin: true } : {}), name, ...(body.category ? { category: body.category } : {}) };
      if (body.category && !(row.categories ?? []).includes(body.category))
        row.categories = [...(row.categories ?? []), body.category];
      return { ...a, ...off };
    });
    rows[name] = row;
    tasks[project] = rows;
    written.push({ taskProject: project, task: name, row });
    return answer(json({ tasks: written, pages: offs }));
  }
  return answer(new Response('', { status: 404 }));
};

const SCENES = [
  ['?data=full&project=shop', 'shop'],
  ['?data=full&project=shop&view=todo', 'shop todo'],
  ['?data=full&project=floe&view=todo', 'none of the view'],
  ['?data=full&project=solo-gone', 'a project gone'],
  ['?data=full&project=' + LONG, 'long project'],
  ['?data=nopins&project=shop', 'no pins'],
  ['?data=pins&project=solo', 'pins alone'],
  ['?data=empty', 'nothing'],
  ['?data=failed', 'unreadable'],
  ['?data=full&project=shop&task=%2312', 'task'],
  ['?data=full&project=shop&task=%2313', 'a child'],
  ['?data=full&project=shop&task=%2310', 'done'],
  ['?data=full&project=shop&task=%239', 'archived'],
  ['?data=full&project=shop&task=%2315', 'no page'],
  ['?data=full&project=shop&task=%2318', 'none on no category'],
  ['?data=full&project=shop&task=%2319', 'no categories'],
  ['?data=full&project=shop&task=%2320', 'categories, no page'],
  ['?data=full&project=shop&task=%2322', 'archived, a page'],
  ['?data=full&project=shop&task=%2321', 'ten URLs'],
  ['?data=full&project=shop&task=%2312&scene=not-link', 'not an http URL'],
  ['?data=full&project=shop&task=%2316', 'no page, children'],
  ['?data=full&project=shop&task=%2399', 'missing'],
  ['?data=flaky&project=shop&task=%2312', 'unreadable after'],
  ['?data=flaky&project=shop&task=%2313', 'unreadable, a child'],
  ['?data=failed&project=shop&task=%2312', 'never read'],
  ['?data=full&project=shop&task=%2312&stale=1', 'task, a write refused once'],
  ['?data=throw&project=shop&task=%2312', 'a draw that throws'],
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

// typeInAdd opens the task's edit once its rows are drawn, then its URL's add modal, and adds what it types there, as a
// person would, a step a frame: the field's value is set by the input's own setter, which React hears
async function typeInAdd(typed: string) {
  const frame = () =>
    new Promise((done) => {
      setTimeout(done, 200);
    });
  const press = (id: string) => {
    document.getElementById(id)?.click();
  };
  while (!document.querySelector('#list > li')) await frame();
  press('task-edit');
  await frame();
  press('edit-url-plus');
  await frame();
  const field = document.getElementById('edit-add-new');
  if (!(field instanceof HTMLInputElement)) return;
  Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, 'value')?.set?.call(field, typed);
  field.dispatchEvent(new Event('input', { bubbles: true }));
  await frame();
  press('edit-add-submit');
}

const boot = import.meta.glob<Boot>('./boot.json', { eager: true, import: 'default' })['./boot.json'];
const el = document.getElementById('root');
if (el !== null && boot) {
  setBoot({ ...boot, screen: 'tasks' });
  createRoot(el).render(
    <StrictMode>
      <Scenes />
      {params.get('scene') === 'suggest' ? (
        <>
          <Suggest />
          <Toasts />
        </>
      ) : (
        <App search={window.location.search} />
      )}
    </StrictMode>,
  );
  if (params.get('scene') === 'not-link') void typeInAdd('ftp://example.com/files');
} else if (el !== null) {
  el.textContent = 'gallery/boot.json is not there: start the gallery with just web-gallery';
}
