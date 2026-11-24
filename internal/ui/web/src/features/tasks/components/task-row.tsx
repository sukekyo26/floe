import { OpenIcon } from '@/components/icons';
import { ReadMark } from '@/components/page-row';
import { RowCard } from '@/components/row-card';
import { NEW_PILL } from '@/components/row-sub';
import { taskRowKey, unreadOf, urlEnd } from '@/features/tasks/utils/task-list';
import { goTo } from '@/lib/platform';
import { projectWord, t } from '@/lib/t';
import type { TaskOf } from '@/types/task';
import { taskHref } from '@/utils/page-urls';
import { composing } from '@/utils/keys';
import { enterOpens } from '@/utils/row-press';

// A task's name, before its title: muted but on a row unread
export const TNAME = 'font-mono text-small font-semibold text-muted group-data-unread/row:text-fg';

export interface XLinkProps {
  readonly url: string;
  readonly words: string;
}

// XLink is the way to a task's other place, above the row's own hit area: by words, the URL whole in its title when
// they are not all of it
export function XLink({ url, words }: XLinkProps) {
  return (
    <a
      data-testid="xlink"
      href={url}
      // oxlint-disable-next-line react/jsx-no-target-blank -- noopener is all a page of a modern browser needs; the referrer is the task's own site's to see
      target="_blank"
      rel="noopener"
      title={words === url ? undefined : url}
      className="relative z-1 inline-flex max-w-full items-start gap-[0.2rem] font-mono text-label wrap-anywhere text-muted underline underline-offset-2 hover:text-fg [&>svg]:mt-[0.2em] [&>svg]:size-3 [&>svg]:flex-none"
    >
      <OpenIcon />
      <span>{words}</span>
    </a>
  );
}

// TaskKin is what a row of the list says of the task's family, the one place that draws it: words, not links
function TaskKin({ task }: { readonly task: TaskOf }) {
  return (
    <>
      {task.parentTask ? (
        <span data-testid="kin" className="inline-flex max-w-full min-w-0 items-baseline gap-[0.3rem] text-muted">
          <span>{t('ui_task_parent_word')}</span>
          <span data-testid="tname" className={`m-0 font-mono text-small font-medium text-muted`}>
            {task.parentTask.name}
          </span>
          <span data-testid="kin-title" className="max-w-44 min-w-0 truncate">
            {task.parentTask.title ?? ''}
          </span>
        </span>
      ) : null}
      {task.children.length > 0 ? (
        <span data-testid="task-pages" className="whitespace-nowrap">
          {t('ui_task_kids', task.children.length)}
        </span>
      ) : null}
    </>
  );
}

export interface TaskRowProps {
  readonly task: TaskOf;
  // index is its place in the list, which staggers its rise
  readonly index: number;
  // kept: drawn before, so not risen in again; fresh: just made
  readonly kept: boolean;
  readonly fresh: boolean;
  readonly assign?: (url: string) => void;
}

// TaskRow is a row of a task on the task list, of the page's own form with nothing to select or do but open it and its
// links. Enter on its title opens it, from the page's keys too (a key sent by a script, which opens no link of itself),
// as on a page's row; it keeps no order of rows, as no page is of it
export function TaskRow({ task, index, kept, fresh, assign = goTo }: TaskRowProps) {
  const unread = unreadOf(task);
  const href = taskHref(task);
  return (
    <RowCard
      rowKey={taskRowKey(task.project, task.name)}
      index={index}
      unread={unread > 0}
      kept={kept}
      fresh={fresh}
      lead={<ReadMark unread={unread > 0} />}
      title={{
        href,
        words: (
          <>
            <span data-testid="tname" className={`me-[0.45rem] ${TNAME}`}>
              {task.name}
            </span>
            {task.title ?? ''}
          </>
        ),
        tip: [task.name, task.title, projectWord(task.project), ...(task.urls ?? [])].filter(Boolean).join('\n'),
        onKeyDown: (e) => {
          const opens = enterOpens(e, { hasHref: true, expandable: false, composing: composing(e.nativeEvent) });
          if (!opens) return;
          e.preventDefault();
          assign(href);
        },
      }}
      time={task.at}
      sub={
        <>
          <span data-testid="task-pages" className="whitespace-nowrap">
            {t('ui_task_pages', task.pages.length)}
          </span>
          {task.archived.length > 0 ? (
            <span data-testid="task-pages" className="whitespace-nowrap">
              {t('ui_task_archived', task.archived.length)}
            </span>
          ) : null}
          {unread > 0 ? (
            <span data-testid="ver" className={NEW_PILL}>
              {t('ui_task_unread', unread)}
            </span>
          ) : null}
          {(task.urls ?? []).map((url) => (
            <XLink key={url} url={url} words={urlEnd(url)} />
          ))}
          <TaskKin task={task} />
        </>
      }
    />
  );
}
