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
import { Tab, TabList } from '@/components/tabs';
import { ServerState } from '@/components/server-state';
import { Toasts } from '@/components/toasts';
import { setBoot, type Boot } from '@/config/boot';
import { KeysDialog, type HelpScreen, type HelpTab } from '@/features/help/components/keys-dialog';
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
import { useOpenMenus } from '@/hooks/use-open-menu';
import { useStore } from '@/hooks/use-store';
import { client } from '@/lib/api-client';
import { createStore } from '@/lib/store';
import { t } from '@/lib/t';
import { isMac } from '@/lib/platform';
import { createActOn } from '@/stores/act-on';
import { notice } from '@/stores/notice';
import type { OpenPage } from '@/stores/open-page';
import type { Order } from '@/stores/order';
import { createPageActions } from '@/stores/page-actions';
import type { PageWrites } from '@/stores/page-writes';
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
// The list as a read of the fake server would give it: the pages as they are now
const listed = createStore({ entries: PAGES, tasks: { error: '', tasks: {}, raw: '' }, readWhy: '', drawWhy: '' });
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
  state: listed,
} as unknown as Pages;
// A tag write adds and removes; any other is the fields it names
const patched = (a: Page, body: unknown): Partial<Page> => {
  const { tags } = body as { tags?: { add: string[]; remove: string[] } };
  return tags
    ? { tags: [...new Set([...(a.tags ?? []), ...tags.add])].filter((x) => !tags.remove.includes(x)) }
    : (body as Partial<Page>);
};
const answered = { ok: true, status: 200, text: '', rev: 1, stale: '', pages: [], written: [] };
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
  reads: { markUnread: () => Promise.resolve({ kind: 'sent', cleared: false }) } as unknown as Reads,
  go: (href) => {
    toasts.toast(`would go to ${href}`);
  },
});
// The modals the menus open, each held by the gallery as the app will hold it
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
            ...(themesAs === 'settings' ? { settings: 'settings.json: unexpected end' } : {}),
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
  const settingsOf = useStore(settingsTarget);
  const [themesNow, setThemesNow] = useState(themesAs);
  const [screen, setScreen] = useState<HelpScreen>('list');
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
          {plain && folder && gone ? (
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
              page={plain ?? null}
              hidden={false}
              onEdit={(a, back) => {
                tagTarget.set(pageTags(a, back));
              }}
            />
            <TagLine
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
