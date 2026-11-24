import { BackIcon } from '@/components/icons';
import type { Up } from '@/components/screen-switch';
import { onPlainPress } from '@/utils/plain-press';

export interface BackLinkProps {
  readonly id: string;
  readonly up: Up;
  // label says in full where it goes: its name and its tip
  readonly label: string;
  // word is what stands beside its mark, gone at 56rem or less; none, the mark alone
  readonly word?: string;
  // narrowOnly shows it at 40rem or less alone
  readonly narrowOnly?: boolean;
}

// BackLink is the one way back of a screen beside the header's links. A page whole's goes to where it was opened from
// (the user's choice): to a task's page, which the header's links are not, by the task's name; and at 40rem or less,
// where the header's links are gone, from anywhere. Opened from either list on a wider screen, the header's link to it
// goes the same way and this is hidden (out of the tab order and of what is read out), there for h. A task's page's is
// its mark alone (the user's choice), going as the header's task list does. A plain link, darker under the pointer
export function BackLink({ id, up, label, word = '', narrowOnly = false }: BackLinkProps) {
  return (
    <a
      id={id}
      href={up.href}
      aria-label={label}
      title={label}
      onClick={onPlainPress(up.onPress)}
      className={`${narrowOnly ? 'hidden narrow:inline-flex' : 'inline-flex'} h-icon-btn min-w-icon-btn flex-none items-center justify-center gap-1.5 px-1.5 text-muted no-underline transition-colors duration-150 hover:text-fg [&>svg]:size-4 [&>svg]:flex-none`}
    >
      <BackIcon />
      {word === '' ? null : (
        <span id={`${id}-word`} className="max-w-48 truncate mid:hidden">
          {word}
        </span>
      )}
    </a>
  );
}
