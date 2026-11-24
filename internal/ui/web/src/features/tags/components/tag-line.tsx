import { useRef } from 'react';

import { focusOn } from '@/components/focus-places';
import { EditIcon, PlusIcon, TagIcon } from '@/components/icons';
import { IconButton } from '@/components/icon-button';
import { TagPill } from '@/components/tag-pill';
import { t } from '@/lib/t';
import type { FocusBack } from '@/types/focus';
import type { Page } from '@/types/page';

interface TagLineProps {
  // id is the line's: read-tags beside the list, frame-tags over a page whole
  readonly id: 'read-tags' | 'frame-tags';
  // page is the page open, null for none; hidden is where it has no tags of its own to show: a folder's file, a link's
  // file gone, a diff (its placer tells)
  readonly page: Page | null;
  readonly hidden: boolean;
  readonly onEdit: (a: Page, back: FocusBack) => void;
}

// TagLine is the tags of a page open, under the bar over it (#read-tags, #frame-tags): outside the bar, its button none
// of the bar's (ui.md). The line and its button stay through redraws, hidden rather than gone, so that the focus a press
// gave the button is there to go back to; its placer keeps it from one page to the next (no key by page)
export function TagLine({ id, page: a, hidden, onEdit }: TagLineProps) {
  const button = useRef<HTMLButtonElement>(null);
  const tags = a?.tags ?? [];
  const named = t(tags.length > 0 ? 'ui_tag_change_named' : 'ui_tag_add_named', a?.title ?? a?.slug ?? '');
  return (
    <div
      id={id}
      hidden={hidden || a === null}
      className={`flex min-h-icon-btn flex-none flex-wrap items-center gap-1.25 border-b border-line bg-panel px-4 py-1 text-small text-muted [&>svg]:size-3.5 [&>svg]:flex-none ${id === 'frame-tags' ? 'print:hidden' : ''}`}
    >
      <TagIcon />
      <span className="me-0.75">{t('ui_tags')}</span>
      {tags.map((name) => (
        <TagPill key={name} name={name} />
      ))}
      <IconButton
        ref={button}
        testid="tag-edit"
        tagged={tags.length > 0}
        label={named}
        onClick={() => {
          if (a) onEdit(a, () => focusOn(button.current));
        }}
      >
        {tags.length > 0 ? <EditIcon /> : <PlusIcon />}
      </IconButton>
    </div>
  );
}
