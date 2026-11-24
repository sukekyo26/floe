import { useEffect, useEffectEvent, useLayoutEffect, useRef, useState } from 'react';

import { ArchivedBadge } from '@/components/archived-badge';
import { Dialog, DialogActions, DialogClose, DialogHead } from '@/components/dialog';
import { barMenuId, focusOn } from '@/components/focus-places';
import { SearchIcon } from '@/components/icons';
import { HitRow } from '@/components/hit-row';
import { KeyHint } from '@/components/key-hint';
import { BarActions } from '@/components/page-actions';
import type { Openers } from '@/components/page-menu';
import { RowSub } from '@/components/row-sub';
import { findCount, findHits } from '@/features/find/utils/find-hits';
import { FIRST, findKey, findReached, type Reached } from '@/features/find/utils/find-state';
import { useMovedOverlay } from '@/hooks/use-moved-overlay';
import { closeMenu } from '@/hooks/use-open-menu';
import { useStore } from '@/hooks/use-store';
import { t } from '@/lib/t';
import { menus } from '@/stores/menus';
import { order as pageOrder, type Order } from '@/stores/order';
import { pages as pagePages, type Pages } from '@/stores/pages';
import { nextReached } from '@/utils/list-keys';
import { FIND_ROWS } from '@/utils/search';
import { projectOrder } from '@/utils/project-order';

const FOOT = 'find-actions';
const FOOT_MENU = barMenuId(FOOT);

interface FindDialogProps {
  readonly open: boolean;
  readonly onClosed: () => void;
  // openers open the modals of the foot's menu, the search closed first and the focus given back nowhere
  readonly openers: Openers;
  readonly pages?: Pages;
  readonly order?: Order;
  readonly go?: (href: string) => void;
}

