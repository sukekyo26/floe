// The gallery: the shared components in their states, for a person to try them before any screen is switched. It is
// served by Vite's dev server alone (just web-gallery), never built into dist
import './gallery.css';

import { StrictMode, useState, type ReactNode } from 'react';
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
import { ServerState } from '@/components/server-state';
import { Toasts } from '@/components/toasts';
import { setBoot, type Boot } from '@/config/boot';
import { useHold } from '@/hooks/use-hold';
import { useKeys } from '@/hooks/use-keys';
import { useOpenMenus } from '@/hooks/use-open-menu';
import { useStore } from '@/hooks/use-store';
import { client } from '@/lib/api-client';
import { createStore } from '@/lib/store';
import { t } from '@/lib/t';
import { notice } from '@/stores/notice';
import { createPageActions } from '@/stores/page-actions';
import type { Pages } from '@/stores/pages';
import type { Reads } from '@/stores/reads';
import type { TaskWrites } from '@/stores/tasks';
import { toasts } from '@/stores/toasts';
import type { Page } from '@/types/page';

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
    updatedAt: '',
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
// slow: the list's read after a write takes 1.5 s, to see the toast and its focus come before it
let slow = false;
const answered = { ok: true, status: 200, text: '', rev: 1, stale: '', pages: [], written: [] };
const actions = createPageActions({
  writes: {
    writePage: () => Promise.resolve(''),
    patchPage: (slug, body) => {
      edit(slug, body as Partial<Page>);
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
  pages: {
    load: () =>
      new Promise((go) => {
        setTimeout(go, slow ? 1500 : 0);
      }),
    rereadTasks: () => Promise.resolve(),
    readRev: () => 0,
    state: createStore({ entries: [], tasks: { error: '', tasks: {}, raw: '' }, readWhy: '', drawWhy: '' }),
  } as unknown as Pages,
  reads: { markUnread: () => Promise.resolve({ kind: 'sent', cleared: false }) } as unknown as Reads,
  go: (href) => {
    toasts.toast(`would go to ${href}`);
  },
});
const openers: Openers = {
  onTags: (a) => toasts.toast(`tags of ${a.slug}`),
  onTheme: (a) => toasts.toast(`theme of ${a.slug}`),
  onEdit: (a) => toasts.toast(`edit ${a.slug}`),
  onTaskSet: (a) => toasts.toast(`set the task of ${a.slug}`),
};

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
  const [slowNow, setSlow] = useState(false);
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
      add: { onAdd: () => toasts.toast('add pressed') },
      empty: t('ui_category_empty'),
    },
  ];
  return (
    <div className="flex items-center gap-3 bg-bg">
      <main className="w-[26rem] p-6">
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
          <Rows items={items} selecting={selecting} />
        </Section>
        <Section title="The bars over a page: beside the list, in the frame (narrow the window under 40rem), a folder’s page, the diff">
          {plain && folder ? (
            <div className="grid gap-2">
              <BarActions
                id="read-actions"
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
        <Section title="Back (narrow the window under 56rem: the mark alone)">
          <BackLink />
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
  createRoot(el).render(
    <StrictMode>
      <Gallery />
    </StrictMode>,
  );
} else if (el !== null) {
  el.textContent = 'gallery/boot.json is not there: start the gallery with just web-gallery';
}
