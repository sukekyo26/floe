import { useLayoutEffect, useRef, useState, type KeyboardEvent, type MouseEvent } from 'react';

import { Badge } from '@/components/badge';
import { Dialog, DialogActions, DialogClose, DialogHead, useDialog } from '@/components/dialog';
import { HitRow } from '@/components/hit-row';
import { PlusIcon } from '@/components/icons';
import { KeyHint } from '@/components/key-hint';
import { RowSub } from '@/components/row-sub';
import { moveAsk as ask0, type MoveAsk } from '@/features/tasks/stores/move-ask';
import { addHits, addSaid, hereBadge, pageFrom, type AddTo } from '@/features/tasks/utils/add-page';
import { useHold, type Hold } from '@/hooks/use-hold';
import { useStore } from '@/hooks/use-store';
import { t } from '@/lib/t';
import { pages as pagePages, type Pages } from '@/stores/pages';
import { taskWrites, type TaskWrites } from '@/stores/tasks';
import { toasts as pageToasts, type Toasts } from '@/stores/toasts';
import type { FocusBack } from '@/types/focus';
import type { Page } from '@/types/page';
import type { TaskOf } from '@/types/task';
import { composing } from '@/utils/keys';
import { nextReached } from '@/utils/list-keys';
import { FIND_ROWS } from '@/utils/search';

// What Add a page is opened for: the task as its page drew it, the category, and the way back to its add button
export interface AddPageOpen {
  task: TaskOf;
  category: string;
  back: FocusBack;
}

export interface AddPageDialogProps {
  readonly open: AddPageOpen | null;
  readonly onClosed: () => void;
  readonly pages?: Pages;
  readonly writes?: Pick<TaskWrites, 'putPageOn' | 'tasksOf'>;
  readonly toasts?: Toasts;
  readonly ask?: MoveAsk;
}

// No pill: the pages are of one project, the task's, as on the task's page
const NO_RANKS: ReadonlyMap<string, number> = new Map();

// AddPageDialog puts a page of the task's project on one of its categories (ui.html #add-page-dialog, openAddPage,
// addPageTo): the words narrow the pages, ↑ ↓ reach one and Enter or a press puts it, and the modal closes. Nothing is
// reached as it opens, so that no Enter puts a page by itself. It sends the rev it opened at; refused as stale, the rev
// read again. A page on another task is asked about first (MoveDialog)
export function AddPageDialog({
  open,
  onClosed,
  pages = pagePages,
  writes = taskWrites,
  toasts = pageToasts,
  ask = ask0,
}: AddPageDialogProps) {
  const hold = useHold();
  // The last opened, whose add button the focus goes back to as it closes: open is null by then
  const [last, setLast] = useState<AddPageOpen | null>(null);
  if (open !== null && open !== last) setLast(open);
  return (
    <Dialog
      id="add-page-dialog"
      open={open !== null}
      onClosed={onClosed}
      hold={hold}
      labelledBy="add-page-title"
      raised
      afterClose={() => last?.back() ?? false}
    >
      <AddPageBody
        open={open}
        onClosed={onClosed}
        pages={pages}
        writes={writes}
        toasts={toasts}
        ask={ask}
        hold={hold}
      />
    </Dialog>
  );
}

