import { getBoot, type Boot } from '@/config/boot';
import {
  versionLeave,
  versionsWanted,
  versionView,
  type SelectView,
  type VersionView,
} from '@/features/versions/utils/version-view';
import type { VersionList } from '@/features/versions/utils/version-words';
import { client as pageClient, type Client } from '@/lib/api-client';
import { createStore, type Store } from '@/lib/store';
import { openPage as pageOpen, type OpenAt, type OpenPage } from '@/stores/open-page';
import { reads as sharedReads, type Reads } from '@/stores/reads';
import { versionMove as sharedMove, type VersionMove } from '@/stores/version-move';
import type { Page } from '@/types/page';
import { PAGES_API, pageHref } from '@/utils/page-urls';

// What is known of the versions of the page shown: the page they are of, the key of the entry they were read for, the
// list ('' and null for none), whether the version asked for was gone, whether the choice is to take the focus once
// shown, and whether the frame waits for a modal to close before it goes to the latest
export interface VersionsState {
  slug: string | undefined;
  key: string;
  list: VersionList | null;
  gone: boolean;
  focus: boolean;
  waiting: boolean;
}

export interface Versions {
  state: Store<VersionsState>;
  // view is what the choice and its words show of at, page its entry: never with the versions of another page
  view: (at: OpenAt | null, page: Page | null) => VersionView;
  // want reads the versions of at as its entry page has changed, and moves to the latest a version given way
  want: (at: OpenAt | null, page: Page | null) => void;
  // again reads the versions of the page last wanted anew: a mark read or unread moves the version read before
  again: () => void;
  // pick opens the version n of the page shown; focus takes the choice the focus, held tells it has the focus now
  pick: (n: number, how: { focus: boolean; held: boolean }) => void;
  // arrive takes, once as the frame starts, what a move to this version carried over and the version asked for gone
  arrive: () => void;
  // takeFocus tells, once the choice is drawn of versions read (select), whether to give it the focus asked for: asked
  // and shown. It is taken either way, so that a choice hidden then does not take the focus when it shows later
  takeFocus: (select: SelectView | null) => boolean;
}

interface Deps {
  client?: Client;
  openPage?: OpenPage;
  reads?: Reads;
  versionMove?: VersionMove;
  boot?: () => Boot;
  go?: (href: string) => void;
  reload?: () => void;
  // afterModal runs run once the modal open closes, and tells there was one; with none open it does nothing
  afterModal?: (run: () => void) => boolean;
}

function modalClosed(run: () => void): boolean {
  const modal = document.querySelector('dialog[open]');
  modal?.addEventListener('close', run, { once: true });
  return modal !== null;
}

const keyOf = (a: Page) => [a.slug, a.version, a.versionCount, a.news].join();
const same = (x: OpenAt | null, y: OpenAt) => x !== null && x.slug === y.slug && x.file === y.file && x.v === y.v;

