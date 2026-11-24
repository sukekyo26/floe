import { client as pageClient, type Client } from '@/lib/api-client';
import { sendRead } from '@/lib/pages';
import type { ReadMark } from '@/types/page';
import { createStore, type Store } from '@/lib/store';
import { t } from '@/lib/t';
import { notice as pageNotice, type Notice } from '@/stores/notice';
import { openPage as pageOpen, type OpenPage } from '@/stores/open-page';
import { pages as pagePages, type Pages } from '@/stores/pages';

// What a read mark asks: the version read and whether to say so when it clears the page's news; null marks it unread
export type Told = { read: ReadMark; say: boolean } | null;

// How a mark ended, as of its answer: dropped for a later mark of the same page, gone with the page, sent (cleared
// telling a read that cleared the page's news), or failed, said on the page's error line or not
export type ReadOutcome =
  | { kind: 'dropped' }
  | { kind: 'gone' }
  | { kind: 'sent'; cleared: boolean }
  | { kind: 'failed'; reason: string; said: boolean };

export interface Reads {
  // keptUnread is the page marked unread while it is shown, which its reads leave unread
  keptUnread: Store<string | null>;
  // tellRead sends the version read of slug, or marks it unread, one request at a time: while one is out, each page's
  // last mark waits, its earlier ones dropped. The slug is the caller's own, never one a page's message names
  tellRead: (slug: string, told: Told) => Promise<ReadOutcome>;
  // markUnread marks slug unread, and keeps it so while it is shown; an archived page is never unseen, so null
  markUnread: (slug: string) => Promise<ReadOutcome> | null;
  // answered is the page of the last mark the server took, read or unread: a new value each time, as the version read
  // it moves is told by nothing in the list of pages
  answered: Store<{ slug: string } | null>;
}

interface Deps {
  client?: Client;
  notice?: Notice;
  pages?: Pages;
  openPage?: OpenPage;
}

// LIMIT: a read cut off at 5 seconds, or a mark so, may still be written by the server after what follows it, while the
// store's lock is held long: the page ends as read, or unread with the frame no longer holding it. Hold them in order if
// that comes to matter
export function createReads({
  client = pageClient,
  notice = pageNotice,
  pages = pagePages,
  openPage = pageOpen,
}: Deps = {}): Reads {
  const keptUnread = createStore<string | null>(null);
  const answered = createStore<{ slug: string } | null>(null);
  const waiting = new Map<string, { told: Told; done: (outcome: ReadOutcome) => void }>();
  let going = false;

  async function send() {
    going = true;
    // A Map's iteration also visits what is set while it goes
    for (const [to] of waiting) {
      // Taken once the load's reading ends, so that the page's last mark is the one sent
      for (let reading = pages.whileReading(); reading; reading = pages.whileReading()) await reading;
      const mark = waiting.get(to);
      if (!mark) continue;
      waiting.delete(to);
      const { told, done } = mark;
      // The mark that did not reach the server holds nothing back
      const letGo = () => {
        if (!told && keptUnread.get() === to && !waiting.has(to)) keptUnread.set(null);
      };
      const fail = (reason: string) => {
        notice.say(reason, 'read');
        letGo();
        done({ kind: 'failed', reason, said: true });
      };
      // Whether the server took the mark: told once the try is over, so that what listens cannot fail the mark
      let took = false;
      try {
        const res = await sendRead(client, to, told?.read ?? null);
        if (!res.ok && res.status !== 404) {
          fail(t(told ? 'ui_read_failed' : 'ui_unread_failed', t('ui_http_error', await res.text(), res.status)));
          continue;
        }
        notice.say('', 'read');
        if (res.status === 404) {
          letGo();
          done({ kind: 'gone' });
        } else if (told?.say) {
          // Read only when it may be said: an answer of no JSON fails no mark that needs none
          done({ kind: 'sent', cleared: Boolean(((await res.json()) as { cleared?: boolean }).cleared) });
        } else {
          done({ kind: 'sent', cleared: false });
        }
        took = res.status !== 404;
      } catch (err) {
        const message = err instanceof Error ? err.message : String(err);
        // A read is told again the next time it is opened; a mark, only if pressed again
        if (!told) fail(t('ui_unread_failed', message));
        else done({ kind: 'failed', reason: message, said: false });
      }
      if (took) answered.set({ slug: to });
    }
    going = false;
    // The dot and the counts go now, not at the next look
    void pages.load(true);
  }

  function tellRead(slug: string, told: Told): Promise<ReadOutcome> {
    return new Promise((done) => {
      const before = waiting.get(slug);
      before?.done({ kind: 'dropped' });
      // A read that replaces one to be said is said (S428: a page opened that reads itself anew at once tells twice, the
      // second not to be said); a mark unread is its own. A mark already sent is not waiting, and is not said twice
      const said = told && before?.told?.say === true ? { ...told, say: true } : told;
      // A page waiting keeps its place, as a Map's set does
      waiting.set(slug, { told: said, done });
      if (!going) void send();
    });
  }

  return {
    keptUnread,
    answered,
    tellRead,
    markUnread: (slug) => {
      // Archived, it is never unseen: from a toast shown before it was archived too
      if (pages.state.get().entries.find((a) => a.slug === slug)?.archived) return null;
      if (slug === openPage.shownSlug()) keptUnread.set(slug);
      return tellRead(slug, null);
    },
  };
}

export const reads = createReads();
