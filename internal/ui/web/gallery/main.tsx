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
import { MenuItem, MoreMenu } from '@/components/more-menu';
import { PageRow } from '@/components/page-row';
import { Rows, type RowItem } from '@/components/rows';
import { ServerState } from '@/components/server-state';
import { Toasts } from '@/components/toasts';
import { setBoot, type Boot } from '@/config/boot';
import { useHold } from '@/hooks/use-hold';
import { useKeys } from '@/hooks/use-keys';
import { useOpenMenus } from '@/hooks/use-open-menu';
import { client } from '@/lib/api-client';
import { t } from '@/lib/t';
import { notice } from '@/stores/notice';
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

function Actions({ page }: { readonly page: Page }) {
  const [star, setStar] = useState(page.slug === 'picked');
  return (
    <>
      <IconButton
        size="row"
        testid="archive"
        label={t('ui_archive_named', page.title ?? '')}
        on={page.archived === true}
      >
        <ArchiveIcon />
      </IconButton>
      <IconButton
        size="row"
        testid="star"
        tone="star"
        pressed={star}
        label={t('ui_favorite_named', page.title ?? '')}
        onClick={() => {
          setStar(!star);
        }}
      >
        <StarIcon />
      </IconButton>
      <MoreMenu id={`more:${page.slug}`} label={t('ui_more_named', page.title ?? '')}>
        <MenuItem icon={<StarIcon />}>{t('ui_favorite')}</MenuItem>
        <MenuItem icon={<ArchiveIcon />} keepsOpen tone="danger">
          {t('ui_delete')} (keeps the menu open)
        </MenuItem>
      </MoreMenu>
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
  const [oldPad, setOldPad] = useState(false);
  const [open, setOpen] = useState(false);
  const [width, setWidth] = useState(300);
  const hold = useHold();
  const items: RowItem[] = [
    { kind: 'group', key: 'group:Rows', heading: 'Today' },
    ...PAGES.map((p, i): RowItem => ({
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
          actions={<Actions page={p} />}
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
              pressed={oldPad}
              onClick={() => {
                setOldPad(!oldPad);
              }}
            >
              ui.html’s 0.4rem padding of the row buttons
            </Button>
          </div>
          <div className={oldPad ? 'old-pad' : ''}>
            <Rows items={items} selecting={selecting} />
          </div>
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
                  button: <Button>{t('ui_mark_unread')}</Button>,
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
