import { createStore, type Store } from '@/lib/store';

// The statuses whose answer holds no body: a Response of them is made with none
const NO_BODY = new Set([101, 204, 205, 304]);

export interface Client {
  // api asks the server: every request of the page goes through it. Whether the server was reached is in reached; an
  // HTTP error is still an answer
  api: (url: string, init?: RequestInit) => Promise<Response>;
  // noWrites is a promise of the moment no write waits for its answer any more, or null when none waits
  noWrites: () => Promise<void> | null;
  // writesSent counts the writes sent, for a read to tell one sent while it read
  writesSent: () => number;
  reached: Store<boolean>;
}

// createClient counts the writes (any request but a GET) sent, and those not answered yet: the list is read only while
// none goes, so that what an answer says is never put over what a read had since
export function createClient(): Client {
  let sent = 0;
  let going = 0;
  const done: (() => void)[] = [];
  const reached = createStore(true);

  async function api(url: string, init?: RequestInit): Promise<Response> {
    const write = Boolean(init?.method && init.method !== 'GET');
    if (write) {
      sent++;
      going++;
    }
    try {
      // Looked up at each call, never kept: the browser tests replace window.fetch once the page has loaded
      const res = await window.fetch(url, init);
      reached.set(true);
      if (!write) return res;
      // A write ends once its answer is read, as a read of the list waits for it
      const body = NO_BODY.has(res.status) ? null : await res.text();
      return new Response(body, { status: res.status, statusText: res.statusText, headers: res.headers });
    } catch (err) {
      // One cut off by its own time limit, or called off by its caller, tells nothing of the server
      const name = err instanceof Error ? err.name : '';
      if (name !== 'TimeoutError' && name !== 'AbortError') reached.set(false);
      throw err;
    } finally {
      if (write && --going === 0) for (const go of done.splice(0)) go();
    }
  }

  return {
    api,
    noWrites: () =>
      going
        ? new Promise((go) => {
            done.push(go);
          })
        : null,
    writesSent: () => sent,
    reached,
  };
}

// client is the page's one
export const client = createClient();
