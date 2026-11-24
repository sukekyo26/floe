import { useEffect, useRef, useState } from 'react';

import { unreadFocus } from '@/components/focus-places';
import { pageReads as sharedReads, type PageReads } from '@/features/reader/stores/page-reads';
import { usePageMessages } from '@/hooks/use-page-messages';
import type { OpenAt } from '@/stores/open-page';
import { rawHref } from '@/utils/page-urls';

interface FramePageProps {
  // at is the page whole (frameAt), shown once found: the list read, the page there and not a diff
  readonly at: OpenAt;
  readonly found: boolean;
  readonly diff: boolean;
  // name is the iframe's title, the page's name
  readonly name: string;
  // keys are the screen's table of keys (a page passes those of PAGE_KEYS alone); onKey presses one
  readonly keys: ReadonlySet<string>;
  readonly onKey: (key: string) => void;
  readonly reads?: PageReads;
}

// FramePage is the page whole in its iframe, and what the page tells of itself. The iframe is always there, hidden till
// found and in a diff, so that the messages' load is on it from the first draw: drawn only once found, the page would
// never be told its keys are taken. Its src is given once, when found, with the URL's fragment, and never again: given
// anew, a move within the iframe and its scroll would be lost. The page's moves within itself are written into the
// URL's fragment, and the URL's into the page's (hashchange); the read is told of the page whole's own slug (ui.md),
// and its opening is the screen's, whose first read is of a page a person opened
export function FramePage({ at, found, diff, name, keys, onKey, reads = sharedReads }: FramePageProps) {
  const frame = useRef<HTMLIFrameElement>(null);
  const href = rawHref(at);
  const [src, setSrc] = useState<string | null>(null);
  // A diff's frame is not loaded, so that opening it reads nothing of the latest
  if (found && !diff && src === null) setSrc(href + window.location.hash);
  usePageMessages(frame, 'whole', {
    shown: () => found && !diff,
    givenPath: () => new URL(href, window.location.href).pathname,
    keys,
    onKey,
    onHash: (hash) => {
      window.history.replaceState(window.history.state, '', window.location.pathname + window.location.search + hash);
    },
    onServed: (mark) => {
      void reads.served(at.slug, mark, unreadFocus);
    },
  });
  // A fragment written above fires no hashchange, so the two do not echo. No fragment is the empty one, which moves to
  // the page's top: the page's URL alone would load it again
  useEffect(() => {
    if (src === null) return undefined;
    const moved = () => {
      frame.current?.contentWindow?.location.replace(href + (window.location.hash || '#'));
    };
    window.addEventListener('hashchange', moved);
    return () => {
      window.removeEventListener('hashchange', moved);
    };
  }, [src, href]);
  return (
    // No sandbox attribute: the server sandboxes the page by its header (ui.md), and allow lets its copy and fullscreen
    // buttons work
    // oxlint-disable-next-line react/iframe-missing-sandbox -- sandboxed by the server's header
    <iframe
      ref={frame}
      id="frame"
      title={name}
      src={src ?? undefined}
      data-slug={at.slug}
      data-version={String(at.v)}
      allow="clipboard-write *; fullscreen *"
      hidden={!found || diff}
      className="ms-[var(--side,0px)] block w-[calc(100%-var(--side,0px))] flex-1 border-0 bg-frame-bg print:hidden"
    />
  );
}
