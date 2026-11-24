import { useStore } from '@/hooks/use-store';
import { t } from '@/lib/t';
import { listQuery as pageQuery, type ListQuery } from '@/stores/list-query';
import type { Screen } from '@/types/screen';
import { onPlainPress } from '@/utils/plain-press';

// look is a link's: both of one weight, heavier than the bar's words, the screen shown darker by its colour alone, so that
// a switch moves none; no line under it (the user's choice, S448)
const look = (now: boolean): string =>
  `flex items-center px-0.5 text-small font-semibold whitespace-nowrap no-underline transition-colors duration-150 ${now ? 'text-fg' : 'text-muted hover:text-fg'}`;

// A link that is a way back: where it goes, and what a plain press keeps, the row to come back to (S450's task list
// on a task's page, the frame's link to where it was opened from)
export interface Up {
  readonly href: string;
  readonly onPress: () => void;
}

// ScreenSwitch is the bar's way between the two screens: links, as each goes to a URL of its own; the screen shown is
// aria-current, and a page whole is neither (the user's choice), whose bar has its back link alone at 40rem or less.
// Each is as high as the bar, its words in the bar's middle
// Each goes back to the list as this tab last had it, read again as the list's query is kept, or where its up says: on
// a task's page, the task list's (S450), and in a frame, the one it was opened from
export function ScreenSwitch({
  screen,
  listUp,
  taskUp,
  query = pageQuery,
}: {
  readonly screen: Screen;
  readonly listUp?: Up | undefined;
  readonly taskUp?: Up | undefined;
  readonly query?: ListQuery;
}) {
  useStore(query.synced);
  const pages = screen === 'list';
  const tasks = screen === 'taskList' || screen === 'taskPage';
  return (
    <nav
      id="screens"
      aria-label={t('ui_screens')}
      className={`flex flex-none gap-x-5 self-stretch ${screen === 'frame' ? 'narrow:hidden' : ''}`}
    >
      <a
        id="to-list"
        href={listUp?.href ?? `/${query.savedList()}`}
        onClick={onPlainPress(listUp?.onPress)}
        aria-current={pages ? 'page' : undefined}
        className={look(pages)}
      >
        {t('ui_screen_pages')}
      </a>
      <a
        id="to-tasks"
        href={taskUp?.href ?? `/tasks${query.savedTasks()}`}
        onClick={onPlainPress(taskUp?.onPress)}
        aria-current={tasks ? 'page' : undefined}
        className={look(tasks)}
      >
        {t('ui_screen_tasks')}
      </a>
    </nav>
  );
}