// FindDialog is the search: every page by the words typed, the archived after the rest, the first rows; ↑ ↓ from
// anywhere in it reach a row, Enter on the field opens it, and the foot holds the bar's buttons of the row reached.
// While open, the page's toasts and error line are in it (useMovedOverlay)
export function FindDialog({
  open,
  onClosed,
  openers,
  pages = pagePages,
  order = pageOrder,
  go = (href) => {
    window.location.assign(href);
  },
}: FindDialogProps) {
  const { entries } = useStore(pages.state);
  const [words, setWords] = useState('');
  const [reached, setReached] = useState<Reached>(FIRST);
  // Opened again from closed, the foot is drawn anew: its menu shut, a delete armed let go
  const [opened, setOpened] = useState(0);
  const [wasOpen, setWasOpen] = useState(false);
  // The render that opens it sets the first row; holding the row of the words before would put the old one back
  const opening = open && !wasOpen;
  if (open !== wasOpen) {
    setWasOpen(open);
    if (open) {
      setWords('');
      setReached(FIRST);
      setOpened(opened + 1);
    }
  }
  const field = useRef<HTMLInputElement>(null);
  const errorBox = useRef<HTMLDivElement>(null);
  const toastBox = useRef<HTMLDivElement>(null);
  const foot = useRef<HTMLSpanElement>(null);
  useMovedOverlay(open, { toasts: toastBox, error: errorBox });
  // Opened, the foot's menu left open last time shuts and the field takes the focus, whatever had it: a toast moved in
  // with the focus loses it
  const footFor = useRef<string | null>(null);
  useLayoutEffect(() => {
    if (!open) return;
    menus.hide(FOOT_MENU);
    footFor.current = null;
    focusOn(field.current);
  }, [open]);

  const hits = findHits(words, entries);
  const shown = hits.slice(0, FIND_ROWS);
  const at = findReached(shown, reached);
  const page = shown[at] ?? null;
  // The row reached is held by its page and its place at once, as opened or typed too: read again, a new page at the
  // top does not take its place, and it moved, its new place is the one left when it goes; gone, what came to its place
  // is reached now; none, its place is held for the rows to come back
  const reachedNow = page?.slug ?? null;
  const atNow = at >= 0 ? at : reached.at;
  if (!opening && (reachedNow !== reached.slug || atNow !== reached.at)) setReached({ slug: reachedNow, at: atNow });
  const ranks = projectOrder(entries);
  const count = findCount(words, hits.length);

  // The foot follows the row reached: another row's menu shuts; the focus goes to the field only where it would be
  // lost — in the menu of the row left, on a button the row reached has not, or in the foot of a row gone
  // byKey tells that the row changed by an arrow, not by the rows read again
  const byKey = useRef(false);
  // footHeld tells that the focus was in the foot: a button the row reached has not is gone before the effect reads
  // the focus, and the focus left on the body
  const footHeld = useRef(false);
  useLayoutEffect(() => {
    if (!open) return;
    const now = page?.slug ?? null;
    const before = footFor.current;
    footFor.current = now;
    const moved = byKey.current;
    byKey.current = false;
    const active = document.activeElement;
    const inFoot = foot.current?.contains(active) ?? false;
    if (now !== before) {
      const inMenu = inFoot && active?.closest(`[data-menu-id="${FOOT_MENU}"]`) !== null;
      menus.hide(FOOT_MENU);
      if (inFoot && (inMenu || !moved || now === null)) focusOn(field.current);
    }
    if (inFoot && active instanceof HTMLElement && active.closest('[hidden]')) focusOn(field.current);
    if (footHeld.current && !inFoot && (active === null || active === document.body)) {
      footHeld.current = false;
      focusOn(field.current);
    }
    if (at >= 0) document.getElementById(`find-${String(at)}`)?.scrollIntoView({ block: 'nearest' });
  }, [open, page, at]);

  // Keys and presses on the whole modal, the toasts moved into it too: a DOM listener, as React's events of the
  // toasts go up React's tree, not the modal's
  const onKey = useEffectEvent((e: KeyboardEvent) => {
    const what = findKey(e, {
      onField: e.target === field.current,
      reached: at,
      menuOpen: menus.open.get() === FOOT_MENU,
    });
    if (what === 'closeMenu') {
      e.preventDefault();
      e.stopPropagation();
      // Its button takes the focus back, where the arrows reach the rows again
      closeMenu(menus, FOOT_MENU);
    } else if (what === 'hold') e.stopPropagation();
    else if (what === 'close') {
      e.preventDefault();
      onClosed();
    } else if (what === 'move') {
      e.preventDefault();
      const next = nextReached(at, shown.length, e.key);
      // No rows, or the one: nothing to move to
      if (shown.length === 0 || next === at) return;
      byKey.current = true;
      setReached({ slug: shown[next]?.slug ?? null, at: next });
    } else if (what === 'open' && page) {
      e.preventDefault();
      order.forgetOrder();
      go(document.getElementById(`find-${String(at)}`)?.querySelector('a')?.href ?? '');
    }
  });
  useEffect(() => {
    const d = document.getElementById('find');
    if (!open || !d) return;
    // A press on what does nothing leaves the focus where it is, as the field takes what is typed; the words of an
    // error stay selectable
    // LIMIT: the other words in the modal cannot be selected with the mouse; the menu copies a path. Let them if others
    // come to be copied
    const onDown = (e: MouseEvent) => {
      if (e.target instanceof Element && !e.target.closest('a, button, input, summary, #error')) e.preventDefault();
    };
    const key = (e: KeyboardEvent) => {
      onKey(e);
    };
    d.addEventListener('keydown', key, true);
    d.addEventListener('mousedown', onDown);
    return () => {
      d.removeEventListener('keydown', key, true);
      d.removeEventListener('mousedown', onDown);
    };
  }, [open]);

  // The foot's menu opens a modal of another feature: the search closes first, and the focus goes nowhere back
  const closeThen =
    <A,>(open: (a: A, back: () => boolean) => void) =>
    (a: A) => {
      onClosed();
      open(a, () => false);
    };
  const footOpeners: Openers = {
    onTags: closeThen(openers.onTags),
    onTheme: closeThen(openers.onTheme),
    onEdit: closeThen(openers.onEdit),
    onTaskSet: closeThen(openers.onTaskSet),
  };

  return (
    <Dialog id="find" open={open} onClosed={onClosed} labelledBy="find-title" raised>
      <DialogHead
        icon={<SearchIcon />}
        titleId="find-title"
        title={t('ui_find')}
        lead={
          <span id="find-lead" className="flex flex-wrap items-center gap-x-2 gap-y-0.75">
            <span id="find-cut">{t(count.key, count.n)}</span>
            {/* Without rows the arrows have nothing to move over; the line keeps its height */}
            <KeyHint keys={['↑', '↓']} words={t('ui_find_pick')} shown={shown.length > 0} />
            <KeyHint keys={['Enter']} words={t('ui_find_open')} shown={shown.length > 0} />
          </span>
        }
      />
      <input
        ref={field}
        id="find-q"
        data-testid="text"
        type="search"
        role="combobox"
        aria-expanded="true"
        aria-autocomplete="list"
        aria-controls="find-list"
        aria-activedescendant={at >= 0 ? `find-${String(at)}` : undefined}
        aria-labelledby="find-title"
        placeholder={t('ui_find_placeholder')}
        autoComplete="off"
        value={words}
        onChange={(e) => {
          setWords(e.target.value);
          setReached(FIRST);
        }}
        className="h-9 w-full min-w-0 rounded-small border border-line bg-panel px-3 text-fg shadow-sm placeholder:text-faint [&::-webkit-search-cancel-button]:hidden"
      />
      {/* The page's error line, while open */}
      <div ref={errorBox} className="contents [&>#error]:m-0" />
      <ul
        id="find-list"
        tabIndex={-1}
        // oxlint-disable-next-line jsx-a11y/prefer-tag-over-role, jsx-a11y/no-noninteractive-element-to-interactive-role -- the field's listbox of rows holding links, which a select cannot hold
        role="listbox"
        aria-labelledby="find-title"
        hidden={shown.length === 0}
        className="m-0 me-[-10px] h-[min(20rem,calc(100dvh-17rem))] [scrollbar-gutter:stable] list-none overflow-y-auto p-0"
      >
        {shown.map((a, i) => (
          <HitRow
            key={a.slug}
            id={`find-${String(i)}`}
            page={a}
            reached={i === at}
            sub={<RowSub page={a} ranks={ranks} middle={a.archived ? <ArchivedBadge /> : null} />}
            onOpen={() => {
              order.forgetOrder();
            }}
          />
        ))}
      </ul>
      <div
        id="find-none"
        hidden={shown.length > 0}
        className="grid h-[min(20rem,calc(100dvh-17rem))] place-content-center px-6 text-center text-muted"
      >
        <strong className="mb-1 block text-[1rem] text-fg">{t('ui_nomatch')}</strong>
        {t('ui_find_no_files')}
      </div>
      <DialogActions testid="find-foot">
        <div
          id="find-acts"
          hidden={page === null}
          className="-ms-1.75 me-auto flex items-center [@media(hover:none)]:hidden"
        >
          <span
            ref={foot}
            className="contents"
            onFocus={() => {
              footHeld.current = true;
            }}
            onBlur={(e) => {
              if (!foot.current?.contains(e.relatedTarget)) footHeld.current = false;
            }}
          >
            {page ? (
              <BarActions
                key={opened}
                id={FOOT}
                place="find"
                page={page}
                at={{ slug: page.slug }}
                openers={footOpeners}
              />
            ) : null}
          </span>
        </div>
        <DialogClose />
      </DialogActions>
      {/* The page's toasts, while open */}
      <div ref={toastBox} className="contents" />
    </Dialog>
  );
}
