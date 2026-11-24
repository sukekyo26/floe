import type { Client } from '@/lib/api-client';
import { t } from '@/lib/t';
import type { Listing, Page, ReadMark } from '@/types/page';
import { PAGES_API } from '@/utils/page-urls';

// readPages is one read of the list: its text as answered, to tell a change, and its entries
export async function readPages(client: Client, signal: AbortSignal): Promise<{ text: string; entries: Page[] }> {
  const res = await client.api(PAGES_API, { signal });
  if (!res.ok) throw new Error(`HTTP ${res.status}`);
  const text = await res.text();
  return { text, entries: JSON.parse(text) as Page[] };
}

// readListing is GET /api/pages/{slug}/files of dir: null for one that is not there (any more), an Error for another
// refusal
export async function readListing(
  client: Client,
  slug: string,
  dir: string,
  signal?: AbortSignal,
): Promise<Listing | Error | null> {
  const res = await client.api(
    `${PAGES_API}/${encodeURIComponent(slug)}/files?dir=${encodeURIComponent(dir)}`,
    signal ? { signal } : undefined,
  );
  if (res.status === 404) return null;
  if (!res.ok) return new Error(t('ui_http_error', await res.text(), res.status));
  return (await res.json()) as Listing;
}

// sendPage sends a change of the page slug: the reason of a failure, '' when written, and the store's answer when
// asked for. gone counts a 404 as done, for a delete
export async function sendPage(
  client: Client,
  slug: string,
  init: RequestInit,
  { gone = false, answer = false } = {},
): Promise<{ reason: string; answer: unknown }> {
  try {
    const res = await client.api(`${PAGES_API}/${slug}`, init);
    if (!res.ok && !(gone && res.status === 404)) {
      return { reason: t('ui_http_error', await res.text(), res.status), answer: null };
    }
    // An answer of no JSON, as a stubbed server gives, tells nothing more
    return { reason: '', answer: answer ? await res.json().catch(() => null) : null };
  } catch (err) {
    return { reason: err instanceof Error ? err.message : String(err), answer: null };
  }
}

// sendRead tells the server the version read of slug, or, for null, marks it unread; cut off at 5 seconds
export const sendRead = (client: Client, slug: string, read: ReadMark | null): Promise<Response> => {
  const url = `${PAGES_API}/${encodeURIComponent(slug)}/read`;
  const signal = AbortSignal.timeout(5000);
  return client.api(
    url,
    read
      ? { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(read), signal }
      : { method: 'DELETE', signal },
  );
};
