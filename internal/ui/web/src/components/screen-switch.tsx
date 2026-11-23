import { tabLook } from '@/components/tabs';
import { t } from '@/lib/t';

// ScreenSwitch is the bar's way between the two screens: links, as each goes to a URL of its own, in the look of the
// help's tabs (the user's choice, S422-P8); the screen shown is aria-current
export function ScreenSwitch({ tasks }: { readonly tasks: boolean }) {
  return (
    <nav id="screens" aria-label={t('ui_screens')} className="flex flex-none gap-x-5 self-stretch">
      <a
        id="to-list"
        href="/"
        aria-current={tasks ? undefined : 'page'}
        className={`flex items-center no-underline ${tabLook(!tasks)}`}
      >
        {t('ui_screen_pages')}
      </a>
      <a
        id="to-tasks"
        href="/tasks"
        aria-current={tasks ? 'page' : undefined}
        className={`flex items-center no-underline ${tabLook(tasks)}`}
      >
        {t('ui_screen_tasks')}
      </a>
    </nav>
  );
}
