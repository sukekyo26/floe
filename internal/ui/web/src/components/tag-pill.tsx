import { t } from '@/lib/t';

export interface TagPillProps {
  readonly name: string;
  // onPress narrows the list to the tag: a button then, words alone without it (a task's page, the tag line)
  readonly onPress?: (() => void) | undefined;
}

const PILL = 'max-w-48 truncate rounded-full border-0 bg-subtle px-2 text-label text-fg';

// TagPill is a tag of a page as its row and its tag line show it
export function TagPill({ name, onPress }: TagPillProps) {
  return onPress ? (
    <button
      type="button"
      data-testid="tagpill"
      title={name}
      aria-label={t('ui_tag_filter_named', name)}
      onClick={onPress}
      className={`${PILL} hover:shadow-[inset_0_0_0_1px_var(--color-fg)]`}
    >
      {name}
    </button>
  ) : (
    <span data-testid="tagpill" title={name} className={PILL}>
      {name}
    </span>
  );
}
