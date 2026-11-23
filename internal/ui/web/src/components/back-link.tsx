import { BackIcon } from '@/components/icons';
import { backWords } from '@/components/back-words';
import type { Back } from '@/utils/back';
import { onPlainPress } from '@/utils/plain-press';

// BackLink is a page whole's way back to where it was opened from (the user's choice): to a task's page, which the
// header's links are not, by the task's name, its mark alone at 56rem or less; and at 40rem or less, where the header's
// links are gone, its mark alone from anywhere. Opened from either list on a wider screen, the header's link to it goes
// the same way and this is hidden (out of the tab order and of what is read out), there for h. In full, it says
// where it goes. A plain link, its words alone darker under the pointer
export function BackLink({ back, onPress }: { readonly back: Back; readonly onPress: () => void }) {
  const whole = backWords(back);
  return (
    <a
      id="back"
      href={back.href}
      aria-label={whole}
      title={whole}
      onClick={onPlainPress(onPress)}
      className={`${back.to === 'task' ? 'inline-flex' : 'hidden narrow:inline-flex'} h-icon-btn min-w-icon-btn flex-none items-center justify-center gap-1.5 px-1.5 text-muted no-underline transition-colors duration-150 hover:text-fg [&>svg]:size-4 [&>svg]:flex-none`}
    >
      <BackIcon />
      <span id="back-word" className="max-w-48 truncate mid:hidden">
        {back.task}
      </span>
    </a>
  );
}
