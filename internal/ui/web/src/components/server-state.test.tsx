import { renderToString } from 'react-dom/server';
import { describe, expect, it } from 'vitest';

import { ErrorBar } from '@/components/error-bar';
import { ServerState } from '@/components/server-state';
import { testBoot } from '@/components/test-boot';
import { createClient } from '@/lib/api-client';
import { createNotice } from '@/stores/notice';

describe(ServerState, () => {
  it('says nothing while connected and the words when the connection is lost, the address in its name', () => {
    expect.assertions(3);
    testBoot();
    const client = createClient();
    expect(renderToString(<ServerState client={client} host={() => 'h:1'} />)).toMatch(
      /^<output id="server" title="ui_server_up:h:1" aria-label="ui_server_up:h:1" class="[^"]*"><\/output>$/,
    );
    client.reached.set(false);
    const down = renderToString(<ServerState client={client} host={() => 'h:1'} />);
    expect(down).toContain('data-down="true"');
    expect(down).toMatch(/>ui_server_down:[^<]*<\/output>$/);
  });
});

describe(ErrorBar, () => {
  it('is hidden with no mistake, and says the one there is', () => {
    expect.assertions(2);
    const notice = createNotice();
    expect(renderToString(<ErrorBar notice={notice} />)).toContain('hidden=""');
    notice.say('failed');
    expect(renderToString(<ErrorBar notice={notice} />)).toMatch(/role="alert"(?![^>]*hidden)[^>]*>failed</);
  });
});
