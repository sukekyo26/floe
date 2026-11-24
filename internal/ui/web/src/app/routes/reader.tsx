import type { ReactNode } from 'react';

import { BarActions } from '@/components/page-actions';
import type { Openers } from '@/components/page-menu';
import { ReaderPane } from '@/features/reader/components/reader-pane';
import { TagLine } from '@/features/tags/components/tag-line';
import { VersionNote } from '@/features/versions/components/version-note';
import { VersionSelect } from '@/features/versions/components/version-select';
import { useStore } from '@/hooks/use-store';
import { openPage } from '@/stores/open-page';
import { order } from '@/stores/order';
import { pages } from '@/stores/pages';
import type { Page } from '@/types/page';
import { orderOf } from '@/utils/row-press';

// keepDrawn keeps the rows of #list as drawn now for j and k on the page whole and its way back: a folder's own rows
// (its tree) left out
export function keepDrawn(): void {
  const { entries } = pages.state.get();
  const drawn = [...document.querySelectorAll<HTMLElement>('#list > li[data-slug]:not([data-testid=tree])')]
    .map((li) => entries.find((a) => a.slug === li.dataset['slug']))
    .filter((a): a is Page => a !== undefined);
  const { order: slugs, shown } = orderOf(drawn);
  order.keepRows(slugs, shown);
}

interface PageReaderProps {
  readonly openers: Openers;
  readonly keys: ReadonlySet<string>;
  readonly onKey: (key: string) => void;
  // what is the list's narrowing in words, in the head with the list folded (the page list's); none, null
  readonly what?: ReactNode;
}

// PageReader is the page picked beside the list, with its bar, its version, its note of versions and its tags. The
// choice of versions and its note wait for the list read once (P7: a page the list does not hold yet is not one gone)
export function PageReader({ openers, keys, onKey, what = null }: PageReaderProps) {
  const at = useStore(openPage.picked);
  const { entries, read } = useStore(pages.state);
  const page = at ? (entries.find((a) => a.slug === at.slug) ?? null) : null;
  return (
    <ReaderPane
      at={at}
      page={page}
      actions={
        page && at ? (
          <BarActions id="read-actions" place="list" page={page} at={at} openers={openers} onDiffOpen={keepDrawn} />
        ) : null
      }
      version={read ? <VersionSelect at={at} page={page} /> : null}
      versionNote={read ? <VersionNote at={at} page={page} onGo={keepDrawn} /> : null}
      tags={
        <TagLine
          id="read-tags"
          page={page}
          hidden={!page || Boolean(at?.file) || Boolean(page.missing)}
          onEdit={openers.onTags}
        />
      }
      what={what}
      keys={keys}
      onKey={onKey}
      onTitle={keepDrawn}
    />
  );
}
