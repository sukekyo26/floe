import { useEffect, useState } from 'react';

import { Bar } from '@/app/bar';
import { frameUp } from '@/app/frame-up';
import { appKeys, frameRows, pressDiff, pressStar, pressUp } from '@/app/keys';
import { useScreenModals } from '@/app/modals';
import { BackLink } from '@/components/back-link';
import { focusOn } from '@/components/focus-places';
import { backWords } from '@/components/back-words';
import { BarActions } from '@/components/page-actions';
import { TextLink } from '@/components/text-link';
import { getBoot } from '@/config/boot';
import { SearchBox } from '@/features/page-list/components/search-box';
import { frameMoves } from '@/features/page-tree/stores/frame-moves';
import { SideTree } from '@/features/page-tree/components/side-tree';
import { FrameHead } from '@/features/reader/components/frame-head';
import { FramePage } from '@/features/reader/components/frame-page';
import { NotFound } from '@/features/reader/components/not-found';
import { PageNote } from '@/features/reader/components/page-note';
import { TagLine } from '@/features/tags/components/tag-line';
import { DiffView } from '@/features/versions/components/diff-view';
import { VersionNote } from '@/features/versions/components/version-note';
import { VersionSelect } from '@/features/versions/components/version-select';
import { diff as diffs } from '@/features/versions/stores/diff';
import { useKeys } from '@/hooks/use-keys';
import { useStore } from '@/hooks/use-store';
import { t } from '@/lib/t';
import { listQuery } from '@/stores/list-query';
import { openPage } from '@/stores/open-page';
import { order } from '@/stores/order';
import { pages } from '@/stores/pages';
import type { Page } from '@/types/page';
import { backOf } from '@/utils/back';
import { rawHref } from '@/utils/page-urls';

// FrameRoute is a page whole, /a/{slug}/: the bar of the page's own (its way back, name, version and actions), its
// tags, a folder's tree, the notes of its versions, and the page, its diff or the words of none found, with the modals
// every screen opens. Nothing of the page shows before the list is read once: drawn before, a page there would show as
// not found for a moment
export function FrameRoute() {
  const { framed, served } = getBoot();
  const at = openPage.frameAt() ?? { slug: framed, file: '', v: 0 };
  const diff = openPage.diff();
  const { entries, read } = useStore(pages.state);
  const a = entries.find((x) => x.slug === framed) ?? null;
  // A folder's page is there when the server served its frame (its directory's list may leave it out, cut at its bound);
  // ?file= of any other entry is no page
  const found = read && a !== null && !a.missing && (a.folder ? served : !at.file);
  const page = found ? a : null;
  // A page not found has the list's search in its bar, to look for it
  const seeking = read && !found;
  // The diff is opened once: the list is read every few seconds, and what is chosen and the place reached must stay
  const [diffOf, setDiffOf] = useState<Page | null>(null);
  if (diff && page && diffOf === null) setDiffOf(page);

  // Where the page goes back to, as the rows that opened it kept it, read anew for a page come back to from the
  // browser's memory; a press of the link that goes there keeps that the rows go on (order.comeBack)
  const [back, setBack] = useState(() => backOf(order.keptBack(), window.location.origin));
  useEffect(() => {
    const shown = (e: PageTransitionEvent) => {
      if (!e.persisted) return;
      setBack(backOf(order.keptBack(), window.location.origin));
      const now = openPage.frameAt();
      if (now) order.keepAt(now);
    };
    addEventListener('pageshow', shown);
    return () => {
      removeEventListener('pageshow', shown);
    };
  }, []);
  const up = frameUp(back, order);
  const words = backWords(back);

  const { openers, openFind, findToggle, openHelp, openSettings, modals } = useScreenModals('frame', {
    diff,
    backWords: words,
  });
  const actions = () => document.getElementById('frame-actions');
  const keys = appKeys({
    rows: frameRows({
      move: (step) => {
        frameMoves.moveFrame(step);
      },
      diff: diff
        ? (step) => {
            diffs.step(step);
          }
        : null,
    }),
    find: openFind,
    search: seeking
      ? () => {
          focusOn(document.getElementById('q'));
        }
      : undefined,
    findToggle,
    help: () => {
      openHelp('keys');
    },
    star: () => {
      pressStar(actions());
    },
    diff: () => {
      pressDiff(actions());
    },
    up: () => {
      // The back link is always there, hidden where the header's link goes the same way: h presses it alone
      pressUp('back');
    },
  });
  useKeys(keys);
  const keyNames = new Set(Object.keys(keys));
  const onKey = (key: string) => {
    keys[key]?.();
  };

  const name = (at.file || a?.title) ?? framed;
  useEffect(() => {
    document.title = `${name} · floe`;
  }, [name]);
  const list = `/${listQuery.savedList()}`;
  return (
    <>
      <Bar
        screen="frame"
        brand={list}
        back={<BackLink back={back} onPress={up.onPress} />}
        listUp={back.to === 'pages' ? up : undefined}
        taskUp={back.to === 'tasks' ? up : undefined}
        lead={
          <>
            <FrameHead page={page} file={at.file} />
            {read ? <VersionSelect at={page ? at : null} page={a} framed /> : null}
          </>
        }
        actions={
          page ? (
            <BarActions id="frame-actions" place="frame" page={page} at={at} diffing={diff} openers={openers} />
          ) : null
        }
        search={
          <SearchBox
            hidden={!seeking}
            go={(q) => {
              window.location.assign(`/?q=${encodeURIComponent(q.trim())}`);
            }}
          />
        }
        seeking={seeking}
        onFind={openFind}
        onHelp={() => {
          openHelp('about');
        }}
        onSettings={openSettings}
      />
      <TagLine id="frame-tags" page={page} hidden={!page || Boolean(at.file) || diff} onEdit={openers.onTags} />
      <SideTree folder={page && at.file ? page : null} file={at.file} />
      {read ? (
        <div className="print:hidden">
          <VersionNote at={page ? at : null} page={a} />
          <PageNote slug={page ? at.slug : undefined} />
        </div>
      ) : null}
      <FramePage at={at} found={found} diff={diff} name={name} keys={keyNames} onKey={onKey} />
      <DiffView page={diffOf} hidden={!found || !diff} keys={keyNames} onKey={onKey} />
      <NotFound page={a} slug={framed} back={list} hidden={!seeking} />
      {/* Printed in place of the page, which an iframe prints one screen of, but where the page is not found: that
          prints its words */}
      {seeking ? null : (
        <p id="print-alone" className="m-8 hidden text-fg print:block">
          <span id="print-alone-text">{t('ui_print_alone', t('ui_open_raw'))}</span>{' '}
          <TextLink id="print-raw" tone="inherit" href={new URL(rawHref(at), window.location.href).href}>
            {new URL(rawHref(at), window.location.href).href}
          </TextLink>
        </p>
      )}
      {modals}
    </>
  );
}