function AddPageBody({
  open,
  onClosed,
  pages,
  writes,
  toasts,
  ask,
  hold,
}: Required<Omit<AddPageDialogProps, 'open'>> & { readonly open: AddPageOpen | null; readonly hold: Hold }) {
  const { close } = useDialog();
  const { entries } = useStore(pages.state);
  // The last opened, which the modal shows and goes back from as it closes: open is null by then
  const [last, setLast] = useState<AddPageOpen | null>(null);
  const [to, setTo] = useState<AddTo>({ project: '', name: '', category: '', rev: 0 });
  const [words, setWords] = useState('');
  // The row reached, by its page: read again, the same page stays reached where it still is
  const [reached, setReached] = useState<string | null>(null);
  const [error, setError] = useState('');
  // A page being put: a second press while it goes does nothing
  const adding = useRef(false);
  if (open !== null && open !== last) {
    setLast(open);
    const x = writes.tasksOf().find((y) => y.project === open.task.project && y.name === open.task.name) ?? open.task;
    setTo({ project: x.project, name: x.name, category: open.category, rev: pages.readRev(x.project, x.name) });
    setWords('');
    setReached(null);
    setError('');
  }
  const { all, hits } = addHits(entries, to, words);
  const shown = hits.slice(0, FIND_ROWS);
  const at = shown.findIndex((a) => a.slug === reached);
  useLayoutEffect(() => {
    if (at >= 0) document.getElementById(`add-page-${String(at)}`)?.scrollIntoView({ block: 'nearest' });
  }, [at]);

  const put = async (a: Page | undefined) => {
    if (!a || adding.current) return;
    adding.current = true;
    const from = pageFrom(a, to);
    try {
      // No leaves the modal as it was, the page still reached: its rows take no focus, so the field has it
      if (!(await ask.ask(a, to.name))) {
        document.getElementById('add-page-q')?.focus();
        return;
      }
      const done = { put: false };
      await hold.whileHeld(async () => {
        let res;
        try {
          res = await writes.putPageOn(a, to.name, { new: false, rev: to.rev }, to.category);
        } catch (err) {
          setError(t('ui_save_failed', err instanceof Error ? err.message : String(err)));
          return;
        }
        // Refused as stale, the rev is the one read again
        if (!res.ok) {
          if (res.status === 409) setTo({ ...to, rev: pages.readRev(to.project, to.name) });
          setError(t('ui_save_failed', res.status === 409 ? res.stale : res.text));
          return;
        }
        toasts.toast(addSaid(a, to, from, res.pages[0]));
        await pages.load();
        done.put = true;
      });
      if (done.put) onClosed();
    } finally {
      adding.current = false;
    }
  };

  const onKey = (e: KeyboardEvent<HTMLInputElement>) => {
    if (composing(e.nativeEvent)) return;
    if (e.key === 'ArrowDown' || e.key === 'ArrowUp') {
      e.preventDefault();
      const next = nextReached(at, shown.length, e.key);
      if (shown.length > 0) setReached(shown[next]?.slug ?? null);
    } else if (e.key === 'Enter' && at >= 0) {
      e.preventDefault();
      void put(shown[at]);
    } else if (e.key === 'Escape') {
      e.preventDefault();
      close();
    }
  };
  const onPress = (e: MouseEvent<HTMLUListElement>) => {
    const li = e.target instanceof Element ? e.target.closest('#add-page-list > li') : null;
    if (!li) return;
    void put(shown[[...e.currentTarget.children].indexOf(li)]);
  };

  const none = shown.length === 0;
  const task = open?.task;
  return (
    <>
      <DialogHead
        icon={<PlusIcon />}
        titleId="add-page-title"
        title={t('ui_page_add_named', to.category)}
        lead={
          <span id="add-page-lead" className="flex flex-wrap items-center gap-x-2 gap-y-0.75">
            <span id="add-page-target">{[task?.name, task?.title].filter(Boolean).join(' ')}</span>
            <KeyHint keys={['↑', '↓']} words={t('ui_find_pick')} shown={!none} />
            <KeyHint keys={['Enter']} words={t('ui_page_add_put')} shown={!none} />
          </span>
        }
      />
      <input
        id="add-page-q"
        data-testid="text"
        type="search"
        role="combobox"
        aria-expanded="true"
        aria-autocomplete="list"
        aria-controls="add-page-list"
        aria-activedescendant={at >= 0 ? `add-page-${String(at)}` : undefined}
        aria-labelledby="add-page-title"
        placeholder={t('ui_find_placeholder')}
        autoComplete="off"
        data-autofocus=""
        value={words}
        onChange={(e) => {
          setWords(e.target.value);
          setError('');
        }}
        onKeyDown={onKey}
        className="h-9 w-full min-w-0 rounded-small border border-line bg-panel px-3 text-fg shadow-sm placeholder:text-faint [&::-webkit-search-cancel-button]:hidden"
      />
      {/* oxlint-disable-next-line jsx-a11y/click-events-have-key-events -- a press on a row puts it; the keys are the field's (aria-activedescendant) */}
      <ul
        id="add-page-list"
        tabIndex={-1}
        // oxlint-disable-next-line jsx-a11y/prefer-tag-over-role, jsx-a11y/no-noninteractive-element-to-interactive-role -- the field's listbox of rows, which a select cannot hold (ui.html's markup)
        role="listbox"
        aria-labelledby="add-page-title"
        hidden={none}
        onClick={onPress}
        className="m-0 me-[-10px] h-[min(20rem,calc(100dvh-17rem))] [scrollbar-gutter:stable] list-none overflow-y-auto p-0"
      >
        {shown.map((a, i) => {
          const badge = hereBadge(a, to);
          return (
            <HitRow
              key={a.slug}
              id={`add-page-${String(i)}`}
              page={a}
              reached={i === at}
              link={false}
              sub={<RowSub page={a} ranks={NO_RANKS} middle={badge ? <Badge>{badge}</Badge> : null} />}
            />
          );
        })}
      </ul>
      <div
        id="add-page-none"
        hidden={!none}
        className="grid h-[min(20rem,calc(100dvh-17rem))] place-content-center px-6 text-center text-muted"
      >
        <strong id="add-page-none-words" className="mb-1 block text-[1rem] text-fg">
          {t(all.length === 0 ? 'ui_page_add_none' : 'ui_nomatch')}
        </strong>
      </div>
      <p
        id="add-page-error"
        role="alert"
        hidden={!error}
        className="m-0 rounded-menu bg-danger-soft px-3.5 py-2.5 whitespace-pre-wrap text-danger"
      >
        {error}
      </p>
      <DialogActions>
        <DialogClose />
      </DialogActions>
    </>
  );
}
