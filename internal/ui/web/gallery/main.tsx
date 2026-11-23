// The gallery: the shared components in their states, for a person to try them before any screen is switched. It is
// served by Vite's dev server alone (just web-gallery), never built into dist
import './gallery.css';

import { Profiler, StrictMode, useRef, useState, type ReactNode } from 'react';
import { createRoot } from 'react-dom/client';

import { BackLink } from '@/components/back-link';
import { Button } from '@/components/button';
import { Dialog, DialogActions, DialogClose, DialogHead } from '@/components/dialog';
import { ErrorBar } from '@/components/error-bar';
import { Grip } from '@/components/grip';
import { IconButton } from '@/components/icon-button';
import { ArchiveIcon, StarIcon } from '@/components/icons';
import { BarActions, RowActions } from '@/components/page-actions';
import type { Openers } from '@/components/page-menu';
import { PageRow } from '@/components/page-row';
import { Rows, type RowItem } from '@/components/rows';
import { Tab, TabList } from '@/components/tabs';
import { ServerState } from '@/components/server-state';
import { Toasts } from '@/components/toasts';
import { setBoot, type Boot } from '@/config/boot';
import { KeysDialog, type HelpTab } from '@/features/help/components/keys-dialog';
import { EditDialog, type EditTarget } from '@/features/page-edit/components/edit-dialog';
import { createSaveEdit } from '@/features/page-edit/stores/save-edit';
import { pageTags, TagDialog, type TagTarget } from '@/features/tags/components/tag-dialog';
import { TagLine } from '@/features/tags/components/tag-line';
import { createApplyTags } from '@/features/tags/stores/apply-tags';
import { FindDialog } from '@/features/find/components/find-dialog';
import type { ThemesRead } from '@/features/settings/api/themes';
import { SettingsDialog, type SettingsTarget } from '@/features/settings/components/settings-dialog';
import { createThemeWrites } from '@/features/settings/stores/choose-theme';
import { createSettings } from '@/features/settings/stores/settings';
import { useHold } from '@/hooks/use-hold';
import { useKeys } from '@/hooks/use-keys';
import { useRowKeys } from '@/hooks/use-row-keys';
import { useOpenMenus } from '@/hooks/use-open-menu';
import { useStore } from '@/hooks/use-store';
import { client } from '@/lib/api-client';
import { createStore } from '@/lib/store';
import { t } from '@/lib/t';
import { isMac } from '@/lib/platform';
import { createActOn } from '@/stores/act-on';
import { notice } from '@/stores/notice';
import type { OpenPage } from '@/stores/open-page';
import { order as pageOrder, type Order } from '@/stores/order';
import { createPageActions } from '@/stores/page-actions';
import type { PageWrites } from '@/stores/page-writes';
import type { Pages } from '@/stores/pages';
import { createOpenPage } from '@/stores/open-page';
import { createPageNote } from '@/stores/page-note';
import { createReads } from '@/stores/reads';
import type { Client } from '@/lib/api-client';
import { ReaderPane } from '@/features/reader/components/reader-pane';
import { createPageReads } from '@/features/reader/stores/page-reads';
import type { TaskWrites } from '@/stores/tasks';
import { toasts } from '@/stores/toasts';
import { createColumns } from '@/stores/columns';
import { backToRow, focusOn, moveRow, tickRow } from '@/components/focus-places';
import { createListQuery } from '@/stores/list-query';
import { ListCount, ListWhat } from '@/features/page-list/components/list-head';
import { ListShelf } from '@/features/page-list/components/list-shelf';
import { News } from '@/features/page-list/components/news';
import { AddDialog } from '@/features/page-list/components/add-dialog';
import { BulkBar } from '@/features/page-list/components/bulk-bar';
import { PageList } from '@/features/page-list/components/page-list';
import { SearchBox } from '@/features/page-list/components/search-box';
import type { Listing, Page } from '@/types/page';
import type { Screen } from '@/types/screen';
import type { Folders } from '@/stores/open-folders';
import { backOf } from '@/utils/back';
import { openKey } from '@/utils/paths';
import { startVersions, VersionsSection } from './versions';

// The boot values go test -gallery wrote (just web-gallery); none when it has not run
const found = import.meta.glob<Boot>('./boot.json', { eager: true, import: 'default' });

const ago = (minutes: number) => new Date(Date.now() - minutes * 60000).toISOString();
const PAGES: Page[] = [
  {
    slug: 'plain',
    title: 'A page read',
    description: 'A row as it is most of the time',
    project: 'floe',
    createdAt: ago(90),
    updatedAt: ago(32),
    taskProject: 'floe',
    template: 'design',
    tags: ['design', 'floe'],
    task: { name: '#12', category: 'designs' },
    themed: true,
    version: 3,
    versionCount: 3,
  },
  {
    slug: 'unread',
    title: 'A page with a version not opened yet',
    description: '',
    project: 'floe',
    createdAt: ago(90),
    updatedAt: ago(1),
    taskProject: 'floe',
    tags: ['floe'],
    news: 'version',
  },
  {
    slug: 'picked',
    title: 'The page open beside the list',
    description: '',
    project: 'demo',
    createdAt: ago(900),
    updatedAt: ago(200),
    taskProject: 'demo',
    tags: ['Foo', 'Foobar'],
    favorite: true,
  },
  {
    slug: 'gone',
    title: 'A link whose file is gone',
    description: '',
    path: '/tmp/gone.html',
    createdAt: ago(900),
    updatedAt: ago(500),
    taskProject: '/',
    missing: true,
  },
  {
    slug: 'folder',
    title: 'A folder',
    description: '',
    path: '/tmp/site',
    folder: true,
    createdAt: '',
    // When it was linked, as the server gives a folder: its row shows no time
    updatedAt: ago(600),
    taskProject: '/',
  },
  {
    slug: 'archived',
    title: 'An archived page, dimmed on a task’s page',
    description: '',
    createdAt: ago(9000),
    updatedAt: ago(9000),
    taskProject: '/',
    archived: true,
  },
];

