import { useState, type KeyboardEvent } from 'react';

import { Button } from '@/components/button';
import { Dialog, DialogActions, DialogClose, DialogHead } from '@/components/dialog';
import { FIELD, HINT, TEXT } from '@/components/fields';
import { drawThenFocus, focusOn } from '@/components/focus-places';
import { EditIcon } from '@/components/icons';
import { getBoot } from '@/config/boot';
import { SortableList } from '@/features/tasks/components/sortable-list';
import {
  categoryRefusal,
  editBase,
  editChange,
  leftOver,
  mergeLists,
  mergeURLs,
  pagedOf,
  refillTitle,
  urlRefusal,
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
import { composing } from '@/utils/keys';

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

// A field that adds a row (a URL, a category): shown or the button that turns into it, what is typed, and why it was
// not added
interface Adding {
  on: boolean;
  typed: string;
  said: string;
}

const OFF: Adding = { on: false, typed: '', said: '' };

const ROW_BOX = 'overflow-hidden rounded-small border border-line';
// As high as its button, the line over it not counted, so that it keeps its height as it turns into the field
const ADD_ROW = 'box-content flex min-h-[2.6rem] items-center bg-panel';
const ADD_FIELD =
  'min-w-0 flex-1 self-stretch border-0 bg-panel px-[0.6rem] text-body font-normal text-fg placeholder:text-faint focus-visible:-outline-offset-2';
const SAID = 'm-0 font-normal text-danger empty:hidden';

// TaskEditDialog is an existing task's own fields (ui.html #edit-task-dialog, openTaskEdit, saveTaskEdit): its title,
// URLs and the order of its categories, as its page alone writes them; the parent is the family card's. It opens over
// the task as read now, and saving sends what was changed since, with the rev it showed. Refused as stale, what was
// changed in it stays and the rest is as read now; a task gone is said, and nothing more can be saved
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
  const [url, setURL] = useState<Adding>(OFF);
  const [category, setCategory] = useState<Adding>(OFF);
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
    setURL(OFF);
    setCategory(OFF);
    setURLsSaid('');
    setCategoriesSaid('');
    setError('');
  }
  const max = getBoot().maxURLs;

  const field = (id: string) => document.getElementById(id) as HTMLInputElement | null;
  const addKeys =
    (which: 'url' | 'category', add: () => void, off: () => void) => (e: KeyboardEvent<HTMLInputElement>) => {
      if (composing(e.nativeEvent)) return;
      if (e.key === 'Enter') {
        e.preventDefault();
        add();
      } else if (e.key === 'Escape') {
        // The field turns back to the button, the modal staying open
        e.preventDefault();
        e.stopPropagation();
        drawThenFocus(off, which === 'url' ? 'edit-url-plus' : 'edit-category-plus');
      }
    };
  // The field turned back to the button: empty, and no longer marked
  const urlOff = () => {
    field('edit-url-new')?.setCustomValidity('');
    setURL(OFF);
  };
  const categoryOff = () => {
    field('edit-category-new')?.setCustomValidity('');
    setCategory(OFF);
  };
  const addURL = () => {
    const typed = url.typed.trim();
    field('edit-url-new')?.setCustomValidity('');
    const why = urlRefusal(typed, urls, max);
    setURL({ ...url, said: why });
    if (why) return;
    setURLs([...urls, typed]);
    setURL({ on: true, typed: '', said: '' });
    setURLsSaid(t('ui_url_added', typed));
  };
  const addCategory = () => {
    const typed = category.typed.trim();
    field('edit-category-new')?.setCustomValidity('');
    const why = categoryRefusal(typed, categories);
    setCategory({ ...category, said: why });
    if (why) return;
    const next = [...categories, typed];
    setCategories(next);
    setCategory({ on: true, typed: '', said: '' });
    setCategoriesSaid(t('ui_category_added', typed, next.length, next.length));
  };

  const save = async () => {
    if (!task) return;
    // Words typed and not added are not dropped unsaid
    for (const [id, adding, names, isURL] of [
      ['edit-category-new', category, categories, false],
      ['edit-url-new', url, urls, true],
    ] as const) {
      const left = adding.on ? leftOver(adding.typed.trim(), names, isURL) : '';
      if (!left) continue;
      const input = field(id);
      input?.setCustomValidity(left);
      input?.reportValidity();
      return;
    }
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
  return (
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
        // Typing again lets a refusal go, but not that the task is gone, which no typing changes; a field marked
        // blank is free again as it is typed in
        onInput={(e) => {
          if (e.target instanceof HTMLInputElement) e.target.setCustomValidity('');
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
          <div className={ROW_BOX}>
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
                setURLsSaid(t('ui_url_moved', u, next.indexOf(u) + 1, next.length));
              }}
              onDrop={(u) => {
                setURLs(urls.filter((x) => x !== u));
                setURLsSaid(t('ui_url_dropped', u));
              }}
              onNone={() => {
                focusOn(document.getElementById(url.on ? 'edit-url-new' : 'edit-url-plus'));
              }}
            />
            <div className={`${ADD_ROW} ${urls.length > 0 ? 'border-t border-line' : ''}`}>
              <span hidden={url.on} className="flex px-[0.4rem] py-[0.3rem]">
                <Button
                  id="edit-url-plus"
                  hidden={url.on}
                  onClick={() => {
                    drawThenFocus(() => {
                      setURL({ ...OFF, on: true });
                    }, 'edit-url-new');
                  }}
                >
                  {t('ui_url_add')}
                </Button>
              </span>
              <input
                id="edit-url-new"
                data-testid="path"
                hidden={!url.on}
                className={`${ADD_FIELD} font-mono`}
                autoComplete="off"
                spellCheck={false}
                aria-label={t('ui_url_add')}
                placeholder={t('ui_url_new_hint')}
                aria-describedby="edit-url-said"
                value={url.typed}
                onChange={(e) => {
                  setURL({ ...url, typed: e.currentTarget.value });
                }}
                onKeyDown={addKeys('url', addURL, () => {
                  urlOff();
                })}
                // Left empty, the field is the button again; with words in it, it stays, for saving to say so
                onBlur={(e) => {
                  if (!e.currentTarget.value.trim()) urlOff();
                }}
              />
            </div>
          </div>
          <p id="edit-url-said" aria-live="polite" className={SAID}>
            {url.said}
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
          <div className={ROW_BOX}>
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
                setCategoriesSaid(t('ui_category_moved', name, next.indexOf(name) + 1, next.length));
              }}
              onDrop={(name) => {
                // A category with pages is not deleted, as the store refuses to leave one out: its x says why
                if (!noPages(name)) {
                  setCategory({ ...category, said: t('ui_category_drop_paged', name) });
                  return;
                }
                setCategories(categories.filter((x) => x !== name));
                setCategoriesSaid(t('ui_category_dropped', name));
              }}
              onNone={() => {
                focusOn(document.getElementById(category.on ? 'edit-category-new' : 'edit-category-plus'));
              }}
            />
            <div className={`${ADD_ROW} ${categories.length > 0 ? 'border-t border-line' : ''}`}>
              <span hidden={category.on} className="flex px-[0.4rem] py-[0.3rem]">
                <Button
                  id="edit-category-plus"
                  hidden={category.on}
                  label={t('ui_category_add')}
                  onClick={() => {
                    drawThenFocus(() => {
                      setCategory({ ...OFF, on: true });
                    }, 'edit-category-new');
                  }}
                >
                  {t('ui_category_add')}
                </Button>
              </span>
              <input
                id="edit-category-new"
                data-testid="text"
                hidden={!category.on}
                className={ADD_FIELD}
                autoComplete="off"
                aria-label={t('ui_category_name')}
                placeholder={t('ui_category_name_hint')}
                aria-describedby="edit-category-said"
                value={category.typed}
                onChange={(e) => {
                  setCategory({ ...category, typed: e.currentTarget.value });
                }}
                onKeyDown={addKeys('category', addCategory, () => {
                  categoryOff();
                })}
                onBlur={(e) => {
                  if (!e.currentTarget.value.trim()) categoryOff();
                }}
              />
            </div>
          </div>
          <p id="edit-category-said" aria-live="polite" className={SAID}>
            {category.said}
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
  );
}