// createVersions is the choice of a page's versions: one per screen, beside the list or whole. The versions are read as
// the entry of the page shown changes; each want takes a turn, and an answer come after another want, read or not, is
// dropped
// LIMIT: a browser without base-select (Firefox and Safari for now) shows the whole words closed, and goes to a version at each arrow key
// LIMIT: a list that could not be read is read again only as the list of pages changes; the page shown does not wait for it
// LIMIT: the page of a version gone asks first, up to 3 seconds before this, and shows its 404 until then
// LIMIT: a number come again after a removal, its page may ask first and tell it was read, the version it shows being the latest
export function createVersions({
  client = pageClient,
  openPage = pageOpen,
  reads = sharedReads,
  versionMove = sharedMove,
  boot = getBoot,
  go = (href) => {
    window.location.assign(href);
  },
  reload = () => {
    window.location.reload();
  },
  afterModal = modalClosed,
}: Deps = {}): Versions {
  const state = createStore<VersionsState>({
    slug: undefined,
    key: '',
    list: null,
    gone: false,
    focus: false,
    waiting: false,
  });
  // A change of nothing tells no one, as the choice wants the versions again as it is drawn
  const put = (change: Partial<VersionsState>) => {
    const now = state.get();
    if (Object.entries(change).some(([k, v]) => !Object.is(now[k as keyof VersionsState], v)))
      state.set({ ...now, ...change });
  };
  const framed = () => boot().screen === 'frame';
  // The page whose versions are read, as last wanted: null for one whose versions are not
  let wanted: { at: OpenAt; slug: string; key: string } | null = null;
  let turn = 0;
  let leaving = false;

  // The page shown gives way to the latest, no one having asked to open it: told read no more than a page marked unread
  function leave(at: OpenAt, key: string, list: VersionList, gone: boolean) {
    if (!framed()) {
      reads.keptUnread.set(at.slug);
      leaving = true;
      openPage.picked.set({ ...at, v: 0 });
      leaving = false;
      put({ key, list, gone });
      return;
    }
    // Not from under what is being typed: the choice and words stay until the modal closes, then the versions are
    // read again and the page goes
    if (state.get().waiting) return;
    if (
      afterModal(() => {
        put({ waiting: false });
        again();
      })
    ) {
      put({ key, waiting: true });
      return;
    }
    versionMove.keep(false, true);
    // The server answers as for a URL that asks for it, and all that follows from the URL goes with it
    reload();
  }

  // What is read of at, as its key: kept, or the page left for the latest. Beside the list, an answer for another
  // version than the one open now (a row picked before the read it starts) writes nothing
  function answer(at: OpenAt, key: string, list: VersionList) {
    if (!framed() && !same(openPage.picked.get(), at)) return;
    const away = versionLeave(at, list);
    if (away) leave(at, key, list, away.gone);
    else put({ key, list });
  }

  async function read(at: OpenAt, key: string) {
    const asked = ++turn;
    let list: VersionList;
    try {
      const res = await client.api(`${PAGES_API}/${encodeURIComponent(at.slug)}/versions`);
      if (!res.ok) return;
      list = (await res.json()) as VersionList;
    } catch {
      // The client has told if the server was not reached
      return;
    }
    if (asked === turn) answer(at, key, list);
  }

  function again() {
    if (!wanted) return;
    put({ key: '' });
    void read(wanted.at, wanted.key);
  }

  reads.answered.subscribe(() => {
    if (wanted && reads.answered.get()?.slug === wanted.slug) again();
  });
  // A row picked, or a version chosen, ends the words of one gone; the frame's are the server's, and stay
  openPage.picked.subscribe(() => {
    if (!leaving && !framed() && state.get().gone) put({ gone: false });
  });

  return {
    state,
    view: (at, page) => {
      const s = state.get();
      return versionView({
        at,
        page,
        list: page && s.slug === page.slug ? s.list : null,
        gone: s.gone,
        diff: openPage.diff(),
      });
    },
    want: (at, page) => {
      const s = state.get();
      if (s.waiting) return;
      turn++;
      // Nothing of another page stays while this one's is read, or if it cannot be
      if (page?.slug !== s.slug) put({ slug: page?.slug, key: '', list: null });
      if (!page && s.gone) put({ gone: false });
      if (!at || !page || !versionsWanted(at, page, openPage.diff())) {
        wanted = null;
        put({ key: '' });
        return;
      }
      const key = keyOf(page);
      wanted = { at, slug: page.slug, key };
      const { key: had, list } = state.get();
      if (key === had && list) answer(at, key, list);
      else void read(at, key);
    },
    again,
    pick: (n, { focus, held }) => {
      // The latest as the choice lists it: the list of pages, read at other times, may not know of it yet
      const v = n === state.get().list?.versions.at(-1)?.n ? 0 : n;
      if (framed()) {
        const slug = boot().framed;
        versionMove.keep(focus || held, reads.keptUnread.get() === slug);
        go(pageHref({ slug, v }));
        return;
      }
      const at = openPage.picked.get();
      if (!at) return;
      openPage.picked.set({ ...at, v });
      put({ gone: false, focus });
    },
    arrive: () => {
      const { focus, kept } = versionMove.take();
      put({ focus, gone: boot().version === 'gone' });
      if (kept) reads.keptUnread.set(boot().framed);
    },
    takeFocus: (select) => {
      if (!state.get().focus || !select) return false;
      put({ focus: false });
      return !select.hidden;
    },
  };
}

export const versions = createVersions();