// The pages as the gallery's fake server keeps them: what the actions write changes them, as a read would show
const shownPages = createStore<Page[]>(PAGES);
const edit = (slug: string, change: Partial<Page>) => {
  shownPages.set(shownPages.get().map((p) => (p.slug === slug ? { ...p, ...change } : p)));
};
// The list as a read of the fake server would give it: the pages as they are now
const listed = createStore({
  read: true,
  entries: PAGES,
  tasks: { error: '', tasks: {}, raw: '' },
  readWhy: '',
  drawWhy: '',
});
shownPages.subscribe(() => {
  listed.set({ ...listed.get(), entries: shownPages.get() });
});
// slow: the list's read after a write takes 1.5 s, to see the toast and its focus come before it
let slow = false;
// refuse: the next tag or edit writes are refused, as a server that says no would
let refuse = false;
const galleryPages = {
  load: () =>
    new Promise((go) => {
      setTimeout(go, slow ? 1500 : 0);
    }),
  rereadTasks: () => Promise.resolve(),
  readRev: () => 0,
  whileReading: () => null,
  state: listed,
  pressed: () => undefined,
  released: () => undefined,
} as unknown as Pages;
// The shelf's picks and the columns, as the page list keeps them, but in no URL
const galleryQuery = createListQuery({
  search: () => '',
  replace: () => undefined,
  folders: { takeOpen: () => undefined },
});
const galleryColumns = createColumns({ screen: () => 'list' });
// A tag write adds and removes; any other is the fields it names
const patched = (a: Page, body: unknown): Partial<Page> => {
  const { tags } = body as { tags?: { add: string[]; remove: string[] } };
  return tags
    ? { tags: [...new Set([...(a.tags ?? []), ...tags.add])].filter((x) => !tags.remove.includes(x)) }
    : (body as Partial<Page>);
};
const answered = { ok: true, status: 200, text: '', rev: 1, stale: '', pages: [], written: [] };
// The page beside the list: its marks go to a fake server, whose answers can be slowed, and the words of its versions
// come from fake /versions and /diff
let readsSlow = false;
const readServer = {
  api: (url: string, init?: RequestInit) =>
    new Promise<Response>((go) => {
      setTimeout(
        () => {
          if (url.endsWith('/versions')) go(new Response(JSON.stringify({ read: 1 })));
          else if (url.includes('/diff'))
            go(new Response(JSON.stringify({ counts: { changed: 2, added: 1, removed: 0 } })));
          else go(init?.method === 'DELETE' ? new Response(null, { status: 204 }) : new Response('{"cleared":true}'));
        },
        readsSlow ? 1500 : 50,
      );
    }),
} as unknown as Client;
const galleryOpen = createOpenPage({ boot: () => ({ screen: 'list', framed: '' }) as Boot, search: () => '' });
const galleryNote = createPageNote({ client: readServer });
const galleryReads = createReads({ client: readServer, pages: galleryPages, openPage: galleryOpen });
const actions = createPageActions({
  writes: {
    writePage: () => Promise.resolve(''),
    patchPage: (slug, body) => {
      const a = shownPages.get().find((p) => p.slug === slug);
      if (refuse) return Promise.resolve('refused (500)');
      if (a) edit(slug, patched(a, body));
      return Promise.resolve('');
    },
    deletePage: (slug) => {
      shownPages.set(shownPages.get().filter((p) => p.slug !== slug));
      return Promise.resolve('');
    },
  },
  tasks: {
    changePageTask: (slug: string, body: { pin?: boolean; task?: string; category?: string }) => {
      const a = shownPages.get().find((p) => p.slug === slug);
      if ('pin' in body) edit(slug, { task: { ...a?.task, pin: body.pin ?? false } });
      else edit(slug, { task: {} });
      return Promise.resolve('');
    },
    putPageOn: (page: Page, task: string, _body: unknown, category: string) => {
      edit(page.slug, { task: { name: task, category } });
      return Promise.resolve(answered);
    },
  } as unknown as TaskWrites,
  pages: galleryPages,
  reads: galleryReads,
  openPage: galleryOpen,
  note: galleryNote,
  go: (href) => {
    toasts.toast(`would go to ${href}`);
  },
});
// The modals the menus open, each held by the gallery as the app will hold it
const galleryPageReads = createPageReads({
  reads: galleryReads,
  note: galleryNote,
  pages: galleryPages,
  openPage: galleryOpen,
  actions,
});
const tagTarget = createStore<TagTarget | null>(null);
const settingsTarget = createStore<SettingsTarget | null>(null);
// themesAs is what the fake /api/themes answers: the themes, one's own things broken, settings.json unreadable, or no
// answer at all
let themesAs: 'themes' | 'broken' | 'settings' | 'fails' = 'themes';
const defaults = { theme: '', taskMode: false };
const THEMES = [
  { name: 'default', about: 'Plain, for any page', dir: '' },
  { name: 'paper', about: 'Warm paper, serif headings', dir: '' },
  { name: 'mine', about: 'One of my own', dir: '~/.config/floe/themes/mine' },
];
const gallerySettings = createSettings({
  readThemes: () =>
    new Promise<ThemesRead>((go, fail) => {
      setTimeout(() => {
        if (themesAs === 'fails') fail(new Error('connection refused'));
        else
          go({
            themes: THEMES,
            default: defaults.theme,
            taskMode: defaults.taskMode,
            ...(themesAs === 'settings' ? { settings: { errors: [] } } : {}),
            ...(themesAs === 'broken'
              ? {
                  drawing: { mistakes: [{ file: 'parts/card/part.html', line: 3, column: 7, text: 'unexpected "}"' }] },
                }
              : {}),
          });
      }, 300);
    }),
  readPageTheme: () => Promise.resolve('default'),
});
// A write of the settings takes 0.8 s, to see the modal held
const later = <T,>(value: () => T) =>
  new Promise<T>((go) => {
    setTimeout(() => {
      go(value());
    }, 800);
  });
