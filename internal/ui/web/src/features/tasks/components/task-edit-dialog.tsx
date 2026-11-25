import { useState } from 'react';
import { flushSync } from 'react-dom';

import { Button } from '@/components/button';
import { Dialog, DialogActions, DialogClose, DialogHead } from '@/components/dialog';
import { FIELD, HINT, SAID, TEXT } from '@/components/fields';
import { focusOn } from '@/components/focus-places';
import { EditIcon } from '@/components/icons';
import { getBoot } from '@/config/boot';
import { AddNameDialog, type AddNameOpen } from '@/features/tasks/components/add-name-dialog';
import { SortableList } from '@/features/tasks/components/sortable-list';
import {
  boxSaid,
  categoryRefusal,
  editBase,
  editChange,
  mergeLists,
  mergeURLs,
  pagedOf,
  refillTitle,
  urlRefusal,
  urlsFull,
  type BoxAct,
  type EditBase,
} from '@/features/tasks/utils/task-edit';
import { useHold } from '@/hooks/use-hold';
import { useStore } from '@/hooks/use-store';
import { t } from '@/lib/t';
import { pages as pagePages, type Pages } from '@/stores/pages';
import { taskWrites, type TaskWrites } from '@/stores/tasks';
import { toasts as pageToasts, type Toasts } from '@/stores/toasts';
import type { FocusBack } from '@/types/focus';
import type { TaskOf } from '@/types/task';

// What the task edit is opened for: the task as its page drew it, and the way back to its edit button
export interface TaskEditOpen {
  task: TaskOf;
  back: FocusBack;
}

export interface TaskEditDialogProps {
  readonly open: TaskEditOpen | null;
  readonly onClosed: () => void;
  readonly pages?: Pages;
  readonly writes?: Pick<TaskWrites, 'writeTask' | 'tasksOf'>;
  readonly toasts?: Toasts;
}

// What the modal holds of the task: what it opened over or last read (base), the rev it was read at, its categories
// with pages, which the x does not take off, and whether it is gone
interface Read {
  base: EditBase;
  rev: number;
  paged: string[];
  gone: boolean;
}

const ROW_BOX = 'overflow-hidden rounded-small border border-line';

