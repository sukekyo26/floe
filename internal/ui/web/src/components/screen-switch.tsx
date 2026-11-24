import { useStore } from '@/hooks/use-store';
import { t } from '@/lib/t';
import { listQuery as pageQuery, type ListQuery } from '@/stores/list-query';
import type { Screen } from '@/types/screen';
import { onPlainPress } from '@/utils/plain-press';

// look is a link's in the switch, the one of the screen shown on the thumb: a white ground and a shadow that lifts it off
// the track, which the other has neither of. Both are of one size and one weight, so that a switch moves none, and no
// frame is drawn (the user's choice). The words are muted where there is a thumb, to set them back; with none, on a page
// whole, both are dark (the user's choice)
// LIMIT: the thumb is told by its ground, the words' colour and the shadow alone: with forced colours they are gone, and
// aria-current alone tells the screen. Draw it some other way if people use floe so
const look = (now: boolean, any: boolean): string =>
  `inline-flex h-icon-btn items-center rounded-full px-3 text-small font-semibold whitespace-nowrap no-underline transition-colors duration-150 tiny:px-1.5 ${now ? 'bg-frame-bg text-fg shadow-sm' : any ? 'text-muted hover:text-fg' : 'text-fg'}`;

// A link that is a way back: where it goes, and what a plain press keeps, the row to come back to (S450's task list
// on a task's page, the frame's link to where it was opened from)
export interface Up {
  readonly href: string;
  readonly onPress: () => void;
}

// ScreenSwitch is the bar's way between the two screens: links, as each goes to a URL of its own; the screen shown is
// aria-current, and a page whole is neither (the user's choice), whose bar has its back link alone at 40rem or less.
// The track shows the room a press has: that of the bar's buttons of icons, not the bar's height. Its padding holds the
// focus ring of each
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
      className={`flex flex-none gap-0.5 self-center rounded-full bg-line p-1 ${screen === 'frame' ? 'narrow:hidden' : ''}`}
    >
      <a
        id="to-list"
        href={listUp?.href ?? `/${query.savedList()}`}
        onClick={onPlainPress(listUp?.onPress)}
        aria-current={pages ? 'page' : undefined}
        className={look(pages, pages || tasks)}
      >
        {t('ui_screen_pages')}
      </a>
      <a
        id="to-tasks"
        href={taskUp?.href ?? `/tasks${query.savedTasks()}`}
        onClick={onPlainPress(taskUp?.onPress)}
        aria-current={tasks ? 'page' : undefined}
        className={look(tasks, pages || tasks)}
      >
        {t('ui_screen_tasks')}
      </a>
    </nav>
  );
}