const galleryThemeWrites = createThemeWrites({
  putSettings: (body) =>
    later(() => {
      if (refuse) return 'refused (500)';
      if ('theme' in body) defaults.theme = body.theme;
      else defaults.taskMode = body.taskMode;
      return '';
    }),
  putPageTheme: (slug, theme) =>
    later(() => {
      if (refuse) return 'refused (409)';
      edit(slug, { pickedTheme: theme });
      return '';
    }),
  pages: galleryPages,
});
const editTarget = createStore<EditTarget | null>(null);
const openers: Openers = {
  onTags: (a, back) => {
    tagTarget.set(pageTags(a, back));
  },
  onTheme: (a, back) => {
    settingsTarget.set({ page: a, back });
  },
  onEdit: (a, back) => {
    editTarget.set({ page: a, back });
  },
  onTaskSet: (a) => toasts.toast(`set the task of ${a.slug}`),
};
const applyTags = createApplyTags({
  pages: galleryPages,
  writes: {
    patchPage: (slug: string, body: unknown) => {
      const a = shownPages.get().find((p) => p.slug === slug);
      if (refuse) return Promise.resolve('refused (500)');
      if (a) edit(slug, patched(a, body));
      return Promise.resolve('');
    },
  } as unknown as PageWrites,
  actOn: createActOn({ pages: galleryPages }),
});
// whole: the edit is of the page open whole, which goes to its new name
let whole = false;
const saveEdit = createSaveEdit({
  writes: {
    writePage: (slug: string, init: RequestInit, options?: { answer?: (got: unknown) => void }) => {
      const change = JSON.parse(init.body as string) as Partial<Page>;
      const a = shownPages.get().find((p) => p.slug === slug);
      if (refuse) return Promise.resolve('refused (409)');
      if (a) edit(slug, change);
      options?.answer?.({ offTask: change.project !== undefined && a?.task?.name ? a.task.name : '' });
      return Promise.resolve('');
    },
  } as unknown as PageWrites,
  pages: galleryPages,
  openPage: {
    frameAt: () => (whole ? { slug: editTarget.get()?.page.slug ?? '', file: '', v: 0 } : null),
    picked: createStore(null),
  } as unknown as OpenPage,
  order: { renameKept: () => undefined } as unknown as Order,
  go: (href) => {
    toasts.toast(`would go to ${href}`);
  },
});

// The folder's tree, as the fake server reads it: a directory cut, one empty, one unreadable
const TREE = new Map<string, Listing | Error>([
  ['folder', { dirs: ['docs', 'empty', 'locked'], files: [{ file: 'index.html', updatedAt: ago(5) }], more: 0 }],
  [
    'folder:docs',
    {
      dirs: [],
      files: [
        { file: 'docs/a.html', updatedAt: ago(30) },
        { file: 'docs/b.html', updatedAt: ago(90) },
      ],
      more: 3,
    },
  ],
  ['folder:empty', { dirs: [], files: [], more: 0 }],
  ['folder:locked', new Error('permission denied')],
]);
const galleryTree = createStore<{ opened: ReadonlySet<string>; listings: ReadonlyMap<string, Listing | Error> }>({
  opened: new Set(),
  listings: new Map(),
});
// Opens or closes a directory at once; closed, what is open under it closes too
const galleryFolders = {
  list: galleryTree,
  toggle: (slug: string, dir: string) => {
    const key = openKey(slug, dir);
    const { opened, listings } = galleryTree.get();
    const under = dir ? `${key}/` : `${slug}:`;
    const next = opened.has(key)
      ? new Set([...opened].filter((k) => k !== key && !k.startsWith(under)))
      : new Set([...opened, key]);
    const read = TREE.get(key);
    galleryTree.set({ opened: next, listings: read ? new Map([...listings, [key, read]]) : listings });
    return Promise.resolve();
  },
} as unknown as Folders;
// Many rows over a year, to measure a redraw of the list
const MANY: Page[] = Array.from({ length: 1000 }, (_, i) => ({
  slug: `many-${String(i)}`,
  title: `Page ${String(i)} of many`,
  description: '',
  project: ['floe', 'demo', 'site'][i % 3] ?? 'floe',
  createdAt: '',
  updatedAt: ago(i * 500),
  taskProject: 'floe',
  tags: i % 4 === 0 ? ['many'] : [],
}));

