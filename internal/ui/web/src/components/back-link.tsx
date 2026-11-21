import { useEffect, useState } from 'react';

import { BackIcon } from '@/components/icons';
import { t } from '@/lib/t';
import { order as pageOrder, type Order } from '@/stores/order';
import { backOf, type Back } from '@/utils/back';

const words = (back: Back): [string, string] => {
  if (back.to === 'pages') return [t('ui_screen_pages'), t('ui_to_page_list')];
  if (back.to === 'tasks') return [t('ui_screen_tasks'), t('ui_tasks_back')];
  return [back.task, t('ui_back_to_task', back.task)];
};

// BackLink is the way back from a page whole to where it was opened from, which the logo is not: its word, and in
// full its name. A narrow screen has its mark alone. A press keeps that the rows go on; a page come back to from the
// browser's memory reads where it goes anew, as what was kept may have changed since
export function BackLink({ order = pageOrder }: { readonly order?: Order }) {
  const [back, setBack] = useState(() => backOf(order.keptBack()));
  useEffect(() => {
    const shown = (e: PageTransitionEvent) => {
      if (e.persisted) setBack(backOf(order.keptBack()));
    };
    window.addEventListener('pageshow', shown);
    return () => {
      window.removeEventListener('pageshow', shown);
    };
  }, [order]);
  const [word, whole] = words(back);
  return (
    <a
      id="back"
      href={back.href}
      aria-label={whole}
      title={whole}
      onClick={(e) => {
        // A modified press is the link's, in a new tab: this tab goes nowhere
        if (!e.button && !e.ctrlKey && !e.metaKey && !e.shiftKey && !e.altKey)
          order.editKept(() => ({ pressed: true }));
      }}
      className="inline-flex h-icon-btn min-w-icon-btn flex-none items-center justify-center gap-1.5 rounded-full border border-transparent px-1.5 text-muted no-underline transition-colors duration-150 hover:bg-subtle hover:text-fg [&>svg]:size-4 [&>svg]:flex-none"
    >
      <BackIcon />
      <span id="back-word" className="max-w-48 truncate mid:hidden">
        {word}
      </span>
    </a>
  );
}
