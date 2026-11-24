// The versions' choice and words, beside the list and whole, over a fake server whose versions the buttons change
import { useState, type ReactNode } from 'react';

import { Button } from '@/components/button';
import { Dialog, DialogActions, DialogClose, DialogHead } from '@/components/dialog';
import { StarIcon } from '@/components/icons';
import { getBoot, type Boot } from '@/config/boot';
import { DiffView } from '@/features/versions/components/diff-view';
import { createDiff } from '@/features/versions/stores/diff';
import type { Section } from '@/features/versions/utils/diff-words';
import { useKeys } from '@/hooks/use-keys';
import { createKeptSide } from '@/stores/kept-side';
import { VersionNote } from '@/features/versions/components/version-note';
import { VersionSelect } from '@/features/versions/components/version-select';
import { createVersions, type Versions } from '@/features/versions/stores/versions';
import type { Version } from '@/features/versions/utils/version-words';
import { useStore } from '@/hooks/use-store';
import type { Client } from '@/lib/api-client';
import { createStore } from '@/lib/store';
import { createOpenPage, type OpenAt } from '@/stores/open-page';
import type { Reads } from '@/stores/reads';
import { toasts } from '@/stores/toasts';
import type { Page } from '@/types/page';

const hoursAgo = (h: number) => new Date(Date.now() - h * 3600000).toISOString();

// The server's versions of each page and the one read before, as the buttons change them
const kept: Record<string, { versions: Version[]; read: number }> = {
  three: { versions: [1, 2, 3].map((n) => ({ n, at: hoursAgo(30 - n * 6) })), read: 2 },
  one: { versions: [{ n: 1, at: hoursAgo(5) }], read: 1 },
};
let slow = false;
const server = {
  api: (url: string) =>
    new Promise<Response>((go) => {
      const slug = url.split('/')[3] ?? '';
      setTimeout(
        () => {
          go(new Response(JSON.stringify(kept[slug])));
        },
        slow ? 1500 : 50,
      );
    }),
} as unknown as Client;
// The list of pages, as the server tells it: what the choice reads its versions again by
const entry = (slug: string): Page => {
  const { versions } = kept[slug] ?? { versions: [] };
  return {
    slug,
    title: slug === 'three' ? 'Three versions' : 'One version',
    version: versions.at(-1)?.n ?? 0,
    versionCount: versions.length,
  } as Page;
};
const listed = createStore<Page[]>([entry('three'), entry('one')]);
const relist = () => {
  listed.set([entry('three'), entry('one')]);
};

const fakeReads = () =>
  ({
    keptUnread: createStore<string | null>(null),
    answered: createStore<{ slug: string } | null>(null),
  }) as unknown as Reads;
const say = (what: string) => (href?: string) => {
  toasts.toast(href === undefined ? what : `${what} ${href}`);
};

const besideOpen = createOpenPage({ boot: () => ({ screen: 'list', framed: '' }) as Boot, search: () => '' });
const besideReads = fakeReads();
const beside = createVersions({
  client: server,
  openPage: besideOpen,
  reads: besideReads,
  go: say('would go to'),
  reload: say('would reload'),
});
const frameBoot = (): Boot => ({ ...getBoot(), screen: 'frame', framed: 'three', version: '2' });
const frameOpen = createOpenPage({ boot: frameBoot, search: () => '?v=2' });
const frame = createVersions({
  client: server,
  openPage: frameOpen,
  reads: fakeReads(),
  boot: frameBoot,
  versionMove: { keep: () => undefined, take: () => ({ focus: false, kept: false }) },
  go: say('would go to'),
  reload: say('would reload'),
});

// Head is a page's head as the reader's (#read-head) and the frame's hold the choice, with the words under it
function Head({
  title,
  children,
  note,
}: {
  readonly title: string;
  readonly children: ReactNode;
  readonly note: ReactNode;
}) {
  return (
    <div className="overflow-hidden rounded-small border border-line bg-panel">
      <div className="flex h-lhead items-center gap-2.5 border-b border-line px-4">
        <b className="min-w-0 truncate">{title}</b>
        {children}
      </div>
      {note}
      <p className="m-0 p-4 text-muted">(the page)</p>
    </div>
  );
}

