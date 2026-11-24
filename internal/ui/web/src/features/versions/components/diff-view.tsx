import { useEffect, useRef, type CSSProperties } from 'react';

import { Button } from '@/components/button';
import { CloseIcon, PageMark } from '@/components/icons';
import { DiffCount } from '@/features/versions/components/diff-count';
import { DiffPicks } from '@/features/versions/components/diff-picks';
import { DiffToc } from '@/features/versions/components/diff-toc';
import { diff as sharedDiff, type Diff } from '@/features/versions/stores/diff';
import { diffToc as sharedToc } from '@/features/versions/stores/diff-toc';
import { frameSrc } from '@/features/versions/utils/diff-words';
import { usePageMessages } from '@/hooks/use-page-messages';
import { useStore } from '@/hooks/use-store';
import { useWindowWidth } from '@/hooks/use-window-width';
import { t } from '@/lib/t';
import type { KeptSide } from '@/stores/kept-side';
import type { Page } from '@/types/page';
import { clampSide, shownSide } from '@/utils/edge';
import { pageHref } from '@/utils/page-urls';

interface DiffViewProps {
  // page is the frame's page as the list of pages has it, once that list was read; hidden, the frame shows none
  readonly page: Page | null;
  readonly hidden: boolean;
  // keys are the screen's table of keys (a page passes those of PAGE_KEYS alone); onKey presses one
  readonly keys: ReadonlySet<string>;
  readonly onKey: (key: string) => void;
  readonly diff?: Diff;
  readonly toc?: KeptSide;
}

// DiffView is what changed between two versions of the frame's page: the pair, what the answer counts, the contents,
// and the later version's page marked in an iframe, in the sandbox as any page. Its head, columns and iframe are always
// there, hidden while there is nothing to compare: the iframe is listened to as it is first drawn
export function DiffView({ page, hidden, keys, onKey, diff = sharedDiff, toc = sharedToc }: DiffViewProps) {
  const state = useStore(diff.state);
  const side = useStore(toc.side());
  const innerWidth = useWindowWidth();
  const box = useRef<HTMLElement>(null);
  const frame = useRef<HTMLIFrameElement>(null);
  const slug = page?.slug ?? '';
  useEffect(() => {
    if (page) diff.open(page);
  }, [diff, page]);
  // The page marked is loaded anew at each answer, place and step, the same place too: what n, p or the contents moved
  // to is a move within it. None before an answer: the latest's page would be told read
  const { answer, at, turn } = state;
  useEffect(() => {
    const src = frameSrc(slug, answer, at);
    if (src !== null && frame.current) frame.current.src = src;
  }, [slug, answer, at, turn]);
  // Of the one that scrolls at a phone's width, all of it: back to its top as an answer comes, not at a place
  useEffect(() => {
    if (box.current) box.current.scrollTop = 0;
  }, [answer]);
  usePageMessages(frame, 'diff', {
    shown: () => !hidden,
    givenPath: () => {
      const src = frame.current?.getAttribute('src');
      return src ? new URL(src, window.location.href).pathname : null;
    },
    keys,
    onKey,
    onChanged: () => {
      if (page) diff.reopen(page);
    },
  });
  const close = slug ? pageHref({ slug }) : undefined;
  // The contents' width within bounds, which their grip tells folded too; the columns give them none folded
  const tocWidth = clampSide(side.width, innerWidth);
  const tocShown = shownSide(side.open, side.width, innerWidth);
  return (
    <main
      ref={box}
      id="diff"
      hidden={hidden}
      className="flex min-h-0 flex-1 flex-col bg-frame-bg print:hidden narrow:block narrow:overflow-auto"
    >
      <div
        id="diff-head"
        hidden={state.none}
        className="flex flex-none flex-wrap items-center gap-x-5 gap-y-2 border-b border-line bg-bg px-4 py-2.5 text-small narrow:px-3"
      >
        <DiffPicks slug={slug} list={state.list} pair={state.pair} onChoose={diff.choose} />
        <DiffCount counts={answer?.counts ?? null} />
        <div className="ms-auto flex flex-wrap gap-x-3 gap-y-1.5 narrow:ms-0">
          {close ? (
            <Button id="diff-close" href={close} label={t('ui_diff_close_title')}>
              <CloseIcon />
              {t('ui_close')}
            </Button>
          ) : null}
        </div>
      </div>
      <p
        id="diff-url-note"
        hidden={!state.bad}
        className="m-0 flex flex-none flex-wrap items-center gap-x-3 gap-y-1 border-b border-line bg-new-soft px-4 py-1.75 text-small text-fg narrow:px-3"
      >
        {t('ui_diff_bad_url')}
      </p>
      <div
        id="diff-main"
        hidden={state.none}
        style={{ '--toc': `${String(tocShown)}px` } as CSSProperties}
        className="relative grid min-h-0 flex-1 grid-cols-[var(--toc)_minmax(0,1fr)] bg-panel mid:grid-cols-1"
      >
        <DiffToc
          sections={answer?.sections ?? []}
          section={state.section}
          onPlace={diff.place}
          width={tocWidth}
          innerWidth={innerWidth}
          toc={toc}
        />
        <div
          id="diff-body"
          className={`col-start-2 flex min-w-0 border-l border-line bg-frame-bg narrow:h-[85dvh] mid:col-auto mid:border-l-0 ${side.open ? '' : 'not-mid:border-l-0 not-mid:pb-foot'}`}
        >
          {/* No sandbox attribute: the server sandboxes the page by its header (ui.md) */}
          {/* oxlint-disable-next-line react/iframe-missing-sandbox -- sandboxed by the server's header */}
          <iframe
            ref={frame}
            id="diff-frame"
            title={t('ui_diff_frame')}
            allow="clipboard-write *; fullscreen *"
            className="min-w-0 flex-1 border-0 bg-frame-bg"
          />
        </div>
      </div>
      <div
        data-testid="state"
        id="diff-none"
        hidden={!state.none}
        className="grid flex-1 content-center justify-items-center gap-2 px-6 py-14 text-center text-muted"
      >
        <p className="m-0 max-w-136">{t('ui_diff_none')}</p>
        {close ? (
          <Button id="diff-page" href={close}>
            <PageMark />
            {t('ui_diff_open_page')}
          </Button>
        ) : null}
      </div>
    </main>
  );
}
