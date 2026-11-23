import { tabLook } from '@/components/tabs';
import { useStore } from '@/hooks/use-store';
import { t } from '@/lib/t';
import { listQuery as pageQuery, type ListQuery } from '@/stores/list-query';

// ScreenSwitch is the bar's way between the two screens: links, as each goes to a URL of its own, in the look of the
// help's tabs (the user's choice, S422-P8); the screen shown is aria-current. Each is as high as the bar, its line on the
// bar's: a line as thick above, unseen, keeps its words in the bar's middle
// Each goes back to the list as this tab last had it (ui.html drawScreens): read as the query changes
export function ScreenSwitch({ tasks, query = pageQuery }: { readonly tasks: boolean; readonly query?: ListQuery }) {
  useStore(query.query);
  return (
    <nav id="screens" aria-label={t('ui_screens')} className="flex flex-none gap-x-5 self-stretch">
      <a
        id="to-list"
        href={`/${query.savedList()}`}
        aria-current={tasks ? undefined : 'page'}
        className={`-mb-px flex items-center border-t-2 border-t-transparent px-0.5 no-underline ${tabLook(!tasks)}`}
      >
        {t('ui_screen_pages')}
      </a>
      <a
        id="to-tasks"
        href={`/tasks${query.savedTasks()}`}
        aria-current={tasks ? 'page' : undefined}
        className={`-mb-px flex items-center border-t-2 border-t-transparent px-0.5 no-underline ${tabLook(tasks)}`}
      >
        {t('ui_screen_tasks')}
      </a>
    </nav>
  );
}
