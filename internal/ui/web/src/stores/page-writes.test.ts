import { describe, expect, it } from 'vitest';

import { setBoot, type Boot } from '@/config/boot';
import type { Client } from '@/lib/api-client';
import { createNotice } from '@/stores/notice';
import { createPageWrites } from '@/stores/page-writes';

const answering = (status: number): Client =>
  ({ api: () => Promise.resolve(new Response(status === 204 ? null : 'refused', { status })) }) as unknown as Client;

function made(status: number) {
  setBoot({ messages: { ui_http_error: '%s (%d)' } } as unknown as Boot);
  const notice = createNotice();
  notice.say('before');
  return { notice, writes: createPageWrites({ client: answering(status), notice }) };
}

describe(createPageWrites, () => {
  it('takes back the error said before once a write goes through, and keeps it with the reason of one refused', async () => {
    expect.assertions(4);
    const ok = made(204);
    await expect(ok.writes.patchPage('a', { favorite: true })).resolves.toBe('');
    expect(ok.notice.said.get().message).toBe('');
    const refused = made(500);
    await expect(refused.writes.patchPage('a', { favorite: true })).resolves.toBe('refused (500)');
    expect(refused.notice.said.get().message).toBe('before');
  });

  it('counts a page gone as deleted, and as nothing else', async () => {
    expect.assertions(2);
    await expect(made(404).writes.deletePage('a')).resolves.toBe('');
    await expect(made(404).writes.patchPage('a', {})).resolves.toBe('refused (404)');
  });
});