function Scene({
  v,
  at,
  page,
  framed = false,
}: {
  readonly v: Versions;
  readonly at: OpenAt | null;
  readonly page: Page | null;
  readonly framed?: boolean;
}) {
  return (
    <Head title={page?.title ?? '(none)'} note={<VersionNote at={at} page={page} versions={v} />}>
      <VersionSelect at={at} page={page} framed={framed} versions={v} />
    </Head>
  );
}

// startVersions is the frame's start, before anything is drawn (P10 does the same): what a move carried is taken once
export function startVersions(): void {
  frame.arrive();
}

export function VersionsSection() {
  const pages = useStore(listed);
  const picked = useStore(besideOpen.picked);
  const page = pages.find((p) => p.slug === picked?.slug) ?? null;
  const framePage = pages.find((p) => p.slug === 'three') ?? null;
  const frameAt = frameOpen.frameAt();
  const [slowNow, setSlow] = useState(false);
  const [modal, setModal] = useState(false);
  const publish = (slug: string) => {
    const k = kept[slug];
    if (!k) return;
    k.versions.push({ n: (k.versions.at(-1)?.n ?? 0) + 1, at: new Date().toISOString() });
    relist();
  };
  const remove = (slug: string, n: number) => {
    const k = kept[slug];
    if (!k) return;
    k.versions = k.versions.filter((x) => x.n !== n);
    relist();
  };
  return (
    <>
      <div className="mb-2 flex flex-wrap gap-2">
        {pages.map((p) => (
          <Button
            key={p.slug}
            pressed={picked?.slug === p.slug}
            onClick={() => {
              besideOpen.picked.set({ slug: p.slug, file: '', v: 0 });
            }}
          >
            {p.slug}
          </Button>
        ))}
        <Button
          onClick={() => {
            publish('three');
          }}
        >
          Publish a version
        </Button>
        <Button
          disabled={!picked?.v}
          onClick={() => {
            if (picked) remove(picked.slug, picked.v);
          }}
        >
          Remove the version shown
        </Button>
        <Button
          onClick={() => {
            const k = kept['three'];
            if (k) k.read = Math.max(1, k.read - 1);
            besideReads.answered.set({ slug: 'three' });
          }}
        >
          A read of three taken (moves the version read)
        </Button>
        <Button
          pressed={slowNow}
          onClick={() => {
            slow = !slowNow;
            setSlow(slow);
          }}
        >
          Slow answers
        </Button>
      </div>
      <div className="mb-4">
        <Scene v={beside} at={picked} page={page} />
      </div>
      <div className="mb-2 flex flex-wrap gap-2">
        <Button
          onClick={() => {
            setModal(true);
          }}
        >
          Open a modal
        </Button>
        <Button
          onClick={() => {
            remove('three', 2);
          }}
        >
          Remove v2 (the frame’s)
        </Button>
      </div>
      <Scene v={frame} at={frameAt} page={framePage} framed />
      <Dialog
        id="gallery-versions-dialog"
        open={modal}
        onClosed={() => {
          setModal(false);
        }}
        labelledBy="gallery-versions-dialog-title"
      >
        <DialogHead
          icon={<StarIcon />}
          titleId="gallery-versions-dialog-title"
          title="A modal open"
          lead="Remove v2 behind it: the frame waits until it closes, then would reload"
        />
        <DialogActions>
          <Button
            onClick={() => {
              remove('three', 2);
            }}
          >
            Remove v2
          </Button>
          <DialogClose />
        </DialogActions>
      </Dialog>
      <DiffScenes />
    </>
  );
}

