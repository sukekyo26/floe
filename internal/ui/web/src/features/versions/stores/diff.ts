import { firstPair, type Pair } from '@/features/versions/utils/diff-pair';
import type { DiffAnswer } from '@/features/versions/utils/diff-words';
import type { VersionList } from '@/features/versions/utils/version-words';
import { client as pageClient, type Client } from '@/lib/api-client';
import { createStore, type Store } from '@/lib/store';
import { t } from '@/lib/t';
import { notice as pageNotice, type Notice } from '@/stores/notice';
import type { Page } from '@/types/page';
import { canDiff } from '@/utils/list-view';
import { diffHref, PAGES_API } from '@/utils/page-urls';

// What the diff shows: whether it was opened, none to compare, the versions, the pair chosen, the answer shown (of the
// pair answered, whatever is chosen meanwhile), the place n and p went to last (-1 for none) and the section of it,
// turn moving each time the frame is to load (an answer, a place, a step: the same place too), and bad telling the
// URL's pair could not be compared
export interface DiffState {
  opened: boolean;
  none: boolean;
  list: VersionList | null;
  pair: Pair | null;
  answer: DiffAnswer | null;
  at: number;
  section: number;
  turn: number;
  bad: boolean;
}

export interface Diff {
  state: Store<DiffState>;
  // open reads the versions of page and its first pair, once: as the page is first found
  open: (page: Page) => void;
  // reopen reads them again: the page's versions changed (pageChanged)
  reopen: (page: Page) => void;
  // choose sets one of the pair, and reads its diff
  choose: (name: keyof Pair, n: number) => void;
  // place goes to the section i of the contents; step to the next or the one before of the places (n, p)
  place: (i: number) => void;
  step: (by: 1 | -1) => void;
}

interface Deps {
  client?: Client;
  notice?: Notice;
  // search reads the URL's query now; replace writes the URL in place of this one
  search?: () => string;
  replace?: (href: string) => void;
}

const messageOf = (err: unknown) => (err instanceof Error ? err.message : String(err));

// createDiff is what changed between two versions, read and moved through: one for a load of the frame at ?diff, as
// what closes it (the bar's button, j and k) loads another URL. A read asked again calls the one before off: one
// controller for /versions and /diff alike
// LIMIT: it does not follow a version published while it is open: reload it, or choose
// LIMIT: the contents follow n, p and a press, not the scrolling
export function createDiff({
  client = pageClient,
  notice = pageNotice,
  search = () => window.location.search,
  replace = (href) => {
    window.history.replaceState(window.history.state, '', href);
  },
}: Deps = {}): Diff {
  const state = createStore<DiffState>({
    opened: false,
    none: false,
    list: null,
    pair: null,
    answer: null,
    at: -1,
    section: -1,
    turn: 0,
    bad: false,
  });
  const put = (change: Partial<DiffState>) => {
    state.set({ ...state.get(), ...change });
  };
  let slug = '';
  let call: AbortController | null = null;
  let failed = false;
  const restart = () => {
    call?.abort();
    call = new AbortController();
    return call.signal;
  };
  // A read called off says nothing; one that fails says so on the error line
  const fail = (signal: AbortSignal, err: unknown) => {
    if (signal.aborted) return false;
    failed = true;
    notice.say(t('ui_diff_failed', messageOf(err)));
    return true;
  };

  // What is shown is of the pair answered; one that fails goes back to what is shown, its choices and URL with it
  async function read(signal: AbortSignal) {
    const { pair } = state.get();
    if (!pair) return;
    let answer: DiffAnswer;
    try {
      const res = await client.api(
        `${PAGES_API}/${encodeURIComponent(slug)}/diff?from=${String(pair.from)}&to=${String(pair.to)}`,
        { signal },
      );
      if (!res.ok) throw new Error(await res.text());
      answer = (await res.json()) as DiffAnswer;
    } catch (err) {
      if (!fail(signal, err)) return;
      const shown = state.get().answer;
      if (!shown) return;
      put({ pair: { from: shown.from.n, to: shown.to.n } });
      replace(diffHref(slug, shown.from.n, shown.to.n));
      return;
    }
    if (signal.aborted) return;
    if (failed) {
      failed = false;
      notice.say('');
    }
    put({ answer, at: -1, section: -1, turn: state.get().turn + 1 });
  }

  async function start(page: Page) {
    slug = page.slug;
    const signal = restart();
    if (!canDiff(page)) {
      put({ none: true });
      return;
    }
    let list: VersionList;
    try {
      const res = await client.api(`${PAGES_API}/${encodeURIComponent(slug)}/versions`, { signal });
      if (!res.ok) throw new Error(await res.text());
      list = (await res.json()) as VersionList;
    } catch (err) {
      fail(signal, err);
      return;
    }
    if (signal.aborted) return;
    const { pair, bad } = firstPair(search(), list);
    // The words stay until a version is chosen: a read again leaves them
    put({ none: false, list, pair, bad: state.get().bad || bad });
    if (bad) replace(diffHref(slug));
    await read(signal);
  }

  return {
    state,
    open: (page) => {
      // Set before anything waits: a second call (a draw done twice) reads nothing
      if (state.get().opened) return;
      put({ opened: true });
      void start(page);
    },
    reopen: (page) => {
      void start(page);
    },
    choose: (name, n) => {
      const { pair } = state.get();
      if (!pair) return;
      const next = { ...pair, [name]: n };
      put({ pair: next, bad: false });
      replace(diffHref(slug, next.from, next.to));
      void read(restart());
    },
    place: (i) => {
      const { answer, turn } = state.get();
      const at = answer?.sections[i]?.place;
      if (at === undefined) return;
      put({ at, section: i, turn: turn + 1 });
    },
    step: (by) => {
      const { answer, at, turn } = state.get();
      const next = at + by;
      if (!answer || next < 0 || next >= answer.places.length) return;
      // The section of the place, as the answer tells it: what is gone of a section may lie after the next one's place
      put({ at: next, section: answer.places[next] ?? -1, turn: turn + 1 });
    },
  };
}
export const diff = createDiff();