// ListScene is the page list as the list screen will draw it, with the time of its last commit: with 1000 rows, pick a
// row or wait a minute for the times, to measure what React draws anew
// The keys on the page list's scene: ↑ ↓ and j k go through #list, which the Rows section leaves out meanwhile, as
// the page has one list
const listKeysOn = createStore(false);
const NO_KEYS = {};

// comeBackTo does as a page opened whole from the row at and come back by its back button would: the row kept, then
// what the list does as it is drawn again (backToRow)
function comeBackTo(at: { slug: string; file: string }) {
  pageOrder.writeKept({
    order: [],
    shown: [],
    back: window.location.pathname + window.location.search,
    at: { ...at, v: 0 },
    pressed: true,
  });
  pageOrder.comeBack(false);
  backToRow({ pages: galleryPages, openPage: galleryOpen });
}

function ListScene() {
  const keysOn = useStore(listKeysOn);
  // ↑ ↓ and j k through the list's rows, as the app's table of keys will (P8)
  const keys = useRowKeys({ pages: galleryPages, openPage: galleryOpen, move: moveRow, tick: tickRow });
  useKeys(keysOn ? keys : NO_KEYS);
  const [folded, setFolded] = useState(false);
  const [came, setCame] = useState('');
  const [adding, setAdding] = useState(false);
  // The list's width: narrow phones', and a wide column, out past the gallery's narrow column
  const [width, setWidth] = useState<'320px' | '390px' | '720px'>('390px');
  // The page has one list for it, as the screen will: the Rows section's goes first
  const back = (at: { slug: string; file: string }) => {
    listKeysOn.set(true);
    setTimeout(() => {
      comeBackTo(at);
      const active = document.activeElement;
      const row = active?.closest('#list > li')?.getAttribute('data-key');
      const focused = row ?? (active?.id === undefined || active.id === '' ? 'none' : active.id);
      const picked = galleryOpen.picked.get();
      setCame(`focus: ${focused} · beside: ${picked ? `${picked.slug} ${picked.file}` : 'none'}`);
    }, 0);
  };
  const [scene, setScene] = useState<'samples' | 'many' | 'none' | 'archived'>('samples');
  // Written as it is measured: a state would draw the list again, and measure that
  const took = useRef<HTMLSpanElement>(null);
  const show = (next: typeof scene, pages: Page[]) => {
    setScene(next);
    shownPages.set(pages);
  };
  return (
    <>
      <div className="mb-2 flex flex-wrap items-center gap-2">
        <Button
          pressed={scene === 'many'}
          onClick={() => {
            // Newest first, as the server gives them
            show(
              'many',
              [...PAGES, ...MANY].sort((x, y) => y.updatedAt.localeCompare(x.updatedAt)),
            );
          }}
        >
          1000 rows
        </Button>
        <Button
          pressed={scene === 'none'}
          onClick={() => {
            show('none', []);
          }}
        >
          No page
        </Button>
        <Button
          pressed={scene === 'archived'}
          onClick={() => {
            show(
              'archived',
              PAGES.map((a) => ({ ...a, archived: true })),
            );
          }}
        >
          All archived
        </Button>
        <Button
          pressed={scene === 'samples'}
          onClick={() => {
            show('samples', PAGES);
          }}
        >
          The samples
        </Button>
        <Button
          onClick={() => {
            const rows = shownPages.get();
            const at = rows[Math.floor(Math.random() * rows.length)];
            if (at) galleryOpen.picked.set({ slug: at.slug, file: '', v: 0 });
          }}
        >
          Pick a row
        </Button>
        <span ref={took} className="font-mono text-label text-muted">
          last commit
        </span>
      </div>
      <div className="mb-2 flex flex-wrap items-center gap-2">
        <Button
          pressed={keysOn}
          onClick={() => {
            listKeysOn.set(!keysOn);
          }}
        >
          This list alone: keys ↑ ↓ j k
        </Button>
        <Button
          onClick={() => {
            back({ slug: 'plain', file: '' });
          }}
        >
          Back to a read row
        </Button>
        <Button
          onClick={() => {
            back({ slug: 'unread', file: '' });
          }}
        >
          Back to an unread row
        </Button>
        <Button
          onClick={() => {
            // The folder's tree open first, as a list come back to has it
            const opened = galleryTree.get().opened.has('folder');
            void (opened ? Promise.resolve() : galleryFolders.toggle('folder', '')).then(() => {
              setTimeout(() => {
                back({ slug: 'folder', file: 'index.html' });
              }, 0);
            });
          }}
        >
          Back to a folder’s page
        </Button>
        <Button
          pressed={folded}
          onClick={() => {
            setFolded(!folded);
          }}
        >
          Fold the list
        </Button>
        <span className="font-mono text-label text-muted">{came}</span>
      </div>
      {folded ? (
        <Button
          id="list-show"
          onClick={() => {
            setFolded(false);
          }}
        >
          Open the list (a stand-in: the columns’ own is in columns.html)
        </Button>
      ) : null}
      <div className="mb-2 flex flex-wrap items-center gap-2">
        <Button
          onClick={() => {
            setAdding(true);
          }}
        >
          Add a link (#add-link)
        </Button>
        <span className="text-small text-muted">List width:</span>
        {(['320px', '390px', '720px'] as const).map((w) => (
          <Button
            key={w}
            pressed={width === w}
            onClick={() => {
              setWidth(w);
            }}
          >
            {w}
          </Button>
        ))}
      </div>
      <AddDialog
        open={adding}
        onClosed={() => {
          setAdding(false);
        }}
      />
      <div
        hidden={folded}
        style={{ width }}
        className="max-h-[34rem] overflow-auto rounded-small border border-line bg-bg"
      >
        {/* The list's head as P9 will make it: the bar of the selected over the rest while some are selected */}
        <div className="@container/bar sticky top-0 z-5 flex h-lhead items-center gap-2.5 border-b border-line bg-panel px-4 text-small text-muted has-[>#bulk:not([hidden])]:bg-sel has-[>#bulk:not([hidden])]:px-2 has-[>#bulk:not([hidden])]:[&>:not(#bulk)]:hidden">
          <ListCount query={galleryQuery} pages={galleryPages} />
          <BulkBar
            onTags={(target) => {
              tagTarget.set(target);
            }}
            query={galleryQuery}
            pages={galleryPages}
          />
        </div>
        <Profiler
          id="page-list"
          onRender={(_id, _phase, actual) => {
            if (took.current) took.current.textContent = `last commit ${actual.toFixed(1)} ms`;
          }}
        >
          <PageList
            openers={openers}
            query={galleryQuery}
            pages={galleryPages}
            folders={galleryFolders}
            openPage={galleryOpen}
            onAdd={() => {
              setAdding(true);
            }}
          />
        </Profiler>
      </div>
    </>
  );
}

function Section({ title, children }: { readonly title: string; readonly children: ReactNode }) {
  return (
    <section className="mb-8">
      <h2 className="mb-2 text-small font-bold text-muted">{title}</h2>
      {children}
    </section>
  );
}

function Gallery() {
  useOpenMenus();
  useKeys({
    '/': () => toasts.toast('/ pressed'),
    '?': () => toasts.toast('? pressed'),
    'Ctrl+K': () => toasts.toast('Ctrl+K pressed'),
  });
  const [selected, setSelected] = useState<ReadonlySet<string>>(new Set(['plain']));
  const [selecting, setSelecting] = useState(true);
  const pagesNow = useStore(shownPages);
  const plain = pagesNow.find((p) => p.slug === 'plain');
  const folder = pagesNow.find((p) => p.slug === 'folder');
  const gone = pagesNow.find((p) => p.slug === 'gone');
  const [slowNow, setSlow] = useState(false);
  const [refuseNow, setRefuse] = useState(false);
  const [wholeNow, setWhole] = useState(false);
  const tagsOf = useStore(tagTarget);
  const editOf = useStore(editTarget);
  const [help, setHelp] = useState<HelpTab | null>(null);
  const [finding, setFinding] = useState(false);
  const picked = useStore(galleryOpen.picked);
  const readPage = pagesNow.find((p) => p.slug === picked?.slug) ?? null;
  const [slowReads, setSlowReads] = useState(false);
  const settingsOf = useStore(settingsTarget);
  const [themesNow, setThemesNow] = useState(themesAs);
  const [screen, setScreen] = useState<Screen>('list');
  const [helpDiff, setHelpDiff] = useState(false);
  const [diffGo, setDiffGo] = useState(true);
  const [mac, setMac] = useState(isMac());
  const [tabNow, setTabNow] = useState<'one' | 'two'>('one');
  const [open, setOpen] = useState(false);
  const [width, setWidth] = useState(300);
  const hold = useHold();
  const items: RowItem[] = [
    { kind: 'group', key: 'group:Rows', heading: 'Today' },
    ...pagesNow.map((p, i): RowItem => ({
      kind: 'row',
      key: `entry:${p.slug}`,
      titled: true,
      render: (kept) => (
        <PageRow
          key={p.slug}
          page={p}
          index={i}
          kept={kept}
          picked={p.slug === 'picked'}
          unread={Boolean(p.news)}
          archived={p.archived === true}
          selected={selected.has(p.slug)}
          sub={
            <span className="rounded-small border border-line px-1.25 whitespace-nowrap">{p.template ?? p.slug}</span>
          }
          actions={
            <RowActions
              page={p}
              openers={openers}
              actions={actions}
              onPick={() => {
                setSelected(new Set([...selected, p.slug]));
              }}
            />
          }
          onSelect={(on) => {
            const next = new Set(selected);
            if (on) next.add(p.slug);
            else next.delete(p.slug);
            setSelected(next);
          }}
          onTitleClick={(e) => {
            e.preventDefault();
          }}
        />
      ),
    })),
    {
      kind: 'group',
      key: 'group:Plans',
      heading: 'Plans',
      empty: t('ui_category_empty'),
    },
  ];
  return (
    <div className="flex items-center gap-3 bg-bg">
      <main className="w-[26rem] p-6">
        <p>
          <a href="columns.html">The columns of the list screens</a> and{' '}
          <a href="side.html">the tree on the frame’s left</a>, <a href="tasks.html">the task list</a> and{' '}
          <a href="frame.html">a page whole</a>, in windows of their own
        </p>
        <Section title="Server state and the error">
          <div className="mb-2 flex flex-wrap gap-2">
            <Button
              onClick={() => {
                client.reached.set(!client.reached.get());
              }}
            >
              Toggle the server
            </Button>
            <Button
              onClick={() => {
                notice.say(notice.said.get().message ? '' : t('ui_save_failed', 'connection refused'));
              }}
            >
              Toggle the error
            </Button>
          </div>
          <ServerState />
          <ErrorBar />
        </Section>
        <Section title="Rows (hover, Tab, the box, …; the menu closes on a click elsewhere, an item or Escape)">
          <div className="mb-2 flex flex-wrap gap-2">
            <Button
              pressed={selecting}
              onClick={() => {
                setSelecting(!selecting);
              }}
            >
              Selecting
            </Button>
            <Button
              pressed={slowNow}
              onClick={() => {
                slow = !slowNow;
                setSlow(!slowNow);
              }}
            >
              A slow read after each write (1.5 s)
            </Button>
          </div>
          {useStore(listKeysOn) ? (
            <p className="text-small text-muted">
              Drawn again once the keys on the page list are off: the page has one list.
            </p>
          ) : (
            <Rows items={items} selecting={selecting} />
          )}
        </Section>
        <Section title="The bars over a page: beside the list, in the frame (narrow the window under 40rem), a folder’s page, the diff">
          {plain && folder && gone ? (
            <div className="grid gap-2">
              <BarActions
                id="list-actions"
                place="list"
                page={plain}
                at={{ slug: 'plain' }}
                openers={openers}
                actions={actions}
              />
              <BarActions
                id="frame-actions"
                place="frame"
                page={plain}
                at={{ slug: 'plain' }}
                openers={openers}
                actions={actions}
              />
              <BarActions
                id="folder-actions"
                place="frame"
                page={folder}
                at={{ slug: 'folder', file: 'docs/index.html' }}
                openers={openers}
                actions={actions}
              />
              <BarActions
                id="gone-actions"
                place="frame"
                page={gone}
                at={{ slug: 'gone' }}
                openers={openers}
                actions={actions}
              />
              <BarActions
                id="diff-actions"
                place="frame"
                diffing
                page={plain}
                at={{ slug: 'plain' }}
                openers={openers}
                actions={actions}
              />
            </div>
          ) : null}
        </Section>
        <Section title="Tags: the line under a page’s bar, and the one modal (also from a row’s …)">
          <div className="mb-2 flex flex-wrap gap-2">
            <Button
              onClick={() => {
                tagTarget.set({
                  slugs: ['plain', 'unread', 'picked'],
                  title: (n) => t('ui_tag_bulk_title', n),
                  done: (n) => t('ui_tag_bulk_done', n),
                  failed: (n, why) => t('ui_tag_bulk_failed', n, why),
                  back: null,
                });
              }}
            >
              Tags of three pages
            </Button>
            <Button
              pressed={refuseNow}
              onClick={() => {
                refuse = !refuseNow;
                setRefuse(refuse);
              }}
            >
              Refuse the writes
            </Button>
          </div>
          <div className="rounded-small border border-line">
            <TagLine
              id="read-tags"
              page={plain ?? null}
              hidden={false}
              onEdit={(a, back) => {
                tagTarget.set(pageTags(a, back));
              }}
            />
            <TagLine
              id="frame-tags"
              page={pagesNow.find((p) => p.slug === 'picked') ?? null}
              hidden={false}
              onEdit={(a, back) => {
                tagTarget.set(pageTags(a, back));
              }}
            />
          </div>
        </Section>
        <Section title="Edit (from a row’s …; Refuse the writes above says no)">
          <div className="flex flex-wrap gap-2">
            <Button
              pressed={wholeNow}
              onClick={() => {
                whole = !wholeNow;
                setWhole(whole);
              }}
            >
              As the page open whole
            </Button>
          </div>
        </Section>
        <Section title="Shelf: views, parts and the search (under 70rem the views are a row, the rest in the filters’ modal, and the bar’s new pages)">
          <div className="mb-3 flex flex-wrap items-center gap-4">
            <span className="text-small text-muted">The bar’s new pages:</span>
            <News query={galleryQuery} pages={galleryPages} columns={galleryColumns} />
            <Button
              id="fold-shelf"
              onClick={() => {
                galleryColumns.set({ open: !galleryColumns.columns.get().open });
              }}
            >
              Fold or open the shelf
            </Button>
          </div>
          <div className="mb-3 flex max-w-xl items-center gap-2.5 rounded-small border border-line bg-panel p-2">
            <SearchBox query={galleryQuery} columns={galleryColumns} />
          </div>
          <div className="grid h-[34rem] grid-cols-[15rem_minmax(0,1fr)] overflow-hidden rounded-small border border-line bg-bg wide:h-auto wide:grid-cols-1">
            <ListShelf
              onAdd={() => toasts.toast('Add a link (P5d’s modal)')}
              query={galleryQuery}
              pages={galleryPages}
              columns={galleryColumns}
            />
            <div className="flex h-lhead items-center gap-2.5 border-b border-line px-4">
              <ListWhat
                query={galleryQuery}
                columns={galleryColumns}
                onShow={() => {
                  // As showColumn opens it (columns.html): open, the focus on what folds it (here, the button above)
                  galleryColumns.set({ open: true });
                  focusOn(document.getElementById('fold-shelf'));
                }}
              />
              <ListCount query={galleryQuery} pages={galleryPages} />
            </div>
          </div>
        </Section>
        <Section title="Page list (the shelf above narrows it; open the folder and its directories; a title opens beside, again whole; ↑ ↓ j k move; 1000 rows measures a redraw)">
          <ListScene />
        </Section>
        <Section title="Reading a page beside the list (the page tells its frame it was read; Slow answers waits 1.5 s)">
          <div className="mb-2 flex flex-wrap gap-2">
            {(['plain', 'unread', 'archived', 'gone'] as const).map((slug) => (
              <Button
                key={slug}
                pressed={picked?.slug === slug}
                onClick={() => {
                  galleryOpen.picked.set({ slug, file: '', v: 0 });
                }}
              >
                {slug}
              </Button>
            ))}
            <Button
              onClick={() => {
                galleryOpen.picked.set(null);
              }}
            >
              None
            </Button>
            <Button
              pressed={slowReads}
              onClick={() => {
                readsSlow = !slowReads;
                setSlowReads(readsSlow);
              }}
            >
              Slow answers
            </Button>
          </div>
          <div className="flex h-96 flex-col overflow-hidden rounded-small border border-line bg-panel">
            <ReaderPane
              at={picked}
              page={readPage}
              actions={
                readPage && picked ? (
                  <BarActions
                    id="read-actions"
                    place="list"
                    page={readPage}
                    at={picked}
                    openers={openers}
                    actions={actions}
                  />
                ) : null
              }
              keys={new Set(['/', '?'])}
              onKey={(key) => {
                toasts.toast(`key ${key} from the page`);
              }}
              reads={galleryPageReads}
              note={galleryNote}
            />
          </div>
        </Section>
        <Section title="Versions: beside the list (pick three, open the choice, v1 and the words; one has no choice) and the frame at v2 (narrow the window under 40rem: the words wrap)">
          <VersionsSection />
        </Section>
        <Section title="Search (↑ ↓, Enter; the foot holds the row’s buttons; its … opens up)">
          <div className="flex flex-wrap gap-2">
            <Button
              onClick={() => {
                setFinding(true);
              }}
            >
              Open the search
            </Button>
            <Button
              onClick={() => {
                // A way back that took the focus, as an archive from the keys gives
                toasts.toast('Archived “A page read”', {
                  ms: 8000,
                  focus: true,
                  button: { label: 'Undo', undo: true, holds: false, onPress: () => undefined },
                });
                setTimeout(() => {
                  setFinding(true);
                }, 500);
              }}
            >
              A way back with the focus, then the search
            </Button>
          </div>
        </Section>
        <Section title="Settings (the list, a theme, the task mode; a page’s themes from a row’s …)">
          <div className="flex flex-wrap gap-2">
            <Button
              onClick={() => {
                settingsTarget.set({ page: null, back: null });
              }}
            >
              Open the settings
            </Button>
            {(['themes', 'broken', 'settings', 'fails'] as const).map((as) => (
              <Button
                key={as}
                pressed={themesNow === as}
                onClick={() => {
                  themesAs = as;
                  setThemesNow(as);
                }}
              >
                {as === 'themes'
                  ? 'Read'
                  : as === 'broken'
                    ? 'Own things broken'
                    : as === 'settings'
                      ? 'settings.json unreadable'
                      : 'Read fails'}
              </Button>
            ))}
          </div>
        </Section>
        <Section title="Tabs (← → between them; the narrow box wraps them)">
          {[26, 12].map((rem) => (
            <div key={rem} className="mb-3" style={{ width: `${String(rem)}rem` }}>
              <TabList label="Tabs">
                {(['one', 'two'] as const).map((name) => (
                  <Tab
                    key={name}
                    id={`tab-${String(rem)}-${name}`}
                    selected={tabNow === name}
                    onSelect={() => {
                      setTabNow(name);
                    }}
                  >
                    {name === 'one' ? t('ui_help_about_tasks') : t('ui_keys')}
                  </Tab>
                ))}
              </TabList>
              <p
                id={`panel-${String(rem)}`}
                role="tabpanel"
                aria-labelledby={`tab-${String(rem)}-${tabNow}`}
                className="m-0 pt-2 text-small text-muted"
              >
                {tabNow}
              </p>
            </div>
          ))}
        </Section>
        <Section title="Help (? or the bar’s button)">
          <div className="flex flex-wrap gap-2">
            <Button
              onClick={() => {
                setHelp('about');
              }}
            >
              The bar’s button
            </Button>
            <Button
              onClick={() => {
                setHelp('keys');
              }}
            >
              ?
            </Button>
            {(['list', 'taskList', 'taskPage', 'frame'] as const).map((name) => (
              <Button
                key={name}
                pressed={screen === name}
                onClick={() => {
                  setScreen(name);
                }}
              >
                {name}
              </Button>
            ))}
            <Button
              pressed={helpDiff}
              onClick={() => {
                setHelpDiff(!helpDiff);
              }}
            >
              A diff
            </Button>
            <Button
              pressed={diffGo}
              onClick={() => {
                setDiffGo(!diffGo);
              }}
            >
              d goes to a diff
            </Button>
            <Button
              pressed={mac}
              onClick={() => {
                setMac(!mac);
              }}
            >
              Mac
            </Button>
          </div>
        </Section>
        <FindDialog
          open={finding}
          onClosed={() => {
            setFinding(false);
          }}
          openers={openers}
          pages={galleryPages}
          order={{ forgetOrder: () => undefined } as unknown as Order}
          go={(href) => {
            toasts.toast(`would go to ${href}`);
          }}
        />
        <SettingsDialog
          target={settingsOf}
          onClosed={() => {
            settingsTarget.set(null);
          }}
          settings={gallerySettings}
          writes={galleryThemeWrites}
        />
        <TagDialog
          target={tagsOf}
          onClosed={() => {
            tagTarget.set(null);
          }}
          pages={galleryPages}
          apply={applyTags}
        />
        <EditDialog
          target={editOf}
          onClosed={() => {
            editTarget.set(null);
          }}
          save={saveEdit}
        />
        <KeysDialog
          open={help}
          screen={screen}
          diff={helpDiff}
          diffGo={diffGo}
          backWords=""
          mac={mac}
          onClosed={() => {
            setHelp(null);
          }}
        />
        <Section title="Buttons">
          <div className="flex flex-wrap items-center gap-2">
            <Button>{t('ui_close')}</Button>
            <Button kind="primary">{t('ui_save')}</Button>
            <Button disabled>{t('ui_save')}</Button>
            <IconButton label="Archived: restore" on>
              <ArchiveIcon />
            </IconButton>
            <IconButton label="Star" tone="star" pressed>
              <StarIcon />
            </IconButton>
          </div>
        </Section>
        <Section title="Modal (Escape, the backdrop, Close; Hold holds it 3 s)">
          <Button
            onClick={() => {
              setOpen(true);
            }}
          >
            Open the modal
          </Button>
          <Dialog
            id="gallery-dialog"
            open={open}
            onClosed={() => {
              setOpen(false);
            }}
            hold={hold}
            labelledBy="gallery-dialog-title"
          >
            <DialogHead
              icon={<StarIcon />}
              titleId="gallery-dialog-title"
              title={t('ui_narrow')}
              lead="A modal's head, body and foot"
            />
            <input className="rounded-small border border-line bg-panel px-2 py-1" aria-label="A field" />
            <DialogActions>
              <Button
                onClick={() => {
                  void hold.whileHeld(() => new Promise((go) => setTimeout(go, 3000)));
                }}
              >
                Hold 3 s
              </Button>
              <DialogClose />
            </DialogActions>
          </Dialog>
        </Section>
        <Section title="Toasts">
          <div className="flex flex-wrap gap-2">
            <Button onClick={() => toasts.toast(t('ui_undone'))}>A toast</Button>
            <Button
              onClick={() =>
                toasts.toast(t('ui_read_done'), {
                  ms: 8000,
                  page: true,
                  button: {
                    label: t('ui_mark_unread'),
                    undo: false,
                    holds: false,
                    onPress: () => {
                      toasts.toast('pressed');
                    },
                  },
                })
              }
            >
              A page’s toast with a button
            </Button>
          </div>
        </Section>
        <Section title="Back to a task's page (narrow the window under 56rem: the mark alone)">
          <BackLink
            back={backOf('/tasks?project=floe&task=%23128', window.location.origin)}
            onPress={() => undefined}
          />
        </Section>
        <Section title="Keys: / ? Ctrl+K show a toast">
          <p className="text-small text-muted">Press them outside a field.</p>
        </Section>
      </main>
      <Grip
        id="gallery-grip"
        label={t('ui_shelf_width')}
        controls="root"
        at={width + 448}
        width={width}
        min={200}
        max={600}
        left={() => 448}
        onSet={(w) => {
          setWidth(Math.min(Math.max(Math.round(w), 200), 600));
        }}
        onKeep={() => toasts.toast('kept')}
      />
      <Toasts />
    </div>
  );
}

const el = document.getElementById('root');
const boot = found['./boot.json'];
if (el !== null && boot) {
  setBoot(boot);
  startVersions();
  createRoot(el).render(
    <StrictMode>
      <Gallery />
    </StrictMode>,
  );
} else if (el !== null) {
  el.textContent = 'gallery/boot.json is not there: start the gallery with just web-gallery';
}