// The diff, over a fake /versions and /diff: sections changed, added, removed and the same, one with no place, levels 1
// to 3, two sharing a place. Its URL is the gallery's own, told as it is written
const SECTIONS: Section[] = [
  { state: 'same', level: 1, heading: [{ op: '', text: 'Overview' }] },
  {
    state: 'changed',
    level: 2,
    heading: [
      { op: '', text: 'Plan ' },
      { op: 'del', text: 'old' },
      { op: 'ins', text: 'new' },
    ],
    place: 0,
  },
  { state: 'added', level: 3, heading: [{ op: 'ins', text: 'A step added' }], place: 0 },
  { state: 'removed', level: 2, heading: [{ op: 'del', text: 'A part gone' }], place: 1 },
  { state: 'changed', level: 5, heading: [], place: 1 },
];
let diffFails = false;
const diffServer = {
  api: (url: string, init?: RequestInit) =>
    new Promise<Response>((go, fail) => {
      init?.signal?.addEventListener('abort', () => {
        fail(new DOMException('called off', 'AbortError'));
      });
      setTimeout(
        () => {
          if (url.endsWith('/versions')) {
            versionsRead.set(versionsRead.get() + 1);
            go(new Response(JSON.stringify(kept['three'])));
            return;
          }
          if (diffFails) {
            go(new Response('refused by the gallery', { status: 500 }));
            return;
          }
          const q = new URL(url, 'http://floe').searchParams;
          go(
            new Response(
              JSON.stringify({
                from: { n: Number(q.get('from')) },
                to: { n: Number(q.get('to')) },
                key: 'k',
                counts: { changed: 2, added: 1, removed: 1, same: 1 },
                sections: SECTIONS,
                places: [1, 3],
              }),
            ),
          );
        },
        slow ? 1500 : 50,
      );
    }),
} as unknown as Client;
const diffOf = (query: string) => {
  let now = query;
  return createDiff({
    client: diffServer,
    search: () => now,
    replace: (href) => {
      now = new URL(href, 'http://floe').search;
      toasts.toast(`URL written: ${href}`);
    },
  });
};
const shownDiff = diffOf('?diff');
const badDiff = diffOf('?diff&from=9&to=1');
const noneDiff = diffOf('?diff');
const galleryToc = createKeptSide('gallery.diff.toc', 240);

// The versions read of three by the diffs, as the page's frame tells it they changed (pageChanged)
const versionsRead = createStore(0);
const SCENES = ['the pair', 'a pair that cannot be compared', 'a page of one version', 'hidden'] as const;
type Scene = (typeof SCENES)[number];
const hiddenDiff = diffOf('?diff');

// DiffScenes shows one diff at a time (their ids are the screen's): three's versions, one asked for by a pair that
// cannot be compared, a page of one version, and one hidden, which takes no word of its frame. n, p and d work from
// the page, not from a choice open
function DiffScenes() {
  const pages = useStore(listed);
  const three = pages.find((p) => p.slug === 'three') ?? null;
  const read = useStore(versionsRead);
  const [failsNow, setFails] = useState(false);
  const [scene, setScene] = useState<Scene>('the pair');
  const keys = {
    n: () => {
      shownDiff.step(1);
    },
    p: () => {
      shownDiff.step(-1);
    },
    d: () => {
      toasts.toast('d: would close the diff');
    },
  };
  useKeys(keys);
  const [diff, page] =
    scene === 'the pair'
      ? [shownDiff, three]
      : scene === 'a pair that cannot be compared'
        ? [badDiff, three]
        : scene === 'a page of one version'
          ? [noneDiff, { slug: 'one', versionCount: 1 } as Page]
          : [hiddenDiff, three];
  return (
    <>
      <h3 className="mt-6 mb-2 text-small font-bold text-muted">
        The diff (n, p, d; open a choice and press them: nothing; narrow the window under 56rem and 40rem)
      </h3>
      <div className="mb-2 flex flex-wrap gap-2">
        {SCENES.map((x) => (
          <Button
            key={x}
            pressed={scene === x}
            onClick={() => {
              setScene(x);
            }}
          >
            {x}
          </Button>
        ))}
      </div>
      <div className="mb-2 flex flex-wrap gap-2">
        <Button
          pressed={failsNow}
          onClick={() => {
            diffFails = !failsNow;
            setFails(diffFails);
          }}
        >
          Refuse the diffs
        </Button>
        <Button
          onClick={() => {
            (document.getElementById('diff-frame') as HTMLIFrameElement | null)?.contentWindow?.postMessage(
              { galleryTell: 'changed' },
              '*',
            );
          }}
        >
          Have the page tell its versions changed
        </Button>
        <span className="self-center text-small text-muted">{`versions read: ${String(read)}`}</span>
      </div>
      {/* Wider than the gallery's column, as the frame is the window's width */}
      <div className="flex h-[30rem] w-[min(72rem,calc(100vw-3rem))] flex-col overflow-hidden rounded-small border border-line">
        <DiffView
          key={scene}
          page={page}
          hidden={scene === 'hidden'}
          keys={new Set(Object.keys(keys))}
          onKey={(key) => {
            keys[key as keyof typeof keys]();
          }}
          diff={diff}
          toc={galleryToc}
        />
      </div>
    </>
  );
}
