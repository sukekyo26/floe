import { useStore } from '@/hooks/use-store';
import { t } from '@/lib/t';
import { listQuery as pageQuery, type ListQuery } from '@/stores/list-query';

// look is a link's: both of one weight, heavier than the bar's words, the screen shown darker by its colour alone, so that
// a switch moves none; no line under it (the user's choice, S448)
const look = (now: boolean): string =>
  `flex items-center px-0.5 text-small font-semibold whitespace-nowrap no-underline transition-colors duration-150 ${now ? 'text-fg' : 'text-muted hover:text-fg'}`;

// ScreenSwitch is the bar's way between the two screens: links, as each goes to a URL of its own; the screen shown is
// aria-current. Each is as high as the bar, its words in the bar's middle
// Each goes back to the list as this tab last had it (ui.html drawScreens): read again as the list's query is kept
export function ScreenSwitch({ tasks, query = pageQuery }: { readonly tasks: boolean; readonly query?: ListQuery }) {
  useStore(query.synced);
  return (
    <nav id="screens" aria-label={t('ui_screens')} className="flex flex-none gap-x-5 self-stretch">
      <a id="to-list" href={`/${query.savedList()}`} aria-current={tasks ? undefined : 'page'} className={look(!tasks)}>
        {t('ui_screen_pages')}
      </a>
      <a
        id="to-tasks"
        href={`/tasks${query.savedTasks()}`}
        aria-current={tasks ? 'page' : undefined}
        className={look(tasks)}
      >
        {t('ui_screen_tasks')}
      </a>
    </nav>
  );
}