// TaskEditDialog is an existing task's own fields: its title, URLs and the order of its categories, as its page alone
// writes them; the parent is the family card's. It opens over the task as read now, and saving sends what was changed
// since, with the rev it showed. Refused as stale, what was changed in it stays and the rest is as read now; a task
// gone is said, and nothing more can be saved
export function TaskEditDialog({
  open,
  onClosed,
  pages = pagePages,
  writes = taskWrites,
  toasts = pageToasts,
}: TaskEditDialogProps) {
  const hold = useHold();
  const { tasks: taken } = useStore(pages.state);
  // The last opened, which the modal shows and goes back from as it closes: open is null by then
  const [last, setLast] = useState<TaskEditOpen | null>(null);
  const [read, setRead] = useState<Read>({ base: editBase({}), rev: 0, paged: [], gone: false });
  const [title, setTitle] = useState('');
  const [urls, setURLs] = useState<string[]>([]);
  const [categories, setCategories] = useState<string[]>([]);
  // The add modal over it, and the words under each list's box (boxSaid)
  const [adding, setAdding] = useState<AddNameOpen | null>(null);
  const [urlSaid, setURLSaid] = useState('');
  const [categorySaid, setCategorySaid] = useState('');
  // What the screen reader is told of the lists' changes
  const [urlsSaid, setURLsSaid] = useState('');
  const [categoriesSaid, setCategoriesSaid] = useState('');
  const [error, setError] = useState('');
  const task = open?.task;
  // Opened over the task as the tasks read now have it, its rev and fields of one read: the page drawn before may be
  // older
  const now = (x: TaskOf) => writes.tasksOf().find((y) => y.project === x.project && y.name === x.name);
  if (open !== null && open !== last) {
    setLast(open);
    const x = now(open.task) ?? open.task;
    const base = editBase(x);
    setRead({ base, rev: pages.readRev(x.project, x.name), paged: pagedOf(x), gone: false });
    setTitle(base.title);
    setURLs(base.urls);
    setCategories(base.categories);
    setAdding(null);
    setURLSaid('');
    setCategorySaid('');
    setURLsSaid('');
    setCategoriesSaid('');
    setError('');
  }
  const max = getBoot().maxURLs;

  // The words under a list's box once act is done to it. The same words again are emptied first, so that the live
  // region says them anew
  // LIMIT: not tried with a screen reader. Emptied, the words are display:none a moment (SAID's empty:hidden), which a
  // reader may take as a region come anew rather than changed; give the words a region that stays shown if one is not
  // read twice
  const under = (set: (words: string) => void, was: string) => (act: BoxAct) => {
    const words = boxSaid(act);
    if (words && words === was)
      flushSync(() => {
        set('');
      });
    set(words);
  };
  const urlUnder = under(setURLSaid, urlSaid);
  const categoryUnder = under(setCategorySaid, categorySaid);
  const openAdd = (kind: AddNameOpen['kind'], id: string) => {
    (kind === 'url' ? urlUnder : categoryUnder)({ act: 'open' });
    setAdding({ kind, back: () => focusOn(document.getElementById(id)) });
  };
  // Added in the add modal beside it, which its form's onInput does not hear: a refusal of saving lets go, as typing does
  const add = (typed: string) => {
    if (!read.gone) setError('');
    if (adding?.kind === 'category') {
      const next = [...categories, typed];
      setCategories(next);
      categoryUnder({ act: 'added' });
      setCategoriesSaid(t('ui_category_added', typed, next.length, next.length));
    } else {
      setURLs([...urls, typed]);
      urlUnder({ act: 'added' });
      setURLsSaid(t('ui_url_added', typed));
    }
    setAdding(null);
  };

  const save = async () => {
    if (!task) return;
    if (taken.error) {
      setError(t('ui_tasks_failed', taken.error));
      return;
    }
    const changed = editChange(read.base, { title, urls, categories });
    if (changed === null) {
      onClosed();
      return;
    }
    const done = { saved: false };
    await hold.whileHeld(async () => {
      let res;
      try {
        res = await writes.writeTask(task.project, task.name, { new: false, rev: read.rev, ...changed });
      } catch (err) {
        setError(t('ui_save_failed', err instanceof Error ? err.message : String(err)));
        return;
      }
      if (res.status === 409) {
        // Read again by the write: what was changed in it stays, over the task as read now
        const x = now(task);
        // Unreadable, whether it is gone is not known: said so, and the fields stay as they are
        const unread = pages.state.get().tasks.error;
        if (!x && unread) {
          setError(t('ui_tasks_failed', unread));
          return;
        }
        if (!x) {
          setRead({ ...read, gone: true });
          setError(t('ui_task_gone_edit'));
          return;
        }
        const base = editBase(x);
        setTitle(refillTitle(title, read.base.title, base.title));
        setURLs(mergeURLs(urls, read.base.urls, base.urls));
        setCategories(mergeLists(categories, read.base.categories, base.categories));
        setRead({ base, rev: pages.readRev(x.project, x.name), paged: pagedOf(x), gone: false });
        setError(t('ui_save_failed', res.stale));
        return;
      }
      if (!res.ok) {
        setError(t('ui_save_failed', res.text));
        return;
      }
      toasts.toast(t('ui_saved'));
      await pages.load();
      done.saved = true;
    });
    if (done.saved) onClosed();
  };

  const noPages = (name: string) => !read.paged.includes(name);
  // The add modal stands before the edit modal: closed together (by script), it closes first and gives the focus back to
  // its button, which the edit modal then takes back to where it was opened from
  return (
    <>
      <AddNameDialog
        id="edit-add"
        open={open === null ? null : adding}
        name={task?.name ?? ''}
        refusal={(x) => (adding?.kind === 'category' ? categoryRefusal(x, categories) : urlRefusal(x, urls, max))}
        onAdd={add}
        onClosed={() => {
          setAdding(null);
        }}
      />
      <Dialog
        id="edit-task-dialog"
        open={open !== null}
        onClosed={onClosed}
        hold={hold}
        labelledBy="edit-task-heading"
        afterClose={() => last?.back() ?? false}
      >
        <form
          id="edit-task"
          className="grid gap-4"
          onSubmit={(e) => {
            e.preventDefault();
            void save();
          }}
          // Typing again lets a refusal go, but not that the task is gone, which no typing changes
          onInput={() => {
            if (!read.gone) setError('');
          }}
        >
          <DialogHead
            icon={<EditIcon />}
            titleId="edit-task-heading"
            title={t('ui_task_edit')}
            lead={task?.name ?? ''}
            leadId="edit-task-target"
          />
          <label data-testid="field" className={FIELD}>
            {t('ui_task_title')}
            <input
              id="edit-task-title"
              data-testid="text"
              className={TEXT}
              autoComplete="off"
              data-autofocus=""
              value={title}
              onChange={(e) => {
                setTitle(e.currentTarget.value);
              }}
            />
          </label>
          <div data-testid="field" className={FIELD}>
            <span id="edit-urls-head">
              {t('ui_task_url')} <small className={HINT}>{t('ui_optional')}</small>
            </span>
            <div className={ROW_BOX} hidden={urls.length === 0}>
              <SortableList
                id="edit-urls"
                rows={urls}
                kind="url"
                labelledBy="edit-urls-head"
                label={{
                  up: (u) => t('ui_url_up_named', u),
                  down: (u) => t('ui_url_down_named', u),
                  drop: (u) => t('ui_url_drop_named', u),
                }}
                onMove={(next, u) => {
                  setURLs(next);
                  urlUnder({ act: 'moved' });
                  setURLsSaid(t('ui_url_moved', u, next.indexOf(u) + 1, next.length));
                }}
                onDrop={(u) => {
                  setURLs(urls.filter((x) => x !== u));
                  urlUnder({ act: 'dropped' });
                  setURLsSaid(t('ui_url_dropped', u));
                }}
                onNone={() => {
                  focusOn(document.getElementById('edit-url-plus'));
                }}
              />
            </div>
            <span className="justify-self-start">
              <Button
                id="edit-url-plus"
                onClick={() => {
                  // At the most, none more is added: said under the box, no modal opened to type in
                  if (urlsFull(urls, max)) urlUnder({ act: 'full', max });
                  else openAdd('url', 'edit-url-plus');
                }}
              >
                {t('ui_url_add')}
              </Button>
            </span>
            <p id="edit-url-said" aria-live="polite" className={SAID}>
              {urlSaid}
            </p>
            <small id="edit-urls-hint" className={HINT}>
              {t('ui_task_urls_hint', max)}
            </small>
            <p id="edit-urls-said" className="sr-only" aria-live="polite">
              {urlsSaid}
            </p>
          </div>
          <div data-testid="field" className={FIELD}>
            <span id="edit-categories-head">{t('ui_task_categories')}</span>
            <p id="edit-categories-none" hidden={categories.length > 0} className="m-0 font-normal text-muted">
              {t('ui_task_categories_none')}
            </p>
            <div className={ROW_BOX} hidden={categories.length === 0}>
              <SortableList
                id="edit-categories"
                rows={categories}
                kind="category"
                labelledBy="edit-categories-head"
                describedBy="edit-categories-hint"
                note={(name) => (noPages(name) ? t('ui_category_no_pages') : '')}
                label={{
                  up: (name) => t('ui_category_up_named', name),
                  down: (name) => t('ui_category_down_named', name),
                  drop: (name) => t(noPages(name) ? 'ui_category_drop_named' : 'ui_category_drop_paged', name),
                }}
                unpressable={(name) => !noPages(name)}
                onMove={(next, name) => {
                  setCategories(next);
                  categoryUnder({ act: 'moved' });
                  setCategoriesSaid(t('ui_category_moved', name, next.indexOf(name) + 1, next.length));
                }}
                onDrop={(name) => {
                  // A category with pages is not deleted, as the store refuses to leave one out: its x says why
                  if (!noPages(name)) {
                    categoryUnder({ act: 'paged', name });
                    return;
                  }
                  setCategories(categories.filter((x) => x !== name));
                  categoryUnder({ act: 'dropped' });
                  setCategoriesSaid(t('ui_category_dropped', name));
                }}
                onNone={() => {
                  focusOn(document.getElementById('edit-category-plus'));
                }}
              />
            </div>
            <span className="justify-self-start">
              <Button
                id="edit-category-plus"
                label={t('ui_category_add')}
                onClick={() => {
                  openAdd('category', 'edit-category-plus');
                }}
              >
                {t('ui_category_add')}
              </Button>
            </span>
            <p id="edit-category-said" aria-live="polite" className={SAID}>
              {categorySaid}
            </p>
            <small id="edit-categories-hint" hidden={categories.length === 0} className={HINT}>
              {t('ui_task_categories_hint')}
            </small>
            <p id="edit-categories-said" className="sr-only" aria-live="polite">
              {categoriesSaid}
            </p>
          </div>
          <p
            id="edit-task-error"
            role="alert"
            hidden={!error}
            className="m-0 rounded-menu bg-danger-soft px-3.5 py-2.5 whitespace-pre-wrap text-danger"
          >
            {error}
          </p>
          <DialogActions>
            <DialogClose words="ui_cancel" />
            <Button id="edit-task-submit" kind="primary" type="submit" disabled={read.gone}>
              {t('ui_save')}
            </Button>
          </DialogActions>
        </form>
      </Dialog>
    </>
  );
}
