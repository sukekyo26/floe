import { BackIcon } from '@/components/icons';
import { backWords } from '@/components/back-words';
import type { Back } from '@/utils/back';
import { onPlainPress } from '@/utils/plain-press';

// BackLink is the way back from a page whole to the task's page it was opened from, which the header's links are not
// (the user's choice): the task's name, and in full where it goes; a narrow screen has its mark alone. A plain link,
// its words alone darker under the pointer. Opened from either list, the header's link to it is the way back, and
// this is not drawn
export function BackLink({ back, onPress }: { readonly back: Back; readonly onPress: () => void }) {
  if (back.to !== 'task') return null;
  const whole = backWords(back);
  return (
    <a
      id="back"
      href={back.href}
      aria-label={whole}
      title={whole}
      onClick={onPlainPress(onPress)}
      className="inline-flex h-icon-btn min-w-icon-btn flex-none items-center justify-center gap-1.5 px-1.5 text-muted no-underline transition-colors duration-150 hover:text-fg [&>svg]:size-4 [&>svg]:flex-none"
    >
      <BackIcon />
      <span id="back-word" className="max-w-48 truncate mid:hidden">
        {back.task}
      </span>
    </a>
  );
}
