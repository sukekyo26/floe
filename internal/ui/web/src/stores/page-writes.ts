import { client as pageClient, type Client } from '@/lib/api-client';
import { sendPage } from '@/lib/pages';
import { notice as pageNotice, type Notice } from '@/stores/notice';

export interface PageWrites {
  // writePage sends a change of one page and returns the reason of a failure, '' when written; one written takes back
  // the error said before. gone counts a 404 as done, for a delete; answer takes what the store left
  writePage: (
    slug: string,
    init: RequestInit,
    options?: { gone?: boolean; answer?: (got: unknown) => void },
  ) => Promise<string>;
  // patchPage edits a page (its title, star, archive, tags…): '' or the reason
  patchPage: (slug: string, body: unknown) => Promise<string>;
  // deletePage deletes a stored page or unregisters a link: a page gone already counts as done
  deletePage: (slug: string) => Promise<string>;
}

interface Deps {
  client?: Client;
  notice?: Notice;
}

// createPageWrites is the one way the screen writes a page: the task writes go by stores/tasks.ts, which takes back no
// error
export function createPageWrites({ client = pageClient, notice = pageNotice }: Deps = {}): PageWrites {
  async function writePage(
    slug: string,
    init: RequestInit,
    { gone = false, answer }: { gone?: boolean; answer?: (got: unknown) => void } = {},
  ): Promise<string> {
    const sent = await sendPage(client, slug, init, { gone, answer: answer !== undefined });
    if (sent.reason) return sent.reason;
    answer?.(sent.answer);
    notice.say('');
    return '';
  }
  return {
    writePage,
    patchPage: (slug, body) =>
      writePage(slug, { method: 'PATCH', headers: { 'content-type': 'application/json' }, body: JSON.stringify(body) }),
    deletePage: (slug) => writePage(slug, { method: 'DELETE' }, { gone: true }),
  };
}

export const pageWrites = createPageWrites();
