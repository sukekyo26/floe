import { useRef, type ReactNode } from 'react';

import { unreadFocus } from '@/components/focus-places';
import { PageNote } from '@/features/reader/components/page-note';
import { usePageMessages } from '@/hooks/use-page-messages';
import { useReplacedFrame } from '@/features/reader/hooks/use-replaced-frame';
import { pageReads as sharedReads, type PageReads } from '@/features/reader/stores/page-reads';
import { t } from '@/lib/t';
import { pageNote as sharedNote, type PageNote as Notes } from '@/stores/page-note';
import type { OpenAt } from '@/stores/open-page';
import type { Page } from '@/types/page';
import { pageHref, rawHref } from '@/utils/page-urls';

interface ReaderPaneProps {
  // at is the row's page open beside the list (picked), page its entry as listed; null for none
  readonly at: OpenAt | null;
  readonly page: Page | null;
  // actions are the bar's (BarActions), version the versions' select (VersionSelect) and versionNote its words
  // (VersionNote), tags the tag line (TagLine)
  readonly actions?: ReactNode;
  readonly version?: ReactNode;
  readonly versionNote?: ReactNode;
  readonly tags?: ReactNode;
  // what is the list's narrowing in words, shown in the head with the list folded (P9's), null for none
  readonly what?: ReactNode;
  // keys are the screen's table of keys (a page passes those of PAGE_KEYS alone); onKey presses one
  readonly keys: ReadonlySet<string>;
  readonly onKey: (key: string) => void;
  // onTitle is what pressing the title does first (the list's order goes on with j and k)
  readonly onTitle?: () => void;
  readonly reads?: PageReads;
  readonly note?: Notes;
}

// ReaderPane is the page open beside the list: its head, its tags, and the page in an iframe loaded in place of its
// history entry, or the words of none chosen or of a link's file gone. It is always there: drawn anew, its iframe would
// load the page again
export function ReaderPane({
  at,
  page: a,
  actions,
  version,
  versionNote,
  tags,
  what = null,
  keys,
  onKey,
  onTitle,
  reads = sharedReads,
  note = sharedNote,
}: ReaderPaneProps) {
  const frame = useRef<HTMLIFrameElement>(null);
  const open = a !== null && at !== null && !a.missing;
  // Loaded only while it is there, as the frame's, so that a link's missing file is no 404 kept when the file is back
  const href = open ? rawHref(at) : 'about:blank';
  useReplacedFrame(frame, href, () => {
    reads.opened(at?.slug);
  });
  usePageMessages(frame, 'beside', {
    givenPath: () => (at ? new URL(rawHref(at), window.location.href).pathname : null),
    keys,
    onKey,
    onServed: (mark) => {
      if (at) void reads.served(at.slug, mark, unreadFocus);
    },
  });
  // A folder's file open is named by its file, any other page by its title
  const file = at?.file ?? '';
  const name = file === '' ? (a?.title ?? '') : file;
  return (
    <section id="reader" className="flex min-h-0 min-w-0 flex-col mid:hidden">
      <div
        id="read-head"
        hidden={!a && what === null}
        className={`flex h-lhead flex-none items-center gap-2.5 border-b border-line px-4 [&>#shelf-what]:max-w-[calc(100%-11rem)] ${a ? '' : 'narrow:hidden'}`}
      >
        {/* The filters' words leave room for the bar's buttons, however long */}
        {what}
        <a
          id="read-title"
          hidden={!a}
          href={at ? pageHref(at) : undefined}
          title={name}
          onClick={onTitle}
          className="min-w-0 truncate font-bold text-fg underline decoration-transparent hover:decoration-current"
        >
          {name}
        </a>
        {version}
        {/* The bar is BarActions, of its own id (read-actions); a box of none puts it on the right */}
        {a ? <span className="ms-auto">{actions}</span> : null}
      </div>
      {tags}
      {versionNote}
      <PageNote slug={at?.slug} note={note} {...(onTitle ? { onGo: onTitle } : {})} />
      {/* No sandbox attribute: the server sandboxes the page by its header (ui.md), and allow lets its copy and
          fullscreen buttons work */}
      {/* oxlint-disable-next-line react/iframe-missing-sandbox -- sandboxed by the server's header */}
      <iframe
        ref={frame}
        id="read-frame"
        // Before any page: useReplacedFrame writes each one loaded after
        data-loaded="about:blank"
        title={name}
        data-slug={at?.slug}
        data-file={file === '' ? undefined : file}
        data-version={at ? String(at.v) : undefined}
        allow="clipboard-write *; fullscreen *"
        hidden={!open}
        className="min-h-0 w-full flex-1 border-0 bg-frame-bg"
      />
      <p
        id="read-blank"
        hidden={open}
        className="m-0 grid flex-1 place-content-center p-6 text-center whitespace-pre-line text-muted"
      >
        {a?.missing ? t('ui_link_missing', a.path ?? '') : t('ui_read_blank')}
      </p>
    </section>
  );
}
