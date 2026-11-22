// The columns of the list screens alone, in a window of their own: their edges stand fixed on the window and their
// widths go to its root, and their ids would meet the gallery's. ?screen= picks the screen (list, taskList, taskPage).
// The columns are kept in memory, never in the browser's storage
import './gallery.css';

import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';

import { Columns, ShelfFoot, showColumn } from '@/components/columns';
import { setBoot, type Boot } from '@/config/boot';
import { ListWhat } from '@/features/page-list/components/list-head';
import { ListShelf } from '@/features/page-list/components/list-shelf';
import { useColumns } from '@/hooks/use-columns';
import { useStore } from '@/hooks/use-store';
import type { Kept } from '@/lib/storage';
import { createStore } from '@/lib/store';
import { createColumns } from '@/stores/columns';
import { createListQuery } from '@/stores/list-query';
import { createOpenPage } from '@/stores/open-page';
import type { Pages } from '@/stores/pages';
import type { Page } from '@/types/page';
import type { Screen } from '@/types/screen';

const SCREENS = ['list', 'taskList', 'taskPage'] as const;
const asked = new URLSearchParams(window.location.search).get('screen');
const screen: Screen = SCREENS.find((s) => s === asked) ?? 'list';

const ago = (h: number) => new Date(Date.now() - h * 3600000).toISOString();
const PAGES: Page[] = Array.from({ length: 60 }, (_, i) => ({
  slug: `p${i}`,
  title: `Page ${i + 1}`,
  description: '',
  createdAt: ago(i + 2),
  updatedAt: ago(i + 1),
  taskProject: '/',
}));
const pages = {
  load: () => Promise.resolve(),
  state: createStore({
    read: true,
    entries: PAGES,
    tasks: { error: '', tasks: {}, raw: '' },
    readWhy: '',
    drawWhy: '',
  }),
  pressed: () => undefined,
  released: () => undefined,
} as unknown as Pages;
const query = createListQuery({ search: () => '', replace: () => undefined, folders: { takeOpen: () => undefined } });

// What the browser would keep, here in memory: a task's page starts with its list kept folded
const held = new Map<string, string>(screen === 'taskPage' ? [['list.columns', '{"listOpen":false}']] : []);
const memory: Kept = {
  read: (key) => held.get(key) ?? null,
  write: (key, value) => held.set(key, value),
  remove: (key) => held.delete(key),
};
const openPage = createOpenPage({ boot: () => ({ screen: 'list', framed: '' }) as Boot, search: () => '' });
const columns = createColumns({ screen: () => screen, storage: memory, openPage });

function Head() {
  return (
    <ListWhat
      query={query}
      columns={columns}
      onShow={() => {
        showColumn('shelf', true, columns);
      }}
    />
  );
}

function List() {
  const { listShown } = useColumns(columns);
  const picked = useStore(openPage.picked);
  return (
    <>
      {screen === 'taskPage' ? null : (
        <div data-testid="lhead" className="flex h-lhead items-center gap-2.5 border-b border-line px-4">
          {listShown ? <Head /> : null}
          <span className="text-small text-muted">{PAGES.length} pages</span>
        </div>
      )}
      <ul id="list" className="m-0 list-none p-0">
        {PAGES.map((p) => (
          <li
            key={p.slug}
            aria-current={picked?.slug === p.slug || undefined}
            className="border-b border-line aria-current:bg-sel"
          >
            <button
              type="button"
              className="w-full cursor-pointer border-0 bg-transparent px-4 py-2.5 text-start text-fg"
              onClick={() => {
                openPage.picked.set({ slug: p.slug, file: '', v: 0 });
              }}
            >
              {p.title}
            </button>
          </li>
        ))}
      </ul>
    </>
  );
}

function Reader() {
  const { listShown } = useColumns(columns);
  const picked = useStore(openPage.picked);
  return (
    <section id="reader" className="flex min-h-0 min-w-0 flex-col mid:hidden">
      <div id="read-head" className="flex h-lhead flex-none items-center gap-2.5 border-b border-line px-4">
        {listShown || screen === 'taskPage' ? null : <Head />}
        <strong>{picked ? `Page ${Number(picked.slug.slice(1)) + 1}` : ''}</strong>
      </div>
      <p className="m-0 grid flex-1 place-content-center p-6 text-center text-muted">
        {picked
          ? 'The page beside the list. Print preview: the buttons that open a column do not print.'
          : 'Press a row: on a task’s page, the list kept folded folds, the focus on what opens it.'}
      </p>
    </section>
  );
}

function Bar() {
  return (
    <header className="flex h-bar flex-none items-center gap-3 border-b border-line bg-panel px-4 text-small">
      <strong>Columns</strong>
      {SCREENS.map((s) => (
        <a key={s} href={`?screen=${s}`} aria-current={s === screen || undefined} className="aria-current:font-bold">
          {s}
        </a>
      ))}
      {screen === 'taskPage' ? (
        <button
          type="button"
          onClick={() => {
            openPage.picked.set(null);
            columns.show('list', false);
          }}
        >
          No page, the list kept folded
        </button>
      ) : null}
      <span className="text-muted">Narrow the window under 70rem and 56rem; widen the text with Ctrl and +</span>
    </header>
  );
}

function ColumnsWindow() {
  return (
    <div className="flex h-dvh flex-col">
      <Bar />
      <Columns
        columns={columns}
        shelf={
          screen === 'taskPage' ? undefined : (
            <ListShelf
              onAdd={() => undefined}
              foot={<ShelfFoot columns={columns} />}
              query={query}
              pages={pages}
              columns={columns}
            />
          )
        }
        list={<List />}
        reader={screen === 'taskList' ? undefined : <Reader />}
      />
    </div>
  );
}

const el = document.getElementById('root');
const boot = import.meta.glob<Boot>('./boot.json', { eager: true, import: 'default' })['./boot.json'];
if (el !== null && boot) {
  setBoot(boot);
  createRoot(el).render(
    <StrictMode>
      <ColumnsWindow />
    </StrictMode>,
  );
} else if (el !== null) {
  el.textContent = 'gallery/boot.json is not there: start the gallery with just web-gallery';
}
